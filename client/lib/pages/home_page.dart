import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../models/media.dart';
import '../services/app_state.dart';
import '../services/availability.dart';
import '../theme/app_theme.dart';
import '../widgets/focus.dart';
import '../widgets/poster_card.dart';
import '../widgets/resolve_modal.dart';
import 'detail_page.dart';
import 'player_page.dart';

class HomePage extends StatefulWidget {
  const HomePage({super.key});

  @override
  State<HomePage> createState() => _HomePageState();
}

class _HomePageState extends State<HomePage> {
  /// 当前 Hero 焦点海报（用于 Hero 区联动展示）。
  /// 默认取第一个榜单第一项。
  MediaSummary? _heroItem;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      // 初始化默认 Hero 海报
      final s = context.read<AppState>().sections;
      if (s.isNotEmpty && s.first.items.isNotEmpty && mounted) {
        setState(() => _heroItem = s.first.items.first);
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    final app = context.watch<AppState>();

    if (app.loading && app.sections.isEmpty) {
      return const _Skeleton();
    }
    if (app.error.isNotEmpty && app.sections.isEmpty) {
      return _ErrorState(message: app.error, onRetry: () => app.refresh());
    }

    // 同步默认 Hero 项
    if (_heroItem == null && app.sections.isNotEmpty && app.sections.first.items.isNotEmpty) {
      _heroItem = app.sections.first.items.first;
    }

    // 保持 ListView scroll 永远从 0 开始（避免焦点 ensureVisible 把 Hero 顶出屏幕）
    return NotificationListener<ScrollNotification>(
      onNotification: (n) {
        if (n is ScrollStartNotification && n.metrics.pixels > 0) {
          // ignore
        }
        return false;
      },
      child: _HomeContent(heroItem: _heroItem, sections: app.sections, continueWatching: app.continueWatching, onOpen: _openDetail, onHover: (m) => setState(() => _heroItem = m)),
    );
  }

  void _openDetail(BuildContext context, int id, String source) {
    Navigator.of(context).push(MaterialPageRoute(builder: (_) => DetailPage(externalId: id, source: source)));
  }
}

class _HomeContent extends StatefulWidget {
  final MediaSummary? heroItem;
  final List<Section> sections;
  final List<ContinueWatching> continueWatching;
  final void Function(BuildContext, int, String) onOpen;
  final void Function(MediaSummary) onHover;
  const _HomeContent({
    required this.heroItem,
    required this.sections,
    required this.continueWatching,
    required this.onOpen,
    required this.onHover,
  });

  @override
  State<_HomeContent> createState() => _HomeContentState();
}

class _HomeContentState extends State<_HomeContent> {
  // [V7 §17.2 D53] 批量查可播放性后回填, PosterCard 拿这个渲染左上角 ✓ 角标.
  Set<int> _availableIds = const {};

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    _refreshAvailability();
  }

  @override
  void didUpdateWidget(covariant _HomeContent oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.sections != widget.sections) {
      _refreshAvailability();
    }
  }

  Future<void> _refreshAvailability() async {
    final api = context.read<AppState>().api;
    final ids = <int>[];
    for (final s in widget.sections) {
      for (final m in s.items) {
        ids.add(m.externalId);
      }
    }
    if (ids.isEmpty) return;
    try {
      final keys = ids
          .map((id) => availabilityKeyForSummary(externalId: id, externalSource: 'tmdb'))
          .toList();
      final available = await api.checkAvailability(keys);
      if (mounted) {
        setState(() {
          _availableIds = available.map((k) => k.externalId).toSet();
        });
      }
    } catch (_) {
      // 静默: 不阻塞首页展示
    }
  }

  @override
  Widget build(BuildContext context) {
    return ListView(
      key: const PageStorageKey('home'),
      padding: const EdgeInsets.only(bottom: 32),
      children: [
        if (widget.heroItem != null) _HeroSection(item: widget.heroItem!),
        if (widget.continueWatching.isNotEmpty) ...[
          const _SectionHeader(title: '继续观看', enTitle: 'CONTINUE WATCHING', icon: Icons.play_circle_outline_rounded),
          SizedBox(
            height: 142,
            child: FocusTraversalGroup(
              policy: KodiTraversalPolicy(),
              child: ListView.builder(
                scrollDirection: Axis.horizontal,
                padding: const EdgeInsets.symmetric(horizontal: AppSpacing.xl),
                itemCount: widget.continueWatching.length,
                itemBuilder: (context, i) {
                  final cw = widget.continueWatching[i];
                  return Padding(
                    padding: const EdgeInsets.only(right: AppSpacing.md),
                    child: LandscapeCard(
                      title: cw.title,
                      subtitle: cw.episodeLabel,
                      progress: cw.progress,
                      posterUrl: cw.posterUrl,
                      seed: cw.externalId,
                      onTap: () => widget.onOpen(context, cw.externalId, cw.externalSource),
                    ),
                  );
                },
              ),
            ),
          ),
          const SizedBox(height: 8),
        ],
        ...widget.sections.take(3).map((s) => _SectionShelf(
              section: s,
              autofocusFirst: widget.sections.indexOf(s) == 0,
              availableIds: _availableIds,
              onTap: (m) => widget.onOpen(context, m.externalId, m.externalSource),
              onHover: widget.onHover,
            )),
      ],
    );
  }
}


