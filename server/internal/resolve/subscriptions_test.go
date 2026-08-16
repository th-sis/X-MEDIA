package resolve

import (
	"context"
	"testing"
	"time"

	"xmedia/internal/domain"
	"xmedia/internal/pansearch"
)

// subRepoStub 记录订阅状态迁移的 stub。
type subRepoStub struct {
	items   map[int64]*domain.Subscription
	touched []int64
	updated map[int64]domain.SubStatus
}

func newSubRepoStub(subs ...*domain.Subscription) *subRepoStub {
	r := &subRepoStub{items: map[int64]*domain.Subscription{}, updated: map[int64]domain.SubStatus{}}
	for _, s := range subs {
		r.items[s.ID] = s
	}
	return r
}
func (r *subRepoStub) Add(_ context.Context, s *domain.Subscription) (int64, error) {
	r.items[s.ID] = s
	return s.ID, nil
}
func (r *subRepoStub) Remove(context.Context, int64, string) error { return nil }
func (r *subRepoStub) List(context.Context) ([]*domain.Subscription, error) {
	out := make([]*domain.Subscription, 0, len(r.items))
	for _, s := range r.items {
		out = append(out, s)
	}
	return out, nil
}
func (r *subRepoStub) UpdateStatus(_ context.Context, id int64, status domain.SubStatus, _ string, _ int64, _ string) error {
	r.updated[id] = status
	if s, ok := r.items[id]; ok {
		s.Status = status
	}
	return nil
}
func (r *subRepoStub) Exists(context.Context, int64, string) (bool, error) { return false, nil }
func (r *subRepoStub) ActiveCount(context.Context) (int, error)            { return 0, nil }
func (r *subRepoStub) TouchSearch(_ context.Context, id int64) error {
	r.touched = append(r.touched, id)
	if s, ok := r.items[id]; ok {
		s.SearchCount++
	}
	return nil
}

// TestRunPassFoundMarksAndPushes 命中 -> found 状态迁移。
func TestRunPassFoundMarksAndPushes(t *testing.T) {
	sub := &domain.Subscription{
		ID: 1, ExternalID: 19995, ExternalSource: "tmdb", MediaType: "movie",
		Title: "阿凡达", Year: 2009, Status: domain.SubWatching, MaxSearches: 12,
	}
	repo := newSubRepoStub(sub)
	searcher := NewSubscriptionSearcher(SubscriptionSearcherOptions{
		Subscriptions: repo,
		Probe:         func(context.Context, *domain.Subscription) bool { return true },
		Throttle:      time.Millisecond,
	})
	searcher.runPass(context.Background())

	if sub.Status != domain.SubFound {
		t.Fatalf("命中后状态 = %s, want found", sub.Status)
	}
	if len(repo.touched) != 0 {
		t.Fatalf("命中不应 TouchSearch: %v", repo.touched)
	}
}

// TestRunPassMissTouchesSearch 未命中 -> TouchSearch 计数。
func TestRunPassMissTouchesSearch(t *testing.T) {
	sub := &domain.Subscription{
		ID: 2, ExternalID: 19995, ExternalSource: "tmdb", MediaType: "movie",
		Title: "阿凡达", Status: domain.SubWatching, MaxSearches: 12,
	}
	repo := newSubRepoStub(sub)
	searcher := NewSubscriptionSearcher(SubscriptionSearcherOptions{
		Subscriptions: repo,
		Probe:         func(context.Context, *domain.Subscription) bool { return false },
		Throttle:      time.Millisecond,
	})
	searcher.runPass(context.Background())

	if len(repo.touched) != 1 || repo.touched[0] != 2 {
		t.Fatalf("未命中应 TouchSearch 一次: %v", repo.touched)
	}
	if sub.SearchCount != 1 {
		t.Fatalf("SearchCount 应递增: %d", sub.SearchCount)
	}
	if sub.Status != domain.SubWatching {
		t.Fatalf("未达上限应保持 watching: %s", sub.Status)
	}
}

