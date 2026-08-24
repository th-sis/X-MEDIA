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

import 'dart:async';

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
  VoidCallback? _focusListener;
  Timer? _lossTimer;

  /// [V7 §17.x.5] 焦点丢失守卫是否运行中 (调试 / 测试).
  @visibleForTesting
  bool get lossGuardActive => _lossTimer != null;

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

  /// 恢复到栈顶仍附着的节点 (全部失效则用 rootFocus).
  ///
  /// 栈内节点可能已离树 (弹窗销毁/页面路由移除), 对其 requestFocus 是
  /// no-op — 因此从栈顶向下寻找第一个仍附着 (context != null) 的目标;
  /// 栈本身保持原样, 维护 §17.x.1 的历史记录语义.
  void restoreFocus() {
    for (final node in _stack.reversed) {
      if (node.context != null) {
        node.requestFocus();
        return;
      }
    }
    _rootFocus?.requestFocus();
  }

  /// [V7 §17.x.5] 启动焦点丢失守卫:
  /// - 监听 primaryFocusChanged, 变 null 时下一帧自动恢复到栈顶/root;
  /// - 每 60s 定时兜底检查 (防 listener 漏报的场景).
  /// 幂等: 重复调用只注册一份. App 启动时调用一次, 全局共享.
  ///
  /// 实测注记: Flutter binding 的 root scope 会兜住焦点, 页面销毁后
  /// primaryFocus 实际几乎不会变 null — null 判定按文档保留作防御,
  /// 高频生效的恢复路径是 popFocus/restoreFocus 的存活节点选择.
  void startLossGuard() {
    if (_lossTimer != null) return;
    assert(_focusListener == null);
    _focusListener = _onPrimaryFocusChanged;
    FocusManager.instance.addListener(_focusListener!);
    _lossTimer = Timer.periodic(const Duration(seconds: 60), (_) => recoverIfLost());
  }

  /// 停止守卫 (测试清理用; App 级单例正常不停止).
  void stopLossGuard() {
    if (_focusListener != null) {
      FocusManager.instance.removeListener(_focusListener!);
      _focusListener = null;
    }
    _lossTimer?.cancel();
    _lossTimer = null;
  }

  void _onPrimaryFocusChanged() {
    // 弹窗打开瞬间焦点切换可能短暂为 null, 延迟一帧再判断,
    // 避免误恢复抢走弹窗内的首焦点 (§17.x.3 弹窗焦点策略).
    WidgetsBinding.instance.addPostFrameCallback((_) => recoverIfLost());
  }

  /// [V7 §17.x.2/§17.x.5] primaryFocus 为空则恢复到栈顶 (或 root).
  @visibleForTesting
  void recoverIfLost() {
    if (FocusManager.instance.primaryFocus == null) {
      restoreFocus();
    }
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
