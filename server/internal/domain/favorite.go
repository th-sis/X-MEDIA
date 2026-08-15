package domain

import "time"

// Favorite 收藏条目。
type Favorite struct {
	ID             int64     `json:"id"`
	ExternalID     int64     `json:"external_id"`
	ExternalSource string    `json:"external_source"`
	MediaType      string    `json:"media_type"`
	Title          string    `json:"title"`
	PosterURL      string    `json:"poster_url"`
	Year           int       `json:"year"`
	CreatedAt      time.Time `json:"created_at"`
}
