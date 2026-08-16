package app

import (
	"context"
	"strconv"
	"strings"

	"xmedia/internal/account"
	"xmedia/internal/domain"
	"xmedia/internal/driver"
	"xmedia/internal/eventbus"
	"xmedia/internal/indexengine"
	"xmedia/internal/logx"
	"xmedia/internal/media"
	"xmedia/internal/pansearch"
	"xmedia/internal/playback"
	"xmedia/internal/resolve"
	"xmedia/internal/tmdb"
	"xmedia/internal/websocket"
)

const xmediaVersion = "7.0.0"

// xmediaBundle 聚合 X-MEDIA 播放器侧的运行时服务。
type xmediaBundle struct {
	tmdb        *tmdb.Service
	media       *media.Service
	pansearch   *pansearch.Service
	streamProxy *playback.StreamProxy
	hub         *websocket.Hub
	resolve     *resolve.Service
	rateLimiter *resolve.RateLimiter
	// [P0-3] 索引引擎（§9）：NAS 三阶段扫描 + 匹配 + 增量维护
	indexEngine *indexengine.Service
	// [A3] §20 订阅自动搜寻器
	subSearcher *resolve.SubscriptionSearcher
}

func wireXMedia(st *storeBundle, svc *servicesBundle, core *coreBundle, logs *logx.Manager) *xmediaBundle {
	tmdbSvc := tmdb.NewService(st.store.Configs, st.store.MediaLibrary)
	// [A4] §15.2 LRU 淘汰保护源（收藏/订阅/播放记录）
	tmdbSvc.SetLRUProtectors(tmdb.LRUProtectors{
		Favorites:     st.store.Favorites,
		Subscriptions: st.store.Subscriptions,
		PlayHistory:   st.store.PlayHistory,
	})
	mediaSvc := media.NewService(media.Options{
		PlayHistory:   st.store.PlayHistory,
		Favorites:     st.store.Favorites,
		Subscriptions: st.store.Subscriptions,
		SearchHistory: st.store.SearchHistory,
		MediaIndex:    st.store.MediaIndex,
		MediaLibrary:  st.store.MediaLibrary,
	})
	pansearchSvc := pansearch.NewService(st.store.Configs, st.store.PansearchCache)
	signer := playback.NewTicketSigner(st.store.Configs)
	streamProxy := playback.NewStreamProxy(signer, svc.playback, st.store.Configs)
	hub := websocket.NewHub()

	resolveSvc := resolve.NewService(resolve.Options{
		Tasks:           st.store.ResolveTasks,
		MediaIndex:      st.store.MediaIndex,
		Subscriptions:   st.store.Subscriptions,
		Configs:         st.store.Configs,
		MediaLibrary:    st.store.MediaLibrary,
		PansearchHealth: pansearchSvc.Health,
		LoggedInDrivers: loggedInDriversFn(svc.account),
		NASConfigured:   nasConfiguredFn(st.store.Configs),
		// [v7 整改] 智能跳过 P0：查询索引条数
		IndexCount: func(ctx context.Context) (int, error) {
			return st.store.MediaIndex.Count(ctx)
		},
		// [v7 整改] P2 磁力兜底：PanSou 搜索 magnet/ed2k
		PansearchSearch: pansearchSvc.Search,
		// [P0-2] P1 盘搜：链接有效性批量检测
		PansearchCheck: pansearchSvc.CheckLinks,
		// [P0-2] P1 转存：按账号取注入好凭据的驱动实例
		DriverGet: func(ctx context.Context, accountID int64) (driver.Driver, error) {
			return core.drivers.Get(ctx, accountID)
		},
		// [P0-2] P1 转存：认证状态为 active 的账号列表
		Accounts: activeAccountsFn(svc.account),
		// [v7 整改] 配置开关读取
		MagnetEnabled: configBool(st.store.Configs, domain.ConfigResolveMagnetEnabled, true),
		DemoFallback:  configBool(st.store.Configs, domain.ConfigResolveDemoFallback, true),
		P0MinScore:    configFloat(st.store.Configs, domain.ConfigResolveP0MinScore, 0.6),
		Signer:        signer,
		Hub:           hub,
		ServerVersion: xmediaVersion,
	})

	maxReq := configInt(st.store.Configs, domain.ConfigResolveRateLimitMax, 3)
	winSec := configInt(st.store.Configs, domain.ConfigResolveRateLimitSec, 30)
	rateLimiter := resolve.NewRateLimiter(st.store.RateLimits, maxReq, winSec)

	// [P0-3] 索引引擎（§9）
	indexEngine := indexengine.NewService(indexengine.Options{
		MediaIndex:   st.store.MediaIndex,
		MediaLibrary: st.store.MediaLibrary,
		Configs:      st.store.Configs,
		Hub:          hub,
		WorkerCount:  8,
	})

	// [A3] §20 订阅自动搜寻器（复用 resolve 的 P1 轻量探测）
	subSearcher := resolve.NewSubscriptionSearcher(resolve.SubscriptionSearcherOptions{
		Subscriptions: st.store.Subscriptions,
		Probe:         resolveSvc.ProbeAvailability,
		Hub:           hub,
		Log:           logs.For("subscription-searcher"),
	})

	// V7 §11.1.1：配置变更（resolve_priority / nas_enabled / magnet_* 等）发布
	// eventbus.ConfigChanged，Hub 监听后通过 WS 推 capabilities_changed 消息。
	eventbus.Subscribe(core.bus, func(_ context.Context, evt eventbus.ConfigChanged) {
		hub.Broadcast("config_changed", map[string]any{
			"key":           evt.Key,
			"capabilities":  resolveSvc.Capabilities(context.Background()),
		})
	})

	return &xmediaBundle{
		tmdb:        tmdbSvc,
		media:       mediaSvc,
		pansearch:   pansearchSvc,
		streamProxy: streamProxy,
		hub:         hub,
		resolve:     resolveSvc,
		rateLimiter: rateLimiter,
		indexEngine: indexEngine,
		subSearcher: subSearcher,
	}
}

