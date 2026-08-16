import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../models/media.dart';
import '../services/app_state.dart';
import '../theme/app_theme.dart';
import '../widgets/poster_wall.dart';
import '../widgets/skeleton.dart';
import 'detail_page.dart';

class CategoryPage extends StatefulWidget {
  final String type;
  final String title;
  const CategoryPage({super.key, required this.type, required this.title});

  @override
  State<CategoryPage> createState() => _CategoryPageState();
}

class _CategoryPageState extends State<CategoryPage> {
  List<MediaSummary> _items = const [];
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
      final items = await context.read<AppState>().api.discover(widget.type);
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
      body: _loading
          ? const GridSkeleton()
          : _error.isNotEmpty
              ? Center(
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      const Icon(Icons.error_outline_rounded, color: AppColors.error, size: 48),
                      const SizedBox(height: 12),
                      Text('加载失败', style: AppTypography.subtitle),
                      const SizedBox(height: 8),
                      TextButton(onPressed: _load, child: const Text('重试')),
                    ],
                  ),
                )
              : _items.isEmpty
                  ? Center(
                      child: Column(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          const Icon(Icons.movie_filter_rounded, color: AppColors.textMuted, size: 56),
                          const SizedBox(height: 12),
                          Text('暂无内容', style: AppTypography.subtitle),
                          const SizedBox(height: 6),
                          Text('请稍后再试或切换其他分类', style: AppTypography.small),
                        ],
                      ),
                    )
                  : PosterGrid(
                      items: _items,
                      autofocusFirst: true,
                      onTap: (m) => Navigator.of(context).push(
                        MaterialPageRoute(builder: (_) => DetailPage(externalId: m.externalId, source: m.externalSource)),
                      ),
                    ),
    );
  }
}
