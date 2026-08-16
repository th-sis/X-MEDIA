package api

import (
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// wsJWTClaims WebSocket JWT 载荷。
type wsJWTClaims struct {
	Username string `json:"username"`
	IsAdmin  bool   `json:"is_admin"`
	jwt.RegisteredClaims
}

// wsAuthMiddleware 允许通过 JWT 或 admin_session cookie 访问 WebSocket。
// 1) ?token= / Authorization: Bearer 走 JWT 校验；
// 2) 否则读取 admin_session cookie，复用 adminauth 会话校验。
func (h *Handler) wsAuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.adminAuth == nil {
			http.Error(w, "websocket unavailable", http.StatusServiceUnavailable)
			return
		}
		authorized := false

		tokenStr := r.URL.Query().Get("token")
		if tokenStr == "" {
			if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
				tokenStr = strings.TrimPrefix(auth, "Bearer ")
			}
		}
		if strings.TrimSpace(tokenStr) != "" {
			secret := h.adminAuth.SecretKey()
			if _, err := jwt.ParseWithClaims(tokenStr, &wsJWTClaims{}, func(t *jwt.Token) (interface{}, error) {
				return secret, nil
			}); err == nil {
				authorized = true
			}
		}

		if !authorized {
			if _, ok := h.adminAuth.ReadSession(r); ok {
				authorized = true
			}
		}

		if !authorized {
			// 本地调试模式：允许 localhost 无认证连接，避免 Flutter 桌面端无法建立 WebSocket
			if strings.Contains(r.RemoteAddr, "127.0.0.1") || strings.Contains(r.RemoteAddr, "[::1]") {
				authorized = true
			}
		}

		if !authorized {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	}
}
