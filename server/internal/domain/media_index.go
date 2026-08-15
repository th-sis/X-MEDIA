package domain

import "time"

type MatchStatus string

const (
	MatchMatched     MatchStatus = "matched"
	MatchUnconfirmed MatchStatus = "unconfirmed"
	MatchOrphaned    MatchStatus = "orphaned"
)

// MediaIndex 文件索引条目（NAS / 网盘）。
type MediaIndex struct {
	ID             int64       `json:"id"`
	ExternalID     int64       `json:"external_id"`
	ExternalSource string      `json:"external_source"`
	Season         int         `json:"season"`
	Episode        int         `json:"episode"`
	MediaType      string      `json:"media_type"`
	Title          string      `json:"title"`
	OriginalName   string      `json:"original_name"`
	Year           int         `json:"year"`
	SourceType     string      `json:"source_type"`
	AccountID      int64       `json:"account_id"`
	FilePath       string      `json:"file_path"`
	FileID         string      `json:"file_id"`
	FileSize       int64       `json:"file_size"`
	FileFormat     string      `json:"file_format"`
	MatchStatus    MatchStatus `json:"match_status"`
	MatchScore     float64     `json:"match_score"`
	StreamURL      string      `json:"stream_url,omitempty"`
	URLExpires     *time.Time  `json:"url_expires,omitempty"`
	LastPlayedAt   *time.Time  `json:"last_played_at,omitempty"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
}
