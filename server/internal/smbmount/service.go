// Package smbmount 提供 SMB 挂载点的高层服务:
//   - 启动时把 DB 中所有 saved=true 且 state=mounted 的记录重新挂上;
//   - Mount/Unmount 包装 Mounter + SMBMountRepository, 写 last_error/state;
//   - Refresh 全量重新读取 /proc/self/mounts, 把实际状态写回 DB.
package smbmount

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"xmedia/internal/domain"
)

// resolveSMBCreds 从 MountRequest 解析用户名/密码。
// 两者都为空表示 guest 挂载（无密共享，URL 形如 //host/share 或 smb://host/share）。
// 放无 build tag 文件以便跨平台单测（不依赖 mount.cifs 二进制）。
func resolveSMBCreds(req MountRequest) (user, pass string) {
	user, pass = req.Username, req.Password
	if user == "" || pass == "" {
		u, err := url.Parse(req.SMBURL)
		if err != nil {
			return "", ""
		}
		if u.User != nil {
			if user == "" {
				user = u.User.Username()
			}
			if p, ok := u.User.Password(); ok && pass == "" {
				pass = p
			}
		}
		// 只有用户名没有密码（或完全无 userinfo）→ 匿名访问
		if pass == "" {
			user = ""
		}
	}
	return user, pass
}

// Mounter 抽象 mount.cifs / umount 调用 — 见 mounter_linux.go 与 mounter_other.go.
// 共享类型定义在此文件（无 build tag），平台文件只提供实现。
type Mounter interface {
	Mount(ctx context.Context, req MountRequest) error
	Unmount(ctx context.Context, mountPoint string) error
	IsMounted(mountPoint string) (bool, error)
	Refresh(ctx context.Context, mountPoint string) (MountStatus, error)
}

// MountRequest 单次挂载请求参数.
type MountRequest struct {
	SMBURL     string // smb://user:pass@host/share 或 //host/share (无密)
	RemotePath string // 共享下的子目录, 留空表示挂根
	MountPoint string // 容器内挂载点 (必须已存在或可创建)
	UID        int    // 挂载后文件属主, 0=root
	GID        int
	Username   string // 可选覆盖 URL 里的 user
	Password   string // 可选覆盖 URL 里的 pass
}

// MountStatus 描述容器内某挂载点当前状态.
type MountStatus struct {
	Mounted    bool
	Source     string // mountinfo 里的 fs source, 用于核对
	Filesystem string // cifs / smbfs
}

// DefaultMountTimeout mount.cifs 阻塞调用超时, 避免卡死 UI 按钮.
const DefaultMountTimeout = 10 * time.Second

// ErrMountBinMissing mount.cifs 在容器内不可用 (通常非特权/缺少 cifs-utils).
var ErrMountBinMissing = errors.New("smbmount: mount.cifs not found — container needs privileged + cifs-utils installed")

// Service 把挂载操作与 DB 状态写回打通.
type Service struct {
	repo    domain.SMBMountRepository
	mounter Mounter
	log     *slog.Logger
}

// New 构造 Service (Linux 用 NewExecMounter, 其他平台自动走 stub).
func New(repo domain.SMBMountRepository, mounter Mounter, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	if mounter == nil {
		mounter = NewExecMounter()
	}
	return &Service{repo: repo, mounter: mounter, log: log}
}

// Mount 创建/重新挂载. 若 DB 中已存在同 mount_point 则更新 (idempotent).
func (s *Service) Mount(ctx context.Context, m *domain.SMBMount) error {
	if err := m.Validate(); err != nil {
		return err
	}
	_ = s.repo.UpdateRuntime(ctx, m.ID, domain.SMBMountStateMounting, "")
	if err := s.mounter.Mount(ctx, MountRequest{
		SMBURL:     m.SMBURL,
		RemotePath: m.RemotePath,
		MountPoint: m.MountPoint,
		UID:        m.UID,
		GID:        m.GID,
	}); err != nil {
		_ = s.repo.UpdateRuntime(ctx, m.ID, domain.SMBMountStateError, err.Error())
		return fmt.Errorf("mount failed: %w", err)
	}
	// 同步持久化 — 首次创建或编辑后的 mount 都存入 DB.
	if m.ID == 0 {
		id, err := s.repo.Create(ctx, m)
		if err != nil && !strings.Contains(err.Error(), "UNIQUE") {
			return err
		}
		if err == nil {
			m.ID = id
		}
	}
	return s.repo.UpdateRuntime(ctx, m.ID, domain.SMBMountStateMounted, "")
}

// Unmount 卸载并把 state 写回 unmounted. err 仍写 error 状态便于 UI 显示.
func (s *Service) Unmount(ctx context.Context, id int64) error {
	m, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := s.mounter.Unmount(ctx, m.MountPoint); err != nil {
		_ = s.repo.UpdateRuntime(ctx, id, domain.SMBMountStateError, err.Error())
		return err
	}
	return s.repo.UpdateRuntime(ctx, id, domain.SMBMountStateUnmounted, "")
}

// RefreshState 全量查 /proc/self/mounts 把真实状态写回 DB — 启动时与周期任务调用.
// 用于: 1) 服务被外部重启时纠正 DB 状态; 2) mount 点被运维手动 unmount 后 UI 同步.
func (s *Service) RefreshState(ctx context.Context) error {
	mounts, err := s.repo.List(ctx)
	if err != nil {
		return err
	}
	for _, m := range mounts {
		st, err := s.mounter.Refresh(ctx, m.MountPoint)
		if err != nil {
			s.log.Warn("smbmount: refresh state failed", "name", m.Name, "err", err)
			continue
		}
		newState := domain.SMBMountStateUnmounted
		if st.Mounted {
			newState = domain.SMBMountStateMounted
		}
		_ = s.repo.UpdateRuntime(ctx, m.ID, newState, "")
	}
	return nil
}

// ReattachOnStartup 启动时把 DB 中所有 state=mounted 的记录重新挂载.
// 失败一个不影响其他, 最后统一 RefreshState 校准.
func (s *Service) ReattachOnStartup(ctx context.Context) error {
	mounts, err := s.repo.List(ctx)
	if err != nil {
		return err
	}
	var failures int
	for _, m := range mounts {
		// 已经在的跳过 (幂等).
		if mounted, _ := s.mounter.IsMounted(m.MountPoint); mounted {
			continue
		}
		if err := s.Mount(ctx, m); err != nil {
			failures++
			s.log.Warn("smbmount: reattach failed", "name", m.Name, "err", err)
		}
	}
	_ = s.RefreshState(ctx)
	if failures > 0 {
		s.log.Warn("smbmount: reattach had failures", "count", failures)
	}
	return nil
}
