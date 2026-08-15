package domain

import "time"

// PlayHistory 播放历史（季集维度）。
type PlayHistory struct {
	ID             int64     `json:"id"`
	ExternalID     int64     `json:"external_id"`
	ExternalSource string    `json:"external_source"`
	MediaType      string    `json:"media_type"`
	Title          string    `json:"title"`
	PosterURL      string    `json:"poster_url"`
	SourceType     string    `json:"source_type"`
	Season         int       `json:"season"`
	Episode        int       `json:"episode"`
	PositionMs     int64     `json:"position_ms"`
	DurationMs     int64     `json:"duration_ms"`
	PlayedAt       time.Time `json:"played_at"`
}
