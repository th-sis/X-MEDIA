CREATE TABLE resolve_rate_limits (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    client_ip   TEXT NOT NULL,
    window_start TIMESTAMP NOT NULL,
    request_count INTEGER NOT NULL DEFAULT 1,
    UNIQUE(client_ip, window_start)
);
CREATE INDEX idx_rate_limits_window ON resolve_rate_limits(window_start);
