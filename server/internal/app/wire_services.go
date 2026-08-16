package app

import (
	"context"

	"xmedia/internal/account"
	"xmedia/internal/automation"
	"xmedia/internal/cacheretention"
	"xmedia/internal/config"
	"xmedia/internal/crosstransfer"
	"xmedia/internal/domain"
	"xmedia/internal/file"
	"xmedia/internal/logx"
	"xmedia/internal/offlinedownload"
	"xmedia/internal/playback"
	"xmedia/internal/settings"
	"xmedia/internal/upload"
)

type servicesBundle struct {
	files            *file.Service
	uploads          *upload.Manager
	offlineDownloads *offlinedownload.Service
	playback         *playback.Service
	account          *account.Service
	automation       *automation.Service
	cacheRetention   *cacheretention.Service
	crossTransfer    *crosstransfer.Service
}

func wireServices(cfg config.Config, logs *logx.Manager, st *storeBundle, core *coreBundle) *servicesBundle {
	fileSvc := file.NewService(core.exec, core.cache, st.store.Accounts, core.bus, st.settings, core.listHits)
	fileSvc.SetLogger(logs.For(logx.ModuleFileOp))
	playbackSvc := playback.NewService(core.exec, core.cache)
	retentionSvc, retentionCoord := wireCacheRetention(st, fileSvc, core.cache, core.bus, logs)
	offlineDownloadSvc := offlinedownload.New(offlinedownload.Options{
		Exec:     core.exec,
		Accounts: st.store.Accounts,
		Repo:     st.store.OfflineDownloads,
		Bus:      core.bus,
		Log:      logs.For(logx.ModuleFileOp),
	})
	lifecycle := accountLifecycle{
		retention: retentionCoord,
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
	crossTransferSvc := crosstransfer.New(crosstransfer.Options{
		Exec:    core.exec,
		Files:   fileSvc,
		Uploads: uploadSvc,
		Log:     logs.For(logx.ModuleAPI),
	})
	automationSvc := automation.New(automation.Options{
		Rules:   st.store.AutomationRules,
		Runs:    st.store.AutomationRuns,
		Configs: st.store.Configs,
		Files:   fileSvc,
		Log:     logs.For(logx.ModuleSystem),
	})
	automationSvc.Register(core.bus)
	return &servicesBundle{
		files:            fileSvc,
		uploads:          uploadSvc,
		offlineDownloads: offlineDownloadSvc,
		playback:         playbackSvc,
		account:          accountSvc,
		automation:       automationSvc,
		cacheRetention:   retentionSvc,
		crossTransfer:    crossTransferSvc,
	}
}
