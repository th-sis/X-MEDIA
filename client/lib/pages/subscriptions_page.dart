import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../models/media.dart';
import '../services/app_state.dart';
import '../theme/app_theme.dart';

class SubscriptionsPage extends StatefulWidget {
  const SubscriptionsPage({super.key});

  @override
  State<SubscriptionsPage> createState() => _SubscriptionsPageState();
}

class _SubscriptionsPageState extends State<SubscriptionsPage> {
  List<SubscriptionItem> _items = const [];
  bool _loading = true;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() => _loading = true);
    try {
      final items = await context.read<AppState>().api.subscriptions();
      if (mounted) setState(() => _items = items);
    } catch (_) {}
    if (mounted) setState(() => _loading = false);
  }

  String _statusText(String s) {
    switch (s) {
      case 'found': return '已找到';
      case 'downloaded': return '可观看';
      case 'failed': return '搜寻失败';
      default: return '搜寻中';
    }
  }

  Color _statusColor(String s) {
    switch (s) {
      case 'found': return AppColors.info;
      case 'downloaded': return AppColors.success;
      case 'failed': return AppColors.error;
      default: return AppColors.warning;
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.transparent,
      body: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Padding(
            padding: EdgeInsets.fromLTRB(AppSpacing.xl, 16, AppSpacing.xl, 10),
            child: Text('订阅', style: AppTypography.heading),
          ),
          Expanded(
            child: _loading
                ? const Center(child: CircularProgressIndicator())
                : _items.isEmpty
                    ? const Center(child: Text('暂无订阅', style: AppTypography.body))
                    : ListView.builder(
                        padding: const EdgeInsets.symmetric(horizontal: AppSpacing.xl, vertical: 8),
                        itemCount: _items.length,
                        itemBuilder: (context, i) {
                          final s = _items[i];
                          return Container(
                            margin: const EdgeInsets.only(bottom: 10),
                            padding: const EdgeInsets.all(14),
                            decoration: BoxDecoration(
                              color: AppColors.surface,
                              borderRadius: BorderRadius.circular(AppRadius.md),
                            ),
                            child: Row(
                              children: [
                                Expanded(
                                  child: Column(
                                    crossAxisAlignment: CrossAxisAlignment.start,
                                    children: [
                                      Text(s.title, style: AppTypography.subtitle),
                                      const SizedBox(height: 4),
                                      Text('搜寻次数 ${s.searchCount}/${s.maxSearches}', style: AppTypography.caption),
                                    ],
                                  ),
                                ),
                                Container(
                                  padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 5),
                                  decoration: BoxDecoration(
                                    color: _statusColor(s.status).withValues(alpha: 0.15),
                                    borderRadius: BorderRadius.circular(999),
                                  ),
                                  child: Text(_statusText(s.status),
                                      style: TextStyle(color: _statusColor(s.status), fontSize: 12, fontWeight: FontWeight.w600)),
                                ),
                              ],
                            ),
                          );
                        },
                      ),
          ),
        ],
      ),
    );
  }
}
