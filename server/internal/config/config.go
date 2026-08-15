package config

import (
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	DataDir    string
	StrmDir    string
	DBPath     string
	ListenAddr string
	LogLevel   string
}

func Default() Config {
	dataDir := "./data"
	return Config{
		DataDir:    dataDir,
		StrmDir:    "./strm",
		DBPath:     filepath.Join(dataDir, "xmedia.db"),
		ListenAddr: ":8080",
		LogLevel:   "info",
	}
}

// StrmDirForData 返回数据目录同级的 STRM 目录。
func StrmDirForData(dataDir string) string {
	return filepath.Join(filepath.Dir(filepath.Clean(dataDir)), "strm")
}

// Load 在默认值基础上应用 XMEDIA_* 环境变量覆盖。
func Load() Config {
	c := Default()
	if v := strings.TrimSpace(os.Getenv("XMEDIA_DATA_DIR")); v != "" {
		c.DataDir = v
		c.DBPath = filepath.Join(v, "xmedia.db")
		if strings.TrimSpace(os.Getenv("XMEDIA_STRM_DIR")) == "" {
			c.StrmDir = StrmDirForData(v)
		}
	}
	if v := strings.TrimSpace(os.Getenv("XMEDIA_STRM_DIR")); v != "" {
		c.StrmDir = v
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
