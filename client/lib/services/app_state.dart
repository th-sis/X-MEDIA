import 'dart:async';
import 'package:flutter/foundation.dart';
import 'package:shared_preferences/shared_preferences.dart';
import '../models/media.dart';
import 'api_client.dart';
import 'ws_client.dart';

class AppState extends ChangeNotifier {
  static const _kBackendUrl = 'backend_url';

  late ApiClient api;
  WsClient? ws;
  String backendUrl = 'http://127.0.0.1:38088';

  bool loading = false;
  List<Section> sections = const [];
  List<ContinueWatching> continueWatching = const [];
  Capabilities capabilities = const Capabilities();
  Map<String, dynamic> health = const {};
  bool wsConnected = false;
  String error = '';

  // [P2#8] 服务优雅退出通知: 后端 SIGTERM 时通过 WS 推送 server_stopping,
  // 这里截获后转为 UI 提示, 避免用户误以为服务挂掉.
  String? serverStoppingMessage;

  StreamSubscription<WsEvent>? _wsSub;

  AppState() {
    api = ApiClient(backendUrl);
    _init();
  }

  Future<void> _init() async {
    debugPrint('[XMedia] _init start, backendUrl=$backendUrl');
    final prefs = await SharedPreferences.getInstance();
    final saved = prefs.getString(_kBackendUrl);
    if (saved != null && saved.isNotEmpty) {
      debugPrint('[XMedia] loaded saved backendUrl=$saved');
      backendUrl = saved;
      api = ApiClient(backendUrl);
    }
    await _connectWs();
    debugPrint('[XMedia] _connectWs done, calling refresh');
    await refresh();
    debugPrint('[XMedia] refresh done, sections=${sections.length}');
  }

  Future<void> setBackendUrl(String url) async {
    final cleaned = url.trim().replaceAll(RegExp(r'/+$'), '');
    if (cleaned.isEmpty) return;
    backendUrl = cleaned;
    api = ApiClient(cleaned);
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_kBackendUrl, cleaned);
    _connectWs();
    await refresh();
    notifyListeners();
  }

  Future<void> _connectWs() async {
    await _wsSub?.cancel();
    ws?.dispose();
    try {
      final host = backendUrl.replaceFirst('http://', '').replaceFirst('https://', '');
      ws = WsClient(host);
      _wsSub = ws!.events.listen(_onWsEvent);
      ws!.connected.addListener(() {
        wsConnected = ws!.connected.value;
        notifyListeners();
      });
      ws!.connect();
    } catch (_) {}
  }

  void _onWsEvent(WsEvent e) {
    switch (e.type) {
      case 'health_check':
        health = e.payload;
        capabilities = Capabilities.fromJson((e.payload['capabilities'] as Map<String, dynamic>?) ?? const {});
        notifyListeners();
        break;
      case 'capabilities':
        capabilities = Capabilities.fromJson(e.payload);
        notifyListeners();
        break;
      case 'server_stopping':
        // [P2#8] §28.4 服务优雅退出: 弹"维护中"提示, 引导用户稍候重试
        final reason = e.payload['reason'] as String? ?? 'graceful';
        final retry = e.payload['retry_after_sec'] as int? ?? 8;
        serverStoppingMessage = '服务正在维护（$reason），约 ${retry}s 后可重试';
        notifyListeners();
        break;
    }
  }

  /// 清除 server_stopping 提示（UI 已展示 SnackBar 后调用）。
  void clearServerStopping() {
    if (serverStoppingMessage == null) return;
    serverStoppingMessage = null;
    notifyListeners();
  }

  Future<void> refresh() async {
    debugPrint('[XMedia] refresh: backendUrl=$backendUrl');
    loading = true;
    error = '';
    notifyListeners();
    try {
      final results = await Future.wait([
        _safeSections(),
        _safeContinue(),
        _safeCaps(),
      ]);
      sections = results[0] as List<Section>;
      continueWatching = results[1] as List<ContinueWatching>;
      capabilities = results[2] as Capabilities;
      debugPrint('[XMedia] refresh ok: sections=${sections.length}, cw=${continueWatching.length}');
    } catch (e, st) {
      debugPrint('[XMedia] refresh FAILED: $e\n$st');
      error = e.toString();
    } finally {
      loading = false;
      notifyListeners();
    }
  }

  Future<List<Section>> _safeSections() async {
    try {
      debugPrint('[XMedia] _safeSections calling home()');
      final r = await api.home();
      debugPrint('[XMedia] _safeSections got ${r.length} sections');
      return r;
    } catch (e) {
      debugPrint('[XMedia] _safeSections failed: $e');
      return const [];
    }
  }

  Future<List<ContinueWatching>> _safeContinue() async {
    try {
      return await api.continueWatching();
    } catch (_) {
      return const [];
    }
  }

  Future<Capabilities> _safeCaps() async {
    try {
      return await api.capabilities();
    } catch (_) {
      return capabilities;
    }
  }

  @override
  void dispose() {
    _wsSub?.cancel();
    ws?.dispose();
    super.dispose();
  }
}
