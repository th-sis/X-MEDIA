CREATE TABLE favorites (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    external_id     INTEGER NOT NULL,
    external_source TEXT NOT NULL DEFAULT 'tmdb',
    media_type      TEXT NOT NULL,
    title           TEXT NOT NULL,
    poster_url      TEXT NOT NULL DEFAULT '',
    year            INTEGER NOT NULL DEFAULT 0,
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(external_id, external_source)
);
