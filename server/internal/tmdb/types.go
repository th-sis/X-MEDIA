package tmdb

// MediaSummary 卡片级元数据（列表/榜单/搜索）。
type MediaSummary struct {
	ExternalID     int64    `json:"external_id"`
	ExternalSource string   `json:"external_source"`
	MediaType      string   `json:"media_type"`
	Title          string   `json:"title"`
	TitleOrig      string   `json:"title_orig"`
	Year           int      `json:"year"`
	VoteAvg        float64  `json:"vote_avg"`
	PosterURL      string   `json:"poster_url"`
	BackdropURL    string   `json:"backdrop_url"`
	Overview       string   `json:"overview"`
	Genres         []string `json:"genres"`
}

// MediaDetail 详情页元数据。
type MediaDetail struct {
	MediaSummary
	Runtime     int          `json:"runtime"`
	Seasons     int          `json:"seasons"`
	Episodes    int          `json:"episodes"`
	SeasonsList []SeasonInfo `json:"seasons_list"`
	Cast        []CastMember `json:"cast"`
}

// SeasonInfo 季信息。
type SeasonInfo struct {
	SeasonNumber int           `json:"season_number"`
	Name         string        `json:"name"`
	EpisodeCount int           `json:"episode_count"`
	AirDate      string        `json:"air_date,omitempty"`
	Overview     string        `json:"overview,omitempty"`
	PosterURL    string        `json:"poster_url,omitempty"`
	Episodes     []EpisodeInfo `json:"episodes"`
}

// EpisodeInfo 集信息（含可用性角标）。
type EpisodeInfo struct {
	EpisodeNumber int    `json:"episode_number"`
	Name          string `json:"name"`
	Available     bool   `json:"available"`
}

// CastMember 演职员。
type CastMember struct {
	Name       string `json:"name"`
	Character  string `json:"character"`
	ProfileURL string `json:"profile_url"`
}

// Section 榜单行。
type Section struct {
	Key   string         `json:"key"`
	Title string         `json:"title"`
	Items []MediaSummary `json:"items"`
}

// ListResponse 列表响应（§18.1 契约）。
type ListResponse struct {
	Items   []MediaSummary `json:"items"`
	Page    int            `json:"page"`
	HasMore bool           `json:"has_more"`
	Total   int            `json:"total"`
}