class _HeroSection extends StatelessWidget {
  final MediaSummary item;
  const _HeroSection({required this.item});

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.fromLTRB(AppSpacing.xl, 12, AppSpacing.xl, 12),
      height: 360,
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(AppRadius.xl),
        boxShadow: [
          BoxShadow(color: Colors.black.withValues(alpha: 0.4), blurRadius: 20, offset: const Offset(0, 6)),
        ],
      ),
      child: ClipRRect(
        borderRadius: BorderRadius.circular(AppRadius.xl),
        child: Stack(
        fit: StackFit.expand,
        children: [
          // 背景图（backdrop / poster / 渐变占位）
          _heroBackdrop(),
          // 暗色渐变
          Positioned.fill(
            child: DecoratedBox(
              decoration: BoxDecoration(
                gradient: LinearGradient(
                  begin: Alignment.topCenter,
                  end: Alignment.bottomCenter,
                  colors: [
                    Colors.transparent,
                    Colors.black.withValues(alpha: 0.55),
                    Colors.black.withValues(alpha: 0.9),
                  ],
                  stops: const [0.35, 0.75, 1.0],
                ),
              ),
            ),
          ),
          Positioned.fill(
            child: DecoratedBox(
              decoration: BoxDecoration(
                gradient: LinearGradient(
                  begin: Alignment.centerLeft,
                  end: Alignment.centerRight,
                  colors: [
                    Colors.black.withValues(alpha: 0.7),
                    Colors.transparent,
                  ],
                  stops: const [0.0, 0.55],
                ),
              ),
            ),
          ),
          // 文本层
          Positioned(
            left: 32, right: 32, bottom: 24,
            child: _HeroContent(item: item),
          ),
          // 右上角"特色推荐"标签
          Positioned(
            top: 18, left: 24,
            child: Container(
              padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
              decoration: BoxDecoration(
                color: AppColors.accent,
                borderRadius: BorderRadius.circular(4),
              ),
              child: const Text('精选推荐', style: TextStyle(fontSize: 11, color: Colors.black, fontWeight: FontWeight.w700, letterSpacing: 0.5)),
            ),
          ),
        ],
      ),
      ),
    );
  }

  Widget _heroBackdrop() {
    if (item.backdropUrl.isNotEmpty) {
      return Image.network(item.backdropUrl, fit: BoxFit.cover, errorBuilder: (_, __, ___) => _heroPlaceholder());
    }
    if (item.posterUrl.isNotEmpty) {
      return Image.network(item.posterUrl, fit: BoxFit.cover, errorBuilder: (_, __, ___) => _heroPlaceholder());
    }
    return _heroPlaceholder();
  }

  Widget _heroPlaceholder() {
    final grad = posterGradient(item.externalId);
    return DecoratedBox(
      decoration: BoxDecoration(
        gradient: LinearGradient(
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
          colors: grad,
        ),
      ),
    );
  }
}

class _HeroContent extends StatelessWidget {
  final MediaSummary item;
  const _HeroContent({required this.item});

  String _meta() {
    final parts = <String>[];
    if (item.year > 0) parts.add('${item.year}');
    if (item.voteAvg > 0) parts.add('★ ${item.voteAvg.toStringAsFixed(1)}');
    if (item.genres.isNotEmpty) parts.add(item.genres.take(2).join(' · '));
    return parts.join('   ');
  }

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        Text(_meta(), style: AppTypography.small.copyWith(fontWeight: FontWeight.w600, letterSpacing: 0.4)),
        const SizedBox(height: 8),
        Text(item.title,
            maxLines: 1, overflow: TextOverflow.ellipsis,
            style: AppTypography.display),
        if (item.titleOrig.isNotEmpty && item.titleOrig != item.title)
          Padding(
            padding: const EdgeInsets.only(top: 2),
            child: Text(item.titleOrig,
                maxLines: 1, overflow: TextOverflow.ellipsis,
                style: AppTypography.small.copyWith(color: AppColors.textSecondary, fontWeight: FontWeight.w500)),
          ),
        const SizedBox(height: 10),
        if (item.overview.isNotEmpty)
          ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 720),
            child: Text(
              item.overview,
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
              style: AppTypography.small.copyWith(color: Colors.white70, height: 1.4),
            ),
          ),
        const SizedBox(height: 14),
        _HeroActions(item: item),
      ],
    );
  }
}

