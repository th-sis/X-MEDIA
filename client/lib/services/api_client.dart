import 'dart:convert';
import 'package:http/http.dart' as http;
import '../models/media.dart';

class ApiClient {
  String baseUrl;
  ApiClient(this.baseUrl);

  Uri _u(String path, [Map<String, String>? q]) {
    final base = baseUrl.endsWith('/') ? baseUrl.substring(0, baseUrl.length - 1) : baseUrl;
    return Uri.parse('$base$path').replace(queryParameters: q?.isNotEmpty == true ? q : null);
  }

  Map<String, dynamic> _decode(http.Response r) {
    if (r.statusCode >= 200 && r.statusCode < 300) {
      return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
    }
    throw ApiException(r.statusCode, _errMsg(r));
  }

  String _errMsg(http.Response r) {
    try {
      final m = jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
      return m['error'] as String? ?? m['message'] as String? ?? '请求失败';
    } catch (_) {
      return '请求失败 (${r.statusCode})';
    }
  }

  // ---- TMDB 元数据 ----

  Future<List<Section>> home() async {
    final r = await http.get(_u('/api/tmdb/home')).timeout(const Duration(seconds: 12));
    final j = _decode(r);
    final list = (j['sections'] as List<dynamic>?) ?? const [];
    return list.map((e) => Section.fromJson(e as Map<String, dynamic>)).toList();
  }

  Future<List<MediaSummary>> discover(String type, {String genre = '', int page = 1}) async {
    final r = await http.get(_u('/api/tmdb/discover', {
      'type': type, if (genre.isNotEmpty) 'genre': genre, 'page': '$page',
    })).timeout(const Duration(seconds: 12));
    final j = _decode(r);
    return (j['items'] as List<dynamic>?)?.map((e) => MediaSummary.fromJson(e as Map<String, dynamic>)).toList() ?? const [];
  }

  Future<List<MediaSummary>> search(String q) async {
    final r = await http.get(_u('/api/tmdb/search', {'q': q})).timeout(const Duration(seconds: 12));
    final j = _decode(r);
    return (j['items'] as List<dynamic>?)?.map((e) => MediaSummary.fromJson(e as Map<String, dynamic>)).toList() ?? const [];
  }

  Future<List<MediaSummary>> panSearch(String q) async {
    final r = await http.get(_u('/api/media/pansearch', {'q': q})).timeout(const Duration(seconds: 12));
    final j = _decode(r);
    return (j['items'] as List<dynamic>?)?.map((e) => MediaSummary.fromJson(e as Map<String, dynamic>)).toList() ?? const [];
  }

  Future<MediaDetail> detail(int id, {String source = 'tmdb'}) async {
    final r = await http.get(_u('/api/tmdb/detail/$id', {'source': source})).timeout(const Duration(seconds: 12));
    return MediaDetail.fromJson(_decode(r));
  }

  Future<List<SeasonInfo>> seasons(int id, {String source = 'tmdb'}) async {
    final r = await http.get(_u('/api/tmdb/seasons/$id', {'source': source})).timeout(const Duration(seconds: 12));
    final j = _decode(r);
    return (j as List<dynamic>?)?.map((e) => SeasonInfo.fromJson(e as Map<String, dynamic>)).toList() ?? const [];
  }

  // ---- 播放引擎 ----

  Future<ResolveState> resolve({
    required int externalId,
    String source = 'tmdb',
    String mediaType = 'movie',
    String title = '',
    int year = 0,
    int season = 0,
    int episode = 0,
  }) async {
    final r = await http.post(
      _u('/api/resolve'),
      headers: {'Content-Type': 'application/json'},
      body: jsonEncode({
        'external_id': externalId,
        'external_source': source,
        'media_type': mediaType,
        'title': title,
        'year': year,
        'season': season,
        'episode': episode,
      }),
    ).timeout(const Duration(seconds: 8));
    final j = _decode(r);
    return ResolveState(taskId: (j['task_id'] as num?)?.toInt() ?? 0, reused: j['reused'] == true);
  }

  Future<ResolveState> resolveResult(int taskId) async {
    final r = await http.get(_u('/api/resolve/result/$taskId')).timeout(const Duration(seconds: 8));
    final j = _decode(r);
    if (j['stream_url'] != null) {
      return ResolveState(
        taskId: taskId,
        stage: ResolveStage.playReady,
        streamUrl: j['stream_url'] as String,
        source: j['source'] as String?,
      );
    }
    final status = j['status'] as String? ?? '';
    if (status == 'failed') {
      return ResolveState(
        taskId: taskId,
        stage: ResolveStage.from(j['stage'] as String?),
        errorMsg: j['error_msg'] as String? ?? '暂无可用资源',
      );
    }
    return ResolveState(
      taskId: taskId,
      stage: ResolveStage.from(j['stage'] as String?),
      detail: j['stage_detail'] as String? ?? '',
      progressPct: (j['progress_pct'] as num?)?.toInt() ?? 0,
    );
  }

