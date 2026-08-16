package tmdb

import (
	"context"
	"testing"
	"time"

	"xmedia/internal/domain"
)

// evictLibraryRepo 支持淘汰计数检查的 media_library mock。
type evictLibraryRepo struct {
	*stubLibraryRepo
	items     map[int64]*domain.MediaLibrary // id -> item
	total     int
	deleted   []int64
	evictCand []*domain.MediaLibrary
}

func newEvictLibrary(total int, items map[int64]*domain.MediaLibrary) *evictLibraryRepo {
	if items == nil {
		items = map[int64]*domain.MediaLibrary{}
	}
	return &evictLibraryRepo{stubLibraryRepo: &stubLibraryRepo{}, items: items, total: total}
}
func (r *evictLibraryRepo) CountTotal(context.Context) (int, error) { return r.total, nil }
func (r *evictLibraryRepo) ListForEviction(_ context.Context, limit int) ([]*domain.MediaLibrary, error) {
	if len(r.evictCand) > 0 {
		return r.evictCand, nil
	}
	out := make([]*domain.MediaLibrary, 0, len(r.items))
	for _, m := range r.items {
		out = append(out, m)
	}
	return out, nil
}
func (r *evictLibraryRepo) Delete(_ context.Context, id int64) error {
	r.deleted = append(r.deleted, id)
	delete(r.items, id)
	return nil
}

// existsRepoStub 通用保护源：按 external_id 集合应答 Exists。
// 注意：Favorite/Subscription/PlayHistory 三个接口的 Add/Remove/List 签名
// 冲突，无法单类型实现多接口，故拆为三个独立类型。
type existsRepoStub struct {
	ids map[int64]bool
}

func (e *existsRepoStub) exists(externalID int64) bool { return e.ids[externalID] }

type favoriteStub struct{ existsRepoStub }

func (f *favoriteStub) Add(context.Context, *domain.Favorite) (int64, error) { return 0, nil }
func (f *favoriteStub) Remove(context.Context, int64, string) error          { return nil }
func (f *favoriteStub) List(context.Context) ([]*domain.Favorite, error)     { return nil, nil }
func (f *favoriteStub) Exists(_ context.Context, id int64, _ string) (bool, error) {
	return f.exists(id), nil
}

type subscriptionStub struct{ existsRepoStub }

func (s *subscriptionStub) Add(context.Context, *domain.Subscription) (int64, error) { return 0, nil }
func (s *subscriptionStub) Remove(context.Context, int64, string) error              { return nil }
func (s *subscriptionStub) List(context.Context) ([]*domain.Subscription, error)     { return nil, nil }
func (s *subscriptionStub) Exists(_ context.Context, id int64, _ string) (bool, error) {
	return s.exists(id), nil
}
func (s *subscriptionStub) UpdateStatus(context.Context, int64, domain.SubStatus, string, int64, string) error {
	return nil
}
func (s *subscriptionStub) ActiveCount(context.Context) (int, error) { return 0, nil }
func (s *subscriptionStub) TouchSearch(context.Context, int64) error { return nil }

type playHistoryStub struct{ existsRepoStub }

func (p *playHistoryStub) Upsert(context.Context, *domain.PlayHistory) error { return nil }
func (p *playHistoryStub) Get(context.Context, int64, string, int, int) (*domain.PlayHistory, error) {
	return nil, domain.Errf(domain.CodeNotFound)
}
func (p *playHistoryStub) Exists(_ context.Context, id int64, _ string) (bool, error) {
	return p.exists(id), nil
}
func (p *playHistoryStub) List(context.Context, int) ([]*domain.PlayHistory, error) { return nil, nil }
func (p *playHistoryStub) ListContinueWatching(context.Context, int) ([]*domain.PlayHistory, error) {
	return nil, nil
}
func (p *playHistoryStub) DeleteByKey(context.Context, int64, string, int, int) error { return nil }
func (p *playHistoryStub) DeleteAll(context.Context) error                            { return nil }
func (p *playHistoryStub) HasAny(_ context.Context, id int64, _ string) (bool, error) {
	return p.exists(id), nil
}

