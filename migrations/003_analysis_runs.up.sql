CREATE TABLE IF NOT EXISTS analysis.runs
(
    id                UUID PRIMARY KEY,
    status_code       INTEGER     NOT NULL DEFAULT 0,
    status_message    TEXT,
    exchange          TEXT        NOT NULL,
    category          TEXT        NOT NULL,
    symbol            TEXT        NOT NULL,
    interval          TEXT        NOT NULL,
    detector_code     TEXT        NOT NULL,
    detector_version  TEXT        NOT NULL,
    detector_label    TEXT        NOT NULL,
    detector_opts     JSONB       NOT NULL,
    first_candle_time TIMESTAMPTZ NOT NULL,
    last_candle_time  TIMESTAMPTZ NOT NULL,
    signals_count     BIGINT               DEFAULT 0,
    avg_profit_ppm    BIGINT,
    created_by        TEXT        NOT NULL,
    created_at        TIMESTAMPTZ          DEFAULT NOW(),
    is_shared         BOOLEAN     NOT NULL DEFAULT FALSE,
    shared_at         TIMESTAMPTZ
);
