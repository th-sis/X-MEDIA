package indexengine

import (
	"context"
	"sync"
	"time"

	"xmedia/internal/domain"
	"xmedia/internal/websocket"
)

// Service 索引引擎（§9）：NAS 三阶段扫描 + 匹配 + 增量维护。
type Service struct {
	mediaIndex domain.MediaIndexRepository
	library    domain.MediaLibraryRepository
	configs    domain.ConfigRepository
	nasSources domain.NASSourceRepository
	hub        *websocket.Hub

	workerCount int

	mu       sync.Mutex
	scanning bool
	progress NASProgress
	lastScan time.Time
}

// NASProgress 当前/最近一次 NAS 扫描进度（WS index_status 推送源，§9.7.1）。
type NASProgress struct {
	Scope string `json:"scope"`
	// [V7 §9.7] Root 单条 NAS 媒体源路径 basename（多路径扫描时区分来源）；
	// 汇总进度时为空字符串。
	Root        string `json:"root,omitempty"`
	Phase       string `json:"phase"` // A / B / C / ""
	Status      string `json:"status"`
	Processed   int    `json:"processed"`
	Total       int    `json:"total"`
	Matched     int    `json:"matched"`
	Unconfirmed int    `json:"unconfirmed"`
	Orphaned    int    `json:"orphaned"`
	RatePerSec  int    `json:"rate_per_sec"`
	ErrorMsg    string `json:"error_msg"`
}

// Options 索引引擎依赖。
type Options struct {
	MediaIndex   domain.MediaIndexRepository
	MediaLibrary domain.MediaLibraryRepository
	Configs      domain.ConfigRepository
	NASSources   domain.NASSourceRepository
	Hub          *websocket.Hub
	WorkerCount  int
}

func NewService(opts Options) *Service {
	workers := opts.WorkerCount
	if workers <= 0 {
		workers = 8
	}
	return &Service{
		mediaIndex:  opts.MediaIndex,
		library:     opts.MediaLibrary,
		configs:     opts.Configs,
		nasSources:  opts.NASSources,
		hub:         opts.Hub,
		workerCount: workers,
	}
}

// NASPath 读取单条 NAS 本地路径（兼容旧 nas_local_path 单字符串，V7 §9.7 之前）。
// 推荐使用 NASPaths() 多路径版本。
func (s *Service) NASPath(ctx context.Context) string {
	paths := s.NASPaths(ctx)
	if len(paths) == 0 {
		return ""
	}
	return paths[0]
}

// NASPaths 读取所有启用的 NAS 媒体源路径（[V7 §9.4+ 扩展] G1.H）。
//
// 优先级：
//  1. 查 nas_sources 表 enabled=1 的 source 集合
//  2. 表为空 → 回退到 configs KV（nas_local_paths → nas_local_path）
//     此回退仅兼容迁移期遗留 KV；启动时 MigrateFromConfigsKV 会一次性清空
//  3. 都为空 → 返回 nil
//
// 返回的列表可直接用于 filepath.WalkDir；调用方负责确认目录存在性。
func (s *Service) NASPaths(ctx context.Context) []string {
	if s.nasSources != nil {
		sources, err := s.nasSources.ListEnabled(ctx)
		if err == nil && len(sources) > 0 {
			out := make([]string, 0, len(sources))
			for _, src := range sources {
				out = append(out, src.Path)
			}
			return out
		}
	}
	// 回退：兼容迁移前残余 KV
	if s.configs == nil {
		return nil
	}
	newJSON, _, _ := s.configs.Get(ctx, domain.ConfigNASLocalPaths)
	legacy, _, _ := s.configs.Get(ctx, domain.ConfigNASLocalPath)
	return domain.ParseNASPaths(newJSON, legacy)
}

