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

// shouldSkipP0 判定是否跳过 P0，返回原因（空表示不跳过）。
func (s *Service) shouldSkipP0(ctx context.Context) string {
	if !s.nasConfigured(ctx) {
		return "未配置 NAS 路径"
	}
	if s.mediaIndex == nil {
		return "索引服务不可用"
	}
	cnt, err := s.indexCountFn(ctx)
	if err != nil {
		return "" // 查询失败不强制跳过，让 P0 自己 fail
	}
	if cnt == 0 {
		return "NAS 索引为空"
	}
	return ""
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

func (s *Service) notFound(t *domain.ResolveTask) {
	t.Status = domain.ResolveFailed
	t.Stage = domain.StageNotFound
	t.StageDetail = "暂无可用资源"
	t.ErrorMsg = "暂无可用资源"
	_ = s.tasks.Update(context.Background(), t)
	if s.subs != nil {
		_, _ = s.subs.Add(context.Background(), &domain.Subscription{
			ExternalID:     t.ExternalID,
			ExternalSource: t.ExternalSource,
			MediaType:      t.MediaType,
			Title:          t.Title,
			Year:           t.Year,
			Status:         domain.SubWatching,
			MaxSearches:    12,
		})
	}
	if s.hub != nil {
		s.hub.Broadcast(websocket.TypeResolveFailed, websocket.ResolveFailedPayload{
			TaskID:     t.ID,
			Reason:     "暂无可用资源",
			Suggestion: "已自动创建订阅，系统将每周自动搜寻",
			Stage:      string(domain.StageNotFound),
		})
	}
}

// ResultOf 查询任务结果（轮询兜底）。
func (s *Service) ResultOf(ctx context.Context, taskID int64) (*domain.ResolveTask, error) {
	return s.tasks.Get(ctx, taskID)
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
	indexComplete := false
	cnt := 0
	if s.mediaIndex != nil {
		if n, err := s.mediaIndex.Count(ctx); err == nil && n > 0 {
			indexComplete = true
			cnt = n
		}
	}
	return domain.Capabilities{
		NASAvailable:       s.nasConfigured(ctx),
		NASIndexComplete:   indexComplete,
		NASIndexCount:      cnt,
		PansearchAvailable: s.pansearchHealth(ctx),
		LoggedInDrivers:    s.loggedInDrivers(ctx),
		MagnetEnabled:      s.magnetEnabledFn(ctx),
		DemoFallback:       s.demoFallbackFn(ctx),
		ServerVersion:      s.serverVersion,
	}
}
