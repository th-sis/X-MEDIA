// [V7 §9.4 UI-first] 用户在 NAS 配置页填 SMB URL + 容器内挂载点, 后端在容器内
// 自动 mount.cifs. Validate 阻止脏数据入库 (避免挂载阶段才报错).

package domain

import (
	"strings"
	"testing"
)

func TestSMBMount_Validate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(m *SMBMount)
		wantErr string
	}{
		{"ok smb://", func(m *SMBMount) {}, ""},
		{"ok // (UNC)", func(m *SMBMount) { m.SMBURL = "//192.168.7.154/BTORAGE" }, ""},
		{"name empty", func(m *SMBMount) { m.Name = "" }, "名称不能为空"},
		{"url empty", func(m *SMBMount) { m.SMBURL = "" }, "SMB URL"},
		{"url wrong scheme", func(m *SMBMount) { m.SMBURL = "/mnt/BTORAGE" }, "必须以 smb"},
		{"mountpoint empty", func(m *SMBMount) { m.MountPoint = "" }, "挂载点不能为空"},
		{"mountpoint not absolute", func(m *SMBMount) { m.MountPoint = "mnt/nas-root/Asia" }, "绝对路径"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &SMBMount{
				Name:       "Asia-Movie",
				SMBURL:     "smb://user:pass@192.168.7.154/BTORAGE",
				RemotePath: "Asia-Movie",
				MountPoint: "/mnt/nas-root/Asia-Movie",
				UID:        0,
				GID:        0,
			}
			tt.mutate(m)
			err := m.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected ok, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected err containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %q, want containing %q", err.Error(), tt.wantErr)
			}
		})
	}
}
