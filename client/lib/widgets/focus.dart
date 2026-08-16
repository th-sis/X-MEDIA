import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

/// 四方向焦点导航策略：复用 Flutter 的几何阅读顺序遍历（支持方向键）。
typedef KodiTraversalPolicy = ReadingOrderTraversalPolicy;

/// 焦点容器：封装 FocusNode，聚焦时通过 builder 的 `focused` 参数渲染高亮。
class KodiFocus extends StatefulWidget {
  final Widget Function(BuildContext context, bool focused) builder;
  final VoidCallback? onActivate;
  final bool autofocus;
  final String? debugLabel;
  final ValueChanged<bool>? onFocusChange;

  const KodiFocus({super.key, required this.builder, this.onActivate, this.autofocus = false, this.debugLabel, this.onFocusChange});

  @override
  State<KodiFocus> createState() => _KodiFocusState();
}

class _KodiFocusState extends State<KodiFocus> {
  late final FocusNode _node = FocusNode(debugLabel: widget.debugLabel);

  @override
  void initState() {
    super.initState();
    _node.addListener(_onFocusChange);
  }

  void _onFocusChange() {
    if (!mounted) return;
    final has = _node.hasFocus;
    widget.onFocusChange?.call(has);
    setState(() {});
    if (has) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted && _node.hasFocus) {
          Scrollable.ensureVisible(context, alignment: 0.5, duration: const Duration(milliseconds: 200));
        }
      });
    }
  }

  @override
  void dispose() {
    _node.removeListener(_onFocusChange);
    _node.dispose();
    super.dispose();
  }

  KeyEventResult _onKey(FocusNode node, KeyEvent event) {
    if (event is KeyDownEvent || event is KeyRepeatEvent) {
      final k = event.logicalKey;
      if (k == LogicalKeyboardKey.enter ||
          k == LogicalKeyboardKey.space ||
          k == LogicalKeyboardKey.select ||
          k == LogicalKeyboardKey.gameButtonA) {
        widget.onActivate?.call();
        return KeyEventResult.handled;
      }
    }
    return KeyEventResult.ignored;
  }

  @override
  Widget build(BuildContext context) {
    return Focus(
      focusNode: _node,
      autofocus: widget.autofocus,
      onKeyEvent: _onKey,
      child: GestureDetector(
        behavior: HitTestBehavior.opaque,
        onTap: widget.onActivate,
        child: widget.builder(context, _node.hasFocus),
      ),
    );
  }
}

/// 为整个 App 注册方向键 → 焦点移动的快捷键映射。
class KodiShortcuts extends StatelessWidget {
  final Widget child;
  const KodiShortcuts({super.key, required this.child});

  @override
  Widget build(BuildContext context) {
    return Shortcuts(
      shortcuts: const <ShortcutActivator, Intent>{
        SingleActivator(LogicalKeyboardKey.arrowLeft): DirectionalFocusIntent(TraversalDirection.left),
        SingleActivator(LogicalKeyboardKey.arrowRight): DirectionalFocusIntent(TraversalDirection.right),
        SingleActivator(LogicalKeyboardKey.arrowUp): DirectionalFocusIntent(TraversalDirection.up),
        SingleActivator(LogicalKeyboardKey.arrowDown): DirectionalFocusIntent(TraversalDirection.down),
      },
      child: Actions(
        actions: <Type, Action<Intent>>{
          DirectionalFocusIntent: DirectionalFocusAction(),
        },
        child: child,
      ),
    );
  }
}
