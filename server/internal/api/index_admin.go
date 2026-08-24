package api

import (
	"context"
	"net/http"
	"os"
	"strconv"

	"xmedia/internal/domain"
	"xmedia/internal/indexengine"
)

// indexAdminHandlers 索引管理端点（§12.2）。
type indexAdminHandlers struct {
	engine *indexengine.Service
	index  domain.MediaIndexRepository
	// [实测回归] 扫描预检用: 列 enabled sources + 判可达性 + 部署指引.
	nasSources domain.NASSourceRepository
	resolver   *nasMountResolver
}

// precheckScanTargets [V7 §9.7 实测回归] 全量/增量扫描触发前预检:
// 无启用源 / 全部路径不可达时直接 400 拦截, 不再异步空跑后
// done/0 文件让界面"无反应". 错误信息含 deployHint.
func (h *indexAdminHandlers) precheckScanTargets(ctx context.Context) error {
	if h.nasSources == nil {
		return nil // 未接线时不拦截, 保持旧行为
	}
	sources, err := h.nasSources.ListEnabled(ctx)
	if err != nil {
		return nil // 读失败不拦截, 让引擎自己报错
	}
	if len(sources) == 0 {
		return domain.Errorf(domain.CodeValidation,
			"未配置启用的 NAS 媒体源，请先在「媒体配置 → NAS 配置」中添加")
	}
	accessible := 0
	for _, src := range sources {
		if info, serr := os.Stat(src.Path); serr == nil && info.IsDir() {
			accessible++
		}
	}
	if accessible > 0 {
		return nil
	}
	msg := "全部 " + strconv.Itoa(len(sources)) + " 个启用中的媒体源路径在容器内均不可达，扫描不会产出任何索引"
	if h.resolver != nil {
		if hint := h.resolver.deployHint(); hint != "" {
			msg += "。" + hint
		}
	}
	return domain.Errorf(domain.CodeValidation, "%s", msg)
}

// handleIndexStatus GET /api/admin/index/status — 当前/最近一次索引进度。
func (h *indexAdminHandlers) handleIndexStatus(w http.ResponseWriter, r *http.Request) {
	p := h.engine.Progress()
	count := 0
	if h.index != nil {
		if n, err := h.index.Count(r.Context()); err == nil {
			count = n
		}
	}
	writeOK(w, map[string]any{
		"progress":      p,
		"indexed_total": count,
		"last_scan_at":  h.engine.LastScanAt(),
	})
}

// handleIndexNASFull POST /api/admin/index/nas/full — 触发全盘扫描。
func (h *indexAdminHandlers) handleIndexNASFull(w http.ResponseWriter, r *http.Request) {
	if err := h.precheckScanTargets(r.Context()); err != nil {
		writeErr(w, err)
		return
	}
	if err := h.engine.ScanNASFull(r.Context()); err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, map[string]any{"started": true, "scope": "nas", "mode": "full"})
}

// handleIndexNASIncremental POST /api/admin/index/nas/incremental — 触发增量扫描。
func (h *indexAdminHandlers) handleIndexNASIncremental(w http.ResponseWriter, r *http.Request) {
	if err := h.precheckScanTargets(r.Context()); err != nil {
		writeErr(w, err)
		return
	}
	if err := h.engine.ScanNASIncremental(r.Context()); err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, map[string]any{"started": true, "scope": "nas", "mode": "incremental"})
}

// handleIndexRebuild POST /api/admin/index/rebuild/{account_id} — 网盘索引重建（v1：清空该账号索引）。
func (h *indexAdminHandlers) handleIndexRebuild(w http.ResponseWriter, r *http.Request) {
	accountID, err := strconv.ParseInt(r.PathValue("account_id"), 10, 64)
	if err != nil || accountID <= 0 {
		writeErr(w, domain.Errorf(domain.CodeValidation, "账号 ID 无效"))
		return
	}
	items, err := h.index.ListBySource(r.Context(), "pan", accountID)
	if err != nil {
		writeErr(w, err)
		return
	}
	removed := 0
	for _, item := range items {
		if err := h.index.DeleteBySourcePath(r.Context(), item.SourceType, item.FilePath); err != nil {
			writeErr(w, err)
			return
		}
		removed++
	}
	writeOK(w, map[string]any{"removed": removed})
}

// handleIndexCleanup POST /api/admin/index/cleanup/{account_id} — 网盘转存清理（§9.5）。
func (h *indexAdminHandlers) handleIndexCleanup(w http.ResponseWriter, r *http.Request) {
	accountID, err := strconv.ParseInt(r.PathValue("account_id"), 10, 64)
	if err != nil || accountID <= 0 {
		writeErr(w, domain.Errorf(domain.CodeValidation, "账号 ID 无效"))
		return
	}
	removed, err := h.engine.Cleanup(r.Context(), "pan", accountID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, map[string]any{"removed": removed})
}
