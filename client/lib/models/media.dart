// X-MEDIA 客户端数据模型。

class MediaSummary {
  final int externalId;
  final String externalSource;
  final String mediaType;
  final String title;
  final String titleOrig;
  final int year;
  final double voteAvg;
  final String posterUrl;
  final String backdropUrl;
  final String overview;
  final List<String> genres;

  const MediaSummary({
    required this.externalId,
    required this.externalSource,
    required this.mediaType,
    required this.title,
    this.titleOrig = '',
    this.year = 0,
    this.voteAvg = 0,
    this.posterUrl = '',
    this.backdropUrl = '',
    this.overview = '',
    this.genres = const [],
  });

  factory MediaSummary.fromJson(Map<String, dynamic> j) => MediaSummary(
        externalId: (j['external_id'] as num?)?.toInt() ?? 0,
        externalSource: j['external_source'] as String? ?? 'tmdb',
        mediaType: j['media_type'] as String? ?? 'movie',
        title: j['title'] as String? ?? '',
        titleOrig: j['title_orig'] as String? ?? '',
        year: (j['year'] as num?)?.toInt() ?? 0,
        voteAvg: (j['vote_avg'] as num?)?.toDouble() ?? 0,
        posterUrl: j['poster_url'] as String? ?? '',
        backdropUrl: j['backdrop_url'] as String? ?? '',
        overview: j['overview'] as String? ?? '',
        genres: (j['genres'] as List<dynamic>?)?.map((e) => e.toString()).toList() ?? const [],
      );
}

class Section {
  final String key;
  final String title;
  final List<MediaSummary> items;
  const Section({required this.key, required this.title, required this.items});

  factory Section.fromJson(Map<String, dynamic> j) => Section(
        key: j['key'] as String? ?? '',
        title: j['title'] as String? ?? '',
        items: (j['items'] as List<dynamic>?)
                ?.map((e) => MediaSummary.fromJson(e as Map<String, dynamic>))
                .toList() ??
            const [],
      );
}

class CastMember {
  final String name;
  final String character;
  const CastMember({required this.name, this.character = ''});
  factory CastMember.fromJson(Map<String, dynamic> j) => CastMember(
        name: j['name'] as String? ?? '',
        character: j['character'] as String? ?? '',
      );
}

class EpisodeInfo {
  final int episodeNumber;
  final String name;
  final bool available;
  const EpisodeInfo({required this.episodeNumber, this.name = '', this.available = false});
  factory EpisodeInfo.fromJson(Map<String, dynamic> j) => EpisodeInfo(
        episodeNumber: (j['episode_number'] as num?)?.toInt() ?? 0,
        name: j['name'] as String? ?? '',
        available: j['available'] == true,
      );
}

class SeasonInfo {
  final int seasonNumber;
  final String name;
  final int episodeCount;
  final List<EpisodeInfo> episodes;
  const SeasonInfo({required this.seasonNumber, this.name = '', this.episodeCount = 0, this.episodes = const []});
  factory SeasonInfo.fromJson(Map<String, dynamic> j) => SeasonInfo(
        seasonNumber: (j['season_number'] as num?)?.toInt() ?? 0,
        name: j['name'] as String? ?? '',
        episodeCount: (j['episode_count'] as num?)?.toInt() ?? 0,
        episodes: (j['episodes'] as List<dynamic>?)
                ?.map((e) => EpisodeInfo.fromJson(e as Map<String, dynamic>))
                .toList() ??
            const [],
      );
}

class MediaDetail {
  final MediaSummary summary;
  final int runtime;
  final int seasons;
  final int episodes;
  final List<SeasonInfo> seasonsList;
  final List<CastMember> cast;

  const MediaDetail({
    required this.summary,
    this.runtime = 0,
    this.seasons = 0,
    this.episodes = 0,
    this.seasonsList = const [],
    this.cast = const [],
  });

