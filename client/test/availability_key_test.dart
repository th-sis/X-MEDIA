// [V7 §17.2 / §17.4] 可播放角标 — AvailabilityKey 解析 + 去重.
//
// 后端 POST /api/media/check-availability 接受 items 数组:
//   [{external_id, external_source, season, episode}]
// 返回 available 列表. 前端把已索引 ID 集合传入 PosterCard,
// 渲染右上角绿色 ✓ 角标 (§17.2 D53).
import 'package:flutter_test/flutter_test.dart';
import 'package:xmedia_client/services/availability.dart';

void main() {
  group('AvailabilityKey (V7 §17.4 季集维度)', () {
    test('equatable: 同字段 → 同 key (movie season=0 episode=0)', () {
      const a = AvailabilityKey(externalId: 19995, externalSource: 'tmdb', season: 0, episode: 0);
      const b = AvailabilityKey(externalId: 19995, externalSource: 'tmdb', season: 0, episode: 0);
      expect(a, equals(b));
      expect(a.hashCode, equals(b.hashCode));
    });

    test('equatable: 季集不同 → 不同 key (§17.4 季集维度)', () {
      const a = AvailabilityKey(externalId: 1399, externalSource: 'tmdb', season: 1, episode: 1);
      const b = AvailabilityKey(externalId: 1399, externalSource: 'tmdb', season: 1, episode: 2);
      expect(a, isNot(equals(b)));
    });

    test('dedupe: 去重保留首次出现', () {
      final list = [
        const AvailabilityKey(externalId: 19995, externalSource: 'tmdb', season: 0, episode: 0),
        const AvailabilityKey(externalId: 1399, externalSource: 'tmdb', season: 1, episode: 1),
        const AvailabilityKey(externalId: 19995, externalSource: 'tmdb', season: 0, episode: 0), // duplicate
      ];
      final deduped = dedupeAvailabilityKeys(list);
      expect(deduped.length, 2);
    });

    test('fromMediaSummary: 电影默认 season=0 episode=0', () {
      final key = availabilityKeyForSummary(externalId: 19995, externalSource: 'tmdb', season: 0, episode: 0);
      expect(key.season, 0);
      expect(key.episode, 0);
    });

    test('fromMediaSummary: 电视剧季集正确', () {
      final key = availabilityKeyForSummary(externalId: 1399, externalSource: 'tmdb', season: 1, episode: 3);
      expect(key.season, 1);
      expect(key.episode, 3);
    });
  });
}
