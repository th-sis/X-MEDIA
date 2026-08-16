package domain

// Capabilities 能力预检结果。
type Capabilities struct {
	NASAvailable       bool     `json:"nas_available"`
	NASIndexComplete   bool     `json:"nas_index_complete"`
	NASIndexCount      int      `json:"nas_index_count"`
	PansearchAvailable bool     `json:"pansearch_available"`
	LoggedInDrivers    []string `json:"logged_in_drivers"`
	NASPhase           string   `json:"nas_phase"`
	NASProcessedFiles  int      `json:"nas_processed_files"`
	NASTotalFiles      int      `json:"nas_total_files"`
	// [v7 整改] P2 磁力下载是否启用
	MagnetEnabled bool `json:"magnet_enabled"`
	// [v7 整改] 演示兜底开关
	DemoFallback bool `json:"demo_fallback"`
	ServerVersion string `json:"server_version"`
}
