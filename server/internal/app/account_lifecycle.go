package app

import (
	"context"
	"fmt"

	"xmedia/internal/cacheretention"
	"xmedia/internal/domain"
	"xmedia/internal/offlinedownload"
)

// accountLifecycle 账号生命周期钩子（§13.1 裁剪后仅保留缓存保鲜与离线下载清理）。
type accountLifecycle struct {
	retention *cacheretention.Coordinator
	offline   *offlinedownload.Service
}

func (a accountLifecycle) OnAccountDisabled(ctx context.Context, accountID int64) {
	if accountID <= 0 {
		return
	}
	if a.retention != nil {
		_, _ = a.retention.PauseByAccount(ctx, accountID, domain.PauseReasonAccountDisabled, "关联的账号已禁用")
	}
}

func (a accountLifecycle) OnAccountEnabled(ctx context.Context, accountID int64) {
	if accountID <= 0 {
		return
	}
	if a.retention != nil {
		_, _ = a.retention.ResumeByAccount(ctx, accountID)
	}
}

func (a accountLifecycle) OnAccountDeleted(ctx context.Context, accountID int64) error {
	if accountID <= 0 {
		return nil
	}
	if a.retention != nil {
		if _, err := a.retention.RemoveTasksByAccount(ctx, accountID); err != nil {
			return fmt.Errorf("清理缓存保持任务失败: %w", err)
		}
	}
	if a.offline != nil {
		if _, err := a.offline.RemoveTasksByAccount(ctx, accountID); err != nil {
			return fmt.Errorf("清理离线下载任务失败: %w", err)
		}
	}
	return nil
}
