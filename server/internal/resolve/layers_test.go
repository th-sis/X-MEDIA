package resolve

import (
	"context"
	"strings"
	"testing"
	"time"

	"xmedia/internal/domain"
	"xmedia/internal/driver"
	"xmedia/internal/pansearch"
	"xmedia/internal/playback"
)

// ---- mock 基础设施 ----

type mockConfigRepo struct {
	values map[string]string
}

func (m *mockConfigRepo) Get(_ context.Context, key string) (string, bool, error) {
	v, ok := m.values[key]
	return v, ok, nil
}
func (m *mockConfigRepo) Set(_ context.Context, key, value string) error {
	if m.values == nil {
		m.values = map[string]string{}
	}
	m.values[key] = value
	return nil
}
func (m *mockConfigRepo) All(context.Context) (map[string]string, error) { return m.values, nil }

type mockTaskRepo struct {
	tasks   map[int64]*domain.ResolveTask
	nextID  int64
	updated []*domain.ResolveTask
}

func newMockTaskRepo() *mockTaskRepo {
	return &mockTaskRepo{tasks: map[int64]*domain.ResolveTask{}}
}
func (m *mockTaskRepo) Create(_ context.Context, t *domain.ResolveTask) (int64, error) {
	m.nextID++
	t.ID = m.nextID
	m.tasks[t.ID] = t
	return t.ID, nil
}
func (m *mockTaskRepo) Get(_ context.Context, id int64) (*domain.ResolveTask, error) {
	t, ok := m.tasks[id]
	if !ok {
		return nil, domain.Errf(domain.CodeNotFound)
	}
	return t, nil
}
func (m *mockTaskRepo) FindActiveByKey(context.Context, int64, string, int, int) (*domain.ResolveTask, error) {
	return nil, domain.Errf(domain.CodeNotFound)
}
func (m *mockTaskRepo) Update(_ context.Context, t *domain.ResolveTask) error {
	m.tasks[t.ID] = t
	m.updated = append(m.updated, t)
	return nil
}
func (m *mockTaskRepo) ListActive(context.Context) ([]*domain.ResolveTask, error) { return nil, nil }

type mockIndexRepo struct {
	upserted []*domain.MediaIndex
}

func (m *mockIndexRepo) Upsert(_ context.Context, i *domain.MediaIndex) (int64, error) {
	m.upserted = append(m.upserted, i)
	return 1, nil
}
func (m *mockIndexRepo) FindBest(context.Context, int64, string, int, int) (*domain.MediaIndex, error) {
	return nil, domain.Errf(domain.CodeNotFound)
}
func (m *mockIndexRepo) AvailableKeys(context.Context, []domain.AvailabilityKey) ([]domain.AvailabilityKey, error) {
	return nil, nil
}
func (m *mockIndexRepo) Count(context.Context) (int, error) { return 0, nil }
func (m *mockIndexRepo) ListBySource(context.Context, string, int64) ([]*domain.MediaIndex, error) {
	return nil, nil
}
func (m *mockIndexRepo) DeleteBySourcePath(context.Context, string, string) error { return nil }
func (m *mockIndexRepo) ListUnconfirmedBefore(context.Context, time.Time) ([]*domain.MediaIndex, error) {
	return nil, nil
}
func (m *mockIndexRepo) MarkOrphaned(context.Context, []int64) error { return nil }

type mockSubsRepo struct{ added []*domain.Subscription }

func (m *mockSubsRepo) Add(_ context.Context, s *domain.Subscription) (int64, error) {
	m.added = append(m.added, s)
	return 1, nil
}
func (m *mockSubsRepo) Remove(context.Context, int64, string) error          { return nil }
func (m *mockSubsRepo) List(context.Context) ([]*domain.Subscription, error) { return nil, nil }
func (m *mockSubsRepo) UpdateStatus(context.Context, int64, domain.SubStatus, string, int64, string) error {
	return nil
}
func (m *mockSubsRepo) Exists(context.Context, int64, string) (bool, error) { return false, nil }
func (m *mockSubsRepo) ActiveCount(context.Context) (int, error)            { return 0, nil }
func (m *mockSubsRepo) TouchSearch(context.Context, int64) error            { return nil }

