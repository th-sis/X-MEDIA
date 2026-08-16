import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../services/app_state.dart';
import '../theme/app_theme.dart';

class SettingsPage extends StatefulWidget {
  const SettingsPage({super.key});

  @override
  State<SettingsPage> createState() => _SettingsPageState();
}

class _SettingsPageState extends State<SettingsPage> {
  late final TextEditingController _urlController;

  @override
  void initState() {
    super.initState();
    _urlController = TextEditingController(text: context.read<AppState>().backendUrl);
  }

  @override
  void dispose() {
    _urlController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final app = context.watch<AppState>();
    final caps = app.capabilities;

    return Scaffold(
      backgroundColor: Colors.transparent,
      body: ListView(
        padding: const EdgeInsets.all(AppSpacing.xl),
        children: [
          const Text('设置', style: AppTypography.heading),
          const SizedBox(height: 16),

          // 后端连接
          _card('后端连接', [
            Row(
              children: [
                Expanded(
                  child: TextField(
                    controller: _urlController,
                    style: AppTypography.small,
                    decoration: InputDecoration(
                      labelText: '后端地址',
                      hintText: 'http://127.0.0.1:8080',
                      filled: true,
                      fillColor: AppColors.surface,
                      border: OutlineInputBorder(borderRadius: BorderRadius.circular(AppRadius.md), borderSide: BorderSide.none),
                    ),
                  ),
                ),
                const SizedBox(width: 12),
                ElevatedButton(
                  onPressed: () => app.setBackendUrl(_urlController.text),
                  child: const Text('连接'),
                ),
              ],
            ),
            const SizedBox(height: 8),
            _StatusRow(ok: app.wsConnected, text: app.wsConnected ? '已连接' : '未连接'),
          ]),

          // 能力预检
          _card('系统能力', [
            _StatusRow(ok: caps.nasAvailable, text: caps.nasAvailable ? 'NAS 本地索引可用' : 'NAS 未配置（跳过本地秒播）'),
            _StatusRow(ok: caps.nasIndexComplete, text: caps.nasIndexComplete ? 'NAS 索引已完成' : 'NAS 索引未完成'),
            _StatusRow(ok: caps.pansearchAvailable, text: caps.pansearchAvailable ? '盘搜服务可用' : '盘搜服务不可用'),
            _StatusRow(
              ok: caps.loggedInDrivers.isNotEmpty,
              text: caps.loggedInDrivers.isEmpty ? '未登录任何网盘（演示模式）' : '已登录网盘：${caps.loggedInDrivers.join(', ')}',
            ),
            Text('服务端版本：${caps.serverVersion.isEmpty ? '未知' : caps.serverVersion}', style: AppTypography.caption),
          ]),

          // 健康检查
          if (app.health.isNotEmpty) _card('健康检查', _healthRows(app)),

          const SizedBox(height: 24),
          const Text('操作说明：方向键 / 鼠标导航，回车或点击确认，Esc 或返回键退出。', style: AppTypography.caption),
        ],
      ),
    );
  }

  List<Widget> _healthRows(AppState app) {
    final h = app.health;
    String status(String k) => h[k] as String? ?? '未知';
    return [
      _StatusRow(ok: status('db') == 'ok', text: '数据库：${status('db')}'),
      _StatusRow(ok: status('tmdb') == 'ok', text: 'TMDB：${status('tmdb')}'),
      _StatusRow(ok: status('pansearch') == 'ok', text: '盘搜：${status('pansearch')}'),
      const SizedBox(height: 4),
      Text('整体状态：${h['overall'] ?? '未知'}', style: AppTypography.small),
    ];
  }

  Widget _card(String title, List<Widget> children) {
    return Container(
      margin: const EdgeInsets.only(bottom: 20),
      padding: const EdgeInsets.all(18),
      decoration: BoxDecoration(
        color: AppColors.surface.withValues(alpha: 0.6),
        borderRadius: BorderRadius.circular(AppRadius.lg),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(title, style: AppTypography.subtitle),
          const SizedBox(height: 14),
          ...children,
        ],
      ),
    );
  }
}

class _StatusRow extends StatelessWidget {
  final bool ok;
  final String text;
  const _StatusRow({required this.ok, required this.text});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 5),
      child: Row(
        children: [
          Icon(ok ? Icons.check_circle_rounded : Icons.warning_amber_rounded,
              color: ok ? AppColors.success : AppColors.warning, size: 18),
          const SizedBox(width: 10),
          Expanded(child: Text(text, style: AppTypography.body.copyWith(fontSize: 14))),
        ],
      ),
    );
  }
}
