import 'package:flutter/material.dart';
import '../models/media.dart';
import '../theme/app_theme.dart';
import 'focus.dart';
import 'poster_card.dart';

/// 横向海报行（榜单）。
class PosterShelf extends StatelessWidget {
  final String title;
  final List<MediaSummary> items;
  final void Function(MediaSummary) onTap;
  final bool autofocusFirst;

  const PosterShelf({
    super.key,
    required this.title,
    required this.items,
    required this.onTap,
    this.autofocusFirst = false,
  });

  @override
  Widget build(BuildContext context) {
    if (items.isEmpty) return const SizedBox.shrink();
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(AppSpacing.xl, 10, AppSpacing.xl, 8),
          child: Text(
            title,
            style: AppTypography.title,
          ),
        ),
        SizedBox(
          height: 225,
          child: FocusTraversalGroup(
            policy: KodiTraversalPolicy(),
            child: ListView.builder(
              scrollDirection: Axis.horizontal,
              padding: const EdgeInsets.symmetric(horizontal: AppSpacing.xl),
              itemCount: items.length,
              itemBuilder: (context, i) => Padding(
                padding: const EdgeInsets.only(right: AppSpacing.md),
                child: PosterCard(
                  media: items[i],
                  autofocus: autofocusFirst && i == 0,
                  onTap: () => onTap(items[i]),
                ),
              ),
            ),
          ),
        ),
      ],
    );
  }
}

/// 海报网格（分类/搜索页）。
class PosterGrid extends StatelessWidget {
  final List<MediaSummary> items;
  final void Function(MediaSummary) onTap;
  final bool autofocusFirst;
  final double posterWidth;

  const PosterGrid({
    super.key,
    required this.items,
    required this.onTap,
    this.autofocusFirst = false,
    this.posterWidth = 150,
  });

  @override
  Widget build(BuildContext context) {
    final crossAxisCount = (MediaQuery.of(context).size.width / (posterWidth + 24)).floor().clamp(2, 8);
    return FocusTraversalGroup(
      policy: KodiTraversalPolicy(),
      child: GridView.builder(
        padding: const EdgeInsets.all(AppSpacing.xl),
        gridDelegate: SliverGridDelegateWithFixedCrossAxisCount(
          crossAxisCount: crossAxisCount,
          mainAxisSpacing: 20,
          crossAxisSpacing: 16,
          childAspectRatio: 150 / 225,
        ),
        itemCount: items.length,
        itemBuilder: (context, i) => PosterCard(
          media: items[i],
          autofocus: autofocusFirst && i == 0,
          onTap: () => onTap(items[i]),
        ),
      ),
    );
  }
}
