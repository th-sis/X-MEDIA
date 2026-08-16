package config

import (
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	DataDir    string
	DBPath     string
	ListenAddr string
	LogLevel   string
}

func Default() Config {
	dataDir := "./data"
	return Config{
		DataDir:    dataDir,
		DBPath:     filepath.Join(dataDir, "xmedia.db"),
		ListenAddr: ":38088", // V7 端口约定，与 client 默认 / docker-compose / README 保持一致
		LogLevel:   "info",
	}
}

// Load 在默认值基础上应用 XMEDIA_* 环境变量覆盖。
func Load() Config {
	c := Default()
	if v := strings.TrimSpace(os.Getenv("XMEDIA_DATA_DIR")); v != "" {
		c.DataDir = v
		c.DBPath = filepath.Join(v, "xmedia.db")
	}
	if v := os.Getenv("XMEDIA_DB_PATH"); v != "" {
		c.DBPath = v
	}
	if v := os.Getenv("XMEDIA_LISTEN"); v != "" {
		c.ListenAddr = v
	}
	if v := os.Getenv("XMEDIA_LOG_LEVEL"); v != "" {
		c.LogLevel = strings.ToLower(v)
	}
	return c
}
