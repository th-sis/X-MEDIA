package api

import (
	"net/http"

	"xmedia/internal/domain"
	"xmedia/internal/eventbus"
)

// configAdminHandlers X-MEDIA 配置读写（§12.2 管理 API；仅白名单键可写）。
// V7 §11.1：保存后通过 Bus 发布 ConfigChanged 事件，由 WS Hub 监听后推送
// capabilities 变更消息给所有客户端。
type configAdminHandlers struct {
	configs domain.ConfigRepository
	bus     *eventbus.Bus
}

// writableConfigKeys 允许经管理界面写入的 X-MEDIA 配置键白名单。
// 其他键（密钥/内部状态）不可经 HTTP 修改。
//
// §6.9 转存/配额相关键以 pan_ 前缀动态放行；具体键由 handleConfigsPut 校验前缀。
var writableConfigKeys = map[string]struct{}{
	domain.ConfigTMDBAPIKey:   {},
	domain.ConfigTMDBLanguage: {},
	domain.ConfigNASLocalPath: {},
	// [V7 §9.7] NAS 多媒体源列表（JSON 数组）+ 父目录展示
	domain.ConfigNASLocalPaths:          {},
	domain.ConfigNASRootPath:            {},
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
	domain.ConfigPanRenameEnabled:       {},
}

// allowedConfigKey 检查请求 key 是否在白名单或 §6.9 前缀白名单内。
func allowedConfigKey(key string) bool {
	if _, ok := writableConfigKeys[key]; ok {
		return true
	}
	for _, prefix := range []string{
		domain.ConfigKeyPrefixPanSaveRoot,
		domain.ConfigKeyPrefixPanQuotaWarn,
		domain.ConfigKeyPrefixPanCleanupMode,
		domain.ConfigKeyPrefixPanCleanupKeep,
		// [V7 整改 commit #4] NAS mount map: nas_mount_<host_path> -> container_path
		domain.ConfigKeyPrefixNASMount,
	} {
		if len(key) > len(prefix) && key[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

// handleConfigsGet GET /api/admin/configs — 返回全部白名单键当前值（脱敏 token）。
// §6.9 前缀键（pan_save_root_* / pan_quota_warning_* / pan_cleanup_mode_* /
// pan_cleanup_keep_recent_days_*）一并返回，按账号/驱动粒度。
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
	for key, value := range all {
		if !allowedConfigKey(key) {
			continue
		}
		if key == domain.ConfigPansearchToken {
			value = maskSecret(value)
		}
		out[key] = value
	}
	writeOK(w, map[string]any{"items": out})
}

// handleConfigsPut PUT /api/admin/configs — 写入单个白名单键或 §6.9 前缀键。
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
	if !allowedConfigKey(req.Key) {
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
	// V7 §11.1.1:配置保存后发布 ConfigChanged 事件；WS Hub 监听后会推送
	// capabilities 变更消息给所有前端，触发客户端重新拉取 /api/capabilities。
	if h.bus != nil {
		h.bus.Publish(r.Context(), eventbus.ConfigChanged{Key: req.Key, Value: req.Value})
	}
	writeOK(w, map[string]any{"key": req.Key, "saved": true})
}

func maskSecret(v string) string {
	if len(v) <= 8 {
		return "********"
	}
	return v[:4] + "****" + v[len(v)-4:]
}
