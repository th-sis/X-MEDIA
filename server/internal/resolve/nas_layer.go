package resolve

import (
	"context"
)

// shouldSkipP0 判定是否跳过 P0，返回原因（空表示不跳过）。
func (s *Service) shouldSkipP0(ctx context.Context) string {
	if !s.nasConfigured(ctx) {
		return "未配置 NAS 路径"
	}
	if s.mediaIndex == nil {
		return "索引服务不可用"
	}
	cnt, err := s.indexCountFn(ctx)
	if err != nil {
		return "" // 查询失败不强制跳过，让 P0 自己 fail
	}
	if cnt == 0 {
		return "NAS 索引为空"
	}
	return ""
}
