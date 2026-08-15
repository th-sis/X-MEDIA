package app

import (
	"xmedia/internal/cache"
	"xmedia/internal/cacheretention"
	"xmedia/internal/eventbus"
	"xmedia/internal/file"
	"xmedia/internal/logx"
)

func wireCacheRetention(st *storeBundle, files *file.Service, cacheSvc *cache.Service, bus *eventbus.Bus, logs *logx.Manager) (*cacheretention.Service, *cacheretention.Coordinator) {
	svc := cacheretention.NewService(cacheretention.Options{
		Repo:     st.store.CacheRetentionTasks,
		Accounts: st.store.Accounts,
		Files:    files,
		Cache:    cacheSvc,
		Settings: st.settings,
		Bus:      bus,
		Log:      logs.For(logx.ModuleCache),
	})
	coord := cacheretention.NewCoordinator(svc)
	coord.Register(bus)
	return svc, coord
}
