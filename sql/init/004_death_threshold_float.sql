-- =====================================================
-- 修改 death_threshold 为 DECIMAL 类型
-- 支持 .5 阈值避免平局情况
-- =====================================================

-- 修改列类型从 INTEGER 到 DECIMAL(4,1)
-- 允许范围 0.5 到 20.5
ALTER TABLE riot_watch_bindings
    ALTER COLUMN death_threshold TYPE DECIMAL(4,1) USING death_threshold::DECIMAL(4,1);

-- 删除旧的约束
ALTER TABLE riot_watch_bindings
    DROP CONSTRAINT IF EXISTS riot_watch_bindings_death_threshold_check;

-- 添加新的约束
ALTER TABLE riot_watch_bindings
    ADD CONSTRAINT riot_watch_bindings_death_threshold_check
    CHECK (death_threshold >= 0.5 AND death_threshold <= 99.5);

-- 将现有的整数阈值转换为 .5 格式（避免平局）
UPDATE riot_watch_bindings
    SET death_threshold = death_threshold + 0.5
    WHERE death_threshold = FLOOR(death_threshold);

-- 更新默认值
ALTER TABLE riot_watch_bindings
    ALTER COLUMN death_threshold SET DEFAULT 5.5;

-- 更新注释
COMMENT ON COLUMN riot_watch_bindings.death_threshold IS '死亡数阈值，建议使用 .5 结尾避免平局（如 5.5 表示死亡 <=5 或 >=6）';
