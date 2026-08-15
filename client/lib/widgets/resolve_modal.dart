import 'dart:async';
import 'package:flutter/material.dart';
import '../models/media.dart';
import '../services/api_client.dart';
import '../services/app_state.dart';
import '../services/ws_client.dart';
import '../theme/app_theme.dart';

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

  int _layerIndex() {
    switch (_state.stage) {
      case ResolveStage.resolveStart:
      case ResolveStage.nasLookup:
      case ResolveStage.nasHit:
        return 0;
      case ResolveStage.panSearching:
      case ResolveStage.panSearched:
      case ResolveStage.transferring:
      case ResolveStage.resolvingLink:
        return 1;
      case ResolveStage.magnetDownloading:
        return 2;
      case ResolveStage.notFound:
        return 3;
      case ResolveStage.playReady:
        return 4;
      case ResolveStage.error:
        return 4;
    }
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
                style: const TextStyle(fontSize: 17, fontWeight: FontWeight.w700, color: AppColors.textPrimary)),
            const SizedBox(height: 20),
            // 四层步骤条
            Row(
              children: List.generate(4, (i) {
                final done = _state.isSuccess || li > i;
                final active = li == i;
                return Expanded(
                  child: Column(
                    children: [
                      Row(children: [
                        Expanded(child: Container(height: 3, color: i == 0 ? Colors.transparent : (done || active ? AppColors.accent : AppColors.surfaceHigh))),
                        Container(
                          width: 26, height: 26,
                          decoration: BoxDecoration(
                            shape: BoxShape.circle,
                            color: done ? AppColors.success : active ? AppColors.accent : AppColors.surfaceHigh,
                          ),
                          child: Center(
                            child: done
                                ? const Icon(Icons.check_rounded, size: 16, color: Colors.white)
                                : Text('${i + 1}', style: TextStyle(fontSize: 12, color: active ? Colors.black : AppColors.textMuted, fontWeight: FontWeight.bold)),
                          ),
                        ),
                        Expanded(child: Container(height: 3, color: i == 3 ? Colors.transparent : (done ? AppColors.success : AppColors.surfaceHigh))),
                      ]),
                      const SizedBox(height: 6),
                      Text(layers[i], style: TextStyle(fontSize: 11, color: active ? AppColors.accent : AppColors.textMuted)),
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
                    style: const TextStyle(fontSize: 14, color: AppColors.textPrimary),
                  ),
                ),
                Text('${_state.progressPct}%', style: const TextStyle(fontSize: 13, color: AppColors.textMuted)),
              ],
            ),
            const SizedBox(height: 10),
            ClipRRect(
              borderRadius: BorderRadius.circular(3),
              child: LinearProgressIndicator(
                value: _state.progressPct / 100,
                minHeight: 5,
                backgroundColor: AppColors.surfaceHigh,
                valueColor: const AlwaysStoppedAnimation(AppColors.accent),
              ),
            ),
            const SizedBox(height: 20),
            if (_state.stage == ResolveStage.notFound) ...[
              Text('已自动创建订阅，系统将每周搜寻资源', style: TextStyle(fontSize: 12, color: AppColors.textSecondary)),
              const SizedBox(height: 8),
            ],
            TextButton(
              onPressed: () => Navigator.of(context).pop(),
              child: Text(_state.isTerminal ? '关闭' : '后台继续', style: const TextStyle(color: AppColors.textSecondary)),
            ),
          ],
        ),
      ),
    );
  }
}
