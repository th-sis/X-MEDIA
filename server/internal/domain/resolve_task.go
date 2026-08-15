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
	ID              int64        `json:"id"`
	ExternalID      int64        `json:"external_id"`
	ExternalSource  string       `json:"external_source"`
	MediaType       string       `json:"media_type"`
	Title           string       `json:"title"`
	Year            int          `json:"year"`
	Season          int          `json:"season"`
	Episode         int          `json:"episode"`
	Status          ResolveStatus `json:"status"`
	Stage           ResolveStage `json:"stage"`
	StageDetail     string       `json:"stage_detail"`
	ProgressPct     int          `json:"progress_pct"`
	ResultSource    string       `json:"result_source"`
	ResultFileID    string       `json:"result_file_id"`
	ResultAccountID int64        `json:"result_account_id"`
	ResultFilePath  string       `json:"result_file_path"`
	ErrorMsg        string       `json:"error_msg"`
	CreatedAt       time.Time    `json:"created_at"`
	UpdatedAt       time.Time    `json:"updated_at"`
}
