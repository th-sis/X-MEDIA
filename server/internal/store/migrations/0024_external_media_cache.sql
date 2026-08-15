CREATE TABLE external_media_cache (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id      INTEGER NOT NULL,
    external_id    TEXT NOT NULL,
    title          TEXT NOT NULL,
    media_type     TEXT NOT NULL,
    year           INTEGER DEFAULT 0,
    poster_url     TEXT NOT NULL DEFAULT '',
    backdrop_url   TEXT NOT NULL DEFAULT '',
    overview       TEXT NOT NULL DEFAULT '',
    rating         REAL DEFAULT 0,
    extra          TEXT NOT NULL DEFAULT '{}',
    cached_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(server_id, external_id)
);