  bool get isSeries => seasons > 0;

  factory MediaDetail.fromJson(Map<String, dynamic> j) => MediaDetail(
        summary: MediaSummary.fromJson(j),
        runtime: (j['runtime'] as num?)?.toInt() ?? 0,
        seasons: (j['seasons'] as num?)?.toInt() ?? 0,
        episodes: (j['episodes'] as num?)?.toInt() ?? 0,
        seasonsList: (j['seasons_list'] as List<dynamic>?)
                ?.map((e) => SeasonInfo.fromJson(e as Map<String, dynamic>))
                .toList() ??
            const [],
        cast: (j['cast'] as List<dynamic>?)
                ?.map((e) => CastMember.fromJson(e as Map<String, dynamic>))
                .toList() ??
            const [],
      );
}

class ContinueWatching {
  final int externalId;
  final String externalSource;
  final String mediaType;
  final String title;
  final String posterUrl;
  final int season;
  final int episode;
  final int positionMs;
  final int durationMs;
  final String playedAt;

  const ContinueWatching({
    required this.externalId,
    this.externalSource = 'tmdb',
    this.mediaType = '',
    this.title = '',
    this.posterUrl = '',
    this.season = 0,
    this.episode = 0,
    this.positionMs = 0,
    this.durationMs = 0,
    this.playedAt = '',
  });

  double get progress => durationMs > 0 ? (positionMs / durationMs).clamp(0, 1) : 0;
  String get episodeLabel => season > 0 ? 'S${season.toString().padLeft(2, '0')}E${episode.toString().padLeft(2, '0')}' : '';

  factory ContinueWatching.fromJson(Map<String, dynamic> j) => ContinueWatching(
        externalId: (j['external_id'] as num?)?.toInt() ?? 0,
        externalSource: j['external_source'] as String? ?? 'tmdb',
        mediaType: j['media_type'] as String? ?? '',
        title: j['title'] as String? ?? '',
        posterUrl: j['poster_url'] as String? ?? '',
        season: (j['season'] as num?)?.toInt() ?? 0,
        episode: (j['episode'] as num?)?.toInt() ?? 0,
        positionMs: (j['position_ms'] as num?)?.toInt() ?? 0,
        durationMs: (j['duration_ms'] as num?)?.toInt() ?? 0,
        playedAt: j['played_at'] as String? ?? '',
      );
}

class HistoryItem {
  final int externalId;
  final String externalSource;
  final String mediaType;
  final String title;
  final String posterUrl;
  final String sourceType;
  final int season;
  final int episode;
  final int positionMs;
  final int durationMs;

  const HistoryItem({
    required this.externalId,
    this.externalSource = 'tmdb',
    this.mediaType = '',
    this.title = '',
    this.posterUrl = '',
    this.sourceType = '',
    this.season = 0,
    this.episode = 0,
    this.positionMs = 0,
    this.durationMs = 0,
  });

  factory HistoryItem.fromJson(Map<String, dynamic> j) => HistoryItem(
        externalId: (j['external_id'] as num?)?.toInt() ?? 0,
        externalSource: j['external_source'] as String? ?? 'tmdb',
        mediaType: j['media_type'] as String? ?? '',
        title: j['title'] as String? ?? '',
        posterUrl: j['poster_url'] as String? ?? '',
        sourceType: j['source_type'] as String? ?? '',
        season: (j['season'] as num?)?.toInt() ?? 0,
        episode: (j['episode'] as num?)?.toInt() ?? 0,
        positionMs: (j['position_ms'] as num?)?.toInt() ?? 0,
        durationMs: (j['duration_ms'] as num?)?.toInt() ?? 0,
      );
}

