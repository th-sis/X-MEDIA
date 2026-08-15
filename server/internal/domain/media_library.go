package domain

import (
	"encoding/json"
	"time"
)

// MediaLibrary TMDB/Bangumi 元数据缓存。
type MediaLibrary struct {
	ID             int64           `json:"id"`
	ExternalID     int64           `json:"external_id"`
	ExternalSource string          `json:"external_source"`
	MediaType      string          `json:"media_type"`
	Title          string          `json:"title"`
	TitleOrig      string          `json:"title_orig"`
	PosterURL      string          `json:"poster_url"`
	BackdropURL    string          `json:"backdrop_url"`
	Overview       string          `json:"overview"`
	Year           int             `json:"year"`
	VoteAvg        float64         `json:"vote_avg"`
	VoteCount      int             `json:"vote_count"`
	Genres         json.RawMessage `json:"genres"`
	Runtime        int             `json:"runtime"`
	Seasons        int             `json:"seasons"`
	Episodes       int             `json:"episodes"`
	SeasonsJSON    json.RawMessage `json:"seasons_json"`
	Cast           json.RawMessage `json:"cast"`
	Extra          json.RawMessage `json:"extra"`
	CachedAt       time.Time       `json:"cached_at"`
	LastAccessedAt *time.Time      `json:"last_accessed_at"`
}

// Genre 轻量类型标签，用于 JSON 序列化。
type Genre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// CastMember 演职员。
type CastMember struct {
	Name       string `json:"name"`
	Character  string `json:"character"`
	ProfileURL string `json:"profile_url"`
}
