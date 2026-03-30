CREATE DATABASE IF NOT EXISTS siem;

CREATE TABLE IF NOT EXISTS siem.events
(
    id String,
    timestamp DateTime,
    source_type String,
    host String,
    user_name String,
    src_ip String,
    dst_ip String,
    event_code String,
    severity Int32,
    message String,
    raw String
)
ENGINE = MergeTree
ORDER BY (timestamp, source_type, host, user_name);