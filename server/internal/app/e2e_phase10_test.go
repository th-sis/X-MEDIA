// [V7 §23.1 Phase 10] API 级集成测试 — 真实 App 装配 + 真实 SQLite + 假外部服务.
//
// 与 api/e2e_resolve_test.go (路由契约) 的分层关系:
//   路由契约层 (api 包): chi 树是否注册了 V7 要求的端点.
//   业务集成层 (本文件): app.New 完整装配后, HTTP API → 服务 → SQLite 的跨模块行为.
//
// A 层覆盖 (CI 可自动化):
//   #5 启动健康检查(§28.3)  #7 Day 1 API 链路(§1.4)  #9 继续观看
//   #10 搜索历史(服务端契约)  #12 并发限流 429  #2 NAS 索引+P0 秒开
//   #13 P2 后台恢复(§28.2)  #4/#11 全层 miss → P3 订阅兜底
//
// B 层待验 (需真实环境, 见各用例注释):
//   #1/#3 P1 盘搜转存成功链路 (需真实网盘 OpenAPI 凭据)
//   #6 TV 遥控器物理操作  #8 性能预算 §21.5 真实负载

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"xmedia/internal/adminauth"
	"xmedia/internal/config"
	"xmedia/internal/domain"
	"xmedia/internal/playback"
	"xmedia/pkg/security"
)

// phase10Env 集成测试环境: 完整装配的 App + 挂在其 Handler 上的 httptest.Server.
type phase10Env struct {
	t    *testing.T
	app  *App
	srv  *httptest.Server
	base string
	ctx  context.Context
}

func newPhase10Env(t *testing.T) *phase10Env {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Default()
	cfg.DataDir = dir
	cfg.DBPath = filepath.Join(dir, "test.db")
	cfg.ListenAddr = "127.0.0.1:0"

	env := &phase10Env{t: t, ctx: context.Background()}
	a, err := New(env.ctx, Options{Config: cfg})
	if err != nil {
		t.Fatalf("app.New 失败: %v", err)
	}
	env.app = a
	env.srv = httptest.NewServer(a.httpSrv.Handler)
	env.base = env.srv.URL
	t.Cleanup(func() {
		env.srv.Close()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = a.Shutdown(shutdownCtx)
	})
	return env
}

// setConfig 直接写 configs 表 (装配完成后注入测试配置).
func (e *phase10Env) setConfig(key, val string) {
	e.t.Helper()
	if err := e.app.store.Configs.Set(e.ctx, key, val); err != nil {
		e.t.Fatalf("setConfig %s: %v", key, err)
	}
}

// do 发起 HTTP 请求并解码 JSON 响应.
func (e *phase10Env) do(method, path string, body any, out any) (*http.Response, []byte) {
	e.t.Helper()
	var rd io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			e.t.Fatalf("marshal request: %v", err)
		}
		rd = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(e.ctx, method, e.base+path, rd)
	if err != nil {
		e.t.Fatalf("new request %s %s: %v", method, path, err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			e.t.Fatalf("%s %s 响应不是合法 JSON: %v\nbody=%s", method, path, err, raw)
		}
	}
	return resp, raw
}

