//go:build linux

// [V7 §9.4 UI-first] mount.cifs 鉴权文件与可观察参数 (Linux 专测, Windows 跳过).
package smbmount

import (
	"context"
	"os"
	"strings"
	"testing"
)

// TestMount_CredentialsFilePermissions 保证 credentials 文件 0600,
// 避免 mount.cifs 时密码信息泄露到 /proc/cmdline.
func TestMount_CredentialsFilePermissions(t *testing.T) {
	path, cleanup, err := writeCredentialsFile("alice", "s3cret")
	if err != nil {
		t.Fatalf("writeCredentialsFile failed: %v", err)
	}
	defer cleanup()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("credentials file perm = %o, want 0600", perm)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	if !strings.Contains(body, "username=alice") || !strings.Contains(body, "password=s3cret") {
		t.Fatalf("credentials body wrong: %q", body)
	}
}

// TestRefresh_ReadsProcMounts 空 mount 点应报未挂载.
func TestRefresh_ReadsProcMounts(t *testing.T) {
	st, err := (&execMounter{}).Refresh(context.Background(), "/this/never/exists")
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if st.Mounted {
		t.Fatalf("random path 不应报 mounted")
	}
}

// TestIsMounted_FalseOnEmpty 保证 IsMounted 在路径不存在时返回 false 不报错.
func TestIsMounted_FalseOnEmpty(t *testing.T) {
	em := &execMounter{}
	mounted, err := em.IsMounted("/this/never/exists/mountpoint")
	if err != nil {
		t.Fatalf("IsMounted 期望无错, got %v", err)
	}
	if mounted {
		t.Fatalf("随机路径不应被识别为已挂载")
	}
}