// TestMaybeEvictUnderLimitNoOp 未超上限不淘汰。
func TestMaybeEvictUnderLimitNoOp(t *testing.T) {
	cfg := &stubConfigRepo{values: map[string]string{"media_library_max_rows": "5000", "media_library_keep_rows": "3000"}}
	lib := newEvictLibrary(4000, nil)
	s := NewService(cfg, lib)
	if removed := s.MaybeEvict(context.Background()); removed != 0 {
		t.Fatalf("未超上限不应淘汰，实际 %d", removed)
	}
}

// TestMaybeEvictRemovesOldestUnprotected 超上限淘汰最旧未保护条目。
func TestMaybeEvictRemovesOldestUnprotected(t *testing.T) {
	cfg := &stubConfigRepo{values: map[string]string{"media_library_max_rows": "100", "media_library_keep_rows": "60"}}
	items := map[int64]*domain.MediaLibrary{}
	for id := int64(1); id <= 45; id++ {
		items[id] = &domain.MediaLibrary{ID: id, ExternalID: id, ExternalSource: "tmdb"}
	}
	lib := newEvictLibrary(105, items)
	s := NewService(cfg, lib)
	removed := s.MaybeEvict(context.Background())
	if removed != 45 {
		t.Fatalf("应淘汰 45 条（105->60），实际 %d", removed)
	}
	if len(lib.deleted) != 45 {
		t.Fatalf("删除调用数 = %d, want 45", len(lib.deleted))
	}
}

// TestMaybeEvictSkipsProtected 收藏/订阅/播放中的条目跳过。
func TestMaybeEvictSkipsProtected(t *testing.T) {
	cfg := &stubConfigRepo{values: map[string]string{"media_library_max_rows": "100", "media_library_keep_rows": "50"}}
	items := map[int64]*domain.MediaLibrary{
		1: {ID: 1, ExternalID: 10, ExternalSource: "tmdb"},
		2: {ID: 2, ExternalID: 20, ExternalSource: "tmdb"},
		3: {ID: 3, ExternalID: 30, ExternalSource: "tmdb"},
		4: {ID: 4, ExternalID: 40, ExternalSource: "tmdb"},
	}
	lib := newEvictLibrary(105, items)
	s := NewService(cfg, lib)
	s.SetLRUProtectors(LRUProtectors{
		Favorites:     &favoriteStub{existsRepoStub{ids: map[int64]bool{10: true}}},
		Subscriptions: &subscriptionStub{existsRepoStub{ids: map[int64]bool{20: true}}},
		PlayHistory:   &playHistoryStub{existsRepoStub{ids: map[int64]bool{30: true}}},
	})
	removed := s.MaybeEvict(context.Background())
	if removed != 1 {
		t.Fatalf("应仅淘汰 1 条未保护条目（54->50），实际 %d（deleted=%v）", removed, lib.deleted)
	}
	if len(lib.deleted) != 1 || lib.deleted[0] != 4 {
		t.Fatalf("应删除 id=4（唯一未保护），实际 %v", lib.deleted)
	}
}

// TestConfigIntParsing 配置解析边界。
func TestConfigIntParsing(t *testing.T) {
	cfg := &stubConfigRepo{values: map[string]string{"k": "3000", "bad": "abc", "empty": ""}}
	s := NewService(cfg, nil)
	if got := s.configInt(context.Background(), "k", 5000); got != 3000 {
		t.Fatalf("configInt = %d, want 3000", got)
	}
	if got := s.configInt(context.Background(), "bad", 5000); got != 5000 {
		t.Fatalf("非法值应回落默认: %d", got)
	}
	if got := s.configInt(context.Background(), "missing", 5000); got != 5000 {
		t.Fatalf("缺失键应回落默认: %d", got)
	}
}

// 保证 time import 在 test 包内被使用（LastAccessedAt 排序场景文档化）。
var _ = time.Now
