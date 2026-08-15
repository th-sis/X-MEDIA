import 'dart:async';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:video_player/video_player.dart';
import '../services/app_state.dart';
import '../theme/app_theme.dart';

class PlayerPage extends StatefulWidget {
  final String streamUrl;
  final String title;
  final int externalId;
  final String source;
  final String mediaType;
  final String posterUrl;
  final int season;
  final int episode;
  final String playSource;

  const PlayerPage({
    super.key,
    required this.streamUrl,
    required this.title,
    required this.externalId,
    this.source = 'tmdb',
    this.mediaType = 'movie',
    this.posterUrl = '',
    this.season = 0,
    this.episode = 0,
    this.playSource = '',
  });

  @override
  State<PlayerPage> createState() => _PlayerPageState();
}

class _PlayerPageState extends State<PlayerPage> with WidgetsBindingObserver {
  VideoPlayerController? _controller;
  bool _ready = false;
  bool _error = false;
  bool _controlsVisible = true;
  Timer? _hideTimer;
  Timer? _progressTimer;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _init();
  }

  Future<void> _init() async {
    final url = context.read<AppState>().api.absolute(widget.streamUrl);
    final controller = VideoPlayerController.networkUrl(Uri.parse(url));
    _controller = controller;
    try {
      await controller.initialize();
      if (!mounted) return;
      setState(() => _ready = true);
      controller.play();
      _progressTimer = Timer.periodic(const Duration(seconds: 10), (_) => _report());
      _scheduleHide();
    } catch (_) {
      if (mounted) setState(() => _error = true);
    }
  }

  void _report() {
    final c = _controller;
    if (c == null || !c.value.isInitialized) return;
    context.read<AppState>().api.reportProgress(
          externalId: widget.externalId,
          source: widget.source,
          mediaType: widget.mediaType,
          title: widget.title,
          posterUrl: widget.posterUrl,
          sourceType: widget.playSource,
          season: widget.season,
          episode: widget.episode,
          positionMs: c.value.position.inMilliseconds,
          durationMs: c.value.duration.inMilliseconds,
        );
  }

  void _scheduleHide() {
    _hideTimer?.cancel();
    _hideTimer = Timer(const Duration(seconds: 4), () {
      if (mounted) setState(() => _controlsVisible = false);
    });
  }

  void _toggleControls() {
    setState(() => _controlsVisible = !_controlsVisible);
    if (_controlsVisible) _scheduleHide();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    final c = _controller;
    if (c == null) return;
    if (state == AppLifecycleState.paused) {
      c.pause();
      _report();
    } else if (state == AppLifecycleState.resumed) {
      c.play();
    }
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    _progressTimer?.cancel();
    _hideTimer?.cancel();
    _controller?.dispose();
    super.dispose();
  }

  String _fmt(Duration d) {
    final h = d.inHours;
    final m = d.inMinutes.remainder(60);
    final s = d.inSeconds.remainder(60);
    if (h > 0) return '${h.toString().padLeft(2, '0')}:${m.toString().padLeft(2, '0')}:${s.toString().padLeft(2, '0')}';
    return '${m.toString().padLeft(2, '0')}:${s.toString().padLeft(2, '0')}';
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.black,
      body: GestureDetector(
        behavior: HitTestBehavior.opaque,
        onTap: _toggleControls,
        child: Stack(
          fit: StackFit.expand,
          children: [
            if (_error)
              const Center(child: Text('播放失败', style: TextStyle(color: AppColors.textSecondary, fontSize: 16)))
            else if (_ready && _controller != null)
              Center(child: AspectRatio(aspectRatio: _controller!.value.aspectRatio, child: VideoPlayer(_controller!)))
            else
              const Center(child: CircularProgressIndicator()),
            // 控制层
            AnimatedOpacity(
              opacity: _controlsVisible ? 1 : 0,
              duration: AppDuration.normal,
              child: _controls(),
            ),
          ],
        ),
      ),
    );
  }

  Widget _controls() {
    final c = _controller;
    final pos = c != null && c.value.isInitialized ? c.value.position : Duration.zero;
    final dur = c != null && c.value.isInitialized ? c.value.duration : Duration.zero;
    return Column(
      children: [
        // 顶栏
        Container(
          color: Colors.black.withValues(alpha: 0.6),
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
          child: Row(
            children: [
              IconButton(icon: const Icon(Icons.arrow_back_rounded, color: Colors.white), onPressed: () => Navigator.of(context).pop()),
              const SizedBox(width: 8),
              Expanded(child: Text(widget.title, maxLines: 1, overflow: TextOverflow.ellipsis,
                  style: const TextStyle(color: Colors.white, fontSize: 16, fontWeight: FontWeight.w600))),
            ],
          ),
        ),
        const Spacer(),
        // 底栏
        Container(
          color: Colors.black.withValues(alpha: 0.6),
          padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 12),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              if (c != null && c.value.isInitialized)
                Slider(
                  value: dur.inMilliseconds > 0 ? pos.inMilliseconds.clamp(0, dur.inMilliseconds).toDouble() : 0,
                  max: dur.inMilliseconds > 0 ? dur.inMilliseconds.toDouble() : 1,
                  activeColor: AppColors.accent,
                  onChanged: (v) => c.seekTo(Duration(milliseconds: v.toInt())),
                ),
              Row(
                children: [
                  IconButton(
                    icon: Icon(c != null && c.value.isPlaying ? Icons.pause_rounded : Icons.play_arrow_rounded, color: Colors.white, size: 32),
                    onPressed: () => c != null && c.value.isPlaying ? c.pause() : c?.play(),
                  ),
                  const SizedBox(width: 12),
                  Text('${_fmt(pos)} / ${_fmt(dur)}', style: const TextStyle(color: Colors.white70, fontSize: 13)),
                  const Spacer(),
                ],
              ),
            ],
          ),
        ),
      ],
    );
  }
}
