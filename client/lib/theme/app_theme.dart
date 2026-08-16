import 'package:flutter/material.dart';

/// X-MEDIA 设计令牌（附录 C）+ Kodi 风格暗色主题。
///
/// 导航风格（v7 修订）：
/// - 按钮默认显示大写英文
/// - 聚焦/选中时切换为中文
/// - 透明背景 + 大字号
class AppColors {
  AppColors._();

  // 主品牌色（深色模式，X-MEDIA 视觉基调）
  static const accent = Color(0xFF00B8E6);     // 蓝青强调色（与 cdn-yingshi.png 一致）
  static const accentDim = Color(0xFF00779A);
  static const accentGlow = Color(0xFF6B21A8);  // 紫色辅助光（背景辉光用）

  // 背景层级
  static const background = Color(0xFF0A0D14);  // 主背景（深空蓝黑）
  static const surface = Color(0xFF141A24);     // 卡片底
  static const surfaceHigh = Color(0xFF1F2630); // 卡片浮起
  static const sidebar = Color(0xFF10141C);

  // 文字
  static const textPrimary = Color(0xFFF5F7FA);
  static const textSecondary = Color(0xFFA8B0BD);
  static const textMuted = Color(0xFF6B7280);

  // 语义色
  static const success = Color(0xFF22C55E);
  static const warning = Color(0xFFF59E0B);
  static const error = Color(0xFFFF4D4D);
  static const info = Color(0xFF00E5FF);

  // 顶/底栏高亮色（Kodi 风格选中指示）
  static const selection = Color(0xFFFF007A); // 粉红强调线/焦点色

  // 四层引擎语义色（附录 C.1 resolve_p0-p3）
  static const resolveP0 = Color(0xFF22C55E);
  static const resolveP1 = Color(0xFF6E6CF0);
  static const resolveP2 = Color(0xFFF59E0B);
  static const resolveP3 = Color(0xFF9CA3AF);
}

class AppSpacing {
  AppSpacing._();
  static const xs = 4.0;
  static const sm = 8.0;
  static const md = 12.0;
  static const lg = 16.0;
  static const xl = 24.0;
  static const xxl = 32.0;
  static const xxxl = 48.0;
}

class AppRadius {
  AppRadius._();
  static const sm = 4.0;
  static const md = 8.0;
  static const lg = 12.0;
  static const xl = 16.0;
  static const full = 9999.0;
}

class AppDuration {
  AppDuration._();
  static const fast = Duration(milliseconds: 150);
  static const normal = Duration(milliseconds: 250);
  static const slow = Duration(milliseconds: 400);
}

/// 排版 token（附录 C C.5 + 导航大字）。
class AppTypography {
  AppTypography._();

  // 正文
  static const TextStyle caption = TextStyle(fontSize: 11, fontWeight: FontWeight.w500, letterSpacing: 0.3, color: AppColors.textMuted);
  static const TextStyle small = TextStyle(fontSize: 13, color: AppColors.textSecondary);
  static const TextStyle body = TextStyle(fontSize: 15, color: AppColors.textPrimary);
  static const TextStyle subtitle = TextStyle(fontSize: 16, fontWeight: FontWeight.w600, color: AppColors.textPrimary);
  static const TextStyle title = TextStyle(fontSize: 22, fontWeight: FontWeight.w700, color: AppColors.textPrimary, letterSpacing: 0.2);
  static const TextStyle heading = TextStyle(fontSize: 28, fontWeight: FontWeight.w800, color: AppColors.textPrimary, letterSpacing: 0.3);
  static const TextStyle display = TextStyle(fontSize: 36, fontWeight: FontWeight.w800, color: Colors.white, letterSpacing: 0.5);

  // 导航栏 —— 透明大字号（默认英文）
  static const TextStyle navEnglish = TextStyle(
    fontSize: 22,
    fontWeight: FontWeight.w500,
    letterSpacing: 2.5,
    height: 1.0,
    color: AppColors.textSecondary,
  );
  static const TextStyle navEnglishFocused = TextStyle(
    fontSize: 24,
    fontWeight: FontWeight.w600,
    letterSpacing: 1.5,
    height: 1.0,
    color: AppColors.accent,
    shadows: [Shadow(color: Color(0x6600B8E6), blurRadius: 14)],
  );
  static const TextStyle navChinese = TextStyle(
    fontSize: 26,
    fontWeight: FontWeight.w700,
    letterSpacing: 4.0,
    height: 1.0,
    color: AppColors.textPrimary,
  );
  static const TextStyle navChineseActive = TextStyle(
    fontSize: 28,
    fontWeight: FontWeight.w800,
    letterSpacing: 4.0,
    height: 1.0,
    color: AppColors.accent,
    shadows: [Shadow(color: Color(0x9900B8E6), blurRadius: 18)],
  );
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

/// 数据源连接徽章配色（顶部右上角圆角胶囊）。
class SourceBadge {
  final String label;
  final Color color;
  const SourceBadge(this.label, this.color);
}

/// 根据后端 capabilities 派生连接源徽章列表。
List<SourceBadge> sourceBadgesFromCapabilities({
  required bool nasAvailable,
  required bool pansearchAvailable,
  required List<String> loggedInDrivers,
}) {
  final out = <SourceBadge>[
    if (nasAvailable) const SourceBadge('NAS', AppColors.info),
    if (pansearchAvailable) const SourceBadge('PANSOU', AppColors.accent),
    for (final d in loggedInDrivers) SourceBadge(d.toUpperCase(), AppColors.success),
  ];
  if (out.isEmpty) {
    // 演示 / 离线：固定一组空状态徽章
    return const [
      SourceBadge('LOCAL', AppColors.textMuted),
    ];
  }
  return out;
}
