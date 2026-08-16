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
	hub        *websocket.Hub

	workerCount int

	mu       sync.Mutex
	scanning bool
	progress NASProgress
	lastScan time.Time
}

// NASProgress 当前/最近一次 NAS 扫描进度（WS index_status 推送源，§9.7.1）。
type NASProgress struct {
	Scope       string `json:"scope"`
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
		hub:         opts.Hub,
		workerCount: workers,
	}
}

// NASPath 读取 NAS 本地路径配置（§9.4，configs nas_local_path）。
func (s *Service) NASPath(ctx context.Context) string {
	if s.configs == nil {
		return ""
	}
	v, ok, err := s.configs.Get(ctx, domain.ConfigNASLocalPath)
	if err != nil || !ok {
		return ""
	}
	return v
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
func (s *Service) ScanNASFull(ctx context.Context) error {
	path := s.NASPath(ctx)
	if path == "" {
		return domain.Errorf(domain.CodeValidation, "未配置 NAS 路径")
	}
	go s.scanNAS(context.Background(), path, false)
	return nil
}

// ScanNASIncremental 触发 NAS 增量扫描（§9.7.4：仅扫描 mtime 新于上次扫描的文件 + 清理消失文件）。
func (s *Service) ScanNASIncremental(ctx context.Context) error {
	path := s.NASPath(ctx)
	if path == "" {
		return domain.Errorf(domain.CodeValidation, "未配置 NAS 路径")
	}
	go s.scanNAS(context.Background(), path, true)
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
