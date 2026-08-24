// [V7 §28.3] StateSnapshot 反序列化测试.
import 'package:flutter_test/flutter_test.dart';
import 'package:xmedia_client/models/media.dart';

void main() {
  group('StateSnapshot.fromJson (V7 §28.3)', () {
    test('完整字段', () {
      final snap = StateSnapshot.fromJson({
        'server_started_at': '2026-08-21T10:00:00Z',
        'last_restart_reason': 'config_change',
      });
      expect(snap.serverStartedAt, '2026-08-21T10:00:00Z');
      expect(snap.lastRestartReason, 'config_change');
    });

    test('缺省字段 (后端旧版兼容)', () {
      final snap = StateSnapshot.fromJson({});
      expect(snap.serverStartedAt, '');
      expect(snap.lastRestartReason, '');
    });

    test('空字符串合法 (无重启历史)', () {
      final snap = StateSnapshot.fromJson({
        'server_started_at': '',
        'last_restart_reason': '',
      });
      expect(snap.serverStartedAt, '');
    });
  });
}
