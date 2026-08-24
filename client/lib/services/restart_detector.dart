// [V7 §28.3] 客户端感知重启检测器.
//
// 后端 GET /api/state/snapshot 返回 server_started_at (RFC3339 字符串)
// 与 last_restart_reason. 前端每次 refresh 时调用 [detectRestart], 若
// server_started_at 相对上次记录发生变化, 表明后端已重启, UI 层据此
// 弹通知 + 强制刷新所有页面.

class RestartDetector {
  String? _serverStartedAt;
  String? _lastRestartReason;
  bool _restartJustDetected = false;

  /// 最近一次记录的 server_started_at.
  String? get serverStartedAt => _serverStartedAt;

  /// 最近一次记录的 last_restart_reason.
  String? get lastRestartReason => _lastRestartReason;

  /// 上一次调用 [detectRestart] 是否检测到重启. UI 消费后应调
  /// [acknowledgeRestart] 清除标志.
  bool get restartJustDetected => _restartJustDetected;

  /// 比较新旧 server_started_at, 返回是否检测到重启.
  ///
  /// - 首次调用 (上次为空): 不算重启, 仅记录.
  /// - 新旧相同: 不算重启.
  /// - 新旧不同: 重启, 设置 [_restartJustDetected] = true.
  /// - 新值为空字符串: 视为首次.
  bool detectRestart(String newStartedAt, {String? reason}) {
    _restartJustDetected = false;
    if (reason != null) _lastRestartReason = reason;

    final normalized = newStartedAt.trim();
    if (normalized.isEmpty) {
      // 空值不更新, 不触发重启.
      return false;
    }
    if (_serverStartedAt == null) {
      _serverStartedAt = normalized;
      return false;
    }
    if (_serverStartedAt == normalized) {
      return false;
    }
    _serverStartedAt = normalized;
    _restartJustDetected = true;
    return true;
  }

  /// UI 消费完重启通知后调, 清 [_restartJustDetected] 标志.
  void acknowledgeRestart() {
    _restartJustDetected = false;
  }

  /// 测试 / 切换会话时重置.
  void reset() {
    _serverStartedAt = null;
    _lastRestartReason = null;
    _restartJustDetected = false;
  }
}
