import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:provider/provider.dart';
import 'services/app_state.dart';
import 'theme/app_theme.dart';
import 'widgets/focus.dart';
import 'widgets/kodi_shell.dart';

void main() {
  WidgetsFlutterBinding.ensureInitialized();
  runApp(const XMediaApp());
}

class XMediaApp extends StatelessWidget {
  const XMediaApp({super.key});

  @override
  Widget build(BuildContext context) {
    return ChangeNotifierProvider(
      create: (_) => AppState(),
      child: MaterialApp(
        title: 'X-MEDIA',
        debugShowCheckedModeBanner: false,
        theme: kodiTheme(),
        darkTheme: kodiTheme(),
        themeMode: ThemeMode.dark,
        builder: (context, child) {
          return KodiShortcuts(
            child: _BackHandler(child: child ?? const SizedBox()),
          );
        },
        home: const KodiShell(),
      ),
    );
  }
}

/// 全局 Esc 键回退。
class _BackHandler extends StatelessWidget {
  final Widget child;
  const _BackHandler({required this.child});

  @override
  Widget build(BuildContext context) {
    return Shortcuts(
      shortcuts: const {
        SingleActivator(LogicalKeyboardKey.escape): _BackIntent(),
      },
      child: Actions(
        actions: {
          _BackIntent: CallbackAction<_BackIntent>(onInvoke: (_) {
            final nav = Navigator.of(context);
            if (nav.canPop()) {
              nav.pop();
            }
            return null;
          }),
        },
        child: Focus(
          autofocus: true,
          canRequestFocus: true,
          child: child,
        ),
      ),
    );
  }
}

class _BackIntent extends Intent {
  const _BackIntent();
}
