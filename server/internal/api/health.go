package api

import (
	"encoding/json"
	"net/http"

	"xmedia/internal/domain"
)

// health 健康检查（[v7 整改] 增加 validation 字段暴露启动配置验证结果）。
func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	resp := map[string]any{"status": "ok"}
	if h.configs != nil {
		if raw, ok, err := h.configs.Get(r.Context(), "validation_last_result"); err == nil && ok {
			var validation domain.ValidationResult
			if err := json.Unmarshal([]byte(raw), &validation); err == nil {
				resp["validation"] = validation
			}
		}
	}
	writeOK(w, resp)
}
