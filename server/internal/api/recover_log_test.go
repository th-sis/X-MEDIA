// [V7 §21.4 / §27.2 可观测性回归] chi Recoverer 会把 500 panic 吞成静默
// 响应 — 真机实测时 /api/capabilities 因 nasPathsKnown nil panic 返回 500,
// /api/logs 里却零 ERROR 记录, 排查只能靠本地复现. 本文件提供带日志的
// recoverer: panic 时写 ERROR (method/path/panic 值/堆栈) 后再返回 500.

package api

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newBufferLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func ctxWithLogger(req *http.Request, log *slog.Logger) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), requestLoggerCtxKey{}, log))
}

func TestRecoverWithLog_WritesErrorAndReturns500(t *testing.T) {
	var buf bytes.Buffer
	log := newBufferLogger(&buf)

	var h Handler
	mw := h.recoverWithLog(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom: nasPathsKnown is nil")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/capabilities", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, ctxWithLogger(req, log))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("panic 后应返回 500, got %d", rec.Code)
	}
	out := buf.String()
	for _, want := range []string{"HTTP panic recovered", "boom", "/api/capabilities", "panic recovered"} {
		if !strings.Contains(out, want) {
			t.Fatalf("ERROR 日志应包含 %q, 实际: %s", want, out)
		}
	}
	if !strings.Contains(out, "goroutine") && !strings.Contains(out, "recoverWithLog") {
		t.Fatalf("ERROR 日志应包含堆栈信息, 实际: %s", out)
	}
}

// http.ErrAbortHandler 是框架约定的"静默中止"信号, recoverer 必须放行不吞.
func TestRecoverWithLog_PreservesErrAbortHandler(t *testing.T) {
	var buf bytes.Buffer
	log := newBufferLogger(&buf)

	var h Handler
	mw := h.recoverWithLog(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(http.ErrAbortHandler)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/x", nil)
	rec := httptest.NewRecorder()

	defer func() {
		if rv := recover(); rv != nil {
			if rv != http.ErrAbortHandler {
				t.Fatalf("应原样放行 ErrAbortHandler, got %v", rv)
			}
			if strings.Contains(buf.String(), "HTTP panic recovered") {
				t.Fatalf("ErrAbortHandler 不应写 ERROR 日志")
			}
		}
	}()
	mw.ServeHTTP(rec, ctxWithLogger(req, log))
	t.Fatal("ErrAbortHandler 应向上传播 panic (不应到达这里)")
}

// 正常请求不受影响.
func TestRecoverWithLog_PassesThroughHealthyRequests(t *testing.T) {
	var buf bytes.Buffer
	log := newBufferLogger(&buf)

	var h Handler
	mw := h.recoverWithLog(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/ok", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, ctxWithLogger(req, log))

	if rec.Code != http.StatusTeapot {
		t.Fatalf("正常请求应透传, got %d", rec.Code)
	}
	if strings.Contains(buf.String(), "HTTP panic recovered") {
		t.Fatalf("正常请求不应产生 panic 日志")
	}
}
