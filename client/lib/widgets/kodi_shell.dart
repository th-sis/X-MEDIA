import 'dart:async';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../pages/home_page.dart';
import '../pages/category_page.dart';
import '../pages/search_page.dart';
import '../pages/history_page.dart';
import '../pages/favorites_page.dart';
import '../pages/subscriptions_page.dart';
import '../pages/settings_page.dart';
import '../theme/app_theme.dart';
import 'focus.dart';
import '../services/app_state.dart';

/// 顶部分类条目（5 个，固定）。点击导航到对应 CategoryPage。
class _CategoryTab {
  final String english;
  final String chinese;
  final String type; // API mediaType
  const _CategoryTab(this.english, this.chinese, this.type);
}

const _kCategoryTabs = <_CategoryTab>[
  _CategoryTab('FILMS', '电影', 'movie'),
  _CategoryTab('TV', '电视', 'tv'),
  _CategoryTab('VARIETY', '综艺', 'variety'),
  _CategoryTab('ANIME', '动漫', 'anime'),
  _CategoryTab('DOCS', '纪录', 'documentary'),
];

/// 底部主导航条目（6 个，页面级）。
class _PageNav {
  final String english;
  final String chinese;
  final IconData icon;
  final WidgetBuilder builder;
  const _PageNav(this.english, this.chinese, this.icon, this.builder);
}

final _kPageNavs = <_PageNav>[
  _PageNav('EXPLORE', '探索', Icons.explore_rounded, (_) => const HomePage()),
  _PageNav('SEARCH', '搜索', Icons.search_rounded, (_) => const SearchPage()),
  _PageNav('HISTORY', '历史', Icons.history_rounded, (_) => const HistoryPage()),
  _PageNav('FAVORITES', '收藏', Icons.favorite_rounded, (_) => const FavoritesPage()),
  _PageNav('SUBS', '订阅', Icons.notifications_active_rounded, (_) => const SubscriptionsPage()),
  _PageNav('SETTINGS', '设置', Icons.settings_rounded, (_) => const SettingsPage()),
];

class KodiShell extends StatefulWidget {
  const KodiShell({super.key});

  @override
  State<KodiShell> createState() => _KodiShellState();
}

class _KodiShellState extends State<KodiShell> {
  /// 当前底部页面索引（-1 表示正在顶部分类页，不属于任何底部条目）
  int _pageIndex = 0;
  /// 当前顶部分类索引（-1 表示不在分类页）
  int _categoryIndex = -1;
  /// 自定义子页面栈（顶部分类点击会 push 到这里；返回时回到主页面栈）
  final List<Widget> _stack = [];

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

  // ===== 导航动作 =====

  void _selectCategory(int i) {
    final tab = _kCategoryTabs[i];
    final page = CategoryPage(type: tab.type, title: tab.chinese);
    setState(() {
      _categoryIndex = i;
      _pageIndex = -1; // 离开底部栈
      _stack.add(page);
    });
  }

  void _selectPage(int i) {
    setState(() {
      _pageIndex = i;
      _categoryIndex = -1; // 离开顶部分类
      _stack.clear();
    });
  }

