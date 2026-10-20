-- =====================================================
-- Discord LFG Bot 数据库初始化脚本
-- =====================================================

-- 启用必要的扩展
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";

-- =====================================================
-- 用户表
-- =====================================================
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    discord_id VARCHAR(20) UNIQUE NOT NULL,
    username VARCHAR(100) NOT NULL,
    discriminator VARCHAR(10),
    avatar_url TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    -- 用户统计
    total_teams_created INTEGER DEFAULT 0,
    total_teams_joined INTEGER DEFAULT 0,
    total_check_ins INTEGER DEFAULT 0,
    
    -- 用户设置
    timezone VARCHAR(50) DEFAULT 'UTC',
    language VARCHAR(10) DEFAULT 'zh-CN',
    preferences JSONB DEFAULT '{}',
    
    -- 索引
    CONSTRAINT users_discord_id_check CHECK (discord_id ~ '^[0-9]+$')
);

-- =====================================================
-- 签到系统
-- =====================================================
CREATE TABLE check_ins (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    discord_id VARCHAR(20) NOT NULL, -- 冗余字段，便于查询
    guild_id VARCHAR(20) NOT NULL,
    channel_id VARCHAR(20) NOT NULL,
    
    check_in_date DATE NOT NULL,
    check_in_time TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    -- 签到奖励
    points_earned INTEGER DEFAULT 1,
    consecutive_days INTEGER DEFAULT 1,
    
    -- 元数据
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    -- 唯一约束：每个用户每天只能签到一次
    UNIQUE(user_id, guild_id, check_in_date)
);

