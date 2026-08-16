package websocket

import (
	"net/http"
	"sync"
)

// Hub 维护所有活跃 WebSocket 连接。
type Hub struct {
	mu      sync.RWMutex
	clients map[*Client]struct{}
}

func NewHub() *Hub {
	return &Hub{clients: map[*Client]struct{}{}}
}

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
