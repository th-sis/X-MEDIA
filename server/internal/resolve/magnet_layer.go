package resolve

import (
	"context"
	"fmt"
	"time"

	"xmedia/internal/domain"
	"xmedia/internal/driver"
	"xmedia/internal/pansearch"
)

// runP2 执行 P2 磁力兜底（§6.5）：PanSou 搜 magnet/ed2k → 115 离线下载 → 轮询。
// 下载中服务重启时保留 running + offline_task_id（§28.2 恢复查询）。
func (s *Service) runP2(ctx context.Context, t *domain.ResolveTask) bool {
	accounts := s.activeAccounts(ctx)
	target, ok := sourceAccount(accounts, "pan115")
	if !ok {
		// 未登录 115：尝试其他支持离线下载的驱动
		for _, acc := range accounts {
			if acc.IsActive {
				target, ok = acc, true
				break
			}
		}
		if !ok {
			// [V7 §6.4 / §27.4] A3: 账号空也显式汇报, 让 UI 显示"请先登录网盘账号"操作.
			s.push(t, domain.StageNoAccount, "未配置网盘账号，无法进行 P2 磁力兜底", 0)
			return false
		}
	}
	magnetKw := buildMagnetKeyword(t)
	s.push(t, domain.StagePanSearching, fmt.Sprintf("搜索磁力资源：%s ...", magnetKw), 40)
	results, err := s.pansearchSearch(ctx, pansearch.SearchRequest{
		Keyword:    magnetKw,
		CloudTypes: []string{"magnet", "ed2k"},
	})
	if err != nil || len(results) == 0 {
		return false
	}
	var best *domain.PanSearchResult
	for i := range results {
		if results[i].MagnetURL != "" {
			best = &results[i]
			break
		}
	}
	if best == nil {
		return false
	}
	drv, err := s.driverGet(ctx, target.ID)
	if err != nil {
		return false
	}
	od, ok := drv.(driver.OfflineDownloader)
	if !ok {
		return false
	}
	taskID, err := od.AddOfflineTask(ctx, best.MagnetURL)
	if err != nil {
		return false
	}
	t.OfflineTaskID = taskID
	t.Stage = domain.StageMagnetDownload
	t.StageDetail = "云下载中（磁力 0%）..."
	t.ProgressPct = 75
	_ = s.tasks.Update(context.Background(), t)

	return s.pollOfflineDownload(ctx, t, od, taskID, target)
}

// pollOfflineDownload 轮询 115 离线任务直至完成/失败/超时（§6.5）。
// 超时保持 running（§28.2 启动恢复可重新接管）。
func (s *Service) pollOfflineDownload(ctx context.Context, t *domain.ResolveTask, od driver.OfflineDownloader, taskID string, target domain.Account) bool {
	pollCtx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-pollCtx.Done():
			// 保持 running：启动恢复 §28.2 会查询 115 任务状态
			t.StageDetail = "云下载进行中（后台）"
			_ = s.tasks.Update(context.Background(), t)
			return true
		case <-ticker.C:
			st, err := od.GetOfflineTaskStatus(ctx, taskID)
			if err != nil {
				continue
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
					return true
				}
				return false
			case "failed":
				t.StageDetail = "云下载失败"
				_ = s.tasks.Update(context.Background(), t)
				return false
			default:
				detail := fmt.Sprintf("云下载中（磁力 %d%%）...", st.ProgressPct)
				if st.Speed != "" {
					detail = fmt.Sprintf("云下载中（磁力 %d%% · %s）...", st.ProgressPct, st.Speed)
				}
				t.StageDetail = detail
				t.ProgressPct = 75 + st.ProgressPct/4
				_ = s.tasks.Update(context.Background(), t)
				s.pushStageBroadcast(t)
			}
		}
	}
}

func (s *Service) cancelOffline(ctx context.Context, t *domain.ResolveTask) {
	if t.OfflineTaskID == "" {
		return
	}
	accounts := s.activeAccounts(ctx)
	target, ok := sourceAccount(accounts, "pan115")
	if !ok {
		return
	}
	drv, err := s.driverGet(ctx, target.ID)
	if err != nil {
		return
	}
	if od, ok := drv.(driver.OfflineDownloader); ok {
		_ = od.CancelOfflineTask(ctx, t.OfflineTaskID)
	}
}