  // ---- 媒体库 ----

  Future<List<ContinueWatching>> continueWatching() async {
    final r = await http.get(_u('/api/media/continue-watching')).timeout(const Duration(seconds: 8));
    final j = _decode(r);
    return (j['items'] as List<dynamic>?)?.map((e) => ContinueWatching.fromJson(e as Map<String, dynamic>)).toList() ?? const [];
  }

  Future<List<HistoryItem>> history() async {
    final r = await http.get(_u('/api/media/history')).timeout(const Duration(seconds: 8));
    final j = _decode(r);
    return (j['items'] as List<dynamic>?)?.map((e) => HistoryItem.fromJson(e as Map<String, dynamic>)).toList() ?? const [];
  }

  Future<void> reportProgress({
    required int externalId,
    String source = 'tmdb',
    String mediaType = 'movie',
    String title = '',
    String posterUrl = '',
    String sourceType = '',
    int season = 0,
    int episode = 0,
    int positionMs = 0,
    int durationMs = 0,
  }) async {
    await http.post(
      _u('/api/media/history'),
      headers: {'Content-Type': 'application/json'},
      body: jsonEncode({
        'external_id': externalId,
        'external_source': source,
        'media_type': mediaType,
        'title': title,
        'poster_url': posterUrl,
        'source_type': sourceType,
        'season': season,
        'episode': episode,
        'position_ms': positionMs,
        'duration_ms': durationMs,
      }),
    ).timeout(const Duration(seconds: 8));
  }

  Future<void> clearHistory() async {
    await http.delete(_u('/api/media/history')).timeout(const Duration(seconds: 8));
  }

  Future<List<Favorite>> favorites() async {
    final r = await http.get(_u('/api/media/favorites')).timeout(const Duration(seconds: 8));
    final j = _decode(r);
    return (j['items'] as List<dynamic>?)?.map((e) => Favorite.fromJson(e as Map<String, dynamic>)).toList() ?? const [];
  }

  Future<void> addFavorite(int externalId, String title, int year, {String source = 'tmdb', String mediaType = 'movie', String posterUrl = ''}) async {
    await http.post(
      _u('/api/media/favorites'),
      headers: {'Content-Type': 'application/json'},
      body: jsonEncode({'external_id': externalId, 'external_source': source, 'media_type': mediaType, 'title': title, 'year': year, 'poster_url': posterUrl}),
    ).timeout(const Duration(seconds: 8));
  }

  Future<void> removeFavorite(int externalId, {String source = 'tmdb'}) async {
    await http.delete(_u('/api/media/favorites/$externalId', {'source': source})).timeout(const Duration(seconds: 8));
  }

  Future<List<SubscriptionItem>> subscriptions() async {
    final r = await http.get(_u('/api/media/subscriptions')).timeout(const Duration(seconds: 8));
    final j = _decode(r);
    return (j['items'] as List<dynamic>?)?.map((e) => SubscriptionItem.fromJson(e as Map<String, dynamic>)).toList() ?? const [];
  }

  Future<void> addSubscription(int externalId, String title, int year, {String source = 'tmdb', String mediaType = 'movie', String posterUrl = ''}) async {
    await http.post(
      _u('/api/media/subscriptions'),
      headers: {'Content-Type': 'application/json'},
      body: jsonEncode({'external_id': externalId, 'external_source': source, 'media_type': mediaType, 'title': title, 'year': year, 'poster_url': posterUrl}),
    ).timeout(const Duration(seconds: 8));
  }

  Future<void> removeSubscription(int externalId, {String source = 'tmdb'}) async {
    await http.delete(_u('/api/media/subscriptions/$externalId', {'source': source})).timeout(const Duration(seconds: 8));
  }

  Future<List<String>> searchHistory() async {
    final r = await http.get(_u('/api/media/search-history')).timeout(const Duration(seconds: 8));
    final j = _decode(r);
    return (j['items'] as List<dynamic>?)?.map((e) => (e as Map<String, dynamic>)['keyword'] as String? ?? '').where((e) => e.isNotEmpty).toList() ?? const [];
  }

  Future<void> clearSearchHistory() async {
    await http.delete(_u('/api/media/search-history')).timeout(const Duration(seconds: 8));
  }

  Future<Capabilities> capabilities() async {
    final r = await http.get(_u('/api/capabilities')).timeout(const Duration(seconds: 8));
    return Capabilities.fromJson(_decode(r));
  }

  /// 把相对 stream_url 转成绝对地址。
  String absolute(String urlOrPath) {
    if (urlOrPath.startsWith('http://') || urlOrPath.startsWith('https://')) return urlOrPath;
    final base = baseUrl.endsWith('/') ? baseUrl.substring(0, baseUrl.length - 1) : baseUrl;
    return '$base${urlOrPath.startsWith('/') ? '' : '/'}$urlOrPath';
  }
}

class ApiException implements Exception {
  final int statusCode;
  final String message;
  ApiException(this.statusCode, this.message);
  @override
  String toString() => message;
}
