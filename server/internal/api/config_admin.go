package api

import (
	"net/http"

	"xmedia/internal/domain"
)

// configAdminHandlers X-MEDIA 配置读写（§12.2 管理 API；仅白名单键可写）。
type configAdminHandlers struct {
	configs domain.ConfigRepository
}

// writableConfigKeys 允许经管理界面写入的 X-MEDIA 配置键白名单。
// 其他键（密钥/内部状态）不可经 HTTP 修改。
var writableConfigKeys = map[string]struct{}{
	domain.ConfigTMDBAPIKey:             {},
	domain.ConfigTMDBLanguage:           {},
	domain.ConfigNASLocalPath:           {},
	domain.ConfigNASEnabled:             {},
	domain.ConfigPansearchURL:           {},
	domain.ConfigPansearchAuthOn:        {},
	domain.ConfigPansearchToken:         {},
	domain.ConfigPansearchCAMBlock:      {},
	domain.ConfigPansearch4KPriority:    {},
	domain.ConfigResolvePriority:        {},
	domain.ConfigResolveMagnetEnabled:   {},
	domain.ConfigResolveMagnetTarget:    {},
	domain.ConfigResolveDemoFallback:    {},
	domain.ConfigResolveP0MinScore:      {},
	domain.ConfigBangumiAPIBase:         {},
	domain.ConfigSubscriptionSearchDays: {},
	domain.ConfigMediaLibraryMaxRows:    {},
	domain.ConfigMediaLibraryKeepRows:   {},
}

// handleConfigsGet GET /api/admin/configs — 返回全部白名单键当前值（脱敏 token）。
func (h *configAdminHandlers) handleConfigsGet(w http.ResponseWriter, r *http.Request) {
	if h.configs == nil {
		writeOK(w, map[string]any{"items": map[string]string{}})
		return
	}
	all, err := h.configs.All(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	out := map[string]string{}
	for key := range writableConfigKeys {
		if v, ok := all[key]; ok {
			if key == domain.ConfigPansearchToken {
				v = maskSecret(v)
			}
			out[key] = v
		}
	}
	writeOK(w, map[string]any{"items": out})
}

// handleConfigsPut PUT /api/admin/configs — 写入单个白名单键。
// 请求体：{"key": "...", "value": "..."}。
func (h *configAdminHandlers) handleConfigsPut(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	if _, ok := writableConfigKeys[req.Key]; !ok {
		writeErr(w, domain.Errorf(domain.CodeValidation, "配置键 %q 不允许经界面修改", req.Key))
		return
	}
	if h.configs == nil {
		writeErr(w, domain.Errf(domain.CodeInternal))
		return
	}
	if err := h.configs.Set(r.Context(), req.Key, req.Value); err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, map[string]any{"key": req.Key, "saved": true})
}

func maskSecret(v string) string {
	if len(v) <= 8 {
		return "********"
	}
	return v[:4] + "****" + v[len(v)-4:]
}
