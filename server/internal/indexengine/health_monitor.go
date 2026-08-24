package indexengine

import (
	"context"
	"os"
	"time"

	"xmedia/internal/domain"
)

// [V7 §9.4 UI-first] NAS source 有效性自监测.
//
// 用户在管理后台配置 source 后, 系统周期性 stat 每个 enabled 路径,
// 把可达性写回 last_accessibility — 列表页与 Capabilities 三态随之
// 自动更新, 无需人工点「检测」. 只做 stat, 不做 WalkDir 数文件
// (大目录代价高, file_count 仍由扫描/手动 bulk-health 负责).

// RunHealthCheckOnce 对全部 enabled source 做一次 stat 探测并回写.
// 返回探测条数. nasSources 未接线时安全返回 0.
func (s *Service) RunHealthCheckOnce(ctx context.Context) int {
	if s.nasSources == nil {
		return 0
	}
	sources, err := s.nasSources.ListEnabled(ctx)
	if err != nil {
		return 0
	}
	now := time.Now()
	checked := 0
	for _, src := range sources {
		acc := domain.NASAccessibilityNotAccessible
		if info, err := os.Stat(src.Path); err == nil && info.IsDir() {
			acc = domain.NASAccessibilityOK
		}
		if uerr := s.nasSources.UpdateHealth(ctx, src.ID, acc, src.FileCount, now); uerr == nil {
			checked++
		}
	}
	return checked
}

// StartHealthMonitor 启动周期监测循环 (ctx 取消即退出).
// interval 建议 5 分钟; 首轮立即执行一次.
func (s *Service) StartHealthMonitor(ctx context.Context, interval time.Duration) {
	if s.nasSources == nil || interval <= 0 {
		return
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			_ = s.RunHealthCheckOnce(ctx)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}
