package app

import (
	"context"
	"net/http"
	"time"

	"xmedia/internal/adminauth"
	"xmedia/internal/api"
	"xmedia/internal/cache"
	"xmedia/internal/config"
	"xmedia/internal/domain"
	"xmedia/internal/logx"
	"xmedia/internal/notification"
	"xmedia/internal/settings"
)

func wireHTTPServer(cfg config.Config, logs *logx.Manager, st *storeBundle, core *coreBundle, svc *servicesBundle) (*http.Server, error) {
	notifySvc := notification.NewService(notification.Options{
		Repo:     st.store.Notifications,
		Accounts: st.store.Accounts,
		Log:      logs.For(logx.ModuleSystem),
	})
	notifySvc.Register(core.bus)

	xm := wireXMedia(st, svc, core, logs)
	// [A1] §28.2 启动恢复：接管重启前遗留的 active 任务（HTTP 就绪前同步执行）
	if xm.resolve != nil {
		xm.resolve.RecoverStartup(context.Background())
	}
	// [A3] §20 订阅自动搜寻：后台周期执行（间隔可配，默认 7 天）
	if xm.subSearcher != nil {
		days := configInt(st.store.Configs, domain.ConfigSubscriptionSearchDays, 7)
		xm.subSearcher.Start(context.Background(), time.Duration(days)*24*time.Hour)
	}
	router := api.NewRouter(api.Deps{
		Logs:              logs,
		AccountSvc:        svc.account,
		Accounts:          st.store.Accounts,
		Configs:           st.store.Configs,
		Settings:          st.settings,
		Cache:             core.cache,
		ListHitTracker:    core.listHits,
		Files:             svc.files,
		Uploads:           svc.uploads,
		OfflineDownloads:  svc.offlineDownloads,
		Playback:          svc.playback,
		CacheRetention:    svc.cacheRetention,
		Automation:        svc.automation,
		CrossTransfer:     svc.crossTransfer,
		Auth:              core.auth,
		AuthSched:         core.sched,
		AdminAuth:         adminauth.New(st.store.Configs, core.secret, logs.For(logx.ModuleAPI)),
		Notifications:     notifySvc,
		DataDir:           cfg.DataDir,
		OnSettingsUpdated: cacheSettingsHook(core.cache, st.settings, cfg.DataDir),
		TMDB:              xm.tmdb,
		Media:             xm.media,
		Resolve:           xm.resolve,
		RateLimiter:       xm.rateLimiter,
		StreamProxy:       xm.streamProxy,
		Pansearch:         xm.pansearch,
		IndexEngine:       xm.indexEngine,
		MediaIndex:        st.store.MediaIndex,
		Hub:               xm.hub,
		ServerVersion:     xmediaVersion,
		ServerStartedAt:   time.Now(),
	})

	return &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}, nil
}

func cacheSettingsHook(cacheSvc *cache.Service, settingsSvc *settings.Service, dataDir string) func(map[string]string) {
	return func(changed map[string]string) {
		if !settingsTouchesCache(changed) {
			return
		}
		applyCacheRuntime(cacheSvc, settingsSvc, dataDir)
	}
}
