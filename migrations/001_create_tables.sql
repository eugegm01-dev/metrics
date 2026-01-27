-- +goose Up
CREATE TABLE IF NOT EXISTS gauges (
    name VARCHAR(255) PRIMARY KEY,
    value DOUBLE PRECISION
);

CREATE TABLE IF NOT EXISTS counters (
    name VARCHAR(255) PRIMARY KEY,
    value BIGINT
);
-- +goose Down
DROP TABLE IF EXISTS counters;
DROP TABLE IF EXISTS gauges;
