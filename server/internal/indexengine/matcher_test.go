package indexengine

import (
	"strings"
	"testing"

	"xmedia/internal/domain"
)

// TestCleanFilenameSamples 文件名清洗器代表性样本（§23.1 Phase 3 验收：准确率 >90%）。
func TestCleanFilenameSamples(t *testing.T) {
	cases := []struct {
		name    string
		title   string
		year    int
		season  int
		episode int
		typ     string
	}{
		{"阿凡达.2009.1080p.BluRay.x264.mkv", "阿凡达", 2009, 0, 0, "movie"},
		{"Avatar.2009.2160p.UHD.BluRay.DTS.mkv", "Avatar", 2009, 0, 0, "movie"},
		{"权力的游戏.S01E03.1080p.mkv", "权力的游戏", 0, 1, 3, "tv"},
		{"Game.of.Thrones.S02E05.720p.mkv", "Game of Thrones", 0, 2, 5, "tv"},
		{"進擊的巨人.第三季.第10集.mp4", "進擊的巨人", 0, 3, 10, "tv"},
		{"流浪地球2.2023.4K.WEB-DL.mp4", "流浪地球2", 2023, 0, 0, "movie"},
		{"老友记.Friends.S10E18.DVDrip.avi", "", 0, 10, 18, "tv"},
		{"[VCB-Studio] Fate Stay Night [10][1080p].mkv", "Fate Stay Night", 0, 0, 10, "tv"},
		{"让子弹飞.2010.BluRay.1080p.x265.ts", "让子弹飞", 2010, 0, 0, "movie"},
		{"The.Mandalorian.S03E01.Chapter.17.4K.DV.mkv", "The Mandalorian", 0, 3, 1, "tv"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CleanFilename(tc.name)
			if got.Title == "" {
				t.Fatalf("标题清洗失败: %q", tc.name)
			}
			if tc.title != "" && got.Title != tc.title && !strings.Contains(got.Title, tc.title) {
				t.Errorf("标题 = %q, want %q (文件 %q)", got.Title, tc.title, tc.name)
			}
			if tc.year != 0 && got.Year != tc.year {
				t.Errorf("年份 = %d, want %d (文件 %q)", got.Year, tc.year, tc.name)
			}
			if tc.season != 0 && got.Season != tc.season {
				t.Errorf("季 = %d, want %d (文件 %q)", got.Season, tc.season, tc.name)
			}
			if tc.episode != 0 && got.Episode != tc.episode {
				t.Errorf("集 = %d, want %d (文件 %q)", got.Episode, tc.episode, tc.name)
			}
			if tc.typ != "" && got.Type != tc.typ {
				t.Errorf("类型 = %q, want %q (文件 %q)", got.Type, tc.typ, tc.name)
			}
		})
	}
}

// TestCleanFilenameFormatAndQuality 扩展名与画质派生。
func TestCleanFilenameFormatAndQuality(t *testing.T) {
	got := CleanFilename("Avatar.2009.2160p.UHD.BluRay.mkv")
	if got.Format != "mkv" {
		t.Fatalf("格式 = %q, want mkv", got.Format)
	}
	if got.Quality != "4K" {
		t.Fatalf("画质 = %q, want 4K", got.Quality)
	}
	if got := CleanFilename("sample.1080p.mp4").Quality; got != "1080P" {
		t.Fatalf("1080P 画质 = %q", got)
	}
}

// TestMatchTitleScores 匹配评分三档边界（matched/unconfirmed/orphaned 阈值 0.85/0.6）。
func TestMatchTitleScores(t *testing.T) {
	lib := []*domain.MediaLibrary{
		{Title: "阿凡达", TitleOrig: "Avatar", Year: 2009, ExternalID: 19995},
		{Title: "权力的游戏", TitleOrig: "Game of Thrones", Year: 2011, ExternalID: 1399},
	}

	exact := MatchTitle(CleanResult{Title: "阿凡达", Year: 2009}, lib)
	if exact.Status != domain.MatchMatched || exact.Media.ExternalID != 19995 || exact.Score < 0.85 {
		t.Fatalf("完全匹配失败: %#v", exact)
	}

	orig := MatchTitle(CleanResult{Title: "Avatar", Year: 2009}, lib)
	if orig.Status != domain.MatchMatched || orig.Media.ExternalID != 19995 {
		t.Fatalf("原文匹配失败: %#v", orig)
	}

	contains := MatchTitle(CleanResult{Title: "权力的游戏 S01E03"}, lib)
	if contains.Status != domain.MatchMatched || contains.Media.ExternalID != 1399 {
		t.Fatalf("包含匹配失败: %#v", contains)
	}

	yearMismatch := MatchTitle(CleanResult{Title: "阿凡达", Year: 2015}, lib)
	if yearMismatch.Score >= 0.85 || yearMismatch.Media == nil {
		t.Fatalf("年份不匹配应降档: %#v", yearMismatch)
	}

	miss := MatchTitle(CleanResult{Title: "不存在的影片XYZ"}, lib)
	if miss.Status != domain.MatchOrphaned {
		t.Fatalf("未命中应判 orphaned: %#v", miss)
	}
}

// TestIsVideoFile 扩展名过滤。
func TestIsVideoFile(t *testing.T) {
	if !IsVideoFile("a.mkv") || !IsVideoFile("b.MP4") || !IsVideoFile("c.m2ts") {
		t.Fatalf("视频扩展名应通过")
	}
	if IsVideoFile("a.txt") || IsVideoFile("b.nfo") || IsVideoFile("c.srt") || IsVideoFile("d.jpg") {
		t.Fatalf("非视频扩展名应拒绝")
	}
}
