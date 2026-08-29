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


// TestMount_OptsIncludeNTLMSSP [V7 §9.4+ 真实部署修复] TrueNAS / 现代 SMB 默认认证
// 机制是 ntlmssp。kernel.cifs 默认 sec=ntlm 在新服务器上被拒、mount.cifs 静默成功
// 但拿到匿名空视图（DB state=mounted 但 ls /mnt/... 是空，典型 root cause）。
// 强制 sec=ntlmssp 让 cifs 客户端与 TrueNAS 完成正确协议握手。
func TestMount_OptsIncludeNTLMSSP(t *testing.T) {
	// 跑一遍 Mount（实际会因 mountBin 不存在而失败，但 opts 已拼好 — 从错误消息提取验证）。
	em := &execMounter{mountBin: "/__definitely_not_exists_mount_cifs__"}
	req := MountRequest{
		SMBURL:     "smb://user:pass@192.168.7.154/Asia-Movie",
		MountPoint: "/tmp/__opts_check__",
		UID:        0,
		GID:        0,
	}
	err := em.Mount(context.Background(), req)
	if err == nil {
		t.Fatal("用不存在的 mountBin 应报错")
	}
	msg := err.Error()
	if !strings.Contains(msg, "sec=ntlmssp") {
		t.Fatalf("opts 应包含 sec=ntlmssp（TrueNAS 必需），实际 err: %v", err)
	}
	if strings.Contains(msg, "vers=3.0") {
		t.Fatalf("opts 不应再强制 vers=3.0（强制会被 TrueNAS 拒绝），实际 err: %v", err)
	}
	if !strings.Contains(msg, "credentials=") {
		t.Fatalf("opts 应包含 credentials= 文件（避免密码暴露 /proc/cmdline），实际 err: %v", err)
	}
}

// TestMount_GuestOptsIncludeNTLMSSP [V7 §9.4+] guest 无密模式也需 sec=ntlmssp。
func TestMount_GuestOptsIncludeNTLMSSP(t *testing.T) {
	em := &execMounter{mountBin: "/__definitely_not_exists_mount_cifs__"}
	req := MountRequest{
		SMBURL:     "//192.168.7.154/Public", // 无凭据 → guest
		MountPoint: "/tmp/__opts_guest_check__",
		UID:        0,
		GID:        0,
	}
	err := em.Mount(context.Background(), req)
	if err == nil {
		t.Fatal("用不存在的 mountBin 应报错")
	}
	if !strings.Contains(err.Error(), "guest") {
		t.Fatalf("guest 模式 opts 应包含 guest，实际 err: %v", err)
	}
	if !strings.Contains(err.Error(), "sec=ntlmssp") {
		t.Fatalf("guest 模式 opts 也应包含 sec=ntlmssp，实际 err: %v", err)
	}
}
