package resolve

import (
	"fmt"

	"xmedia/internal/domain"
)

// buildSearchKeywords 构造多语言关键词回退链（§6.7，v7 D46）：
// 主关键词（中文 + 季集）→ 原文（英文/罗马音）→ 中英混合。
func buildSearchKeywords(task *domain.ResolveTask, media *domain.MediaLibrary) []string {
	var keywords []string

	primary := task.Title
	orig := ""
	if media != nil {
		orig = media.TitleOrig
	}
	if task.MediaType == "tv" && task.Season > 0 {
		if task.Episode > 0 {
			primary = fmt.Sprintf("%s S%02dE%02d", task.Title, task.Season, task.Episode)
		} else {
			primary = fmt.Sprintf("%s S%02d", task.Title, task.Season)
		}
	}
	keywords = append(keywords, primary)

	if orig != "" && orig != task.Title {
		o := orig
		if task.MediaType == "tv" && task.Season > 0 {
			if task.Episode > 0 {
				o = fmt.Sprintf("%s S%02dE%02d", orig, task.Season, task.Episode)
			} else {
				o = fmt.Sprintf("%s S%02d", orig, task.Season)
			}
		}
		keywords = append(keywords, o)

		mixed := fmt.Sprintf("%s %s", task.Title, orig)
		if task.MediaType == "tv" && task.Season > 0 {
			if task.Episode > 0 {
				mixed = fmt.Sprintf("%s %s S%02dE%02d", task.Title, orig, task.Season, task.Episode)
			} else {
				mixed = fmt.Sprintf("%s %s S%02d", task.Title, orig, task.Season)
			}
		}
		keywords = append(keywords, mixed)
	}
	return keywords
}

// buildMagnetKeyword 构造 P2 磁力关键词（§6.4）：去掉季集信息 + 加"磁力 高清"后缀。
func buildMagnetKeyword(t *domain.ResolveTask) string {
	base := t.Title
	if base == "" {
		base = "unknown"
	}
	return base + " 磁力 高清"
}
