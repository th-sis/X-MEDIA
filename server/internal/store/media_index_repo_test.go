package store_test

import (
	"context"
	"testing"
	"time"

	"xmedia/internal/domain"
	"xmedia/internal/store"
)

// TestMediaIndexUpsertRoundTrip [B 实测回归] 真实 SQL 往返：
// 捕获 mediaIndexRepo.Upsert 占位符与参数数量不匹配类 bug
// （2026-08-16 实测发现：INSERT 19 占位符缺传 last_played_at 参数）。
func TestMediaIndexUpsertRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	lastPlayed := time.Now().Add(-time.Hour)
	m := &domain.MediaIndex{
		ExternalID:     19995,
		ExternalSource: "tmdb",
		Season:         1,
		Episode:        3,
		MediaType:      "tv",
		Title:          "权力的游戏",
		OriginalName:   "Game of Thrones",
		Year:           2011,
		SourceType:     "nas",
		AccountID:      0,
		FilePath:       "tv/权力的游戏.S01E03.mkv",
		FileID:         "nas-file-1",
		FileSize:       1024,
		FileFormat:     "mkv",
		MatchStatus:    domain.MatchMatched,
		MatchScore:     1.0,
		StreamURL:      "",
		LastPlayedAt:   &lastPlayed,
	}
	id, err := s.MediaIndex.Upsert(ctx, m)
	if err != nil {
		t.Fatalf("Upsert 失败（占位符/参数不匹配类 bug）: %v", err)
	}
	if id == 0 {
		t.Fatalf("Upsert 应返回有效 id")
	}
	got, err := s.MediaIndex.FindBest(ctx, 19995, "tmdb", 1, 3)
	if err != nil {
		t.Fatalf("FindBest 失败: %v", err)
	}
	if got.Title != "权力的游戏" || got.FilePath != "tv/权力的游戏.S01E03.mkv" {
		t.Fatalf("读回内容不匹配: %#v", got)
	}
	if got.MatchStatus != domain.MatchMatched || got.MatchScore != 1.0 {
		t.Fatalf("匹配状态/分数不匹配: %#v", got)
	}
	if got.LastPlayedAt == nil || !got.LastPlayedAt.Equal(lastPlayed.Truncate(time.Second)) {
		t.Fatalf("last_played_at 往返失败: %v", got.LastPlayedAt)
	}

	// Upsert 冲突更新路径（同 source_type+file_path）
	m.MatchScore = 0.85
	if _, err := s.MediaIndex.Upsert(ctx, m); err != nil {
		t.Fatalf("冲突更新失败: %v", err)
	}
	n, err := s.MediaIndex.Count(ctx)
	if err != nil || n != 1 {
		t.Fatalf("冲突更新后应仍 1 行: n=%d err=%v", n, err)
	}
}

// TestMediaIndexPhaseC [B 实测回归] ListUnconfirmedBefore/MarkOrphaned 真实 SQL。
func TestMediaIndexPhaseC(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	_, err := s.MediaIndex.Upsert(ctx, &domain.MediaIndex{
		ExternalSource: "tmdb", MediaType: "movie", Title: "旧条目",
		SourceType: "nas", FilePath: "old.mkv", MatchStatus: domain.MatchUnconfirmed,
	})
	if err != nil {
		t.Fatalf("Upsert 失败: %v", err)
	}
	rows, err := s.MediaIndex.ListUnconfirmedBefore(ctx, time.Now().Add(time.Second))
	if err != nil {
		t.Fatalf("ListUnconfirmedBefore 失败: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("应返回 1 条 unconfirmed: %d", len(rows))
	}
	if err := s.MediaIndex.MarkOrphaned(ctx, []int64{rows[0].ID}); err != nil {
		t.Fatalf("MarkOrphaned 失败: %v", err)
	}
	all, _ := s.MediaIndex.ListBySource(ctx, "nas", 0)
	if len(all) != 1 || all[0].MatchStatus != domain.MatchOrphaned {
		t.Fatalf("标记后状态应 orphaned: %#v", all)
	}
}

// 确保 store import 被使用（显式类型引用）。
var _ = store.Options{}
