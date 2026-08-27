package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"xmedia/internal/auth"
	"xmedia/internal/automation"
	"xmedia/internal/cache"
	"xmedia/internal/cacheretention"
	"xmedia/internal/config"
	"xmedia/internal/driver"
	"xmedia/internal/domain"
	"xmedia/internal/eventbus"
	"xmedia/internal/file"
	"xmedia/internal/logx"
	"xmedia/internal/playback"
	"xmedia/internal/settings"
	"xmedia/internal/smbmount"
	"xmedia/internal/store"
	"xmedia/internal/tmdb"
	"xmedia/internal/upload"
	"xmedia/internal/websocket"
)

// App 按依赖顺序构造与关闭各子系统。
type App struct {
	cfg            config.Config
	logs           *logx.Manager
	log            *slog.Logger
	db             *store.DB
	store          *store.Store
	settings       *settings.Service
	bus            *eventbus.Bus
	cache          *cache.Service
	drivers        *driver.Manager
	auth           *auth.Service
	sched          *auth.Scheduler
	files          *file.Service
	uploads        *upload.Manager
	playback       *playback.Service
	automation     *automation.Service
	cacheRetention *cacheretention.Service
	tmdb           *tmdb.Service     // [P2#7] 启动时主动 LRU 检查
	hub            *websocket.Hub   // [P2#8] Shutdown 时广播 server_stopping
	// [V7 §9.4 UI-first] 容器内 SMB 挂载点服务 (特权 mount.cifs).
	smbMount       *smbmount.Service
	httpSrv        *http.Server
	httpBaseCancel context.CancelFunc
}

// Options 是构造 App 所需的外部依赖。
type Options struct {
	Config config.Config
	Logs   *logx.Manager
}

// New 按依赖顺序装配 App：目录 → DB(+迁移) → 仓储 → 驱动管理器 → 文件服务 → HTTP。
func New(ctx context.Context, opts Options) (*App, error) {
	logs := opts.Logs
	if logs == nil {
		logs = logx.NewDiscard()
	}
	log := logs.Root()
	cfg := opts.Config

	if err := prepareDataDirs(cfg); err != nil {
		return nil, err
	}

	stBundle, err := openStore(ctx, cfg, logs)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = stBundle.db.Close()
		}
	}()

	core, err := wireCore(ctx, cfg, logs, stBundle)
	if err != nil {
		return nil, err
	}
	svc := wireServices(cfg, logs, stBundle, core)
	xm := wireXMedia(stBundle, svc, core, logs)

	// [v7 整改] 启动序第 1.5 步：关键配置验证（不阻塞，失败仅记日志）
	runStartupValidation(ctx, stBundle)

	// [A1] §28.2 启动恢复：接管重启前遗留的 active 任务（HTTP 就绪前同步执行）
	if xm.resolve != nil {
		xm.resolve.RecoverStartup(context.Background())
	}
	// [A3] §20 订阅自动搜寻：后台周期执行（间隔可配，默认 7 天）
	if xm.subSearcher != nil {
		days := configInt(stBundle.store.Configs, domain.ConfigSubscriptionSearchDays, 7)
		xm.subSearcher.Start(context.Background(), time.Duration(days)*24*time.Hour)
	}
	// [V7 §9.4 UI-first] NAS source 有效性周期自监测（5 分钟 stat 一轮），
	// 列表页与 Capabilities 三态自动刷新，无需人工点「检测」。
	if xm.indexEngine != nil {
		xm.indexEngine.StartHealthMonitor(context.Background(), 5*time.Minute)
	}

	httpSrv, err := wireHTTPServer(cfg, logs, stBundle, core, svc, xm)
	if err != nil {
		return nil, err
	}
	httpBaseCtx, httpBaseCancel := context.WithCancel(context.Background())
	httpSrv.BaseContext = func(_ net.Listener) context.Context { return httpBaseCtx }

	log.Info("应用初始化完成", "db", cfg.DBPath, "log_level", logs.Level())
	return &App{
		cfg:            cfg,
		logs:           logs,
		log:            log,
		db:             stBundle.db,
		store:          stBundle.store,
		settings:       stBundle.settings,
		bus:            core.bus,
		cache:          core.cache,
		drivers:        core.drivers,
		auth:           core.auth,
		sched:          core.sched,
		files:          svc.files,
		uploads:        svc.uploads,
		playback:       svc.playback,
		automation:     svc.automation,
		cacheRetention: svc.cacheRetention,
		tmdb:           xm.tmdb, // [P2#7] 启动时主动 LRU 检查
		hub:            xm.hub,  // [P2#8] Shutdown 时广播 server_stopping
		smbMount:       xm.smbMount,
		httpSrv:        httpSrv,
		httpBaseCancel: httpBaseCancel,
	}, nil
}

