package media

import (
	"context"

	"xmedia/internal/domain"
)

// Service 媒体库应用服务（历史/收藏/订阅/继续观看/搜索历史/可用性）。
type Service struct {
	history    domain.PlayHistoryRepository
	favorites  domain.FavoriteRepository
	subs       domain.SubscriptionRepository
	search     domain.SearchHistoryRepository
	mediaIndex domain.MediaIndexRepository
	library    domain.MediaLibraryRepository
}

// Options 媒体库服务依赖。
type Options struct {
	PlayHistory domain.PlayHistoryRepository
	Favorites   domain.FavoriteRepository
	Subscriptions domain.SubscriptionRepository
	SearchHistory domain.SearchHistoryRepository
	MediaIndex  domain.MediaIndexRepository
	MediaLibrary domain.MediaLibraryRepository
}

func NewService(opts Options) *Service {
	return &Service{
		history:    opts.PlayHistory,
		favorites:  opts.Favorites,
		subs:       opts.Subscriptions,
		search:     opts.SearchHistory,
		mediaIndex: opts.MediaIndex,
		library:    opts.MediaLibrary,
	}
}

// UpsertHistory 上报/更新播放进度。
func (s *Service) UpsertHistory(ctx context.Context, h *domain.PlayHistory) error {
	return s.history.Upsert(ctx, h)
}

func (s *Service) ListHistory(ctx context.Context, limit int) ([]*domain.PlayHistory, error) {
	if limit <= 0 {
		limit = 100
	}
	return s.history.List(ctx, limit)
}

func (s *Service) ContinueWatching(ctx context.Context, limit int) ([]*domain.PlayHistory, error) {
	if limit <= 0 {
		limit = 20
	}
	return s.history.ListContinueWatching(ctx, limit)
}

func (s *Service) DeleteHistory(ctx context.Context, externalID int64, source string, season, episode int) error {
	return s.history.DeleteByKey(ctx, externalID, source, season, episode)
}

func (s *Service) ClearHistory(ctx context.Context) error {
	return s.history.DeleteAll(ctx)
}

// --- 收藏 ---

func (s *Service) ListFavorites(ctx context.Context) ([]*domain.Favorite, error) {
	return s.favorites.List(ctx)
}

func (s *Service) AddFavorite(ctx context.Context, f *domain.Favorite) error {
	_, err := s.favorites.Add(ctx, f)
	return err
}

func (s *Service) RemoveFavorite(ctx context.Context, externalID int64, source string) error {
	return s.favorites.Remove(ctx, externalID, source)
}

func (s *Service) FavoriteExists(ctx context.Context, externalID int64, source string) (bool, error) {
	return s.favorites.Exists(ctx, externalID, source)
}

// --- 订阅 ---

func (s *Service) ListSubscriptions(ctx context.Context) ([]*domain.Subscription, error) {
	return s.subs.List(ctx)
}

func (s *Service) AddSubscription(ctx context.Context, sub *domain.Subscription) error {
	if sub.Status == "" {
		sub.Status = domain.SubWatching
	}
	if sub.MaxSearches == 0 {
		sub.MaxSearches = 12
	}
	_, err := s.subs.Add(ctx, sub)
	return err
}

func (s *Service) RemoveSubscription(ctx context.Context, externalID int64, source string) error {
	return s.subs.Remove(ctx, externalID, source)
}

func (s *Service) SubscriptionExists(ctx context.Context, externalID int64, source string) (bool, error) {
	return s.subs.Exists(ctx, externalID, source)
}

// --- 搜索历史 ---

func (s *Service) RecordSearch(ctx context.Context, keyword string) error {
	if keyword == "" {
		return nil
	}
	return s.search.Add(ctx, keyword)
}

func (s *Service) ListSearchHistory(ctx context.Context, limit int) ([]*domain.SearchHistory, error) {
	if limit <= 0 {
		limit = 20
	}
	return s.search.List(ctx, limit)
}

func (s *Service) ClearSearchHistory(ctx context.Context) error {
	return s.search.Clear(ctx)
}

// --- 可用性 ---

// CheckAvailability 批量返回已索引（可秒播）的键列表。
func (s *Service) CheckAvailability(ctx context.Context, items []domain.AvailabilityKey) ([]domain.AvailabilityKey, error) {
	return s.mediaIndex.AvailableKeys(ctx, items)
}

// MarkAvailable 标记某内容可播放（用于演示/索引后更新角标）。
func (s *Service) IndexedCount(ctx context.Context) (int, error) {
	return s.mediaIndex.Count(ctx)
}
