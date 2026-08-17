package domain

// Capabilities 能力预检结果。
// [V7 §9.4 + §27.4] nas_status 三态：ok / not_configured / not_accessible
type Capabilities struct {
	NASAvailable       bool     `json:"nas_available"` // 是否可访问（路径存在 + 启用）
	NASStatus          string   `json:"nas_status"`    // V7 §27.4 三态
	NASIndexComplete   bool     `json:"nas_index_complete"`
	NASIndexCount      int      `json:"nas_index_count"`
	PansearchAvailable bool     `json:"pansearch_available"`
	LoggedInDrivers    []string `json:"logged_in_drivers"`
	NASPhase           string   `json:"nas_phase"`
	NASProcessedFiles  int      `json:"nas_processed_files"`
	NASTotalFiles      int      `json:"nas_total_files"`
	NASScanning        bool     `json:"nas_scanning"`
	// [V7 §9.4+ 扩展 G1.E] NAS source 总数：NASTotalSources / NASEnabledSources
	NASTotalSources   int `json:"nas_total_sources"`
	NASEnabledSources int `json:"nas_enabled_sources"`
	// [v7 整改] P2 磁力下载是否启用
	MagnetEnabled bool    `json:"magnet_enabled"`
	P0MinScore    float64 `json:"p0_min_score"`
	// [v7 整改] 演示兜底开关
	DemoFallback  bool   `json:"demo_fallback"`
	ServerVersion string `json:"server_version"`
}