-- =====================================================
-- 抽奖系统
-- =====================================================
CREATE TABLE lotteries (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(200) NOT NULL,
    description TEXT,
    guild_id VARCHAR(20) NOT NULL,
    channel_id VARCHAR(20) NOT NULL,
    created_by UUID NOT NULL REFERENCES users(id),
    
    -- 抽奖设置
    max_participants INTEGER DEFAULT 0, -- 0 表示无限制
    cost_points INTEGER DEFAULT 0, -- 参与消耗的积分
    prize_description TEXT NOT NULL,
    
    -- 时间设置
    start_time TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    end_time TIMESTAMP WITH TIME ZONE NOT NULL,
    
    -- 状态
    status VARCHAR(20) DEFAULT 'active' CHECK (status IN ('active', 'ended', 'cancelled')),
    winner_count INTEGER DEFAULT 1 CHECK (winner_count > 0),
    winners JSONB DEFAULT '[]'::jsonb,
    winner_selected_at TIMESTAMP WITH TIME ZONE,
    
    -- 元数据
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- =====================================================
-- 抽奖参与记录
-- =====================================================
CREATE TABLE lottery_participants (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    lottery_id UUID NOT NULL REFERENCES lotteries(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    discord_id VARCHAR(20) NOT NULL,
    
    -- 参与信息
    points_spent INTEGER DEFAULT 0,
    participated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    -- 唯一约束：每个用户每个抽奖只能参与一次
    UNIQUE(lottery_id, user_id)
);

-- =====================================================
-- 团队记录 (扩展现有功能)
-- =====================================================
CREATE TABLE teams (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    team_id VARCHAR(100) NOT NULL, -- 原有的团队ID
    guild_id VARCHAR(20) NOT NULL,
    text_channel_id VARCHAR(20) NOT NULL,
    voice_channel_id VARCHAR(20),
    
    -- 团队信息
    game_name VARCHAR(100) NOT NULL,
    max_players INTEGER NOT NULL,
    current_players INTEGER DEFAULT 0,
    creator_id UUID NOT NULL REFERENCES users(id),
    creator_discord_id VARCHAR(20) NOT NULL,
    
    -- 状态
    status VARCHAR(20) DEFAULT 'active' CHECK (status IN ('active', 'full', 'disbanded')),
    
    -- 时间戳
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    disbanded_at TIMESTAMP WITH TIME ZONE,
    
    -- 统计
    total_join_requests INTEGER DEFAULT 0,
    average_fill_time INTERVAL
);

-- =====================================================
-- 团队成员记录
-- =====================================================
CREATE TABLE team_members (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    team_id UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    discord_id VARCHAR(20) NOT NULL,
    
    -- 加入信息
    joined_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    left_at TIMESTAMP WITH TIME ZONE,
    duration INTERVAL GENERATED ALWAYS AS (left_at - joined_at) STORED,
    
    -- 状态
    is_active BOOLEAN DEFAULT TRUE,
    
    -- 唯一约束：活跃状态下，每个用户每个团队只能有一条记录
    UNIQUE(team_id, user_id, is_active) DEFERRABLE
);

-- =====================================================
-- 用户积分系统
-- =====================================================
CREATE TABLE user_points (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    guild_id VARCHAR(20) NOT NULL,
    
    -- 积分信息
    total_points INTEGER DEFAULT 0,
    available_points INTEGER DEFAULT 0, -- 可用积分（总积分 - 已消费）
    lifetime_earned INTEGER DEFAULT 0,
    lifetime_spent INTEGER DEFAULT 0,
    
    -- 等级系统
    level INTEGER DEFAULT 1,
    experience INTEGER DEFAULT 0,
    
    -- 时间戳
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    -- 唯一约束
    UNIQUE(user_id, guild_id)
);

-- =====================================================
-- 积分变动记录
-- =====================================================
CREATE TABLE point_transactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    guild_id VARCHAR(20) NOT NULL,
    
    -- 交易信息
    amount INTEGER NOT NULL, -- 正数为获得，负数为消费
    transaction_type VARCHAR(50) NOT NULL, -- 'check_in', 'lottery_participation', 'team_creation', etc.
    description TEXT,
    
    -- 关联信息
    related_id UUID, -- 可以关联到签到、抽奖等记录的ID
    related_type VARCHAR(50), -- 'check_in', 'lottery', 'team', etc.
    
    -- 时间戳
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- =====================================================
-- 服务器配置
-- =====================================================
CREATE TABLE guild_configs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    guild_id VARCHAR(20) UNIQUE NOT NULL,
    
    -- 签到配置
    check_in_enabled BOOLEAN DEFAULT TRUE,
    check_in_points INTEGER DEFAULT 1,
    check_in_channel_id VARCHAR(20),
    consecutive_bonus_enabled BOOLEAN DEFAULT TRUE,
    max_consecutive_bonus INTEGER DEFAULT 7,
    
    -- 抽奖配置
    lottery_enabled BOOLEAN DEFAULT TRUE,
    default_lottery_duration INTERVAL DEFAULT '24 hours',
    min_lottery_duration INTERVAL DEFAULT '1 hour',
    max_lottery_duration INTERVAL DEFAULT '7 days',
    
    -- 积分配置
    points_enabled BOOLEAN DEFAULT TRUE,
    team_creation_points INTEGER DEFAULT 5,
    team_completion_points INTEGER DEFAULT 10,
    
    -- 时间戳
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- =====================================================
-- 创建索引
-- =====================================================

-- 用户相关索引
CREATE INDEX idx_users_discord_id ON users(discord_id);
CREATE INDEX idx_users_created_at ON users(created_at DESC);

-- 签到相关索引
CREATE INDEX idx_check_ins_user_date ON check_ins(user_id, check_in_date DESC);
CREATE INDEX idx_check_ins_discord_id ON check_ins(discord_id);
CREATE INDEX idx_check_ins_guild_date ON check_ins(guild_id, check_in_date DESC);

-- 抽奖相关索引
CREATE INDEX idx_lotteries_guild_status ON lotteries(guild_id, status);
CREATE INDEX idx_lotteries_end_time ON lotteries(end_time);
CREATE INDEX idx_lotteries_winners_gin ON lotteries USING GIN (winners);
CREATE INDEX idx_lottery_participants_lottery ON lottery_participants(lottery_id);
CREATE INDEX idx_lottery_participants_user ON lottery_participants(user_id);

-- 团队相关索引
CREATE INDEX idx_teams_guild_status ON teams(guild_id, status);
CREATE INDEX idx_teams_creator ON teams(creator_id);
CREATE INDEX idx_teams_created_at ON teams(created_at DESC);
CREATE INDEX idx_team_members_team_active ON team_members(team_id, is_active);
CREATE INDEX idx_team_members_user_active ON team_members(user_id, is_active);

-- 积分相关索引
CREATE INDEX idx_user_points_guild ON user_points(guild_id, total_points DESC);
CREATE INDEX idx_point_transactions_user ON point_transactions(user_id, created_at DESC);
CREATE INDEX idx_point_transactions_type ON point_transactions(transaction_type, created_at DESC);

-- =====================================================
-- 创建触发器函数
-- =====================================================

-- 更新 updated_at 字段的函数
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- 为需要的表创建 updated_at 触发器
CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_lotteries_updated_at BEFORE UPDATE ON lotteries
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_teams_updated_at BEFORE UPDATE ON teams
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_user_points_updated_at BEFORE UPDATE ON user_points
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_guild_configs_updated_at BEFORE UPDATE ON guild_configs
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- =====================================================
-- 初始化数据
-- =====================================================

-- 可以在这里插入默认的服务器配置等数据
-- INSERT INTO guild_configs (guild_id) VALUES ('your_guild_id_here');

-- =====================================================
-- 创建视图
-- =====================================================

-- 用户签到统计视图
CREATE VIEW user_check_in_stats AS
SELECT 
    u.discord_id,
    u.username,
    COUNT(c.id) as total_check_ins,
    MAX(c.consecutive_days) as max_consecutive_days,
    MAX(c.check_in_date) as last_check_in_date,
    SUM(c.points_earned) as total_points_from_check_ins
FROM users u
LEFT JOIN check_ins c ON u.id = c.user_id
GROUP BY u.id, u.discord_id, u.username;

-- 活跃团队统计视图
CREATE VIEW active_team_stats AS
SELECT 
    t.guild_id,
    t.game_name,
    COUNT(*) as active_teams,
    AVG(t.current_players) as avg_current_players,
    AVG(t.max_players) as avg_max_players
FROM teams t
WHERE t.status = 'active'
GROUP BY t.guild_id, t.game_name;

-- 用户积分排行榜视图
CREATE VIEW user_leaderboard AS
SELECT 
    u.discord_id,
    u.username,
    up.total_points,
    up.level,
    up.experience,
    RANK() OVER (PARTITION BY up.guild_id ORDER BY up.total_points DESC) as rank
FROM users u
JOIN user_points up ON u.id = up.user_id
ORDER BY up.guild_id, up.total_points DESC;

-- 语音会话表
CREATE TABLE voice_sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    discord_id VARCHAR(20) NOT NULL,
    guild_id VARCHAR(20) NOT NULL,
    channel_id VARCHAR(20),
    channel_name VARCHAR(100),
    joined_at TIMESTAMP WITH TIME ZONE NOT NULL,
    left_at TIMESTAMP WITH TIME ZONE,
    duration INTERVAL GENERATED ALWAYS AS (left_at - joined_at) STORED,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 语音会话索引
CREATE INDEX idx_voice_sessions_user_id ON voice_sessions(user_id);
CREATE INDEX idx_voice_sessions_discord_id ON voice_sessions(discord_id);
CREATE INDEX idx_voice_sessions_guild_id ON voice_sessions(guild_id);
CREATE INDEX idx_voice_sessions_channel_id ON voice_sessions(channel_id);
CREATE INDEX idx_voice_sessions_joined_at ON voice_sessions(joined_at);
CREATE INDEX idx_voice_sessions_active ON voice_sessions(is_active) WHERE is_active = true;
CREATE INDEX idx_voice_sessions_user_guild ON voice_sessions(user_id, guild_id);
-- 注意：DATE(timestamptz) 非 IMMUTABLE，不能做索引表达式；复合索引同样支持按时间范围查询
CREATE INDEX idx_voice_sessions_guild_date ON voice_sessions(guild_id, joined_at);

-- 语音统计视图
CREATE VIEW user_voice_stats AS
SELECT 
    u.id as user_id,
    u.discord_id,
    u.username,
    vs.guild_id,
    COUNT(vs.id) as total_sessions,
    COALESCE(SUM(EXTRACT(EPOCH FROM vs.duration)), 0) as total_seconds,
    COALESCE(AVG(EXTRACT(EPOCH FROM vs.duration)), 0) as avg_session_seconds,
    MAX(vs.joined_at) as last_voice_activity,
    COUNT(CASE WHEN vs.joined_at >= CURRENT_DATE THEN 1 END) as today_sessions,
    COALESCE(SUM(CASE WHEN vs.joined_at >= CURRENT_DATE THEN EXTRACT(EPOCH FROM vs.duration) ELSE 0 END), 0) as today_seconds
FROM users u
LEFT JOIN voice_sessions vs ON u.id = vs.user_id AND vs.left_at IS NOT NULL
GROUP BY u.id, u.discord_id, u.username, vs.guild_id;

-- =====================================================
-- 成就系统
-- =====================================================

-- 成就定义表
CREATE TABLE achievements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('checkin', 'voice', 'message', 'lottery', 'lottery_win', 'team', 'team_created', 'activity')),
    target_value INT NOT NULL DEFAULT 1,
    reward_type TEXT NOT NULL CHECK (reward_type IN ('points', 'role', 'badge')),
    reward_value TEXT NOT NULL,
    icon_emoji TEXT,
    category TEXT NOT NULL DEFAULT 'general',
    display_order INT NOT NULL DEFAULT 0,
    is_hidden BOOLEAN NOT NULL DEFAULT FALSE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- 用户成就进度表
CREATE TABLE user_achievements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    achievement_id UUID NOT NULL REFERENCES achievements(id) ON DELETE CASCADE,
    progress INT NOT NULL DEFAULT 0,
    unlocked BOOLEAN NOT NULL DEFAULT FALSE,
    unlocked_at TIMESTAMP WITH TIME ZONE,
    reward_claimed BOOLEAN NOT NULL DEFAULT FALSE,
    reward_claimed_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, achievement_id)
);

