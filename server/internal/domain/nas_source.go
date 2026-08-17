package domain

import (
	"context"
	"time"
)

// NASAccessibility 描述 NAS 路径最近一次可访问性探测的结果。
// 用于 capabilities.nas.status 三态（§27.4）+ UI 健康卡颜色。
type NASAccessibility string

const (
	NASAccessibilityUnknown       NASAccessibility = "unknown"
	NASAccessibilityOK            NASAccessibility = "ok"
	NASAccessibilityNotAccessible NASAccessibility = "not_accessible"
)

// NASSource 是 NAS 媒体源（[V7 §9.4+ 扩展]）的多源管理模型。
//
// 每条记录对应一个独立的 SMB 共享下的子目录（如 `/mnt/nas-root/Asia-Movie`），
// 后端通过 bind mount 把 SMB 父目录挂载到容器 `/mnt/nas-root`，每条 source 都是
// 容器内绝对路径。索引引擎会逐条遍历所有 enabled source 并合并到 media_index。
//
// 设计要点：
//   - name 唯一（管理员可读标识），path 唯一（防重复添加）
//   - enabled=false 时该 source 不参与扫描、不参与 P0 智能跳过命中（§6.3）
//   - file_count 由后台定期探测写入（避免每次全量 stat）
//   - last_accessibility 由 `/api/admin/nas-sources/test-path` 或后台探测写入
type NASSource struct {
	ID                int64
	Name              string
	Path              string
	Enabled           bool
	FileCount         int64
	LastAccessibility NASAccessibility
	LastCheckedAt     *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// Clone 返回 source 的浅拷贝（用于 handler 内修改后回写）。
func (s *NASSource) Clone() *NASSource {
	if s == nil {
		return nil
	}
	out := *s
	return &out
}

// NASSourceRepository 是 NAS 媒体源仓储接口（[V7 §9.4+ 扩展]）。
// 方法命名与 AccountRepository 对齐，保持上层心智一致。
type NASSourceRepository interface {
	// Create 新增一条 source（要求 name/path 唯一）。
	Create(ctx context.Context, s *NASSource) (int64, error)
	// Update 修改 name/path/enabled 等可编辑字段。
	Update(ctx context.Context, s *NASSource) error
	// Delete 物理删除一条 source（不影响 media_index；Phase D 自然清掉）。
	Delete(ctx context.Context, id int64) error
	// Get 按 ID 查询。
	Get(ctx context.Context, id int64) (*NASSource, error)
	// List 列出全部（按 ID 升序，admin 后台用）。
	List(ctx context.Context) ([]*NASSource, error)
	// ListEnabled 列出启用的 source（索引扫描、Capabilities 用）。
	ListEnabled(ctx context.Context) ([]*NASSource, error)
	// PathTaken 检查路径是否已被其它 source 占用（excludeID>0 时排除自身）。
	PathTaken(ctx context.Context, path string, excludeID int64) (bool, error)
	// NameTaken 检查名称是否已被其它 source 占用。
	NameTaken(ctx context.Context, name string, excludeID int64) (bool, error)
	// UpdateHealth 写入最近一次可访问性探测结果（由 stat 探测或 cron 周期性调用）。
	UpdateHealth(ctx context.Context, id int64, accessibility NASAccessibility, fileCount int64, at time.Time) error
}
