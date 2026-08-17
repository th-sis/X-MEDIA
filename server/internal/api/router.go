package api

import (
	"compress/gzip"
	"embed"
	"io"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"xmedia/internal/account"
	"xmedia/internal/adminauth"
	"xmedia/internal/auth"
	"xmedia/internal/automation"
	"xmedia/internal/cache"
	"xmedia/internal/cacheretention"
	"xmedia/internal/crosstransfer"
	"xmedia/internal/domain"
	"xmedia/internal/eventbus"
	"xmedia/internal/file"
	"xmedia/internal/indexengine"
	"xmedia/internal/logx"
	"xmedia/internal/media"
	"xmedia/internal/notification"
	"xmedia/internal/offlinedownload"
	"xmedia/internal/pansearch"
	"xmedia/internal/playback"
	"xmedia/internal/resolve"
	"xmedia/internal/settings"
	"xmedia/internal/tmdb"
	"xmedia/internal/upload"
	"xmedia/internal/websocket"
)

//go:embed web
var webFS embed.FS

// Deps 是 API 层所需依赖（只依赖接口/服务，不感知具体存储实现）。
// §13.1 裁剪后移除 Strm/MediaOrganize/StrmScrape/Fuse/EmbyProxy/FnosProxy/ApiKeys/Favorites(旧)。
type Deps struct {
	Logs              *logx.Manager
	AccountSvc        *account.Service
	Accounts          domain.AccountRepository
	Configs           domain.ConfigRepository
	Settings          *settings.Service
	Cache             *cache.Service
	ListHitTracker    *cache.HitTracker
	Files             *file.Service
	Uploads           *upload.Manager
	OfflineDownloads  *offlinedownload.Service
	Playback          *playback.Service
	CacheRetention    *cacheretention.Service
	Automation        *automation.Service
	CrossTransfer     *crosstransfer.Service
	Auth              *auth.Service
	AuthSched         *auth.Scheduler
	AdminAuth         *adminauth.Service
	Notifications     *notification.Service
	DataDir           string
	OnSettingsUpdated func(map[string]string)
	// X-MEDIA 播放器 API 依赖
	TMDB            *tmdb.Service
	Media           *media.Service
	Resolve         *resolve.Service
	RateLimiter     *resolve.RateLimiter
	StreamProxy     *playback.StreamProxy
	Pansearch       *pansearch.Service
	IndexEngine     *indexengine.Service
	MediaIndex      domain.MediaIndexRepository
	Hub             *websocket.Hub
	Bus             *eventbus.Bus
	ServerVersion   string
	ServerStartedAt time.Time
	// LastRestartReason V7 §28.3：客户端感知重启（graceful/config_change/oom/panic）。
	LastRestartReason string
	// [V7 §9.4+ 扩展] NAS 媒体源仓储（G1.C：admin CRUD handler 用）。
	NASSources domain.NASSourceRepository
}

// Handler 持有处理请求所需的依赖。
type Handler struct {
	logs              *logx.Manager
	log               *slog.Logger
	accountSvc        *account.Service
	settings          *settings.Service
	cache             *cache.Service
	listHits          *cache.HitTracker
	files             *file.Service
	uploads           *upload.Manager
	offlineDownloads  *offlinedownload.Service
	playback          *playback.Service
	cacheRetention    *cacheretention.Service
	automation        *automation.Service
	crossTransfer     *crosstransfer.Service
	auth              *auth.Service
	authSched         *auth.Scheduler
	adminAuth         *adminauth.Service
	notifications     *notification.Service
	onSettingsUpdated func(map[string]string)
	// X-MEDIA 播放器 API
	tmdb            *tmdb.Service
	media           *media.Service
	resolveSvc      *resolve.Service
	rateLimiter     *resolve.RateLimiter
	streamProxy     *playback.StreamProxy
	pansearch       *pansearch.Service
	indexAdmin      *indexAdminHandlers
	configAdmin     *configAdminHandlers
	hub               *websocket.Hub
	configs           domain.ConfigRepository
	mediaIndex        domain.MediaIndexRepository
	serverVersion   string
	serverStartedAt time.Time
	lastRestartReason string
	// [V7 §9.4+ 扩展] NAS 媒体源仓储（G1.C：admin CRUD handler 用）。
	nasSources domain.NASSourceRepository
}

