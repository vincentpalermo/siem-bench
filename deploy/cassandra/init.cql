CREATE KEYSPACE IF NOT EXISTS siem
WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 1};

CREATE TABLE IF NOT EXISTS siem.events (
    bucket text,
    ts timestamp,
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
    PRIMARY KEY ((bucket), ts, id)
) WITH CLUSTERING ORDER BY (ts DESC, id ASC);