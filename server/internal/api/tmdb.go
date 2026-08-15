package api

import (
	"fmt"
	"net/http"

	"xmedia/internal/domain"
	"xmedia/internal/tmdb"
)

// GET /api/tmdb/home — 首页 12 榜单。
func (h *Handler) tmdbHome(w http.ResponseWriter, r *http.Request) {
	secs, err := h.tmdb.Home(r.Context())
	if err != nil {
		writeAPIError(w, http.StatusBadGateway, "UPSTREAM_FAILED", "元数据服务不可用", "请稍后重试")
		return
	}
	writeJSONBody(w, http.StatusOK, map[string]any{"sections": secs})
}

// GET /api/tmdb/discover?type=&genre=&page= — 分类页分页（trending 亦复用）。
func (h *Handler) tmdbDiscover(w http.ResponseWriter, r *http.Request) {
	mediaType := r.URL.Query().Get("type")
	if mediaType == "" {
		mediaType = "movie"
	}
	genre := r.URL.Query().Get("genre")
	page := queryIntDefault(r, "page", 1)
	resp, err := h.tmdb.Discover(r.Context(), mediaType, genre, page)
	if err != nil {
		writeAPIError(w, http.StatusBadGateway, "UPSTREAM_FAILED", "元数据服务不可用", "请稍后重试")
		return
	}
	writeList(w, resp.Items, resp.Page, resp.HasMore, resp.Total)
}

// GET /api/tmdb/search?q=&page= — 搜索。
func (h *Handler) tmdbSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	page := queryIntDefault(r, "page", 1)
	if q != "" && h.media != nil {
		_ = h.media.RecordSearch(r.Context(), q)
	}
	resp, err := h.tmdb.Search(r.Context(), q, page)
	if err != nil {
		writeAPIError(w, http.StatusBadGateway, "UPSTREAM_FAILED", "搜索服务不可用", "请稍后重试")
		return
	}
	writeList(w, resp.Items, resp.Page, resp.HasMore, resp.Total)
}

// GET /api/tmdb/detail/{id}?source= — 详情。
func (h *Handler) tmdbDetail(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathInt64(r, "id")
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error(), "")
		return
	}
	source := r.URL.Query().Get("source")
	if source == "" {
		source = "tmdb"
	}
	det, err := h.tmdb.Detail(r.Context(), id, source)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, "NOT_FOUND", "未找到该内容", "")
		return
	}
	writeJSONBody(w, http.StatusOK, det)
}

// GET /api/tmdb/seasons/{id}?source= — 季集列表（含可用性角标）。
func (h *Handler) tmdbSeasons(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathInt64(r, "id")
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error(), "")
		return
	}
	source := r.URL.Query().Get("source")
	if source == "" {
		source = "tmdb"
	}
	seasons, err := h.tmdb.Seasons(r.Context(), id, source)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, "NOT_FOUND", "未找到该内容", "")
		return
	}
	annotateAvailability(r, h, id, source, seasons)
	writeJSONBody(w, http.StatusOK, seasons)
}

func annotateAvailability(r *http.Request, h *Handler, id int64, source string, seasons []tmdb.SeasonInfo) {
	if h.media == nil {
		return
	}
	var keys []domain.AvailabilityKey
	type pos struct{ si, ei int }
	var positions []pos
	for si, s := range seasons {
		for ei, e := range s.Episodes {
			keys = append(keys, domain.AvailabilityKey{
				ExternalID: id, ExternalSource: source, Season: s.SeasonNumber, Episode: e.EpisodeNumber,
			})
			positions = append(positions, pos{si, ei})
		}
	}
	if len(keys) == 0 {
		return
	}
	available, err := h.media.CheckAvailability(r.Context(), keys)
	if err != nil {
		return
	}
	set := map[string]bool{}
	for _, k := range available {
		set[fmt.Sprintf("%d-%d", k.Season, k.Episode)] = true
	}
	for i, p := range positions {
		k := keys[i]
		if set[fmt.Sprintf("%d-%d", k.Season, k.Episode)] {
			seasons[p.si].Episodes[p.ei].Available = true
		}
	}
}

func queryIntDefault(r *http.Request, name string, def int) int {
	v := r.URL.Query().Get(name)
	if v == "" {
		return def
	}
	n := 0
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil {
		return def
	}
	return n
}