  void _onBack() {
    if (_stack.isNotEmpty) {
      setState(() {
        _stack.removeLast();
        if (_stack.isEmpty) {
          _categoryIndex = -1;
          _pageIndex = 0; // 回到默认首页
        }
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    // 当前 body：栈非空就显示栈顶；否则按底部 page 渲染
    final Widget body;
    if (_stack.isNotEmpty) {
      body = _stack.last;
    } else {
      body = _kPageNavs[_pageIndex.clamp(0, _kPageNavs.length - 1)].builder(context);
    }

    // [P2#8] 监听 server_stopping: 复用 _ServerStoppingBanner 在 build 外做副作用,
    // 避免 build 期间 addPostFrameCallback 反复注册导致 SnackBar 重复弹.
    return _ServerStoppingBanner(
      child: PopScope(
        canPop: false,
        onPopInvokedWithResult: (didPop, _) {
          if (didPop) return;
          _onBack();
        },
        child: Scaffold(
          backgroundColor: AppColors.background,
          body: Column(
            children: [
              _TopBar(
                now: _now,
                categoryIndex: _categoryIndex,
                pageIndex: _pageIndex,
                onCategoryTap: _selectCategory,
              ),
              Expanded(
                child: Stack(
                  children: [
                    const Positioned.fill(child: _Background()),
                    Positioned.fill(child: KeyedSubtree(key: ValueKey(_currentKey()), child: body)),
                  ],
                ),
              ),
              _BottomBar(
                pageIndex: _pageIndex,
                onSelect: _selectPage,
              ),
            ],
          ),
        ),
      ),
    );
  }

  String _currentKey() {
    if (_stack.isNotEmpty) return 'stack:${_stack.length}';
    return 'page:$_pageIndex';
  }
}

// ============== 背景 ==============

class _Background extends StatelessWidget {
  const _Background();
  @override
  Widget build(BuildContext context) {
    return DecoratedBox(
      decoration: const BoxDecoration(
        gradient: LinearGradient(
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
          colors: [Color(0xFF0B0F1A), Color(0xFF0E1320), Color(0xFF0A0D14)],
        ),
      ),
      child: DecoratedBox(
        decoration: BoxDecoration(
          gradient: RadialGradient(
            center: const Alignment(0.2, -0.4),
            radius: 1.3,
            colors: [AppColors.accent.withValues(alpha: 0.08), Colors.transparent],
          ),
        ),
        child: DecoratedBox(
          decoration: BoxDecoration(
            gradient: RadialGradient(
              center: const Alignment(1.2, 1.4),
              radius: 1.0,
              colors: [AppColors.accentGlow.withValues(alpha: 0.10), Colors.transparent],
            ),
          ),
        ),
      ),
    );
  }
}

// ============== 顶部栏：Logo + 5 分类 + 时钟 + 连接徽章 ==============

class _TopBar extends StatelessWidget {
  final DateTime now;
  final int categoryIndex;
  final int pageIndex;
  final void Function(int) onCategoryTap;

  const _TopBar({
    required this.now,
    required this.categoryIndex,
    required this.pageIndex,
    required this.onCategoryTap,
  });

  @override
  Widget build(BuildContext context) {
    final app = context.watch<AppState>();
    final badges = sourceBadgesFromCapabilities(
      nasAvailable: app.capabilities.nasAvailable,
      pansearchAvailable: app.capabilities.pansearchAvailable,
      loggedInDrivers: app.capabilities.loggedInDrivers,
    );

    return Container(
      height: 80,
      padding: const EdgeInsets.symmetric(horizontal: 20),
      decoration: BoxDecoration(
        color: AppColors.background,
        border: const Border(bottom: BorderSide(color: Colors.white10, width: 1)),
      ),
      child: Row(
        children: [
          // Logo + 名称
          const _LogoMark(),
          const SizedBox(width: 12),
          const Text('X-MEDIA',
              style: TextStyle(fontSize: 18, fontWeight: FontWeight.w800, color: AppColors.textPrimary, letterSpacing: 1.2)),
          const SizedBox(width: 18),
          // 5 个顶部分类 Tab
          Expanded(
            child: FocusTraversalGroup(
              policy: KodiTraversalPolicy(),
              child: Row(
                mainAxisAlignment: MainAxisAlignment.start,
                children: List.generate(_kCategoryTabs.length, (i) {
                  return Padding(
                    padding: const EdgeInsets.only(right: 28),
                    child: _CategoryTabButton(
                      tab: _kCategoryTabs[i],
                      active: categoryIndex == i,
                      onActivate: () => onCategoryTap(i),
                    ),
                  );
                }),
              ),
            ),
          ),
          // 右侧：连接徽章 + 时钟
          FittedBox(
            fit: BoxFit.scaleDown,
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                _SourceBadges(badges: badges),
                const SizedBox(width: 12),
                _ClockBlock(now: now),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _LogoMark extends StatelessWidget {
  const _LogoMark();
  @override
  Widget build(BuildContext context) {
    return Container(
      width: 36,
      height: 36,
      decoration: BoxDecoration(
        gradient: const LinearGradient(
          colors: [AppColors.accent, Color(0xFF8B5CF6)],
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
        ),
        borderRadius: BorderRadius.circular(10),
        boxShadow: [
          BoxShadow(color: AppColors.accent.withValues(alpha: 0.4), blurRadius: 12, spreadRadius: 0.5),
        ],
      ),
      child: const Icon(Icons.play_arrow_rounded, color: Colors.black, size: 26),
    );
  }
}

/// 单个顶部分类按钮：默认英文大写，聚焦/选中切中文，透明大字号。
class _CategoryTabButton extends StatelessWidget {
  final _CategoryTab tab;
  final bool active;
  final VoidCallback onActivate;
  const _CategoryTabButton({required this.tab, required this.active, required this.onActivate});

  @override
  Widget build(BuildContext context) {
    return KodiFocus(
      onActivate: onActivate,
      debugLabel: 'top:${tab.english}',
      builder: (context, focused) {
        final emphasized = focused || active;
        return AnimatedContainer(
          duration: AppDuration.normal,
          curve: Curves.easeOut,
          padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 8),
          decoration: BoxDecoration(
            color: Colors.transparent,
            border: Border(
              bottom: BorderSide(
                color: active ? AppColors.selection : Colors.transparent,
                width: 2.5,
              ),
            ),
          ),
          child: FittedBox(
            fit: BoxFit.scaleDown,
            child: AnimatedDefaultTextStyle(
              duration: AppDuration.normal,
              style: emphasized
                  ? (active ? AppTypography.navChineseActive : AppTypography.navChinese)
                  : AppTypography.navEnglish,
              child: Text(emphasized ? tab.chinese : tab.english.toUpperCase()),
            ),
          ),
        );
      },
    );
  }
}

/// 数据源连接徽章（NAS / PANSOU / 网盘 / LOCAL）。
class _SourceBadges extends StatelessWidget {
  final List<SourceBadge> badges;
  const _SourceBadges({required this.badges});

  @override
  Widget build(BuildContext context) {
    if (badges.isEmpty) return const SizedBox.shrink();
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: badges.map((b) => Container(
            padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
            margin: const EdgeInsets.only(right: 6),
            decoration: BoxDecoration(
              color: b.color.withValues(alpha: 0.15),
              border: Border.all(color: b.color.withValues(alpha: 0.55), width: 1),
              borderRadius: BorderRadius.circular(999),
            ),
            child: Text(
              b.label,
              style: TextStyle(
                fontSize: 10,
                fontWeight: FontWeight.w700,
                color: b.color,
                letterSpacing: 0.4,
              ),
            ),
          )).toList(),
    );
  }
}

class _ClockBlock extends StatelessWidget {
  final DateTime now;
  const _ClockBlock({required this.now});

  @override
  Widget build(BuildContext context) {
    final hh = now.hour.toString().padLeft(2, '0');
    final mm = now.minute.toString().padLeft(2, '0');
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Text('$hh:$mm',
            style: const TextStyle(fontSize: 24, fontWeight: FontWeight.w300, color: Colors.white, letterSpacing: 1.5)),
      ],
    );
  }
}

// ============== 底栏：6 个页面级导航 ==============

class _BottomBar extends StatelessWidget {
  final int pageIndex;
  final ValueChanged<int> onSelect;
  const _BottomBar({required this.pageIndex, required this.onSelect});

  @override
  Widget build(BuildContext context) {
    return Container(
      height: 90,
      decoration: const BoxDecoration(
        color: AppColors.background,
        border: Border(top: BorderSide(color: Colors.white10, width: 1)),
      ),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 28, vertical: 6),
        child: FocusTraversalGroup(
          policy: KodiTraversalPolicy(),
          child: Row(
            mainAxisAlignment: MainAxisAlignment.spaceEvenly,
            children: List.generate(_kPageNavs.length, (i) {
              return Expanded(
                child: _PageNavButton(
                  nav: _kPageNavs[i],
                  active: pageIndex == i,
                  onActivate: () => onSelect(i),
                ),
              );
            }),
          ),
        ),
      ),
    );
  }
}

/// 单个底部导航按钮：默认大写英文，聚焦切中文，激活态中文 + 强调下划线。
class _PageNavButton extends StatelessWidget {
  final _PageNav nav;
  final bool active;
  final VoidCallback onActivate;
  const _PageNavButton({required this.nav, required this.active, required this.onActivate});

  @override
  Widget build(BuildContext context) {
    return KodiFocus(
      onActivate: onActivate,
      debugLabel: 'bottom:${nav.english}',
      builder: (context, focused) {
        final emphasized = focused || active;
        return AnimatedContainer(
          duration: AppDuration.normal,
          curve: Curves.easeOut,
          decoration: BoxDecoration(
            color: Colors.transparent,
            border: Border(
              top: BorderSide(
                color: active ? AppColors.selection : Colors.transparent,
                width: 2.5,
              ),
            ),
          ),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Icon(
                nav.icon,
                size: emphasized ? 18 : 16,
                color: emphasized
                    ? (active ? AppColors.accent : AppColors.textPrimary)
                    : AppColors.textSecondary,
              ),
              const SizedBox(height: 3),
              AnimatedDefaultTextStyle(
                duration: AppDuration.normal,
                style: emphasized
                    ? (active ? AppTypography.navChineseActive : AppTypography.navChinese)
                    : AppTypography.navEnglish,
                child: FittedBox(
                  fit: BoxFit.scaleDown,
                  child: Text(
                    emphasized ? nav.chinese : nav.english.toUpperCase(),
                    maxLines: 1,
                  ),
                ),
              ),
            ],
          ),
        );
      },
    );
  }
}

/// [P2#8] 监听 AppState.serverStoppingMessage 变化, 弹一次 SnackBar 后自动 clear.
/// 用 StatefulWidget + didChangeDependencies 替代 build 内 addPostFrameCallback,
/// 避免 build 期间反复注册回调导致 SnackBar 重复弹.
///
/// 重入守门: msg 从 "X" → null → "X" 都能再次弹, 但同一 msg 在
/// serverStoppingMessage 生命周期内只弹一次.
class _ServerStoppingBanner extends StatefulWidget {
  final Widget child;
  const _ServerStoppingBanner({required this.child});

  @override
  State<_ServerStoppingBanner> createState() => _ServerStoppingBannerState();
}

class _ServerStoppingBannerState extends State<_ServerStoppingBanner> {
  String? _lastShown;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    final msg = context.select<AppState, String?>((s) => s.serverStoppingMessage);
    if (msg == null) {
      // msg 已被 clear, 重置守门允许下次再弹
      _lastShown = null;
      return;
    }
    if (msg == _lastShown) return;
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      _lastShown = msg;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(msg),
          duration: const Duration(seconds: 8),
          backgroundColor: AppColors.warning,
        ),
      );
      context.read<AppState>().clearServerStopping();
    });
  }

  @override
  Widget build(BuildContext context) => widget.child;
}
