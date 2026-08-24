// [V7 §17.x] TV 焦点归属栈 + 丢失恢复 + 弹窗 savepoint.
//
// 与 widgets/focus.dart 的关系:
//   focus.dart        = 组件层封装 (KodiFocus / KodiShortcuts)
//   focus_manager.dart = 中枢层 (本页) — 跨页面共享栈 + primaryFocus 监听
//
// 核心 API:
//   KodiFocusManager.instance.pushFocus(node)   弹窗打开时 push 触发按钮的节点
//   KodiFocusManager.instance.popFocus()       弹窗关闭时 pop 并自动 restoreFocus()
//   KodiFocusManager.instance.restoreFocus()   焦点丢失时手动恢复到栈顶
//   KodiFocusManager.instance.savepoint()      弹窗嵌套场景: 保存/恢复栈状态
//
// 健康面板 (V7 §17.x.5) 通过监听 Flutter WidgetsBinding 的 primaryFocus 即可.

import 'package:flutter/widgets.dart';

/// 弹窗打开前保存当前栈状态, 关闭后调用 [restore] 恢复到那一刻.
/// 用法:
///
/// ```dart
/// final sp = KodiFocusManager.instance.savepoint();
/// KodiFocusManager.instance.pushFocus(firstDialogNode);
/// showDialog(...).whenComplete(sp.restore);
/// ```
class FocusSavepoint {
  FocusSavepoint._(this._snapshot);
  final List<FocusNode> _snapshot;

  /// 恢复栈到 savepoint 时刻并请求焦点.
  void restore() {
    final fm = KodiFocusManager.instance;
    fm._stack
      ..clear()
      ..addAll(_snapshot);
    fm.restoreFocus();
  }
}

/// 焦点管理单例 (V7 §17.x.1). 整个 App 共享一个栈.
class KodiFocusManager {
  KodiFocusManager._();
  static final KodiFocusManager instance = KodiFocusManager._();

  final List<FocusNode> _stack = [];
  FocusNode? _rootFocus;

  /// 栈深度 (用于调试 / 健康面板).
  int get stackDepth => _stack.length;

  /// 当前栈顶焦点节点 (栈空时为 null).
  FocusNode? get currentFocus => _stack.isEmpty ? null : _stack.last;

  /// 设置根焦点节点 — 当栈空时 [restoreFocus] 退而求其次使用它.
  void setRootFocus(FocusNode node) {
    _rootFocus = node;
  }

  /// 推送焦点节点. 调用方负责节点生命周期 (一般由 KodiFocus 自管).
  void pushFocus(FocusNode node) {
    _stack.add(node);
  }

  /// 弹出栈顶. 若弹完后栈非空, 自动调用 [restoreFocus].
  void popFocus() {
    if (_stack.isEmpty) return;
    _stack.removeLast();
    restoreFocus();
  }

  /// 恢复到栈顶节点 (栈空则用 rootFocus). 异步触发 Flutter 焦点树重建.
  void restoreFocus() {
    final target = currentFocus ?? _rootFocus;
    target?.requestFocus();
  }

  /// 保存当前栈快照用于嵌套弹窗场景 (V7 §17.x.3).
  FocusSavepoint savepoint() {
    return FocusSavepoint._(List<FocusNode>.from(_stack));
  }

  /// 测试 / 路由切换时整体重置.
  void reset() {
    _stack.clear();
    _rootFocus = null;
  }
}
