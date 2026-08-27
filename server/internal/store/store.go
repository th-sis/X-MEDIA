package store

import "xmedia/internal/domain"

// Store 聚合各仓储实现，作为上层注入的统一入口。
type Store struct {
	DB                  *DB
	Accounts            domain.AccountRepository
	AuthStates          domain.AuthStateRepository
	Configs             domain.ConfigRepository
	Notifications       domain.NotificationRepository
	UploadTasks         domain.UploadTaskRepository
	OfflineDownloads    domain.OfflineDownloadTaskRepository
	CacheRetentionTasks domain.CacheRetentionTaskRepository
	AutomationRules     domain.AutomationRuleRepository
	AutomationRuns      domain.AutomationRunRepository
	MediaIndex          domain.MediaIndexRepository
	MediaLibrary        domain.MediaLibraryRepository
	PlayHistory         domain.PlayHistoryRepository
	Favorites           domain.FavoriteRepository
	Subscriptions       domain.SubscriptionRepository
	PansearchCache      domain.PansearchCacheRepository
	ResolveTasks        domain.ResolveTaskRepository
	SearchHistory       domain.SearchHistoryRepository
	RateLimits          domain.RateLimitRepository
	NASSources          domain.NASSourceRepository
	// [V7 §9.4 UI-first] 容器内 SMB 挂载点持久化.
	SMBMounts           domain.SMBMountRepository
}

// New 基于已打开的 DB 构造仓储集合。
// §13.1 裁剪后移除 ApiKeys/StrmTasks/StrmBranches/MediaOrganizeTasks/FuseMounts 仓储。
func New(db *DB) *Store {
	return &Store{
		DB:                  db,
		Accounts:            &accountRepo{db: db},
		AuthStates:          &authStateRepo{db: db},
		Configs:             &configRepo{db: db},
		Notifications:       &notificationRepo{db: db},
		UploadTasks:         &uploadTaskRepo{db: db},
		OfflineDownloads:    &offlineDownloadTaskRepo{db: db},
		CacheRetentionTasks: &cacheRetentionRepo{db: db},
		AutomationRules:     &automationRuleRepo{db: db},
		AutomationRuns:      &automationRunRepo{db: db},
		MediaIndex:          &mediaIndexRepo{db: db},
		MediaLibrary:        &mediaLibraryRepo{db: db},
		PlayHistory:         &playHistoryRepo{db: db},
		Favorites:           &favoriteRepo{db: db},
		Subscriptions:       &subscriptionRepo{db: db},
		PansearchCache:      &pansearchCacheRepo{db: db},
		ResolveTasks:        &resolveTaskRepo{db: db},
		SearchHistory:       &searchHistoryRepo{db: db},
		RateLimits:          &rateLimitRepo{db: db},
		NASSources:          &nasSourceRepo{db: db},
		// [V7 §9.4 UI-first] 容器内 SMB 挂载点持久化 (特权 mount.cifs).
		SMBMounts:           &smbMountRepo{db: db},
	}
}
