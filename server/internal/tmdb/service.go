package tmdb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"xmedia/internal/domain"
)

const tmdbImageBase = "https://image.tmdb.org/t/p/w500"

// Service TMDB/Bangumi 元数据代理。
type Service struct {
	configs    domain.ConfigRepository
	library    domain.MediaLibraryRepository
	client     *http.Client
	base       string
	protectors LRUProtectors
}

func NewService(configs domain.ConfigRepository, library domain.MediaLibraryRepository) *Service {
	return &Service{
		configs: configs,
		library: library,
		client:  &http.Client{Timeout: 10 * time.Second},
		base:    "https://api.themoviedb.org/3",
	}
}

func (s *Service) apiKey(ctx context.Context) string {
	if s.configs == nil {
		return ""
	}
	v, ok, err := s.configs.Get(ctx, domain.ConfigTMDBAPIKey)
	if err != nil || !ok {
		return ""
	}
	return strings.TrimSpace(v)
}

func (s *Service) demoMode(ctx context.Context) bool {
	return s.apiKey(ctx) == ""
}

// toSummary 把演示条目转为卡片。
func (d demoItem) toSummary() MediaSummary {
	return MediaSummary{
		ExternalID:     d.ExternalID,
		ExternalSource: d.Source,
		MediaType:      d.MediaType,
		Title:          d.Title,
		TitleOrig:      d.TitleOrig,
		Year:           d.Year,
		VoteAvg:        d.VoteAvg,
		Overview:       d.Overview,
		Genres:         d.Genres,
		// [v7 整改] 演示数据加 picsum 占位图，不再让客户端只能渲染胶片图标
		PosterURL:   demoPosterURL(d.ExternalID),
		BackdropURL: demoBackdropURL(d.ExternalID),
	}
}

func allSummaries() []MediaSummary {
	out := make([]MediaSummary, 0, len(demoCatalog))
	for _, d := range demoCatalog {
		out = append(out, d.toSummary())
	}
	return out
}

func byType(mediaType string) []MediaSummary {
	out := make([]MediaSummary, 0)
	for _, d := range demoCatalog {
		if d.MediaType == mediaType {
			out = append(out, d.toSummary())
		}
	}
	return out
}

func pick(items []MediaSummary, n int) []MediaSummary {
	if len(items) <= n {
		return items
	}
	return items[:n]
}

// Home 返回首页 12 个榜单（演示目录派生；有 Key 时走真实 TMDB）。
func (s *Service) Home(ctx context.Context) ([]Section, error) {
	if !s.demoMode(ctx) {
		if secs, err := s.homeLive(ctx); err == nil {
			return secs, nil
		}
	}
	all := allSummaries()
	movies := byType("movie")
	tvs := byType("tv")
	sections := []Section{
		{Key: "trending_movie_week", Title: "热门电影", Items: pick(movies, 12)},
		{Key: "trending_tv_week", Title: "热门剧集", Items: pick(tvs, 12)},
		{Key: "trending_all_day", Title: "今日热播", Items: pick(all, 12)},
		{Key: "upcoming", Title: "即将上映", Items: pick(movies, 12)},
		{Key: "top_rated", Title: "评分最高", Items: pick(sortByVote(all), 12)},
		{Key: "action", Title: "动作电影", Items: pick(filterGenre(movies, "动作"), 8)},
		{Key: "scifi", Title: "科幻电影", Items: pick(filterGenre(movies, "科幻"), 8)},
		{Key: "comedy", Title: "喜剧电影", Items: pick(filterGenre(byType("tv"), "喜剧"), 8)},
		{Key: "tv_popular", Title: "热播剧集", Items: pick(tvs, 12)},
		{Key: "tv_top_rated", Title: "高分剧集", Items: pick(sortByVote(tvs), 12)},
		{Key: "anime", Title: "动漫", Items: pick(byType("tv"), 12)},
		{Key: "documentary", Title: "纪录片", Items: pick(byType("documentary"), 8)},
		{Key: "variety", Title: "综艺", Items: pick(byType("variety"), 8)},
	}
	return sections, nil
}

func sortByVote(items []MediaSummary) []MediaSummary {
	out := append([]MediaSummary(nil), items...)
	sort.Slice(out, func(i, j int) bool { return out[i].VoteAvg > out[j].VoteAvg })
	return out
}

func filterGenre(items []MediaSummary, genre string) []MediaSummary {
	out := make([]MediaSummary, 0)
	for _, it := range items {
		for _, g := range it.Genres {
			if g == genre {
				out = append(out, it)
				break
			}
		}
	}
	return out
}

// Discover 分类页分页。[P0-4] 有 Key 时走真实 /discover。
func (s *Service) Discover(ctx context.Context, mediaType, genre string, page int) (*ListResponse, error) {
	if !s.demoMode(ctx) {
		if resp, err := s.discoverLive(ctx, mediaType, genre, page); err == nil {
			return resp, nil
		}
	}
	var items []MediaSummary
	switch mediaType {
	case "movie":
		items = byType("movie")
	case "tv":
		items = byType("tv")
	case "anime":
		items = byType("tv")
	case "documentary":
		items = byType("documentary")
	case "variety":
		items = byType("variety")
	default:
		items = allSummaries()
	}
	if genre != "" {
		items = filterGenre(items, genre)
	}
	// 简单分页
	const pageSize = 18
	start := (page - 1) * pageSize
	total := len(items)
	if start >= total {
		return &ListResponse{Items: []MediaSummary{}, Page: page, HasMore: false, Total: total}, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return &ListResponse{Items: items[start:end], Page: page, HasMore: end < total, Total: total}, nil
}

// Search 搜索。[P0-4] 有 Key 时走真实 /search/multi。
func (s *Service) Search(ctx context.Context, q string, page int) (*ListResponse, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return &ListResponse{Items: []MediaSummary{}, Page: page, HasMore: false, Total: 0}, nil
	}
	if !s.demoMode(ctx) {
		if resp, err := s.searchLive(ctx, q, page); err == nil {
			return resp, nil
		}
	}
	lower := strings.ToLower(q)
	var items []MediaSummary
	for _, d := range demoCatalog {
		if strings.Contains(strings.ToLower(d.Title), lower) ||
			strings.Contains(strings.ToLower(d.TitleOrig), lower) {
			items = append(items, d.toSummary())
		}
	}
	return &ListResponse{Items: items, Page: page, HasMore: false, Total: len(items)}, nil
}

