package app

import (
	"context"
	"strconv"
	"strings"

	"xmedia/internal/account"
	"xmedia/internal/domain"
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
}

func wireXMedia(st *storeBundle, svc *servicesBundle, logs *logx.Manager) *xmediaBundle {
	tmdbSvc := tmdb.NewService(st.store.Configs, st.store.MediaLibrary)
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
		PansearchHealth: pansearchSvc.Health,
		LoggedInDrivers: loggedInDriversFn(svc.account),
		NASConfigured:   nasConfiguredFn(st.store.Configs),
		Signer:          signer,
		Hub:             hub,
		ServerVersion:   xmediaVersion,
	})

	maxReq := configInt(st.store.Configs, domain.ConfigResolveRateLimitMax, 3)
	winSec := configInt(st.store.Configs, domain.ConfigResolveRateLimitSec, 30)
	rateLimiter := resolve.NewRateLimiter(st.store.RateLimits, maxReq, winSec)

	return &xmediaBundle{
		tmdb:        tmdbSvc,
		media:       mediaSvc,
		pansearch:   pansearchSvc,
		streamProxy: streamProxy,
		hub:         hub,
		resolve:     resolveSvc,
		rateLimiter: rateLimiter,
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
