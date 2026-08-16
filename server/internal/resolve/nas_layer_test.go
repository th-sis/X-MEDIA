package resolve

import (
	"context"
	"testing"
	"time"

	"xmedia/internal/domain"
)

// stubRepo 最小化实现，避免引入 store 包。
type stubRepo struct{ count int }

func (s *stubRepo) Upsert(context.Context, *domain.MediaIndex) (int64, error) {
	return 0, nil
}
func (s *stubRepo) FindBest(context.Context, int64, string, int, int) (*domain.MediaIndex, error) {
	return nil, domain.Errf(domain.CodeNotFound)
}
func (s *stubRepo) AvailableKeys(context.Context, []domain.AvailabilityKey) ([]domain.AvailabilityKey, error) {
	return nil, nil
}
func (s *stubRepo) Count(context.Context) (int, error) { return s.count, nil }
func (s *stubRepo) ListBySource(context.Context, string, int64) ([]*domain.MediaIndex, error) {
	return nil, nil
}
func (s *stubRepo) DeleteBySourcePath(context.Context, string, string) error {
	return nil
}
func (s *stubRepo) ListUnconfirmedBefore(context.Context, time.Time) ([]*domain.MediaIndex, error) {
	return nil, nil
}
func (s *stubRepo) MarkOrphaned(context.Context, []int64) error { return nil }

// TestShouldSkipP0_V7 §6.3 智能跳过四种场景
func TestShouldSkipP0_V7(t *testing.T) {
	tests := []struct {
		name       string
		nasOK      bool
		scanning   bool
		count      int
		wantSkip   bool
		wantReason string
	}{
		{name: "未配置 NAS", nasOK: false, scanning: false, count: 100, wantSkip: true, wantReason: "未配置 NAS 路径"},
		{name: "扫描中", nasOK: true, scanning: true, count: 100, wantSkip: true, wantReason: "NAS 正在扫描（索引不完整）"},
		{name: "索引为空", nasOK: true, scanning: false, count: 0, wantSkip: true, wantReason: "NAS 索引为空"},
		{name: "正常查询", nasOK: true, scanning: false, count: 100, wantSkip: false, wantReason: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := &Service{
				nasConfigured:   func(context.Context) bool { return tc.nasOK },
				mediaIndex:      &stubRepo{count: tc.count},
				indexCountFn:    func(context.Context) (int, error) { return tc.count, nil },
				indexScanningFn: func() bool { return tc.scanning },
			}
			reason := s.shouldSkipP0(context.Background())
			gotSkip := reason != ""
			if gotSkip != tc.wantSkip {
				t.Errorf("skip mismatch: got=%v want=%v reason=%q", gotSkip, tc.wantSkip, reason)
			}
			if reason != tc.wantReason {
				t.Errorf("reason mismatch: got=%q want=%q", reason, tc.wantReason)
			}
		})
	}
}

// TestCapabilities_NASStatus 三态：not_configured / not_accessible / ok
func TestCapabilities_NASStatus(t *testing.T) {
	t.Run("not_configured: 未配置 NAS", func(t *testing.T) {
		s := newCapsTestService(t, capsOpts{
			nasOK:     false,
			paths:     nil,
			scanState: false,
			count:     0,
			phase:     "",
			processed: 0,
			total:     0,
		})
		c := s.Capabilities(context.Background())
		if c.NASStatus != "not_configured" {
			t.Errorf("want not_configured, got %q", c.NASStatus)
		}
		if c.NASAvailable {
			t.Errorf("want NASAvailable=false, got true")
		}
	})

	t.Run("not_accessible: 路径配置但 stat 失败", func(t *testing.T) {
		f := false
		s := newCapsTestService(t, capsOpts{
			nasOK:     true,
			paths:     []string{"/nonexistent/path"},
			scanState: false,
			count:     0,
			statOk:    &f,
		})
		c := s.Capabilities(context.Background())
		if c.NASStatus != "not_accessible" {
			t.Errorf("want not_accessible, got %q", c.NASStatus)
		}
		if c.NASAvailable {
			t.Errorf("want NASAvailable=false")
		}
	})

	t.Run("ok: 路径配置且 stat 成功", func(t *testing.T) {
		s := newCapsTestService(t, capsOpts{
			nasOK:     true,
			paths:     []string{"."}, // 相对路径 → stub stat 始终返回 true
			scanState: false,
			count:     5,
		})
		c := s.Capabilities(context.Background())
		if c.NASStatus != "ok" {
			t.Errorf("want ok, got %q", c.NASStatus)
		}
		if !c.NASAvailable {
			t.Errorf("want NASAvailable=true")
		}
	})

	t.Run("扫描中应暴露 NASScanning", func(t *testing.T) {
		s := newCapsTestService(t, capsOpts{
			nasOK:     true,
			paths:     []string{"."},
			scanState: true,
			count:     5,
			phase:     "B",
			processed: 100,
			total:     1000,
		})
		c := s.Capabilities(context.Background())
		if !c.NASScanning {
			t.Errorf("want NASScanning=true")
		}
		if c.NASPhase != "B" || c.NASProcessedFiles != 100 || c.NASTotalFiles != 1000 {
			t.Errorf("progress mismatch: phase=%q processed=%d total=%d",
				c.NASPhase, c.NASProcessedFiles, c.NASTotalFiles)
		}
	})
}

type capsOpts struct {
	nasOK     bool
	paths     []string
	scanState bool
	count     int
	phase     string
	processed int
	total     int
	// statOk 控制 pathStatFn 返回值；nil 时默认 true
	statOk *bool
}

func newCapsTestService(t *testing.T, opts capsOpts) *Service {
	t.Helper()
	stat := true
	if opts.statOk != nil {
		stat = *opts.statOk
	}
	return &Service{
		nasConfigured:   func(context.Context) bool { return opts.nasOK },
		nasPathsKnown:   func() []string { return opts.paths },
		pathStatFn:      func(string) bool { return stat },
		indexScanningFn: func() bool { return opts.scanState },
		indexStatusFn: func() (bool, string, int, int) {
			return opts.scanState, opts.phase, opts.processed, opts.total
		},
		indexCountFn: func(context.Context) (int, error) { return opts.count, nil },
		mediaIndex:   &stubRepo{count: opts.count},
		// Capabilities 还会调 pansearchHealth / loggedInDrivers / magnetEnabledFn / demoFallbackFn / p0MinScoreFn
		// 单测不验证这些字段（Capabilities 是结构体直接返回），只需不 panic
		pansearchHealth: func(context.Context) bool { return false },
		loggedInDrivers: func(context.Context) []string { return nil },
		magnetEnabledFn: func(context.Context) bool { return false },
		demoFallbackFn:  func(context.Context) bool { return false },
		p0MinScoreFn:    func(context.Context) float64 { return 0.6 },
		serverVersion:   "7.0.0",
	}
}
