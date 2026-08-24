// [V7 §17.5] Resolve Modal 分层进度指示器逻辑测试.
//
// 验证:
//   1. resolveLayerForStage: stage → layer (0/1/2/3) 映射
//   2. P0 跳过: NAS 不可用时 P0 layer 灰显
//   3. P2 阶段: 进度条用 resolveP2 黄色高亮
import 'package:flutter_test/flutter_test.dart';
import 'package:xmedia_client/models/media.dart';
import 'package:xmedia_client/widgets/resolve_modal_helpers.dart';

void main() {
  group('resolveLayerForStage (V7 §17.5 四层映射)', () {
    test('正常路径: 全部 stage 映射到 0/1/2/3', () {
      expect(resolveLayerForStage(ResolveStage.resolveStart, skipNas: false), 0);
      expect(resolveLayerForStage(ResolveStage.nasLookup, skipNas: false), 0);
      expect(resolveLayerForStage(ResolveStage.nasHit, skipNas: false), 0);
      expect(resolveLayerForStage(ResolveStage.panSearching, skipNas: false), 1);
      expect(resolveLayerForStage(ResolveStage.panSearched, skipNas: false), 1);
      expect(resolveLayerForStage(ResolveStage.transferring, skipNas: false), 1);
      expect(resolveLayerForStage(ResolveStage.resolvingLink, skipNas: false), 1);
      expect(resolveLayerForStage(ResolveStage.magnetDownloading, skipNas: false), 2);
      expect(resolveLayerForStage(ResolveStage.notFound, skipNas: false), 3);
    });

    test('P0 跳过: NAS 未配置/索引未完成时 P0 灰显', () {
      // §17.5: "P0 跳过时不显示 P0: NAS 未配置或索引未完成时,
      //        指示器直接从 P1 开始, P0 灰显 + 跳过标记."
      expect(resolveLayerForStage(ResolveStage.resolveStart, skipNas: true), 1);
      expect(resolveLayerForStage(ResolveStage.nasLookup, skipNas: true), 1);
      expect(resolveLayerForStage(ResolveStage.nasHit, skipNas: true), 1);
      // 后续层不受影响
      expect(resolveLayerForStage(ResolveStage.panSearching, skipNas: true), 1);
      expect(resolveLayerForStage(ResolveStage.magnetDownloading, skipNas: true), 2);
    });

    test('terminal 状态: playReady/error 视作完成 (返回 4 表示已结束)', () {
      expect(resolveLayerForStage(ResolveStage.playReady, skipNas: false), 4);
      expect(resolveLayerForStage(ResolveStage.error, skipNas: false), 4);
    });
  });

  group('shouldShowProgressBar (V7 §17.5 P2 进度条)', () {
    test('P0/P1 阶段不突出进度条 (普通文字提示)', () {
      expect(shouldShowProgressBar(ResolveStage.resolveStart), isFalse);
      expect(shouldShowProgressBar(ResolveStage.panSearching), isFalse);
      expect(shouldShowProgressBar(ResolveStage.transferring), isFalse);
    });

    test('P2 阶段突出进度条 (云下载小时级)', () {
      expect(shouldShowProgressBar(ResolveStage.magnetDownloading), isTrue);
    });
  });

  group('shouldSkipP0 (V7 §6.3 + §17.5 P0 智能跳过)', () {
    test('NAS 不可用 → skipNas=true', () {
      expect(shouldSkipP0(nasAvailable: false, nasIndexComplete: false), isTrue);
    });
    test('NAS 可用但索引未完成 → skipNas=true', () {
      expect(shouldSkipP0(nasAvailable: true, nasIndexComplete: false), isTrue);
    });
    test('NAS 可用且索引完成 → skipNas=false', () {
      expect(shouldSkipP0(nasAvailable: true, nasIndexComplete: true), isFalse);
    });
  });
}
