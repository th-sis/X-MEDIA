package api

import (
	"net/http"

	"xmedia/internal/domain"
	"xmedia/internal/resolve"
)

// POST /api/resolve — 触发播放引擎。
func (h *Handler) resolveCreate(w http.ResponseWriter, r *http.Request) {
	if h.resolveSvc == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "播放引擎未就绪", "")
		return
	}
	if h.rateLimiter != nil {
		if allow, retry := h.rateLimiter.Allow(r.Context(), clientIP(r)); !allow {
			writeJSONBody(w, http.StatusTooManyRequests, map[string]any{
				"error":           "请求过于频繁，请稍后再试",
				"code":            "RATE_LIMITED",
				"retry_after_sec": retry,
			})
			return
		}
	}
	var req resolve.Request
	if err := decodeBody(r, &req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_PARAM", "请求体解析失败", "")
		return
	}
	if req.ExternalID <= 0 {
		writeAPIError(w, http.StatusBadRequest, "INVALID_PARAM", "缺少 external_id", "")
		return
	}
	if req.ExternalSource == "" {
		req.ExternalSource = "tmdb"
	}
	result, err := h.resolveSvc.Resolve(r.Context(), req)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL", "播放引擎启动失败", "")
		return
	}
	status := http.StatusCreated
	if result.Reused {
		status = http.StatusOK
	}
	writeJSONBody(w, status, result)
}

// GET /api/resolve/result/{task_id} — 查询解析结果。
func (h *Handler) resolveResult(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathInt64(r, "id")
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error(), "")
		return
	}
	task, streamURL, err := h.resolveSvc.Result(r.Context(), id)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, "NOT_FOUND", "任务不存在", "")
		return
	}
	switch task.Status {
	case domain.ResolveDone:
		writeJSONBody(w, http.StatusOK, map[string]any{
			"stream_url":  streamURL,
			"source":      task.ResultSource,
			"file_id":     task.ResultFileID,
			"title":       task.Title,
			"year":        task.Year,
			"transfer_id": 0,
		})
	case domain.ResolveFailed:
		writeJSONBody(w, http.StatusOK, map[string]any{
			"status":       "failed",
			"stage":        string(task.Stage),
			"stage_detail": task.StageDetail,
			"error_msg":    task.ErrorMsg,
		})
	default:
		writeJSONBody(w, http.StatusOK, map[string]any{
			"status":       "running",
			"stage":        string(task.Stage),
			"stage_detail": task.StageDetail,
			"progress_pct": task.ProgressPct,
		})
	}
}
