package domain

import "time"

type ResolveStage string

const (
	StageResolveStart   ResolveStage = "resolve_start"
	StageNASLookup      ResolveStage = "nas_lookup"
	StageNASHit         ResolveStage = "nas_hit"
	StagePanSearching   ResolveStage = "pan_searching"
	StagePanSearched    ResolveStage = "pan_searched"
	StageTransferring   ResolveStage = "transferring"
	StageResolvingLink  ResolveStage = "resolving_link"
	StagePlayReady      ResolveStage = "play_ready"
	StageMagnetDownload ResolveStage = "magnet_downloading"
	StageNotFound       ResolveStage = "not_found"
	StageError          ResolveStage = "error"
	// [V7 §6.4 / §27.4] 阶段可见性 — 用户认知痛点修复.
	// A2: 转存阶段失败累计到阈值后汇报, 让 UI 看到"20 条全失败"而非静默 not_found.
	StageSaveFailed ResolveStage = "save_failed"
	// A3: runP1/runP2 入口账号空时显式汇报, §27.4 健康面板据此显示
	// "请先登录网盘账号"按钮 (而非静默 not_found).
	StageNoAccount ResolveStage = "no_account"
)

type ResolveStatus string

const (
	ResolvePending ResolveStatus = "pending"
	ResolveRunning ResolveStatus = "running"
	ResolveDone    ResolveStatus = "done"
	ResolveFailed  ResolveStatus = "failed"
)

// ResolveTask 播放解析任务。
type ResolveTask struct {
	ID              int64         `json:"id"`
	ExternalID      int64         `json:"external_id"`
	ExternalSource  string        `json:"external_source"`
	MediaType       string        `json:"media_type"`
	Title           string        `json:"title"`
	Year            int           `json:"year"`
	Season          int           `json:"season"`
	Episode         int           `json:"episode"`
	Status          ResolveStatus `json:"status"`
	Stage           ResolveStage  `json:"stage"`
	StageDetail     string        `json:"stage_detail"`
	ProgressPct     int           `json:"progress_pct"`
	ResultSource    string        `json:"result_source"`
	ResultFileID    string        `json:"result_file_id"`
	ResultAccountID int64         `json:"result_account_id"`
	ResultFilePath  string        `json:"result_file_path"`
	OfflineTaskID   string        `json:"offline_task_id"` // [P0-2] P2 磁力下载的驱动任务 ID（启动恢复用）
	ErrorMsg        string        `json:"error_msg"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
}
