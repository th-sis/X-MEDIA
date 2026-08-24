package api

import (
	"fmt"
	"net/http"
	"runtime/debug"
)

// recoverWithLog [V7 §21.4 / §27.2 可观测性] 替代 chimw.Recoverer:
// panic 时先向 logx 写 ERROR (method/path/panic 值/堆栈, 带 request_id),
// 再返回 500. 真机实测教训: capabilities 因 nil panic 返回 500 时
// /api/logs 零 ERROR 记录, 排查只能靠本地复现.
//
// http.ErrAbortHandler 是 net/http 约定的"静默中止"信号 (连接被服务端
// 主动放弃, 非错误), 与 chi Recoverer 行为一致: 原样 re-panic 不记日志.
func (h *Handler) recoverWithLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rv := recover(); rv != nil {
				if rv == http.ErrAbortHandler {
					panic(rv)
				}
				log := requestLogger(r.Context())
				log.Error("HTTP panic recovered",
					"method", r.Method,
					"path", r.URL.Path,
					"panic_value", fmt.Sprint(rv),
					"stack", string(debug.Stack()),
				)
				w.WriteHeader(http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
