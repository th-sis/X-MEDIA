package resolve

import (
	"context"
	"time"

	"xmedia/internal/domain"
	"xmedia/internal/driver"
	"xmedia/internal/pansearch"
	"xmedia/internal/playback"
	"xmedia/internal/websocket"
)

// Service 四层播放引擎。
type Service struct {
	tasks           domain.ResolveTaskRepository
	mediaIndex      domain.MediaIndexRepository
	subs            domain.SubscriptionRepository
	configs         domain.ConfigRepository
	mediaLibrary    domain.MediaLibraryRepository
	pansearchHealth func(ctx context.Context) bool
	loggedInDrivers func(ctx context.Context) []string
	nasConfigured   func(ctx context.Context) bool
	// [v7 整改] P0 智能跳过：检测索引是否为空
	indexCountFn func(ctx context.Context) (int, error)
	// [v7 §6.3] P0 智能跳过：检测是否扫描中（避免扫描中 P0 查询 miss）
	indexScanningFn func() bool
	// [V7 §9.7] 索引引擎真实状态（Capabilities 三态化用）：scanning/phase/processed/total
	indexStatusFn func() (scanning bool, phase string, processed, total int)
	// [V7 §9.4] NAS 路径已知列表（Capabilities not_accessible 检测用）
	nasPathsKnown func() []string
	// [V7 §9.4] NAS 单个路径 stat 检测（Capabilities 三态化用，nil 时兜底视作不可访问）
	pathStatFn func(path string) bool
	// [P0-2] P1 盘搜：调用 PanSou 搜索 share 链接
	pansearchSearch func(ctx context.Context, req pansearch.SearchRequest) ([]domain.PanSearchResult, error)
	// [P0-2] P1 盘搜：批量链接有效性检测（§8.4）
	pansearchCheck func(ctx context.Context, items []pansearch.CheckItem) ([]pansearch.CheckResult, error)
	// [P0-2] P1 转存：按账号取注入好凭据的驱动实例
	driverGet func(ctx context.Context, accountID int64) (driver.Driver, error)
	// [P0-2] P1 转存：认证状态为 active 的账号列表
	accountsFn func(ctx context.Context) []domain.Account
	// [v7 整改] 配置开关读取
	magnetEnabledFn func(ctx context.Context) bool
	demoFallbackFn  func(ctx context.Context) bool
	p0MinScoreFn    func(ctx context.Context) float64
	nasPathsStat    func() map[string]bool
	nasSourcesCount func() (int, int)
	signer          *playback.TicketSigner
	hub             *websocket.Hub
	serverVersion   string
}

// Options 播放引擎依赖。
type Options struct {
	Tasks           domain.ResolveTaskRepository
	MediaIndex      domain.MediaIndexRepository
	Subscriptions   domain.SubscriptionRepository
	Configs         domain.ConfigRepository
	MediaLibrary    domain.MediaLibraryRepository
	PansearchHealth func(ctx context.Context) bool
	LoggedInDrivers func(ctx context.Context) []string
	NASConfigured   func(ctx context.Context) bool
	IndexCount      func(ctx context.Context) (int, error)
	// [V7 §6.3] P0 智能跳过：检测是否扫描中
	IndexScanning func() bool
	// [V7 §9.7] 索引引擎状态：scanning/phase/processed/total
	IndexStatus func() (scanning bool, phase string, processed, total int)
	// [V7 §9.4] NAS 路径列表（Capabilities 三态化用）
	NASPathsKnown func() []string
	// NASPathsStat 列出每条 source 的可访问性（Capabilities 用，¥7 §9.4+ 扩展 G1.E）。
	// 返回的 map key = source path，value = 是否可访问（stat 探测）。
	NASPathsStat func() map[string]bool
	// NASSourcesCount 返回 (total, enabled) source 数（Capabilities 用）。
	NASSourcesCount func() (total int, enabled int)
	// [V7 §9.4] NAS 路径 stat 检测
	PathStat        func(path string) bool
	PansearchSearch func(ctx context.Context, req pansearch.SearchRequest) ([]domain.PanSearchResult, error)
	PansearchCheck  func(ctx context.Context, items []pansearch.CheckItem) ([]pansearch.CheckResult, error)
	DriverGet       func(ctx context.Context, accountID int64) (driver.Driver, error)
	Accounts        func(ctx context.Context) []domain.Account
	MagnetEnabled   func(ctx context.Context) bool
	DemoFallback    func(ctx context.Context) bool
	P0MinScore      func(ctx context.Context) float64
	Signer          *playback.TicketSigner
	Hub             *websocket.Hub
	ServerVersion   string
}

