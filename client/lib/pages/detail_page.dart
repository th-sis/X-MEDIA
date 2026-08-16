import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../models/media.dart';
import '../services/app_state.dart';
import '../theme/app_theme.dart';
import '../widgets/resolve_modal.dart';
import '../widgets/skeleton.dart';
import 'player_page.dart';

class DetailPage extends StatefulWidget {
  final int externalId;
  final String source;
  final MediaSummary? seed;
  const DetailPage({super.key, required this.externalId, this.source = 'tmdb', this.seed});

  @override
  State<DetailPage> createState() => _DetailPageState();
}

class _DetailPageState extends State<DetailPage> {
  MediaDetail? _detail;
  List<SeasonInfo> _seasons = const [];
  bool _loading = true;
  bool _favorite = false;
  bool _subscribed = false;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    final api = context.read<AppState>().api;
    setState(() {
      _loading = true;
    });
    try {
      final d = await api.detail(widget.externalId, source: widget.source);
      if (mounted) {
        setState(() => _detail = d);
        if (d.isSeries) {
          api.seasons(widget.externalId, source: widget.source).then((s) {
            if (mounted) setState(() => _seasons = s);
          }).catchError((_) {});
        }
      }
      _loadMarks(api);
    } catch (e) {
      if (mounted) setState(() => _detail = null);
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  Future<void> _loadMarks(dynamic api) async {
    try {
      final favs = await api.favorites();
      final subs = await api.subscriptions();
      if (mounted) {
        setState(() {
          _favorite = favs.any((f) => f.externalId == widget.externalId);
          _subscribed = subs.any((s) => s.externalId == widget.externalId);
        });
      }
    } catch (_) {}
  }

  Future<void> _toggleFavorite() async {
    final api = context.read<AppState>().api;
    final d = _detail;
    if (d == null) return;
    try {
      if (_favorite) {
        await api.removeFavorite(widget.externalId, source: widget.source);
      } else {
        await api.addFavorite(widget.externalId, d.summary.title, d.summary.year,
            source: widget.source, mediaType: d.summary.mediaType, posterUrl: d.summary.posterUrl);
      }
      if (mounted) setState(() => _favorite = !_favorite);
    } catch (_) {}
  }

  Future<void> _toggleSubscribe() async {
    final api = context.read<AppState>().api;
    final d = _detail;
    if (d == null) return;
    try {
      if (_subscribed) {
        await api.removeSubscription(widget.externalId, source: widget.source);
      } else {
        await api.addSubscription(widget.externalId, d.summary.title, d.summary.year,
            source: widget.source, mediaType: d.summary.mediaType, posterUrl: d.summary.posterUrl);
      }
      if (mounted) setState(() => _subscribed = !_subscribed);
    } catch (_) {}
  }

  Future<void> _play({int season = 0, int episode = 0}) async {
    final d = _detail;
    if (d == null) return;
    final api = context.read<AppState>().api;
    final app = context.read<AppState>();
    await showDialog(
      context: context,
      barrierDismissible: false,
      builder: (_) => ResolveModal(
        app: app,
        api: api,
        externalId: widget.externalId,
        source: widget.source,
        mediaType: d.summary.mediaType,
        title: d.summary.title,
        year: d.summary.year,
        season: season,
        episode: episode,
        onReady: (streamUrl, source) {
          Navigator.of(context).push(MaterialPageRoute(
            builder: (_) => PlayerPage(
              streamUrl: streamUrl,
              title: d.summary.title,
              externalId: widget.externalId,
              source: widget.source,
              mediaType: d.summary.mediaType,
              posterUrl: d.summary.posterUrl,
              season: season,
              episode: episode,
              playSource: source,
            ),
          ));
        },
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.transparent,
      body: _loading
          ? const DetailSkeleton()
          : _detail == null
              ? Center(child: Column(mainAxisSize: MainAxisSize.min, children: [
                  const Text('加载失败', style: TextStyle(color: AppColors.textSecondary)),
                  const SizedBox(height: 8),
                  TextButton(onPressed: _load, child: const Text('重试')),
                ]))
              : _buildContent(_detail!),
    );
  }

  Widget _buildContent(MediaDetail d) {
    final s = d.summary;
    return ListView(
      padding: EdgeInsets.zero,
      children: [
        // 头部横幅
        Container(
          height: 300,
          decoration: BoxDecoration(
            gradient: LinearGradient(
              begin: Alignment.topLeft,
              end: Alignment.bottomRight,
              colors: posterGradient(s.externalId),
            ),
          ),
          child: Stack(
            children: [
              if (s.backdropUrl.isNotEmpty)
                Positioned.fill(child: Image.network(s.backdropUrl, fit: BoxFit.cover, errorBuilder: (_, __, ___) => const SizedBox())),
              Positioned.fill(
                child: DecoratedBox(
                  decoration: BoxDecoration(
                    gradient: LinearGradient(
                      begin: Alignment.topCenter,
                      end: Alignment.bottomCenter,
                      colors: [Colors.black.withValues(alpha: 0.1), AppColors.background.withValues(alpha: 0.95)],
                      stops: const [0.3, 1.0],
                    ),
                  ),
                ),
              ),
              Positioned(
                left: AppSpacing.xl,
                right: AppSpacing.xl,
                bottom: 16,
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        const Icon(Icons.star, color: AppColors.warning, size: 18),
                        const SizedBox(width: 4),
                        Text(s.voteAvg.toStringAsFixed(1), style: AppTypography.body.copyWith(fontWeight: FontWeight.w600)),
                        const SizedBox(width: 12),
                        Text('${s.year}', style: AppTypography.small),
                        if (s.genres.isNotEmpty) ...[
                          const SizedBox(width: 12),
                          Text(s.genres.join(' · '), style: AppTypography.small),
                        ],
                      ],
                    ),
                    const SizedBox(height: 8),
                    Text(s.title, style: AppTypography.display),
                    if (s.titleOrig.isNotEmpty && s.titleOrig != s.title)
                      Text(s.titleOrig, style: AppTypography.body.copyWith(color: Colors.white70)),
                    const SizedBox(height: 14),
                    Row(
                      children: [
                        _PrimaryButton(icon: Icons.play_arrow_rounded, label: '播放', onTap: () => _play()),
                        const SizedBox(width: 12),
                        _GhostButton(
                          icon: _favorite ? Icons.favorite_rounded : Icons.favorite_border_rounded,
                          label: _favorite ? '已收藏' : '收藏',
                          onTap: _toggleFavorite,
                        ),
                        const SizedBox(width: 12),
                        _GhostButton(
                          icon: _subscribed ? Icons.notifications_off_rounded : Icons.notifications_active_rounded,
                          label: _subscribed ? '已订阅' : '订阅',
                          onTap: _toggleSubscribe,
                        ),
                      ],
                    ),
                  ],
                ),
              ),
            ],
          ),
        ),
        // 简介
        if (s.overview.isNotEmpty)
          Padding(
            padding: const EdgeInsets.all(AppSpacing.xl),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Text('简介', style: AppTypography.subtitle),
                const SizedBox(height: 8),
                Text(s.overview, style: AppTypography.body.copyWith(height: 1.6, color: AppColors.textSecondary)),
              ],
            ),
          ),
        // 季集列表
        if (d.isSeries && _seasons.isNotEmpty) ...[
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: AppSpacing.xl),
            child: const Text('分集', style: AppTypography.subtitle),
          ),
          const SizedBox(height: 8),
          ..._seasons.map((season) => _SeasonPanel(
                season: season,
                onPlayEpisode: (ep) => _play(season: season.seasonNumber, episode: ep),
              )),
          const SizedBox(height: 32),
        ],
      ],
    );
  }
}

