package indexengine

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"xmedia/internal/domain"
)

// memoryIndexRepo 内存版 MediaIndex 仓库（扫描集成测试用）。
type memoryIndexRepo struct {
	items map[string]*domain.MediaIndex // source_type|file_path -> item
}

func newMemoryIndexRepo() *memoryIndexRepo {
	return &memoryIndexRepo{items: map[string]*domain.MediaIndex{}}
}

func keyOf(sourceType, filePath string) string { return sourceType + "|" + filePath }

func (m *memoryIndexRepo) Upsert(_ context.Context, item *domain.MediaIndex) (int64, error) {
	m.items[keyOf(item.SourceType, item.FilePath)] = item
	return int64(len(m.items)), nil
}
func (m *memoryIndexRepo) FindBest(context.Context, int64, string, int, int) (*domain.MediaIndex, error) {
	return nil, domain.Errf(domain.CodeNotFound)
}
func (m *memoryIndexRepo) AvailableKeys(context.Context, []domain.AvailabilityKey) ([]domain.AvailabilityKey, error) {
	return nil, nil
}
func (m *memoryIndexRepo) Count(context.Context) (int, error) { return len(m.items), nil }
func (m *memoryIndexRepo) ListBySource(_ context.Context, sourceType string, accountID int64) ([]*domain.MediaIndex, error) {
	var out []*domain.MediaIndex
	for _, item := range m.items {
		if item.SourceType == sourceType && (accountID == 0 || item.AccountID == accountID) {
			out = append(out, item)
		}
	}
	return out, nil
}
func (m *memoryIndexRepo) DeleteBySourcePath(_ context.Context, sourceType, filePath string) error {
	delete(m.items, keyOf(sourceType, filePath))
	return nil
}
func (m *memoryIndexRepo) ListUnconfirmedBefore(_ context.Context, before time.Time) ([]*domain.MediaIndex, error) {
	return nil, nil
}
func (m *memoryIndexRepo) MarkOrphaned(_ context.Context, ids []int64) error { return nil }

// memoryLibraryRepo 内存版 MediaLibrary 仓库。
type memoryLibraryRepo struct {
	items []*domain.MediaLibrary
}

func (m *memoryLibraryRepo) Upsert(context.Context, *domain.MediaLibrary) (int64, error) {
	return 0, nil
}
func (m *memoryLibraryRepo) Get(context.Context, int64, string) (*domain.MediaLibrary, error) {
	return nil, domain.Errf(domain.CodeNotFound)
}
func (m *memoryLibraryRepo) Touch(context.Context, int64, string) error { return nil }
func (m *memoryLibraryRepo) SearchByTitle(_ context.Context, title string, limit int) ([]*domain.MediaLibrary, error) {
	var out []*domain.MediaLibrary
	for _, item := range m.items {
		if item.Title == title || item.TitleOrig == title {
			out = append(out, item)
		}
	}
	return out, nil
}
func (m *memoryLibraryRepo) ListForEviction(context.Context, int) ([]*domain.MediaLibrary, error) {
	return nil, nil
}
func (m *memoryLibraryRepo) CountTotal(context.Context) (int, error) { return 0, nil }
func (m *memoryLibraryRepo) Delete(context.Context, int64) error     { return nil }

// memoryConfigRepo 内存版 Config 仓库。
type memoryConfigRepo struct {
	values map[string]string
}

func (m *memoryConfigRepo) Get(_ context.Context, key string) (string, bool, error) {
	v, ok := m.values[key]
	return v, ok, nil
}
func (m *memoryConfigRepo) Set(_ context.Context, key, value string) error {
	m.values[key] = value
	return nil
}
func (m *memoryConfigRepo) All(context.Context) (map[string]string, error) { return m.values, nil }

// buildScanFixture 构造临时 NAS 目录树：2 个可匹配 + 1 个孤儿 + 1 个非视频。
func buildScanFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"movies/阿凡达.2009.1080p.BluRay.mkv": "x",
		"tv/权力的游戏.S01E03.1080p.mkv":        "x",
		"other/乱七八糟视频文件.mkv":               "x",
		"movies/说明.txt":                    "x",
		"movies/海报.jpg":                    "x",
	}
	for rel, content := range files {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("创建目录失败: %v", err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("写文件失败: %v", err)
		}
	}
	return root
}

// TestScanNASFullPipeline 三阶段扫描端到端（tempdir 文件树 -> 索引写入 -> 分类正确）。
func TestScanNASFullPipeline(t *testing.T) {
	root := buildScanFixture(t)
	index := newMemoryIndexRepo()
	library := &memoryLibraryRepo{items: []*domain.MediaLibrary{
		{Title: "阿凡达", TitleOrig: "Avatar", Year: 2009, ExternalID: 19995, ExternalSource: "tmdb"},
	}}
	cfg := &memoryConfigRepo{values: map[string]string{"nas_local_path": root}}

	s := NewService(Options{
		MediaIndex:   index,
		MediaLibrary: library,
		Configs:      cfg,
		WorkerCount:  4,
	})

	s.scanNAS(context.Background(), root, false)

	items, _ := index.ListBySource(context.Background(), "nas", 0)
	if len(items) != 3 {
		t.Fatalf("应索引 3 个视频文件，实际 %d: %#v", len(items), items)
	}
	var matched, orphaned int
	for _, item := range items {
		switch item.MatchStatus {
		case domain.MatchMatched:
			matched++
			if item.ExternalID != 19995 && item.ExternalID != 0 {
				t.Fatalf("匹配的 external_id 错误: %#v", item)
			}
		case domain.MatchOrphaned:
			orphaned++
		case domain.MatchUnconfirmed:
			// 无候选且 library 非 nil -> unconfirmed（0.5 分）
		}
	}
	if matched == 0 {
		t.Fatalf("至少应有一个匹配条目")
	}
	_ = orphaned

	p := s.Progress()
	if p.Total != 3 || p.Processed != 3 || p.Phase != "C" {
		t.Fatalf("进度快照错误: %#v", p)
	}
	if s.IsScanning() {
		t.Fatalf("扫描结束后 IsScanning 应为 false")
	}
}

