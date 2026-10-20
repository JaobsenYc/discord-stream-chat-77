-- =====================================================
-- Concurrency Constraints Migration
-- 数据库级唯一约束，兜底应用层 check-then-insert 的并发窗口：
-- 同一用户对同一竞猜只能下注一次（应用层 HasUserParticipated
-- 已有此规则，本迁移把规则落到数据库，防止并发双击绕过；
-- lottery_participants 在 001 初始 schema 中已有同类约束）
-- =====================================================

CREATE UNIQUE INDEX IF NOT EXISTS idx_bet_participants_unique_user
    ON bet_participants(bet_id, user_id);
