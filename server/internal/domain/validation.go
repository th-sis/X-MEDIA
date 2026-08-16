package domain

// ValidationResult 启动配置验证结果（[v7 整改] §11.1）。
type ValidationResult struct {
	OK            bool              `json:"ok"`
	CheckedAt     string            `json:"checked_at"`
	TMDBKey       ValidationCheck   `json:"tmdb_key"`
	PanSouURL     ValidationCheck   `json:"pansearch_url"`
	HasAnyAccount ValidationCheck   `json:"has_any_account"`
	MagnetEnabled ValidationCheck   `json:"magnet_enabled"`
	Issues        []string          `json:"issues,omitempty"`
	Warnings      map[string]string `json:"warnings,omitempty"`
}

// ValidationCheck 单项检查结果。
type ValidationCheck struct {
	Status  string `json:"status"`            // "ok" / "warning" / "error"
	Message string `json:"message,omitempty"` // 人类可读说明
}
