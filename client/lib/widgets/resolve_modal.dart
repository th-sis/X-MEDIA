import 'dart:async';
import 'package:flutter/material.dart';
import '../models/media.dart';
import '../services/api_client.dart';
import '../services/app_state.dart';
import '../services/ws_client.dart';
import '../theme/app_theme.dart';
import 'resolve_modal_helpers.dart';

class ResolveModal extends StatefulWidget {
  final AppState app;
  final ApiClient api;
  final int externalId;
  final String source;
  final String mediaType;
  final String title;
  final int year;
  final int season;
  final int episode;
  final void Function(String streamUrl, String source) onReady;

  const ResolveModal({
    super.key,
    required this.app,
    required this.api,
    required this.externalId,
    required this.source,
    required this.mediaType,
    required this.title,
    required this.year,
    required this.season,
    required this.episode,
    required this.onReady,
  });

  @override
  State<ResolveModal> createState() => _ResolveModalState();
}

class _ResolveModalState extends State<ResolveModal> {
  ResolveState _state = const ResolveState(stage: ResolveStage.resolveStart, detail: '正在准备...');
  int? _taskId;
  StreamSubscription<WsEvent>? _wsSub;
  Timer? _pollTimer;

  @override
  void initState() {
    super.initState();
    _start();
  }

  Future<void> _start() async {
    try {
      final r = await widget.api.resolve(
        externalId: widget.externalId,
        source: widget.source,
        mediaType: widget.mediaType,
        title: widget.title,
        year: widget.year,
        season: widget.season,
        episode: widget.episode,
      );
      if (!mounted) return;
      _taskId = r.taskId;
      _listenWs();
      _startPolling();
    } catch (e) {
      if (mounted) {
        setState(() => _state = ResolveState(stage: ResolveStage.error, errorMsg: e.toString()));
      }
    }
  }

  void _listenWs() {
    _wsSub = widget.app.ws?.events.listen((e) {
      final p = e.payload;
      final taskId = (p['task_id'] as num?)?.toInt();
      if (taskId != null && taskId != _taskId) return;
      switch (e.type) {
        case 'resolve_stage':
          if (mounted) {
            setState(() => _state = ResolveState(
                  taskId: _taskId ?? 0,
                  stage: ResolveStage.from(p['stage'] as String?),
                  detail: p['detail'] as String? ?? '',
                  progressPct: (p['progress_pct'] as num?)?.toInt() ?? 0,
                ));
          }
          break;
        case 'resolve_complete':
          final url = p['stream_url'] as String?;
          final source = p['source'] as String? ?? '';
          if (url != null && mounted) _finish(url, source);
          break;
        case 'resolve_failed':
          if (mounted) {
            setState(() => _state = ResolveState(
                  taskId: _taskId ?? 0,
                  stage: ResolveStage.notFound,
                  errorMsg: p['reason'] as String? ?? '暂无可用资源',
                ));
          }
          break;
      }
    });
  }

  void _startPolling() {
    _pollTimer = Timer.periodic(const Duration(milliseconds: 900), (_) async {
      final id = _taskId;
      if (id == null) return;
      try {
        final r = await widget.api.resolveResult(id);
        if (!mounted) return;
        if (r.isSuccess && r.streamUrl != null) {
          _finish(r.streamUrl!, r.source ?? '');
        } else if (r.isTerminal) {
          setState(() => _state = r);
        } else if (_state.stage == ResolveStage.resolveStart) {
          setState(() => _state = r);
        }
      } catch (_) {}
    });
  }

  void _finish(String url, String source) {
    _cleanup();
    widget.onReady(url, source);
  }

  void _cleanup() {
    _wsSub?.cancel();
    _pollTimer?.cancel();
  }

  @override
  void dispose() {
    _cleanup();
    super.dispose();
  }

  /// [V7 §17.5 + §6.3] 当前激活层. 委托给 helper 便于单元测试,
  /// skipNas 由 AppState.capabilities.nasAvailable && nasIndexComplete 计算.
  int _layerIndex() {
    final skipNas = shouldSkipP0(
      nasAvailable: widget.app.capabilities.nasAvailable,
      nasIndexComplete: widget.app.capabilities.nasIndexComplete,
    );
    return resolveLayerForStage(_state.stage, skipNas: skipNas);
  }

