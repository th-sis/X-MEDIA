package api

import (
	"fmt"
	"net/http"
	"strings"

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

// PUT /api/admin/tmdb/config — 保存 TMDB API Key 并立即测试连通（§1.4 Step 2）。
// 请求体：{"api_key": "..."}。测试通过才落库；失败返回具体原因且不保存。
func (h *Handler) tmdbAdminConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		APIKey string `json:"api_key"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	if h.tmdb == nil {
		writeErr(w, domain.Errf(domain.CodeInternal))
		return
	}
	// 先测试，通过才保存
	count, err := h.tmdb.TestKey(r.Context(), req.APIKey)
	if err != nil {
		writeErr(w, err)
		return
	}
	if h.configs != nil {
		if err := h.configs.Set(r.Context(), domain.ConfigTMDBAPIKey, req.APIKey); err != nil {
			writeErr(w, err)
			return
		}
	}
	writeOK(w, map[string]any{"saved": true, "test_count": count})
}

// POST /api/admin/tmdb/test — 测试 TMDB API Key 连通性（§1.4 Step 2）。
// 请求体：{"api_key": "..."}（可省略，省略时用已保存的 key）。
func (h *Handler) tmdbAdminTest(w http.ResponseWriter, r *http.Request) {
	var req struct {
		APIKey string `json:"api_key"`
	}
	_ = decodeJSON(r, &req)
	key := strings.TrimSpace(req.APIKey)
	if key == "" && h.configs != nil {
		if v, ok, err := h.configs.Get(r.Context(), domain.ConfigTMDBAPIKey); err == nil && ok {
			key = v
		}
	}
	if h.tmdb == nil {
		writeErr(w, domain.Errf(domain.CodeInternal))
		return
	}
	count, err := h.tmdb.TestKey(r.Context(), key)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, map[string]any{"ok": true, "test_count": count})
}
