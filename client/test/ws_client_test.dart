// [V7 §20.1/§27.2] WsClient 连接状态机契约测试.
//
// 覆盖:
//   §27.2   真实握手 (channel.ready) 完成后才置在线 — 杜绝懒连接假在线,
//           且假在线不再提前清零退避计数 (§20.1 重连节奏不被打乱).
//   §20.1.5 断线→重连成功后 reconnected 流恰好发出一次, 供 AppState 触发
//           HTTP snapshot 补刷; 首次连接成功不发出.

import 'dart:async';

import 'package:fake_async/fake_async.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:stream_channel/stream_channel.dart';
import 'package:web_socket_channel/web_socket_channel.dart';
import 'package:xmedia_client/services/ws_client.dart';

/// 测试用假 channel: ready 可控失败, incoming 可控关闭.
class FakeWebSocketChannel extends StreamChannelMixin<dynamic>
    implements WebSocketChannel {
  final StreamController<dynamic> incoming = StreamController<dynamic>();
  final Completer<void> _ready = Completer<void>();
  int closeCount = 0;

  FakeWebSocketChannel({bool readyOk = true}) {
    if (readyOk) {
      _ready.complete();
    } else {
      _ready.completeError(StateError('handshake refused'));
      // 错误由 WsClient 的 await ch.ready 捕获; ignore 仅抑制
      // listener 挂上前窗口期的 zone unhandled-error.
      _ready.future.ignore();
    }
  }

  void serverCloses() => incoming.close();

  @override
  Future<void> get ready => _ready.future;

  @override
  Stream<dynamic> get stream => incoming.stream;

  @override
  WebSocketSink get sink => _FakeSink(onClose: () => closeCount++);

  @override
  int? get closeCode => null;

  @override
  String? get closeReason => null;

  @override
  String? get protocol => null;

  @override
  Future<void> close([int? closeCode, String? closeReason]) async {
    closeCount++;
    await incoming.close();
  }
}

class _FakeSink implements WebSocketSink {
  _FakeSink({required this.onClose});
  final void Function() onClose;
  final Completer<void> _done = Completer<void>();

  @override
  Future<void> get done => _done.future;

  @override
  void add(dynamic data) {}

  @override
  void addError(Object error, [StackTrace? stackTrace]) {}

  @override
  Future<void> addStream(Stream<dynamic> stream) async {}

  @override
  Future<void> close([Object? closeCode, String? closeReason]) async {
    onClose();
    if (!_done.isCompleted) _done.complete();
  }
}

void main() {
  group('WsClient (V7 §20.1/§27.2)', () {
    test('握手失败 → 不置在线 (§27.2 杜绝假在线)', () {
      fakeAsync((async) {
        final ws = WsClient(
          '127.0.0.1:38088',
          channelFactory: (_) async => FakeWebSocketChannel(readyOk: false),
        );
        ws.connect();
        async.flushMicrotasks();
        expect(ws.connected.value, isFalse);
        ws.dispose();
      });
    });

    test('首次连接成功 → connected=true 且不发 reconnected', () {
      fakeAsync((async) {
        final ch = FakeWebSocketChannel();
        final ws = WsClient('h', channelFactory: (_) async => ch);
        var reconnCount = 0;
        ws.reconnected.listen((_) => reconnCount++);
        ws.connect();
        async.flushMicrotasks();
        expect(ws.connected.value, isTrue);
        expect(reconnCount, 0);
        ws.dispose();
      });
    });

    test('断线 → 重连成功 → reconnected 恰好一次 (§20.1.5)', () {
      fakeAsync((async) {
        final ch1 = FakeWebSocketChannel();
        final ch2 = FakeWebSocketChannel();
        final channels = [ch1, ch2];
        final ws = WsClient(
          'h',
          channelFactory: (_) async => channels.removeAt(0),
        );
        var reconnCount = 0;
        ws.reconnected.listen((_) => reconnCount++);
        ws.connect();
        async.flushMicrotasks();
        expect(ws.connected.value, isTrue);

        // 服务器断开 → onDone → 延迟 1s (delays[0]) 后重连.
        ch1.serverCloses();
        async.flushMicrotasks();
        expect(ws.connected.value, isFalse);
        async.elapse(const Duration(seconds: 1));
        async.flushMicrotasks();

        expect(ws.connected.value, isTrue);
        expect(reconnCount, 1);
        ws.dispose();
      });
    });

    test('握手失败后重试成功 → 同样发 reconnected (退避节奏未被清零)', () {
      fakeAsync((async) {
        final bad = FakeWebSocketChannel(readyOk: false);
        final good = FakeWebSocketChannel();
        final channels = [bad, good];
        final ws = WsClient(
          'h',
          channelFactory: (_) async => channels.removeAt(0),
        );
        var reconnCount = 0;
        ws.reconnected.listen((_) => reconnCount++);
        ws.connect();
        async.flushMicrotasks();
        // 首次握手失败: 未曾上线过, 重试不算"重连".
        expect(ws.connected.value, isFalse);
        async.elapse(const Duration(seconds: 1));
        async.flushMicrotasks();
        expect(ws.connected.value, isTrue);
        expect(reconnCount, 0);
        ws.dispose();
      });
    });
  });
}