// NewRouter 装配并返回 HTTP 路由（含内嵌管理页面）。
func NewRouter(d Deps) http.Handler {
	apiLog := slog.Default()
	if d.Logs != nil {
		apiLog = d.Logs.For(logx.ModuleAPI)
	}
	h := &Handler{
		logs:              d.Logs,
		log:               apiLog,
		accountSvc:        d.AccountSvc,
		settings:          d.Settings,
		cache:             d.Cache,
		listHits:          d.ListHitTracker,
		files:             d.Files,
		uploads:           d.Uploads,
		offlineDownloads:  d.OfflineDownloads,
		playback:          d.Playback,
		cacheRetention:    d.CacheRetention,
		automation:        d.Automation,
		crossTransfer:     d.CrossTransfer,
		auth:              d.Auth,
		authSched:         d.AuthSched,
		adminAuth:         d.AdminAuth,
		notifications:     d.Notifications,
		onSettingsUpdated: d.OnSettingsUpdated,
		tmdb:              d.TMDB,
		media:             d.Media,
		resolveSvc:        d.Resolve,
		rateLimiter:       d.RateLimiter,
		streamProxy:       d.StreamProxy,
		pansearch:         d.Pansearch,
		hub:               d.Hub,
		configs:           d.Configs,
		mediaIndex:        d.MediaIndex,
		serverVersion:     d.ServerVersion,
		serverStartedAt:   d.ServerStartedAt,
		lastRestartReason: d.LastRestartReason,
		nasSources:        d.NASSources,
	}
	if d.IndexEngine != nil {
		h.indexAdmin = &indexAdminHandlers{engine: d.IndexEngine, index: d.MediaIndex}
	}
	h.configAdmin = &configAdminHandlers{configs: d.Configs, bus: d.Bus}

	r := chi.NewRouter()
	r.Use(trackResponseCommit)
	r.Use(chimw.RequestID)
	r.Use(h.attachRequestLogger)
	r.Use(chimw.Recoverer)

	r.Route("/api", func(r chi.Router) {
		r.Get("/health", h.health)
		r.Get("/auth/status", h.authStatus)
		r.Post("/auth/login", h.authLogin)
		r.Post("/auth/logout", h.authLogout)
		r.Post("/auth/reset-password", h.authResetPassword)

		// X-MEDIA 播放器 API（开放，无鉴权，便于开箱即测）
		r.Get("/capabilities", h.capabilities)
		r.Get("/state/snapshot", h.stateSnapshot)
		r.Get("/tmdb/home", h.tmdbHome)
		r.Get("/tmdb/discover", h.tmdbDiscover)
		r.Get("/tmdb/search", h.tmdbSearch)
		r.Get("/tmdb/detail/{id}", h.tmdbDetail)
		r.Get("/tmdb/seasons/{id}", h.tmdbSeasons)
		r.Get("/bangumi/search", h.bangumiSearch)
		r.Get("/bangumi/detail/{id}", h.bangumiDetail)
		r.Post("/resolve", h.resolveCreate)
		r.Get("/resolve/result/{id}", h.resolveResult)
		r.Get("/stream", h.streamHandler)
		r.Head("/stream", h.streamHandler)
		r.Route("/media", func(r chi.Router) {
			r.Get("/continue-watching", h.mediaContinueWatching)
			r.Get("/history", h.mediaHistoryList)
			r.Post("/history", h.mediaHistoryUpsert)
			r.Delete("/history", h.mediaHistoryClear)
			r.Delete("/history/{id}", h.mediaHistoryDelete)
			r.Get("/favorites", h.mediaFavoritesList)
			r.Post("/favorites", h.mediaFavoriteAdd)
			r.Delete("/favorites/{id}", h.mediaFavoriteRemove)
			r.Get("/subscriptions", h.mediaSubscriptionsList)
			r.Post("/subscriptions", h.mediaSubscriptionAdd)
			r.Delete("/subscriptions/{id}", h.mediaSubscriptionRemove)
			r.Get("/search-history", h.mediaSearchHistoryList)
			r.Delete("/search-history", h.mediaSearchHistoryClear)
			r.Post("/check-availability", h.mediaCheckAvailability)
		})

		r.Route("/public", func(r chi.Router) {
			r.Use(h.requirePublicOrAdmin)
			r.Get("/accounts", h.publicAccounts)
			r.Get("/system-config", h.publicSystemConfig)
			r.Get("/cache/hit-rate", h.publicCacheHitRate)
		})
		r.Route("/open", func(r chi.Router) {
			r.Post("/automation/events", h.automationWebhook)
		})
		r.Group(func(r chi.Router) {
			r.Use(h.requireAdmin)
			r.Route("/cross-transfer", func(r chi.Router) {
				r.Get("/routes", h.crossTransferRoutes)
				r.Post("/scan", h.crossTransferScan)
				r.Post("/scan/stream", h.crossTransferScanStream)
				r.Post("/probe", h.crossTransferProbe)
				r.Post("/execute", h.crossTransferExecute)
				r.Get("/relay/tasks", h.crossTransferRelayTasks)
				r.Get("/relay/tasks/stream", h.crossTransferRelayStream)
				r.Post("/relay/tasks/batch-delete", h.crossTransferRelayBatchDelete)
			})
			r.Get("/logs", h.listLogs)
			r.Get("/logs/stats", h.logStats)
			r.Post("/logs/ack-errors", h.ackRecentErrors)
			r.Post("/logs/cleanup", h.cleanupLogs)
			r.Post("/logs/cleanup/keep-today", h.cleanupLogsKeepToday)
			r.Post("/logs/cleanup/all", h.cleanupLogsAll)
			r.Route("/admin", func(r chi.Router) {
				r.Get("/system-config", h.adminSystemConfig)
				r.Post("/update-credentials", h.adminUpdateCredentials)
				r.Get("/local-fs/browse", h.browseLocalFS)
				r.Get("/drivers", h.listDrivers)
				r.Get("/accounts", h.listAccounts)
				r.Post("/accounts", h.createAccount)
				r.Get("/accounts/{id}", h.getAccount)
				r.Put("/accounts/{id}", h.updateAccount)
				r.Delete("/accounts/{id}", h.deleteAccount)
				r.Post("/accounts/{id}/toggle", h.toggleAccount)
				r.Post("/accounts/{id}/set-default", h.setDefaultAccount)
				r.Post("/accounts/{id}/refresh-auth", h.refreshAccountAuth)
				r.Get("/settings", h.getSettings)
				r.Put("/settings", h.updateSettings)
				r.Get("/cache/stats", h.cacheStats)
				r.Get("/cache/stats/{id}", h.accountCacheStats)
				r.Post("/clear-cache", h.clearCache)
				r.Route("/cache-retention", func(r chi.Router) {
					r.Get("/configs", h.listRetentionTasks)
					r.Get("/stats", h.getRetentionStats)
					r.Get("/defaults", h.retentionDefaults)
					r.Get("/startup", h.retentionStartupRemaining)
					r.Post("/configs", h.createRetentionTask)
					r.Put("/configs/{id}", h.updateRetentionTask)
					r.Delete("/configs/{id}", h.deleteRetentionTask)
					r.Post("/configs/{id}/toggle", h.toggleRetentionTask)
					r.Post("/configs/{id}/refresh", h.refreshRetentionTask)
					r.Post("/configs/{id}/force-stop", h.forceStopRetentionTask)
					r.Post("/configs/{id}/ack-scope-warn", h.ackRetentionScopeWarn)
				})
				r.Route("/index", func(r chi.Router) {
					if h.indexAdmin == nil {
						r.Get("/status", func(w http.ResponseWriter, _ *http.Request) {
							writeOK(w, map[string]any{"progress": nil, "indexed_total": 0})
						})
						return
					}
					r.Get("/status", h.indexAdmin.handleIndexStatus)
					r.Post("/nas/full", h.indexAdmin.handleIndexNASFull)
					r.Post("/nas/incremental", h.indexAdmin.handleIndexNASIncremental)
					r.Post("/rebuild/{account_id}", h.indexAdmin.handleIndexRebuild)
					r.Post("/cleanup/{account_id}", h.indexAdmin.handleIndexCleanup)
				})
				r.Route("/configs", func(r chi.Router) {
								r.Get("/", h.configAdmin.handleConfigsGet)
								r.Put("/", h.configAdmin.handleConfigsPut)
							})
							// [V7 §9.4+ 扩展 G1.C] NAS 媒体源 CRUD 端点（[V7 §9.4+ 多源扩展]）
							r.Route("/nas-sources", func(r chi.Router) {
								r.Get("/", h.listNASSources)
								r.Post("/", h.createNASSource)
								r.Put("/{id}", h.updateNASSource)
								r.Delete("/{id}", h.deleteNASSource)
								r.Post("/{id}/toggle", h.toggleNASSource)
								r.Get("/test-path", h.nasSourceTestPath)
								r.Post("/bulk-health", h.nasSourceBulkHealth)
							})
				// §1.4 Step 2：TMDB 配置专用端点（保存即测试 / 仅测试）
				r.Put("/tmdb/config", h.tmdbAdminConfig)
				r.Post("/tmdb/test", h.tmdbAdminTest)
				r.Get("/notifications", h.listNotifications)
				r.Get("/notifications/unread-count", h.notificationUnreadCount)
				r.Post("/notifications/read-all", h.markAllNotificationsRead)
				r.Delete("/notifications", h.deleteAllNotifications)
				r.Post("/notifications/{id}/read", h.markNotificationRead)
				r.Delete("/notifications/{id}", h.deleteNotification)
				r.Route("/automation", func(r chi.Router) {
					r.Get("/rules", h.listAutomationRules)
					r.Post("/rules", h.createAutomationRule)
					r.Put("/rules/{id}", h.updateAutomationRule)
					r.Delete("/rules/{id}", h.deleteAutomationRule)
					r.Post("/rules/{id}/toggle", h.toggleAutomationRule)
					r.Post("/rules/{id}/run", h.runAutomationRule)
					r.Post("/validate", h.validateAutomationRule)
					r.Get("/runs", h.listAutomationRuns)
					r.Post("/runs/clear", h.clearAutomationRuns)
					r.Get("/options", h.automationOptions)
				})
			})
			r.Post("/oauth/start", h.startOAuth)
			r.Get("/oauth/status/{session_id}", h.oauthStatus)
			r.Post("/oauth/confirm-received/{session_id}", h.oauthConfirmReceived)
			r.Post("/qr/start", h.startQRLogin)
			r.Post("/qr/poll", h.pollQRLogin)
		})
		r.Route("/files", func(r chi.Router) {
			r.Group(func(r chi.Router) {
				r.Use(h.requirePublicOrAdmin)
				r.Get("/list", h.listFiles)
				r.Get("/info", h.fileInfo)
				r.Get("/download", h.downloadFile)
				r.Head("/download", h.downloadFile)
			})
			r.Group(func(r chi.Router) {
				r.Use(h.requireAdmin)
				r.Delete("/delete", h.deleteFiles)
				r.Post("/move", h.moveFiles)
				r.Post("/copy", h.copyFiles)
				r.Put("/rename", h.renameFile)
				r.Post("/name-align/preview", h.previewNameAlign)
				r.Post("/name-align/apply", h.applyNameAlign)
				r.Post("/create-folder", h.createFolder)
				r.Post("/upload-task", h.createUploadTask)
				r.Get("/upload/runtime", h.getUploadRuntime)
				r.Put("/upload/runtime", h.updateUploadRuntime)
				r.Get("/upload/tasks", h.listUploadTasks)
				r.Get("/upload/tasks/stream", h.streamUploadTasks)
				r.Get("/upload/tasks/{taskID}", h.getUploadTask)
				r.Post("/upload/tasks/{taskID}/pause", h.pauseUploadTask)
				r.Post("/upload/tasks/{taskID}/resume", h.resumeUploadTask)
				r.Delete("/upload/tasks/{taskID}", h.deleteUploadTask)
				r.Post("/upload/tasks/batch-delete", h.batchDeleteUploadTasks)
				r.Route("/offline-download", func(r chi.Router) {
					r.Get("/capabilities", h.offlineDownloadCapabilities)
					r.Post("/urls", h.addOfflineURLs)
					r.Post("/torrent/prepare", h.prepareOfflineTorrent)
					r.Post("/torrent", h.addOfflineTorrent)
					r.Get("/tasks", h.listOfflineDownloadTasks)
					r.Post("/tasks/refresh", h.refreshOfflineDownloadTasks)
					r.Post("/tasks/batch-delete", h.batchDeleteOfflineDownloadTasks)
					r.Delete("/tasks/{taskID}", h.deleteOfflineDownloadTask)
				})
			})
		})
	})

	// WebSocket 端点（独立于 /api 前缀；需认证）
	r.Get("/ws", h.wsAuthMiddleware(h.wsHandle))

	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		panic(err) // 编译期内嵌，理论上不会失败
	}
	r.Handle("/*", spaHandler(sub))

	return r
}

