-- =====================================================
-- Discord LFG Bot - 竞猜系统
-- =====================================================
-- 迁移文件: 002_betting_system.sql
-- 描述: LOL比赛竞猜系统数据库表结构
-- =====================================================

-- =====================================================
-- 竞猜表
-- =====================================================
CREATE TABLE IF NOT EXISTS bets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

    -- 基本信息
    name VARCHAR(200),                            -- 竞猜名称（可选）
    guild_id VARCHAR(20) NOT NULL,
    channel_id VARCHAR(20) NOT NULL,              -- 文字频道ID
    voice_channel_id VARCHAR(20),                 -- 语音频道ID（可选，Riot Watch 自动创建的竞猜为空）
    message_id VARCHAR(20),                       -- Discord消息ID
    creator_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    creator_discord_id VARCHAR(20) NOT NULL,

    -- 队伍信息
    team_a_name VARCHAR(100) NOT NULL,            -- 队伍A名称
    team_b_name VARCHAR(100) NOT NULL,            -- 队伍B名称

    -- 状态
    status VARCHAR(20) DEFAULT 'betting' CHECK (
        status IN ('betting', 'locked', 'settled', 'cancelled', 'expired')
    ),

    -- 下注统计
    team_a_total_bet INTEGER DEFAULT 0 CHECK (team_a_total_bet >= 0),
    team_b_total_bet INTEGER DEFAULT 0 CHECK (team_b_total_bet >= 0),
    total_participants INTEGER DEFAULT 0 CHECK (total_participants >= 0),

    -- 结果
    winner_team VARCHAR(1) CHECK (winner_team IN ('A', 'B')),  -- 获胜队伍
    settled_by UUID REFERENCES users(id),         -- 结算操作者
    settled_at TIMESTAMP WITH TIME ZONE,          -- 结算时间

    -- 时间
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    locked_at TIMESTAMP WITH TIME ZONE,           -- 锁定下注时间
    expires_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() + INTERVAL '24 hours',

    -- 元数据
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- =====================================================
-- 下注记录表
-- =====================================================
CREATE TABLE IF NOT EXISTS bet_participants (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    bet_id UUID NOT NULL REFERENCES bets(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    discord_id VARCHAR(20) NOT NULL,

    -- 下注信息
    team_choice VARCHAR(1) NOT NULL CHECK (team_choice IN ('A', 'B')),
    bet_amount INTEGER NOT NULL CHECK (bet_amount >= 10 AND bet_amount <= 1000),

    -- 结算信息
    payout_amount INTEGER DEFAULT 0 CHECK (payout_amount >= 0),  -- 获得的积分（包含本金）
    is_winner BOOLEAN,                            -- 是否获胜
    settled BOOLEAN DEFAULT FALSE,                -- 是否已结算

    -- 时间
    placed_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    settled_at TIMESTAMP WITH TIME ZONE,

    -- 唯一约束：每个用户每个竞猜只能下注一次
    UNIQUE(bet_id, user_id)
);

-- =====================================================
-- 创建索引
-- =====================================================

-- 竞猜表索引
CREATE INDEX IF NOT EXISTS idx_bets_guild_status ON bets(guild_id, status);
CREATE INDEX IF NOT EXISTS idx_bets_voice_channel ON bets(voice_channel_id, status);
CREATE INDEX IF NOT EXISTS idx_bets_creator ON bets(creator_id);
CREATE INDEX IF NOT EXISTS idx_bets_expires ON bets(expires_at) WHERE status IN ('betting', 'locked');
CREATE INDEX IF NOT EXISTS idx_bets_created_at ON bets(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_bets_message_id ON bets(message_id) WHERE message_id IS NOT NULL;

-- 下注记录表索引
CREATE INDEX IF NOT EXISTS idx_bet_participants_bet ON bet_participants(bet_id);
CREATE INDEX IF NOT EXISTS idx_bet_participants_user ON bet_participants(user_id);
CREATE INDEX IF NOT EXISTS idx_bet_participants_team ON bet_participants(bet_id, team_choice);
CREATE INDEX IF NOT EXISTS idx_bet_participants_placed_at ON bet_participants(placed_at DESC);
CREATE INDEX IF NOT EXISTS idx_bet_participants_settled ON bet_participants(settled);

-- =====================================================
-- 创建触发器
-- =====================================================

-- 更新 updated_at 字段
CREATE TRIGGER update_bets_updated_at
    BEFORE UPDATE ON bets
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- =====================================================
-- 创建视图
-- =====================================================

-- 竞猜统计视图
CREATE OR REPLACE VIEW bet_statistics AS
SELECT
    b.guild_id,
    COUNT(*) as total_bets,
    COUNT(CASE WHEN b.status = 'betting' THEN 1 END) as active_bets,
    COUNT(CASE WHEN b.status = 'locked' THEN 1 END) as locked_bets,
    COUNT(CASE WHEN b.status = 'settled' THEN 1 END) as settled_bets,
    COUNT(CASE WHEN b.status = 'cancelled' THEN 1 END) as cancelled_bets,
    COUNT(CASE WHEN b.status = 'expired' THEN 1 END) as expired_bets,
    COALESCE(SUM(b.team_a_total_bet + b.team_b_total_bet), 0) as total_bet_amount,
    COALESCE(AVG(b.total_participants), 0) as avg_participants
FROM bets b
GROUP BY b.guild_id;

-- 用户竞猜统计视图
CREATE OR REPLACE VIEW user_bet_statistics AS
SELECT
    u.id as user_id,
    u.discord_id,
    u.username,
    COUNT(DISTINCT bp.bet_id) as total_bets,
    COUNT(CASE WHEN bp.is_winner = TRUE THEN 1 END) as wins,
    COUNT(CASE WHEN bp.is_winner = FALSE THEN 1 END) as losses,
    COALESCE(SUM(bp.bet_amount), 0) as total_wagered,
    COALESCE(SUM(CASE WHEN bp.is_winner THEN bp.payout_amount ELSE 0 END), 0) as total_winnings,
    COALESCE(SUM(CASE WHEN bp.is_winner THEN bp.payout_amount ELSE -bp.bet_amount END), 0) as net_profit
FROM users u
LEFT JOIN bet_participants bp ON u.id = bp.user_id
GROUP BY u.id, u.discord_id, u.username;

-- =====================================================
-- 辅助函数
-- =====================================================

-- 获取竞猜的参与人数
CREATE OR REPLACE FUNCTION get_bet_participant_count(bet_uuid UUID)
RETURNS INTEGER AS $$
DECLARE
    participant_count INTEGER;
BEGIN
    SELECT COUNT(DISTINCT user_id) INTO participant_count
    FROM bet_participants
    WHERE bet_id = bet_uuid;

    RETURN COALESCE(participant_count, 0);
END;
$$ LANGUAGE plpgsql;

-- 获取竞猜某队的下注总额
CREATE OR REPLACE FUNCTION get_team_total_bet(bet_uuid UUID, team VARCHAR(1))
RETURNS INTEGER AS $$
DECLARE
    total_bet INTEGER;
BEGIN
    SELECT COALESCE(SUM(bet_amount), 0) INTO total_bet
    FROM bet_participants
    WHERE bet_id = bet_uuid AND team_choice = team;

    RETURN total_bet;
END;
$$ LANGUAGE plpgsql;

-- 检查用户是否已在竞猜中下注
CREATE OR REPLACE FUNCTION has_user_bet(bet_uuid UUID, user_uuid UUID)
RETURNS BOOLEAN AS $$
BEGIN
    RETURN EXISTS (
        SELECT 1
        FROM bet_participants
        WHERE bet_id = bet_uuid AND user_id = user_uuid
    );
END;
$$ LANGUAGE plpgsql;

-- =====================================================
-- 字段注释
-- =====================================================

COMMENT ON TABLE bets IS 'LOL比赛竞猜主表';
COMMENT ON COLUMN bets.name IS '竞猜名称，可选';
COMMENT ON COLUMN bets.voice_channel_id IS '语音频道ID（可选），用于验证和自动结束；Riot Watch 自动创建的竞猜为空';
COMMENT ON COLUMN bets.status IS '竞猜状态: betting=下注中, locked=已锁定, settled=已结算, cancelled=已取消, expired=已过期';
COMMENT ON COLUMN bets.team_a_total_bet IS '队伍A总下注额';
COMMENT ON COLUMN bets.team_b_total_bet IS '队伍B总下注额';
COMMENT ON COLUMN bets.winner_team IS '获胜队伍: A或B';
COMMENT ON COLUMN bets.expires_at IS '过期时间，24小时后自动退款';

COMMENT ON TABLE bet_participants IS '竞猜参与记录表';
COMMENT ON COLUMN bet_participants.team_choice IS '下注队伍: A或B';
COMMENT ON COLUMN bet_participants.bet_amount IS '下注金额，范围10-1000';
COMMENT ON COLUMN bet_participants.payout_amount IS '获得的总积分（包含本金和收益）';
COMMENT ON COLUMN bet_participants.is_winner IS '是否获胜';

-- =====================================================
-- 初始化完成
-- =====================================================
