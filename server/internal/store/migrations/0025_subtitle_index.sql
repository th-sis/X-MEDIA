CREATE TABLE subtitle_index (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    external_id   INTEGER NOT NULL,
    external_source TEXT NOT NULL DEFAULT 'tmdb',
    media_type    TEXT NOT NULL,
    season        INTEGER NOT NULL DEFAULT 0,
    episode       INTEGER NOT NULL DEFAULT 0,
    language      TEXT NOT NULL,
    filename      TEXT NOT NULL,
    local_path    TEXT NOT NULL,
    file_size     INTEGER NOT NULL DEFAULT 0,
    source        TEXT NOT NULL,
    source_id     TEXT NOT NULL DEFAULT '',
    format        TEXT NOT NULL DEFAULT 'srt',
    rating        REAL NOT NULL DEFAULT 0,
    download_count INTEGER NOT NULL DEFAULT 0,
    created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(external_id, external_source, season, episode, language, source, source_id)
);
CREATE INDEX idx_subtitle_ext ON subtitle_index(external_id, external_source, language);
