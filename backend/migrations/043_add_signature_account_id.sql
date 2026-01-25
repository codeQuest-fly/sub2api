-- 添加采集来源账号字段
ALTER TABLE signatures ADD COLUMN IF NOT EXISTS collected_from_account_id BIGINT NULL;

-- 添加索引以支持按账号筛选
CREATE INDEX IF NOT EXISTS idx_signatures_collected_from_account_id ON signatures(collected_from_account_id);