// Detail 详情。[P0-4] 有 Key 时走真实详情（+ media_library 缓存）。
func (s *Service) Detail(ctx context.Context, externalID int64, source string) (*MediaDetail, error) {
	if !s.demoMode(ctx) && source == "tmdb" {
		if det, err := s.detailLive(ctx, externalID, "movie"); err == nil {
			return det, nil
		}
		if det, err := s.detailLive(ctx, externalID, "tv"); err == nil {
			return det, nil
		}
	}
	for _, d := range demoCatalog {
		if d.ExternalID == externalID && d.Source == source {
			det := &MediaDetail{
				MediaSummary: d.toSummary(),
				Runtime:      d.Runtime,
				Seasons:      d.Seasons,
				Episodes:     d.EpisodeCnt,
				Cast: []CastMember{
					{Name: "演示演员", Character: "主角", ProfileURL: ""},
				},
			}
			if d.Seasons > 0 {
				det.SeasonsList = buildSeasons(d.Seasons, d.EpisodeCnt)
			}
			return det, nil
		}
	}
	return nil, domain.Errf(domain.CodeNotFound)
}

// Seasons 季集列表。[P0-4] 有 Key 时走真实 /tv/{id}。
func (s *Service) Seasons(ctx context.Context, externalID int64, source string) ([]SeasonInfo, error) {
	if !s.demoMode(ctx) && source == "tmdb" {
		if seasons, err := s.seasonsLive(ctx, externalID); err == nil {
			return seasons, nil
		}
	}
	for _, d := range demoCatalog {
		if d.ExternalID == externalID && d.Source == source && d.Seasons > 0 {
			return buildSeasons(d.Seasons, d.EpisodeCnt), nil
		}
	}
	return nil, domain.Errf(domain.CodeNotFound)
}

// buildSeasons 按季均分集数生成演示季集结构。
func buildSeasons(seasons, total int) []SeasonInfo {
	out := make([]SeasonInfo, 0, seasons)
	perSeason := total / seasons
	remainder := total % seasons
	for i := 1; i <= seasons; i++ {
		n := perSeason
		if i <= remainder {
			n++
		}
		eps := make([]EpisodeInfo, 0, n)
		for e := 1; e <= n; e++ {
			eps = append(eps, EpisodeInfo{EpisodeNumber: e, Name: fmt.Sprintf("第 %d 集", e), Available: false})
		}
		out = append(out, SeasonInfo{SeasonNumber: i, Name: fmt.Sprintf("第 %d 季", i), EpisodeCount: n, Episodes: eps})
	}
	return out
}

// homeLive 有 TMDB Key 时按真实榜单拉取；失败返回 error 由调用方回退演示数据。
func (s *Service) homeLive(ctx context.Context) ([]Section, error) {
	defs := []struct {
		key, title, path string
	}{
		{"trending_movie_week", "热门电影", "/trending/movie/week"},
		{"trending_tv_week", "热门剧集", "/trending/tv/week"},
		{"trending_all_day", "今日热播", "/trending/all/day"},
		{"upcoming", "即将上映", "/movie/upcoming"},
		{"top_rated", "评分最高", "/movie/top_rated"},
		{"tv_popular", "热播剧集", "/tv/popular"},
		{"tv_top_rated", "高分剧集", "/tv/top_rated"},
	}
	key := s.apiKey(ctx)
	var secs []Section
	for _, d := range defs {
		items, err := s.fetchList(ctx, d.path, key)
		if err != nil {
			return nil, err
		}
		secs = append(secs, Section{Key: d.key, Title: d.title, Items: items})
	}
	return secs, nil
}

func (s *Service) fetchList(ctx context.Context, path, key string) ([]MediaSummary, error) {
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	url := s.base + path + sep + "api_key=" + key + "&language=zh-CN"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tmdb status %d", resp.StatusCode)
	}
	var body struct {
		Results []struct {
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
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	items := make([]MediaSummary, 0, len(body.Results))
	for _, r := range body.Results {
		title := r.Title
		if title == "" {
			title = r.Name
		}
		orig := r.OrigTitle
		if orig == "" {
			orig = r.OrigName
		}
		year := 0
		dateStr := r.ReleaseDate
		if dateStr == "" {
			dateStr = r.FirstAir
		}
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
		items = append(items, MediaSummary{
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
		})
	}
	return items, nil
}

func posterURL(path string) string {
	if path == "" {
		return ""
	}
	return tmdbImageBase + path
}

func backdropURL(path string) string {
	if path == "" {
		return ""
	}
	return "https://image.tmdb.org/t/p/w1280" + path
}