func NewService(opts Options) *Service {
	if opts.LoggedInDrivers == nil {
		opts.LoggedInDrivers = func(context.Context) []string { return nil }
	}
	if opts.NASConfigured == nil {
		opts.NASConfigured = func(context.Context) bool { return false }
	}
	if opts.PansearchHealth == nil {
		opts.PansearchHealth = func(context.Context) bool { return false }
	}
	if opts.IndexCount == nil {
		opts.IndexCount = func(context.Context) (int, error) { return 0, nil }
	}
	if opts.IndexScanning == nil {
		opts.IndexScanning = func() bool { return false }
	}
	if opts.IndexStatus == nil {
		opts.IndexStatus = func() (bool, string, int, int) { return false, "", 0, 0 }
	}
	if opts.NASPathsKnown == nil {
		opts.NASPathsKnown = func() []string { return nil }
	}
	if opts.NASPathsStat == nil {
		opts.NASPathsStat = func() map[string]bool { return nil }
	}
	if opts.NASSourcesCount == nil {
		opts.NASSourcesCount = func() (int, int) { return 0, 0 }
	}
	if opts.PathStat == nil {
		opts.PathStat = func(string) bool { return false }
	}
	if opts.PansearchSearch == nil {
		opts.PansearchSearch = func(context.Context, pansearch.SearchRequest) ([]domain.PanSearchResult, error) {
			return nil, nil
		}
	}
	if opts.PansearchCheck == nil {
		opts.PansearchCheck = func(context.Context, []pansearch.CheckItem) ([]pansearch.CheckResult, error) {
			return nil, nil
		}
	}
	if opts.DriverGet == nil {
		opts.DriverGet = func(context.Context, int64) (driver.Driver, error) {
			return nil, domain.Errf(domain.CodeInternal)
		}
	}
	if opts.Accounts == nil {
		opts.Accounts = func(context.Context) []domain.Account { return nil }
	}
	if opts.MagnetEnabled == nil {
		opts.MagnetEnabled = func(context.Context) bool { return true }
	}
	if opts.DemoFallback == nil {
		opts.DemoFallback = func(context.Context) bool { return true }
	}
	if opts.P0MinScore == nil {
		opts.P0MinScore = func(context.Context) float64 { return 0.6 }
	}
	if opts.ServerVersion == "" {
		opts.ServerVersion = "7.0.0"
	}
	return &Service{
		tasks:           opts.Tasks,
		mediaIndex:      opts.MediaIndex,
		subs:            opts.Subscriptions,
		configs:         opts.Configs,
		mediaLibrary:    opts.MediaLibrary,
		pansearchHealth: opts.PansearchHealth,
		loggedInDrivers: opts.LoggedInDrivers,
		nasConfigured:   opts.NASConfigured,
		indexCountFn:    opts.IndexCount,
		nasPathsStat:    opts.NASPathsStat,
		nasSourcesCount: opts.NASSourcesCount,
		pansearchSearch: opts.PansearchSearch,
		pansearchCheck:  opts.PansearchCheck,
		driverGet:       opts.DriverGet,
		accountsFn:      opts.Accounts,
		magnetEnabledFn: opts.MagnetEnabled,
		demoFallbackFn:  opts.DemoFallback,
		p0MinScoreFn:    opts.P0MinScore,
		signer:          opts.Signer,
		hub:             opts.Hub,
		serverVersion:   opts.ServerVersion,
	}
}

