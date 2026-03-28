CREATE TABLE IF NOT EXISTS analysis.runs
(
    id             BIGSERIAL PRIMARY KEY,
    status_code    INTEGER NOT NULL DEFAULT 0,
    status_message TEXT,
    exchange       TEXT    NOT NULL,
    category       TEXT    NOT NULL,
    symbol         TEXT    NOT NULL,
    interval       TEXT    NOT NULL,
    price_type     TEXT    NOT NULL,
    detector_code  TEXT    NOT NULL,
    detector_label TEXT    NOT NULL,
    detector_opts  JSONB   NOT NULL,
    from_time      TIMESTAMPTZ,
    to_time        TIMESTAMPTZ,
    signals_count  BIGINT           DEFAULT 0,
    avg_profit_ppm BIGINT,
    created_by     TEXT    NOT NULL,
    created_at     TIMESTAMPTZ      DEFAULT NOW(),
    is_shared      BOOLEAN NOT NULL DEFAULT FALSE,
    shared_at      TIMESTAMPTZ
);
