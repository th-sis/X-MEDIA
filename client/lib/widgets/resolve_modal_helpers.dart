// [V7 §17.5] Resolve Modal 分层进度指示器 — 纯函数辅助层.
//
// 与 widgets/resolve_modal.dart 的关系:
//   resolve_modal.dart        = Dialog UI + WS 监听 + state
//   resolve_modal_helpers.dart = 纯函数 (本页) — 易于单元测试
//
// 三大函数:
//   resolveLayerForStage(stage, skipNas) → 0/1/2/3/4 (NAS/Pan/Magnet/Sub/Terminal)
//   shouldShowProgressBar(stage)         → bool (P2 阶段高亮进度条)
//   shouldSkipP0(nasAvailable, nasIndexComplete) → bool (V7 §6.3 智能跳过)

import '../models/media.dart';

/// Resolve Stage → 当前激活层 (0/1/2/3/4).
///
/// 0 = NAS 本地 (P0)
/// 1 = 盘搜转存 (P1)
/// 2 = 磁力下载 (P2)
/// 3 = 订阅等待 (P3)
/// 4 = 已结束 (playReady/error)
///
/// [skipNas] = true 时 (NAS 不可用 / 索引未完成), P0 stage 全部映射到 layer 1,
/// 视觉上 P0 步骤圈灰显, 直接从 P1 开始 (§17.5 + §6.3).
int resolveLayerForStage(ResolveStage stage, {required bool skipNas}) {
  // terminal
  if (stage == ResolveStage.playReady || stage == ResolveStage.error) {
    return 4;
  }
  // P3
  if (stage == ResolveStage.notFound) return 3;
  // P2
  if (stage == ResolveStage.magnetDownloading) return 2;
  // P1
  if (stage == ResolveStage.panSearching ||
      stage == ResolveStage.panSearched ||
      stage == ResolveStage.transferring ||
      stage == ResolveStage.resolvingLink) {
    return 1;
  }
  // P0
  if (stage == ResolveStage.resolveStart ||
      stage == ResolveStage.nasLookup ||
      stage == ResolveStage.nasHit) {
    return skipNas ? 1 : 0;
  }
  // 兜底: 未知 stage 视作 resolveStart
  return skipNas ? 1 : 0;
}

/// P2 阶段突出进度条 (分钟~小时级下载, 用户必须有预期).
/// §17.5: "P2 磁力下载阶段取消按钮可用".
bool shouldShowProgressBar(ResolveStage stage) {
  return stage == ResolveStage.magnetDownloading;
}

/// V7 §6.3 P0 智能跳过: NAS 不可用或索引未完成时, 跳过 P0 直接进 P1.
bool shouldSkipP0({required bool nasAvailable, required bool nasIndexComplete}) {
  return !nasAvailable || !nasIndexComplete;
}
