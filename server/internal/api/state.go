package api

import (
	"net/http"
	"time"
)

// GET /api/state/snapshot — 全局状态快照（客户端 app_state 恢复兜底数据源）。
// 聚合 capabilities/索引计数/活跃解析任务/版本与运行时长。
// V7 §28.3 追加 server_started_at / last_restart_reason，客户端对比后可感知重启并强制刷新。
func (h *Handler) stateSnapshot(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	reason := h.lastRestartReason
	if reason == "" {
		reason = "graceful"
	}
	out := map[string]any{
		"server_version":      h.serverVersion,
		"server_started_at":   h.serverStartedAt,
		"server_uptime_secs":  int(time.Since(h.serverStartedAt).Seconds()),
		"last_restart_reason": reason,
	}
	if h.resolveSvc != nil {
		out["capabilities"] = h.resolveSvc.Capabilities(ctx)
		if n, err := h.resolveSvc.ActiveCount(ctx); err == nil {
			out["active_resolve_tasks"] = n
		}
	}
	if h.mediaIndex != nil {
		if n, err := h.mediaIndex.Count(ctx); err == nil {
			out["indexed_total"] = n
		}
	}
	if h.indexAdmin != nil && h.indexAdmin.engine != nil {
		out["index_scanning"] = h.indexAdmin.engine.IsScanning()
		out["index_progress"] = h.indexAdmin.engine.Progress()
	}
	writeOK(w, out)
}