type mockLibraryRepo struct{ media *domain.MediaLibrary }

func (m *mockLibraryRepo) Upsert(context.Context, *domain.MediaLibrary) (int64, error) { return 0, nil }
func (m *mockLibraryRepo) Get(context.Context, int64, string) (*domain.MediaLibrary, error) {
	if m.media == nil {
		return nil, domain.Errf(domain.CodeNotFound)
	}
	return m.media, nil
}
func (m *mockLibraryRepo) Touch(context.Context, int64, string) error { return nil }
func (m *mockLibraryRepo) SearchByTitle(_ context.Context, title string, limit int) ([]*domain.MediaLibrary, error) {
	if m.media == nil {
		return nil, nil
	}
	return []*domain.MediaLibrary{m.media}, nil
}
func (m *mockLibraryRepo) ListForEviction(context.Context, int) ([]*domain.MediaLibrary, error) {
	return nil, nil
}
func (m *mockLibraryRepo) CountTotal(context.Context) (int, error) { return 0, nil }
func (m *mockLibraryRepo) Delete(context.Context, int64) error     { return nil }

// mockDriver 实现 driver.Driver + ShareSaver + OfflineDownloader + FolderCreator。
type mockDriver struct {
	saved       []driver.ShareRequest
	saveResult  *driver.ShareResult
	saveErr     error
	offlineTask string
	offlineSt   *driver.OfflineTaskStatus
	offlineErr  error
	folders     []domain.FileItem
}

func (d *mockDriver) Config() driver.Config { return driver.Config{Name: "mock", DefaultRoot: "root"} }
func (d *mockDriver) GetAddition() any      { return nil }
func (d *mockDriver) Init(context.Context) error {
	return nil
}
func (d *mockDriver) Drop(context.Context) error { return nil }
func (d *mockDriver) Ping(context.Context) error { return nil }
func (d *mockDriver) ListFiles(context.Context, string) ([]domain.FileItem, error) {
	return d.folders, nil
}
func (d *mockDriver) SaveShare(_ context.Context, req driver.ShareRequest) (*driver.ShareResult, error) {
	d.saved = append(d.saved, req)
	if d.saveErr != nil {
		return nil, d.saveErr
	}
	return d.saveResult, nil
}
func (d *mockDriver) CreateFolder(_ context.Context, parentID, name string) (*domain.FileItem, error) {
	return &domain.FileItem{ID: "folder-xmedia", Name: name, IsDir: true}, nil
}
func (d *mockDriver) AddOfflineTask(_ context.Context, magnetURL string) (string, error) {
	if d.offlineErr != nil {
		return "", d.offlineErr
	}
	d.offlineTask = magnetURL
	return "task-1", nil
}
func (d *mockDriver) GetOfflineTaskStatus(context.Context, string) (*driver.OfflineTaskStatus, error) {
	return d.offlineSt, nil
}
func (d *mockDriver) CancelOfflineTask(context.Context, string) error { return nil }

func newTestService(t *testing.T, opts ...func(*Service)) (*Service, *mockConfigRepo, *mockTaskRepo, *mockIndexRepo, *mockDriver) {
	t.Helper()
	cfg := &mockConfigRepo{values: map[string]string{}}
	tasks := newMockTaskRepo()
	index := &mockIndexRepo{}
	drv := &mockDriver{saveResult: &driver.ShareResult{FileID: "f-1", FileName: "阿凡达 (2009).mkv", FileSize: 1000}}
	s := &Service{
		tasks:           tasks,
		mediaIndex:      index,
		subs:            &mockSubsRepo{},
		configs:         cfg,
		mediaLibrary:    &mockLibraryRepo{media: &domain.MediaLibrary{TitleOrig: "Avatar"}},
		pansearchHealth: func(context.Context) bool { return true },
		loggedInDrivers: func(context.Context) []string { return []string{"pan115", "quark"} },
		nasConfigured:   func(context.Context) bool { return false },
		indexCountFn:    func(context.Context) (int, error) { return 0, nil },
		pansearchSearch: func(context.Context, pansearch.SearchRequest) ([]domain.PanSearchResult, error) {
			return []domain.PanSearchResult{
				{Title: "阿凡达 4K", Source: "pan115", ShareURL: "https://115.com/s/abc", Password: "", Quality: "4K"},
			}, nil
		},
		pansearchCheck: func(context.Context, []pansearch.CheckItem) ([]pansearch.CheckResult, error) {
			return []pansearch.CheckResult{{URL: "https://115.com/s/abc", State: "ok"}}, nil
		},
		driverGet: func(context.Context, int64) (driver.Driver, error) { return drv, nil },
		accountsFn: func(context.Context) []domain.Account {
			return []domain.Account{{ID: 7, DriverType: "115_Open", IsActive: true}}
		},
		magnetEnabledFn: func(context.Context) bool { return true },
		demoFallbackFn:  func(context.Context) bool { return false },
		p0MinScoreFn:    func(context.Context) float64 { return 0.6 },
		signer:          playback.NewTicketSigner(cfg),
	}
	for _, fn := range opts {
		fn(s)
	}
	return s, cfg, tasks, index, drv
}

