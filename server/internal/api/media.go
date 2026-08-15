package api

import (
	"net/http"

	"xmedia/internal/domain"
)

// ---- 继续观看 / 历史 ----

func (h *Handler) mediaContinueWatching(w http.ResponseWriter, r *http.Request) {
	items, err := h.media.ContinueWatching(r.Context(), 20)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL", "查询失败", "")
		return
	}
	writeList(w, items, 1, false, len(items))
}

func (h *Handler) mediaHistoryList(w http.ResponseWriter, r *http.Request) {
	items, err := h.media.ListHistory(r.Context(), 100)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL", "查询失败", "")
		return
	}
	writeList(w, items, 1, false, len(items))
}

type historyUpsertReq struct {
	ExternalID     int64  `json:"external_id"`
	ExternalSource string `json:"external_source"`
	MediaType      string `json:"media_type"`
	Title          string `json:"title"`
	PosterURL      string `json:"poster_url"`
	SourceType     string `json:"source_type"`
	Season         int    `json:"season"`
	Episode        int    `json:"episode"`
	PositionMs     int64  `json:"position_ms"`
	DurationMs     int64  `json:"duration_ms"`
}

func (h *Handler) mediaHistoryUpsert(w http.ResponseWriter, r *http.Request) {
	var req historyUpsertReq
	if err := decodeBody(r, &req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_PARAM", "请求体解析失败", "")
		return
	}
	if req.ExternalID <= 0 {
		writeAPIError(w, http.StatusBadRequest, "INVALID_PARAM", "缺少 external_id", "")
		return
	}
	err := h.media.UpsertHistory(r.Context(), &domain.PlayHistory{
		ExternalID:     req.ExternalID,
		ExternalSource: orDefault(req.ExternalSource, "tmdb"),
		MediaType:      req.MediaType,
		Title:          req.Title,
		PosterURL:      req.PosterURL,
		SourceType:     req.SourceType,
		Season:         req.Season,
		Episode:        req.Episode,
		PositionMs:     req.PositionMs,
		DurationMs:     req.DurationMs,
	})
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL", "写入失败", "")
		return
	}
	writeJSONBody(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) mediaHistoryDelete(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathInt64(r, "id")
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error(), "")
		return
	}
	source := r.URL.Query().Get("source")
	if source == "" {
		source = "tmdb"
	}
	season := queryIntDefault(r, "season", 0)
	episode := queryIntDefault(r, "episode", 0)
	if err := h.media.DeleteHistory(r.Context(), id, source, season, episode); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL", "删除失败", "")
		return
	}
	writeJSONBody(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) mediaHistoryClear(w http.ResponseWriter, r *http.Request) {
	if err := h.media.ClearHistory(r.Context()); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL", "清空失败", "")
		return
	}
	writeJSONBody(w, http.StatusOK, map[string]any{"ok": true})
}

// ---- 收藏 ----

func (h *Handler) mediaFavoritesList(w http.ResponseWriter, r *http.Request) {
	items, err := h.media.ListFavorites(r.Context())
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL", "查询失败", "")
		return
	}
	writeList(w, items, 1, false, len(items))
}

type favoriteReq struct {
	ExternalID     int64  `json:"external_id"`
	ExternalSource string `json:"external_source"`
	MediaType      string `json:"media_type"`
	Title          string `json:"title"`
	PosterURL      string `json:"poster_url"`
	Year           int    `json:"year"`
}

func (h *Handler) mediaFavoriteAdd(w http.ResponseWriter, r *http.Request) {
	var req favoriteReq
	if err := decodeBody(r, &req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_PARAM", "请求体解析失败", "")
		return
	}
	if err := h.media.AddFavorite(r.Context(), &domain.Favorite{
		ExternalID: req.ExternalID, ExternalSource: orDefault(req.ExternalSource, "tmdb"),
		MediaType: req.MediaType, Title: req.Title, PosterURL: req.PosterURL, Year: req.Year,
	}); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL", "添加失败", "")
		return
	}
	writeJSONBody(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) mediaFavoriteRemove(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathInt64(r, "id")
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error(), "")
		return
	}
	source := orDefault(r.URL.Query().Get("source"), "tmdb")
	if err := h.media.RemoveFavorite(r.Context(), id, source); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL", "移除失败", "")
		return
	}
	writeJSONBody(w, http.StatusOK, map[string]any{"ok": true})
}

// ---- 订阅 ----

func (h *Handler) mediaSubscriptionsList(w http.ResponseWriter, r *http.Request) {
	items, err := h.media.ListSubscriptions(r.Context())
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL", "查询失败", "")
		return
	}
	writeList(w, items, 1, false, len(items))
}

func (h *Handler) mediaSubscriptionAdd(w http.ResponseWriter, r *http.Request) {
	var req favoriteReq
	if err := decodeBody(r, &req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_PARAM", "请求体解析失败", "")
		return
	}
	if err := h.media.AddSubscription(r.Context(), &domain.Subscription{
		ExternalID: req.ExternalID, ExternalSource: orDefault(req.ExternalSource, "tmdb"),
		MediaType: req.MediaType, Title: req.Title, Year: req.Year, PosterURL: req.PosterURL,
		Status: domain.SubWatching, MaxSearches: 12,
	}); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL", "添加失败", "")
		return
	}
	writeJSONBody(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) mediaSubscriptionRemove(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathInt64(r, "id")
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error(), "")
		return
	}
	source := orDefault(r.URL.Query().Get("source"), "tmdb")
	if err := h.media.RemoveSubscription(r.Context(), id, source); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL", "移除失败", "")
		return
	}
	writeJSONBody(w, http.StatusOK, map[string]any{"ok": true})
}

// ---- 搜索历史 ----

func (h *Handler) mediaSearchHistoryList(w http.ResponseWriter, r *http.Request) {
	items, err := h.media.ListSearchHistory(r.Context(), 20)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL", "查询失败", "")
		return
	}
	writeList(w, items, 1, false, len(items))
}

func (h *Handler) mediaSearchHistoryClear(w http.ResponseWriter, r *http.Request) {
	if err := h.media.ClearSearchHistory(r.Context()); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL", "清空失败", "")
		return
	}
	writeJSONBody(w, http.StatusOK, map[string]any{"ok": true})
}

// ---- 可用性 ----

type availabilityReq struct {
	Items []domain.AvailabilityKey `json:"items"`
}

func (h *Handler) mediaCheckAvailability(w http.ResponseWriter, r *http.Request) {
	var req availabilityReq
	if err := decodeBody(r, &req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_PARAM", "请求体解析失败", "")
		return
	}
	if len(req.Items) > 100 {
		req.Items = req.Items[:100]
	}
	available, err := h.media.CheckAvailability(r.Context(), req.Items)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL", "查询失败", "")
		return
	}
	writeJSONBody(w, http.StatusOK, map[string]any{"available": available})
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
