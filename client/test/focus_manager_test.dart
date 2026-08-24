// [V7 §17.x] TV 焦点管理单元测试.
// 验证焦点归属栈 push/pop/栈深度/savepoint. 真正的 hasFocus 状态依赖 widget tree
// (requestFocus 需要节点附着到 FocusScope), 这里只测栈语义.
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:xmedia_client/widgets/focus_manager.dart';

void main() {
  group('KodiFocusManager (V7 §17.x.1 焦点归属栈)', () {
    setUp(() {
      KodiFocusManager.instance.reset();
    });

    test('push/pop 维护栈顺序', () {
      final fm = KodiFocusManager.instance;
      expect(fm.stackDepth, 0);
      expect(fm.currentFocus, isNull);

      final n1 = FocusNode(debugLabel: 'n1');
      final n2 = FocusNode(debugLabel: 'n2');
      fm.pushFocus(n1);
      expect(fm.stackDepth, 1);
      expect(fm.currentFocus, equals(n1));

      fm.pushFocus(n2);
      expect(fm.stackDepth, 2);
      expect(fm.currentFocus, equals(n2));

      fm.popFocus();
      expect(fm.stackDepth, 1);
      expect(fm.currentFocus, equals(n1));

      fm.popFocus();
      expect(fm.stackDepth, 0);
      expect(fm.currentFocus, isNull);
    });

    test('currentFocus 栈空时为 null', () {
      final fm = KodiFocusManager.instance;
      expect(fm.currentFocus, isNull);
      // rootFocus 未设置时 popFocus 不崩
      fm.popFocus();
      expect(fm.stackDepth, 0);
    });

    test('pop 空栈幂等 (不崩)', () {
      final fm = KodiFocusManager.instance;
      fm.popFocus();
      fm.popFocus();
      fm.popFocus();
      expect(fm.stackDepth, 0);
    });

    test('push 同一节点多次不报错', () {
      final fm = KodiFocusManager.instance;
      final n1 = FocusNode(debugLabel: 'n1');
      fm.pushFocus(n1);
      fm.pushFocus(n1);
      fm.pushFocus(n1);
      expect(fm.stackDepth, 3);
    });

    testWidgets('savepoint.restore 恢复到快照时刻', (tester) async {
      final fm = KodiFocusManager.instance;
      final n1 = FocusNode(debugLabel: 'n1');
      fm.pushFocus(n1);
      expect(fm.stackDepth, 1);

      final sp = fm.savepoint();
      final n2 = FocusNode(debugLabel: 'n2');
      fm.pushFocus(n2);
      expect(fm.stackDepth, 2);

      sp.restore();
      expect(fm.stackDepth, 1);
      expect(fm.currentFocus, equals(n1));
    });

    testWidgets('嵌套 savepoint (弹窗栈)', (tester) async {
      final fm = KodiFocusManager.instance;
      final n1 = FocusNode(debugLabel: 'n1');
      fm.pushFocus(n1);

      // 第一层弹窗
      final sp1 = fm.savepoint();
      final n2 = FocusNode(debugLabel: 'n2');
      fm.pushFocus(n2);
      expect(fm.currentFocus, equals(n2));

      // 第二层嵌套弹窗
      final sp2 = fm.savepoint();
      final n3 = FocusNode(debugLabel: 'n3');
      fm.pushFocus(n3);
      expect(fm.currentFocus, equals(n3));

      // 关闭第二层: 回到 n2
      sp2.restore();
      expect(fm.currentFocus, equals(n2));

      // 关闭第一层: 回到 n1
      sp1.restore();
      expect(fm.currentFocus, equals(n1));
    });
  });

  group('KodiFocusManager 焦点丢失守卫 (V7 §17.x.2/§17.x.5)', () {
    setUp(() {
      KodiFocusManager.instance.reset();
    });

    tearDown(() {
      KodiFocusManager.instance.stopLossGuard();
    });

    testWidgets('子树 detach 场景: 守卫不崩、不误抢 scope 兜底焦点', (tester) async {
      final fm = KodiFocusManager.instance;
      final node = FocusNode(debugLabel: 'guarded');

      await tester.pumpWidget(Directionality(
        textDirection: TextDirection.ltr,
        child: Focus(focusNode: node, autofocus: true, child: const SizedBox()),
      ));
      await tester.pumpAndSettle();
      expect(node.hasFocus, isTrue);

      fm.pushFocus(node);
      fm.startLossGuard();

      // 整棵树销毁. 注意: Flutter binding 的 root scope 会兜住焦点,
      // primaryFocus 实际不会变 null (实测), 守卫因此不应动作.
      await tester.pumpWidget(const SizedBox());
      await tester.pumpAndSettle();
      expect(tester.takeException(), isNull,
          reason: '§17.x.5 守卫在极端场景下不得抛异常或误恢复');

      // testWidgets 的 pending-timer 校验先于 tearDown, 需显式停守卫.
      fm.stopLossGuard();
    });

    testWidgets('restoreFocus 从栈顶向下选中第一个仍附着的节点', (tester) async {
      final fm = KodiFocusManager.instance;
      final stale = FocusNode(debugLabel: 'stale'); // 已离树的失效节点
      final live = FocusNode(debugLabel: 'live'); // 仍在树上

      await tester.pumpWidget(Directionality(
        textDirection: TextDirection.ltr,
        child: Focus(focusNode: live, child: const SizedBox()),
      ));
      await tester.pumpAndSettle();

      fm.pushFocus(stale);
      fm.pushFocus(live);
      // 模拟焦点漂移后手动恢复.
      FocusManager.instance.primaryFocus?.unfocus();
      await tester.pumpAndSettle();
      fm.restoreFocus();
      await tester.pumpAndSettle();

      expect(live.hasFocus, isTrue,
          reason: '恢复应跳过失效节点, 选中栈内最近的存活目标');
    });

    test('stopLossGuard 后守卫停止', () {
      final fm = KodiFocusManager.instance;
      fm.startLossGuard();
      expect(fm.lossGuardActive, isTrue);
      fm.stopLossGuard();
      expect(fm.lossGuardActive, isFalse);
    });

    test('startLossGuard 幂等 (不叠加 listener/timer)', () {
      final fm = KodiFocusManager.instance;
      fm.startLossGuard();
      fm.startLossGuard();
      fm.startLossGuard();
      expect(fm.lossGuardActive, isTrue);
      fm.stopLossGuard();
      expect(fm.lossGuardActive, isFalse, reason: '幂等启动只注册一份, 停一次即全停');
    });

    test('recoverIfLost 无树无焦点时不崩', () {
      final fm = KodiFocusManager.instance;
      fm.recoverIfLost(); // primaryFocus 为 null 但栈/root 也空 → no-op
      expect(fm.currentFocus, isNull);
    });
  });
}
