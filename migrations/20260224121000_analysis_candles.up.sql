CREATE TABLE IF NOT EXISTS analysis.candles
(
    exchange    TEXT             NOT NULL,
    category    TEXT             NOT NULL,
    symbol      TEXT             NOT NULL,
    interval    INTEGER          NOT NULL,
    time        TIMESTAMPTZ      NOT NULL,
    open_price  BIGINT           NOT NULL,
    high_price  BIGINT           NOT NULL,
    low_price   BIGINT           NOT NULL,
    close_price BIGINT           NOT NULL,
    volume      BIGINT           NOT NULL,
    turnover    DOUBLE PRECISION NOT NULL,
    price_type  TEXT             NOT NULL,
    PRIMARY KEY (exchange, category, symbol, interval, time, price_type)
);

CREATE TABLE IF NOT EXISTS analysis.candles_staging
(
    exchange    TEXT             NOT NULL,
    category    TEXT             NOT NULL,
    symbol      TEXT             NOT NULL,
    interval    INTEGER          NOT NULL,
    time        TIMESTAMPTZ      NOT NULL,
    open_price  BIGINT           NOT NULL,
    high_price  BIGINT           NOT NULL,
    low_price   BIGINT           NOT NULL,
    close_price BIGINT           NOT NULL,
    volume      BIGINT           NOT NULL,
    turnover    DOUBLE PRECISION NOT NULL,
    price_type  TEXT             NOT NULL
);

SELECT create_hypertable('analysis.candles', 'time', if_not_exists => TRUE);
