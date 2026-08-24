// [V7 §8 + §6.4 实测回归] PanSou 上游真契约 (2026-08-24 抓取) 与 V7 §8.2 文档不符:
// 实际为 {"code":0,"message":"success","data":{"total":n,"merged_by_type":{...}}}.
// 本用例复刻真上游契约, 验证解析层修复后 resolve 至少能进 pan_searched 阶段.

package app

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"xmedia/internal/domain"
)

// fakeRealPanSou 起一个真 PanSou 形态: 含 code/message/data 外壳, 仅 /api/search+/api/health.
func fakeRealPanSou(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/health":
			_, _ = io.WriteString(w, `{"status":"ok","auth_enabled":false}`)
		case "/api/search":
			var req struct {
				KW string `json:"kw"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			link := "https://pan.quark.cn/s/" + strings.ReplaceAll(req.KW, " ", "-")
			resp := map[string]any{
				"code":    0,
				"message": "success",
				"data": map[string]any{
					"total": 1,
					"merged_by_type": map[string][]map[string]any{
						"quark": {{
							"url":      link,
							"password": "",
							"note":     req.KW + " 4K HDR",
							"datetime": time.Now().UTC().Format(time.RFC3339),
							"source":   "tg:fake_channel",
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

func TestE2E_Phase10_PanSearchRealContractSurfacesInResolve(t *testing.T) {
	env := newPhase10Env(t)
	pansou := fakeRealPanSou(t)
	env.setConfig(domain.ConfigPansearchURL, pansou.URL)
	// 预置一个 active 账号让 P1/P2 不被 activeAccounts=0 跳过.
	accID, err := env.app.store.Accounts.Create(e_ctx(env), &domain.Account{
		Name: "e2e-quark", DriverType: "quark", IsActive: true, IsDefault: true,
		SortOrder: 1,
	})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	now := time.Now().UTC()
	if err := env.app.store.AuthStates.Upsert(e_ctx(env), &domain.AuthState{
		AccountID: accID, Status: domain.AuthActive,
		TokenExpires: now.Add(24 * time.Hour),
	}); err != nil {
		t.Fatalf("upsert auth: %v", err)
	}

	// 触发 resolve (P0 会因 NAS 索引为空跳过, 接着 P1 应能命中 PanSou 资源).
	resp, raw := env.do(http.MethodPost, "/api/resolve", map[string]any{
		"external_id": int64(27205), "external_source": "tmdb", "media_type": "movie",
		"title": "Inception Test", "year": 2010,
	}, nil)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		t.Fatalf("resolve status=%d body=%s", resp.StatusCode, raw)
	}
	var r struct {
		TaskID int64 `json:"task_id"`
	}
	_ = json.Unmarshal(raw, &r)

	// 轮询 result, 抓 pan_searched 阶段的证据 (修复前永远到不了).
	deadline := time.Now().Add(15 * time.Second)
	hitPan := false
	var lastRaw []byte
	for time.Now().Before(deadline) {
		resp, raw = env.do(http.MethodGet,
			fmt.Sprintf("/api/resolve/result/%d", r.TaskID), nil, nil)
		lastRaw = raw
		var rr struct {
			Status string `json:"status"`
			Stage  string `json:"stage"`
		}
		_ = json.Unmarshal(raw, &rr)
		if rr.Stage == "pan_searched" || rr.Stage == "transferring" || rr.Stage == "resolving_link" {
			hitPan = true
		}
		if rr.Status == "done" || rr.Status == "failed" {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}

	// 硬契约: 至少应进入 pan_searching 阶段 (pan_searched 之前的那个).
	// 即便后续转存阶段失败 (无真实驱动), 也证明 PanSou 上游契约解析已修复.
	if !hitPan {
		// 兜底再解析一次 lastRaw 看 stage 字段.
		var last struct {
			Stage       string `json:"stage"`
			StageDetail string `json:"stage_detail"`
		}
		_ = json.Unmarshal(lastRaw, &last)
		t.Fatalf("解析修复后仍未能进入 pan_searching/pan_searched 阶段, 终止 stage=%s detail=%q raw=%s",
			last.Stage, last.StageDetail, lastRaw)
	}
}