// TestRunPassMissReachesMaxFails 未命中且达 MaxSearches -> failed。
func TestRunPassMissReachesMaxFails(t *testing.T) {
	sub := &domain.Subscription{
		ID: 3, ExternalID: 19995, ExternalSource: "tmdb", MediaType: "movie",
		Title: "阿凡达", Status: domain.SubWatching, MaxSearches: 3, SearchCount: 2,
	}
	repo := newSubRepoStub(sub)
	searcher := NewSubscriptionSearcher(SubscriptionSearcherOptions{
		Subscriptions: repo,
		Probe:         func(context.Context, *domain.Subscription) bool { return false },
		Throttle:      time.Millisecond,
	})
	searcher.runPass(context.Background())

	if sub.Status != domain.SubFailed {
		t.Fatalf("达到 MaxSearches 后状态 = %s, want failed", sub.Status)
	}
}

// TestRunPassSkipsNonWatching 非 watching 订阅跳过。
func TestRunPassSkipsNonWatching(t *testing.T) {
	found := &domain.Subscription{ID: 4, Title: "x", Status: domain.SubFound, MaxSearches: 12}
	failed := &domain.Subscription{ID: 5, Title: "y", Status: domain.SubFailed, MaxSearches: 12}
	repo := newSubRepoStub(found, failed)
	probeCalls := 0
	searcher := NewSubscriptionSearcher(SubscriptionSearcherOptions{
		Subscriptions: repo,
		Probe: func(context.Context, *domain.Subscription) bool {
			probeCalls++
			return true
		},
		Throttle: time.Millisecond,
	})
	searcher.runPass(context.Background())
	if probeCalls != 0 {
		t.Fatalf("非 watching 订阅不应探测: %d 次", probeCalls)
	}
}

// TestProbeAvailabilityHit 探测命中：搜索有结果 -> true。
func TestProbeAvailabilityHit(t *testing.T) {
	s, _, _, _, _ := newTestService(t)
	s.pansearchSearch = func(context.Context, pansearch.SearchRequest) ([]domain.PanSearchResult, error) {
		return []domain.PanSearchResult{
			{Title: "阿凡达 4K", Source: "quark", ShareURL: "https://pan.quark.cn/s/abc", Quality: "4K"},
		}, nil
	}
	s.pansearchCheck = func(context.Context, []pansearch.CheckItem) ([]pansearch.CheckResult, error) {
		return []pansearch.CheckResult{{URL: "https://pan.quark.cn/s/abc", State: "ok"}}, nil
	}
	sub := &domain.Subscription{ExternalID: 19995, ExternalSource: "tmdb", MediaType: "movie", Title: "阿凡达", Year: 2009}
	if !s.ProbeAvailability(context.Background(), sub) {
		t.Fatalf("有可用分享应命中")
	}
}

// TestProbeAvailabilityMiss 全部链接失效 -> false。
func TestProbeAvailabilityMiss(t *testing.T) {
	s, _, _, _, _ := newTestService(t)
	s.pansearchSearch = func(context.Context, pansearch.SearchRequest) ([]domain.PanSearchResult, error) {
		return []domain.PanSearchResult{
			{Title: "阿凡达", Source: "quark", ShareURL: "https://pan.quark.cn/s/abc", Quality: "4K"},
		}, nil
	}
	s.pansearchCheck = func(context.Context, []pansearch.CheckItem) ([]pansearch.CheckResult, error) {
		return []pansearch.CheckResult{{URL: "https://pan.quark.cn/s/abc", State: "bad"}}, nil
	}
	sub := &domain.Subscription{ExternalID: 19995, ExternalSource: "tmdb", MediaType: "movie", Title: "阿凡达", Year: 2009}
	if s.ProbeAvailability(context.Background(), sub) {
		t.Fatalf("链接全部失效应 miss")
	}
}

// TestProbeAvailabilityNoResults 无搜索结果 -> false。
func TestProbeAvailabilityNoResults(t *testing.T) {
	s, _, _, _, _ := newTestService(t)
	s.pansearchSearch = func(context.Context, pansearch.SearchRequest) ([]domain.PanSearchResult, error) {
		return nil, nil
	}
	sub := &domain.Subscription{ExternalID: 19995, ExternalSource: "tmdb", MediaType: "movie", Title: "阿凡达", Year: 2009}
	if s.ProbeAvailability(context.Background(), sub) {
		t.Fatalf("无结果应 miss")
	}
}
