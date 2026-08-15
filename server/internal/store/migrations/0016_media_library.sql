CREATE TABLE media_library (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    external_id     INTEGER NOT NULL,
    external_source TEXT NOT NULL DEFAULT 'tmdb',
    media_type      TEXT NOT NULL,
    title           TEXT NOT NULL,
    title_orig      TEXT NOT NULL DEFAULT '',
    poster_url      TEXT NOT NULL DEFAULT '',
    backdrop_url    TEXT NOT NULL DEFAULT '',
    overview        TEXT NOT NULL DEFAULT '',
    year            INTEGER NOT NULL DEFAULT 0,
    vote_avg        REAL NOT NULL DEFAULT 0,
    vote_count      INTEGER NOT NULL DEFAULT 0,
    genres          TEXT NOT NULL DEFAULT '[]',
    runtime         INTEGER NOT NULL DEFAULT 0,
    seasons         INTEGER NOT NULL DEFAULT 0,
    episodes        INTEGER NOT NULL DEFAULT 0,
    seasons_json    TEXT NOT NULL DEFAULT '[]',
    cast            TEXT NOT NULL DEFAULT '[]',
    extra           TEXT NOT NULL DEFAULT '{}',
    cached_at       TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_accessed_at TIMESTAMP,
    UNIQUE(external_id, external_source)
);
CREATE INDEX idx_media_library_ext ON media_library(external_id, external_source);
CREATE INDEX idx_media_library_accessed ON media_library(last_accessed_at);
