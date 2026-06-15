CREATE KEYSPACE IF NOT EXISTS siem
WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 1};

CREATE TABLE IF NOT EXISTS siem.events_by_day (
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

CREATE TABLE IF NOT EXISTS siem.events_by_host (
    host text,
    ts timestamp,
    id text,
    source_type text,
    user_name text,
    src_ip text,
    dst_ip text,
    event_code text,
    severity int,
    message text,
    raw text,
    PRIMARY KEY ((host), ts, id)
) WITH CLUSTERING ORDER BY (ts DESC, id ASC);

CREATE TABLE IF NOT EXISTS siem.events_by_user (
    user_name text,
    ts timestamp,
    id text,
    source_type text,
    host text,
    src_ip text,
    dst_ip text,
    event_code text,
    severity int,
    message text,
    raw text,
    PRIMARY KEY ((user_name), ts, id)
) WITH CLUSTERING ORDER BY (ts DESC, id ASC);

CREATE TABLE IF NOT EXISTS siem.severity_counts (
    severity int PRIMARY KEY,
    cnt counter
);

CREATE TABLE IF NOT EXISTS siem.host_counts (
    host text PRIMARY KEY,
    cnt counter
);
