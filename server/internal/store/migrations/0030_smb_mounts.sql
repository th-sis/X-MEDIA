-- [V7 §9.4 UI-first] 容器内 SMB 挂载点持久化 (§9.4 UI-first, 上一轮 d2bc032 未做).
-- 用户在 NAS 配置页填 smb_url + mount_point, 后端特权容器内 mount.cifs.
CREATE TABLE smb_mounts (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    name              TEXT    NOT NULL UNIQUE,
    smb_url           TEXT    NOT NULL,
    remote_path       TEXT    NOT NULL DEFAULT '',
    mount_point       TEXT    NOT NULL UNIQUE,
    uid               INTEGER NOT NULL DEFAULT 0,
    gid               INTEGER NOT NULL DEFAULT 0,
    state             TEXT    NOT NULL DEFAULT 'unmounted',  -- unmounted/mounting/mounted/error
    last_error        TEXT    NOT NULL DEFAULT '',
    last_checked_at   TIMESTAMP,
    created_at        TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_smb_mounts_state ON smb_mounts(state);
