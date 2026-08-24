// [V7 §26.1] 播放器 seek 计算纯函数.
//
// player_page 的方向键快进快退 (±10s, TV 10-foot UI 惯例) 与进度条
// onChangeEnd 提交共用此钳位逻辑; 提取纯函数便于单元测试.

/// 从 [position] 偏移 [deltaSecs] 秒, 钳位到 [0, duration].
///
/// - duration <= 0 (未初始化/直播流): 返回原 position, 不做任何偏移.
/// - 结果越过边界时贴边.
Duration clampSeek(Duration position, Duration duration, int deltaSecs) {
  assert(deltaSecs != 0);
  if (duration <= Duration.zero) return position;
  final target = position + Duration(seconds: deltaSecs);
  if (target < Duration.zero) return Duration.zero;
  if (target > duration) return duration;
  return target;
}
