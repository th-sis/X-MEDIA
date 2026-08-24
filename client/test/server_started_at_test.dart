// [V7 §28.3] 客户端感知重启逻辑测试.
//
// 后端 GET /api/state/snapshot 返回 server_started_at (RFC3339 字符串).
// AppState 每次 refresh 对比上次记录的 serverStartedAt, 若变化 → 触发
// restartDetected 标志, UI 层强制刷新所有页面 + 弹通知.
import 'package:flutter_test/flutter_test.dart';
import 'package:xmedia_client/services/restart_detector.dart';

void main() {
  group('RestartDetector (V7 §28.3)', () {
    test('首次记录 serverStartedAt → restartDetected = false', () {
      final d = RestartDetector();
      expect(d.detectRestart('2026-08-21T10:00:00Z'), isFalse);
      expect(d.serverStartedAt, '2026-08-21T10:00:00Z');
    });

    test('同一 serverStartedAt → restartDetected = false', () {
      final d = RestartDetector();
      d.detectRestart('2026-08-21T10:00:00Z');
      expect(d.detectRestart('2026-08-21T10:00:00Z'), isFalse);
    });

    test('serverStartedAt 变化 → restartDetected = true (后端重启)', () {
      final d = RestartDetector();
      d.detectRestart('2026-08-21T10:00:00Z');
      expect(d.detectRestart('2026-08-21T10:05:30Z'), isTrue);
      // 新值已记录, 下次同值不再触发
      expect(d.detectRestart('2026-08-21T10:05:30Z'), isFalse);
    });

    test('空字符串视为首次 → 不触发', () {
      final d = RestartDetector();
      expect(d.detectRestart(''), isFalse);
    });

    test('lastRestartReason 透传', () {
      final d = RestartDetector();
      d.detectRestart('2026-08-21T10:00:00Z', reason: 'config_change');
      expect(d.lastRestartReason, 'config_change');
      // 重启后 reason 更新
      d.detectRestart('2026-08-21T10:05:30Z', reason: 'graceful');
      expect(d.lastRestartReason, 'graceful');
    });
  });
}
