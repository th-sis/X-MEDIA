import 'dart:async';
import 'dart:convert';
import 'package:flutter/foundation.dart';
import 'package:web_socket_channel/web_socket_channel.dart';

class WsEvent {
  final String type;
  final Map<String, dynamic> payload;
  WsEvent(this.type, this.payload);
}

class WsClient {
  final String host;

  /// [V7 §23.0] 测试接缝: 注入假 channel 工厂, 默认真实连接.
  @visibleForTesting
  final Future<WebSocketChannel> Function(Uri url)? channelFactory;

  WebSocketChannel? _channel;
  StreamSubscription? _sub;
  final _events = StreamController<WsEvent>.broadcast();
  final _reconnected = StreamController<void>.broadcast();
  final ValueNotifier<bool> connected = ValueNotifier(false);
  int _attempt = 0;
  bool _disposed = false;
  bool _everConnected = false;

  Stream<WsEvent> get events => _events.stream;

  /// [V7 §20.1.5] 断线→重连成功后发出一次事件, 供 AppState 触发
  /// HTTP snapshot 补刷 (§28.3 对比 server_started_at). 首次上线不发出.
  Stream<void> get reconnected => _reconnected.stream;

  WsClient(this.host, {this.channelFactory});

  String get _wsUrl {
    final h = host.startsWith('http') ? host.replaceFirst('http', 'ws') : 'ws://$host';
    final base = h.endsWith('/') ? h.substring(0, h.length - 1) : h;
    return '$base/ws';
  }

  Future<void> connect() async {
    if (_disposed) return;
    WebSocketChannel? ch;
    try {
      ch = channelFactory != null
          ? await channelFactory!(Uri.parse(_wsUrl))
          : WebSocketChannel.connect(Uri.parse(_wsUrl));
      if (_disposed) {
        await ch.sink.close();
        return;
      }
      // [V7 §27.2] 真实握手完成才置在线. WebSocketChannel.connect 是懒连接,
      // 同步置 true 会造成"假在线" (服务器不可达时 UI 先显示已连再翻车),
      // 且提前清零退避计数会打乱 §20.1 的重连节奏.
      await ch.ready;
      if (_disposed) {
        await ch.sink.close();
        return;
      }
      _channel = ch;
      connected.value = true;
      _attempt = 0;
      if (_everConnected) {
        _reconnected.add(null);
      }
      _everConnected = true;
      _sub = ch.stream.listen(
        (data) => _onData(data),
        onDone: () => _reconnect(),
        onError: (_) => _reconnect(),
      );
    } catch (_) {
      connected.value = false;
      _scheduleReconnect();
    }
  }

  void _onData(dynamic data) {
    try {
      final map = jsonDecode(data as String) as Map<String, dynamic>;
      final type = map['type'] as String? ?? '';
      final payload = (map['payload'] as Map<String, dynamic>?) ?? const {};
      _events.add(WsEvent(type, payload));
    } catch (_) {}
  }

  void _reconnect() {
    connected.value = false;
    _sub?.cancel();
    _scheduleReconnect();
  }

  void _scheduleReconnect() {
    if (_disposed) return;
    const delays = [1, 2, 4, 8, 16, 30];
    final delay = Duration(seconds: delays[_attempt.clamp(0, delays.length - 1)]);
    _attempt++;
    Future.delayed(delay, () {
      if (!_disposed && !connected.value) connect();
    });
  }

  void dispose() {
    _disposed = true;
    _sub?.cancel();
    _channel?.sink.close();
    _events.close();
    _reconnected.close();
    connected.dispose();
  }
}
