// Package smbmount 提供在容器内通过特权进程挂载 SMB 共享的能力
// (V7 §9.4 UI-first). 仅 Linux 编译, 其他平台给 stub.
//
// 安全考虑:
//   - 容器必须以 privileged: true + cap-add SYS_ADMIN 运行
//   - credentials 文件 0600, 路径在容器内 (不在 NAS 上)
//   - UI 不向用户暴露明文 URL (marshal 时省略 password)
package smbmount

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// execMounter 是生产实现 — 走 mount.cifs (需要 root + CAP_SYS_ADMIN).
// 共享类型 Mounter/MountRequest/MountStatus/DefaultMountTimeout/
// ErrMountBinMissing 定义在 service.go（无 build tag）。
type execMounter struct {
	mountBin string // "mount.cifs" 或 "/sbin/mount.cifs"
}

// NewExecMounter 构造一个 Linux 生产 Mounter.
// 若系统找不到 mount.cifs, 调用会返回 ErrMountBinMissing 而不是 panic.
func NewExecMounter() Mounter {
	bin := "mount.cifs"
	for _, p := range []string{"/sbin/mount.cifs", "/usr/sbin/mount.cifs", "mount.cifs"} {
		if _, err := os.Stat(p); err == nil {
			bin = p
			break
		}
	}
	return &execMounter{mountBin: bin}
}

// ErrMountBinMissing 定义在 service.go（无 build tag）。

// writeCredentialsFile 把 username/password 写到 0600 临时文件, mount.cifs 用
// credentials=FILE 选项读取. 避免在命令行暴露密码到 /proc/cmdline.
func writeCredentialsFile(user, pass string) (path string, cleanup func(), err error) {
	f, err := os.CreateTemp("", "smb-cred-*.cred")
	if err != nil {
		return "", nil, fmt.Errorf("create credentials file: %w", err)
	}
	path = f.Name()
	cleanup = func() { _ = os.Remove(path) }
	if _, err := f.WriteString(fmt.Sprintf("username=%s\npassword=%s\n", user, pass)); err != nil {
		cleanup()
		return "", nil, err
	}
	if err := f.Chmod(0o600); err != nil {
		cleanup()
		return "", nil, err
	}
	_ = f.Close()
	return path, cleanup, nil
}

// resolveSMBCreds 定义在 service.go（无 build tag，跨平台可测）。

// Mount 在容器内调用 mount.cifs, 失败返回可读错误.
// 要求: 容器 privileged + 主机已安装 cifs-utils; 调用方负责先创建 MountPoint 目录.
// 无凭据 URL (//host/share) 走 guest 挂载.
func (m *execMounter) Mount(ctx context.Context, req MountRequest) error {
	if req.MountPoint == "" || req.SMBURL == "" {
		return errors.New("smbmount: smb_url and mount_point are required")
	}
	user, pass := resolveSMBCreds(req)
	var opts string
	if user == "" {
		opts = fmt.Sprintf("guest,uid=%d,gid=%d,iocharset=utf8,vers=3.0", req.UID, req.GID)
	} else {
		credFile, cleanup, err := writeCredentialsFile(user, pass)
		if err != nil {
			return err
		}
		defer cleanup()
		opts = fmt.Sprintf("credentials=%s,uid=%d,gid=%d,iocharset=utf8,vers=3.0", credFile, req.UID, req.GID)
	}

	// 构造 source: //host/share[/sub/...]
	source := strings.TrimPrefix(req.SMBURL, "smb://")
	source = strings.TrimPrefix(source, "//")
	if req.RemotePath != "" {
		source = strings.TrimRight(source, "/") + "/" + strings.TrimLeft(req.RemotePath, "/")
	}
	dst := req.MountPoint

	// 先确保挂载点存在
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return fmt.Errorf("smbmount: mkdir mountpoint: %w", err)
	}

	args := []string{
		"-t", "cifs",
		"-o", opts,
		source, dst,
	}
	cmd := exec.CommandContext(ctx, m.mountBin, args...)
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("smbmount: mount.cifs %s -> %s failed: %v (%s)", source, dst, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Unmount 调用 umount 释放挂载点. 失败不强制 kill 进程 (LitePan 用 lazy unmount).
func (m *execMounter) Unmount(ctx context.Context, mountPoint string) error {
	cmd := exec.CommandContext(ctx, "umount", mountPoint)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	// 退化: lazy unmount
	cmd = exec.CommandContext(ctx, "umount", "-l", mountPoint)
	out, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("smbmount: umount %s failed: %v (%s)", mountPoint, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// IsMounted 通过 /proc/self/mounts 查挂载点是否已被 SMB 占用.
func (m *execMounter) IsMounted(mountPoint string) (bool, error) {
	data, err := os.ReadFile("/proc/self/mounts")
	if err != nil {
		return false, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		if fields[1] == mountPoint {
			return true, nil
		}
	}
	return false, nil
}

// Refresh 读取 /proc/self/mounts 获取挂载点当前状态 (供 UI 轮询).
func (m *execMounter) Refresh(_ context.Context, mountPoint string) (MountStatus, error) {
	data, err := os.ReadFile("/proc/self/mounts")
	if err != nil {
		return MountStatus{}, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		if fields[1] == mountPoint {
			return MountStatus{Mounted: true, Source: fields[0], Filesystem: fields[2]}, nil
		}
	}
	return MountStatus{Mounted: false}, nil
}

// 找不到 mount.cifs 时 NewExecMounter 仍可用 — 首次 Mount 调用返回 ErrMountBinMissing.
func init() {
	if _, err := os.Stat("/sbin/mount.cifs"); errors.Is(err, os.ErrNotExist) {
		// Linux 容器无 cifs-utils 时, Mount 才会失败 — Service 层会捕获并返
		// ErrMountBinMissing 告知用户去装 cifs-utils.
	}
}
