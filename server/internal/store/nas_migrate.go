package store

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"xmedia/internal/domain"
)

// MigrateFromConfigsKV 把旧的 configs.nas_local_paths / configs.nas_local_path
// 解析后写入 nas_sources 表，写入后清空旧 key 实现单向迁移。
//
// 行为（[V7 §9.4+ 扩展] Q2=A：迁移后清空 KV）：
//   - 若 nas_sources 已有数据 → 不做任何操作（避免重复运行导致重复 source）
//   - 解析 KV 得到 pathList
//   - 每条 path 写一条 source（name 用 path basename；同 path 重复被 UNIQUE 跳过）
//   - 清空旧 configs 两 key
//
// 调用方：必须在 Migrate() 后、wire_xmedia.go 启动 goroutine 中调用一次。
func (s *Store) MigrateFromConfigsKV(ctx context.Context) error {
	existing, err := s.NASSources.List(ctx)
	if err != nil {
		return fmt.Errorf("check nas_sources: %w", err)
	}
	if len(existing) > 0 {
		// 已经迁移过（或人工添加过），跳过；用户拍板 Q2=A 的不可重复保证
		return nil
	}

	newJSON, newOK, err := s.Configs.Get(ctx, domain.ConfigNASLocalPaths)
	if err != nil {
		return fmt.Errorf("read nas_local_paths: %w", err)
	}
	legacy, legacyOK, err := s.Configs.Get(ctx, domain.ConfigNASLocalPath)
	if err != nil {
		return fmt.Errorf("read nas_local_path: %w", err)
	}
	if !newOK && !legacyOK {
		// 没配置过 NAS，跳过
		return nil
	}

	paths := domain.ParseNASPaths(stringOrEmpty(newOK, newJSON), stringOrEmpty(legacyOK, legacy))
	if len(paths) == 0 {
		return nil
	}

	for _, p := range paths {
		src := &domain.NASSource{
			Name:    deriveSourceName(p),
			Path:    p,
			Enabled: true,
		}
		if _, err := s.NASSources.Create(ctx, src); err != nil {
			// UNIQUE 冲突（同名或同路径）→ 跳过这一条，不阻塞其它
			// 但其它错误必须冒泡，避免半迁移状态
			if isUniqueConstraintErr(err) {
				continue
			}
			return fmt.Errorf("insert migrated source %q: %w", p, err)
		}
	}

	// 清空旧 KV（单向迁移，避免重复启动时再次迁移）
	if newOK {
		if err := s.Configs.Set(ctx, domain.ConfigNASLocalPaths, ""); err != nil {
			return fmt.Errorf("clear nas_local_paths: %w", err)
		}
	}
	if legacyOK {
		if err := s.Configs.Set(ctx, domain.ConfigNASLocalPath, ""); err != nil {
			return fmt.Errorf("clear nas_local_path: %w", err)
		}
	}
	return nil
}

func stringOrEmpty(ok bool, v string) string {
	if ok {
		return v
	}
	return ""
}

// deriveSourceName 把 NAS 路径转成可读 name。
// 如 `/mnt/nas-root/Asia-Movie` → `Asia-Movie`。
// 冲突时（多 path 同 basename）由 UNIQUE(name) 保护，跳过即可。
func deriveSourceName(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return "nas-source"
	}
	base := filepath.Base(p)
	if base == "." || base == string(filepath.Separator) || base == "/" || base == "" {
		return "nas-source"
	}
	return base
}

// isUniqueConstraintErr 判断 SQLite 是否报 UNIQUE 约束冲突。
// 用于迁移循环中批量跳过已存在条目。
func isUniqueConstraintErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint failed")
}
