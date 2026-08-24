// [V7 §26.1] 播放器 seek 钳位逻辑单元测试.

import 'package:flutter_test/flutter_test.dart';
import 'package:xmedia_client/pages/player_seek.dart';

void main() {
  group('clampSeek (V7 §26.1 播放器 seek 防抖钳位)', () {
    const pos = Duration(seconds: 60);
    const dur = Duration(minutes: 10);

    test('前进 10s', () {
      expect(clampSeek(pos, dur, 10), const Duration(seconds: 70));
    });

    test('后退 10s', () {
      expect(clampSeek(pos, dur, -10), const Duration(seconds: 50));
    });

    test('越界贴边: 接近片头时后退 → 0', () {
      expect(clampSeek(const Duration(seconds: 5), dur, -10), Duration.zero);
    });

    test('越界贴边: 接近片尾时前进 → duration', () {
      final nearEnd = dur - const Duration(seconds: 3);
      expect(clampSeek(nearEnd, dur, 10), dur);
    });

    test('duration 未初始化 (0) 时原样返回', () {
      expect(clampSeek(pos, Duration.zero, 10), pos);
      expect(clampSeek(Duration.zero, Duration.zero, 10), Duration.zero);
    });
  });
}
