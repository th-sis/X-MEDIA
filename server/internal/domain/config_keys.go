package domain

// X-MEDIA 配置键常量。
const (
	ConfigTMDBAPIKey           = "tmdb_api_key"
	ConfigTMDBLanguage         = "tmdb_language"
	ConfigResolvePriority      = "resolve_priority"
	ConfigResolveMagnetEnabled = "resolve_magnet_enabled"
	ConfigResolveMagnetTarget  = "resolve_magnet_target"
	ConfigPansearchURL         = "pansearch_url"
	ConfigPansearchAuthOn      = "pansearch_auth_enabled"
	ConfigPansearchToken       = "pansearch_token"
	ConfigPansearchCAMBlock    = "pansearch_cam_block"
	ConfigPansearch4KPriority  = "pansearch_4k_priority"
	ConfigNASLocalPath         = "nas_local_path"
	ConfigNASFullScanDay       = "nas_index_full_scan_day"
	ConfigNASIncrementalDay    = "nas_index_incremental_day"
	ConfigWebSocketEnabled     = "websocket_enabled"
	ConfigBangumiAPIBase       = "bangumi_api_base"
	ConfigTicketSigningSecret  = "ticket_signing_secret"
	ConfigPanRenameEnabled     = "pan_rename_enabled"
	ConfigNASEnabled           = "nas_enabled"
	ConfigResolveRateLimitMax  = "resolve_rate_limit_max"
	ConfigResolveRateLimitSec  = "resolve_rate_limit_sec"
	ConfigMediaLibraryMaxRows  = "media_library_max_rows"
	ConfigMediaLibraryKeepRows = "media_library_keep_rows"
	ConfigDemoVideoURL         = "demo_video_url"
	// [v7 整改] 演示兜底开关：未配网盘/PanSou 时是否走演示播放
	ConfigResolveDemoFallback = "resolve_demo_fallback"
	// [v7 整改] P0 NAS 索引匹配度阈值，低于阈值直接跳 P0 进 P1
	ConfigResolveP0MinScore = "resolve_p0_min_score"
	// [v7 整改] 启动配置验证结果持久化（无需修改）
	// - 用 Get(validation_last_run) 查询最近一次运行时间
)

// ConfigDefaults 启动时缺失配置的默认值。
var ConfigDefaults = map[string]string{
	ConfigTMDBLanguage:         "zh-CN",
	ConfigResolvePriority:      `["nas","pan115","quark","pan123","baidu","guangya"]`,
	ConfigResolveMagnetEnabled: "true",
	ConfigResolveMagnetTarget:  "pan115",
	ConfigPansearchURL:         "http://localhost:8888",
	ConfigPansearchAuthOn:      "false",
	ConfigPansearchCAMBlock:    "true",
	ConfigPansearch4KPriority:  "true",
	ConfigNASFullScanDay:       "1",
	ConfigNASIncrementalDay:    "7",
	ConfigWebSocketEnabled:     "true",
	ConfigPanRenameEnabled:     "true",
	ConfigNASEnabled:           "true",
	ConfigResolveRateLimitMax:  "3",
	ConfigResolveRateLimitSec:  "30",
	ConfigMediaLibraryMaxRows:  "5000",
	ConfigMediaLibraryKeepRows: "3000",
	ConfigDemoVideoURL:         "https://commondatastorage.googleapis.com/gtv-videos-bucket/sample/BigBuckBunny.mp4",
	ConfigResolveDemoFallback:  "true",
	ConfigResolveP0MinScore:    "0.6",
}