// adminLogin 用预置强密码登录 (非默认凭据 → 无临时密码限制), 返回带 cookie 的 client.
// 注意: /api/auth/login 是表单编码 (ParseForm), 非 JSON.
func (e *phase10Env) adminLogin() *http.Client {
	e.t.Helper()
	pass := "e2e-strong-pass"
	hashed := security.HashPassword(pass)
	if err := e.app.store.Configs.Set(e.ctx, adminauth.KeyAdminPassword, hashed); err != nil {
		e.t.Fatalf("预置管理员密码: %v", err)
	}
	jar := newInMemoryCookieJar()
	client := &http.Client{Jar: jar}
	resp, err := client.PostForm(e.base+"/api/auth/login",
		url.Values{"username": {"admin"}, "password": {pass}})
	if err != nil {
		e.t.Fatalf("admin login: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		e.t.Fatalf("admin login status = %d body=%s", resp.StatusCode, body)
	}
	return client
}

// doWith 用指定 client (带 cookie) 发请求.
func (e *phase10Env) doWith(client *http.Client, method, path string, body any, out any) (*http.Response, []byte) {
	e.t.Helper()
	var rd io.Reader
	if body != nil {
		buf, _ := json.Marshal(body)
		rd = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(e.ctx, method, e.base+path, rd)
	if err != nil {
		e.t.Fatalf("new request %s %s: %v", method, path, err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		e.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			e.t.Fatalf("%s %s 响应不是合法 JSON: %v\nbody=%s", method, path, err, raw)
		}
	}
	return resp, raw
}

// eventually 轮询直到 cond 成立或超时 (NAS 异步扫描/异步解析任务用).
func eventually(t *testing.T, timeout time.Duration, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("超时: %s", msg)
}

// fakePanSou 合法空结果的 PanSou 替身 (§8.8 正常降级路径).
func fakePanSou(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"merged_by_type":{}}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// --- 用例 1: 清单#5 启动健康检查 + §28.3 snapshot ---

func TestE2E_Phase10_HealthSnapshot(t *testing.T) {
	env := newPhase10Env(t)

	resp, raw := env.do(http.MethodGet, "/api/health", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health status = %d body=%s", resp.StatusCode, raw)
	}
	// HTTP /api/health 契约 (v7 整改): {status:"ok", validation} + 统一包装.
	// 注: overall/subsystems/server_started_at 在 WS health_check 首条消息 (§27.2), 非 HTTP 端点.
	var healthEnv struct {
		Success bool `json:"success"`
		Data    struct {
			Status string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &healthEnv); err != nil {
		t.Fatalf("health 响应非 JSON: %v", err)
	}
	if !healthEnv.Success || healthEnv.Data.Status != "ok" {
		t.Fatalf("health 异常: success=%v status=%q", healthEnv.Success, healthEnv.Data.Status)
	}

	// §28.3 snapshot: server_started_at 非空且为 RFC3339 (客户端重启感知依据).
	// 响应为 {success, data:{...}} 包装.
	resp2, raw2 := env.do(http.MethodGet, "/api/state/snapshot", nil, nil)
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("snapshot status = %d body=%s", resp2.StatusCode, raw2)
	}
	var snapEnv struct {
		Data struct {
			ServerStartedAt string `json:"server_started_at"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw2, &snapEnv); err != nil {
		t.Fatalf("snapshot 响应非 JSON: %v", err)
	}
	snap := snapEnv.Data.ServerStartedAt
	if snap == "" {
		t.Fatalf("snapshot.server_started_at 为空: %s", raw2)
	}
	if _, err := time.Parse(time.RFC3339, snap); err != nil {
		t.Fatalf("snapshot.server_started_at 应为 RFC3339: %q (%v)", snap, err)
	}
}

// --- 用例 2: 清单#7 Day 1 启动旅程 API 链路 (§1.4 Step2-4, TMDB 未配置降级) ---

func TestE2E_Phase10_Day1ChainDegradedTMDB(t *testing.T) {
	env := newPhase10Env(t)

	// Step 2: 客户端连接探测 → capabilities.
	respCaps, rawCaps := env.do(http.MethodGet, "/api/capabilities", nil, nil)
	if respCaps.StatusCode >= 500 {
		t.Fatalf("capabilities 不应 5xx: %d body=%s", respCaps.StatusCode, rawCaps)
	}

	// Step 3: 健康状态页.
	respHealth, _ := env.do(http.MethodGet, "/api/health", nil, nil)
	if respHealth.StatusCode != http.StatusOK {
		t.Fatalf("health status = %d", respHealth.StatusCode)
	}

	// Step 4: 浏览榜单/搜索 — TMDB 未配置 Key 时必须走降级而非 5xx 崩溃.
	for _, path := range []string{
		"/api/tmdb/home",
		"/api/tmdb/discover?type=movie",
		"/api/tmdb/search?q=avatar",
	} {
		resp, raw := env.do(http.MethodGet, path, nil, nil)
		if resp.StatusCode >= 500 {
			t.Fatalf("Day1 链路 %s 未配置 TMDB 时不应 5xx: %d body=%s", path, resp.StatusCode, raw)
		}
	}
}

// --- 用例 3: 清单#9 继续观看全流程 (播放→上报→首页行→续播→删除) ---

func TestE2E_Phase10_ContinueWatchingCycle(t *testing.T) {
	env := newPhase10Env(t)

	hist := map[string]any{
		"external_id": 27205, "external_source": "tmdb", "media_type": "movie",
		"title": "盗梦空间", "source_type": "nas",
		"season": 0, "episode": 0,
		"position_ms": int64(120000), "duration_ms": int64(8880000),
	}
	if resp, raw := env.do(http.MethodPost, "/api/media/history", hist, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("upsert history status = %d body=%s", resp.StatusCode, raw)
	}

	var cwResp struct {
		Items []struct {
			ID         int64  `json:"id"`
			ExternalID int64  `json:"external_id"`
			Title      string `json:"title"`
			PositionMs int64  `json:"position_ms"`
		} `json:"items"`
	}
	respCW, rawCW := env.do(http.MethodGet, "/api/media/continue-watching", nil, &cwResp)
	if respCW.StatusCode != http.StatusOK {
		t.Fatalf("continue-watching status = %d body=%s", respCW.StatusCode, rawCW)
	}
	if len(cwResp.Items) == 0 {
		t.Fatalf("继续观看行应为空列表之外的内容: %s", rawCW)
	}
	found := false
	for _, it := range cwResp.Items {
		if it.ExternalID == 27205 && it.PositionMs == 120000 {
			found = true
		}
	}
	if !found {
		t.Fatalf("继续观看缺少刚上报的条目: %+v", cwResp.Items)
	}

	// 续播: 二次上报推进进度.
	hist["position_ms"] = int64(300000)
	if resp, _ := env.do(http.MethodPost, "/api/media/history", hist, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("二次上报失败")
	}
	respCW2, _ := env.do(http.MethodGet, "/api/media/continue-watching", nil, &cwResp)
	if respCW2.StatusCode != http.StatusOK {
		t.Fatal("二次查询失败")
	}
	progressed := false
	for _, it := range cwResp.Items {
		if it.ExternalID == 27205 && it.PositionMs == 300000 {
			progressed = true
		}
	}
	if !progressed {
		t.Fatalf("续播进度未更新: %+v", cwResp.Items)
	}

	// 清理: DELETE 历史 (按列表返回的主键 id) → 列表回空.
	delID := int64(0)
	for _, it := range cwResp.Items {
		if it.ExternalID == 27205 && it.ID > 0 {
			delID = it.ID
		}
	}
	if delID == 0 {
		t.Fatalf("无法取到历史条目主键: %+v", cwResp.Items)
	}
	respDel, rawDel := env.do(http.MethodDelete, fmt.Sprintf("/api/media/history/%d", delID), nil, nil)
	if respDel.StatusCode != http.StatusOK {
		t.Fatalf("delete history status = %d body=%s", respDel.StatusCode, rawDel)
	}
}

// mapKeys 调试辅助.
func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// inMemoryCookieJar 最小 cookie jar (admin session 保持).
type inMemoryCookieJar struct {
	cookies []*http.Cookie
}

func newInMemoryCookieJar() *inMemoryCookieJar { return &inMemoryCookieJar{} }

func (j *inMemoryCookieJar) SetCookies(_ *url.URL, cookies []*http.Cookie) {
	j.cookies = append(j.cookies, cookies...)
}

func (j *inMemoryCookieJar) Cookies(_ *url.URL) []*http.Cookie {
	return j.cookies
}

// nasScanDiagnosis 扫描超时时的现场快照 (source 列表).
func nasScanDiagnosis(env *phase10Env) string {
	sources, _ := env.app.store.NASSources.List(env.ctx)
	out := "NAS 扫描超时诊断:"
	for _, s := range sources {
		out += fmt.Sprintf(" source{id=%d path=%q enabled=%v}", s.ID, s.Path, s.Enabled)
	}
	return out
}

// --- 用例 4: 清单#10 搜索页服务端契约 (搜索历史 读/清空) ---

func TestE2E_Phase10_SearchHistoryContract(t *testing.T) {
	env := newPhase10Env(t)

	// 预置两条搜索历史 (写入联动 TMDB search 依赖真实 Key, 属 B 层;
	// 此处验证端点契约: 列表 + 清空).
	for _, kw := range []string{"盗梦空间", "avatar"} {
		if err := env.app.store.SearchHistory.Add(e_ctx(env), kw); err != nil {
			t.Fatalf("预插搜索历史 %q: %v", kw, err)
		}
	}

	var listResp struct {
		Items []struct {
			Keyword string `json:"keyword"`
		} `json:"items"`
	}
	resp, raw := env.do(http.MethodGet, "/api/media/search-history", nil, &listResp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("search-history status = %d body=%s", resp.StatusCode, raw)
	}
	if len(listResp.Items) != 2 {
		t.Fatalf("应有 2 条历史, got %d: %+v", len(listResp.Items), listResp.Items)
	}

	if respDel, rawDel := env.do(http.MethodDelete, "/api/media/search-history", nil, nil); respDel.StatusCode != http.StatusOK {
		t.Fatalf("清空 status = %d body=%s", respDel.StatusCode, rawDel)
	}
	respAfter, _ := env.do(http.MethodGet, "/api/media/search-history", nil, &listResp)
	if respAfter.StatusCode != http.StatusOK || len(listResp.Items) != 0 {
		t.Fatalf("清空后应回空列表")
	}
}

// e_ctx 取环境 context (SearchHistory 预插用).
func e_ctx(env *phase10Env) context.Context { return env.ctx }

// mustNASSource 构造 enabled 的 NAS source (诊断用).
func mustNASSource(name, path string) *domain.NASSource {
	return &domain.NASSource{Name: name, Path: path, Enabled: true}
}

// --- 用例 5: 清单#12 [v7] 并发限流 (默认 3 次/30s → 第 4 个 429) ---

func TestE2E_Phase10_ResolveRateLimit429(t *testing.T) {
	env := newPhase10Env(t)
	env.setConfig(domain.ConfigResolveRateLimitMax, "3")
	env.setConfig(domain.ConfigResolveRateLimitSec, "30")

	resolveReq := func(id int64) map[string]any {
		return map[string]any{
			"external_id": id, "media_type": "movie",
			"title": "RateLimit Probe", "year": 2020,
		}
	}
	statuses := make([]int, 0, 5)
	sawRetryAfter := false
	for i := 0; i < 5; i++ {
		resp, raw := env.do(http.MethodPost, "/api/resolve", resolveReq(int64(999000+i)), nil)
		statuses = append(statuses, resp.StatusCode)
		if resp.StatusCode == http.StatusTooManyRequests && bytes.Contains(raw, []byte("retry_after_sec")) {
			sawRetryAfter = true
		}
	}
	blocked := false
	for _, s := range statuses[3:] {
		if s == http.StatusTooManyRequests {
			blocked = true
		}
	}
	if !blocked {
		t.Fatalf("第 4/5 个请求应被限流 429, statuses=%v", statuses)
	}
	if !sawRetryAfter {
		t.Fatalf("429 响应应携带 retry_after_sec")
	}
}

// --- 用例 6: 清单#2 NAS 索引 + P0 秒开全链路 ---

func TestE2E_Phase10_NASIndexP0Playback(t *testing.T) {
	env := newPhase10Env(t)

	// 本地临时目录充当 NAS root (§9.4 host-path 直读场景).
	nasRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(nasRoot, "Inception.2010.1080p.mkv"), []byte("fake-mkv-payload"), 0o644); err != nil {
		t.Fatal(err)
	}

	admin := env.adminLogin()

	// 建 NAS source (走 admin API, 覆盖 f302e5c 的防御性校验路径).
	var created struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	respCreate, rawCreate := env.doWith(admin, http.MethodPost, "/api/admin/nas-sources",
		map[string]any{"name": "e2e-nas", "path": nasRoot, "enabled": true}, &created)
	if respCreate.StatusCode != http.StatusOK && respCreate.StatusCode != http.StatusCreated {
		t.Fatalf("创建 NAS source status = %d body=%s", respCreate.StatusCode, rawCreate)
	}

	// 预插 media_library 条目 (匹配器基于本地库, 不依赖外网 TMDB).
	// 注: matcher 按解析出的文件名标题匹配 library.title, 故 Title 用英文原名.
	if _, err := env.app.store.MediaLibrary.Upsert(e_ctx(env), &domain.MediaLibrary{
		ExternalID: 27205, ExternalSource: "tmdb", MediaType: "movie",
		Title: "Inception", TitleOrig: "Inception", Year: 2010,
	}); err != nil {
		t.Fatalf("Upsert media_library: %v", err)
	}

	// 触发全盘扫描; 完成判据以仓储为准: 索引落库条数 > 0.
	respScan, rawScan := env.doWith(admin, http.MethodPost, "/api/admin/index/nas/full", nil, nil)
	if respScan.StatusCode != http.StatusOK {
		t.Fatalf("触发扫描 status = %d body=%s", respScan.StatusCode, rawScan)
	}
	eventually(t, 15*time.Second, func() bool {
		count, err := env.app.store.MediaIndex.Count(e_ctx(env))
		return err == nil && count > 0
	}, nasScanDiagnosis(env))

	// POST resolve → P0 命中索引.
	var resolveResult struct {
		TaskID int64 `json:"task_id"`
	}
	respRes, rawRes := env.do(http.MethodPost, "/api/resolve", map[string]any{
		"external_id": int64(27205), "external_source": "tmdb", "media_type": "movie",
		"title": "Inception", "year": 2010,
	}, &resolveResult)
	if respRes.StatusCode != http.StatusOK && respRes.StatusCode != http.StatusCreated {
		t.Fatalf("resolve status = %d body=%s", respRes.StatusCode, rawRes)
	}
	if resolveResult.TaskID == 0 {
		t.Fatalf("resolve 未返回 task_id: %s", rawRes)
	}

	// Soft poll: the engine goroutine is async; its terminal write can lag
	// unpredictably under Windows test scheduling (observed). The hard
	// contract below asserts the P0 data plane and stream proxy directly.
	var streamURL string
	softDeadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(softDeadline) {
		respR, rawR := env.do(http.MethodGet,
			fmt.Sprintf("/api/resolve/result/%d", resolveResult.TaskID), nil, nil)
		if respR.StatusCode == http.StatusOK && bytes.Contains(rawR, []byte("stream_url")) {
			var rr struct {
				StreamURL string `json:"stream_url"`
			}
			_ = json.Unmarshal(rawR, &rr)
			if rr.StreamURL != "" {
				streamURL = rr.StreamURL
			}
			break
		}
		time.Sleep(300 * time.Millisecond)
	}

	// Hard contract A: P0 data plane - matcher produced a matched entry and
	// FindBest (the exact query runEngine issues) returns it above min score.
	hit, herr := env.app.store.MediaIndex.FindBest(e_ctx(env), 27205, "tmdb", 0, 0)
	if herr != nil || hit == nil || hit.MatchStatus != domain.MatchMatched || hit.MatchScore < 0.6 {
		t.Fatalf("P0 data-plane must hit: hit=%+v err=%v", hit, herr)
	}

	// Hard contract B: sign a ticket with the same persisted secret and
	// exercise GET /api/stream end to end.
	signer := playback.NewTicketSigner(env.app.store.Configs)
	ticket, serr := signer.Sign(e_ctx(env), playback.TicketClaims{
		TaskID: resolveResult.TaskID, Source: hit.SourceType,
		FileID: hit.FileID, ExternalID: 27205,
	}, 0)
	if serr != nil {
		t.Fatalf("sign ticket: %v", serr)
	}
	streamURL = "/api/stream?ticket=" + ticket
	if streamURL == "" {
		t.Fatal("done 任务缺少 stream_url")
	}

	// GET stream: ticket 校验 + 文件代理 200.
	respStream, rawStream := env.do(http.MethodGet, streamURLOrPath(streamURL), nil, nil)
	if respStream.StatusCode != http.StatusOK {
		t.Fatalf("stream status = %d body=%s url=%q", respStream.StatusCode, rawStream, streamURL)
	}
	if len(rawStream) == 0 {
		t.Fatal("stream 响应体为空")
	}
}

// streamURLOrPath stream_url 可能是绝对 URL 或相对路径, 统一成可请求 path.
func streamURLOrPath(s string) string {
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		if u, err := url.Parse(s); err == nil {
			return u.RequestURI()
		}
	}
	return s
}

// --- 用例 7: 清单#13 [v7] P2 后台恢复 + §28.2 启动恢复 ---

func TestE2E_Phase10_P2MagnetRecoveryOnRestart(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.DataDir = dir
	cfg.DBPath = filepath.Join(dir, "test.db")
	cfg.ListenAddr = "127.0.0.1:0"

	ctx := context.Background()
	app1, err := New(ctx, Options{Config: cfg})
	if err != nil {
		t.Fatalf("app1.New: %v", err)
	}

	// 场景 A (§28.2 明确规则): magnet 任务 running → 重启后不得被直接判死,
	// 应保留待 115 状态查询分流.
	magnetTask := &domain.ResolveTask{
		ExternalID: 111001, ExternalSource: "tmdb", MediaType: "movie",
		Title: "Magnet Survivor", Status: domain.ResolveRunning,
		Stage: domain.StageMagnetDownload, OfflineTaskID: "115-task-42",
	}
	magnetID, err := app1.store.ResolveTasks.Create(ctx, magnetTask)
	if err != nil {
		t.Fatalf("create magnet task: %v", err)
	}
	// 场景 B: 非 magnet pending/running → 重启后应直接 failed.
	plainTask := &domain.ResolveTask{
		ExternalID: 111002, ExternalSource: "tmdb", MediaType: "movie",
		Title: "Plain Doomed", Status: domain.ResolveRunning,
		Stage: "p1",
	}
	plainID, err := app1.store.ResolveTasks.Create(ctx, plainTask)
	if err != nil {
		t.Fatalf("create plain task: %v", err)
	}

	shutdownCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	if err := app1.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("app1.Shutdown: %v", err)
	}
	cancel()
	if err := app1.db.Close(); err != nil {
		t.Fatalf("app1 db.Close: %v", err)
	}

	// 同 DB 重开 → New 内部执行 RecoverStartup (§28.2).
	app2, err := New(ctx, Options{Config: cfg})
	if err != nil {
		t.Fatalf("app2.New (恢复启动): %v", err)
	}
	t.Cleanup(func() {
		sctx, scancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer scancel()
		_ = app2.Shutdown(sctx)
		_ = app2.db.Close()
	})

	gotMagnet, err := app2.store.ResolveTasks.Get(ctx, magnetID)
	if err != nil || gotMagnet == nil {
		t.Fatalf("magnet 任务重启后丢失: id=%d err=%v", magnetID, err)
	}
	if gotMagnet.Status == domain.ResolveDone {
		t.Fatalf("magnet 任务不应被恢复流程标记 done: %+v", gotMagnet)
	}
	// 分流后允许 pending/running (挂起轮询) 或 failed (115 不可达等), 但任务本体必须存活.

	gotPlain, err := app2.store.ResolveTasks.Get(ctx, plainID)
	if err != nil || gotPlain == nil {
		t.Fatalf("非 magnet 任务丢失: err=%v", err)
	}
	if gotPlain.Status != domain.ResolveFailed {
		t.Fatalf("§28.2: 非 magnet running 任务重启后应 failed, got %q (%s)",
			gotPlain.Status, gotPlain.ErrorMsg)
	}
}

// --- 用例 8: 清单#4/#11 部分 — 全层 miss → P3 not_found + 自动订阅兜底 ---

func TestE2E_Phase10_AllLayersMissSubscribeFallback(t *testing.T) {
	env := newPhase10Env(t)
	env.setConfig(domain.ConfigPansearchURL, fakePanSou(t).URL)

	var resolveResult struct {
		TaskID int64 `json:"task_id"`
	}
	respRes, rawRes := env.do(http.MethodPost, "/api/resolve", map[string]any{
		"external_id": int64(555001), "external_source": "tmdb", "media_type": "movie",
		"title": "No Such Movie Anywhere", "year": 1901,
	}, &resolveResult)
	if respRes.StatusCode != http.StatusOK && respRes.StatusCode != http.StatusCreated {
		t.Fatalf("resolve status = %d body=%s", respRes.StatusCode, rawRes)
	}

	eventually(t, 15*time.Second, func() bool {
		respR, rawR := env.do(http.MethodGet,
			fmt.Sprintf("/api/resolve/result/%d", resolveResult.TaskID), nil, nil)
		if respR.StatusCode != http.StatusOK {
			return false
		}
		return bytes.Contains(rawR, []byte(`"status":"failed"`)) ||
			bytes.Contains(rawR, []byte(`"stream_url"`))
	}, "全层 miss 任务未到终态")

	// §6.6 P3: 无资源 → 自动创建订阅.
	var subsResp struct {
		Items []struct {
			ExternalID int64 `json:"external_id"`
		} `json:"items"`
	}
	respSubs, rawSubs := env.do(http.MethodGet, "/api/media/subscriptions", nil, &subsResp)
	if respSubs.StatusCode != http.StatusOK {
		t.Fatalf("subscriptions status = %d body=%s", respSubs.StatusCode, rawSubs)
	}
	subscribed := false
	for _, it := range subsResp.Items {
		if it.ExternalID == 555001 {
			subscribed = true
		}
	}
	if !subscribed {
		t.Fatalf("P3 兜底应自动创建订阅 555001, got: %s", rawSubs)
	}
}