func newTestTask() *domain.ResolveTask {
	return &domain.ResolveTask{
		ID:             1,
		ExternalID:     19995,
		ExternalSource: "tmdb",
		MediaType:      "movie",
		Title:          "阿凡达",
		Year:           2009,
		Status:         domain.ResolveRunning,
		Stage:          domain.StageResolveStart,
	}
}

// ---- 测试用例 ----

func TestRunP1SavesShareAndCompletes(t *testing.T) {
	s, _, tasks, index, drv := newTestService(t)

	task := newTestTask()
	ok := s.runP1(context.Background(), task)

	if !ok {
		t.Fatalf("P1 应成功，任务状态=%s stage=%s", task.Status, task.Stage)
	}
	if task.Status != domain.ResolveDone || task.ResultFileID != "f-1" || task.ResultSource != "pan115" {
		t.Fatalf("任务未完成: status=%s file=%q source=%q", task.Status, task.ResultFileID, task.ResultSource)
	}
	if len(drv.saved) != 1 {
		t.Fatalf("SaveShare 应被调用 1 次，实际 %d", len(drv.saved))
	}
	if drv.saved[0].ShareURL != "https://115.com/s/abc" {
		t.Fatalf("转存 URL 错误: %q", drv.saved[0].ShareURL)
	}
	if drv.saved[0].TargetParentID == "" {
		t.Fatalf("转存目标目录不应为空（save root 管理）")
	}
	if len(index.upserted) != 1 || index.upserted[0].FileID != "f-1" {
		t.Fatalf("media_index 应写入 1 条: %#v", index.upserted)
	}
	if index.upserted[0].MatchStatus != domain.MatchMatched || index.upserted[0].AccountID != 7 {
		t.Fatalf("索引条目状态错误: %#v", index.upserted[0])
	}
	if tasks.updated[0].Stage != domain.StagePlayReady {
		t.Fatalf("最终阶段应为 play_ready: %s", tasks.updated[len(tasks.updated)-1].Stage)
	}
}

func TestRunP1KeywordFallback(t *testing.T) {
	s, _, _, _, drv := newTestService(t)
	callCount := 0
	s.pansearchSearch = func(_ context.Context, req pansearch.SearchRequest) ([]domain.PanSearchResult, error) {
		callCount++
		if callCount == 1 {
			return nil, nil // 中文关键词无结果
		}
		return []domain.PanSearchResult{
			{Title: "Avatar 4K", Source: "quark", ShareURL: "https://pan.quark.cn/s/xyz", Quality: "4K"},
		}, nil
	}
	s.pansearchCheck = func(context.Context, []pansearch.CheckItem) ([]pansearch.CheckResult, error) {
		return []pansearch.CheckResult{{URL: "https://pan.quark.cn/s/xyz", State: "ok"}}, nil
	}
	s.accountsFn = func(context.Context) []domain.Account {
		return []domain.Account{{ID: 8, DriverType: "Quark", IsActive: true}}
	}
	task := newTestTask()
	ok := s.runP1(context.Background(), task)
	if !ok || task.ResultSource != "quark" {
		t.Fatalf("英文关键词回退应命中: ok=%v source=%q", ok, task.ResultSource)
	}
	if callCount != 2 {
		t.Fatalf("应搜索 2 个关键词，实际 %d", callCount)
	}
	if len(drv.saved) != 1 {
		t.Fatalf("回退命中后应转存 1 次")
	}
}

