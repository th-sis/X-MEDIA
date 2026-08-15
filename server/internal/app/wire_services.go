package app

import (
	"context"

	"xmedia/internal/account"
	"xmedia/internal/automation"
	"xmedia/internal/cacheretention"
	"xmedia/internal/config"
	"xmedia/internal/crosstransfer"
	"xmedia/internal/domain"
	"xmedia/internal/embyproxy"
	"xmedia/internal/favorites"
	"xmedia/internal/file"
	"xmedia/internal/fnosproxy"
	"xmedia/internal/fusemount"
	"xmedia/internal/fusereadcache"
	"xmedia/internal/logx"
	"xmedia/internal/mediaorganize"
	"xmedia/internal/offlinedownload"
	"xmedia/internal/playback"
	"xmedia/internal/settings"
	"xmedia/internal/strm"
	"xmedia/internal/strmscrape"
	"xmedia/internal/upload"
)

type servicesBundle struct {
	files            *file.Service
	uploads          *upload.Manager
	offlineDownloads *offlinedownload.Service
	playback         *playback.Service
	account          *account.Service
	strm             *strm.Service
	mediaOrganize    *mediaorganize.Service
	strmScrape       *strmscrape.Service
	automation       *automation.Service
	fuse             *fusemount.Service
	fuseReadCache    *fusereadcache.Service
	cacheRetention   *cacheretention.Service
	crossTransfer    *crosstransfer.Service
	embyProxy        *embyproxy.Service
	fnosProxy        *fnosproxy.Service
	favorites        *favorites.Service
}

func wireServices(cfg config.Config, logs *logx.Manager, st *storeBundle, core *coreBundle) *servicesBundle {
	favoritesSvc := favorites.NewService(cfg.DBPath, logs.For(logx.ModuleSystem))
	fileSvc := file.NewService(core.exec, core.cache, st.store.Accounts, core.bus, st.settings, core.listHits)
	fileSvc.SetLogger(logs.For(logx.ModuleFileOp))
	playbackSvc := playback.NewService(core.exec, core.cache)
	strmSvc, coord := wireSTRM(st, fileSvc, playbackSvc, core.bus, logs, cfg.DataDir, cfg.StrmDir, cfg.ListenAddr, core.secret)
	core.strm = coord
	retentionSvc, retentionCoord := wireCacheRetention(st, fileSvc, core.cache, core.bus, logs)
	mediaOrganizeSvc := wireMediaOrganize(st, fileSvc, logs, cfg.DataDir)
	strmScrapeSvc := strmscrape.New(strmscrape.Options{
		Strm:     strmSvc,
		Settings: st.settings,
		Bus:      core.bus,
		DataDir:  cfg.DataDir,
		StrmDir:  cfg.StrmDir,
		Log:      logs.For(logx.ModuleSystem),
	})
	strmSvc.SetOrganizeBusyChecker(mediaOrganizeSvc)
	strmSvc.SetRetentionBusyChecker(retentionSvc)
	retentionSvc.SetStrmBusyChecker(strmSvc)
	retentionSvc.SetOrganizeBusyChecker(mediaOrganizeSvc)
	fuseReadCache := wireFuseReadCacheOrNil(context.Background(), cfg, logs, st, core.bus)
	offlineDownloadSvc := offlinedownload.New(offlinedownload.Options{
		Exec:     core.exec,
		Accounts: st.store.Accounts,
		Repo:     st.store.OfflineDownloads,
		Bus:      core.bus,
		Log:      logs.For(logx.ModuleFileOp),
	})
	fuseSvc := fusemount.New(fusemount.Options{
		Repo:      st.store.FuseMounts,
		Configs:   st.store.Configs,
		Accounts:  st.store.Accounts,
		Notify:    st.store.Notifications,
		Files:     fileSvc,
		Playback:  playbackSvc,
		ReadCache: fuseReadCache,
		Bus:       core.bus,
		Log:       logs.For(logx.ModuleSystem),
	})
	_ = fuseSvc.PrepareMountRoot()
	lifecycle := accountLifecycle{
		fuse:      fuseSvc,
		readCache: fuseReadCache,
		strm:      coord,
		retention: retentionCoord,
		media:     mediaOrganizeSvc,
		favorites: favoritesSvc,
		offline:   offlineDownloadSvc,
	}
	accountSvc := account.NewService(account.Options{
		Accounts:      st.store.Accounts,
		AuthStates:    st.store.AuthStates,
		Drivers:       core.drivers,
		Auth:          core.auth,
		Playback:      playbackSvc,
		MetadataCache: core.cache,
		Lifecycle:     lifecycle,
		OAuthURL: func(context.Context) string {
			return domain.NormalizeOAuthServerURL(st.settings.String(settings.KeyOAuthServerURL))
		},
	})
	uploadSvc := upload.NewManager(upload.Options{
		Exec:     core.exec,
		Files:    fileSvc,
		Playback: playbackSvc,
		Accounts: accountSvc,
		Repo:     st.store.UploadTasks,
		Settings: st.settings,
		Bus:      core.bus,
		DataDir:  cfg.DataDir,
		Log:      logs.For(logx.ModuleFileOp),
	})
	fuseSvc.SetUploads(uploadSvc)
	crossTransferSvc := crosstransfer.New(crosstransfer.Options{
		Exec:    core.exec,
		Files:   fileSvc,
		Uploads: uploadSvc,
		Log:     logs.For(logx.ModuleAPI),
	})
	embyProxySvc := embyproxy.New(embyproxy.Options{
		Settings: st.settings,
		Playback: playbackSvc,
		Strm:     strmSvc,
		Log:      logs.For(logx.ModuleSystem),
	})
	fnosProxySvc := fnosproxy.New(fnosproxy.Options{
		Settings: st.settings,
		Playback: playbackSvc,
		Strm:     strmSvc,
		StrmDir:  cfg.StrmDir,
		Log:      logs.For(logx.ModuleSystem),
	})
	automationSvc := automation.New(automation.Options{
		Rules:      st.store.AutomationRules,
		Runs:       st.store.AutomationRuns,
		Strm:       strmSvc,
		StrmScrape: strmScrapeSvc,
		Organize:   mediaOrganizeSvc,
		Emby:       embyProxySvc,
		Files:      fileSvc,
		Log:        logs.For(logx.ModuleSystem),
	})
	automationSvc.Register(core.bus)
	strmSvc.SetAutomationManagedChecker(automationSvc.IsStrmTaskManaged)
	return &servicesBundle{
		files:            fileSvc,
		uploads:          uploadSvc,
		offlineDownloads: offlineDownloadSvc,
		playback:         playbackSvc,
		account:          accountSvc,
		strm:             strmSvc,
		mediaOrganize:    mediaOrganizeSvc,
		strmScrape:       strmScrapeSvc,
		automation:       automationSvc,
		fuse:             fuseSvc,
		fuseReadCache:    fuseReadCache,
		cacheRetention:   retentionSvc,
		crossTransfer:    crossTransferSvc,
		embyProxy:        embyProxySvc,
		fnosProxy:        fnosProxySvc,
		favorites:        favoritesSvc,
	}
}
