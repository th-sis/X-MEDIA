package indexengine

import (
	"context"
	"encoding/json"
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

// memoryNASSourcesRepo 内存版 NASSource 仓库（[G4] file_count 回填测试用）。
type memoryNASSourcesRepo struct {
	items     map[int64]*domain.NASSource
	healthLog map[int64]struct {
		acc   domain.NASAccessibility
		count int64
		at    time.Time
	}
	nextID int64
}

func newMemoryNASSourcesRepo() *memoryNASSourcesRepo {
	return &memoryNASSourcesRepo{
		items:     map[int64]*domain.NASSource{},
		healthLog: map[int64]struct {
			acc   domain.NASAccessibility
			count int64
			at    time.Time
		}{},
	}
}

func (m *memoryNASSourcesRepo) Create(_ context.Context, s *domain.NASSource) (int64, error) {
	m.nextID++
	s.ID = m.nextID
	cp := *s
	m.items[s.ID] = &cp
	return s.ID, nil
}
func (m *memoryNASSourcesRepo) Update(_ context.Context, s *domain.NASSource) error {
	cp := *s
	m.items[s.ID] = &cp
	return nil
}
func (m *memoryNASSourcesRepo) Delete(_ context.Context, id int64) error {
	delete(m.items, id)
	return nil
}
func (m *memoryNASSourcesRepo) Get(_ context.Context, id int64) (*domain.NASSource, error) {
	v, ok := m.items[id]
	if !ok {
		return nil, domain.Errf(domain.CodeNotFound)
	}
	cp := *v
	return &cp, nil
}
func (m *memoryNASSourcesRepo) List(_ context.Context) ([]*domain.NASSource, error) {
	out := make([]*domain.NASSource, 0, len(m.items))
	for _, v := range m.items {
		cp := *v
		out = append(out, &cp)
	}
	return out, nil
}
func (m *memoryNASSourcesRepo) ListEnabled(ctx context.Context) ([]*domain.NASSource, error) {
	return m.List(ctx)
}
func (m *memoryNASSourcesRepo) PathTaken(_ context.Context, path string, _ int64) (bool, error) {
	for _, v := range m.items {
		if v.Path == path {
			return true, nil
		}
	}
	return false, nil
}
func (m *memoryNASSourcesRepo) NameTaken(_ context.Context, name string, _ int64) (bool, error) {
	for _, v := range m.items {
		if v.Name == name {
			return true, nil
		}
	}
	return false, nil
}
func (m *memoryNASSourcesRepo) UpdateHealth(_ context.Context, id int64, acc domain.NASAccessibility, count int64, at time.Time) error {
	m.healthLog[id] = struct {
		acc   domain.NASAccessibility
		count int64
		at    time.Time
	}{acc, count, at}
	if v, ok := m.items[id]; ok {
		v.LastAccessibility = acc
		v.FileCount = count
		v.LastCheckedAt = &at
	}
	return nil
}

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

	s.scanNAS(context.Background(), []string{root}, false)

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
	s.scanNAS(context.Background(), []string{root}, false) // 全量后 lastScan 已设置

	// 新增一个文件
	newFile := filepath.Join(root, "movies", "新片.2026.1080p.mp4")
	if err := os.WriteFile(newFile, []byte("x"), 0o644); err != nil {
		t.Fatalf("写新文件失败: %v", err)
	}
	before := time.Now().Add(-time.Minute)
	_ = before
	time.Sleep(50 * time.Millisecond)

	s.scanNAS(context.Background(), []string{root}, true)

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
	s.scanNAS(context.Background(), []string{root}, false)
	if n, _ := index.Count(context.Background()); n != 3 {
		t.Fatalf("全量后应 3 条，实际 %d", n)
	}

	// 删除孤儿文件后跑增量
	if err := os.Remove(filepath.Join(root, "other", "乱七八糟视频文件.mkv")); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	s.scanNAS(context.Background(), []string{root}, true)

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

// TestScanNASMultiplePaths [V7 §9.7] 多媒体源遍历：两条 NAS 媒体源路径合并入库。
// 使用 t.TempDir() + filepath.ToSlash 转为 POSIX 风格（容器生产环境只走 /mnt/...）。
func TestScanNASMultiplePaths(t *testing.T) {
	// 两个独立的临时 NAS 媒体源（统一 POSIX 路径）
	rootA := filepath.ToSlash(t.TempDir()) + "/nasA"
	rootB := filepath.ToSlash(t.TempDir()) + "/nasB"
	// rootA: 2 个视频（1 个匹配 + 1 个孤儿）
	if err := os.MkdirAll(filepath.FromSlash(rootA+"/movies"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.FromSlash(rootA+"/movies/阿凡达.2009.1080p.mkv"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.FromSlash(rootA+"/movies/未知片A.mkv"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// rootB: 1 个视频（匹配同一条 library 记录）
	if err := os.MkdirAll(filepath.FromSlash(rootB+"/tv"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.FromSlash(rootB+"/tv/阿凡达.S01E01.1080p.mkv"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	index := newMemoryIndexRepo()
	library := &memoryLibraryRepo{items: []*domain.MediaLibrary{
		{Title: "阿凡达", TitleOrig: "Avatar", Year: 2009, ExternalID: 19995, ExternalSource: "tmdb"},
	}}
	// 用新格式 nas_local_paths 数组（用 json.Marshal 自动正确转义反斜杠）
	pathsJSON, err := json.Marshal([]string{rootA, rootB})
	if err != nil {
		t.Fatalf("marshal paths: %v", err)
	}
	cfg := &memoryConfigRepo{values: map[string]string{
		"nas_local_paths": string(pathsJSON),
	}}

	s := NewService(Options{
		MediaIndex:   index,
		MediaLibrary: library,
		Configs:      cfg,
		WorkerCount:  2,
	})

	paths := s.NASPaths(context.Background())
	if len(paths) != 2 {
		t.Fatalf("NASPaths 应返回 2 条，实际 %d: %v", len(paths), paths)
	}
	s.scanNAS(context.Background(), paths, false)

	items, _ := index.ListBySource(context.Background(), "nas", 0)
	if len(items) != 3 {
		t.Fatalf("两条媒体源应共索引 3 个视频文件，实际 %d: %#v", len(items), items)
	}

	// 验证进度快照
	p := s.Progress()
	if p.Total != 3 || p.Processed != 3 || p.Phase != "C" {
		t.Fatalf("进度快照错误: total=%d processed=%d phase=%s", p.Total, p.Processed, p.Phase)
	}
	// 验证至少 1 个匹配（阿凡达 rootA 或 rootB）
	var matched int
	for _, item := range items {
		if item.MatchStatus == domain.MatchMatched && item.ExternalID == 19995 {
			matched++
		}
	}
	if matched == 0 {
		t.Fatalf("至少应有 1 个 ExternalID=19995 的匹配条目")
	}
}

// TestNASPathsParsesNewFormat [V7 §9.7] NASPaths() 正确解析 JSON 数组格式。
func TestNASPathsParsesNewFormat(t *testing.T) {
	root := t.TempDir()
	cfg := &memoryConfigRepo{values: map[string]string{
		"nas_local_paths": `["/mnt/nas-root/Asia-Movie","/mnt/nas-root/Western-Movie"]`,
	}}
	s := NewService(Options{Configs: cfg})
	paths := s.NASPaths(context.Background())
	if len(paths) != 2 {
		t.Fatalf("应返回 2 条路径，实际 %d: %v", len(paths), paths)
	}
	_ = root
}

// TestNASPathsFallbackLegacy [V7 §9.7] NASPaths() 在新格式为空时回退到旧 nas_local_path。
func TestNASPathsFallbackLegacy(t *testing.T) {
	cfg := &memoryConfigRepo{values: map[string]string{
		"nas_local_path": "/mnt/nas-root/Legacy",
	}}
	s := NewService(Options{Configs: cfg})
	paths := s.NASPaths(context.Background())
	if len(paths) != 1 || paths[0] != "/mnt/nas-root/Legacy" {
		t.Fatalf("应回退到旧字段单条，实际 %v", paths)
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

// TestScanNASBackfillsFileCount [G4 修正] V7 §9.4+: ScanNASFull 完成后
// 应自动调 nasSources.UpdateHealth 回填 file_count (与 WalkDir+IsVideoFile 同口径).
// 这是 4 个顶级分类目录 (Asia-Movie/West-Movie/Documentary/X-RATED) 部署后
// file_count 一直显示 0 的根因 — 老实现不调 UpdateHealth.
func TestScanNASBackfillsFileCount(t *testing.T) {
	// 构造模拟 NAS 顶级分类目录: 一级都是子目录, 视频文件埋在 2 级
	root := t.TempDir()
	for _, subdir := range []string{"Asia-Movie", "West-Movie"} {
		for _, movie := range []string{
			"Inception (2010)/movie.mkv",
			"Avatar (2009)/disc1.mkv",
			"Avatar (2009)/disc2.mp4",
			"说明.txt",
		} {
			full := filepath.Join(root, subdir, movie)
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				t.Fatalf("创建目录失败: %v", err)
			}
			if err := os.WriteFile(full, []byte("x"), 0o644); err != nil {
				t.Fatalf("写文件失败: %v", err)
			}
		}
	}
	// 期望: 每个 subdir 有 3 个视频 (.mkv/.mkv/.mp4) + 1 个 .txt (非视频)
	// 即每个 source.file_count 应 = 3

	nas := newMemoryNASSourcesRepo()
	for _, subdir := range []string{"Asia-Movie", "West-Movie"} {
		_, err := nas.Create(context.Background(), &domain.NASSource{
			Name: subdir, Path: filepath.Join(root, subdir), Enabled: true,
		})
		if err != nil {
			t.Fatalf("Create 失败: %v", err)
		}
	}

	index := newMemoryIndexRepo()
	s := NewService(Options{
		MediaIndex:   index,
		MediaLibrary: &memoryLibraryRepo{},
		Configs:      &memoryConfigRepo{values: map[string]string{}},
		NASSources:   nas,
		WorkerCount:  2,
	})

	// 同步直接调 scanNAS 避免 goroutine 调度 race (ScanNASFull 内部 go s.scanNAS,
	// 测试等待循环可能错过 scanning=true 的窗口).
	sources, _ := nas.List(context.Background())
	paths := make([]string, 0, len(sources))
	for _, src := range sources {
		paths = append(paths, src.Path)
	}
	s.scanNAS(context.Background(), paths, false)

	// 验证: 2 个 source 都被回填, 每个 file_count = 3
	sources, _ = nas.List(context.Background())
	if len(sources) != 2 {
		t.Fatalf("应有 2 个 source, 实际 %d", len(sources))
	}
	for _, src := range sources {
		health, ok := nas.healthLog[src.ID]
		if !ok {
			t.Fatalf("source %d (%s) 未被回填 health (UpdateHealth 未调用)", src.ID, src.Name)
		}
		if health.acc != domain.NASAccessibilityOK {
			t.Fatalf("source %s 路径应可访问, 实际 %s", src.Name, health.acc)
		}
		if health.count != 3 {
			t.Fatalf("source %s 应数 3 个视频文件, 实际 %d", src.Name, health.count)
		}
		if src.FileCount != 3 {
			t.Fatalf("source %s FileCount 字段应=3, 实际 %d", src.Name, src.FileCount)
		}
		if src.LastCheckedAt == nil {
			t.Fatalf("source %s LastCheckedAt 应被设置", src.Name)
		}
	}
}
