// [V7 §9.4 UI-first] smb_mounts 仓储真实 SQL 回归（复用历史教训：
// mock 单测覆盖不到占位符/参数错误，新 repo 上线必须配真实 SQL 测试）。
package store_test

import (
	"context"
	"testing"

	"xmedia/internal/domain"
)

func TestSMBMountRepoCRUD(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	// 1. Create
	id, err := s.SMBMounts.Create(ctx, &domain.SMBMount{
		Name:       "Asia-Movie",
		SMBURL:     "smb://user:pass@192.168.7.154/BTORAGE",
		RemotePath: "Asia-Movie",
		MountPoint: "/mnt/nas-root/Asia-Movie",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if id <= 0 {
		t.Fatalf("expected positive id, got %d", id)
	}

	// 2. Get
	got, err := s.SMBMounts.Get(ctx, id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "Asia-Movie" || got.SMBURL != "smb://user:pass@192.168.7.154/BTORAGE" {
		t.Fatalf("unexpected: %+v", got)
	}
	if got.MountPoint != "/mnt/nas-root/Asia-Movie" {
		t.Fatalf("mount_point = %q", got.MountPoint)
	}
	if got.State != domain.SMBMountStateUnmounted {
		t.Fatalf("state default should be unmounted, got %q", got.State)
	}
	if got.CreatedAt.IsZero() || got.UpdatedAt.IsZero() {
		t.Fatal("created_at / updated_at should be populated")
	}

	// 3. List
	list, err := s.SMBMounts.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1, got %d", len(list))
	}

	// 4. Update（改 remote_path / mount_point）
	got.RemotePath = "Western-Movie"
	got.MountPoint = "/mnt/nas-root/Western-Movie"
	if err := s.SMBMounts.Update(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	after, _ := s.SMBMounts.Get(ctx, id)
	if after.RemotePath != "Western-Movie" || after.MountPoint != "/mnt/nas-root/Western-Movie" {
		t.Fatalf("update not persisted: %+v", after)
	}

	// 5. UpdateRuntime（状态机写回）
	if err := s.SMBMounts.UpdateRuntime(ctx, id, domain.SMBMountStateMounted, ""); err != nil {
		t.Fatalf("UpdateRuntime: %v", err)
	}
	rt, _ := s.SMBMounts.Get(ctx, id)
	if rt.State != domain.SMBMountStateMounted {
		t.Fatalf("state = %q, want mounted", rt.State)
	}
	if rt.LastCheckedAt == nil {
		t.Fatal("last_checked_at should be set by UpdateRuntime")
	}
	if err := s.SMBMounts.UpdateRuntime(ctx, id, domain.SMBMountStateError, "mount.cifs: access denied"); err != nil {
		t.Fatalf("UpdateRuntime error: %v", err)
	}
	er, _ := s.SMBMounts.Get(ctx, id)
	if er.State != domain.SMBMountStateError || er.LastError != "mount.cifs: access denied" {
		t.Fatalf("error state not persisted: %+v", er)
	}

	// 6. Delete
	if err := s.SMBMounts.Delete(ctx, id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	list2, _ := s.SMBMounts.List(ctx)
	if len(list2) != 0 {
		t.Fatalf("expected 0 after delete, got %d", len(list2))
	}
}

func TestSMBMountRepoNameUnique(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	m1 := &domain.SMBMount{
		Name:       "dup",
		SMBURL:     "smb://user:pass@h1/share",
		MountPoint: "/mnt/nas-root/one",
	}
	id1, err := s.SMBMounts.Create(ctx, m1)
	if err != nil {
		t.Fatal(err)
	}
	m2 := &domain.SMBMount{
		Name:       "dup", // 与 m1 同名 → UNIQUE 冲突
		SMBURL:     "smb://user:pass@h2/share",
		MountPoint: "/mnt/nas-root/two",
	}
	if _, err := s.SMBMounts.Create(ctx, m2); err == nil {
		t.Fatal("同名挂载应触发 UNIQUE 冲突")
	}
	// 同 mount_point 冲突
	m3 := &domain.SMBMount{
		Name:       "other",
		SMBURL:     "smb://user:pass@h3/share",
		MountPoint: "/mnt/nas-root/one",
	}
	if _, err := s.SMBMounts.Create(ctx, m3); err == nil {
		t.Fatal("同挂载点应触发 UNIQUE 冲突")
	}
	_ = id1
}
