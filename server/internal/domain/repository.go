package domain

import (
	"context"
	"time"
)



type AccountRepository interface {
	Create(ctx context.Context, a *Account) (int64, error)
	Update(ctx context.Context, a *Account) error
	Delete(ctx context.Context, id int64) error
	Get(ctx context.Context, id int64) (*Account, error)
	List(ctx context.Context) ([]*Account, error)
	SetDefault(ctx context.Context, id int64) error
	// NameTaken 检查名称是否已被其它账号占用（大小写不敏感；excludeID>0 时排除该账号）。
	NameTaken(ctx context.Context, name string, excludeID int64) (bool, error)
}

type AuthStateRepository interface {
	Get(ctx context.Context, accountID int64) (*AuthState, error)
	Upsert(ctx context.Context, s *AuthState) error
	Delete(ctx context.Context, accountID int64) error
}

type ConfigRepository interface {
	Get(ctx context.Context, key string) (value string, ok bool, err error)
	Set(ctx context.Context, key, value string) error
	All(ctx context.Context) (map[string]string, error)
}

type NotificationRepository interface {
	Create(ctx context.Context, n *Notification) (int64, error)
	List(ctx context.Context, limit, offset int) ([]*Notification, error)
	UnreadCount(ctx context.Context) (int, error)
	MarkRead(ctx context.Context, id int64) error
	MarkAllRead(ctx context.Context) (int64, error)
	Delete(ctx context.Context, id int64) error
	DeleteAll(ctx context.Context) (int64, error)
	DeleteByRef(ctx context.Context, category string, refID int64) (int64, error)
}

// AvailabilityKey 标识一条可播放内容（季集维度）。
type AvailabilityKey struct {
	ExternalID     int64 `json:"external_id"`
	ExternalSource string `json:"external_source"`
	Season         int   `json:"season"`
	Episode        int   `json:"episode"`
}

type MediaIndexRepository interface {
	Upsert(ctx context.Context, m *MediaIndex) (int64, error)
	// FindBest 按 external_id + season/episode 查询最佳命中（P0）。
	FindBest(ctx context.Context, externalID int64, source string, season, episode int) (*MediaIndex, error)
	// AvailableKeys 返回 items 中已索引的键列表（可用性角标）。
	AvailableKeys(ctx context.Context, items []AvailabilityKey) ([]AvailabilityKey, error)
	Count(ctx context.Context) (int, error)
	ListBySource(ctx context.Context, sourceType string, accountID int64) ([]*MediaIndex, error)
	DeleteBySourcePath(ctx context.Context, sourceType, filePath string) error
}

type MediaLibraryRepository interface {
	Upsert(ctx context.Context, m *MediaLibrary) (int64, error)
	Get(ctx context.Context, externalID int64, source string) (*MediaLibrary, error)
	Touch(ctx context.Context, externalID int64, source string) error
	// ListForEviction 按 last_accessed_at 升序返回候选淘汰列表。
	ListForEviction(ctx context.Context, limit int) ([]*MediaLibrary, error)
	// IsProtected 判断该条目是否被收藏/订阅/有播放记录（LRU 保护）。
	CountTotal(ctx context.Context) (int, error)
	Delete(ctx context.Context, id int64) error
}

type PlayHistoryRepository interface {
	Upsert(ctx context.Context, h *PlayHistory) error
	Get(ctx context.Context, externalID int64, source string, season, episode int) (*PlayHistory, error)
	List(ctx context.Context, limit int) ([]*PlayHistory, error)
	ListContinueWatching(ctx context.Context, limit int) ([]*PlayHistory, error)
	DeleteByKey(ctx context.Context, externalID int64, source string, season, episode int) error
	DeleteAll(ctx context.Context) error
	HasAny(ctx context.Context, externalID int64, source string) (bool, error)
}

type FavoriteRepository interface {
	Add(ctx context.Context, f *Favorite) (int64, error)
	Remove(ctx context.Context, externalID int64, source string) error
	List(ctx context.Context) ([]*Favorite, error)
	Exists(ctx context.Context, externalID int64, source string) (bool, error)
}

type SubscriptionRepository interface {
	Add(ctx context.Context, s *Subscription) (int64, error)
	Remove(ctx context.Context, externalID int64, source string) error
	List(ctx context.Context) ([]*Subscription, error)
	UpdateStatus(ctx context.Context, id int64, status SubStatus, resultSource string, resultAccountID int64, resultPath string) error
	Exists(ctx context.Context, externalID int64, source string) (bool, error)
	ActiveCount(ctx context.Context) (int, error)
}

type PansearchCacheRepository interface {
	Get(ctx context.Context, keyword, cloudTypes string) (results string, linkCount int, cachedAt *time.Time, err error)
	Set(ctx context.Context, keyword, cloudTypes, results string, linkCount int) error
	MarkStale(ctx context.Context, keyword, cloudTypes string) error
	Delete(ctx context.Context, keyword string) error
}

type ResolveTaskRepository interface {
	Create(ctx context.Context, t *ResolveTask) (int64, error)
	Get(ctx context.Context, id int64) (*ResolveTask, error)
	// FindActiveByKey 返回同 (external_id, source, season, episode) 的运行中任务。
	FindActiveByKey(ctx context.Context, externalID int64, source string, season, episode int) (*ResolveTask, error)
	Update(ctx context.Context, t *ResolveTask) error
	ListActive(ctx context.Context) ([]*ResolveTask, error)
}

type SearchHistoryRepository interface {
	Add(ctx context.Context, keyword string) error
	List(ctx context.Context, limit int) ([]*SearchHistory, error)
	Clear(ctx context.Context) error
}

type RateLimitRepository interface {
	Count(ctx context.Context, clientIP string, windowStart time.Time) (int, error)
	Increment(ctx context.Context, clientIP string, windowStart time.Time) error
	Cleanup(ctx context.Context, before time.Time) error
}
