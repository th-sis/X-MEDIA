package api

import "testing"

// 验证 §6.9 配置键白名单：pan_save_root_*/pan_quota_warning_*/pan_cleanup_mode_*/pan_cleanup_keep_recent_days_*
// 应当被允许；其他 pan_* 随机键应当被拒绝。
func TestAllowedConfigKey_PrefixAccept(t *testing.T) {
	cases := []struct {
		key   string
		want  bool
	}{
		// §6.9 前缀键 → 应通过
		{"pan_save_root_1", true},
		{"pan_save_root_42", true},
		{"pan_quota_warning_pan115", true},
		{"pan_cleanup_mode_quark", true},
		{"pan_cleanup_keep_recent_days_baidu", true},
		// 已知白名单静态键
		{"tmdb_api_key", true},
		{"pan_rename_enabled", true},
		{"resolve_priority", true},
		// 越界键
		{"pan_unknown_foo", false},
		{"save_root_1", false},
		{"quota_warning_x", false},
		{"", false},
		{"pan_", false},
	}
	for _, c := range cases {
		if got := allowedConfigKey(c.key); got != c.want {
			t.Errorf("allowedConfigKey(%q)=%v want %v", c.key, got, c.want)
		}
	}
}