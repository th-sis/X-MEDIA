package api

import (
	"net/http"
)

// GET /api/bangumi/search?q= — Bangumi 动漫搜索（§7.3）。
func (h *Handler) bangumiSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	items, err := h.tmdb.SearchBangumi(r.Context(), q)
	if err != nil {
		writeAPIError(w, http.StatusBadGateway, "UPSTREAM_FAILED", "Bangumi 服务不可用", "请稍后重试")
		return
	}
	writeList(w, items, 1, false, len(items))
}

// GET /api/bangumi/detail/{id} — Bangumi 条目详情（§7.3）。
func (h *Handler) bangumiDetail(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathInt64(r, "id")
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error(), "")
		return
	}
	det, err := h.tmdb.BangumiDetail(r.Context(), id)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, "NOT_FOUND", "未找到该条目", "")
		return
	}
	writeJSONBody(w, http.StatusOK, det)
}
