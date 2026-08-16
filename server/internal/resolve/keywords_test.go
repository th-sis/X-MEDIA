package resolve

import (
	"testing"

	"xmedia/internal/domain"
)

func TestBuildSearchKeywordsMovieWithOrig(t *testing.T) {
	task := &domain.ResolveTask{MediaType: "movie", Title: "阿凡达", Season: 0, Episode: 0}
	media := &domain.MediaLibrary{TitleOrig: "Avatar"}
	got := buildSearchKeywords(task, media)
	want := []string{"阿凡达", "Avatar", "阿凡达 Avatar"}
	if len(got) != len(want) {
		t.Fatalf("关键词数 = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("关键词[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestBuildSearchKeywordsMovieNoOrig(t *testing.T) {
	task := &domain.ResolveTask{MediaType: "movie", Title: "阿凡达"}
	got := buildSearchKeywords(task, nil)
	if len(got) != 1 || got[0] != "阿凡达" {
		t.Fatalf("关键词 = %#v, want [阿凡达]", got)
	}
}

func TestBuildSearchKeywordsEpisode(t *testing.T) {
	task := &domain.ResolveTask{MediaType: "tv", Title: "权力的游戏", Season: 1, Episode: 3}
	media := &domain.MediaLibrary{TitleOrig: "Game of Thrones"}
	got := buildSearchKeywords(task, media)
	want := []string{
		"权力的游戏 S01E03",
		"Game of Thrones S01E03",
		"权力的游戏 Game of Thrones S01E03",
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("关键词[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestBuildSearchKeywordsSeasonOnly(t *testing.T) {
	task := &domain.ResolveTask{MediaType: "tv", Title: "权力的游戏", Season: 2, Episode: 0}
	got := buildSearchKeywords(task, nil)
	if got[0] != "权力的游戏 S02" {
		t.Fatalf("关键词[0] = %q, want 权力的游戏 S02", got[0])
	}
}

func TestBuildSearchKeywordsSameOrigSkipped(t *testing.T) {
	task := &domain.ResolveTask{MediaType: "movie", Title: "Avatar"}
	media := &domain.MediaLibrary{TitleOrig: "Avatar"}
	got := buildSearchKeywords(task, media)
	if len(got) != 1 || got[0] != "Avatar" {
		t.Fatalf("中英一致时不应产生回退关键词: %#v", got)
	}
}

func TestBuildMagnetKeyword(t *testing.T) {
	task := &domain.ResolveTask{MediaType: "tv", Title: "权力的游戏", Season: 1, Episode: 3}
	if got := buildMagnetKeyword(task); got != "权力的游戏 磁力 高清" {
		t.Fatalf("磁力关键词 = %q", got)
	}
	if got := buildMagnetKeyword(&domain.ResolveTask{}); got != "unknown 磁力 高清" {
		t.Fatalf("空标题磁力关键词 = %q", got)
	}
}

func TestParsePriorityList(t *testing.T) {
	got := parsePriorityList(`["nas","pan115","quark"]`)
	if len(got) != 3 || got[0] != "nas" || got[2] != "quark" {
		t.Fatalf("解析结果 = %#v", got)
	}
	if got := parsePriorityList(""); got != nil {
		t.Fatalf("空串应返回 nil: %#v", got)
	}
}

func TestPansearchCloudTypes(t *testing.T) {
	got := pansearchCloudTypes([]string{"pan115", "quark", "pan123", "baidu"})
	want := []string{"115", "quark", "123", "baidu"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("cloud_types[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestDriverSourceOf(t *testing.T) {
	cases := map[string]string{
		"115_open": "pan115",
		"115":      "pan115",
		"Quark":    "quark",
		"123_Open": "pan123",
		"baidu":    "baidu",
		"localfs":  "nas",
	}
	for in, want := range cases {
		if got := driverSourceOf(in); got != want {
			t.Fatalf("driverSourceOf(%q) = %q, want %q", in, got, want)
		}
	}
}
