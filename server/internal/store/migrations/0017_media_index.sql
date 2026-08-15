CREATE TABLE media_index (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    external_id     INTEGER NOT NULL,
    external_source TEXT NOT NULL DEFAULT 'tmdb',
    season          INTEGER NOT NULL DEFAULT 0,
    episode         INTEGER NOT NULL DEFAULT 0,
    media_type      TEXT NOT NULL,
    title           TEXT NOT NULL,
    original_name   TEXT NOT NULL DEFAULT '',
    year            INTEGER NOT NULL DEFAULT 0,
    source_type     TEXT NOT NULL,
    account_id      INTEGER NOT NULL DEFAULT 0,
    file_path       TEXT NOT NULL,
    file_id         TEXT NOT NULL DEFAULT '',
    file_size       INTEGER NOT NULL DEFAULT 0,
    file_format     TEXT NOT NULL DEFAULT '',
    match_status    TEXT NOT NULL DEFAULT 'unconfirmed',
    match_score     REAL NOT NULL DEFAULT 0,
    stream_url      TEXT NOT NULL DEFAULT '',
    url_expires     TIMESTAMP,
    last_played_at  TIMESTAMP,
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(source_type, file_path)
);
CREATE INDEX idx_media_index_ext ON media_index(external_id, external_source, season, episode);
CREATE INDEX idx_media_index_source ON media_index(source_type, account_id);
