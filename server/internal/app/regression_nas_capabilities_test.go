// [实测回归] 生产环境数据形态复刻: enabled NAS source 指向不可达路径.
//
// 背景 (2026-08-24 真机实测 192.168.7.154): 存在 enabled source 时
// nasConfigured=true, Capabilities 走 nasPathsKnown() — 而 NewService
// 构造器漏赋该字段导致 nil deref panic, /api/capabilities 与
// /api/state/snapshot 双双 500, 客户端首页完全无法加载.
// 本用例锁定该形态: 4 个不可达 source 下两端点必须 200 且 NAS 三态为
// not_accessible (V7 §9.4/§27.4).

package app

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestRegression_CapabilitiesWithUnreachableNAS(t *testing.T) {
	env := newPhase10Env(t)

	for _, name := range []string{"Asia-Movie", "West-Movie", "Documentary", "X-RATED"} {
		if _, err := env.app.store.NASSources.Create(e_ctx(env), mustNASSource(name, "/mnt/BTORAGE-does-not-exist/"+name)); err != nil {
			t.Fatalf("create source %s: %v", name, err)
		}
	}

	respCaps, rawCaps := env.do(http.MethodGet, "/api/capabilities", nil, nil)
	if respCaps.StatusCode != http.StatusOK {
		t.Fatalf("capabilities 应 200, got %d body=%.300s", respCaps.StatusCode, rawCaps)
	}
	var caps struct {
		NASStatus       string `json:"nas_status"`
		NASTotalSources int    `json:"nas_total_sources"`
	}
	_ = json.Unmarshal(rawCaps, &caps)
	if caps.NASStatus != "not_accessible" {
		t.Fatalf("不可达路径应报 not_accessible, got %q", caps.NASStatus)
	}
	if caps.NASTotalSources != 4 {
		t.Fatalf("nas_total_sources 应为 4, got %d", caps.NASTotalSources)
	}

	respSnap, rawSnap := env.do(http.MethodGet, "/api/state/snapshot", nil, nil)
	if respSnap.StatusCode != http.StatusOK {
		t.Fatalf("snapshot 应 200, got %d body=%.300s", respSnap.StatusCode, rawSnap)
	}
}