func loggedInDriversFn(accountSvc *account.Service) func(ctx context.Context) []string {
	return func(ctx context.Context) []string {
		if accountSvc == nil {
			return nil
		}
		views, err := accountSvc.List(ctx)
		if err != nil {
			return nil
		}
		var out []string
		for _, v := range views {
			if v.AuthStatus == domain.AuthActive {
				out = append(out, driverSourceName(v.Account.DriverType))
			}
		}
		return out
	}
}

// activeAccountsFn 返回认证状态为 active 的账号列表（P1 转存候选，§11.1 运行时跳过未登录）。
func activeAccountsFn(accountSvc *account.Service) func(ctx context.Context) []domain.Account {
	return func(ctx context.Context) []domain.Account {
		if accountSvc == nil {
			return nil
		}
		views, err := accountSvc.List(ctx)
		if err != nil {
			return nil
		}
		out := make([]domain.Account, 0, len(views))
		for _, v := range views {
			if v.AuthStatus == domain.AuthActive && v.Account != nil && v.Account.IsActive {
				out = append(out, *v.Account)
			}
		}
		return out
	}
}

// driverSourceName 把 LitePan 驱动名映射为 X-MEDIA source_type 命名。
func driverSourceName(driverType string) string {
	switch strings.ToLower(driverType) {
	case "115_open", "115":
		return "pan115"
	case "123_open", "123":
		return "pan123"
	case "baidu_open", "baidu":
		return "baidu"
	case "quark":
		return "quark"
	case "guangya":
		return "guangya"
	case "localfs", "local":
		return "nas"
	default:
		return strings.ToLower(driverType)
	}
}

func nasConfiguredFn(configs domain.ConfigRepository) func(ctx context.Context) bool {
	return func(ctx context.Context) bool {
		if configs == nil {
			return false
		}
		enabled := true
		if v, ok, err := configs.Get(ctx, domain.ConfigNASEnabled); err == nil && ok && strings.TrimSpace(v) != "" {
			enabled = v == "true" || v == "1"
		}
		path := ""
		if v, ok, err := configs.Get(ctx, domain.ConfigNASLocalPath); err == nil && ok {
			path = v
		}
		return enabled && strings.TrimSpace(path) != ""
	}
}

func configInt(configs domain.ConfigRepository, key string, def int) int {
	if configs == nil {
		return def
	}
	v, ok, err := configs.Get(context.Background(), key)
	if err != nil || !ok || strings.TrimSpace(v) == "" {
		return def
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return def
	}
	return n
}

// configBool 读取布尔配置（接受 true/1/yes/on）。
func configBool(configs domain.ConfigRepository, key string, def bool) func(context.Context) bool {
	return func(ctx context.Context) bool {
		if configs == nil {
			return def
		}
		v, ok, err := configs.Get(ctx, key)
		if err != nil || !ok || strings.TrimSpace(v) == "" {
			return def
		}
		s := strings.ToLower(strings.TrimSpace(v))
		return s == "true" || s == "1" || s == "yes" || s == "on"
	}
}

// configFloat 读取浮点配置。
func configFloat(configs domain.ConfigRepository, key string, def float64) func(context.Context) float64 {
	return func(ctx context.Context) float64 {
		if configs == nil {
			return def
		}
		v, ok, err := configs.Get(ctx, key)
		if err != nil || !ok || strings.TrimSpace(v) == "" {
			return def
		}
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			return def
		}
		return f
	}
}
