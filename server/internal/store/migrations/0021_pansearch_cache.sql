CREATE TABLE pansearch_cache (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    keyword     TEXT NOT NULL,
    cloud_types TEXT NOT NULL DEFAULT '',
    results     TEXT NOT NULL,
    link_count  INTEGER NOT NULL DEFAULT 0,
    cached_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(keyword, cloud_types)
);
