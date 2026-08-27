// Package domain 业务领域模型.
package domain

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// SMBMountState 描述容器内 SMB 挂载点当前生命周期状态 (§9.4 UI-first).
//   - unmounted  未挂载或挂载失败后的终态, UI 显示「挂载」按钮
//   - mounting   正在执行 mount.cifs, UI 显示加载态
//   - mounted    已挂载且最近 stat 通过
//   - error      上次挂载失败或 unmount 后仍有进程占用
type SMBMountState string

const (
	SMBMountStateUnmounted SMBMountState = "unmounted"
	SMBMountStateMounting   SMBMountState = "mounting"
	SMBMountStateMounted    SMBMountState = "mounted"
	SMBMountStateError      SMBMountState = "error"
)

// SMBMount 是容器内 SMB 共享挂载点 (§9.4 UI-first).
// 用户在 NAS 配置页填 SMB URL (smb://user:pass@host/share/path) 与容器内
// 挂载点, 后端通过特权进程调用 mount.cifs 自动挂载, 持久化 DB, 重启重建.
//
// 安全注意: 用户名/密码字段以明文存 DB 仅用于容器内 mount.cifs 鉴权;
// 部署侧建议使用 SMB 无密共享或限制 SQLite 文件权限, 不要在前端日志暴露.
type SMBMount struct {
	ID                int64        `json:"id"`
	Name              string       `json:"name"`
	// SMBURL 形如 smb://user:pass@host/share 或 smb://host/share (无密).
	SMBURL            string       `json:"smb_url"`
	// RemotePath SMB 共享下的子目录 (例如 "Asia-Movie"), 留空表示挂根.
	RemotePath        string       `json:"remote_path"`
	// MountPoint 容器内挂载点 (例如 "/mnt/nas-root/Asia-Movie"), 必须是
	// 已存在或可创建的目录. 与 NASSource.path 一一对应.
	MountPoint        string       `json:"mount_point"`
	// UID/GID 容器内挂载后文件属主 (默认 0/0 = root). 写入 fstab 时透传.
	UID               int          `json:"uid"`
	GID               int          `json:"gid"`
	// State 当前生命周期状态 (实时从 /proc/mounts 探测).
	State             SMBMountState `json:"state"`
	// LastError 最近一次错误消息 (用户可见, 排查用).
	LastError         string       `json:"last_error,omitempty"`
	LastCheckedAt     *time.Time    `json:"last_checked_at,omitempty"`
	CreatedAt         time.Time     `json:"created_at"`
	UpdatedAt         time.Time     `json:"updated_at"`
}

// Validate 校验 SMBMount 字段合法 (避免入库后挂载阶段才报错).
func (m *SMBMount) Validate() error {
	if strings.TrimSpace(m.Name) == "" {
		return errors.New("名称不能为空")
	}
	if strings.TrimSpace(m.SMBURL) == "" {
		return errors.New("SMB URL 不能为空")
	}
	if !strings.HasPrefix(m.SMBURL, "smb://") && !strings.HasPrefix(m.SMBURL, "//") {
		return fmt.Errorf("SMB URL 必须以 smb:// 或 // 开头: %q", m.SMBURL)
	}
	if strings.TrimSpace(m.MountPoint) == "" {
		return errors.New("容器内挂载点不能为空")
	}
	if !strings.HasPrefix(m.MountPoint, "/") {
		return fmt.Errorf("容器内挂载点必须是绝对路径: %q", m.MountPoint)
	}
	return nil
}

// SMBMountRepository 容器内 SMB 挂载点仓储接口 (§9.4 UI-first).
type SMBMountRepository interface {
	Create(ctx context.Context, m *SMBMount) (int64, error)
	Update(ctx context.Context, m *SMBMount) error
	Delete(ctx context.Context, id int64) error
	Get(ctx context.Context, id int64) (*SMBMount, error)
	List(ctx context.Context) ([]*SMBMount, error)
	UpdateRuntime(ctx context.Context, id int64, state SMBMountState, lastErr string) error
}
