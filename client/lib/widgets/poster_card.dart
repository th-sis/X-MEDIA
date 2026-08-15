import 'package:flutter/material.dart';
import '../models/media.dart';
import '../theme/app_theme.dart';
import 'focus.dart';

/// 海报卡片：真实海报或渐变占位 + 焦点高亮 + 评分角标。
class PosterCard extends StatelessWidget {
  final MediaSummary media;
  final VoidCallback onTap;
  final double width;
  final double height;
  final bool autofocus;

  const PosterCard({
    super.key,
    required this.media,
    required this.onTap,
    this.width = 150,
    this.height = 225,
    this.autofocus = false,
  });

  @override
  Widget build(BuildContext context) {
    return KodiFocus(
      autofocus: autofocus,
      debugLabel: 'poster:${media.title}',
      onActivate: onTap,
      builder: (context, focused) {
        return AnimatedScale(
          scale: focused ? 1.06 : 1.0,
          duration: AppDuration.normal,
          child: AnimatedContainer(
            duration: AppDuration.normal,
            width: width,
            height: height,
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(AppRadius.md),
              border: Border.all(
                color: focused ? AppColors.accent : Colors.transparent,
                width: 2.5,
              ),
              boxShadow: focused
                  ? [BoxShadow(color: AppColors.accent.withValues(alpha: 0.35), blurRadius: 16, spreadRadius: 1)]
                  : const [],
            ),
            child: ClipRRect(
              borderRadius: BorderRadius.circular(AppRadius.md - 2),
              child: Stack(
                fit: StackFit.expand,
                children: [
                  _poster(),
                  // 评分角标
                  if (media.voteAvg > 0)
                    Positioned(
                      top: 6,
                      right: 6,
                      child: Container(
                        padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                        decoration: BoxDecoration(
                          color: Colors.black.withValues(alpha: 0.7),
                          borderRadius: BorderRadius.circular(AppRadius.sm),
                        ),
                        child: Row(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            const Icon(Icons.star, size: 12, color: AppColors.warning),
                            const SizedBox(width: 2),
                            Text(media.voteAvg.toStringAsFixed(1),
                                style: const TextStyle(fontSize: 11, color: AppColors.textPrimary, fontWeight: FontWeight.w600)),
                          ],
                        ),
                      ),
                    ),
                ],
              ),
            ),
          ),
        );
      },
    );
  }

  Widget _poster() {
    if (media.posterUrl.isNotEmpty) {
      return Image.network(
        media.posterUrl,
        fit: BoxFit.cover,
        errorBuilder: (_, __, ___) => _placeholder(),
        loadingBuilder: (context, child, progress) =>
            progress == null ? child : _placeholder(),
      );
    }
    return _placeholder();
  }

  Widget _placeholder() {
    final grad = posterGradient(media.externalId);
    return DecoratedBox(
      decoration: BoxDecoration(
        gradient: LinearGradient(
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
          colors: grad,
        ),
      ),
      child: Padding(
        padding: const EdgeInsets.all(10),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Icon(Icons.local_movies_rounded, color: Colors.white.withValues(alpha: 0.55), size: 30),
            const SizedBox(height: 10),
            Text(
              media.title,
              maxLines: 3,
              overflow: TextOverflow.ellipsis,
              style: const TextStyle(
                color: Colors.white,
                fontWeight: FontWeight.bold,
                fontSize: 14,
                height: 1.25,
                shadows: [Shadow(color: Colors.black45, blurRadius: 4)],
              ),
            ),
            const Spacer(),
            if (media.year > 0)
              Text(
                '${media.year}',
                style: TextStyle(color: Colors.white.withValues(alpha: 0.8), fontSize: 12, fontWeight: FontWeight.w600),
              ),
          ],
        ),
      ),
    );
  }
}

/// 横版卡片（继续观看行用）。
class LandscapeCard extends StatelessWidget {
  final String title;
  final String subtitle;
  final double progress;
  final String posterUrl;
  final int seed;
  final VoidCallback onTap;
  final bool autofocus;
  final double width;
  final double height;

  const LandscapeCard({
    super.key,
    required this.title,
    required this.subtitle,
    this.progress = 0,
    this.posterUrl = '',
    this.seed = 0,
    required this.onTap,
    this.autofocus = false,
    this.width = 240,
    this.height = 135,
  });

  @override
  Widget build(BuildContext context) {
    return KodiFocus(
      autofocus: autofocus,
      onActivate: onTap,
      builder: (context, focused) {
        return AnimatedScale(
          scale: focused ? 1.05 : 1.0,
          duration: AppDuration.normal,
          child: AnimatedContainer(
            duration: AppDuration.normal,
            width: width,
            height: height,
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(AppRadius.md),
              border: Border.all(color: focused ? AppColors.accent : Colors.transparent, width: 2.5),
              gradient: LinearGradient(
                begin: Alignment.topLeft,
                end: Alignment.bottomRight,
                colors: posterGradient(seed),
              ),
              boxShadow: focused ? [BoxShadow(color: AppColors.accent.withValues(alpha: 0.35), blurRadius: 14)] : const [],
            ),
            child: ClipRRect(
              borderRadius: BorderRadius.circular(AppRadius.md - 2),
              child: Stack(
                fit: StackFit.expand,
                children: [
                  if (posterUrl.isNotEmpty)
                    Image.network(posterUrl, fit: BoxFit.cover, errorBuilder: (_, __, ___) => const SizedBox()),
                  // 底部渐变遮罩
                  Positioned.fill(
                    child: DecoratedBox(
                      decoration: BoxDecoration(
                        gradient: LinearGradient(
                          begin: Alignment.topCenter,
                          end: Alignment.bottomCenter,
                          colors: [Colors.transparent, Colors.black.withValues(alpha: 0.85)],
                          stops: const [0.45, 1.0],
                        ),
                      ),
                    ),
                  ),
                  Positioned(
                    left: 10, right: 10, bottom: 8,
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Text(title, maxLines: 1, overflow: TextOverflow.ellipsis,
                            style: const TextStyle(color: Colors.white, fontWeight: FontWeight.bold, fontSize: 13)),
                        if (subtitle.isNotEmpty)
                          Text(subtitle, maxLines: 1, overflow: TextOverflow.ellipsis,
                              style: TextStyle(color: Colors.white.withValues(alpha: 0.8), fontSize: 11)),
                        const SizedBox(height: 4),
                        ClipRRect(
                          borderRadius: BorderRadius.circular(2),
                          child: LinearProgressIndicator(
                            value: progress.clamp(0, 1),
                            minHeight: 3,
                            backgroundColor: Colors.white.withValues(alpha: 0.25),
                            valueColor: const AlwaysStoppedAnimation(AppColors.accent),
                          ),
                        ),
                      ],
                    ),
                  ),
                ],
              ),
            ),
          ),
        );
      },
    );
  }
}
