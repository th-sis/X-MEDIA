package pansearch

import (
	"testing"

	"xmedia/internal/domain"
)

func TestSortResultsCamBlocked(t *testing.T) {
	results := []domain.PanSearchResult{
		{Title: "枪版", Source: "quark", Quality: "CAM", Datetime: "2026-08-01T00:00:00Z"},
		{Title: "1080", Source: "quark", Quality: "1080P", Datetime: "2026-08-02T00:00:00Z"},
	}
	got := SortResults(results, []string{"pan115", "quark"}, true)
	if len(got) != 1 || got[0].Title != "1080" {
		t.Fatalf("CAM 应被排除: %#v", got)
	}
}

func TestSortResultsCamAllowed(t *testing.T) {
	results := []domain.PanSearchResult{
		{Title: "枪版", Source: "quark", Quality: "CAM"},
	}
	got := SortResults(results, nil, false)
	if len(got) != 1 {
		t.Fatalf("关闭过滤时 CAM 应保留: %#v", got)
	}
}

func TestSortResultsQualityThenTimeThenPriority(t *testing.T) {
	results := []domain.PanSearchResult{
		{Title: "quark-720-new", Source: "quark", Quality: "720P", Datetime: "2026-08-10T00:00:00Z"},
		{Title: "quark-1080-old", Source: "quark", Quality: "1080P", Datetime: "2026-01-01T00:00:00Z"},
		{Title: "pan115-1080", Source: "pan115", Quality: "1080P", Datetime: "2026-01-01T00:00:00Z"},
		{Title: "quark-4k", Source: "quark", Quality: "4K", Datetime: "2026-02-01T00:00:00Z"},
	}
	got := SortResults(results, []string{"pan115", "quark"}, true)
	want := []string{"pan115-1080", "quark-4k", "quark-1080-old", "quark-720-new"}
	for i := range want {
		if got[i].Title != want[i] {
			t.Fatalf("排序[%d] = %q, want %q（全序: %#v）", i, got[i].Title, want[i], got)
		}
	}
}

func TestSortResultsUnknownPriorityGoesLast(t *testing.T) {
	results := []domain.PanSearchResult{
		{Title: "unknown-source", Source: "magnet", Quality: "4K"},
		{Title: "known-source", Source: "quark", Quality: "720P"},
	}
	got := SortResults(results, []string{"quark"}, true)
	if got[0].Title != "known-source" || got[1].Title != "unknown-source" {
		t.Fatalf("未登录网盘应排最后: %#v", got)
	}
}

func TestQualityRank(t *testing.T) {
	cases := map[string]int{"4K": 0, "1080P": 1, "720P": 2, "": 3, "CAM": 3}
	for in, want := range cases {
		if got := qualityRank(in); got != want {
			t.Fatalf("qualityRank(%q) = %d, want %d", in, got, want)
		}
	}
}
