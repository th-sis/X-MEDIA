package api

import (
	"net/http"
	"strings"
	"time"

	"xmedia/internal/domain"
	"xmedia/internal/websocket"
)

// GET /ws — WebSocket 升级，连接后推送 health_check 首条消息。
func (h *Handler) wsHandle(w http.ResponseWriter, r *http.Request) {
	if h.hub == nil {
		http.Error(w, "websocket unavailable", http.StatusServiceUnavailable)
		return
	}
	h.hub.HandleWith(w, r, func(c *websocket.Client) {
		c.Send(websocket.TypeHealthCheck, h.healthPayload(r))
	})
}

func (h *Handler) healthPayload(r *http.Request) websocket.HealthPayload {
	ctx := r.Context()

	db := "ok"
	tmdb := "not_configured"
	if h.configs != nil {
		if v, ok, _ := h.configs.Get(ctx, domain.ConfigTMDBAPIKey); ok && strings.TrimSpace(v) != "" {
			tmdb = "ok"
		}
	}

	pan := "unavailable"
	if h.pansearch != nil && h.pansearch.Health(ctx) {
		pan = "ok"
	}

	var accounts []websocket.AccountHealth
	if h.accountSvc != nil {
		if views, err := h.accountSvc.List(ctx); err == nil {
			for _, v := range views {
				status := "not_logged_in"
				if v.AuthStatus == domain.AuthActive {
					status = "ok"
				}
				accounts = append(accounts, websocket.AccountHealth{
					Driver: v.Account.DriverType,
					Status: status,
					Label:  v.Account.Name,
				})
			}
		}
	}

	nas := websocket.NASHealth{Status: "not_configured"}
	if h.configs != nil {
		if v, ok, _ := h.configs.Get(ctx, domain.ConfigNASLocalPath); ok && strings.TrimSpace(v) != "" {
			nas.Status = "ok"
			nas.Path = v
		}
	}

	index := websocket.IndexHealth{}
	if h.media != nil {
		if n, err := h.media.IndexedCount(ctx); err == nil {
			index.TotalFiles = n
		}
	}

	caps := domain.Capabilities{}
	if h.resolveSvc != nil {
		caps = h.resolveSvc.Capabilities(ctx)
	}

	overall := "ok"
	if tmdb == "not_configured" || pan == "unavailable" {
		overall = "warning"
	}

	return websocket.HealthPayload{
		DB:              db,
		TMDB:            tmdb,
		Pansearch:       pan,
		Accounts:        accounts,
		NAS:             nas,
		Index:           index,
		Capabilities:    caps,
		Version:         h.serverVersion,
		ServerStartedAt: h.serverStartedAt.Format(time.RFC3339),
		Overall:         overall,
	}
}
