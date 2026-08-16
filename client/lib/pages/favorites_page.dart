import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../models/media.dart';
import '../services/app_state.dart';
import '../theme/app_theme.dart';
import '../widgets/poster_wall.dart';
import 'detail_page.dart';

class FavoritesPage extends StatefulWidget {
  const FavoritesPage({super.key});

  @override
  State<FavoritesPage> createState() => _FavoritesPageState();
}

class _FavoritesPageState extends State<FavoritesPage> {
  List<Favorite> _items = const [];
  bool _loading = true;
  String _error = '';

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = '';
    });
    try {
      final items = await context.read<AppState>().api.favorites();
      if (mounted) setState(() => _items = items);
    } catch (e) {
      if (mounted) setState(() => _error = e.toString());
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.transparent,
      body: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(AppSpacing.xl, 16, AppSpacing.xl, 10),
            child: Row(
              children: [
                const Text('我的收藏', style: AppTypography.heading),
                const SizedBox(width: 10),
                Container(
                  padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
                  decoration: BoxDecoration(
                    color: AppColors.surface,
                    borderRadius: BorderRadius.circular(6),
                  ),
                  child: Text('${_items.length}', style: AppTypography.caption),
                ),
                const Spacer(),
                if (_items.isNotEmpty)
                  TextButton.icon(
                    onPressed: () => setState(() => _items = const []),
                    icon: const Icon(Icons.refresh_rounded, size: 16, color: AppColors.textSecondary),
                    label: const Text('刷新', style: AppTypography.small),
                  ),
              ],
            ),
          ),
          Expanded(
            child: _loading
                ? const Center(child: CircularProgressIndicator())
                : _error.isNotEmpty
                    ? Center(child: Text('加载失败：$_error', style: AppTypography.body.copyWith(color: AppColors.error)))
                    : _items.isEmpty
                        ? const _Empty()
                        : PosterGrid(
                            items: _items
                                .map((f) => MediaSummary(
                                      externalId: f.externalId,
                                      externalSource: 'tmdb',
                                      mediaType: 'movie',
                                      title: f.title,
                                      year: f.year,
                                      posterUrl: f.posterUrl,
                                    ))
                                .toList(),
                            autofocusFirst: true,
                            onTap: (m) => Navigator.of(context).push(
                              MaterialPageRoute(builder: (_) => DetailPage(externalId: m.externalId, source: m.externalSource)),
                            ),
                          ),
          ),
        ],
      ),
    );
  }
}

class _Empty extends StatelessWidget {
  const _Empty();
  @override
  Widget build(BuildContext context) {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: const [
          Icon(Icons.favorite_border_rounded, size: 56, color: AppColors.textMuted),
          SizedBox(height: 12),
          Text('还没有收藏', style: AppTypography.subtitle),
          SizedBox(height: 6),
          Text('在详情页点击收藏按钮，把喜欢的影视加入收藏', style: AppTypography.small),
        ],
      ),
    );
  }
}
