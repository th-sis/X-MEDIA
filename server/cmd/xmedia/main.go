package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"xmedia/internal/app"
	"xmedia/internal/config"
	"xmedia/internal/logx"

	_ "xmedia/drivers" //
)

const shutdownBudget = 25 * time.Second

func main() {
	cfg := config.Load()

	listen := flag.String("listen", cfg.ListenAddr, "HTTP 监听地址")
	dataDir := flag.String("data-dir", cfg.DataDir, "数据目录")
	flag.Parse()

	cfg.ListenAddr = *listen
	if *dataDir != cfg.DataDir {
		cfg.DataDir = *dataDir
		cfg.DBPath = filepath.Join(*dataDir, "xmedia.db")
	}

	logs, err := logx.New(logx.Options{
		Dir:   filepath.Join(cfg.DataDir, "log"),
		Level: cfg.LogLevel,
	})
	if err != nil {
		// 日志目录创建失败时降级为 stdout-only，不阻塞服务启动。
		// 常见原因：/app/data 挂载点权限不足（NAS SMB/CIFS 挂载常见问题）。
		// 管理后台仍可正常访问；日志会丢失落盘，但服务可运行。
		fmt.Fprintf(os.Stderr, "WARN: log dir init failed: %v; falling back to stdout-only\n", err)
		logs, _ = logx.New(logx.Options{DisableFile: true, Level: cfg.LogLevel})
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	a, err := app.New(ctx, app.Options{Config: cfg, Logs: logs})
	if err != nil {
		logs.Root().Error("startup failed", "err", err)
		os.Exit(1)
	}

	runErr := a.Run(ctx)
	stop()

	shCtx, cancel := context.WithTimeout(context.Background(), shutdownBudget)
	defer cancel()
	if err := a.Shutdown(shCtx); err != nil {
		logs.Root().Error("shutdown error", "err", err)
		os.Exit(1)
	}
	if runErr != nil {
		logs.Root().Error("run error", "err", runErr)
		os.Exit(1)
	}
}