class Favorite {
  final int externalId;
  final String title;
  final int year;
  final String posterUrl;
  const Favorite({required this.externalId, this.title = '', this.year = 0, this.posterUrl = ''});
  factory Favorite.fromJson(Map<String, dynamic> j) => Favorite(
        externalId: (j['external_id'] as num?)?.toInt() ?? 0,
        title: j['title'] as String? ?? '',
        year: (j['year'] as num?)?.toInt() ?? 0,
        posterUrl: j['poster_url'] as String? ?? '',
      );
}

class SubscriptionItem {
  final int externalId;
  final String title;
  final int year;
  final String status;
  final int searchCount;
  final int maxSearches;
  const SubscriptionItem({
    required this.externalId,
    this.title = '',
    this.year = 0,
    this.status = 'watching',
    this.searchCount = 0,
    this.maxSearches = 12,
  });
  factory SubscriptionItem.fromJson(Map<String, dynamic> j) => SubscriptionItem(
        externalId: (j['external_id'] as num?)?.toInt() ?? 0,
        title: j['title'] as String? ?? '',
        year: (j['year'] as num?)?.toInt() ?? 0,
        status: j['status'] as String? ?? 'watching',
        searchCount: (j['search_count'] as num?)?.toInt() ?? 0,
        maxSearches: (j['max_searches'] as num?)?.toInt() ?? 12,
      );
}

class Capabilities {
  final bool nasAvailable;
  final bool nasIndexComplete;
  final bool pansearchAvailable;
  final List<String> loggedInDrivers;
  final String serverVersion;
  const Capabilities({
    this.nasAvailable = false,
    this.nasIndexComplete = false,
    this.pansearchAvailable = false,
    this.loggedInDrivers = const [],
    this.serverVersion = '',
  });
  factory Capabilities.fromJson(Map<String, dynamic> j) => Capabilities(
        nasAvailable: j['nas_available'] == true,
        nasIndexComplete: j['nas_index_complete'] == true,
        pansearchAvailable: j['pansearch_available'] == true,
        loggedInDrivers: (j['logged_in_drivers'] as List<dynamic>?)?.map((e) => e.toString()).toList() ?? const [],
        serverVersion: j['server_version'] as String? ?? '',
      );
}

/// 解析阶段（对应后端 ResolveStage）。
enum ResolveStage {
  resolveStart('准备中...'),
  nasLookup('查询本地索引...'),
  nasHit('找到本地文件 ✓'),
  panSearching('搜索全网盘资源...'),
  panSearched('分析搜索结果...'),
  transferring('转存中...'),
  resolvingLink('获取播放链接...'),
  playReady('播放就绪 ✓'),
  magnetDownloading('云端下载中...'),
  notFound('暂无可用资源'),
  error('出错了');

  final String label;
  const ResolveStage(this.label);

  static ResolveStage from(String? s) => ResolveStage.values.firstWhere(
        (e) => e.name == s,
        orElse: () => ResolveStage.error,
      );
}

/// [V7 §28.3] 后端快照 — 用于客户端感知重启.
class StateSnapshot {
  final String serverStartedAt;
  final String lastRestartReason;
  const StateSnapshot({this.serverStartedAt = '', this.lastRestartReason = ''});

  factory StateSnapshot.fromJson(Map<String, dynamic> j) => StateSnapshot(
        serverStartedAt: j['server_started_at'] as String? ?? '',
        lastRestartReason: j['last_restart_reason'] as String? ?? '',
      );
}

class ResolveState {
  final int taskId;
  final ResolveStage stage;
  final String detail;
  final int progressPct;
  final String? streamUrl;
  final String? source;
  final bool reused;
  final String? errorMsg;

  const ResolveState({
    this.taskId = 0,
    this.stage = ResolveStage.resolveStart,
    this.detail = '',
    this.progressPct = 0,
    this.streamUrl,
    this.source,
    this.reused = false,
    this.errorMsg,
  });

  bool get isTerminal =>
      stage == ResolveStage.playReady || stage == ResolveStage.notFound || stage == ResolveStage.error;
  bool get isSuccess => stage == ResolveStage.playReady;
}