// Request 播放请求。
type Request struct {
	ExternalID     int64  `json:"external_id"`
	ExternalSource string `json:"external_source"`
	MediaType      string `json:"media_type"`
	Title          string `json:"title"`
	Year           int    `json:"year"`
	Season         int    `json:"season"`
	Episode        int    `json:"episode"`
}

// Result 触发结果。
type Result struct {
	TaskID int64 `json:"task_id"`
	Reused bool  `json:"reused"`
}

// Resolve 触发播放引擎，异步运行四层流程。
func (s *Service) Resolve(ctx context.Context, req Request) (*Result, error) {
	if active, err := s.tasks.FindActiveByKey(ctx, req.ExternalID, req.ExternalSource, req.Season, req.Episode); err == nil && active != nil {
		return &Result{TaskID: active.ID, Reused: true}, nil
	}

	t := &domain.ResolveTask{
		ExternalID:     req.ExternalID,
		ExternalSource: req.ExternalSource,
		MediaType:      req.MediaType,
		Title:          req.Title,
		Year:           req.Year,
		Season:         req.Season,
		Episode:        req.Episode,
		Status:         domain.ResolveRunning,
		Stage:          domain.StageResolveStart,
		StageDetail:    "正在准备...",
	}
	id, err := s.tasks.Create(ctx, t)
	if err != nil {
		return nil, err
	}
	t.ID = id
	go s.runEngine(context.Background(), t)
	return &Result{TaskID: id, Reused: false}, nil
}

func (s *Service) push(t *domain.ResolveTask, stage domain.ResolveStage, detail string, pct int) {
	t.Stage = stage
	t.StageDetail = detail
	t.ProgressPct = pct
	if err := s.tasks.Update(context.Background(), t); err == nil && s.hub != nil {
		s.hub.Broadcast(websocket.TypeResolveStage, websocket.ResolveStagePayload{
			TaskID:      t.ID,
			ExternalID:  t.ExternalID,
			Stage:       string(stage),
			Detail:      detail,
			ProgressPct: pct,
		})
	}
	time.Sleep(220 * time.Millisecond)
}

func (s *Service) runEngine(ctx context.Context, t *domain.ResolveTask) {
	s.push(t, domain.StageResolveStart, "正在准备...", 5)

	// P0: NAS 本地索引（[v7 整改] 智能跳过：未配置/索引为空时直接跳过）
	skipP0Reason := s.shouldSkipP0(ctx)
	if skipP0Reason == "" && s.nasConfigured(ctx) && s.mediaIndex != nil {
		s.push(t, domain.StageNASLookup, "查询本地索引...", 15)
		if hit, err := s.mediaIndex.FindBest(ctx, t.ExternalID, t.ExternalSource, t.Season, t.Episode); err == nil && hit != nil {
			minScore := s.p0MinScoreFn(ctx)
			if hit.MatchScore >= minScore {
				ticket, err := s.signer.Sign(ctx, ticketClaimsFor(t, hit.FileID, hit.SourceType, hit.AccountID), 0)
				if err == nil {
					s.push(t, domain.StageNASHit, "本地命中 ✓", 50)
					s.complete(t, hit.SourceType, hit.FileID, ticket, hit.Title)
					return
				}
			} else {
				s.push(t, domain.StageNASLookup, "本地匹配度过低，跳过", 18)
			}
		}
	} else if skipP0Reason != "" {
		t.StageDetail = "P0 已跳过：" + skipP0Reason
		_ = s.tasks.Update(context.Background(), t)
	}

	// P1: 盘搜 + 分享转存（[P0-2] 真实链路：Search → CheckLinks → ShareSaver → 索引 → ticket）
	panAvailable := s.pansearchHealth(ctx)
	if panAvailable && s.runP1(ctx, t) {
		return
	}

	// P2: 磁力兜底（[P0-2] 真实链路：PanSou 搜 magnet → 115 离线下载 → 轮询进度）
	if s.magnetEnabledFn(ctx) && panAvailable && s.runP2(ctx, t) {
		return
	}

	// 演示兜底：[v7 整改] 走完前面所有层仍无资源，且无任何外部能力时，开关允许才进演示
	noExternalCapability := len(s.loggedInDrivers(ctx)) == 0 && !s.pansearchHealth(ctx) && !s.nasConfigured(ctx)
	if noExternalCapability && s.demoFallbackFn(ctx) {
		s.push(t, domain.StageResolvingLink, "演示模式回放...", 92)
		ticket, err := s.signer.Sign(ctx, playback.TicketClaims{
			TaskID:     t.ID,
			Source:     "demo",
			FileID:     "demo",
			ExternalID: t.ExternalID,
		}, 0)
		if err == nil {
			s.completeWithTag(t, "demo", "demo", ticket, t.Title, "演示")
			return
		}
	}

	// P3: 真实未命中 -> 创建订阅
	s.notFound(t)
}

