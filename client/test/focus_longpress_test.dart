// [V7 §17.x.4] 长按与确认键标准测试.
//
// Android TV: onLongPress = 显示详情; D-pad Center / Enter / Space / Select
// = onActivate (确认). 两者互不触发.

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:xmedia_client/widgets/focus.dart';

void main() {
  group('KodiFocus 长按 (V7 §17.x.4)', () {
    testWidgets('onLongPress 触发且不误触 onActivate', (tester) async {
      var longPressed = false;
      var activated = false;

      await tester.pumpWidget(MaterialApp(
        home: Scaffold(
          body: KodiFocus(
            onActivate: () => activated = true,
            onLongPress: () => longPressed = true,
            builder: (context, focused) => SizedBox(
              width: 100,
              height: 60,
              child: ColoredBox(
                color: focused ? Colors.blue : Colors.grey,
              ),
            ),
          ),
        ),
      ));

      await tester.longPress(find.byType(KodiFocus));
      expect(longPressed, isTrue, reason: '§17.x.4 长按应触发 onLongPress');
      expect(activated, isFalse, reason: '长按不得误触确认回调');
    });

    testWidgets('未注册 onLongPress 时长按不崩 (fallback 为 tap)', (tester) async {
      var activated = false;
      await tester.pumpWidget(MaterialApp(
        home: Scaffold(
          body: KodiFocus(
            onActivate: () => activated = true,
            builder: (context, focused) => const SizedBox(width: 100, height: 60),
          ),
        ),
      ));

      // GestureDetector 无 longPress recognizer 时, 长按序列由 tap 接管
      // (框架默认行为): 不崩, 且确认回调触发.
      await tester.longPress(find.byType(KodiFocus));
      expect(tester.takeException(), isNull);
      expect(activated, isTrue);
    });

    testWidgets('点击仍走 onActivate (回归保护)', (tester) async {
      var activated = false;
      await tester.pumpWidget(MaterialApp(
        home: Scaffold(
          body: KodiFocus(
            onActivate: () => activated = true,
            onLongPress: () {},
            builder: (context, focused) => const SizedBox(width: 100, height: 60),
          ),
        ),
      ));

      await tester.tap(find.byType(KodiFocus));
      expect(activated, isTrue);
    });
  });
}
