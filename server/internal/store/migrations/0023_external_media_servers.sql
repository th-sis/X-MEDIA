CREATE TABLE external_media_servers (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    server_type   TEXT NOT NULL,
    name          TEXT NOT NULL,
    base_url      TEXT NOT NULL,
    username      TEXT NOT NULL DEFAULT '',
    password      TEXT NOT NULL DEFAULT '',
    api_key       TEXT NOT NULL DEFAULT '',
    is_enabled    INTEGER NOT NULL DEFAULT 1,
    last_test_at  TIMESTAMP,
    test_status   TEXT NOT NULL DEFAULT 'untested',
    test_error    TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(server_type, base_url)
);
