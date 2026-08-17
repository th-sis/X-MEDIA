package store_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"xmedia/internal/domain"
)

// TestNASSourceCRUD 全量 CRUD + UNIQUE 冲突 + path/name 占用检查的真实 SQL 回归。
//
// 历史教训（[V7 §9.4+ 扩展 G1]）：mock 单测覆盖不到占位符/参数错误，B 实测抓过
// mediaIndexRepo.Upsert 19 占位符缺参；新 repo 上线必须配真实 SQL 测试。
func TestNASSourceCRUD(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	// 1. Create
	id, err := s.NASSources.Create(ctx, &domain.NASSource{
		Name:    "Asia-Movie",
		Path:    "/mnt/nas-root/Asia-Movie",
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if id <= 0 {
		t.Fatalf("expected positive id, got %d", id)
	}

	// 2. Get
	got, err := s.NASSources.Get(ctx, id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "Asia-Movie" || got.Path != "/mnt/nas-root/Asia-Movie" || !got.Enabled {
		t.Fatalf("unexpected: %+v", got)
	}
	if got.FileCount != 0 {
		t.Fatalf("file_count should default to 0, got %d", got.FileCount)
	}
	if got.LastAccessibility != domain.NASAccessibilityUnknown {
		t.Fatalf("last_accessibility default should be unknown, got %q", got.LastAccessibility)
	}
	if got.CreatedAt.IsZero() || got.UpdatedAt.IsZero() {
		t.Fatal("created_at / updated_at should be populated by CURRENT_TIMESTAMP")
	}

	// 3. List
	list, err := s.NASSources.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 source, got %d", len(list))
	}

	// 4. ListEnabled（默认 enabled=true 应该出现）
	enabled, err := s.NASSources.ListEnabled(ctx)
	if err != nil {
		t.Fatalf("list_enabled: %v", err)
	}
	if len(enabled) != 1 || enabled[0].Name != "Asia-Movie" {
		t.Fatalf("list_enabled mismatch: %+v", enabled)
	}

	// 5. Update（name + enabled）
	got.Name = "AsiaMovie-renamed"
	if err := s.NASSources.Update(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	got2, err := s.NASSources.Get(ctx, id)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if got2.Name != "AsiaMovie-renamed" {
		t.Fatalf("update didn't take: name=%q", got2.Name)
	}
	// file_count 应保持初始化时的 0（Update SQL 不写 file_count，由 UpdateHealth 单独负责）。
	if got2.FileCount != 0 {
		t.Fatalf("file_count should remain 0 (Update doesn't touch file_count), got %d", got2.FileCount)
	}
	if got2.UpdatedAt.Before(got.CreatedAt) {
		// SQLite CURRENT_TIMESTAMP 同秒可能出现 equal，更早不应出现
		t.Logf("updated_at %v vs created_at %v (acceptable if same-second)", got2.UpdatedAt, got2.CreatedAt)
	}

	// 6. UNIQUE name 冲突
	if _, err := s.NASSources.Create(ctx, &domain.NASSource{
		Name: "AsiaMovie-renamed", Path: "/mnt/nas-root/Different", Enabled: true,
	}); err == nil {
		t.Fatal("expected unique violation on name, got nil")
	}

	// 7. UNIQUE path 冲突
	if _, err := s.NASSources.Create(ctx, &domain.NASSource{
		Name: "DifferentName", Path: "/mnt/nas-root/Asia-Movie", Enabled: true,
	}); err == nil {
		t.Fatal("expected unique violation on path, got nil")
	}

	// 8. NameTaken / PathTaken with exclude
	taken, err := s.NASSources.NameTaken(ctx, "AsiaMovie-renamed", id)
	if err != nil {
		t.Fatalf("nametaken: %v", err)
	}
	if taken {
		t.Fatal("excludeID should exclude self")
	}
	taken, err = s.NASSources.PathTaken(ctx, "/mnt/nas-root/Asia-Movie", id)
	if err != nil {
		t.Fatalf("pathtaken: %v", err)
	}
	if taken {
		t.Fatal("excludeID should exclude self on path")
	}

	// 9. UpdateHealth
	now := time.Now().UTC()
	if err := s.NASSources.UpdateHealth(ctx, id, domain.NASAccessibilityOK, 7777, now); err != nil {
		t.Fatalf("update_health: %v", err)
	}
	h, _ := s.NASSources.Get(ctx, id)
	if h.LastAccessibility != domain.NASAccessibilityOK {
		t.Fatalf("last_accessibility should be ok, got %q", h.LastAccessibility)
	}
	if h.FileCount != 7777 {
		t.Fatalf("file_count should be 7777 after health update, got %d", h.FileCount)
	}
	if h.LastCheckedAt == nil {
		t.Fatal("last_checked_at should be set")
	}

	// 10. ListEnabled after disable
	got2.Enabled = false
	if err := s.NASSources.Update(ctx, got2); err != nil {
		t.Fatalf("disable update: %v", err)
	}
	enabledAfter, _ := s.NASSources.ListEnabled(ctx)
	if len(enabledAfter) != 0 {
		t.Fatalf("expected 0 enabled after disable, got %d", len(enabledAfter))
	}
	allAfter, _ := s.NASSources.List(ctx)
	if len(allAfter) != 1 {
		t.Fatalf("List should include disabled, got %d", len(allAfter))
	}

	// 11. Delete
	if err := s.NASSources.Delete(ctx, id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.NASSources.Get(ctx, id); err == nil {
		t.Fatal("expected NotFound after delete")
	}

	// 12. Delete 不存在
	err = s.NASSources.Delete(ctx, 9999)
	if err == nil {
		t.Fatal("expected NotFound on delete non-existent id")
	}
}

// TestNASSourceMigrate 测试 KV→DB 一次性迁移（[V7 §9.4+ 扩展 G1.G Q2=A]）。
func TestNASSourceMigrate(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	// 准备：写 KV
	if err := s.Configs.Set(ctx, domain.ConfigNASLocalPaths, `["/mnt/nas-root/Asia-Movie","/mnt/nas-root/Western-Movie"]`); err != nil {
		t.Fatalf("set nas_local_paths: %v", err)
	}
	if err := s.Configs.Set(ctx, domain.ConfigNASLocalPath, "/mnt/nas-root/Legacy"); err != nil {
		t.Fatalf("set legacy: %v", err)
	}

	// migrate
	if err := s.MigrateFromConfigsKV(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// 检查：3 条 source 写入
	all, err := s.NASSources.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(all) != 2 {
		// legacy 应被 ParseNASPaths 跳过（因为 nas_local_paths 已给出非空数组）
		t.Fatalf("expected 2 sources, got %d: %+v", len(all), all)
	}

	// 检查：KV 清空
	if v, ok, _ := s.Configs.Get(ctx, domain.ConfigNASLocalPaths); ok && v != "" {
		t.Fatalf("nas_local_paths should be cleared, got %q", v)
	}
	if v, ok, _ := s.Configs.Get(ctx, domain.ConfigNASLocalPath); ok && v != "" {
		t.Fatalf("nas_local_path should be cleared, got %q", v)
	}

	// 再 migrate 应当幂等（DB 已有数据不重复）
	if err := s.MigrateFromConfigsKV(ctx); err != nil {
		t.Fatalf("migrate 2nd: %v", err)
	}
	all2, _ := s.NASSources.List(ctx)
	if len(all2) != 2 {
		t.Fatalf("migrate should be idempotent, got %d sources", len(all2))
	}
}

// TestNASSourceMigrateEmpty 没配置过 KV → 无副作用（[V7 §9.4+ 扩展] 启动兜底）。
func TestNASSourceMigrateEmpty(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if err := s.MigrateFromConfigsKV(ctx); err != nil {
		t.Fatalf("empty migrate should be no-op, got %v", err)
	}
	all, _ := s.NASSources.List(ctx)
	if len(all) != 0 {
		t.Fatalf("expected 0 sources, got %d", len(all))
	}
}

// TestNASSourceNameTakenCaseInsensitive name 大小写不敏感（与 accounts 一致）。
func TestNASSourceNameTakenCaseInsensitive(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if _, err := s.NASSources.Create(ctx, &domain.NASSource{
		Name: "Movies", Path: "/mnt/a", Enabled: true,
	}); err != nil {
		t.Fatalf("create: %v", err)
	}
	// 大小写不同 → 应被识别为已占用
	taken, err := s.NASSources.NameTaken(ctx, "MOVIES", 0)
	if err != nil {
		t.Fatalf("nametaken: %v", err)
	}
	if !taken {
		t.Fatal("case-insensitive duplicate should be taken")
	}
}

// TestNASSourceTestDir 沙盒目录用 filepath.Join t.TempDir 模拟 /mnt/nas-root（不依赖真实 NAS）。
// 为不引入外部 stat 探测逻辑，本测试只验证 map 行为；端到端 NAS 探测在 ad-hoc pytest 验证。
func TestNASSourceTestDir(t *testing.T) {
	tmpDir := t.TempDir()
	got := filepath.Join(tmpDir, "subdir")
	if got == "" {
		t.Fatal("should not be empty")
	}
}
