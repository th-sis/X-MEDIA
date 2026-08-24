// [V7 §8 实测回归] PanSou 上游真实响应契约 (2026-08-24 抓取) 是
//   {"code":0,"message":"success","data":{"total":<n>,"merged_by_type":{...}}}
// 与 §8.2 文档示例 (顶层 merged_by_type) 不一致; 且只有 /api/search +
// /api/health 两个端点, /api/check/links 上游不存在 → 检测能力
// 必须降级为"全部视为有效"或"本地启发式". 本测试锁定这些契约.

package pansearch

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"xmedia/internal/domain"
)

// fakePanSou 起一个真 PanSou 形态的服务 (含 code/message/data 外壳),
// 关键词 → 资源的映射由 caller 注入.
func fakePanSou(t *testing.T, kws map[string]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/health":
			_, _ = io.WriteString(w, `{"status":"ok","auth_enabled":false,"plugin_count":16}`)
		case "/api/search":
			var req struct {
				KW         string   `json:"kw"`
				CloudTypes []string `json:"cloud_types"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			link, ok := kws[req.KW]
			if !ok {
				link = "https://pan.quark.cn/s/no-result-for-" + req.KW
			}
			resp := map[string]any{
				"code":    0,
				"message": "success",
				"data": map[string]any{
					"total": 1,
					"merged_by_type": map[string][]map[string]any{
						"quark": {{
							"url":      link,
							"password": "",
							"note":     req.KW + " 测试资源 4K",
							"datetime": time.Now().UTC().Format(time.RFC3339),
							"source":   "tg:test_channel",
						}},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// memPansearchCache 内存版缓存 (避免依赖 SQL migration).
type memPansearchCache struct{ m map[string]struct {
	results string
	cnt     int
	at      time.Time
} }

func newMemPansearchCache() *memPansearchCache {
	return &memPansearchCache{m: map[string]struct {
		results string
		cnt     int
		at      time.Time
	}{}}
}

func (c *memPansearchCache) Get(_ context.Context, kw, cloudTypes string) (string, int, *time.Time, error) {
	v, ok := c.m[kw+"|"+cloudTypes]
	if !ok {
		return "", 0, nil, nil
	}
	return v.results, v.cnt, &v.at, nil
}
func (c *memPansearchCache) Set(_ context.Context, kw, cloudTypes, results string, cnt int) error {
	c.m[kw+"|"+cloudTypes] = struct {
		results string
		cnt     int
		at      time.Time
	}{results, cnt, time.Now().UTC()}
	return nil
}
func (c *memPansearchCache) MarkStale(_ context.Context, kw, cloudTypes string) error {
	if v, ok := c.m[kw+"|"+cloudTypes]; ok { v.cnt = 0; c.m[kw+"|"+cloudTypes] = v }
	return nil
}
func (c *memPansearchCache) Delete(_ context.Context, _ string) error {
	c.m = map[string]struct {
		results string
		cnt     int
		at      time.Time
	}{}
	return nil
}

// memConfig 内存版配置 (写入 pansearch_url).
type memConfig struct{ v map[string]string }

func (c *memConfig) Get(_ context.Context, k string) (string, bool, error) {
	v, ok := c.v[k]
	return v, ok, nil
}
func (c *memConfig) Set(_ context.Context, k, v string) error { c.v[k] = v; return nil }
func (c *memConfig) Delete(_ context.Context, k string) error    { delete(c.v, k); return nil }
func (c *memConfig) All(_ context.Context) (map[string]string, error) {
	out := map[string]string{}
	for k, v := range c.v { out[k] = v }
	return out, nil
}

// [V7 §8 实测回归] 上游真实契约: data.merged_by_type.
func TestSearch_RealPanSouResponseContract(t *testing.T) {
	srv := fakePanSou(t, map[string]string{
		"Inception": "https://pan.quark.cn/s/inception-real",
	})
	cfg := &memConfig{v: map[string]string{
		domain.ConfigPansearchURL: srv.URL,
	}}
	s := NewService(cfg, newMemPansearchCache())

	results, err := s.Search(context.Background(), SearchRequest{
		Keyword:    "Inception",
		CloudTypes: []string{"quark"},
	})
	if err != nil {
		t.Fatalf("Search 失败: %v", err)
	}
	if len(results) == 0 {
		t.Fatalf("真 PanSou 契约下应能解析到资源, got 0")
	}
	got := results[0]
	if !strings.Contains(got.ShareURL, "inception-real") {
		t.Fatalf("URL 应为 https://pan.quark.cn/s/inception-real, got %q", got.ShareURL)
	}
	if got.Source != "quark" {
		t.Fatalf("Source 应映射为 quark, got %q", got.Source)
	}
}

// [V7 §8 实测回归] 上游真正不存在 /api/check/links 端点, 本地启发式检测必须降级.
func TestCheckLinks_FallsBackWhenUpstreamLacksEndpoint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r) // 所有端点都 404, 模拟当前 PanSou 主分支
	}))
	t.Cleanup(srv.Close)
	cfg := &memConfig{v: map[string]string{
		domain.ConfigPansearchURL: srv.URL,
	}}
	s := NewService(cfg, newMemPansearchCache())
	items := []CheckItem{
		{DiskType: "quark", URL: "https://pan.quark.cn/s/x1", Password: ""},
		{DiskType: "115", URL: "https://115.com/s/x2", Password: ""},
	}
	got, err := s.CheckLinks(context.Background(), items)
	// 设计意图 (§6.4 上游检测失败时降级): 必须返回 err 表示能力缺失, 不得静默冒充成功.
	if err == nil {
		t.Fatalf("/api/check/links 上游不存在, 应返回明确错误让调用方降级, got err=nil, results=%v", got)
	}
	if len(got) != 0 {
		t.Fatalf("上游失败时不应返回编造的 ok 结果, got %v", got)
	}
}