package domain

import "time"

// SearchHistory 搜索历史条目。
type SearchHistory struct {
	ID         int64     `json:"id"`
	Keyword    string    `json:"keyword"`
	SearchedAt time.Time `json:"searched_at"`
}
