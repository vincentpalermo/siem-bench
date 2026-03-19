CREATE TABLE IF NOT EXISTS events (
    id TEXT PRIMARY KEY,
    timestamp TIMESTAMPTZ NOT NULL,
    source_type TEXT NOT NULL,
    host TEXT NOT NULL,
    user_name TEXT NOT NULL,
    src_ip TEXT NOT NULL,
    dst_ip TEXT NOT NULL,
    event_code TEXT NOT NULL,
    severity INT NOT NULL,
    message TEXT NOT NULL,
    raw TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events (timestamp);
CREATE INDEX IF NOT EXISTS idx_events_host ON events (host);
CREATE INDEX IF NOT EXISTS idx_events_user_name ON events (user_name);
CREATE INDEX IF NOT EXISTS idx_events_source_type ON events (source_type);