func TestRunP1SaveFailureReturnsFalse(t *testing.T) {
	s, _, _, _, drv := newTestService(t)
	drv.saveErr = domain.Errf(domain.CodeInternal)

	task := newTestTask()
	ok := s.runP1(context.Background(), task)
	if ok {
		t.Fatalf("转存失败时 P1 应返回 false")
	}
	if task.Status == domain.ResolveDone {
		t.Fatalf("转存失败不应标记完成")
	}
}

func TestRunP1InvalidLinksSkipped(t *testing.T) {
	s, _, _, _, drv := newTestService(t)
	s.pansearchCheck = func(context.Context, []pansearch.CheckItem) ([]pansearch.CheckResult, error) {
		return []pansearch.CheckResult{{URL: "https://115.com/s/abc", State: "bad"}}, nil
	}
	task := newTestTask()
	ok := s.runP1(context.Background(), task)
	if ok {
		t.Fatalf("链接全部失效时 P1 应失败")
	}
	if len(drv.saved) != 0 {
		t.Fatalf("失效链接不应触发转存")
	}
}

// [V7 §6.4 / §27.4] A2: 转存阶段 SaveShare 失败必须显式上报 (StageSaveFailed)
// + ErrorMsg 记录最近错误, 防止 20 条全失败用户只见 not_found 不知根因.
func TestRunP1SaveFailuresReported(t *testing.T) {
	s, _, tasks, _, drv := newTestService(t)
	drv.saveErr = domain.Errorf(domain.CodeInternal, "网盘返回 401 鉴权失败")

	task := newTestTask()
	ok := s.runP1(context.Background(), task)
	if ok {
		t.Fatalf("转存全部失败时 P1 应返回 false")
	}
	if task.ErrorMsg == "" {
		t.Fatalf("task.ErrorMsg 应记录最近错误细节, got 空")
	}
	if !strings.Contains(task.ErrorMsg, "401") {
		t.Fatalf("task.ErrorMsg 应包含驱动错误内容, got %q", task.ErrorMsg)
	}
	// 断言 push 序列中出现 StageSaveFailed.
	sawSaveFailed := false
	for _, u := range tasks.updated {
		if u.Stage == domain.StageSaveFailed {
			sawSaveFailed = true
			if !strings.Contains(u.StageDetail, "转存") {
				t.Fatalf("StageSaveFailed detail 应说明转存失败, got %q", u.StageDetail)
			}
		}
	}
	if !sawSaveFailed {
		t.Fatalf("P1 应在转存失败累计后 push StageSaveFailed, 实际 push 序列: %v", stageSeq(tasks.updated))
	}
}

// [V7 §6.4 / §27.4] A3: 无网盘账号时 runP1 必须显式 StageNoAccount,
// 让 §27.4 健康面板可以显示"请先登录网盘账号"操作按钮, 而非静默 not_found.
func TestRunP1NoAccountReported(t *testing.T) {
	s, _, tasks, _, _ := newTestService(t, func(s *Service) {
		s.accountsFn = func(context.Context) []domain.Account { return nil }
		s.driverGet = func(context.Context, int64) (driver.Driver, error) {
			t.Fatal("无账号时不应查驱动")
			return nil, nil
		}
	})

	task := newTestTask()
	ok := s.runP1(context.Background(), task)
	if ok {
		t.Fatalf("无账号时 P1 应返回 false")
	}
	sawNoAccount := false
	for _, u := range tasks.updated {
		if u.Stage == domain.StageNoAccount {
			sawNoAccount = true
		}
	}
	if !sawNoAccount {
		t.Fatalf("P1 应 push StageNoAccount, 实际: %v", stageSeq(tasks.updated))
	}
}

// stageSeqP 把 push 序列翻译为人类可读串用于失败断言信息.
func stageSeq(updates []*domain.ResolveTask) []string {
	out := make([]string, 0, len(updates))
	for _, u := range updates {
		out = append(out, string(u.Stage))
	}
	return out
}

