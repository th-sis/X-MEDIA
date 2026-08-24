import 'dart:async';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../models/media.dart';
import '../services/app_state.dart';
import '../services/availability.dart';
import '../theme/app_theme.dart';
import '../widgets/poster_wall.dart';
import '../widgets/skeleton.dart';
import 'detail_page.dart';

class SearchPage extends StatefulWidget {
  const SearchPage({super.key});

  @override
  State<SearchPage> createState() => _SearchPageState();
}

class _SearchPageState extends State<SearchPage> {
  final _controller = TextEditingController();
  final _focus = FocusNode();
  Timer? _debounce;
  List<MediaSummary> _results = const [];
  List<String> _history = const [];
  // [V7 §17.2 D53] 已索引 ID 集合, PosterGrid 用它渲染左上角绿色 ✓ 角标.
  Set<int> _availableIds = const {};
  bool _loading = false;
  bool _searched = false;

  @override
  void initState() {
    super.initState();
    _loadHistory();
    _controller.addListener(() {
      _debounce?.cancel();
      _debounce = Timer(const Duration(milliseconds: 300), () {
        if (_controller.text.trim().isNotEmpty) {
          _search(_controller.text.trim());
        } else {
          setState(() {
            _searched = false;
            _results = const [];
          });
        }
      });
    });
  }

  @override
  void dispose() {
    _debounce?.cancel();
    _controller.dispose();
    _focus.dispose();
    super.dispose();
  }

  Future<void> _loadHistory() async {
    final h = await context.read<AppState>().api.searchHistory();
    if (mounted) setState(() => _history = h);
  }

  Future<void> _search(String q) async {
    setState(() {
      _loading = true;
      _searched = true;
      _availableIds = const {};
    });
    try {
      final r = await context.read<AppState>().api.search(q);
      if (mounted) {
        setState(() => _results = r);
        _loadHistory();
        // [V7 §17.2] 批量查询可播放性, 显示 ✓ 角标.
        _refreshAvailability(r);
      }
    } catch (_) {
      if (mounted) setState(() => _results = const []);
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  /// [V7 §17.2] 批量查可播放性. 调用失败静默(不阻塞搜索结果).
  Future<void> _refreshAvailability(List<MediaSummary> items) async {
    if (items.isEmpty) return;
    try {
      final keys = items
          .map((m) => availabilityKeyForSummary(externalId: m.externalId, externalSource: m.externalSource))
          .toList();
      final available = await context.read<AppState>().api.checkAvailability(keys);
      if (mounted) {
        setState(() {
          _availableIds = available.map((k) => k.externalId).toSet();
        });
      }
    } catch (_) {
      // 静默: 网络/超时不应阻塞搜索结果
    }
  }

  void _submit(String q) {
    if (q.trim().isNotEmpty) _search(q.trim());
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.transparent,
      body: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // 搜索栏
          Padding(
            padding: const EdgeInsets.fromLTRB(AppSpacing.xl, 16, AppSpacing.xl, 16),
            child: Focus(
              focusNode: _focus,
              child: TextField(
                controller: _controller,
                autofocus: false,
                onSubmitted: _submit,
                style: AppTypography.body,
                decoration: InputDecoration(
                  hintText: '搜索电影 / 剧集 / 动漫...',
                  prefixIcon: const Icon(Icons.search_rounded, color: AppColors.textSecondary),
                  suffixIcon: _controller.text.isNotEmpty
                      ? IconButton(
                          icon: const Icon(Icons.close_rounded, color: AppColors.textSecondary),
                          onPressed: () {
                            _controller.clear();
                            setState(() {
                              _searched = false;
                              _results = const [];
                            });
                          },
                        )
                      : null,
                  filled: true,
                  fillColor: AppColors.surface,
                  border: OutlineInputBorder(
                    borderRadius: BorderRadius.circular(AppRadius.lg),
                    borderSide: BorderSide.none,
                  ),
                ),
              ),
            ),
          ),
          Expanded(
            child: !_searched
                ? _buildHistory()
                : _loading
                    ? const GridSkeleton()
                    : _results.isEmpty
                        ? _buildNoResult()
                        : PosterGrid(
                            items: _results,
                            autofocusFirst: true,
                            availableIds: _availableIds,
                            onTap: (m) => Navigator.of(context).push(
                              MaterialPageRoute(builder: (_) => DetailPage(externalId: m.externalId, source: m.externalSource)),
                            ),
                          ),
          ),
        ],
      ),
    );
  }

  Widget _buildHistory() {
    if (_history.isEmpty) {
      return const Center(
        child: Text('输入关键词开始搜索', style: AppTypography.body),
      );
    }
    return ListView(
      padding: const EdgeInsets.all(AppSpacing.xl),
      children: [
        Row(
          children: [
            const Text('搜索历史', style: AppTypography.subtitle),
            const Spacer(),
            TextButton(
              onPressed: () async {
                await context.read<AppState>().api.clearSearchHistory();
                _loadHistory();
              },
              child: const Text('清空', style: AppTypography.small),
            ),
          ],
        ),
        ..._history.map((h) => ListTile(
              leading: const Icon(Icons.history_rounded, color: AppColors.textMuted),
              title: Text(h, style: AppTypography.body),
              onTap: () {
                _controller.text = h;
                _search(h);
              },
            )),
      ],
    );
  }

  Widget _buildNoResult() {
    final hasPan = context.read<AppState>().capabilities.pansearchAvailable;
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          const Icon(Icons.search_off_rounded, size: 56, color: AppColors.textMuted),
          const SizedBox(height: 12),
          Text('未找到“${_controller.text}”相关内容', style: AppTypography.subtitle),
          const SizedBox(height: 10),
          if (hasPan)
            ElevatedButton.icon(
              onPressed: () => _directPanSearch(_controller.text),
              icon: const Icon(Icons.cloud_download_rounded, size: 18),
              label: const Text('直接盘搜'),
              style: ElevatedButton.styleFrom(
                backgroundColor: AppColors.accent,
                foregroundColor: Colors.black,
              ),
            )
          else
            const Text('建议尝试英文名，或简化关键词', style: AppTypography.small),
        ],
      ),
    );
  }

  Future<void> _directPanSearch(String q) async {
    if (q.trim().isEmpty) return;
    setState(() {
      _loading = true;
      _searched = true;
    });
    try {
      final results = await context.read<AppState>().api.panSearch(q.trim());
      if (mounted) {
        setState(() => _results = results);
      }
    } catch (_) {
      if (mounted) {
        setState(() => _results = const []);
      }
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }
}
