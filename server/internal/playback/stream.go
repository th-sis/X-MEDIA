package playback

import (
	"context"
	"net/http"
	"os"
	"strings"

	"xmedia/internal/domain"
)

// StreamProxy 处理 /api/stream?ticket=xxx 的票据校验与分流。
type StreamProxy struct {
	signer   *TicketSigner
	playback *Service
	configs  domain.ConfigRepository
}

func NewStreamProxy(signer *TicketSigner, pb *Service, configs domain.ConfigRepository) *StreamProxy {
	return &StreamProxy{signer: signer, playback: pb, configs: configs}
}

func (sp *StreamProxy) demoURL(ctx context.Context) string {
	v, ok, err := sp.configs.Get(ctx, domain.ConfigDemoVideoURL)
	if err != nil || !ok || strings.TrimSpace(v) == "" {
		return domain.ConfigDefaults[domain.ConfigDemoVideoURL]
	}
	return strings.TrimSpace(v)
}

// ServeHTTP 校验票据并分流到演示/本地/网盘播放源。
func (sp *StreamProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ticket := r.URL.Query().Get("ticket")
	if ticket == "" {
		http.Error(w, "missing ticket", http.StatusBadRequest)
		return
	}
	claims, err := sp.signer.Verify(r.Context(), ticket)
	if err != nil {
		http.Error(w, "invalid or expired ticket", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "no-store")

	switch claims.Source {
	case "demo":
		http.Redirect(w, r, sp.demoURL(r.Context()), http.StatusFound)
		return
	case "nas":
		sp.serveLocal(w, r, claims.FileID)
		return
	default:
		if sp.playback == nil {
			http.Error(w, "playback unavailable", http.StatusBadGateway)
			return
		}
		err := sp.playback.ServeHTTP(w, r, Request{AccountID: claims.AccountID, FileID: claims.FileID}, Intent{FileName: ""})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
		}
		return
	}
}

func (sp *StreamProxy) serveLocal(w http.ResponseWriter, r *http.Request, path string) {
	if path == "" {
		http.Error(w, "empty path", http.StatusBadRequest)
		return
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, path)
}
