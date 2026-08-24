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
}