// Run 启动 HTTP 服务并阻塞直到 ctx 取消或服务出错。
func (a *App) Run(ctx context.Context) error {
	if a.logs != nil && a.settings != nil {
		a.logs.StartAutoCleanup(ctx, a.settings.Int(settings.KeyLogRetentionDays))
	}
	if a.sched != nil && a.settings != nil {
		a.sched.InitActiveRefresh(ctx, a.settings.Bool(settings.KeyAuthActiveRefresh))
	}
	if a.cacheRetention != nil {
		a.cacheRetention.Start(ctx)
	}
	if a.automation != nil {
		a.automation.Start(ctx)
	}
	if a.uploads != nil {
		a.uploads.StartTempCleanup(ctx)
	}
	// [P2#7] 启动时主动检查一次 media_library LRU 淘汰：
	// 平时由 tmdb Upsert 后异步触发, 但首次启动 / 长时间运行后未做 Upsert
	// 的实例需要主动触发一次, 处理历史超额.
	if a.tmdb != nil {
		removed := a.tmdb.MaybeEvict(context.Background())
		if removed > 0 {
			a.log.Info("启动时 LRU 淘汰", "removed", removed)
		}
	}
	// [V7 §9.4 UI-first] 启动时把 DB 中已保存的 SMB 挂载点重新挂上
	// （特权 mount.cifs），取代部署侧 docker-compose bind-mount 手动配置。
	// 失败仅记日志，不阻塞 HTTP 启动；各挂载点状态由 RefreshState 校准。
	if a.smbMount != nil {
		if err := a.smbMount.ReattachOnStartup(context.Background()); err != nil {
			a.log.Warn("SMB 挂载点重挂失败", "err", err)
		}
	}
	errCh := make(chan error, 1)
	go func() {
		a.log.Info("HTTP 服务已监听", "addr", a.cfg.ListenAddr)
		if err := a.httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		a.log.Info("收到停止信号，准备关闭应用")
		return nil
	case err := <-errCh:
		return fmt.Errorf("http server: %w", err)
	}
}

const (
	shutdownHTTPBudget = 8 * time.Second
	shutdownBusBudget  = 3 * time.Second
)

// Shutdown 按依赖反序优雅关闭：先停 HTTP，再关事件总线，最后关 DB。
func (a *App) Shutdown(ctx context.Context) error {
	a.log.Info("正在优雅关闭各组件")
	if a.sched != nil {
		a.sched.Stop()
	}
	if a.uploads != nil {
		a.uploads.FlushPendingResume()
	}
	if a.httpBaseCancel != nil {
		a.httpBaseCancel()
	}

	// [P2#8] §28.4 在 HTTP 关闭前广播 server_stopping, 让 WS 客户端有 1-2s 时间
	// 收消息并弹"服务维护中"提示. Broadcast 是同步操作, 立即写到所有
	// 已连接 client 的 send channel, 客户端处理时我们才执行 HTTP.Shutdown.
	// 注意: 25s shutdownBudget 是 ctx 的总预算, broadcast 本身不耗时.
	if a.hub != nil {
		a.hub.Broadcast(websocket.TypeServerStopping, websocket.ServerStoppingPayload{
			Reason:        "graceful",
			RetryAfterSec: int(shutdownHTTPBudget.Seconds()),
		})
	}

	httpCtx, cancelHTTP := context.WithTimeout(ctx, shutdownHTTPBudget)
	err := a.httpSrv.Shutdown(httpCtx)
	cancelHTTP()
	if err != nil {
		a.log.Warn("HTTP 服务关闭异常", "err", err)
		if cerr := a.httpSrv.Close(); cerr != nil && !errors.Is(cerr, http.ErrServerClosed) {
			a.log.Warn("HTTP 服务强制关闭异常", "err", cerr)
		}
	}

	busCtx, cancelBus := context.WithTimeout(ctx, shutdownBusBudget)
	err = a.bus.Close(busCtx)
	cancelBus()
	if err != nil {
		a.log.Warn("事件总线关闭异常", "err", err)
	}
	snapshotCacheOnShutdown(a.cache, a.settings, a.cfg.DataDir)
	a.cache.Close()
	a.drivers.Close(ctx)
	if err := a.db.Close(); err != nil {
		return fmt.Errorf("close db: %w", err)
	}
	if err := a.logs.Close(ctx); err != nil {
		a.log.Warn("日志刷新异常", "err", err)
	}
	return nil
}