func spaHandler(fsys fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(fsys))
	index, _ := fs.ReadFile(fsys, "index.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upath := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if upath == "" {
			upath = "index.html"
		}
		if _, statErr := fs.Stat(fsys, upath); statErr == nil {
			setStaticCacheHeader(w, upath)
			fileServer.ServeHTTP(w, r)
			return
		}
		if _, statErr := fs.Stat(fsys, upath+".gz"); statErr == nil {
			serveCompressedAsset(w, r, fsys, upath)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if r.Method != http.MethodHead {
			_, _ = w.Write(index)
		}
	})
}

func serveCompressedAsset(w http.ResponseWriter, r *http.Request, fsys fs.FS, upath string) {
	compressed, err := fsys.Open(upath + ".gz")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer compressed.Close()
	info, err := compressed.Stat()
	if err != nil {
		http.Error(w, "静态资源读取失败", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("Content-Type", staticContentType(upath))
	w.Header().Set("Vary", "Accept-Encoding")
	if acceptsGzip(r.Header.Get("Accept-Encoding")) {
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
		if r.Method != http.MethodHead {
			_, _ = io.Copy(w, compressed)
		}
		return
	}

	if r.Method == http.MethodHead {
		return
	}
	reader, err := gzip.NewReader(compressed)
	if err != nil {
		http.Error(w, "静态资源解压失败", http.StatusInternalServerError)
		return
	}
	defer reader.Close()
	_, _ = io.Copy(w, reader)
}

func setStaticCacheHeader(w http.ResponseWriter, upath string) {
	if strings.HasPrefix(upath, "assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
}

func staticContentType(name string) string {
	if contentType := mime.TypeByExtension(path.Ext(name)); contentType != "" {
		return contentType
	}
	switch path.Ext(name) {
	case ".mjs", ".js":
		return "text/javascript; charset=utf-8"
	case ".wasm":
		return "application/wasm"
	default:
		return "application/octet-stream"
	}
}

func acceptsGzip(header string) bool {
	wildcard := false
	for _, value := range strings.Split(header, ",") {
		parts := strings.Split(strings.TrimSpace(value), ";")
		coding := strings.ToLower(strings.TrimSpace(parts[0]))
		quality := 1.0
		for _, parameter := range parts[1:] {
			key, raw, found := strings.Cut(strings.TrimSpace(parameter), "=")
			if !found || !strings.EqualFold(key, "q") {
				continue
			}
			parsed, err := strconv.ParseFloat(raw, 64)
			if err != nil {
				quality = 0
			} else {
				quality = parsed
			}
		}
		if coding == "gzip" {
			return quality > 0
		}
		if coding == "*" {
			wildcard = quality > 0
		}
	}
	return wildcard
}