  @override
  Widget build(BuildContext context) {
    final layers = ['NAS 本地', '盘搜转存', '磁力下载', '订阅'];
    final li = _layerIndex();
    return Dialog(
      backgroundColor: AppColors.surface,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(AppRadius.xl)),
      child: Container(
        width: 520,
        padding: const EdgeInsets.all(28),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text(widget.title, maxLines: 1, overflow: TextOverflow.ellipsis,
                style: AppTypography.subtitle),
            const SizedBox(height: 20),
            // 四层步骤条 (V7 §17.5 P0 跳过逻辑):
            //   P0 灰显 + 跳过标记: skipNas 时 P0 步骤圈颜色变灰 + 文字"已跳过"
            //   当前层高亮 (active): 用对应层语义色 (resolveP0/P1/P2/P3)
            //   已完成层: success 绿色 ✓
            Row(
              children: List.generate(4, (i) {
                final skipNas = shouldSkipP0(
                  nasAvailable: widget.app.capabilities.nasAvailable,
                  nasIndexComplete: widget.app.capabilities.nasIndexComplete,
                );
                final nasSkipped = i == 0 && skipNas;
                final done = _state.isSuccess || li > i;
                final active = li == i && !nasSkipped;
                // 当前层的语义色 (附录 C.1)
                final layerColor = switch (i) {
                  0 => AppColors.resolveP0,
                  1 => AppColors.resolveP1,
                  2 => AppColors.resolveP2,
                  _ => AppColors.resolveP3,
                };
                return Expanded(
                  child: Column(
                    children: [
                      Row(children: [
                        Expanded(child: Container(height: 3, color: i == 0 ? Colors.transparent : (done || active ? layerColor : AppColors.surfaceHigh))),
                        Container(
                          width: 26, height: 26,
                          decoration: BoxDecoration(
                            shape: BoxShape.circle,
                            color: done
                                ? AppColors.success
                                : nasSkipped
                                    ? AppColors.surfaceHigh
                                    : active
                                        ? layerColor
                                        : AppColors.surfaceHigh,
                            border: nasSkipped ? Border.all(color: AppColors.textMuted.withValues(alpha: 0.3), width: 1) : null,
                          ),
                          child: Center(
                            child: done
                                ? const Icon(Icons.check_rounded, size: 16, color: Colors.white)
                                : nasSkipped
                                    ? Icon(Icons.block, size: 14, color: AppColors.textMuted.withValues(alpha: 0.5))
                                    : Text('${i + 1}', style: TextStyle(fontSize: 12, color: active ? Colors.black : AppColors.textMuted, fontWeight: FontWeight.bold)),
                          ),
                        ),
                        Expanded(child: Container(height: 3, color: i == 3 ? Colors.transparent : (done ? AppColors.success : AppColors.surfaceHigh))),
                      ]),
                      const SizedBox(height: 6),
                      Text(
                        nasSkipped ? '${layers[i]} (跳过)' : layers[i],
                        style: AppTypography.caption.copyWith(
                          color: nasSkipped
                              ? AppColors.textMuted.withValues(alpha: 0.5)
                              : active
                                  ? layerColor
                                  : AppColors.textMuted,
                          decoration: nasSkipped ? TextDecoration.lineThrough : null,
                        ),
                      ),
                    ],
                  ),
                );
              }),
            ),
            const SizedBox(height: 24),
            // 阶段文字 + 进度条
            Row(
              children: [
                if (!_state.isTerminal)
                  SizedBox(width: 18, height: 18, child: CircularProgressIndicator(strokeWidth: 2.5, color: AppColors.accent)),
                const SizedBox(width: 12),
                Expanded(
                  child: Text(
                    _state.isSuccess ? _state.stage.label : (_state.errorMsg?.isNotEmpty == true ? _state.errorMsg! : _state.stage.label),
                    style: AppTypography.body,
                  ),
                ),
                Text('${_state.progressPct}%', style: AppTypography.small),
              ],
            ),
            const SizedBox(height: 10),
            ClipRRect(
              borderRadius: BorderRadius.circular(3),
              child: LinearProgressIndicator(
                value: _state.progressPct / 100,
                // [V7 §17.5] P2 阶段进度条加粗 + 黄色, 视觉强调小时级等待
                minHeight: shouldShowProgressBar(_state.stage) ? 8 : 5,
                backgroundColor: AppColors.surfaceHigh,
                valueColor: AlwaysStoppedAnimation(
                  shouldShowProgressBar(_state.stage) ? AppColors.resolveP2 : AppColors.accent,
                ),
              ),
            ),
            const SizedBox(height: 20),
            if (_state.stage == ResolveStage.notFound) ...[
              const Text('已自动创建订阅，系统将每周搜寻资源', style: AppTypography.small),
              const SizedBox(height: 8),
            ],
            Row(
              children: [
                if (!_state.isTerminal)
                  TextButton.icon(
                    onPressed: _cancel,
                    icon: const Icon(Icons.close_rounded, size: 18),
                    label: const Text('取消'),
                  ),
                if (_state.isTerminal) ...[
                  if (_state.isSuccess)
                    ElevatedButton.icon(
                      onPressed: () => _playDirect(),
                      icon: const Icon(Icons.play_arrow_rounded, size: 18),
                      label: const Text('立即播放'),
                      style: ElevatedButton.styleFrom(
                        backgroundColor: AppColors.accent,
                        foregroundColor: Colors.black,
                      ),
                    ),
                  TextButton.icon(
                    onPressed: () => Navigator.of(context).pop(),
                    icon: const Icon(Icons.refresh_rounded, size: 18),
                    label: Text(_state.isSuccess ? '关闭' : '重试'),
                  ),
                ] else
                  TextButton.icon(
                    onPressed: null,
                    icon: const Icon(Icons.hourglass_empty_rounded, size: 18),
                    label: const Text('后台继续'),
                  ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  void _cancel() {
    _cleanup();
    Navigator.of(context).pop();
  }

  void _playDirect() {
    _cleanup();
    Navigator.of(context).pop();
    widget.onReady(_state.streamUrl ?? '', _state.source ?? '');
  }
}
