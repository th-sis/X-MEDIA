package indexengine

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"xmedia/internal/domain"
)

// scanNAS 执行 NAS 扫描（§9.7）：Phase A 路径发现 -> Phase B 元数据提取（worker pool）
// -> Phase C 孤儿标记。incremental=true 时 Phase A 仅收录 mtime 新于上次扫描的文件，
// 且额外执行消失文件清理（Phase D）。
//
// [V7 §9.7] 多媒体源遍历：每条 root 独立 Phase A/B；Phase C 跨路径全局（unconfirmed
// 标记不区分路径）；Phase D 增量清理按路径独立处理。单条路径失败不影响其他路径。
func (s *Service) scanNAS(ctx context.Context, roots []string, incremental bool) {
	s.mu.Lock()
	if s.scanning {
		s.mu.Unlock()
		return
	}
	s.scanning = true
	s.mu.Unlock()

	started := time.Now()
	defer func() {
		s.mu.Lock()
		s.scanning = false
		s.lastScan = started
		s.mu.Unlock()
	}()

	if len(roots) == 0 {
		s.pushProgress(NASProgress{Scope: "nas", Phase: "A", Status: "failed", ErrorMsg: "无 NAS 媒体源"})
		return
	}

	// 跨路径累积
	var (
		totalProcessed   atomic.Int64
		totalMatched     atomic.Int64
		totalUnconfirmed atomic.Int64
		totalOrphaned    atomic.Int64
	)
	totalFiles := 0

	// ---- Phase A + B 逐路径处理 ----
	for _, root := range roots {
		rootName := filepath.Base(root)
		if rootName == "" || rootName == "/" || rootName == "." {
			rootName = root // 兜底
		}
		// 单条路径 Phase A
		s.pushProgress(NASProgress{Scope: "nas", Root: rootName, Phase: "A", Status: "running"})
		paths, err := s.discoverPhaseA(ctx, root, incremental)
		if err != nil {
			s.pushProgress(NASProgress{
				Scope: "nas", Root: rootName, Phase: "A", Status: "failed", ErrorMsg: err.Error(),
			})
			continue // 单条路径失败不影响其他路径
		}
		n := len(paths)
		totalFiles += n
		if n == 0 {
			s.pushProgress(NASProgress{
				Scope: "nas", Root: rootName, Phase: "A", Status: "done", Total: 0,
			})
			continue
		}

		// 单条路径 Phase B
		s.pushProgress(NASProgress{
			Scope: "nas", Root: rootName, Phase: "B", Status: "running", Total: n,
		})
		batch := make([]string, n)
		copy(batch, paths)

		var (
			processed   atomic.Int64
			matched     atomic.Int64
			unconfirmed atomic.Int64
			orphaned    atomic.Int64
		)
		workerCount := s.workerCount
		if workerCount > n {
			workerCount = n
		}
		var wg sync.WaitGroup
		jobs := make(chan string)
		for i := 0; i < workerCount; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for path := range jobs {
					m, un, _ := s.processFile(ctx, root, path)
					processed.Add(1)
					if m {
						matched.Add(1)
					} else if un {
						unconfirmed.Add(1)
					} else {
						orphaned.Add(1)
					}
				}
			}()
		}
		go func() {
			for _, p := range batch {
				select {
				case <-ctx.Done():
					close(jobs)
					return
				case jobs <- p:
				}
			}
			close(jobs)
		}()

		// 进度推送（每条路径独立快照，2s 节流）
		done := make(chan struct{})
		go func() {
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()
			lastSent := int64(0)
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					cur := processed.Load()
					if cur == lastSent {
						continue
					}
					lastSent = cur
					rate := int(float64(cur) / time.Since(started).Seconds())
					s.pushProgress(NASProgress{
						Scope: "nas", Root: rootName, Phase: "B", Status: "running",
						Processed: int(cur), Total: n,
						Matched: int(matched.Load()), Unconfirmed: int(unconfirmed.Load()),
						Orphaned: int(orphaned.Load()), RatePerSec: rate,
					})
				}
			}
		}()
		wg.Wait()
		close(done)

		proc := int(processed.Load())
		rate := int(float64(proc) / time.Since(started).Seconds())
		totalProcessed.Add(int64(proc))
		totalMatched.Add(matched.Load())
		totalUnconfirmed.Add(unconfirmed.Load())
		totalOrphaned.Add(orphaned.Load())
		s.pushProgress(NASProgress{
			Scope: "nas", Root: rootName, Phase: "B", Status: "done",
			Processed: proc, Total: n,
			Matched: int(matched.Load()), Unconfirmed: int(unconfirmed.Load()),
			Orphaned: int(orphaned.Load()), RatePerSec: rate,
		})
	}

	// 推送全局汇总进度（Root="" 表示汇总）
	s.pushProgress(NASProgress{
		Scope: "nas", Root: "", Phase: "B", Status: "done",
		Processed: int(totalProcessed.Load()), Total: totalFiles,
		Matched: int(totalMatched.Load()), Unconfirmed: int(totalUnconfirmed.Load()),
		Orphaned: int(totalOrphaned.Load()),
	})

	if incremental {
		// 增量扫描：每条路径独立清理消失文件
		for _, root := range roots {
			rootName := filepath.Base(root)
			if rootName == "" || rootName == "/" || rootName == "." {
				rootName = root
			}
			s.cleanupMissingFiles(ctx, rootName, root)
		}
	}
	s.runPhaseC(ctx)
}

