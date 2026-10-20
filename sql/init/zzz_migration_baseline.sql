-- =====================================================
-- 迁移基线标记（必须按文件名排在 init 目录最后执行）
-- sql/init/ 下的 001-006 由 Postgres 容器首次启动时自动全量执行，
-- 此处把它们登记进 schema_migrations，使今后 cmd/migrate 管理的
-- 增量迁移（sql/migrations/007+）能正确识别基线、不会重复执行。
-- =====================================================

CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    applied_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

INSERT INTO schema_migrations (version, name) VALUES
    (1, '001_initial_schema'),
    (2, '002_betting_system'),
    (3, '003_riot_watch_system'),
    (4, '004_death_threshold_float'),
    (5, '005_rank_binding_system'),
    (6, '006_concurrency_constraints')
ON CONFLICT (version) DO NOTHING;
