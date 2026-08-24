import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:fvp/fvp.dart' as fvp;
import 'package:provider/provider.dart';
import 'services/app_state.dart';
import 'theme/app_theme.dart';
import 'widgets/focus.dart';
import 'widgets/kodi_shell.dart';

/// [V7 D2] fvp (mdk-sdk) 替代 video_player, 解决 Dolby Vision 色彩问题.
/// 在所有平台启用 fvp 实现; Desktop (windows/linux/macos) 由 fvp 接管,
/// Mobile (android/ios) 由 fvp 提供的 D3D11/Metal/OpenGL/Impeller 渲染,
/// 较 video_player 自带的 ExoPlayer/AVPlayer 色彩更准确.
void _registerVideoBackend() {
  fvp.registerWith();
}

void main() {
  WidgetsFlutterBinding.ensureInitialized();
  _registerVideoBackend();
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
