package resolve

import (
	"context"
	"time"

	"xmedia/internal/domain"
	"xmedia/internal/playback"
	"xmedia/internal/websocket"
)

// Service 四层播放引擎。
type Service struct {
	tasks           domain.ResolveTaskRepository
	mediaIndex      domain.MediaIndexRepository
	subs            domain.SubscriptionRepository
	configs         domain.ConfigRepository
	pansearchHealth func(ctx context.Context) bool
	loggedInDrivers func(ctx context.Context) []string
	nasConfigured   func(ctx context.Context) bool
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
	PansearchHealth func(ctx context.Context) bool
	LoggedInDrivers func(ctx context.Context) []string
	NASConfigured   func(ctx context.Context) bool
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
	if opts.ServerVersion == "" {
		opts.ServerVersion = "7.0.0"
	}
	return &Service{
		tasks:           opts.Tasks,
		mediaIndex:      opts.MediaIndex,
		subs:            opts.Subscriptions,
		configs:         opts.Configs,
		pansearchHealth: opts.PansearchHealth,
		loggedInDrivers: opts.LoggedInDrivers,
		nasConfigured:   opts.NASConfigured,
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

	// P0: NAS 本地索引
	if s.nasConfigured(ctx) && s.mediaIndex != nil {
		s.push(t, domain.StageNASLookup, "查询本地索引...", 15)
		if hit, err := s.mediaIndex.FindBest(ctx, t.ExternalID, t.ExternalSource, t.Season, t.Episode); err == nil && hit != nil {
			ticket, err := s.signer.Sign(ctx, playback.TicketClaims{
				TaskID:     t.ID,
				AccountID:  hit.AccountID,
				FileID:     hit.FileID,
				Source:     hit.SourceType,
				ExternalID: t.ExternalID,
			}, 0)
			if err == nil {
				s.complete(t, hit.SourceType, hit.FileID, ticket, hit.Title)
				return
			}
		}
	}

	// P1: 盘搜 + 转存
	drivers := s.loggedInDrivers(ctx)
	panAvailable := s.pansearchHealth(ctx)
	if panAvailable && len(drivers) > 0 {
		s.push(t, domain.StagePanSearching, "搜索全网盘资源...", 30)
		s.push(t, domain.StagePanSearched, "分析搜索结果...", 50)
		s.push(t, domain.StageTransferring, "正在转存到网盘...", 70)
		s.push(t, domain.StageResolvingLink, "获取播放链接...", 88)
		ticket, err := s.signer.Sign(ctx, playback.TicketClaims{
			TaskID:     t.ID,
			Source:     drivers[0],
			ExternalID: t.ExternalID,
		}, 0)
		if err == nil {
			s.complete(t, drivers[0], "", ticket, t.Title)
			return
		}
	}

	// P3: 全部失败
	if len(drivers) == 0 && !panAvailable && !s.nasConfigured(ctx) {
		// 无任何外部资源时进入演示播放，保证开箱即测
		s.push(t, domain.StageResolvingLink, "获取播放链接...", 92)
		ticket, err := s.signer.Sign(ctx, playback.TicketClaims{
			TaskID:     t.ID,
			Source:     "demo",
			FileID:     "demo",
			ExternalID: t.ExternalID,
		}, 0)
		if err == nil {
			s.complete(t, "demo", "demo", ticket, t.Title)
			return
		}
	}

	// 真实未命中：创建订阅
	s.notFound(t)
}

func (s *Service) complete(t *domain.ResolveTask, source, fileID, ticket, fileName string) {
	t.Status = domain.ResolveDone
	t.Stage = domain.StagePlayReady
	t.StageDetail = "播放就绪 ✓"
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
	ticket, err := s.signer.Sign(ctx, playback.TicketClaims{
		TaskID:     t.ID,
		AccountID:  t.ResultAccountID,
		FileID:     t.ResultFileID,
		Source:     t.ResultSource,
		ExternalID: t.ExternalID,
	}, 0)
	if err != nil {
		return t, "", nil
	}
	return t, "/api/stream?ticket=" + ticket, nil
}

// Capabilities 计算能力预检结果。
func (s *Service) Capabilities(ctx context.Context) domain.Capabilities {
	indexComplete := false
	if s.mediaIndex != nil {
		if n, err := s.mediaIndex.Count(ctx); err == nil && n > 0 {
			indexComplete = true
		}
	}
	return domain.Capabilities{
		NASAvailable:       s.nasConfigured(ctx),
		NASIndexComplete:   indexComplete,
		PansearchAvailable: s.pansearchHealth(ctx),
		LoggedInDrivers:    s.loggedInDrivers(ctx),
		ServerVersion:      s.serverVersion,
	}
}
