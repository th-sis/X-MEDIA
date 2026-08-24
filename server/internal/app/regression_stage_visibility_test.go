// [V7 §6.4 / §27.4 实测回归] resolve 引擎对账号空/SaveShare 失败的可见性.
// 上轮真机测试发现 20 条 SaveShare 全失败时用户只见 not_found, 不知根因;
// 账号空时 runP1 静默 return false 也无任何提示. 本测试锁定两个
// 新增 Stage 的可见性, 让 §27.4 健康面板可显示具体操作按钮.

package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"xmedia/internal/domain"
)

// fakePanSouEmpty 复刻真上游契约, 关键词→资源; 此次用 Inception 触发 50 个命中.
func fakePanSouEmpty(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/health":
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/api/search":
			out := map[string]any{
				"code": 0, "message": "success",
				"data": map[string]any{
					"total": 1,
					"merged_by_type": map[string][]map[string]any{
						"quark": {{
							"url":      "https://pan.quark.cn/s/a2a3",
							"note":     "测试 4K", "datetime": "2026-01-01T00:00:00Z",
							"source": "tg:test",
						}},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(out)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// [V7 §6.4 / §27.4] A3 e2e: 无网盘账号时 runP1 必须留 StageNoAccount 痕迹
// (而不是静默 not_found).
func TestE2E_Phase10_NoAccountPushesStageNoAccount(t *testing.T) {
	env := newPhase10Env(t)
	env.setConfig(domain.ConfigPansearchURL, fakePanSouEmpty(t).URL)
	// 故意不预置任何账号.

	resp, raw := env.do(http.MethodPost, "/api/resolve", map[string]any{
		"external_id": int64(27205), "external_source": "tmdb", "media_type": "movie",
		"title": "Inception A3", "year": 2010,
	}, nil)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		t.Fatalf("resolve status=%d body=%s", resp.StatusCode, raw)
	}
	var r struct {
		TaskID int64 `json:"task_id"`
	}
	_ = json.Unmarshal(raw, &r)

	// 轮询结果, 必须出现 stage=no_account.
	saw := false
	deadline := time.Now().Add(15 * time.Second)
	var lastRaw []byte
	for time.Now().Before(deadline) {
		resp, raw = env.do(http.MethodGet,
			fmt.Sprintf("/api/resolve/result/%d", r.TaskID), nil, nil)
		lastRaw = raw
		var rr struct {
			Stage string `json:"stage"`
		}
		_ = json.Unmarshal(raw, &rr)
		if rr.Stage == "no_account" {
			saw = true
		}
		if strings.Contains(string(raw), `"status":"done"`) ||
			strings.Contains(string(raw), `"status":"failed"`) {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	if !saw {
		var last struct {
			Stage       string `json:"stage"`
			StageDetail string `json:"stage_detail"`
		}
		_ = json.Unmarshal(lastRaw, &last)
		t.Fatalf("无账号时 resolve 应进入 stage=no_account, 终止 stage=%s detail=%q raw=%s",
			last.Stage, last.StageDetail, lastRaw)
	}
}