// TestScanNASIncrementalOnlyNewer 增量扫描仅收录 mtime 新于上次扫描的文件。
func TestScanNASIncrementalOnlyNewer(t *testing.T) {
	root := buildScanFixture(t)
	index := newMemoryIndexRepo()
	cfg := &memoryConfigRepo{values: map[string]string{"nas_local_path": root}}

	s := NewService(Options{MediaIndex: index, MediaLibrary: &memoryLibraryRepo{}, Configs: cfg, WorkerCount: 2})
	s.scanNAS(context.Background(), root, false) // 全量后 lastScan 已设置

	// 新增一个文件
	newFile := filepath.Join(root, "movies", "新片.2026.1080p.mp4")
	if err := os.WriteFile(newFile, []byte("x"), 0o644); err != nil {
		t.Fatalf("写新文件失败: %v", err)
	}
	before := time.Now().Add(-time.Minute)
	_ = before
	time.Sleep(50 * time.Millisecond)

	s.scanNAS(context.Background(), root, true)

	items, _ := index.ListBySource(context.Background(), "nas", 0)
	if len(items) != 4 {
		t.Fatalf("增量后应 4 条索引（3 保留 + 1 新增），实际 %d", len(items))
	}
	found := false
	for _, item := range items {
		if item.FilePath == filepath.Join("movies", "新片.2026.1080p.mp4") {
			found = true
		}
	}
	if !found {
		t.Fatalf("新增文件未被增量索引")
	}
}

// TestScanNASPhaseDCleansMissing 增量扫描清理已消失文件。
func TestScanNASPhaseDCleansMissing(t *testing.T) {
	root := buildScanFixture(t)
	index := newMemoryIndexRepo()
	cfg := &memoryConfigRepo{values: map[string]string{"nas_local_path": root}}

	s := NewService(Options{MediaIndex: index, MediaLibrary: &memoryLibraryRepo{}, Configs: cfg, WorkerCount: 2})
	s.scanNAS(context.Background(), root, false)
	if n, _ := index.Count(context.Background()); n != 3 {
		t.Fatalf("全量后应 3 条，实际 %d", n)
	}

	// 删除孤儿文件后跑增量
	if err := os.Remove(filepath.Join(root, "other", "乱七八糟视频文件.mkv")); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	s.scanNAS(context.Background(), root, true)

	if n, _ := index.Count(context.Background()); n != 2 {
		t.Fatalf("Phase D 清理后应 2 条，实际 %d", n)
	}
}

// TestScanNASNoPathConfig 未配置 NAS 路径时返回验证错误。
func TestScanNASNoPathConfig(t *testing.T) {
	s := NewService(Options{MediaIndex: newMemoryIndexRepo(), MediaLibrary: &memoryLibraryRepo{}, Configs: &memoryConfigRepo{values: map[string]string{}}})
	if err := s.ScanNASFull(context.Background()); err == nil {
		t.Fatalf("未配置 NAS 路径应报错")
	}
}

// TestCleanupSkipsRecentlyPlayed §9.5：最近 2 小时播放的跳过。
func TestCleanupSkipsRecentlyPlayed(t *testing.T) {
	index := newMemoryIndexRepo()
	now := time.Now()
	recent := now.Add(-time.Hour)
	old := now.Add(-24 * time.Hour)
	ctx := context.Background()
	_, _ = index.Upsert(ctx, &domain.MediaIndex{SourceType: "pan", AccountID: 1, FilePath: "a.mkv", LastPlayedAt: &recent})
	_, _ = index.Upsert(ctx, &domain.MediaIndex{SourceType: "pan", AccountID: 1, FilePath: "b.mkv", LastPlayedAt: &old})
	_, _ = index.Upsert(ctx, &domain.MediaIndex{SourceType: "pan", AccountID: 1, FilePath: "c.mkv"})

	s := NewService(Options{MediaIndex: index, MediaLibrary: &memoryLibraryRepo{}})
	removed, err := s.Cleanup(ctx, "pan", 1)
	if err != nil {
		t.Fatalf("清理失败: %v", err)
	}
	if removed != 2 {
		t.Fatalf("应清理 2 条（跳过最近播放 1 条），实际 %d", removed)
	}
	if n, _ := index.Count(ctx); n != 1 {
		t.Fatalf("清理后应剩 1 条，实际 %d", n)
	}
}
