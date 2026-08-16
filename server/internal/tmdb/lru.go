package tmdb

import (
	"context"

	"xmedia/internal/domain"
)

// LRUProtectors 淘汰保护源（§15.2：收藏/订阅/有播放记录的内容不淘汰）。
type LRUProtectors struct {
	Favorites     domain.FavoriteRepository
	Subscriptions domain.SubscriptionRepository
	PlayHistory   domain.PlayHistoryRepository
}

// SetLRUProtectors 注入淘汰保护检查依赖（wire 装配调用）。
func (s *Service) SetLRUProtectors(p LRUProtectors) {
	s.protectors = p
}

// MaybeEvict 按 §15.2 执行 media_library LRU 淘汰：
// 总数超过 max_rows 时，按 last_accessed_at 升序淘汰到 keep_rows，
// 跳过收藏/订阅/有播放记录的条目。返回淘汰数量。
func (s *Service) MaybeEvict(ctx context.Context) int {
	if s.library == nil {
		return 0
	}
	maxRows := s.configInt(ctx, domain.ConfigMediaLibraryMaxRows, 5000)
	keepRows := s.configInt(ctx, domain.ConfigMediaLibraryKeepRows, 3000)
	if keepRows >= maxRows {
		keepRows = maxRows / 2
	}
	total, err := s.library.CountTotal(ctx)
	if err != nil || total <= maxRows {
		return 0
	}
	excess := total - keepRows
	candidates, err := s.library.ListForEviction(ctx, excess+100)
	if err != nil || len(candidates) == 0 {
		return 0
	}
	removed := 0
	for _, c := range candidates {
		if c == nil {
			continue
		}
		if s.isProtected(ctx, c) {
			continue
		}
		if err := s.library.Delete(ctx, c.ID); err != nil {
			continue
		}
		removed++
		if removed >= excess {
			break
		}
	}
	return removed
}

// isProtected 保护检查：收藏/订阅/播放记录任一命中即保护。
func (s *Service) isProtected(ctx context.Context, m *domain.MediaLibrary) bool {
	if s.protectors.Favorites != nil {
		if ok, err := s.protectors.Favorites.Exists(ctx, m.ExternalID, m.ExternalSource); err == nil && ok {
			return true
		}
	}
	if s.protectors.Subscriptions != nil {
		if ok, err := s.protectors.Subscriptions.Exists(ctx, m.ExternalID, m.ExternalSource); err == nil && ok {
			return true
		}
	}
	if s.protectors.PlayHistory != nil {
		if ok, err := s.protectors.PlayHistory.HasAny(ctx, m.ExternalID, m.ExternalSource); err == nil && ok {
			return true
		}
	}
	return false
}

func (s *Service) configInt(ctx context.Context, key string, def int) int {
	if s.configs == nil {
		return def
	}
	v, ok, err := s.configs.Get(ctx, key)
	if err != nil || !ok {
		return def
	}
	n := 0
	for _, r := range v {
		if r < '0' || r > '9' {
			return def
		}
		n = n*10 + int(r-'0')
	}
	return n
}
