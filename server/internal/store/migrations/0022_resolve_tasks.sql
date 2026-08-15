CREATE TABLE resolve_tasks (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    external_id    INTEGER NOT NULL,
    external_source TEXT NOT NULL DEFAULT 'tmdb',
    media_type     TEXT NOT NULL,
    title          TEXT NOT NULL,
    year           INTEGER NOT NULL DEFAULT 0,
    season         INTEGER NOT NULL DEFAULT 0,
    episode        INTEGER NOT NULL DEFAULT 0,
    status         TEXT NOT NULL DEFAULT 'pending',
    stage          TEXT NOT NULL DEFAULT '',
    stage_detail   TEXT NOT NULL DEFAULT '',
    progress_pct   INTEGER NOT NULL DEFAULT 0,
    result_source  TEXT NOT NULL DEFAULT '',
    result_file_id TEXT NOT NULL DEFAULT '',
    result_account_id INTEGER NOT NULL DEFAULT 0,
    result_file_path TEXT NOT NULL DEFAULT '',
    error_msg      TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_resolve_tasks_active ON resolve_tasks(external_id, external_source, season, episode)
    WHERE status IN ('pending', 'running');