func (s *Service) complete(t *domain.ResolveTask, source, fileID, ticket, fileName string) {
	s.completeWithTag(t, source, fileID, ticket, fileName, "")
}

func (s *Service) completeWithTag(t *domain.ResolveTask, source, fileID, ticket, fileName, tag string) {
	t.Status = domain.ResolveDone
	t.Stage = domain.StagePlayReady
	if tag != "" {
		t.StageDetail = "播放就绪 ✓（" + tag + "）"
	} else {
		t.StageDetail = "播放就绪 ✓"
	}
	t.ProgressPct = 100
	t.ResultSource = source
	t.ResultFileID = fileID
	_ = s.tasks.Update(context.Background(), t)
	if s.hub != nil {
		s.hub.Broadcast(websocket.TypeResolveComplete, websocket.ResolveCompletePayload{
			TaskID:    t.ID,
			StreamURL: "/api/stream?ticket=" + ticket,
			Source:    source,
			FileName:  fileName,
			FileID:    fileID,
			Ticket:    ticket,
		})
	}
}

// ResultOf 查询任务结果（轮询兜底）。
func (s *Service) ResultOf(ctx context.Context, taskID int64) (*domain.ResolveTask, error) {
	return s.tasks.Get(ctx, taskID)
}

// ActiveCount 返回 active（running/pending）任务数（/api/state/snapshot 用）。
func (s *Service) ActiveCount(ctx context.Context) (int, error) {
	if s.tasks == nil {
		return 0, nil
	}
	active, err := s.tasks.ListActive(ctx)
	if err != nil {
		return 0, err
	}
	return len(active), nil
}

// Result 查询任务结果，done 时重新签发 stream_url 供轮询兜底。
func (s *Service) Result(ctx context.Context, taskID int64) (*domain.ResolveTask, string, error) {
	t, err := s.tasks.Get(ctx, taskID)
	if err != nil {
		return nil, "", err
	}
	if t.Status != domain.ResolveDone {
		return t, "", nil
	}
	ticket, err := s.signer.Sign(ctx, ticketClaimsFor(t, t.ResultFileID, t.ResultSource, t.ResultAccountID), 0)
	if err != nil {
		return t, "", nil
	}
	return t, "/api/stream?ticket=" + ticket, nil
}

