CREATE TABLE play_history (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    external_id     INTEGER NOT NULL,
    external_source TEXT NOT NULL DEFAULT 'tmdb',
    media_type      TEXT NOT NULL,
    title           TEXT NOT NULL,
    poster_url      TEXT NOT NULL DEFAULT '',
    source_type     TEXT NOT NULL,
    season          INTEGER NOT NULL DEFAULT 0,
    episode         INTEGER NOT NULL DEFAULT 0,
    position_ms     INTEGER NOT NULL DEFAULT 0,
    duration_ms     INTEGER NOT NULL DEFAULT 0,
    played_at       TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(external_id, external_source, season, episode)
);