// ListEnabledSources 返回启用的 NAS source 完整元数据（用于 capabilities +
// 多 path 进度汇报按 source 切分）。
func (s *Service) ListEnabledSources(ctx context.Context) []*domain.NASSource {
	if s.nasSources == nil {
		return nil
	}
	list, err := s.nasSources.ListEnabled(ctx)
	if err != nil {
		return nil
	}
	return list
}

// IsScanning 当前是否有 NAS 扫描在跑（P0 智能跳过用，§6.3）。
func (s *Service) IsScanning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.scanning
}

// Progress 返回最近一次扫描进度快照。
func (s *Service) Progress() NASProgress {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.progress
}

// LastScanAt 返回最近一次完整扫描时间。
func (s *Service) LastScanAt() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastScan
}

// ScanNASFull 触发 NAS 全盘扫描（异步，不阻塞调用方）。已在扫描中则忽略。
// [V7 §9.7] 多路径遍历：每条 NAS 媒体源路径独立 Phase A/B，结果合并。
func (s *Service) ScanNASFull(ctx context.Context) error {
	paths := s.NASPaths(ctx)
	if len(paths) == 0 {
		return domain.Errorf(domain.CodeValidation, "未配置 NAS 路径")
	}
	go s.scanNAS(context.Background(), paths, false)
	return nil
}

// ScanNASIncremental 触发 NAS 增量扫描（§9.7.4：仅扫描 mtime 新于上次扫描的文件 + 清理消失文件）。
// [V7 §9.7] 多路径遍历：每条路径独立处理。
func (s *Service) ScanNASIncremental(ctx context.Context) error {
	paths := s.NASPaths(ctx)
	if len(paths) == 0 {
		return domain.Errorf(domain.CodeValidation, "未配置 NAS 路径")
	}
	go s.scanNAS(context.Background(), paths, true)
	return nil
}

// pushProgress 更新进度并推送 WS index_status（§9.7.1）。
func (s *Service) pushProgress(p NASProgress) {
	s.mu.Lock()
	s.progress = p
	s.scanning = p.Status == "running"
	s.mu.Unlock()
	if s.hub != nil {
		s.hub.Broadcast(websocket.TypeIndexStatus, websocket.IndexStatusPayload{
			Scope:       p.Scope,
			Phase:       p.Phase,
			Status:      p.Status,
			Processed:   p.Processed,
			Total:       p.Total,
			Matched:     p.Matched,
			Unconfirmed: p.Unconfirmed,
			Orphaned:    p.Orphaned,
			RatePerSec:  p.RatePerSec,
			ErrorMsg:    p.ErrorMsg,
		})
	}
}

// Cleanup 网盘转存清理（§9.5）：跳过最近 2 小时内播放过的文件，删除其余索引与网盘文件。
// v1 仅清理索引条目（网盘文件删除依赖驱动 DeleteFile 接入，P0-3 后置）。
func (s *Service) Cleanup(ctx context.Context, sourceType string, accountID int64) (int, error) {
	items, err := s.mediaIndex.ListBySource(ctx, sourceType, accountID)
	if err != nil {
		return 0, err
	}
	cutoff := time.Now().Add(-2 * time.Hour)
	removed := 0
	for _, item := range items {
		if item.LastPlayedAt != nil && item.LastPlayedAt.After(cutoff) {
			continue // 正在播放或刚播放过，跳过（§9.5）
		}
		if err := s.mediaIndex.DeleteBySourcePath(ctx, item.SourceType, item.FilePath); err != nil {
			return removed, err
		}
		removed++
	}
	return removed, nil
}

// IndexFile 单文件索引（网盘转存/下载完成时直写，§9.3 事件驱动路径）。
func (s *Service) IndexFile(ctx context.Context, m *domain.MediaIndex) (int64, error) {
	return s.mediaIndex.Upsert(ctx, m)
}

// RemoveFile 删除单条索引（网盘文件删除联动，§9.3）。
func (s *Service) RemoveFile(ctx context.Context, sourceType, filePath string) error {
	return s.mediaIndex.DeleteBySourcePath(ctx, sourceType, filePath)
}
