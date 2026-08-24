// [V7 §9.7.1 / §27.4 实测回归] 真机抓到: 全部媒体源路径不可达时,
// 扫描静默跑完 Phase C 标记 done/error_msg 空 → 界面"无反应".
// 契约: 所有 root 失败时 progress 必须标记 failed + 明确 error_msg.

package indexengine

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"xmedia/internal/domain"
)

func TestScanNASAllRootsMissingMarksFailed(t *testing.T) {
	nas := newMemoryNASSourcesRepo()
	index := newMemoryIndexRepo()
	cfg := &memoryConfigRepo{values: map[string]string{}}

	s := NewService(Options{
		MediaIndex:   index,
		MediaLibrary: &memoryLibraryRepo{},
		Configs:      cfg,
		NASSources:   nas,
		WorkerCount:  2,
	})
	_, _ = nas.Create(context.Background(), &domain.NASSource{
		Name: "gone", Path: "/nonexistent/root-a", Enabled: true,
	})
	_, _ = nas.Create(context.Background(), &domain.NASSource{
		Name: "gone2", Path: "/nonexistent/root-b", Enabled: true,
	})

	s.scanNAS(context.Background(), []string{"/nonexistent/root-a", "/nonexistent/root-b"}, false)

	p := s.Progress()
	if p.Status != "failed" {
		t.Fatalf("全部 root 不可达时 status 应为 failed, got %q (phase=%s err=%q)", p.Status, p.Phase, p.ErrorMsg)
	}
	if p.ErrorMsg == "" {
		t.Fatalf("error_msg 不应为空 — 这是界面判断'为什么扫了个寂寞'的唯一依据")
	}
}

func TestScanNASPartialRootFailureStillDone(t *testing.T) {
	dirOK := t.TempDir()
	full := filepath.Join(dirOK, "movies")
	if err := os.MkdirAll(full, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(full, "阿凡达.2009.1080p.BluRay.mkv"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	nas := newMemoryNASSourcesRepo()
	library := &memoryLibraryRepo{items: []*domain.MediaLibrary{
		{Title: "阿凡达", TitleOrig: "Avatar", Year: 2009, ExternalID: 19995, ExternalSource: "tmdb"},
	}}
	s := NewService(Options{
		MediaIndex:   newMemoryIndexRepo(),
		MediaLibrary: library,
		Configs:      &memoryConfigRepo{values: map[string]string{}},
		NASSources:   nas,
		WorkerCount:  2,
	})

	// 一好一坏: 只要有一条产出索引, 整体仍应 done (单路径失败不影响其他).
	s.scanNAS(context.Background(), []string{dirOK, "/nonexistent/root-b"}, false)
	p := s.Progress()
	if p.Status == "failed" {
		t.Fatalf("存在可用 root 时不应整体 failed, got %+v", p)
	}
}
