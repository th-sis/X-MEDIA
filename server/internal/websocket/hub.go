package websocket

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

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

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// Hub 维护所有活跃 WebSocket 连接。
type Hub struct {
	mu      sync.RWMutex
	clients map[*Client]struct{}
}

func NewHub() *Hub {
	return &Hub{clients: map[*Client]struct{}{}}
}

// Client 单个连接。
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
	once sync.Once
}

const (
	writeWait  = 10 * time.Second
	pongWait   = 90 * time.Second
	pingPeriod = 30 * time.Second
)

// Handle 升级并注册一个连接，阻塞直到连接关闭。
func (h *Hub) Handle(w http.ResponseWriter, r *http.Request) {
	h.HandleWith(w, r, nil)
}

// HandleWith 升级连接并在注册后回调 onConnect（用于发送 health_check 首条消息）。
func (h *Hub) HandleWith(w http.ResponseWriter, r *http.Request, onConnect func(*Client)) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	c := &Client{hub: h, conn: conn, send: make(chan []byte, 64)}
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()

	if onConnect != nil {
		onConnect(c)
	}

	go c.writeLoop()
	c.readLoop()

	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
	c.once.Do(func() { close(c.send) })
	_ = conn.Close()
}

func (c *Client) readLoop() {
	c.conn.SetReadLimit(4096)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (c *Client) writeLoop() {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// Send 向该连接发送一条消息（非阻塞）。
func (c *Client) Send(msgType string, payload any) {
	raw, err := json.Marshal(payload)
	if err != nil {
		raw, _ = json.Marshal(map[string]string{"error": err.Error()})
	}
	msg := Message{Type: msgType, Payload: raw}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	select {
	case c.send <- data:
	default:
	}
}

// Broadcast 广播消息给所有连接。
func (h *Hub) Broadcast(msgType string, payload any) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		c.Send(msgType, payload)
	}
}

// ClientCount 当前连接数。
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
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
