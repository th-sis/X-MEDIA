package tmdb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"xmedia/internal/domain"
)

// genreIDMap 中文分类名 -> TMDB genre ID（Discover with_genres 参数）。
var genreIDMap = map[string]string{
	"动作": "28", "冒险": "12", "动画": "16", "喜剧": "35",
	"犯罪": "80", "纪录": "99", "剧情": "18", "家庭": "10751",
	"奇幻": "14", "历史": "36", "恐怖": "27", "音乐": "10402",
	"悬疑": "9648", "爱情": "10749", "科幻": "878", "战争": "10752",
	"西部": "37", "惊悚": "53",
}

// liveRequest 发起带 api_key 的 TMDB 请求并解码 JSON。
func (s *Service) liveRequest(ctx context.Context, path string, query url.Values, out any) error {
	key := s.apiKey(ctx)
	if key == "" {
		return domain.Errorf(domain.CodeAuthExpired, "TMDB API Key 未配置")
	}
	rawURL := s.base + path
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	rawURL += sep + "api_key=" + url.QueryEscape(key) + "&language=zh-CN"
	if len(query) > 0 {
		rawURL += "&" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return domain.Errorf(domain.CodeAuthExpired, "TMDB API Key 无效（HTTP %d）", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("tmdb status %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// searchLive 真实搜索（/search/multi）。
func (s *Service) searchLive(ctx context.Context, q string, page int) (*ListResponse, error) {
	var body struct {
		Results    []tmdbResult `json:"results"`
		TotalPages int          `json:"total_pages"`
		TotalCount int          `json:"total_results"`
	}
	query := url.Values{}
	query.Set("query", q)
	query.Set("page", fmt.Sprint(page))
	query.Set("include_adult", "false")
	if err := s.liveRequest(ctx, "/search/multi", query, &body); err != nil {
		return nil, err
	}
	items := make([]MediaSummary, 0, len(body.Results))
	for i := range body.Results {
		items = append(items, body.Results[i].toSummary())
	}
	return &ListResponse{
		Items:   items,
		Page:    page,
		HasMore: page < body.TotalPages,
		Total:   body.TotalCount,
	}, nil
}

// discoverLive 真实分类页（/discover/movie|tv）。
func (s *Service) discoverLive(ctx context.Context, mediaType, genre string, page int) (*ListResponse, error) {
	kind := "movie"
	if mediaType == "tv" || mediaType == "anime" || mediaType == "variety" {
		kind = "tv"
	}
	query := url.Values{}
	query.Set("page", fmt.Sprint(page))
	query.Set("sort_by", "popularity.desc")
	if genre != "" {
		if id, ok := genreIDMap[genre]; ok {
			query.Set("with_genres", id)
		} else {
			query.Set("with_keywords", genre)
		}
	}
	var body struct {
		Results    []tmdbResult `json:"results"`
		TotalPages int          `json:"total_pages"`
		TotalCount int          `json:"total_results"`
	}
	if err := s.liveRequest(ctx, "/discover/"+kind, query, &body); err != nil {
		return nil, err
	}
	items := make([]MediaSummary, 0, len(body.Results))
	for i := range body.Results {
		items = append(items, body.Results[i].toSummary())
	}
	return &ListResponse{
		Items:   items,
		Page:    page,
		HasMore: page < body.TotalPages,
		Total:   body.TotalCount,
	}, nil
}

// detailLive 真实详情（movie/tv 详情 + credits + 写入 media_library 缓存）。
func (s *Service) detailLive(ctx context.Context, externalID int64, mediaType string) (*MediaDetail, error) {
	if mediaType != "movie" {
		mediaType = "tv"
	}
	var body struct {
		ID           int64        `json:"id"`
		Title        string       `json:"title"`
		Name         string       `json:"name"`
		OrigTitle    string       `json:"original_title"`
		OrigName     string       `json:"original_name"`
		ReleaseDate  string       `json:"release_date"`
		FirstAir     string       `json:"first_air_date"`
		VoteAvg      float64      `json:"vote_average"`
		VoteCount    int          `json:"vote_count"`
		PosterPath   string       `json:"poster_path"`
		BackdropPath string       `json:"backdrop_path"`
		Overview     string       `json:"overview"`
		Runtime      int          `json:"runtime"`
		Seasons      []tmdbSeason `json:"seasons"`
		Genres       []struct {
			Name string `json:"name"`
		} `json:"genres"`
	}
	path := "/tv/" + fmt.Sprint(externalID)
	if mediaType == "movie" {
		path = "/movie/" + fmt.Sprint(externalID)
	}
	if err := s.liveRequest(ctx, path, nil, &body); err != nil {
		return nil, err
	}
	if body.ID == 0 {
		return nil, domain.Errf(domain.CodeNotFound)
	}
	title := body.Title
	if title == "" {
		title = body.Name
	}
	orig := body.OrigTitle
	if orig == "" {
		orig = body.OrigName
	}
	dateStr := body.ReleaseDate
	if dateStr == "" {
		dateStr = body.FirstAir
	}
	year := 0
	if len(dateStr) >= 4 {
		fmt.Sscanf(dateStr[:4], "%d", &year)
	}
	genres := make([]string, 0, len(body.Genres))
	for _, g := range body.Genres {
		genres = append(genres, g.Name)
	}

	det := &MediaDetail{
		MediaSummary: MediaSummary{
			ExternalID:     body.ID,
			ExternalSource: "tmdb",
			MediaType:      mediaType,
			Title:          title,
			TitleOrig:      orig,
			Year:           year,
			VoteAvg:        body.VoteAvg,
			PosterURL:      posterURL(body.PosterPath),
			BackdropURL:    backdropURL(body.BackdropPath),
			Overview:       body.Overview,
			Genres:         genres,
		},
		Runtime:  body.Runtime,
		Episodes: totalEpisodes(body.Seasons),
	}
	var seasons []SeasonInfo
	for _, se := range body.Seasons {
		if se.SeasonNumber <= 0 {
			continue
		}
		seasons = append(seasons, SeasonInfo{
			SeasonNumber: se.SeasonNumber,
			Name:         se.Name,
			EpisodeCount: se.EpisodeCount,
			AirDate:      se.AirDate,
			Overview:     se.Overview,
			PosterURL:    posterURL(se.PosterPath),
		})
	}
	det.Seasons = len(seasons)
	det.SeasonsList = seasons

	// §15.2 元数据缓存：写入 media_library（P0-3 匹配器查询源）
	if s.library != nil {
		m := &domain.MediaLibrary{
			ExternalID:     body.ID,
			ExternalSource: "tmdb",
			MediaType:      mediaType,
			Title:          title,
			TitleOrig:      orig,
			PosterURL:      posterURL(body.PosterPath),
			BackdropURL:    backdropURL(body.BackdropPath),
			Overview:       body.Overview,
			Year:           year,
			VoteAvg:        body.VoteAvg,
			VoteCount:      body.VoteCount,
			Genres:         json.RawMessage(mustJSON(genres)),
			Runtime:        body.Runtime,
			Seasons:        len(seasons),
			Episodes:       totalEpisodes(body.Seasons),
			CachedAt:       time.Now(),
		}
		_, _ = s.library.Upsert(ctx, m)
	}
	return det, nil
}

// tmdbSeason TMDB 季信息（detail/seasons 共用）。
type tmdbSeason struct {
	SeasonNumber int    `json:"season_number"`
	Name         string `json:"name"`
	EpisodeCount int    `json:"episode_count"`
	AirDate      string `json:"air_date"`
	Overview     string `json:"overview"`
	PosterPath   string `json:"poster_path"`
}

func totalEpisodes(seasons []tmdbSeason) int {
	total := 0
	for _, se := range seasons {
		if se.SeasonNumber > 0 {
			total += se.EpisodeCount
		}
	}
	return total
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// seasonsLive 真实季集列表（/tv/{id} 的 seasons 数组，episode_count 级）。
func (s *Service) seasonsLive(ctx context.Context, externalID int64) ([]SeasonInfo, error) {
	var body struct {
		Seasons []tmdbSeason `json:"seasons"`
	}
	if err := s.liveRequest(ctx, "/tv/"+fmt.Sprint(externalID), nil, &body); err != nil {
		return nil, err
	}
	out := make([]SeasonInfo, 0, len(body.Seasons))
	for _, se := range body.Seasons {
		if se.SeasonNumber <= 0 {
			continue
		}
		out = append(out, SeasonInfo{
			SeasonNumber: se.SeasonNumber,
			Name:         se.Name,
			EpisodeCount: se.EpisodeCount,
			AirDate:      se.AirDate,
			Overview:     se.Overview,
			PosterURL:    posterURL(se.PosterPath),
		})
	}
	if len(out) == 0 {
		return nil, domain.Errf(domain.CodeNotFound)
	}
	return out, nil
}

// tmdbResult 列表条目（trending/discover/search 共用）。
type tmdbResult struct {
	ID          int64   `json:"id"`
	Title       string  `json:"title"`
	Name        string  `json:"name"`
	OrigTitle   string  `json:"original_title"`
	OrigName    string  `json:"original_name"`
	ReleaseDate string  `json:"release_date"`
	FirstAir    string  `json:"first_air_date"`
	VoteAvg     float64 `json:"vote_average"`
	PosterPath  string  `json:"poster_path"`
	Backdrop    string  `json:"backdrop_path"`
	Overview    string  `json:"overview"`
	MediaType   string  `json:"media_type"`
}

func (r tmdbResult) toSummary() MediaSummary {
	title := r.Title
	if title == "" {
		title = r.Name
	}
	orig := r.OrigTitle
	if orig == "" {
		orig = r.OrigName
	}
	dateStr := r.ReleaseDate
	if dateStr == "" {
		dateStr = r.FirstAir
	}
	year := 0
	if len(dateStr) >= 4 {
		fmt.Sscanf(dateStr[:4], "%d", &year)
	}
	mt := r.MediaType
	if mt == "" {
		if r.Title != "" {
			mt = "movie"
		} else {
			mt = "tv"
		}
	}
	return MediaSummary{
		ExternalID:     r.ID,
		ExternalSource: "tmdb",
		MediaType:      mt,
		Title:          title,
		TitleOrig:      orig,
		Year:           year,
		VoteAvg:        r.VoteAvg,
		PosterURL:      posterURL(r.PosterPath),
		BackdropURL:    backdropURL(r.Backdrop),
		Overview:       r.Overview,
	}
}
