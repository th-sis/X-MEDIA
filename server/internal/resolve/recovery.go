package resolve

import (
	"context"

	"xmedia/internal/domain"
	"xmedia/internal/driver"
)

// RecoverStartup 启动恢复（§28.2）：扫描重启前遗留的 active 任务。
//   - pending：尚未开跑，直接标记 failed（服务重启中断）
//   - running + StageMagnetDownload + OfflineTaskID：查 115 任务状态，
//     已完成则收尾 done，下载中则重新挂起轮询，失败/不可恢复标记 failed
//   - running 其他阶段：无法恢复（无幂等重放），标记 failed
func (s *Service) RecoverStartup(ctx context.Context) {
	if s.tasks == nil {
		return
	}
	active, err := s.tasks.ListActive(ctx)
	if err != nil {
		return
	}
	for _, t := range active {
		if t == nil {
			continue
		}
		switch t.Status {
		case domain.ResolvePending:
			t.Status = domain.ResolveFailed
			t.ErrorMsg = "服务重启中断，请重新触发"
			_ = s.tasks.Update(ctx, t)
		case domain.ResolveRunning:
			s.recoverRunning(ctx, t)
		}
	}
}

func (s *Service) recoverRunning(ctx context.Context, t *domain.ResolveTask) {
	if t.Stage != domain.StageMagnetDownload || t.OfflineTaskID == "" {
		t.Status = domain.ResolveFailed
		t.ErrorMsg = "服务重启中断，请重新触发"
		_ = s.tasks.Update(ctx, t)
		return
	}
	// P2 磁力下载中：按 offline_task_id 查 115 任务
	target, ok := s.recoverTarget(ctx, "pan115")
	if !ok {
		t.Status = domain.ResolveFailed
		t.ErrorMsg = "下载任务无法恢复：115 账号未登录"
		_ = s.tasks.Update(ctx, t)
		return
	}
	drv, err := s.driverGet(ctx, target.ID)
	if err != nil {
		s.failTask(t, "下载任务无法恢复：驱动不可用")
		return
	}
	od, ok := drv.(driver.OfflineDownloader)
	if !ok {
		s.failTask(t, "下载任务无法恢复：驱动不支持离线下载")
		return
	}
	st, err := od.GetOfflineTaskStatus(ctx, t.OfflineTaskID)
	if err != nil {
		// 查询失败不直接判死：可能瞬时错误，挂起轮询重试
		go s.pollOfflineDownload(context.Background(), t, od, t.OfflineTaskID, target)
		return
	}
	switch st.State {
	case "completed":
		if st.FileID != "" {
			s.indexSavedFile(ctx, t, target, "pan115", &driver.ShareResult{
				FileID:   st.FileID,
				FileName: st.FileName,
			})
		}
		t.ResultAccountID = target.ID
		t.ResultFilePath = st.FileName
		ticket, serr := s.signer.Sign(ctx, ticketClaimsFor(t, st.FileID, "pan115", target.ID), 0)
		if serr == nil {
			s.complete(t, "pan115", st.FileID, ticket, st.FileName)
			return
		}
		s.failTask(t, "下载完成但签发播放票据失败")
	case "failed":
		s.failTask(t, "云下载失败")
	default:
		// 仍在下载：重新挂起轮询
		go s.pollOfflineDownload(context.Background(), t, od, t.OfflineTaskID, target)
	}
}

// recoverTarget 从 active 账号中找目标网盘账号（恢复路径）。
func (s *Service) recoverTarget(ctx context.Context, source string) (domain.Account, bool) {
	accounts := s.activeAccounts(ctx)
	for _, acc := range accounts {
		if acc.IsActive && driverSourceOf(acc.DriverType) == source {
			return acc, true
		}
	}
	return domain.Account{}, false
}

func (s *Service) failTask(t *domain.ResolveTask, reason string) {
	t.Status = domain.ResolveFailed
	t.ErrorMsg = reason
	t.StageDetail = reason
	_ = s.tasks.Update(context.Background(), t)
}
