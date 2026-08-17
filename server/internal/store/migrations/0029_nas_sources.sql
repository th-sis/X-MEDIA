-- [V7 §9.4+ 多源扩展] NAS 媒体源多路径 CRUD：
--   - 单源 = 一条 NAS 路径配置（容器内绝对路径），可独立启/停
--   - 兼容迁移：启动时一次性把旧 configs.nas_local_paths 与 configs.nas_local_path
--     解析后写入本表（见 internal/store/nas_migrate.go 与 wire_xmedia.go）
--   - name 唯一，用于管理后台可读标识（如 "亚洲电影"）
--   - UNIQUE(path) 防重复添加
--   - enabled 用于 P0 智能跳过 §6.3 与扫描阶段 §9.7
CREATE TABLE nas_sources (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    name                TEXT    NOT NULL,
    path                TEXT    NOT NULL,
    enabled             INTEGER NOT NULL DEFAULT 1,
    file_count          INTEGER NOT NULL DEFAULT 0,
    last_accessibility  TEXT    NOT NULL DEFAULT 'unknown',  -- ok | not_accessible | unknown
    last_checked_at     TIMESTAMP,
    created_at          TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(name),
    UNIQUE(path)
);

-- [V7 §9.4+ 多源扩展] 索引：enabled 子集常用（启动扫描 P0 跳过的依据）
CREATE INDEX idx_nas_sources_enabled ON nas_sources(enabled);
