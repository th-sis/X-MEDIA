package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadStrmDirDefaultsBesideData(t *testing.T) {
	t.Setenv("XMEDIA_DATA_DIR", "/app/data")
	t.Setenv("XMEDIA_STRM_DIR", "")
	_ = os.Unsetenv("XMEDIA_STRM_DIR")
	cfg := Load()
	if want := filepath.Join("/app", "strm"); cfg.StrmDir != want {
		t.Fatalf("StrmDir=%q, want %q", cfg.StrmDir, want)
	}
	if cfg.DataDir != "/app/data" {
		t.Fatalf("DataDir=%q", cfg.DataDir)
	}
}

func TestLoadStrmDirExplicitOverride(t *testing.T) {
	t.Setenv("XMEDIA_DATA_DIR", "/app/data")
	t.Setenv("XMEDIA_STRM_DIR", "/custom/strm")
	cfg := Load()
	if cfg.StrmDir != "/custom/strm" {
		t.Fatalf("StrmDir=%q, want /custom/strm", cfg.StrmDir)
	}
}

func TestLoadStrmDirDefaultsBesideCustomDataDir(t *testing.T) {
	t.Setenv("XMEDIA_DATA_DIR", "/srv/xmedia-state")
	t.Setenv("XMEDIA_STRM_DIR", "")
	cfg := Load()
	if want := filepath.Join("/srv", "strm"); cfg.StrmDir != want {
		t.Fatalf("StrmDir=%q, want %q", cfg.StrmDir, want)
	}
}

func TestDefaultLocalStrmBesideCWD(t *testing.T) {
	t.Setenv("XMEDIA_DATA_DIR", "")
	t.Setenv("XMEDIA_STRM_DIR", "")
	_ = os.Unsetenv("XMEDIA_DATA_DIR")
	_ = os.Unsetenv("XMEDIA_STRM_DIR")
	cfg := Load()
	if cfg.DataDir != "./data" || cfg.StrmDir != "./strm" {
		t.Fatalf("got data=%q strm=%q", cfg.DataDir, cfg.StrmDir)
	}
	if filepath.Base(cfg.StrmDir) != "strm" {
		t.Fatalf("unexpected strm base: %q", cfg.StrmDir)
	}
}
