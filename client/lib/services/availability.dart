// [V7 §17.2 / §17.4] 可播放角标 — AvailabilityKey + 去重.
//
// 用于前端批量调 POST /api/media/check-availability, 把已索引 ID 集合传给 PosterCard,
// 渲染右上角绿色 ✓ 角标 (§17.2 D53).
import 'package:flutter/foundation.dart';

@immutable
class AvailabilityKey {
  final int externalId;
  final String externalSource;
  final int season;
  final int episode;

  const AvailabilityKey({
    required this.externalId,
    this.externalSource = 'tmdb',
    this.season = 0,
    this.episode = 0,
  });

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) return true;
    return other is AvailabilityKey &&
        other.externalId == externalId &&
        other.externalSource == externalSource &&
        other.season == season &&
        other.episode == episode;
  }

  @override
  int get hashCode => Object.hash(externalId, externalSource, season, episode);

  Map<String, dynamic> toJson() => {
        'external_id': externalId,
        'external_source': externalSource,
        'season': season,
        'episode': episode,
      };
}

/// 构造 AvailabilityKey 的便利函数.
AvailabilityKey availabilityKeyForSummary({
  required int externalId,
  String externalSource = 'tmdb',
  int season = 0,
  int episode = 0,
}) {
  return AvailabilityKey(
    externalId: externalId,
    externalSource: externalSource,
    season: season,
    episode: episode,
  );
}

/// 列表去重, 保留首次出现. 用于 PosterGrid 在批量查询前压平重复 ID.
List<AvailabilityKey> dedupeAvailabilityKeys(List<AvailabilityKey> input) {
  final seen = <AvailabilityKey>{};
  final out = <AvailabilityKey>[];
  for (final k in input) {
    if (seen.add(k)) out.add(k);
  }
  return out;
}
