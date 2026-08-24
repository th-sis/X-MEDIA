// [V7 §9.4 UI-first 实测回归] NAS source 有效性自监测:
// 用户配置后无需人工点"检测", 系统周期性 stat 每个 enabled source,
// 把 ok/not_accessible 写回 last_accessibility — 管理列表与
// Capabilities 三态随之自动更新.

package indexengine

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"xmedia/internal/domain"
)

func TestRunHealthCheckOnce_UpdatesAccessibility(t *testing.T) {
	dirOK := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dirOK, "movies"), 0o755); err != nil {
		t.Fatal(err)
	}
	dirGone := filepath.Join(t.TempDir(), "vanished") // 不存在

	nas := newMemoryNASSourcesRepo()
	idOK, _ := nas.Create(context.Background(), &domain.NASSource{Name: "ok", Path: dirOK, Enabled: true})
	idGone, _ := nas.Create(context.Background(), &domain.NASSource{Name: "gone", Path: dirGone, Enabled: true})

	s := NewService(Options{NASSources: nas})

	checked := s.RunHealthCheckOnce(context.Background())
	if checked != 2 {
		t.Fatalf("checked = %d, want 2", checked)
	}

	gotOK, _ := nas.Get(context.Background(), idOK)
	if gotOK.LastAccessibility != domain.NASAccessibilityOK {
		t.Fatalf("可达目录应标记 ok, got %q", gotOK.LastAccessibility)
	}
	gotGone, _ := nas.Get(context.Background(), idGone)
	if gotGone.LastAccessibility != domain.NASAccessibilityNotAccessible {
		t.Fatalf("消失目录应标记 not_accessible, got %q", gotGone.LastAccessibility)
	}
	if gotOK.LastCheckedAt == nil || time.Since(*gotOK.LastCheckedAt) > time.Minute {
		t.Fatalf("LastCheckedAt 应为刚写入的时间: %v", gotOK.LastCheckedAt)
	}
}

func TestRunHealthCheckOnce_NilRepoSafe(t *testing.T) {
	s := NewService(Options{})
	if n := s.RunHealthCheckOnce(context.Background()); n != 0 {
		t.Fatalf("未接仓库时应返回 0, got %d", n)
	}
}