class _PrimaryButton extends StatelessWidget {
  final IconData icon;
  final String label;
  final VoidCallback onTap;
  const _PrimaryButton({required this.icon, required this.label, required this.onTap});
  @override
  Widget build(BuildContext context) {
    return Material(
      color: AppColors.accent,
      borderRadius: BorderRadius.circular(AppRadius.md),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(AppRadius.md),
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 22, vertical: 12),
          child: Row(mainAxisSize: MainAxisSize.min, children: [
            Icon(icon, color: Colors.black, size: 22),
            const SizedBox(width: 6),
            Text(label, style: AppTypography.body.copyWith(color: Colors.black, fontWeight: FontWeight.w700)),
          ]),
        ),
      ),
    );
  }
}

class _GhostButton extends StatelessWidget {
  final IconData icon;
  final String label;
  final VoidCallback onTap;
  const _GhostButton({required this.icon, required this.label, required this.onTap});
  @override
  Widget build(BuildContext context) {
    return Material(
      color: Colors.white.withValues(alpha: 0.1),
      borderRadius: BorderRadius.circular(AppRadius.md),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(AppRadius.md),
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 18, vertical: 12),
          child: Row(mainAxisSize: MainAxisSize.min, children: [
            Icon(icon, color: AppColors.textPrimary, size: 20),
            const SizedBox(width: 6),
            Text(label, style: AppTypography.body.copyWith(color: AppColors.textPrimary, fontWeight: FontWeight.w600, fontSize: 14)),
          ]),
        ),
      ),
    );
  }
}

class _SeasonPanel extends StatelessWidget {
  final SeasonInfo season;
  final void Function(int episode) onPlayEpisode;
  const _SeasonPanel({required this.season, required this.onPlayEpisode});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(AppSpacing.xl, 8, AppSpacing.xl, 0),
      child: Container(
        decoration: BoxDecoration(
          color: AppColors.surface,
          borderRadius: BorderRadius.circular(AppRadius.lg),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Padding(
              padding: const EdgeInsets.all(14),
              child: Text(season.name, style: const TextStyle(fontSize: 15, fontWeight: FontWeight.w600, color: AppColors.textPrimary)),
            ),
            Wrap(
              spacing: 8,
              runSpacing: 8,
              children: season.episodes.map((ep) {
                return InkWell(
                  onTap: () => onPlayEpisode(ep.episodeNumber),
                  borderRadius: BorderRadius.circular(AppRadius.sm),
                  child: Container(
                    padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
                    decoration: BoxDecoration(
                      color: AppColors.surfaceHigh,
                      borderRadius: BorderRadius.circular(AppRadius.sm),
                    ),
                    child: Row(mainAxisSize: MainAxisSize.min, children: [
                      Text('E${ep.episodeNumber.toString().padLeft(2, '0')}',
                          style: const TextStyle(color: AppColors.textPrimary, fontSize: 13, fontWeight: FontWeight.w500)),
                      if (ep.available) ...[
                        const SizedBox(width: 5),
                        const Icon(Icons.check_circle_rounded, color: AppColors.success, size: 15),
                      ],
                    ]),
                  ),
                );
              }).toList(),
            ),
            const SizedBox(height: 14),
          ],
        ),
      ),
    );
  }
}
