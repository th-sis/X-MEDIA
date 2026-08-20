package resolve

import (
	"context"
	"log/slog"
	"strconv"
	"strings"

	"xmedia/internal/domain"
	"xmedia/internal/driver"
	rules "xmedia/internal/filenamerules"
)

// applyPostSaveRename [V7 §6.9.2] 转存成功后, 把文件名改写为统一模板:
//
//	电影: {title} ({year}) {tmdb-XXX}.{ext}
//	剧集: {title} S{ss}E{ee} {tmdb-XXX}.{ext}
//
// 配置项 pan_rename_enabled 控制是否启用 (默认 false, 显式开启后才生效,
// 避免破坏既有转存结果 — 这是个不可逆操作, 旧文件名换不回来).
//
// 冲突 fallback: 失败时按 target, target__v2, target__v3 三个变体依次重试.
// 全部失败: warning log + 保留原 FileName (不阻塞转存结果, 不影响 P1 命中).
//
// 元数据补全: 转存后的文件名常带广告/分辨率/组名, ParseFilenameStrict 可能
// 解析不出 title/year. 优先用 task 已知元数据 (来自 TMDB) 补全.
func (s *Service) applyPostSaveRename(
	ctx context.Context,
	drv driver.Driver,
	t *domain.ResolveTask,
	saved *driver.ShareResult,
) {
	if s == nil || saved == nil || drv == nil || t == nil {
		return
	}
	if s.configs == nil {
		return
	}

	// 1. 配置项开关: pan_rename_enabled
	raw, ok, err := s.configs.Get(ctx, domain.ConfigPanRenameEnabled)
	if err != nil || !ok || !isTruthy(raw) {
		return
	}

	if saved.FileID == "" || saved.FileName == "" {
		return
	}

	// 2. 提取扩展名 (小写, 不含点)
	ext := rules.FileExtension(saved.FileName)
	if ext == "" {
		return // 无扩展名, 不重命名 (避免破坏目录)
	}
	if !isVideoExtension(ext) {
		return // 只重命名视频文件
	}

	// 3. 解析文件名, 优先用 task 元数据覆盖 (转存文件名常含广告/分辨率/组名等噪声,
//    解析结果不可靠; task 元数据来自 TMDB, 是更权威的来源).
	parsed := rules.ParseFilenameStrict(saved.FileName)
	if strings.TrimSpace(t.Title) != "" {
		parsed.Title = t.Title
	}
	if t.Year > 0 {
		y := t.Year
		parsed.Year = &y
	}
	if t.Season > 0 {
		sn := t.Season
		parsed.Season = &sn
	}
	if t.Episode > 0 {
		ep := t.Episode
		parsed.Episode = &ep
	}

	// 4. 生成目标名 (BuildTargetFilename 不带扩展名, 我们补上)
	tmdbID := strconv.FormatInt(t.ExternalID, 10)
	target := strings.TrimSpace(rules.BuildTargetFilename(parsed, "tmdbid", tmdbID))
	if target == "" {
		return // 信息不足, 跳过
	}

	fullName := target + "." + ext
	fullName = rules.SanitizeFilename(fullName)
	fullName = rules.FitFilenameBytes(fullName, "")

	if rules.IsSameGeneratedName(saved.FileName, fullName) {
		return // 已经符合模板
	}

	// 5. 冲突 fallback: target, target__v2, target__v3
	renamer, ok := drv.(driver.Renamer)
	if !ok {
		slog.Default().Debug("post-save rename skipped: driver not implement Renamer",
			"file_id", saved.FileID, "driver", drv.Config().Name)
		return
	}

	candidates := []string{
		fullName,
		appendVariant(fullName, "__v2"),
		appendVariant(fullName, "__v3"),
	}
	var errs []string
	for _, name := range candidates {
		if name == saved.FileName {
			continue
		}
		if err := renamer.RenameFile(ctx, saved.FileID, name); err == nil {
			slog.Default().Info("post-save rename ok",
				"file_id", saved.FileID,
				"old", saved.FileName,
				"new", name)
			saved.FileName = name
			return
		} else {
			errs = append(errs, name+": "+err.Error())
		}
	}
	slog.Default().Warn("post-save rename failed (all variants)",
		"file_id", saved.FileID,
		"target", fullName,
		"errors", strings.Join(errs, " | "))
}

// appendVariant 把文件名最后一个 . 之前插入 suffix (如 __v2).
// 找不到 . 时直接后缀追加.
func appendVariant(name, suffix string) string {
	dot := strings.LastIndex(name, ".")
	if dot < 0 {
		return name + suffix
	}
	return name[:dot] + suffix + name[dot:]
}

// isTruthy 解析配置开关: "1", "true", "yes", "on", "y" (大小写不敏感) 视为真.
func isTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on", "y":
		return true
	}
	return false
}

// isVideoExtension 判断是否为可重命名的视频扩展名.
// 与 CacheRetention 的扩展名集合对齐 (避免误改 nfo / jpg / 字幕 等元数据).
func isVideoExtension(ext string) bool {
	switch strings.ToLower(strings.TrimSpace(ext)) {
	case "mp4", "mkv", "avi", "mov", "wmv", "flv", "webm", "ts", "m2ts", "m4v", "rmvb", "iso":
		return true
	}
	return false
}
