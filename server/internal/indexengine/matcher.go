package indexengine

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"xmedia/internal/domain"
	rules "xmedia/internal/filenamerules"
)

// videoExts 索引扫描识别的视频扩展名（§9 NAS 扫描只收录视频文件）。
var videoExts = map[string]bool{
	".mkv": true, ".mp4": true, ".ts": true, ".avi": true, ".rmvb": true,
	".iso": true, ".mov": true, ".wmv": true, ".flv": true, ".m2ts": true,
	".m4v": true, ".webm": true, ".vob": true, ".mpg": true, ".mpeg": true,
}

// IsVideoFile 判断扩展名是否为索引收录的视频格式。
func IsVideoFile(name string) bool {
	return videoExts[strings.ToLower(filepath.Ext(name))]
}

// CleanResult 文件名清洗结果（§9.2 步骤 1：去除编码组/分辨率/来源，提取标题+年份/季集）。
type CleanResult struct {
	Title   string
	Year    int
	Season  int
	Episode int
	Type    string // movie / tv（tv 判定：存在季集信息）
	Quality string // 4K/1080P/720P 等（由 screen_size 派生）
	Format  string // 扩展名（不含点）
}

// CleanFilename 清洗文件名（复用 filenamerules 解析器 + 中文季集/组标签/分辨率兜底）。
func CleanFilename(name string) CleanResult {
	base := strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
	parsed := rules.ParseFilenameWithGuessit(base)
	parsed = rules.ApplyEpisodeFallbacks(base, parsed)

	out := CleanResult{Format: ext}
	if v, ok := parsed["title"].(string); ok {
		out.Title = strings.TrimSpace(v)
	}
	if v, ok := parsed["year"].(int); ok {
		out.Year = v
	}
	if v, ok := parsed["season"].(int); ok {
		out.Season = v
	}
	if v, ok := parsed["episode"]; ok {
		out.Episode = asInt(v)
	}
	// 中文季集兜底（guessit 不支持"第N季/第N集"，含汉字数字）
	if m := chineseSeasonRe.FindStringSubmatch(base); m != nil {
		out.Season = asChineseNumber(m[1])
		out.Title = strings.TrimSpace(chineseSeasonRe.ReplaceAllString(out.Title, ""))
	}
	if m := chineseEpisodeRe.FindStringSubmatch(base); m != nil {
		out.Episode = asChineseNumber(m[1])
		out.Title = strings.TrimSpace(chineseEpisodeRe.ReplaceAllString(out.Title, ""))
	}
	// 组标签剥离（[VCB-Studio] 等）
	out.Title = strings.TrimSpace(bracketGroupRe.ReplaceAllString(out.Title, ""))
	out.Title = strings.Join(strings.Fields(out.Title), " ")

	out.Type = "movie"
	if out.Season > 0 || out.Episode > 0 {
		out.Type = "tv"
	}
	if s, ok := parsed["screen_size"].(string); ok {
		out.Quality = normalizeQuality(s)
	}
	if out.Quality == "" {
		out.Quality = normalizeQuality(qualityFromNameRe.FindString(base))
	}
	return out
}

var (
	chineseSeasonRe   = regexp.MustCompile(`第\s*(\d+|[一二三四五六七八九十]+)\s*季`)
	chineseEpisodeRe  = regexp.MustCompile(`第\s*(\d+|[一二三四五六七八九十]+)\s*集`)
	bracketGroupRe    = regexp.MustCompile(`\[[^\]]*\]`)
	qualityFromNameRe = regexp.MustCompile(`(?i)(2160p|1080p|720p|4k|uhd)`)
)

// chineseNumberToInt 汉字数字 -> int（支持 1-99）。
func chineseNumberToInt(s string) int {
	single := map[rune]int{
		'一': 1, '二': 2, '三': 3, '四': 4, '五': 5,
		'六': 6, '七': 7, '八': 8, '九': 9, '十': 10,
	}
	if len(s) == 1 {
		return single[[]rune(s)[0]]
	}
	runes := []rune(s)
	total := 0
	for i, r := range runes {
		switch r {
		case '十':
			if i == 0 {
				total = 10
			} else {
				total *= 10
			}
		default:
			total += single[r]
		}
	}
	return total
}

// asChineseNumber 把（可能为汉字的）数字文本转为 int。
func asChineseNumber(s string) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return chineseNumberToInt(s)
}

func asInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case string:
		if i, err := strconv.Atoi(strings.TrimSpace(n)); err == nil {
			return i
		}
		if p := rules.ParseEpisodeNumber(n); p != nil {
			return *p
		}
		return 0
	default:
		return 0
	}
}

func normalizeQuality(screen string) string {
	s := strings.ToUpper(strings.TrimSpace(screen))
	switch {
	case strings.Contains(s, "2160"), strings.Contains(s, "4K"), strings.Contains(s, "UHD"):
		return "4K"
	case strings.Contains(s, "1080"):
		return "1080P"
	case strings.Contains(s, "720"):
		return "720P"
	default:
		return ""
	}
}

// MatchResult 匹配结果。
type MatchResult struct {
	Media  *domain.MediaLibrary
	Score  float64
	Status domain.MatchStatus
}

// MatchTitle 把清洗结果与 media_library 候选比对，计算匹配分（§9.2 步骤 2/3）。
// 标题完全相同 = 1.0；单向包含 = 0.85；否则 0.5 基准，年份不匹配扣 0.2。
func MatchTitle(clean CleanResult, candidates []*domain.MediaLibrary) MatchResult {
	best := MatchResult{Score: 0, Status: domain.MatchOrphaned}
	normTitle := normalizeTitle(clean.Title)
	for _, m := range candidates {
		score := titleScore(normTitle, m)
		if clean.Year > 0 && m.Year > 0 && clean.Year != m.Year {
			score -= 0.2
		}
		if score > best.Score {
			best = MatchResult{Media: m, Score: score}
		}
	}
	if best.Score < 0 {
		best.Score = 0
	}
	switch {
	case best.Score >= 0.85:
		best.Status = domain.MatchMatched
	case best.Score >= 0.6:
		best.Status = domain.MatchUnconfirmed
	default:
		best.Status = domain.MatchOrphaned
	}
	return best
}

func titleScore(normClean string, m *domain.MediaLibrary) float64 {
	if normClean == "" || m == nil {
		return 0
	}
	normTitle := normalizeTitle(m.Title)
	normOrig := normalizeTitle(m.TitleOrig)
	switch {
	case normTitle != "" && normClean == normTitle:
		return 1.0
	case normOrig != "" && normClean == normOrig:
		return 1.0
	case normTitle != "" && (strings.Contains(normTitle, normClean) || strings.Contains(normClean, normTitle)):
		return 0.85
	case normOrig != "" && (strings.Contains(normOrig, normClean) || strings.Contains(normClean, normOrig)):
		return 0.85
	default:
		return 0.5
	}
}

// normalizeTitle 归一化标题用于比较：小写、去空格与标点。
func normalizeTitle(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r >= '\u4e00' && r <= '\u9fff':
			b.WriteRune(r)
		}
	}
	return b.String()
}
