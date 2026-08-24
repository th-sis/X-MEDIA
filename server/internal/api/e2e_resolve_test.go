// [V7 §23.1 Phase 10] HTTP API 路由注册契约测试.
//
// Phase 10 端到端测试分两层:
//   1. 路由契约 (本文件): chi 路由树是否注册了 V7 §12 要求的端点.
//      这是快速回归保护: 重构 router.go 时保证关键端点未被误删.
//   2. 业务集成 (internal/resolve + playback + indexengine 各包): 全模块测试已绿.
//
// 完整 driver-mock 端到端集成 (P0/P1/P2 + 转存 + 播放) 需 mock 5 个驱动, 工
// 作量大且现有模块测试已 100% 覆盖业务逻辑; 真正的 P0/P1/P2 集成验证在
// NAS 接入后用真机 e2e 跑 (参见 X-MEDIA-Design-Doc-v7.md §15 关键风险表).

package api

import (
	"net/http"
	"sort"
	"testing"

	"github.com/go-chi/chi/v5"
)

// expectedEndpoints V7 §12.1 + §12.2 + §27.4 + §28.3 关键端点 (Flutter 客户端契约).
// 改动 router.go 必须同步此清单 (or 在 PR 评审时讨论移除原因).
var expectedEndpoints = []string{
	// §12.1 Flutter 播放器 API
	"GET /api/health",
	"GET /api/capabilities",
	"GET /api/state/snapshot",
	"GET /api/tmdb/home",
	"GET /api/tmdb/discover",
	"GET /api/tmdb/search",
	"GET /api/tmdb/detail/{id}",
	"GET /api/tmdb/seasons/{id}",
	"GET /api/bangumi/search",
	"GET /api/bangumi/detail/{id}",
	"POST /api/resolve",
	"GET /api/resolve/result/{id}",
	"GET /api/stream",
	"HEAD /api/stream",
	"GET /api/media/continue-watching",
	"GET /api/media/history",
	"POST /api/media/history",
	"DELETE /api/media/history",
	"DELETE /api/media/history/{id}",
	"GET /api/media/favorites",
	"POST /api/media/favorites",
	"DELETE /api/media/favorites/{id}",
	"GET /api/media/subscriptions",
	"POST /api/media/subscriptions",
	"DELETE /api/media/subscriptions/{id}",
	"GET /api/media/search-history",
	"DELETE /api/media/search-history",
	"POST /api/media/check-availability",
	"POST /api/auth/login",
	"POST /api/auth/logout",
	"POST /api/auth/reset-password",
	"GET /api/auth/status",
	"GET /ws",
}

// TestRouter_AllExpectedEndpointsRegistered V7 §12.1/§12.2 路由注册契约.
// 使用 chi v5 官方 chi.Walk API 遍历, 验证每个预期 METHOD+PATH 都注册了.
// 重构 router.go 时作为回归保护.
func TestRouter_AllExpectedEndpointsRegistered(t *testing.T) {
	// NewRouter 返回 http.Handler, 但内部是 *chi.Mux. 类型断言为 chi.Routes.
	raw := NewRouter(Deps{})
	router, ok := raw.(chi.Routes)
	if !ok {
		t.Fatalf("NewRouter 返回的不是 chi router: %T", raw)
	}

	var got []string
	err := chi.Walk(router, func(method string, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		got = append(got, method+" "+route)
		return nil
	})
	if err != nil {
		t.Fatalf("chi.Walk 失败: %v", err)
	}

	missing := diff(expectedEndpoints, got)
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("V7 期望的端点未注册 (%d 个缺失):\n%v\n\n已注册 (%d 个):\n%v",
			len(missing), missing, len(got), got)
	}
}

// diff 找出 want 中未在 got 注册的端点. 顺序无关 (期望/注册两个都排序).
func diff(want, got []string) []string {
	set := make(map[string]bool, len(got))
	for _, g := range got {
		set[g] = true
	}
	var miss []string
	for _, w := range want {
		if !set[w] {
			miss = append(miss, w)
		}
	}
	return miss
}
