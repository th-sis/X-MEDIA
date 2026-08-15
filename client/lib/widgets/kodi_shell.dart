import 'dart:async';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../services/app_state.dart';
import '../theme/app_theme.dart';
import 'focus.dart';
import '../pages/home_page.dart';
import '../pages/category_page.dart';
import '../pages/search_page.dart';
import '../pages/history_page.dart';
import '../pages/subscriptions_page.dart';
import '../pages/settings_page.dart';

class _NavItem {
  final String label;
  final IconData icon;
  const _NavItem(this.label, this.icon);
}

const _navItems = <_NavItem>[
  _NavItem('首页', Icons.home_rounded),
  _NavItem('电影', Icons.movie_rounded),
  _NavItem('剧集', Icons.live_tv_rounded),
  _NavItem('综艺', Icons.mic_rounded),
  _NavItem('动漫', Icons.animation_rounded),
  _NavItem('纪录', Icons.public_rounded),
  _NavItem('搜索', Icons.search_rounded),
  _NavItem('历史', Icons.history_rounded),
  _NavItem('订阅', Icons.notifications_active_rounded),
  _NavItem('设置', Icons.settings_rounded),
];

class KodiShell extends StatefulWidget {
  const KodiShell({super.key});

  @override
  State<KodiShell> createState() => _KodiShellState();
}

class _KodiShellState extends State<KodiShell> {
  int _index = 0;
  Timer? _clockTimer;
  DateTime _now = DateTime.now();

  @override
  void initState() {
    super.initState();
    _clockTimer = Timer.periodic(const Duration(seconds: 30), (_) {
      if (mounted) setState(() => _now = DateTime.now());
    });
  }

  @override
  void dispose() {
    _clockTimer?.cancel();
    super.dispose();
  }

  Widget _buildPage(int i) {
    switch (i) {
      case 0: return const HomePage();
      case 1: return const CategoryPage(type: 'movie', title: '电影');
      case 2: return const CategoryPage(type: 'tv', title: '剧集');
      case 3: return const CategoryPage(type: 'variety', title: '综艺');
      case 4: return const CategoryPage(type: 'anime', title: '动漫');
      case 5: return const CategoryPage(type: 'documentary', title: '纪录');
      case 6: return const SearchPage();
      case 7: return const HistoryPage();
      case 8: return const SubscriptionsPage();
      case 9: return const SettingsPage();
      default: return const HomePage();
    }
  }

  @override
  Widget build(BuildContext context) {
    return FocusTraversalGroup(
      policy: KodiTraversalPolicy(),
      child: Stack(
        children: [
          const _Background(),
          Row(
            children: [
              _Sidebar(
                selected: _index,
                onSelect: (i) => setState(() => _index = i),
              ),
              const SizedBox(width: 1, height: double.infinity, child: ColoredBox(color: Colors.white12)),
              Expanded(
                child: ClipRect(
                  child: KeyedSubtree(
                    key: ValueKey(_index),
                    child: _buildPage(_index),
                  ),
                ),
              ),
            ],
          ),
          // 顶部时钟 + 连接状态（Kodi 风格右上角）
          Positioned(
            top: 18,
            right: 28,
            child: _Clock(now: _now),
          ),
        ],
      ),
    );
  }
}

class _Background extends StatelessWidget {
  const _Background();

  @override
  Widget build(BuildContext context) {
    return Positioned.fill(
      child: DecoratedBox(
        decoration: BoxDecoration(
          gradient: LinearGradient(
            begin: Alignment.topLeft,
            end: Alignment.bottomRight,
            colors: [
              const Color(0xFF12151C),
              const Color(0xFF141414),
              const Color(0xFF0E1B24),
            ],
          ),
        ),
        child: DecoratedBox(
          decoration: BoxDecoration(
            gradient: RadialGradient(
              center: const Alignment(0.9, -0.9),
              radius: 1.4,
              colors: [AppColors.accent.withValues(alpha: 0.06), Colors.transparent],
            ),
          ),
        ),
      ),
    );
  }
}

class _Sidebar extends StatelessWidget {
  final int selected;
  final ValueChanged<int> onSelect;
  const _Sidebar({required this.selected, required this.onSelect});

