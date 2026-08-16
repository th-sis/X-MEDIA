package pan115open

import (
	"context"
	"strings"

	"xmedia/internal/domain"
	"xmedia/internal/driver"
)

// AddOfflineTask 实现 driver.OfflineDownloader（P2 磁力兜底专用）。
// 复用 115 Open API 的磁力离线能力，以 info_hash 作为任务标识。
func (d *Driver) AddOfflineTask(ctx context.Context, magnetURL string) (string, error) {
	magnetURL = strings.TrimSpace(magnetURL)
	if magnetURL == "" {
		return "", domain.Errorf(domain.CodeValidation, "磁力链接不能为空")
	}
	results, err := d.AddOfflineURLs(ctx, driver.OfflineURLRequest{
		URLs:     []string{magnetURL},
		ParentID: d.normalizeParent("0"),
	})
	if err != nil {
		return "", err
	}
	if len(results) == 0 {
		return "", domain.Errorf(domain.CodeDriverError, "115 未返回离线任务提交结果")
	}
	item := results[0]
	if !item.Success || strings.TrimSpace(item.InfoHash) == "" {
		if strings.TrimSpace(item.Message) != "" {
			return "", domain.Errorf(domain.CodeDriverError, "115 离线任务提交失败：%s", item.Message)
		}
		return "", domain.Errorf(domain.CodeDriverError, "115 离线任务提交失败")
	}
	return strings.TrimSpace(item.InfoHash), nil
}

// GetOfflineTaskStatus 实现 driver.OfflineDownloader：按 info_hash 轮询任务状态。
func (d *Driver) GetOfflineTaskStatus(ctx context.Context, taskID string) (*driver.OfflineTaskStatus, error) {
	hash := strings.TrimSpace(taskID)
	if hash == "" {
		return nil, domain.Errorf(domain.CodeValidation, "离线任务 ID 不能为空")
	}
	updates, err := d.RefreshOfflineTasks(ctx, []driver.OfflineTaskRef{{InfoHash: hash}})
	if err != nil {
		return nil, err
	}
	if len(updates) == 0 {
		// 任务列表中未找到：可能已被清理或尚未入列，按下载中处理
		return &driver.OfflineTaskStatus{State: "downloading", ProgressPct: 0}, nil
	}
	u := updates[0]
	switch u.Status {
	case driver.OfflineStatusSuccess:
		return &driver.OfflineTaskStatus{
			State:       "completed",
			ProgressPct: 100,
			FileID:      u.FileID,
			FileName:    u.Name,
		}, nil
	case driver.OfflineStatusFailed:
		return &driver.OfflineTaskStatus{State: "failed", ProgressPct: u.Progress}, nil
	default:
		return &driver.OfflineTaskStatus{
			State:       "downloading",
			ProgressPct: u.Progress,
			FileName:    u.Name,
		}, nil
	}
}

// CancelOfflineTask 实现 driver.OfflineDownloader：删除 115 离线任务（不删除已下载文件）。
func (d *Driver) CancelOfflineTask(ctx context.Context, taskID string) error {
	return d.DeleteOfflineTask(ctx, driver.OfflineTaskRef{InfoHash: strings.TrimSpace(taskID)}, false)
}
