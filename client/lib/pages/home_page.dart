import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../services/app_state.dart';
import '../theme/app_theme.dart';
import '../widgets/focus.dart';
import '../widgets/poster_card.dart';
import '../widgets/poster_wall.dart';
import 'detail_page.dart';

class HomePage extends StatelessWidget {
  const HomePage({super.key});

  @override
  Widget build(BuildContext context) {
    final app = context.watch<AppState>();

    if (app.loading && app.sections.isEmpty) {
      return const _Skeleton();
    }
    if (app.error.isNotEmpty && app.sections.isEmpty) {
      return _ErrorState(message: app.error, onRetry: () => app.refresh());
    }

    return ListView(
      key: const PageStorageKey('home'),
      padding: const EdgeInsets.only(top: 12, bottom: 48),
      children: [
        // 继续观看行
        if (app.continueWatching.isNotEmpty) ...[
          const _SectionHeader(title: '继续观看'),
          SizedBox(
            height: 135,
            child: FocusTraversalGroup(
              policy: KodiTraversalPolicy(),
              child: ListView.builder(
                scrollDirection: Axis.horizontal,
                padding: const EdgeInsets.symmetric(horizontal: AppSpacing.xl),
                itemCount: app.continueWatching.length,
                itemBuilder: (context, i) {
                  final cw = app.continueWatching[i];
                  return Padding(
                    padding: const EdgeInsets.only(right: AppSpacing.md),
                    child: LandscapeCard(
                      title: cw.title,
                      subtitle: cw.episodeLabel,
                      progress: cw.progress,
                      posterUrl: cw.posterUrl,
                      seed: cw.externalId,
                      onTap: () => _openDetail(context, cw.externalId, cw.externalSource),
                    ),
                  );
                },
              ),
            ),
          ),
          const SizedBox(height: 8),
        ],
        // 12 榜单
        ...app.sections.map((s) => PosterShelf(
              title: s.title,
              items: s.items,
              onTap: (m) => _openDetail(context, m.externalId, m.externalSource),
            )),
      ],
    );
  }

  void _openDetail(BuildContext context, int id, String source) {
    Navigator.of(context).push(MaterialPageRoute(builder: (_) => DetailPage(externalId: id, source: source)));
  }
}

class _SectionHeader extends StatelessWidget {
  final String title;
  const _SectionHeader({required this.title});
  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(AppSpacing.xl, 16, AppSpacing.xl, 8),
      child: Text(title, style: const TextStyle(fontSize: 20, fontWeight: FontWeight.w600, color: AppColors.textPrimary)),
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
