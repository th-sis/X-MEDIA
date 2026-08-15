import 'package:flutter/material.dart';

/// X-MEDIA 设计令牌（附录 C）+ Kodi 风格暗色主题。
class AppColors {
  AppColors._();

  // Kodi 风格暗色 + 蓝青强调色
  static const accent = Color(0xFF00B8E6);
  static const accentDim = Color(0xFF00779A);
  static const background = Color(0xFF141414);
  static const surface = Color(0xFF1F1F1F);
  static const surfaceHigh = Color(0xFF2A2A2A);
  static const sidebar = Color(0xFF181818);

  static const textPrimary = Color(0xFFF5F5F5);
  static const textSecondary = Color(0xFFA6A6A6);
  static const textMuted = Color(0xFF6B6B6B);

  static const success = Color(0xFF22C55E);
  static const warning = Color(0xFFF59E0B);
  static const error = Color(0xFFFF4D4D);
  static const info = Color(0xFF00E5FF);

  // 四层引擎语义色
  static const resolveP0 = Color(0xFF22C55E);
  static const resolveP1 = Color(0xFF00B8E6);
  static const resolveP2 = Color(0xFFF59E0B);
  static const resolveP3 = Color(0xFF6B6B6B);
}

class AppSpacing {
  AppSpacing._();
  static const xs = 4.0;
  static const sm = 8.0;
  static const md = 12.0;
  static const lg = 16.0;
  static const xl = 24.0;
  static const xxl = 32.0;
}

class AppRadius {
  AppRadius._();
  static const sm = 4.0;
  static const md = 8.0;
  static const lg = 12.0;
  static const xl = 16.0;
}

class AppDuration {
  AppDuration._();
  static const fast = Duration(milliseconds: 150);
  static const normal = Duration(milliseconds: 250);
  static const slow = Duration(milliseconds: 400);
}

/// Kodi 暗色主题。
ThemeData kodiTheme() {
  final base = ThemeData.dark(useMaterial3: true);
  return base.copyWith(
    scaffoldBackgroundColor: AppColors.background,
    colorScheme: const ColorScheme.dark(
      primary: AppColors.accent,
      secondary: AppColors.accent,
      surface: AppColors.surface,
      error: AppColors.error,
    ),
    textTheme: base.textTheme.apply(
      bodyColor: AppColors.textPrimary,
      displayColor: AppColors.textPrimary,
    ),
    splashColor: Colors.transparent,
    highlightColor: Colors.transparent,
    hoverColor: Colors.white.withValues(alpha: 0.04),
    focusColor: Colors.transparent,
  );
}

/// 海报占位渐变色板（按 id 取模生成稳定颜色）。
List<Color> posterGradient(int seed) {
  const palettes = <List<Color>>[
    [Color(0xFF1A2A6C), Color(0xFF1E3A8A), Color(0xFF3B82F6)],
    [Color(0xFF3E1F47), Color(0xFF6B21A8), Color(0xFFA855F7)],
    [Color(0xFF1F3D2B), Color(0xFF15803D), Color(0xFF22C55E)],
    [Color(0xFF4C1D24), Color(0xFFB91C1C), Color(0xFFEF4444)],
    [Color(0xFF3D2A0C), Color(0xFFB45309), Color(0xFFF59E0B)],
    [Color(0xFF0C3D4A), Color(0xFF0E7490), Color(0xFF06B6D4)],
  ];
  return palettes[seed.abs() % palettes.length];
}
