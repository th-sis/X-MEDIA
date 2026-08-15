package api

import "net/http"

// GET /api/stream?ticket=xxx — 播放流代理。
func (h *Handler) streamHandler(w http.ResponseWriter, r *http.Request) {
	if h.streamProxy == nil {
		http.Error(w, "stream unavailable", http.StatusBadGateway)
		return
	}
	h.streamProxy.ServeHTTP(w, r)
}
