package resolve

import (
	"context"
)

// shouldSkipP0 判定是否跳过 P0，返回原因（空表示不跳过）。
// [V7 §6.3] 智能跳过条件：
//  1. NAS 未配置
//  2. 索引服务不可用
//  3. NAS 处于扫描中（Phase A/B 索引不完整，查询必然 miss）
//  4. 索引为空
//
// [V7 §6.3] 跳过 P0 时 Resolve Modal 阶段不显示 nas_lookup，直接显示 pan_searching。
func (s *Service) shouldSkipP0(ctx context.Context) string {
	if !s.nasConfigured(ctx) {
		return "未配置 NAS 路径"
	}
	if s.mediaIndex == nil {
		return "索引服务不可用"
	}
	if s.indexScanningFn != nil && s.indexScanningFn() {
		return "NAS 正在扫描（索引不完整）"
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
