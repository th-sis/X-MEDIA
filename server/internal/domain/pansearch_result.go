package domain

// PanSearchResult 盘搜结果。
type PanSearchResult struct {
	Title     string  `json:"title"`
	Source    string  `json:"source"`
	ShareURL  string  `json:"share_url"`
	Password  string  `json:"password"`
	MagnetURL string  `json:"magnet_url"`
	Datetime  string  `json:"datetime"`
	Quality   string  `json:"quality"`
	Format    string  `json:"format"`
	Score     float64 `json:"score"`
}
