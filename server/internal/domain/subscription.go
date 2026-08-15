package domain

import "time"

type SubStatus string

const (
	SubWatching   SubStatus = "watching"
	SubFound      SubStatus = "found"
	SubDownloaded SubStatus = "downloaded"
	SubFailed     SubStatus = "failed"
)

var subTransitions = map[SubStatus][]SubStatus{
	SubWatching:   {SubFound, SubFailed},
	SubFound:      {SubDownloaded, SubFailed, SubWatching},
	SubDownloaded: {},
	SubFailed:     {SubWatching},
}

// Subscription 订阅条目。
type Subscription struct {
	ID              int64      `json:"id"`
	ExternalID      int64      `json:"external_id"`
	ExternalSource  string     `json:"external_source"`
	MediaType       string     `json:"media_type"`
	Title           string     `json:"title"`
	Year            int        `json:"year"`
	PosterURL       string     `json:"poster_url"`
	Status          SubStatus  `json:"status"`
	AutoRuleID      int64      `json:"auto_rule_id"`
	LastSearchAt    *time.Time `json:"last_search_at"`
	SearchCount     int        `json:"search_count"`
	MaxSearches     int        `json:"max_searches"`
	ResultSource    string     `json:"result_source"`
	ResultAccountID int64      `json:"result_account_id"`
	ResultPath      string     `json:"result_path"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// CanTransitionTo 校验订阅状态机转换是否合法。
func (s *Subscription) CanTransitionTo(target SubStatus) bool {
	for _, t := range subTransitions[s.Status] {
		if t == target {
			return true
		}
	}
	return false
}
