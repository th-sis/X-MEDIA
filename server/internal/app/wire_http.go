package app

import (
	"context"
	"net/http"
	"os"
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

// restartReason V7 §28.3：根据启动环境判定本次进程的重启原因。
// 优先级：env XMEDIA_RESTART_REASON > 默认 graceful。
// - graceful：用户主动 docker-compose restart / Ctrl-C / kill SIGTERM
// - config_change：env 显式注入（运维/部署脚本识别为配置变更）
// - oom / panic：由崩溃监控外部注入
func restartReason() string {
	if v := os.Getenv("XMEDIA_RESTART_REASON"); v != "" {
		return v
	}
	return "graceful"
}

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
		// [V7 §9.4+ 扩展 G1.C] NAS 媒体源仓储接线（admin CRUD 7 端点依赖）
		NASSources:        st.store.NASSources,
		Hub:               xm.hub,
		Bus:               core.bus,
		ServerVersion:      xmediaVersion,
		ServerStartedAt:    time.Now(),
		LastRestartReason: restartReason(),
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
