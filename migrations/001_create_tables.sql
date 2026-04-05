-- migrations/001_create_tables.sql
-- +goose Up
CREATE TABLE IF NOT EXISTS gauges (
    name VARCHAR(255) PRIMARY KEY,
    value DOUBLE PRECISION,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS counters (
    name VARCHAR(255) PRIMARY KEY,
    value BIGINT,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Триггеры для авто-обновления updated_at (опционально)
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_gauges_updated_at
    BEFORE UPDATE ON gauges
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_counters_updated_at
    BEFORE UPDATE ON counters
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- +goose Down
DROP TRIGGER IF EXISTS update_counters_updated_at ON counters;
DROP TRIGGER IF EXISTS update_gauges_updated_at ON gauges;
DROP FUNCTION IF EXISTS update_updated_at_column();
DROP TABLE IF EXISTS counters;
DROP TABLE IF EXISTS gauges;