// Capabilities 计算能力预检结果。
func (s *Service) Capabilities(ctx context.Context) domain.Capabilities {
	// [V7 §9.4+ 扩展 G1.E] nil-safe：单测直接构造 s.Service{} 跳过 NewService 时
	// nasSourcesCount 字段可能为 nil，回落零值防止 panic（不改外部行为）。
	nasCountTotal, nasCountEnabled := 0, 0
	if s.nasSourcesCount != nil {
		nasCountTotal, nasCountEnabled = s.nasSourcesCount()
	}
	// [V7 §9.4 + §27.4] NAS 三态：
	//   not_configured: 未配置路径或 nas_enabled=false
	//   not_accessible: 配置了路径但路径不存在或无读权限
	//   ok:             配置 + 路径可读
	nasStatus := "not_configured"
	nasAvailable := false
	if s.nasConfigured(ctx) {
		// 路径已配置；进一步检查可达性
		paths := s.nasPathsKnown()
		if len(paths) == 0 {
			nasStatus = "not_configured"
		} else {
			// 任一路径存在即可读即视为可访问
			accessible := false
			for _, p := range paths {
				if s.pathAccessible(p) {
					accessible = true
					break
				}
			}
			if accessible {
				nasStatus = "ok"
				nasAvailable = true
			} else {
				nasStatus = "not_accessible"
			}
		}
	}

	// 索引完整/计数：取 indexStatus 真实状态
	indexComplete := false
	cnt := 0
	scanning, phase, processed, total := s.indexStatusFn()
	if s.mediaIndex != nil {
		if n, err := s.mediaIndex.Count(ctx); err == nil && n > 0 {
			indexComplete = true
			cnt = n
		}
	}
	return domain.Capabilities{
		NASAvailable:       nasAvailable,
		NASStatus:          nasStatus,
		NASIndexComplete:   indexComplete,
		NASIndexCount:      cnt,
		PansearchAvailable: s.pansearchHealth(ctx),
		LoggedInDrivers:    s.loggedInDrivers(ctx),
		NASPhase:           phase,
		NASProcessedFiles:  processed,
		NASTotalFiles:      total,
		NASScanning:        scanning, // 暴露扫描状态供前端/audit
		NASTotalSources:    nasCountTotal,
		NASEnabledSources:  nasCountEnabled,
		MagnetEnabled:      s.magnetEnabledFn(ctx),
		P0MinScore:         s.p0MinScoreFn(ctx),
		DemoFallback:       s.demoFallbackFn(ctx),
		ServerVersion:      s.serverVersion,
	}
}

// pathAccessible 检查单个 NAS 路径是否可读（V7 §9.4 + §27.4 not_accessible 检测）。
// 实现：调用 indexengine 的 IsScanning + 路径存在 + 至少存在一个 file。
// 实际 stat 在 Capabilities 调用时不该阻塞，所以只检查路径存在性。
func (s *Service) pathAccessible(path string) bool {
	// 通过 indexengine 的 NASPaths 函数判断路径是否存在：
	// indexStatusFn 提供的 scanning 状态为 true 即路径在用（视为可达）
	// 但更准确的判断：直接 stat 路径。stat 失败立刻返回 false（不阻塞）
	if s.pathStatFn != nil {
		return s.pathStatFn(path)
	}
	return false // 兜底：未注册 stat 函数时视作不可访问
}

// ticketClaimsFor 构造播放票据载荷。
func ticketClaimsFor(t *domain.ResolveTask, fileID, source string, accountID int64) playback.TicketClaims {
	return playback.TicketClaims{
		TaskID:     t.ID,
		AccountID:  accountID,
		FileID:     fileID,
		Source:     source,
		ExternalID: t.ExternalID,
	}
}

// pushStageBroadcast 直接广播阶段推进（不写库，用于 P2 高频轮询推送）。
func (s *Service) pushStageBroadcast(t *domain.ResolveTask) {
	if s.hub != nil {
		s.hub.Broadcast(websocket.TypeResolveStage, websocket.ResolveStagePayload{
			TaskID:      t.ID,
			ExternalID:  t.ExternalID,
			Stage:       string(t.Stage),
			Detail:      t.StageDetail,
			ProgressPct: t.ProgressPct,
		})
	}
}