-- 成就奖励发放记录表
CREATE TABLE achievement_rewards (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_achievement_id UUID NOT NULL REFERENCES user_achievements(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    guild_id TEXT NOT NULL,
    reward_type TEXT NOT NULL,
    reward_value TEXT NOT NULL,
    points_awarded INT DEFAULT 0,
    role_id TEXT,
    badge_name TEXT,
    processed BOOLEAN NOT NULL DEFAULT FALSE,
    processed_at TIMESTAMP WITH TIME ZONE,
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- 抽奖池与成就关联表（扩展现有抽奖系统）
CREATE TABLE lottery_achievement_requirements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    lottery_id UUID NOT NULL REFERENCES lotteries(id) ON DELETE CASCADE,
    achievement_id UUID NOT NULL REFERENCES achievements(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(lottery_id, achievement_id)
);

-- 成就系统索引
CREATE INDEX idx_user_achievements_user_id ON user_achievements(user_id);
CREATE INDEX idx_user_achievements_achievement_id ON user_achievements(achievement_id);
CREATE INDEX idx_user_achievements_unlocked ON user_achievements(unlocked);
CREATE INDEX idx_user_achievements_progress ON user_achievements(user_id, unlocked, progress);
CREATE INDEX idx_achievements_type ON achievements(type);
CREATE INDEX idx_achievements_active ON achievements(is_active);
CREATE INDEX idx_achievement_rewards_user_id ON achievement_rewards(user_id);
CREATE INDEX idx_achievement_rewards_processed ON achievement_rewards(processed);

-- 成就系统触发器
CREATE TRIGGER update_achievements_updated_at 
    BEFORE UPDATE ON achievements 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_user_achievements_updated_at 
    BEFORE UPDATE ON user_achievements 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- 插入初始成就数据（茶主题）
INSERT INTO achievements (name, description, type, target_value, reward_type, reward_value, icon_emoji, category, display_order) VALUES
-- 签到类成就
('初识茶香', '初次踏入茶道之旅，品味第一口清香', 'checkin', 1, 'points', '50', '🫖', 'checkin', 1),
('连饮七日', '一周不忘，每日一饮，成习惯也', 'checkin', 7, 'points', '100', '🍃', 'checkin', 2),
('不离不弃', '如老茶沉香，日久弥坚', 'checkin', 30, 'points', '300', '🍂', 'checkin', 3),

-- 语音类成就
('语茶有声', '茶话一席，声落杯中', 'voice', 3600, 'points', '100', '🎤', 'voice', 4),
('长谈对饮', '与友对谈，茶伴夜长', 'voice', 86400, 'points', '500', '🕰️', 'voice', 5),

-- 消息类成就
('以茶会友', '云起茶香，你是群中最会说的那位', 'message', 100, 'points', '150', '📢', 'message', 6),
('茶会主持人', '无论文字或语音，处处可见你煮茶论道', 'message', 1000, 'role', 'tea_master_role_id', '🍵', 'message', 7),

-- 抽奖类成就
('幸运首抽', '参与任意一次抽奖', 'lottery', 1, 'points', '50', '🎲', 'lottery', 8),
('连胜王者', '连续三次抽奖中得大奖', 'lottery_win', 3, 'role', 'winner_role_id', '👑', 'lottery', 9),

-- 组队成就类成就
('一壶初遇', '初次组队，如同与茶友初次共饮', 'team_created', 5, 'points', '100', '🫖', 'team', 10),
('共煮清欢', '与友共煮茶汤，乐在清欢', 'team_created', 25, 'points', '300', '🍂', 'team', 11),
('茶半香满席', '人未满，香已溢，社交开始频繁', 'team_created', 50, 'points', '500', '🌿', 'team', 12),
('邀月对酌', '志趣相投之人如月下共饮', 'team_created', 100, 'points', '800', '🍃', 'team', 13),
('茶会宗主', '社群活跃领导者，茶局核心', 'team_created', 200, 'role', 'team_master_role_id', '🀄', 'team', 14),

-- 活跃度类成就
('信手拈茶', '你已习惯在茶馆游走，茶气裹身', 'activity', 10, 'points', '200', '🎴', 'activity', 15),
('茶道宗师', '无茶不欢，炉火纯青', 'activity', 100, 'badge', 'tea_master', '☕', 'activity', 16);

-- =====================================================
-- 抽奖系统辅助函数
-- =====================================================

-- 检查用户是否已中奖的辅助函数
CREATE OR REPLACE FUNCTION is_user_lottery_winner(lottery_uuid UUID, user_uuid UUID)
RETURNS BOOLEAN AS $$
BEGIN
    RETURN EXISTS (
        SELECT 1 
        FROM lotteries 
        WHERE id = lottery_uuid 
        AND winners @> jsonb_build_array(jsonb_build_object('user_id', user_uuid::text))
    );
END;
$$ LANGUAGE plpgsql;

-- 添加字段注释
COMMENT ON COLUMN lotteries.winner_count IS '中奖人数，默认1人';
COMMENT ON COLUMN lotteries.winners IS 'JSON数组存储获奖者信息：[{user_id, discord_id, username, selected_at}]';

 