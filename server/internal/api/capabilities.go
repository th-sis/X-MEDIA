package api

import "net/http"

// GET /api/capabilities — 能力预检。
func (h *Handler) capabilities(w http.ResponseWriter, r *http.Request) {
	caps := h.resolveSvc.Capabilities(r.Context())
	writeJSONBody(w, http.StatusOK, caps)
}
