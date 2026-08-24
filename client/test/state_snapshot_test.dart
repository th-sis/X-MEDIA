// [V7 §28.3] StateSnapshot 反序列化测试 + §20.1.5 WS 重连补刷接线测试.
import 'package:fake_async/fake_async.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:xmedia_client/models/media.dart';
import 'package:xmedia_client/services/app_state.dart';
import 'package:xmedia_client/services/ws_client.dart';

import 'ws_client_test.dart';

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

  group('WS 重连 → snapshot 补刷 (V7 §20.1.5)', () {
    test('断线重连成功后触发 refresh (loading 翻转 + notifyListeners)', () {
      fakeAsync((async) {
        final app = AppState.forTest();
        addTearDown(app.dispose);
        var notifyCount = 0;
        app.addListener(() => notifyCount++);

        final ch1 = FakeWebSocketChannel();
        final ch2 = FakeWebSocketChannel();
        final channels = [ch1, ch2];
        final ws = WsClient(
          'h',
          channelFactory: (_) async => channels.removeAt(0),
        );
        app.attachWsForTest(ws);
        async.flushMicrotasks();
        expect(app.wsConnected, isTrue);

        // 断线 → 1s 后重连成功 → reconnected → refresh().
        ch1.serverCloses();
        async.flushMicrotasks();
        async.elapse(const Duration(seconds: 1));
        async.flushMicrotasks();

        expect(app.wsConnected, isTrue);
        // refresh() 同步置 loading=true 并 notifyListeners;
        // fakeAsync 内真实 http 永不完成, loading 停留在 true.
        expect(app.loading, isTrue);
        expect(notifyCount, greaterThan(0));
      });
    });
  });
}
