package config

import (
	"path/filepath"
	"testing"
)

func TestLoadDataDirSetsDBPathBesideData(t *testing.T) {
	t.Setenv("XMEDIA_DATA_DIR", "/app/data")
	cfg := Load()
	if cfg.DataDir != "/app/data" {
		t.Fatalf("DataDir=%q, want /app/data", cfg.DataDir)
	}
	if want := filepath.Join("/app/data", "xmedia.db"); cfg.DBPath != want {
		t.Fatalf("DBPath=%q, want %q", cfg.DBPath, want)
	}
}

func TestLoadDBPathExplicitOverride(t *testing.T) {
	t.Setenv("XMEDIA_DATA_DIR", "/app/data")
	t.Setenv("XMEDIA_DB_PATH", "/custom/xmedia.db")
	cfg := Load()
	if cfg.DBPath != "/custom/xmedia.db" {
		t.Fatalf("DBPath=%q, want /custom/xmedia.db", cfg.DBPath)
	}
	if cfg.DataDir != "/app/data" {
		t.Fatalf("DataDir=%q, want /app/data", cfg.DataDir)
	}
}

func TestLoadListenOverride(t *testing.T) {
	t.Setenv("XMEDIA_LISTEN", ":9090")
	cfg := Load()
	if cfg.ListenAddr != ":9090" {
		t.Fatalf("ListenAddr=%q, want :9090", cfg.ListenAddr)
	}
}

func TestDefaultLocalLayout(t *testing.T) {
	t.Setenv("XMEDIA_DATA_DIR", "")
	t.Setenv("XMEDIA_DB_PATH", "")
	t.Setenv("XMEDIA_LISTEN", "")
	t.Setenv("XMEDIA_LOG_LEVEL", "")
	cfg := Load()
	if cfg.DataDir != "./data" {
		t.Fatalf("DataDir=%q, want ./data", cfg.DataDir)
	}
	if want := filepath.Join("./data", "xmedia.db"); cfg.DBPath != want {
		t.Fatalf("DBPath=%q, want %q", cfg.DBPath, want)
	}
	if cfg.ListenAddr != ":8080" {
		t.Fatalf("ListenAddr=%q, want :8080", cfg.ListenAddr)
	}
}
