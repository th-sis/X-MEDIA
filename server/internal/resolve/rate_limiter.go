package resolve

import (
	"context"
	"time"

	"xmedia/internal/domain"
)

// RateLimiter Resolve 并发限流（滑动窗口）。
type RateLimiter struct {
	maxRequests int
	windowSec   int
	repo        domain.RateLimitRepository
}

func NewRateLimiter(repo domain.RateLimitRepository, maxRequests, windowSec int) *RateLimiter {
	if maxRequests <= 0 {
		maxRequests = 3
	}
	if windowSec <= 0 {
		windowSec = 30
	}
	return &RateLimiter{maxRequests: maxRequests, windowSec: windowSec, repo: repo}
}

// Allow 判断是否放行；返回 (允许, 剩余秒数)。
func (rl *RateLimiter) Allow(ctx context.Context, clientIP string) (bool, int) {
	window := time.Duration(rl.windowSec) * time.Second
	windowStart := time.Now().Truncate(window)

	count, err := rl.repo.Count(ctx, clientIP, windowStart)
	if err != nil {
		return true, 0 // DB 故障降级放行
	}
	if count >= rl.maxRequests {
		remaining := int(windowStart.Add(window).Sub(time.Now()).Seconds())
		if remaining < 1 {
			remaining = 1
		}
		return false, remaining
	}
	if err := rl.repo.Increment(ctx, clientIP, windowStart); err != nil {
		return true, 0
	}
	return true, 0
}