func TestRunP2MagnetCompleted(t *testing.T) {
	s, _, tasks, index, drv := newTestService(t)
	drv.offlineSt = &driver.OfflineTaskStatus{State: "completed", ProgressPct: 100, FileID: "off-1", FileName: "阿凡达.2009.1080p.mkv"}
	s.pansearchSearch = func(context.Context, pansearch.SearchRequest) ([]domain.PanSearchResult, error) {
		return []domain.PanSearchResult{
			{Title: "magnet", Source: "magnet", MagnetURL: "magnet:?xt=urn:btih:abc"},
		}, nil
	}

	task := newTestTask()
	ok := s.runP2(context.Background(), task)
	if !ok {
		t.Fatalf("P2 应成功: %s %s", task.Status, task.StageDetail)
	}
	if task.Status != domain.ResolveDone || task.ResultFileID != "off-1" {
		t.Fatalf("P2 完成状态错误: %#v", task)
	}
	if drv.offlineTask != "magnet:?xt=urn:btih:abc" {
		t.Fatalf("磁力 URL 未提交: %q", drv.offlineTask)
	}
	if len(index.upserted) != 1 || index.upserted[0].FileID != "off-1" {
		t.Fatalf("下载完成后应写 media_index: %#v", index.upserted)
	}
	if tasks.updated[0].OfflineTaskID != "task-1" {
		t.Fatalf("offline_task_id 应持久化到任务更新: %#v", tasks.updated[0])
	}
}

func TestRunP2NoMagnetResultReturnsFalse(t *testing.T) {
	s, _, _, _, _ := newTestService(t)
	s.pansearchSearch = func(context.Context, pansearch.SearchRequest) ([]domain.PanSearchResult, error) {
		return nil, nil
	}
	if ok := s.runP2(context.Background(), newTestTask()); ok {
		t.Fatalf("无磁力结果时 P2 应失败")
	}
}

func TestRunP2TimeoutKeepsRunning(t *testing.T) {
	s, _, tasks, _, drv := newTestService(t)
	drv.offlineSt = &driver.OfflineTaskStatus{State: "downloading", ProgressPct: 10}
	s.pansearchSearch = func(context.Context, pansearch.SearchRequest) ([]domain.PanSearchResult, error) {
		return []domain.PanSearchResult{
			{Title: "magnet", Source: "magnet", MagnetURL: "magnet:?xt=urn:btih:abc"},
		}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	task := newTestTask()
	ok := s.runP2(ctx, task)
	if !ok {
		t.Fatalf("超时后任务应保持接管状态（后台下载）")
	}
	if task.Status != domain.ResolveRunning || task.OfflineTaskID != "task-1" {
		t.Fatalf("超时后应保持 running + offline_task_id: %#v", task)
	}
	if len(tasks.updated) == 0 {
		t.Fatalf("应有任务更新记录")
	}
}

func TestEnsureSaveRootReusesExistingFolder(t *testing.T) {
	s, cfg, _, _, drv := newTestService(t)
	drv.folders = []domain.FileItem{{ID: "folder-xmedia", Name: "X-MEDIA", IsDir: true}}

	got, err := s.ensureSaveRoot(context.Background(), drv, 7, "pan115")
	if err != nil || got != "folder-xmedia" {
		t.Fatalf("应复用已有 X-MEDIA 目录: got=%q err=%v", got, err)
	}
	if v, ok := cfg.values["pan_115_save_root_7"]; !ok || v != "folder-xmedia" {
		t.Fatalf("save root 应持久化: %#v", cfg.values)
	}
}

func TestEnsureSaveRootCreatesWhenMissing(t *testing.T) {
	s, cfg, _, _, drv := newTestService(t)
	drv.folders = nil

	got, err := s.ensureSaveRoot(context.Background(), drv, 7, "pan115")
	if err != nil || got != "folder-xmedia" {
		t.Fatalf("应创建 X-MEDIA 目录: got=%q err=%v", got, err)
	}
	if v := cfg.values["pan_115_save_root_7"]; v != "folder-xmedia" {
		t.Fatalf("创建后应持久化 save root: %#v", cfg.values)
	}
}
