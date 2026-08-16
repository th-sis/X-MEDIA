package driver

import "context"

// ShareRequest 是分享链接转存的统一输入（§6.8.5）。
type ShareRequest struct {
	ShareURL       string // 分享链接
	Password       string // 提取码（可为空）
	TargetParentID string // 保存到网盘的指定目录 ID
}

// ShareResult 是转存成功后的结果（§6.8.5）。
type ShareResult struct {
	FileID    string // 转存后的文件 ID
	FileName  string // 文件名
	FileSize  int64  // 文件大小（字节）
	FileCount int    // 转存的文件数量
}

// ShareSaver 支持分享链接转存的驱动实现此接口（P1 盘搜转存专用，
// 与 crosstransfer 的跨盘 hash 秒传无关）。
type ShareSaver interface {
	SaveShare(ctx context.Context, req ShareRequest) (*ShareResult, error)
}

// OfflineTaskStatus 是磁力离线任务的归一化状态（§6.8.5）。
type OfflineTaskStatus struct {
	State       string // downloading/completed/failed
	ProgressPct int
	Speed       string // 如 "12.5 MB/s"
	FileID      string // 下载完成后的文件 ID
	FileName    string
}

// OfflineDownloader 支持磁力离线下载的驱动实现此接口（P2 磁力兜底专用，
// 与 LitePan 原有的 OfflineURLDownloader 管理面接口相互独立）。
type OfflineDownloader interface {
	AddOfflineTask(ctx context.Context, magnetURL string) (taskID string, err error)
	GetOfflineTaskStatus(ctx context.Context, taskID string) (*OfflineTaskStatus, error)
	CancelOfflineTask(ctx context.Context, taskID string) error
}