  @override
  Widget build(BuildContext context) {
    return Container(
      width: 210,
      color: AppColors.sidebar.withValues(alpha: 0.92),
      child: Column(
        children: [
          // Logo
          const SizedBox(height: 24),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 20),
            child: Row(
              children: [
                Container(
                  width: 34, height: 34,
                  decoration: BoxDecoration(
                    gradient: const LinearGradient(colors: [AppColors.accent, AppColors.info]),
                    borderRadius: BorderRadius.circular(10),
                  ),
                  child: const Icon(Icons.play_arrow_rounded, color: Colors.black, size: 24),
                ),
                const SizedBox(width: 10),
                const Text('X-MEDIA',
                    style: TextStyle(fontSize: 20, fontWeight: FontWeight.w800, color: AppColors.textPrimary, letterSpacing: 1)),
              ],
            ),
          ),
          const SizedBox(height: 20),
          // 导航项
          Expanded(
            child: FocusTraversalGroup(
              policy: KodiTraversalPolicy(),
              child: ListView.builder(
                padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
                itemCount: _navItems.length,
                itemBuilder: (context, i) => _SidebarItem(
                  item: _navItems[i],
                  selected: selected == i,
                  autofocus: i == 0,
                  onActivate: () => onSelect(i),
                ),
              ),
            ),
          ),
          // 底部连接状态
          Padding(
            padding: const EdgeInsets.all(16),
            child: _ConnectionDot(),
          ),
        ],
      ),
    );
  }
}

class _SidebarItem extends StatelessWidget {
  final _NavItem item;
  final bool selected;
  final bool autofocus;
  final VoidCallback onActivate;

  const _SidebarItem({required this.item, required this.selected, this.autofocus = false, required this.onActivate});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 4),
      child: KodiFocus(
        autofocus: autofocus,
        onActivate: onActivate,
        debugLabel: 'nav:${item.label}',
        builder: (context, focused) {
          final active = focused || selected;
          return AnimatedContainer(
            duration: AppDuration.normal,
            height: 46,
            padding: const EdgeInsets.symmetric(horizontal: 14),
            decoration: BoxDecoration(
              color: focused ? AppColors.accent.withValues(alpha: 0.14) : (selected ? Colors.white.withValues(alpha: 0.06) : Colors.transparent),
              borderRadius: BorderRadius.circular(AppRadius.md),
              border: focused ? Border.all(color: AppColors.accent.withValues(alpha: 0.6)) : null,
            ),
            child: Row(
              children: [
                Icon(item.icon, size: 22, color: active ? AppColors.accent : AppColors.textSecondary),
                const SizedBox(width: 14),
                Text(item.label,
                    style: TextStyle(
                      fontSize: 15,
                      color: active ? AppColors.textPrimary : AppColors.textSecondary,
                      fontWeight: active ? FontWeight.w600 : FontWeight.w400,
                    )),
                if (selected) ...[
                  const Spacer(),
                  Container(width: 4, height: 4, decoration: const BoxDecoration(color: AppColors.accent, shape: BoxShape.circle)),
                ],
              ],
            ),
          );
        },
      ),
    );
  }
}

class _ConnectionDot extends StatelessWidget {
  @override
  Widget build(BuildContext context) {
    final app = context.watch<AppState>();
    final online = app.wsConnected;
    final color = online ? AppColors.success : AppColors.warning;
    return Row(
      children: [
        Container(width: 8, height: 8, decoration: BoxDecoration(color: color, shape: BoxShape.circle)),
        const SizedBox(width: 8),
        Text(online ? '已连接' : '连接中...',
            style: const TextStyle(fontSize: 12, color: AppColors.textMuted)),
        const Spacer(),
        Text('v${app.capabilities.serverVersion.isEmpty ? '7.0' : app.capabilities.serverVersion}',
            style: const TextStyle(fontSize: 11, color: AppColors.textMuted)),
      ],
    );
  }
}

class _Clock extends StatelessWidget {
  final DateTime now;
  const _Clock({required this.now});

  @override
  Widget build(BuildContext context) {
    String two(int n) => n.toString().padLeft(2, '0');
    final time = '${two(now.hour)}:${two(now.minute)}';
    return Text(time, style: const TextStyle(fontSize: 34, fontWeight: FontWeight.w200, color: Colors.white70));
  }
}
