-- 042_create_signatures_table.sql
-- 创建 signatures 表，用于存储 thinking block 签名池

CREATE TABLE IF NOT EXISTS signatures (
    id BIGSERIAL PRIMARY KEY,
    value TEXT NOT NULL,
    hash VARCHAR(64) NOT NULL UNIQUE,
    model VARCHAR(100),
    source VARCHAR(20) NOT NULL DEFAULT 'manual' CHECK (source IN ('collected', 'imported', 'manual')),
    status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled', 'expired')),
    use_count INTEGER NOT NULL DEFAULT 0,
    last_used_at TIMESTAMP,
    last_verified_at TIMESTAMP,
    notes TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_signatures_status ON signatures(status);
CREATE INDEX IF NOT EXISTS idx_signatures_source ON signatures(source);
CREATE INDEX IF NOT EXISTS idx_signatures_model ON signatures(model);
CREATE INDEX IF NOT EXISTS idx_signatures_use_count ON signatures(use_count);
CREATE INDEX IF NOT EXISTS idx_signatures_last_used_at ON signatures(last_used_at);
CREATE INDEX IF NOT EXISTS idx_signatures_deleted_at ON signatures(deleted_at);