// discoverPhaseA 递归遍历根目录，产出候选视频文件路径（§9.7.1 Phase A）。
func (s *Service) discoverPhaseA(ctx context.Context, root string, incremental bool) ([]string, error) {
	var since time.Time
	if incremental {
		since = s.LastScanAt()
	}
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // 跳过不可读条目（SMB 瞬时断开容忍）
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if d.IsDir() {
			return nil
		}
		if !IsVideoFile(d.Name()) {
			return nil
		}
		if incremental && !since.IsZero() {
			info, err := d.Info()
			if err != nil || !info.ModTime().After(since) {
				return nil
			}
		}
		out = append(out, path)
		return nil
	})
	if err != nil && ctx.Err() == nil {
		return nil, err
	}
	return out, ctx.Err()
}

// processFile 单文件处理：清洗 -> 匹配 -> 写索引。返回 (matched, unconfirmed, orphaned) 归类。
func (s *Service) processFile(ctx context.Context, root, path string) (bool, bool, bool) {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		rel = path
	}
	clean := CleanFilename(path)
	if clean.Title == "" {
		return false, false, true
	}
	var matchedMedia *domain.MediaLibrary
	var score float64
	if s.library != nil {
		candidates, _ := s.library.SearchByTitle(ctx, clean.Title, 10)
		mr := MatchTitle(clean, candidates)
		matchedMedia = mr.Media
		score = mr.Score
	}
	m := &domain.MediaIndex{
		ExternalSource: "tmdb",
		Season:         clean.Season,
		Episode:        clean.Episode,
		MediaType:      clean.Type,
		Title:          clean.Title,
		Year:           clean.Year,
		SourceType:     "nas",
		FilePath:       rel,
		FileFormat:     clean.Format,
		MatchStatus:    domain.MatchOrphaned,
	}
	if matchedMedia != nil {
		m.ExternalID = matchedMedia.ExternalID
		m.ExternalSource = matchedMedia.ExternalSource
		m.MediaType = matchedMedia.MediaType
		m.MatchStatus = domain.MatchMatched
		m.MatchScore = score
	} else if clean.Title != "" && s.library == nil {
		m.MatchStatus = domain.MatchUnconfirmed
		m.MatchScore = 0.6
	} else if clean.Title != "" {
		// 有标题但未命中缓存：待 TMDB 匹配（P0-4 后真实搜索），暂记 unconfirmed
		m.MatchStatus = domain.MatchUnconfirmed
		m.MatchScore = 0.5
	}
	if info, err := os.Stat(path); err == nil {
		m.FileSize = info.Size()
	}
	if _, err := s.mediaIndex.Upsert(ctx, m); err != nil {
		// [B 实测修复] Upsert 失败不再静默：计入孤儿并透出错误信息
		return false, false, true
	}
	switch m.MatchStatus {
	case domain.MatchMatched:
		return true, false, false
	case domain.MatchUnconfirmed:
		return false, true, false
	default:
		return false, false, true
	}
}

// runPhaseC 孤儿标记（§9.7.1 Phase C）：unconfirmed 超过 30 天 -> orphaned。
func (s *Service) runPhaseC(ctx context.Context) {
	prev := s.Progress()
	s.pushProgress(NASProgress{
		Scope: "nas", Phase: "C", Status: "running",
		Processed: prev.Processed, Total: prev.Total,
		Matched: prev.Matched, Unconfirmed: prev.Unconfirmed, Orphaned: prev.Orphaned,
	})
	before := time.Now().Add(-30 * 24 * time.Hour)
	rows, err := s.mediaIndex.ListUnconfirmedBefore(ctx, before)
	if err != nil {
		s.pushProgress(NASProgress{Scope: "nas", Phase: "C", Status: "failed", ErrorMsg: err.Error()})
		return
	}
	ids := make([]int64, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	if err := s.mediaIndex.MarkOrphaned(ctx, ids); err != nil {
		s.pushProgress(NASProgress{Scope: "nas", Phase: "C", Status: "failed", ErrorMsg: err.Error()})
		return
	}
	prev = s.Progress()
	s.pushProgress(NASProgress{
		Scope: "nas", Phase: "C", Status: "done",
		Processed: prev.Processed, Total: prev.Total,
		Matched: prev.Matched, Unconfirmed: prev.Unconfirmed,
		Orphaned: prev.Orphaned + len(ids),
	})
}

// cleanupMissingFiles 增量扫描的 Phase D（§9.7.4）：删除索引中已不存在的文件。
// [V7 §9.7] rootName 用于进度推送展示（basename）。
func (s *Service) cleanupMissingFiles(ctx context.Context, rootName, root string) {
	items, err := s.mediaIndex.ListBySource(ctx, "nas", 0)
	if err != nil {
		return
	}
	removed := 0
	for _, item := range items {
		full := filepath.Join(root, filepath.FromSlash(item.FilePath))
		if _, err := os.Stat(full); os.IsNotExist(err) {
			_ = s.mediaIndex.DeleteBySourcePath(ctx, "nas", item.FilePath)
			removed++
		}
	}
	s.pushProgress(NASProgress{Scope: "nas", Root: rootName, Phase: "D", Status: "done", Orphaned: removed})
}
