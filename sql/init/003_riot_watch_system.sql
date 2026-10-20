-- =====================================================
-- Riot Watch System Migration
-- 自动竞猜系统 - 监控 Riot 账号游戏状态
-- =====================================================

-- 启用 UUID 扩展（如果尚未启用）
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- =====================================================
-- Riot Watch Bindings 表
-- 存储频道与 Riot 账号的绑定关系
-- =====================================================
CREATE TABLE IF NOT EXISTS riot_watch_bindings (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

    -- Discord 上下文
    guild_id VARCHAR(20) NOT NULL,
    channel_id VARCHAR(20) NOT NULL,
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_by_discord_id VARCHAR(20) NOT NULL,

    -- Riot 账号信息
    riot_id VARCHAR(100) NOT NULL,              -- 玩家名称 (如 "PlayerName")
    riot_tag VARCHAR(10) NOT NULL,              -- Tag (如 "NA1")
    riot_puuid VARCHAR(100),                    -- 永久唯一ID (用于API查询)

    -- 竞猜配置
    death_threshold INTEGER NOT NULL DEFAULT 5 CHECK (death_threshold >= 1 AND death_threshold <= 20),

    -- 状态追踪
    is_active BOOLEAN DEFAULT TRUE,
    current_game_id VARCHAR(100),               -- 当前游戏ID (null = 不在游戏中)
    current_win_bet_id UUID REFERENCES bets(id) ON DELETE SET NULL,
    current_death_bet_id UUID REFERENCES bets(id) ON DELETE SET NULL,
    last_checked_at TIMESTAMP WITH TIME ZONE,
    last_game_ended_at TIMESTAMP WITH TIME ZONE,

    -- 时间戳
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    -- 唯一约束：一个频道一个 Riot 账号只能绑定一次
    UNIQUE(guild_id, channel_id, riot_puuid)
);

-- =====================================================
-- 索引
-- =====================================================

-- 活跃绑定查询索引
CREATE INDEX IF NOT EXISTS idx_riot_watch_guild_active
    ON riot_watch_bindings(guild_id, is_active)
    WHERE is_active = TRUE;

-- 全局活跃绑定索引
CREATE INDEX IF NOT EXISTS idx_riot_watch_active
    ON riot_watch_bindings(is_active)
    WHERE is_active = TRUE;

-- 当前游戏索引（用于检查游戏结束）
CREATE INDEX IF NOT EXISTS idx_riot_watch_in_game
    ON riot_watch_bindings(current_game_id)
    WHERE current_game_id IS NOT NULL;

-- PUUID 索引
CREATE INDEX IF NOT EXISTS idx_riot_watch_puuid
    ON riot_watch_bindings(riot_puuid);

-- 频道索引
CREATE INDEX IF NOT EXISTS idx_riot_watch_channel
    ON riot_watch_bindings(guild_id, channel_id);

-- =====================================================
-- 触发器 - 自动更新 updated_at
-- =====================================================
CREATE TRIGGER update_riot_watch_bindings_updated_at
    BEFORE UPDATE ON riot_watch_bindings
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- =====================================================
-- 注释
-- =====================================================
COMMENT ON TABLE riot_watch_bindings IS 'Riot 账号绑定表 - 用于自动竞猜系统';
COMMENT ON COLUMN riot_watch_bindings.riot_puuid IS '永久唯一ID，即使玩家改名也不变';
COMMENT ON COLUMN riot_watch_bindings.death_threshold IS '死亡数阈值，超过此值触发"超过"结果';
COMMENT ON COLUMN riot_watch_bindings.current_game_id IS '当前游戏ID，用于追踪游戏状态';
COMMENT ON COLUMN riot_watch_bindings.current_win_bet_id IS '当前输赢竞猜ID';
COMMENT ON COLUMN riot_watch_bindings.current_death_bet_id IS '当前死亡数竞猜ID';
