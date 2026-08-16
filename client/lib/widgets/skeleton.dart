import 'package:flutter/material.dart';
import '../theme/app_theme.dart';

/// 通用骨架屏组件（[v7 整改] §17.12 骨架屏规范）。
///
/// 提供三档：行骨架（shelf 横向列表）、卡片骨架（poster grid）、详情骨架。
class SkeletonBox extends StatefulWidget {
  final double width;
  final double height;
  final BorderRadius? borderRadius;

  const SkeletonBox({
    super.key,
    required this.width,
    required this.height,
    this.borderRadius,
  });

  @override
  State<SkeletonBox> createState() => _SkeletonBoxState();
}

class _SkeletonBoxState extends State<SkeletonBox> with SingleTickerProviderStateMixin {
  late final AnimationController _ctrl;

  @override
  void initState() {
    super.initState();
    _ctrl = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 1500),
    )..repeat();
  }

  @override
  void dispose() {
    _ctrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: _ctrl,
      builder: (context, _) {
        final t = _ctrl.value;
        return Container(
          width: widget.width,
          height: widget.height,
          decoration: BoxDecoration(
            borderRadius: widget.borderRadius ?? BorderRadius.circular(AppRadius.md),
            gradient: LinearGradient(
              begin: Alignment(-1.0 + 2 * t, 0),
              end: Alignment(1.0 + 2 * t, 0),
              colors: const [
                Color(0xFF1F2630),
                Color(0xFF2A3340),
                Color(0xFF1F2630),
              ],
            ),
          ),
        );
      },
    );
  }
}

/// 横向 shelf 骨架（用于首页/分类页/历史/订阅等列表行）。
class ShelfSkeleton extends StatelessWidget {
  final int cardCount;
  final double cardWidth;
  final double cardHeight;
  const ShelfSkeleton({super.key, this.cardCount = 6, this.cardWidth = 150, this.cardHeight = 225});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 8),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Padding(
            padding: EdgeInsets.fromLTRB(AppSpacing.xl, 8, AppSpacing.xl, 12),
            child: SkeletonBox(width: 120, height: 20),
          ),
          SizedBox(
            height: cardHeight,
            child: ListView.builder(
              scrollDirection: Axis.horizontal,
              padding: const EdgeInsets.symmetric(horizontal: AppSpacing.xl),
              itemCount: cardCount,
              itemBuilder: (context, i) => Padding(
                padding: const EdgeInsets.only(right: AppSpacing.md),
                child: SkeletonBox(width: cardWidth, height: cardHeight),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

/// 网格骨架（搜索结果、分类页等网格布局）。
class GridSkeleton extends StatelessWidget {
  final int columns;
  final int rows;
  const GridSkeleton({super.key, this.columns = 6, this.rows = 2});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.all(AppSpacing.xl),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: List.generate(rows, (r) {
          return Padding(
            padding: const EdgeInsets.only(bottom: AppSpacing.md),
            child: Row(
              children: List.generate(columns, (c) {
                final isLast = c == columns - 1;
                return Padding(
                  padding: EdgeInsets.only(right: isLast ? 0 : AppSpacing.md),
                  child: SkeletonBox(
                    width: (MediaQuery.of(context).size.width -
                            AppSpacing.xl * 2 -
                            AppSpacing.md * (columns - 1)) /
                        columns,
                    height: 220,
                  ),
                );
              }),
            ),
          );
        }),
      ),
    );
  }
}

/// 详情页骨架（横幅 + 简介 + 季集）。
class DetailSkeleton extends StatelessWidget {
  const DetailSkeleton({super.key});

  @override
  Widget build(BuildContext context) {
    return ListView(
      padding: EdgeInsets.zero,
      children: [
        const SkeletonBox(width: double.infinity, height: 300, borderRadius: BorderRadius.zero),
        const SizedBox(height: AppSpacing.xl),
        const Padding(
          padding: EdgeInsets.symmetric(horizontal: AppSpacing.xl),
          child: SkeletonBox(width: 200, height: 28),
        ),
        const SizedBox(height: AppSpacing.sm),
        const Padding(
          padding: EdgeInsets.symmetric(horizontal: AppSpacing.xl),
          child: SkeletonBox(width: 320, height: 16),
        ),
        const SizedBox(height: AppSpacing.xl),
        for (var i = 0; i < 3; i++) ...[
          const Padding(
            padding: EdgeInsets.symmetric(horizontal: AppSpacing.xl),
            child: SkeletonBox(width: double.infinity, height: 80),
          ),
          const SizedBox(height: AppSpacing.md),
        ],
      ],
    );
  }
}