class _HeroActions extends StatelessWidget {
  final MediaSummary item;
  const _HeroActions({required this.item});

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        KodiFocus(
          autofocus: true,
          onActivate: () => _play(context),
          debugLabel: 'hero:play',
          builder: (context, focused) => AnimatedContainer(
            duration: AppDuration.normal,
            padding: const EdgeInsets.symmetric(horizontal: 22, vertical: 10),
            decoration: BoxDecoration(
              color: focused ? Colors.white : AppColors.accent,
              borderRadius: BorderRadius.circular(AppRadius.md),
              boxShadow: focused
                  ? [BoxShadow(color: Colors.white.withValues(alpha: 0.4), blurRadius: 12)]
                  : [BoxShadow(color: AppColors.accent.withValues(alpha: 0.4), blurRadius: 10)],
            ),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: const [
                Icon(Icons.play_arrow_rounded, color: Colors.black, size: 22),
                SizedBox(width: 6),
                Text('播放', style: TextStyle(color: Colors.black, fontSize: 15, fontWeight: FontWeight.w700)),
              ],
            ),
          ),
        ),
        const SizedBox(width: 10),
        KodiFocus(
          onActivate: () => _toggleFavorite(context),
          debugLabel: 'hero:fav',
          builder: (context, focused) => AnimatedContainer(
            duration: AppDuration.normal,
            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
            decoration: BoxDecoration(
              color: Colors.white.withValues(alpha: focused ? 0.18 : 0.10),
              borderRadius: BorderRadius.circular(AppRadius.md),
              border: Border.all(color: focused ? Colors.white : Colors.white24),
            ),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: const [
                Icon(Icons.favorite_border_rounded, color: Colors.white, size: 20),
                SizedBox(width: 6),
                Text('收藏', style: TextStyle(color: Colors.white, fontSize: 14, fontWeight: FontWeight.w600)),
              ],
            ),
          ),
        ),
        const SizedBox(width: 10),
        KodiFocus(
          onActivate: () => _subscribe(context),
          debugLabel: 'hero:sub',
          builder: (context, focused) => AnimatedContainer(
            duration: AppDuration.normal,
            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
            decoration: BoxDecoration(
              color: Colors.white.withValues(alpha: focused ? 0.18 : 0.10),
              borderRadius: BorderRadius.circular(AppRadius.md),
              border: Border.all(color: focused ? Colors.white : Colors.white24),
            ),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: const [
                Icon(Icons.notifications_active_outlined, color: Colors.white, size: 20),
                SizedBox(width: 6),
                Text('订阅', style: TextStyle(color: Colors.white, fontSize: 14, fontWeight: FontWeight.w600)),
              ],
            ),
          ),
        ),
        const Spacer(),
        KodiFocus(
          onActivate: () => _openDetail(context),
          debugLabel: 'hero:detail',
          builder: (context, focused) => AnimatedContainer(
            duration: AppDuration.normal,
            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
            decoration: BoxDecoration(
              color: Colors.transparent,
              borderRadius: BorderRadius.circular(AppRadius.md),
              border: Border.all(color: focused ? Colors.white : Colors.white24),
            ),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: const [
                Text('查看详情', style: TextStyle(color: Colors.white, fontSize: 14, fontWeight: FontWeight.w600)),
                SizedBox(width: 4),
                Icon(Icons.chevron_right_rounded, color: Colors.white, size: 20),
              ],
            ),
          ),
        ),
      ],
    );
  }

  void _openDetail(BuildContext context) {
    Navigator.of(context).push(MaterialPageRoute(
      builder: (_) => DetailPage(externalId: item.externalId, source: item.externalSource, seed: item),
    ));
  }

  Future<void> _play(BuildContext context) async {
    final api = context.read<AppState>().api;
    final app = context.read<AppState>();
    await showDialog(
      context: context,
      barrierDismissible: false,
      builder: (_) => ResolveModal(
        app: app,
        api: api,
        externalId: item.externalId,
        source: item.externalSource,
        mediaType: item.mediaType.isEmpty ? 'movie' : item.mediaType,
        title: item.title,
        year: item.year,
        season: 0,
        episode: 0,
        onReady: (streamUrl, source) {
          Navigator.of(context).push(MaterialPageRoute(
            builder: (_) => PlayerPage(
              streamUrl: streamUrl,
              title: item.title,
              externalId: item.externalId,
              source: item.externalSource,
              mediaType: item.mediaType.isEmpty ? 'movie' : item.mediaType,
              posterUrl: item.posterUrl,
              season: 0,
              episode: 0,
              playSource: source,
            ),
          ));
        },
      ),
    );
  }

  Future<void> _toggleFavorite(BuildContext context) async {
    final api = context.read<AppState>().api;
    try {
      await api.addFavorite(item.externalId, item.title, item.year,
          source: item.externalSource, mediaType: item.mediaType, posterUrl: item.posterUrl);
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('已加入收藏')));
      }
    } catch (e) {
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('收藏失败：$e')));
      }
    }
  }

  Future<void> _subscribe(BuildContext context) async {
    final api = context.read<AppState>().api;
    try {
      await api.addSubscription(item.externalId, item.title, item.year,
          source: item.externalSource, mediaType: item.mediaType, posterUrl: item.posterUrl);
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('已添加订阅')));
      }
    } catch (e) {
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('订阅失败：$e')));
      }
    }
  }
}

