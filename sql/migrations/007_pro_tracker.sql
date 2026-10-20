-- =====================================================
-- 职业选手追踪 (/trackpro)
-- pro_accounts 既是职业选手账号库,也保存每个账号的追踪与对局状态。
-- 一个选手可能有多个账号,所以 pro_name 不唯一;Riot ID 不区分大小写唯一。
-- =====================================================
CREATE TABLE IF NOT EXISTS pro_accounts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

    -- 选手信息
    pro_name VARCHAR(50) NOT NULL,          -- 选手名,如 Faker
    team VARCHAR(50),                       -- 战队,可空

    -- Riot 账号
    riot_game_name VARCHAR(100) NOT NULL,
    riot_tag_line VARCHAR(10) NOT NULL,
    riot_puuid VARCHAR(100) UNIQUE,         -- 首次追踪时通过 account-v1 解析

    -- 追踪状态
    is_tracked BOOLEAN NOT NULL DEFAULT FALSE,
    track_channel_id VARCHAR(20),           -- 播报频道(执行 /trackpro add 的频道)
    tracked_by_discord_id VARCHAR(20),

    -- 对局状态(仅单双排)
    current_game_id BIGINT,
    current_game_started_at TIMESTAMP WITH TIME ZONE,
    current_champion_id BIGINT,
    last_game_id BIGINT,                    -- 上一局 ID,用于识别 spectator 抖动避免重复播报
    last_checked_at TIMESTAMP WITH TIME ZONE,

    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_pro_accounts_riot_id
    ON pro_accounts (LOWER(riot_game_name), LOWER(riot_tag_line));
CREATE INDEX IF NOT EXISTS idx_pro_accounts_pro_name ON pro_accounts (LOWER(pro_name));
CREATE INDEX IF NOT EXISTS idx_pro_accounts_tracked ON pro_accounts (is_tracked) WHERE is_tracked;

-- 预置选手账号(美服)
INSERT INTO pro_accounts (pro_name, riot_game_name, riot_tag_line) VALUES
    ('TheShy', 'dkdlwlghkdlxld', '1019'),
    ('Faker', 'Hide in bush', 'NYKR'),
    ('Zeus', 'Austin', 'lll')
ON CONFLICT DO NOTHING;
