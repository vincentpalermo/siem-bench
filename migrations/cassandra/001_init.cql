CREATE KEYSPACE IF NOT EXISTS siem
WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 1};

CREATE TABLE IF NOT EXISTS siem.events (
    bucket text,
    timestamp timestamp,
    id text,
    source_type text,
    host text,
    user_name text,
    src_ip text,
    dst_ip text,
    event_code text,
    severity int,
    message text,
    raw text,
    PRIMARY KEY ((bucket), timestamp, id)
) WITH CLUSTERING ORDER BY (timestamp DESC, id ASC);

CREATE INDEX IF NOT EXISTS idx_events_host ON siem.events (host);
CREATE INDEX IF NOT EXISTS idx_events_user_name ON siem.events (user_name);
CREATE INDEX IF NOT EXISTS idx_events_severity ON siem.events (severity);
CREATE INDEX IF NOT EXISTS idx_events_source_type ON siem.events (source_type);
