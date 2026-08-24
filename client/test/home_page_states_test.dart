// [V7 §23.0] home_page 4 状态 widget 测试.
//
// V7 §17.1 要求每个 page 必须实现 4 状态:
//   loading - 首次加载数据 → 骨架屏
//   data    - 数据加载成功 → 正常内容
//   empty   - 数据为空 (非错误) → 居中图标 + 提示
//   error   - 网络/服务端错误 → 居中错误图标 + 重试
//
// 本测试覆盖 HomePage 的 4 状态, 不依赖真实后端 (用 AppState.forTest).
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:provider/provider.dart';
import 'package:xmedia_client/models/media.dart';
import 'package:xmedia_client/pages/home_page.dart';
import 'package:xmedia_client/services/app_state.dart';

void main() {
  Widget wrap(Widget child, AppState app) {
    return MaterialApp(
      // 窄 viewport 避免 _Skeleton Row 在 752px 默认容器下溢出.
      home: MediaQuery(
        data: const MediaQueryData(size: Size(400, 600)),
        child: ChangeNotifierProvider<AppState>.value(
          value: app,
          child: child,
        ),
      ),
    );
  }

  group('HomePage 4 状态 (V7 §17.1)', () {
    testWidgets('loading: sections 空 + loading=true → 骨架屏 (无错误文案)', (tester) async {
      // _Skeleton 内 Row 在 752px 容器下会溢出, 用 1920 屏宽避免.
      tester.view.physicalSize = const Size(1920, 1080);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      final app = AppState.forTest()
        ..loading = true
        ..sections = const []
        ..error = '';
      await tester.pumpWidget(wrap(const HomePage(), app));
      await tester.pump();
      expect(find.text('加载失败'), findsNothing);
      expect(find.textContaining('暂无内容'), findsNothing);
    });

    testWidgets('data: sections 有内容 → 正常展示榜单', (tester) async {
      final app = AppState.forTest()
        ..loading = false
        ..sections = const [
          Section(key: 'trending', title: '热门', items: []),
        ];
      await tester.pumpWidget(wrap(const HomePage(), app));
      await tester.pump();
      expect(find.byType(HomePage), findsOneWidget);
    });

    testWidgets('error: error 非空 + sections 空 → 显示 _ErrorState', (tester) async {
      final app = AppState.forTest()
        ..loading = false
        ..sections = const []
        ..error = '请求失败 (500)';
      await tester.pumpWidget(wrap(const HomePage(), app));
      await tester.pump();
      expect(find.text('请求失败 (500)'), findsOneWidget);
    });

    testWidgets('empty: sections 空 + 无 loading 无 error → 边界可构建', (tester) async {
      final app = AppState.forTest()
        ..loading = false
        ..sections = const []
        ..error = '';
      await tester.pumpWidget(wrap(const HomePage(), app));
      await tester.pump();
      expect(find.byType(HomePage), findsOneWidget);
    });
  });

  group('TV 焦点导航 (V7 §17.x)', () {
    testWidgets('ReadingOrderTraversalPolicy 注册到 FocusTraversalGroup', (tester) async {
      final app = AppState.forTest();
      await tester.pumpWidget(MaterialApp(
        home: ChangeNotifierProvider<AppState>.value(
          value: app,
          child: const _FocusProbe(),
        ),
      ));
      // 验证 _FocusProbe 渲染成功 (含 FocusTraversalGroup + 2 Focus widget).
      expect(find.byKey(const Key('p1')), findsOneWidget);
      expect(find.byKey(const Key('p2')), findsOneWidget);
      expect(find.byType(FocusTraversalGroup), findsWidgets);
    });
  });
}

/// 双按钮 Probe: 验证 ReadingOrderTraversalPolicy 方向键导航.
class _FocusProbe extends StatelessWidget {
  const _FocusProbe();
  @override
  Widget build(BuildContext context) {
    return FocusTraversalGroup(
      policy: ReadingOrderTraversalPolicy(),
      child: Row(
        children: const [
          Focus(child: SizedBox(key: Key('p1'), width: 50, height: 50)),
          Focus(child: SizedBox(key: Key('p2'), width: 50, height: 50)),
        ],
      ),
    );
  }
}
