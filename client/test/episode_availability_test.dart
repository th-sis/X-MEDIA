// [V7 §17.4] EpisodeInfo.available 反序列化测试.
//
// 后端 GET /api/tmdb/seasons/{id} 通过 annotateAvailability 给每个 episode
// 加 available 字段. 前端 EpisodeInfo.fromJson 必须正确解析, 否则 ✓ 角标丢失.
import 'package:flutter_test/flutter_test.dart';
import 'package:xmedia_client/models/media.dart';

void main() {
  group('EpisodeInfo.available (V7 §17.4 季集可用性)', () {
    test('available=true (后端索引命中)', () {
      final json = {
        'episode_number': 3,
        'name': 'The One Where...',
        'available': true,
      };
      final ep = EpisodeInfo.fromJson(json);
      expect(ep.episodeNumber, 3);
      expect(ep.available, isTrue);
    });

    test('available=false (后端未索引)', () {
      final json = {
        'episode_number': 1,
        'name': 'Pilot',
        'available': false,
      };
      final ep = EpisodeInfo.fromJson(json);
      expect(ep.available, isFalse);
    });

    test('available 缺省 = false (后端旧版或错误响应兼容)', () {
      final json = {
        'episode_number': 5,
        'name': 'Episode 5',
      };
      final ep = EpisodeInfo.fromJson(json);
      expect(ep.available, isFalse);
    });
  });

  group('SeasonInfo.episodes 透传 available', () {
    test('季下含多集, available 字段逐一保留', () {
      final json = {
        'season_number': 1,
        'name': 'Season 1',
        'episode_count': 3,
        'episodes': [
          {'episode_number': 1, 'name': 'Ep 1', 'available': false},
          {'episode_number': 2, 'name': 'Ep 2', 'available': true},
          {'episode_number': 3, 'name': 'Ep 3', 'available': true},
        ],
      };
      final s = SeasonInfo.fromJson(json);
      expect(s.episodes.length, 3);
      expect(s.episodes[0].available, isFalse);
      expect(s.episodes[1].available, isTrue);
      expect(s.episodes[2].available, isTrue);
    });
  });
}
