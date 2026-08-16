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
	// [v7 §8.5] 网盘优先级列表（逗号分隔）：搜索结果按此顺序排序；未列出 source 沉底。
	ConfigPansearchPriority = "pansearch_priority"
	ConfigNASLocalPath      = "nas_local_path"
	// [V7 §9.7] NAS 多媒体源：JSON 数组字符串，存容器内子路径列表。
	// 示例：`["/mnt/nas-root/Asia-Movie","/mnt/nas-root/Western-Movie"]`
	// 由 admin 后台动态增删；扫描时遍历每条路径独立 Phase A/B，结果合并。
	ConfigNASLocalPaths = "nas_local_paths"
	// [V7 §9.7] NAS 父目录（容器内）：展示用，便于"浏览子目录"辅助选择路径。
	ConfigNASRootPath          = "nas_root_path"
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
	// [A3] §20 订阅自动搜寻间隔（天）
	ConfigSubscriptionSearchDays = "subscription_search_days"
	// [v7 §6.9.1] 转存根目录：pan_{driver}_save_root_{account_id}
	// [v7 §6.9.2] 全局转存后重命名开关
	// [v7 §6.9.3] 配额预警阈值 pan_{driver}_quota_warning_gb
	// [v7 §6.9.3] 清理模式 pan_{driver}_cleanup_mode
	// [v7 §6.9.3] 保留天数 pan_{driver}_cleanup_keep_recent_days

	// ConfigKeyPrefixPanSaveRoot  §6.9.1 转存根目录前缀（按账号）：pan_{driver}_save_root_{account_id}
	ConfigKeyPrefixPanSaveRoot = "pan_save_root_"
	// ConfigKeyPrefixPanQuotaWarn §6.9.3 配额预警阈值（按 driver）：pan_{driver}_quota_warning_gb
	ConfigKeyPrefixPanQuotaWarn = "pan_quota_warning_"
	// ConfigKeyPrefixPanCleanupMode §6.9.3 清理模式（按 driver）：pan_{driver}_cleanup_mode
	ConfigKeyPrefixPanCleanupMode = "pan_cleanup_mode_"
	// ConfigKeyPrefixPanCleanupKeep §6.9.3 保留天数（按 driver）：pan_{driver}_cleanup_keep_recent_days
	ConfigKeyPrefixPanCleanupKeep = "pan_cleanup_keep_recent_days_"
)

// ConfigDefaults 启动时缺失配置的默认值。
var ConfigDefaults = map[string]string{
	ConfigTMDBLanguage:           "zh-CN",
	ConfigResolvePriority:        `["nas","pan115","quark","pan123","baidu","guangya"]`,
	ConfigResolveMagnetEnabled:   "true",
	ConfigResolveMagnetTarget:    "pan115",
	ConfigPansearchURL:           "http://localhost:8888",
	ConfigPansearchAuthOn:        "false",
	ConfigPansearchCAMBlock:      "true",
	ConfigPansearch4KPriority:    "true",
	ConfigPansearchPriority:      "pan115,quark,pan123,baidu,guangya",
	ConfigNASFullScanDay:         "1",
	ConfigNASIncrementalDay:      "7",
	ConfigWebSocketEnabled:       "true",
	ConfigPanRenameEnabled:       "true",
	ConfigNASEnabled:             "true",
	ConfigResolveRateLimitMax:    "3",
	ConfigResolveRateLimitSec:    "30",
	ConfigMediaLibraryMaxRows:    "5000",
	ConfigMediaLibraryKeepRows:   "3000",
	ConfigDemoVideoURL:           "https://commondatastorage.googleapis.com/gtv-videos-bucket/sample/BigBuckBunny.mp4",
	ConfigResolveDemoFallback:    "true",
	ConfigResolveP0MinScore:      "0.6",
	ConfigSubscriptionSearchDays: "7",
}
