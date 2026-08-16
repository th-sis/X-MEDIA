package resolve

import (
	"context"
	"log/slog"
	"time"

	"xmedia/internal/domain"
	"xmedia/internal/pansearch"
	"xmedia/internal/websocket"
)

// SubscriptionSearcher 订阅自动搜寻器（§20）：
// 周期性（默认 7 天）扫描 watching 订阅，复用 P1 搜索链做轻量可用性探测；
// 命中 -> 标记 found + WS subscription_ready 推送；未命中 -> search_count+1，
// 达到 MaxSearches 上限 -> failed（放弃）。
type SubscriptionSearcher struct {
	subs    domain.SubscriptionRepository
	probe   func(ctx context.Context, sub *domain.Subscription) bool
	hub     *websocket.Hub
	log     *slog.Logger
	between time.Duration // 单订阅间的节流间隔（防 PanSou 打爆）
}

// SubscriptionSearcherOptions 订阅搜寻器依赖。
type SubscriptionSearcherOptions struct {
	Subscriptions domain.SubscriptionRepository
	Probe         func(ctx context.Context, sub *domain.Subscription) bool
	Hub           *websocket.Hub
	Log           *slog.Logger
	Throttle      time.Duration
}

func NewSubscriptionSearcher(opts SubscriptionSearcherOptions) *SubscriptionSearcher {
	throttle := opts.Throttle
	if throttle <= 0 {
		throttle = 2 * time.Second
	}
	log := opts.Log
	if log == nil {
		log = slog.Default()
	}
	return &SubscriptionSearcher{
		subs:    opts.Subscriptions,
		probe:   opts.Probe,
		hub:     opts.Hub,
		log:     log,
		between: throttle,
	}
}

// Start 启动后台搜寻循环（立即执行首轮，此后按 interval 周期执行）。
func (s *SubscriptionSearcher) Start(ctx context.Context, interval time.Duration) {
	if s.subs == nil || s.probe == nil {
		s.log.Warn("订阅搜寻器未启用：缺少订阅仓库或探测函数")
		return
	}
	if interval <= 0 {
		interval = 7 * 24 * time.Hour
	}
	go s.loop(ctx, interval)
}

func (s *SubscriptionSearcher) loop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	s.runPass(ctx) // 启动即跑首轮
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runPass(ctx)
		}
	}
}

// runPass 扫描一轮 watching 订阅（导出供测试直接调用）。
func (s *SubscriptionSearcher) runPass(ctx context.Context) {
	subs, err := s.subs.List(ctx)
	if err != nil {
		s.log.Warn("订阅搜寻失败：读取订阅列表出错", "err", err)
		return
	}
	checked := 0
	for _, sub := range subs {
		if sub == nil || sub.Status != domain.SubWatching {
			continue
		}
		select {
		case <-ctx.Done():
			return
		default:
		}
		checked++
		if s.probe(ctx, sub) {
			if err := s.subs.UpdateStatus(ctx, sub.ID, domain.SubFound, "pansearch", 0, ""); err == nil && s.hub != nil {
				s.hub.Broadcast(websocket.TypeSubReady, websocket.SubReadyPayload{
					ExternalID:     sub.ExternalID,
					ExternalSource: sub.ExternalSource,
					MediaType:      sub.MediaType,
					Title:          sub.Title,
					Year:           sub.Year,
					ResultSource:   "pansearch",
				})
			}
			continue
		}
		_ = s.subs.TouchSearch(ctx, sub.ID)
		if sub.MaxSearches > 0 && sub.SearchCount+1 >= sub.MaxSearches {
			_ = s.subs.UpdateStatus(ctx, sub.ID, domain.SubFailed, "", 0, "")
			s.log.Info("订阅自动搜寻达上限，标记失败", "title", sub.Title, "searches", sub.SearchCount+1)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(s.between):
		}
	}
	s.log.Info("订阅自动搜寻完成", "checked", checked)
}

// ProbeAvailability 轻量可用性探测（§20）：多语言关键词 -> P1 搜索 -> 排序 -> 链接检测。
// 命中任一条可用分享即返回 true（不转存不播放）。供订阅搜寻器调用。
func (s *Service) ProbeAvailability(ctx context.Context, sub *domain.Subscription) bool {
	if s.pansearchSearch == nil {
		return false
	}
	task := &domain.ResolveTask{
		ExternalID:     sub.ExternalID,
		ExternalSource: sub.ExternalSource,
		MediaType:      sub.MediaType,
		Title:          sub.Title,
		Year:           sub.Year,
	}
	var media *domain.MediaLibrary
	if s.mediaLibrary != nil {
		media, _ = s.mediaLibrary.Get(ctx, sub.ExternalID, sub.ExternalSource)
	}
	keywords := buildSearchKeywords(task, media)
	for _, kw := range keywords {
		results, err := s.pansearchSearch(ctx, pansearch.SearchRequest{
			Keyword:    kw,
			CloudTypes: pansearchCloudTypes(s.loggedInDrivers(ctx)),
		})
		if err != nil || len(results) == 0 {
			continue
		}
		sorted := pansearch.SortResults(results, s.prioritySources(ctx), true)
		// 可选链接检测：有检测函数且结果可用则过滤失效链接
		if s.pansearchCheck != nil {
			items := make([]pansearch.CheckItem, 0, len(sorted))
			for _, r := range sorted {
				if r.ShareURL != "" {
					items = append(items, pansearch.CheckItem{DiskType: r.Source, URL: r.ShareURL, Password: r.Password})
				}
			}
			if len(items) > 0 {
				checks, cerr := s.pansearchCheck(ctx, items)
				if cerr == nil {
					okSet := map[string]bool{}
					for _, c := range checks {
						if c.State == "ok" {
							okSet[c.URL] = true
						}
					}
					for _, r := range sorted {
						if r.ShareURL != "" && okSet[r.ShareURL] {
							return true
						}
					}
					continue // 全部失效 -> 下一个关键词
				}
			}
		}
		for _, r := range sorted {
			if r.ShareURL != "" || r.MagnetURL != "" {
				return true
			}
		}
	}
	return false
}