// ============== 通用 Section / Skeleton / ErrorState ==============

class _SectionHeader extends StatelessWidget {
  final String title;
  final String? enTitle;
  final IconData? icon;
  const _SectionHeader({required this.title, this.enTitle, this.icon});
  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(AppSpacing.xl, 16, AppSpacing.xl, 10),
      child: Row(
        children: [
          if (icon != null) ...[
            Icon(icon, size: 18, color: AppColors.accent),
            const SizedBox(width: 8),
          ],
          Text(title, style: AppTypography.title),
          if (enTitle != null) ...[
            const SizedBox(width: 10),
            Text(enTitle!, style: AppTypography.caption.copyWith(letterSpacing: 1.5)),
          ],
        ],
      ),
    );
  }
}

class _SectionShelf extends StatelessWidget {
  final Section section;
  final bool autofocusFirst;
  final void Function(MediaSummary) onTap;
  final void Function(MediaSummary) onHover;
  final Set<int> availableIds; // [V7 §17.2 D53]
  const _SectionShelf({
    required this.section,
    required this.onTap,
    required this.onHover,
    this.autofocusFirst = false,
    this.availableIds = const {},
  });

  @override
  Widget build(BuildContext context) {
    if (section.items.isEmpty) return const SizedBox.shrink();
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _SectionHeader(title: section.title),
        SizedBox(
          height: 235,
          child: FocusTraversalGroup(
            policy: KodiTraversalPolicy(),
            child: ListView.builder(
              scrollDirection: Axis.horizontal,
              padding: const EdgeInsets.symmetric(horizontal: AppSpacing.xl),
              itemCount: section.items.length,
              itemBuilder: (context, i) {
                final m = section.items[i];
                return Padding(
                  padding: const EdgeInsets.only(right: AppSpacing.md),
                  child: PosterCard(
                    media: m,
                    autofocus: autofocusFirst && i == 0,
                    available: availableIds.contains(m.externalId),
                    onTap: () => onTap(m),
                    onFocusChange: (f) {
                      if (f) onHover(m);
                    },
                  ),
                );
              },
            ),
          ),
        ),
      ],
    );
  }
}

class _Skeleton extends StatelessWidget {
  const _Skeleton();
  @override
  Widget build(BuildContext context) {
    Widget block(double w, double h) => Container(
          width: w, height: h,
          decoration: BoxDecoration(
            color: AppColors.surfaceHigh,
            borderRadius: BorderRadius.circular(AppRadius.md),
          ),
        );
    return ListView(
      padding: const EdgeInsets.all(AppSpacing.xl),
      children: [
        block(double.infinity, 360),
        const SizedBox(height: 20),
        for (var r = 0; r < 5; r++) ...[
          block(120, 20),
          const SizedBox(height: 12),
          SizedBox(height: 225, child: Row(children: List.generate(6, (i) => Padding(padding: const EdgeInsets.only(right: 12), child: block(150, 225))))),
          const SizedBox(height: 24),
        ],
      ],
    );
  }
}

class _ErrorState extends StatelessWidget {
  final String message;
  final VoidCallback onRetry;
  const _ErrorState({required this.message, required this.onRetry});
  @override
  Widget build(BuildContext context) {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          const Icon(Icons.cloud_off_rounded, size: 56, color: AppColors.textMuted),
          const SizedBox(height: 12),
          const Text('无法连接到后端服务', style: TextStyle(fontSize: 18, color: AppColors.textPrimary)),
          const SizedBox(height: 6),
          Text(message, style: const TextStyle(fontSize: 13, color: AppColors.textSecondary)),
          const SizedBox(height: 16),
          TextButton(
            onPressed: onRetry,
            child: const Text('重试'),
          ),
        ],
      ),
    );
  }
}
