package resolve

import (
	"context"
	"testing"

	"xmedia/internal/domain"
	"xmedia/internal/driver"
	"xmedia/internal/pansearch"
	"xmedia/internal/playback"
)

// recoverTaskRepo 支持 ListActive 的 mock。
type recoverTaskRepo struct {
	*mockTaskRepo
	active []*domain.ResolveTask
}

func (r *recoverTaskRepo) ListActive(context.Context) ([]*domain.ResolveTask, error) {
	return r.active, nil
}

func newRecoverService(t *testing.T, active []*domain.ResolveTask) (*Service, *recoverTaskRepo, *mockIndexRepo, *mockDriver) {
	t.Helper()
	cfg := &mockConfigRepo{values: map[string]string{}}
	tasks := &recoverTaskRepo{mockTaskRepo: newMockTaskRepo(), active: active}
	index := &mockIndexRepo{}
	drv := &mockDriver{saveResult: &driver.ShareResult{FileID: "f-1", FileName: "x.mkv"}}
	s := &Service{
		tasks:           tasks,
		mediaIndex:      index,
		subs:            &mockSubsRepo{},
		configs:         cfg,
		mediaLibrary:    &mockLibraryRepo{media: &domain.MediaLibrary{TitleOrig: "Avatar"}},
		pansearchHealth: func(context.Context) bool { return true },
		loggedInDrivers: func(context.Context) []string { return nil },
		nasConfigured:   func(context.Context) bool { return false },
		indexCountFn:    func(context.Context) (int, error) { return 0, nil },
		pansearchSearch: func(context.Context, pansearch.SearchRequest) ([]domain.PanSearchResult, error) {
			return nil, nil
		},
		pansearchCheck: func(context.Context, []pansearch.CheckItem) ([]pansearch.CheckResult, error) {
			return nil, nil
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
	return s, tasks, index, drv
}

// TestRecoverPendingFails 重启时 pending 任务标记失败。
func TestRecoverPendingFails(t *testing.T) {
	pending := newTestTask()
	pending.Status = domain.ResolvePending
	s, tasks, _, _ := newRecoverService(t, []*domain.ResolveTask{pending})

	s.RecoverStartup(context.Background())
	if pending.Status != domain.ResolveFailed {
		t.Fatalf("pending 任务应标记失败，实际 %s", pending.Status)
	}
	_ = tasks
}

// TestRecoverMagnetCompleted 下载完成的任务恢复收尾（done + 索引写入）。
func TestRecoverMagnetCompleted(t *testing.T) {
	task := newTestTask()
	task.Status = domain.ResolveRunning
	task.Stage = domain.StageMagnetDownload
	task.OfflineTaskID = "task-1"
	s, _, index, drv := newRecoverService(t, []*domain.ResolveTask{task})
	drv.offlineSt = &driver.OfflineTaskStatus{State: "completed", ProgressPct: 100, FileID: "off-1", FileName: "阿凡达.mkv"}

	s.RecoverStartup(context.Background())
	if task.Status != domain.ResolveDone || task.ResultFileID != "off-1" {
		t.Fatalf("恢复后应完成: status=%s file=%q err=%q", task.Status, task.ResultFileID, task.ErrorMsg)
	}
	if len(index.upserted) != 1 || index.upserted[0].FileID != "off-1" {
		t.Fatalf("恢复完成应写 media_index: %#v", index.upserted)
	}
}

// TestRecoverMagnetFailed 下载失败的任务标记失败。
func TestRecoverMagnetFailed(t *testing.T) {
	task := newTestTask()
	task.Status = domain.ResolveRunning
	task.Stage = domain.StageMagnetDownload
	task.OfflineTaskID = "task-1"
	s, _, _, drv := newRecoverService(t, []*domain.ResolveTask{task})
	drv.offlineSt = &driver.OfflineTaskStatus{State: "failed", ProgressPct: 30}

	s.RecoverStartup(context.Background())
	if task.Status != domain.ResolveFailed || task.ErrorMsg == "" {
		t.Fatalf("失败任务应标记 failed: status=%s err=%q", task.Status, task.ErrorMsg)
	}
}

// TestRecoverMagnetStillDownloading 下载中的任务保持 running（挂起轮询接管）。
func TestRecoverMagnetStillDownloading(t *testing.T) {
	task := newTestTask()
	task.Status = domain.ResolveRunning
	task.Stage = domain.StageMagnetDownload
	task.OfflineTaskID = "task-1"
	s, _, _, drv := newRecoverService(t, []*domain.ResolveTask{task})
	drv.offlineSt = &driver.OfflineTaskStatus{State: "downloading", ProgressPct: 40}

	s.RecoverStartup(context.Background())
	if task.Status != domain.ResolveRunning {
		t.Fatalf("下载中任务应保持 running: status=%s", task.Status)
	}
}

// TestRecoverNoOfflineTaskIDFails 无 offline_task_id 的磁力任务无法恢复。
func TestRecoverNoOfflineTaskIDFails(t *testing.T) {
	task := newTestTask()
	task.Status = domain.ResolveRunning
	task.Stage = domain.StageMagnetDownload
	s, _, _, _ := newRecoverService(t, []*domain.ResolveTask{task})

	s.RecoverStartup(context.Background())
	if task.Status != domain.ResolveFailed {
		t.Fatalf("无任务 ID 应标记失败: status=%s", task.Status)
	}
}

// TestRecoverRunningOtherStageFails 非磁力阶段的 running 任务标记失败。
func TestRecoverRunningOtherStageFails(t *testing.T) {
	task := newTestTask()
	task.Status = domain.ResolveRunning
	task.Stage = domain.StagePanSearching
	s, _, _, _ := newRecoverService(t, []*domain.ResolveTask{task})

	s.RecoverStartup(context.Background())
	if task.Status != domain.ResolveFailed {
		t.Fatalf("不可恢复阶段应标记失败: status=%s", task.Status)
	}
}
