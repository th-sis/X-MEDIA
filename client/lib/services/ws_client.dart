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
  WebSocketChannel? _channel;
  StreamSubscription? _sub;
  final _events = StreamController<WsEvent>.broadcast();
  final ValueNotifier<bool> connected = ValueNotifier(false);
  int _attempt = 0;
  bool _disposed = false;

  Stream<WsEvent> get events => _events.stream;

  WsClient(this.host);

  String get _wsUrl {
    final h = host.startsWith('http') ? host.replaceFirst('http', 'ws') : 'ws://$host';
    final base = h.endsWith('/') ? h.substring(0, h.length - 1) : h;
    return '$base/ws';
  }

  void connect() {
    if (_disposed) return;
    try {
      _channel = WebSocketChannel.connect(Uri.parse(_wsUrl));
      connected.value = true;
      _attempt = 0;
      _sub = _channel!.stream.listen(
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
    connected.dispose();
  }
}
