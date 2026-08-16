import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../models/media.dart';
import '../services/app_state.dart';
import '../theme/app_theme.dart';
import 'detail_page.dart';

class HistoryPage extends StatefulWidget {
  const HistoryPage({super.key});

  @override
  State<HistoryPage> createState() => _HistoryPageState();
}

class _HistoryPageState extends State<HistoryPage> {
  List<HistoryItem> _items = const [];
  bool _loading = true;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() => _loading = true);
    try {
      final items = await context.read<AppState>().api.history();
      if (mounted) setState(() => _items = items);
    } catch (_) {}
    if (mounted) setState(() => _loading = false);
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
                const Text('播放历史', style: AppTypography.heading),
                const Spacer(),
                if (_items.isNotEmpty)
                  TextButton(
                    onPressed: () async {
                      await context.read<AppState>().api.clearHistory();
                      _load();
                    },
                    child: const Text('清空', style: AppTypography.small),
                  ),
              ],
            ),
          ),
          Expanded(
            child: _loading
                ? const Center(child: CircularProgressIndicator())
                : _items.isEmpty
                    ? const Center(child: Text('暂无播放记录', style: AppTypography.body))
                    : ListView.builder(
                        padding: const EdgeInsets.symmetric(horizontal: AppSpacing.xl, vertical: 8),
                        itemCount: _items.length,
                        itemBuilder: (context, i) {
                          final h = _items[i];
                          final label = h.season > 0 ? 'S${h.season.toString().padLeft(2, '0')}E${h.episode.toString().padLeft(2, '0')}' : '';
                          return ListTile(
                            leading: _thumb(h),
                            title: Text(h.title, style: AppTypography.body),
                            subtitle: Text([label, _fmtProgress(h)].where((e) => e.isNotEmpty).join(' · '),
                                style: AppTypography.caption),
                            trailing: const Icon(Icons.play_arrow_rounded, color: AppColors.accent),
                            onTap: () => Navigator.of(context).push(
                              MaterialPageRoute(builder: (_) => DetailPage(externalId: h.externalId, source: h.externalSource)),
                            ),
                          );
                        },
                      ),
          ),
        ],
      ),
    );
  }

  Widget _thumb(HistoryItem h) {
    if (h.posterUrl.isNotEmpty) {
      return ClipRRect(borderRadius: BorderRadius.circular(6), child: Image.network(h.posterUrl, width: 48, height: 64, fit: BoxFit.cover, errorBuilder: (_, __, ___) => _placeholder()));
    }
    return _placeholder();
  }

  Widget _placeholder() => Container(
        width: 48, height: 64,
        decoration: BoxDecoration(gradient: LinearGradient(colors: posterGradient(0)), borderRadius: BorderRadius.circular(6)),
      );

  String _fmtProgress(HistoryItem h) {
    if (h.durationMs <= 0) return '';
    final pct = (h.positionMs / h.durationMs * 100).round();
    return '已看 $pct%';
  }
}
