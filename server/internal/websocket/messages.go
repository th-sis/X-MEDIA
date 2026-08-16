package websocket

import (
	"encoding/json"

	"xmedia/internal/domain"
)

// 消息类型。
const (
	TypeHealthCheck      = "health_check"
	TypeResolveStage     = "resolve_stage"
	TypeResolveComplete  = "resolve_complete"
	TypeResolveFailed    = "resolve_failed"
	TypeDownloadProgress = "download_progress"
	TypeSubReady         = "subscription_ready"
	TypeIndexStatus      = "index_status"
	TypeNotification     = "notification"
	TypeAccountAuthFail  = "account_auth_failed"
	TypeServerStopping   = "server_stopping"
	TypeCapabilities     = "capabilities"
)

// Message 统一 WS 消息结构。
type Message struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// ResolveStagePayload 解析阶段推送。
type ResolveStagePayload struct {
	TaskID      int64  `json:"task_id"`
	ExternalID  int64  `json:"external_id"`
	Stage       string `json:"stage"`
	Detail      string `json:"detail"`
	ProgressPct int    `json:"progress_pct"`
}

// ResolveCompletePayload 解析完成。
type ResolveCompletePayload struct {
	TaskID    int64  `json:"task_id"`
	StreamURL string `json:"stream_url"`
	Source    string `json:"source"`
	FileName  string `json:"file_name"`
	FileID    string `json:"file_id"`
	Ticket    string `json:"ticket"`
}

// ResolveFailedPayload 解析失败。
type ResolveFailedPayload struct {
	TaskID     int64  `json:"task_id"`
	Reason     string `json:"reason"`
	Suggestion string `json:"suggestion"`
	Stage      string `json:"stage"`
}

// IndexStatusPayload 索引引擎进度（§9.7.1 WS index_status）。
type IndexStatusPayload struct {
	AccountID   int64  `json:"account_id"`
	Scope       string `json:"scope"` // nas / pan
	Phase       string `json:"phase"` // A / B / C / D
	Status      string `json:"status"`
	Processed   int    `json:"processed"`
	Total       int    `json:"total"`
	Matched     int    `json:"matched"`
	Unconfirmed int    `json:"unconfirmed"`
	Orphaned    int    `json:"orphaned"`
	RatePerSec  int    `json:"rate_per_sec"`
	FileCount   int    `json:"file_count"`
	ErrorMsg    string `json:"error_msg"`
}

// SubReadyPayload 订阅自动搜寻命中（§20 subscription_ready）。
type SubReadyPayload struct {
	ExternalID     int64  `json:"external_id"`
	ExternalSource string `json:"external_source"`
	MediaType      string `json:"media_type"`
	Title          string `json:"title"`
	Year           int    `json:"year"`
	ResultSource   string `json:"result_source"`
}

// HealthPayload 健康检查首条消息。
type HealthPayload struct {
	DB              string              `json:"db"`
	TMDB            string              `json:"tmdb"`
	Pansearch       string              `json:"pansearch"`
	Accounts        []AccountHealth     `json:"accounts"`
	NAS             NASHealth           `json:"nas"`
	Index           IndexHealth         `json:"index"`
	Capabilities    domain.Capabilities `json:"capabilities"`
	Version         string              `json:"version"`
	ServerStartedAt string              `json:"server_started_at"`
	Overall         string              `json:"overall"`
}

type AccountHealth struct {
	Driver string `json:"driver"`
	Status string `json:"status"`
	Label  string `json:"label"`
}

type NASHealth struct {
	Status    string `json:"status"`
	Path      string `json:"path"`
	FileCount int    `json:"file_count"`
}

type IndexHealth struct {
	TotalFiles int    `json:"total_files"`
	NASPhase   string `json:"nas_phase"`
}
