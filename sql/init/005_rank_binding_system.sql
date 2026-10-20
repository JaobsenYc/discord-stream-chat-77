-- =====================================================
-- Riot Account Binding System Migration
-- User-level Riot account binding with verification
-- =====================================================

-- =====================================================
-- User Riot Bindings Table
-- Global bindings (not per-guild, max 3 per user)
-- =====================================================
CREATE TABLE IF NOT EXISTS user_riot_bindings (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

    -- Discord user (not FK to users table, just the discord_id string)
    discord_id VARCHAR(20) NOT NULL,

    -- Riot Account Info
    riot_puuid VARCHAR(100) UNIQUE NOT NULL,
    riot_id VARCHAR(100) NOT NULL,          -- Game name (e.g., "PlayerName")
    riot_tag VARCHAR(10) NOT NULL,          -- Tagline (e.g., "NA1")
    summoner_id VARCHAR(100),               -- For league API calls (region-specific)

    -- Verification
    is_verified BOOLEAN DEFAULT FALSE,
    verification_icon_id INT,               -- Expected profile icon ID for verification
    verification_expires_at TIMESTAMP WITH TIME ZONE,

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    -- Constraint: unique binding per discord user + riot account
    UNIQUE(discord_id, riot_puuid)
);

-- =====================================================
-- User Riot Ranks Table (Cached rank data)
-- =====================================================
CREATE TABLE IF NOT EXISTS user_riot_ranks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    binding_id UUID NOT NULL REFERENCES user_riot_bindings(id) ON DELETE CASCADE,

    -- Queue type
    queue_type VARCHAR(50) NOT NULL DEFAULT 'RANKED_SOLO_5x5',

    -- Rank data
    tier VARCHAR(20),                       -- IRON, BRONZE, SILVER, GOLD, etc.
    division VARCHAR(5),                    -- I, II, III, IV (NULL for MASTER+)
    league_points INT DEFAULT 0,
    wins INT DEFAULT 0,
    losses INT DEFAULT 0,

    -- Timestamps
    last_updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    UNIQUE(binding_id, queue_type)
);

-- =====================================================
-- Indexes
-- =====================================================

-- Discord ID index for user lookup
CREATE INDEX IF NOT EXISTS idx_riot_bindings_discord_id
    ON user_riot_bindings(discord_id);

-- PUUID index for quick lookup
CREATE INDEX IF NOT EXISTS idx_riot_bindings_puuid
    ON user_riot_bindings(riot_puuid);

-- Verified bindings index
CREATE INDEX IF NOT EXISTS idx_riot_bindings_verified
    ON user_riot_bindings(is_verified)
    WHERE is_verified = TRUE;

-- Pending verification index
CREATE INDEX IF NOT EXISTS idx_riot_bindings_pending_verification
    ON user_riot_bindings(verification_expires_at)
    WHERE is_verified = FALSE AND verification_expires_at IS NOT NULL;

-- Rank leaderboard index (for sorting by tier and LP)
CREATE INDEX IF NOT EXISTS idx_riot_ranks_leaderboard
    ON user_riot_ranks(tier, league_points DESC)
    WHERE queue_type = 'RANKED_SOLO_5x5';

-- Rank update index
CREATE INDEX IF NOT EXISTS idx_riot_ranks_update
    ON user_riot_ranks(last_updated_at);

-- Binding ID index for rank lookups
CREATE INDEX IF NOT EXISTS idx_riot_ranks_binding_id
    ON user_riot_ranks(binding_id);

-- =====================================================
-- Triggers
-- =====================================================
CREATE TRIGGER update_user_riot_bindings_updated_at
    BEFORE UPDATE ON user_riot_bindings
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- =====================================================
-- Comments
-- =====================================================
COMMENT ON TABLE user_riot_bindings IS 'User-level Riot account bindings (global, max 3 per user enforced in app)';
COMMENT ON COLUMN user_riot_bindings.riot_puuid IS 'Permanent unique ID, does not change even if player changes name';
COMMENT ON COLUMN user_riot_bindings.summoner_id IS 'Region-specific summoner ID for League API calls';
COMMENT ON COLUMN user_riot_bindings.verification_icon_id IS 'Profile icon ID user must set to verify ownership';

COMMENT ON TABLE user_riot_ranks IS 'Cached rank data for bound Riot accounts';
COMMENT ON COLUMN user_riot_ranks.tier IS 'Rank tier: IRON, BRONZE, SILVER, GOLD, PLATINUM, EMERALD, DIAMOND, MASTER, GRANDMASTER, CHALLENGER';
COMMENT ON COLUMN user_riot_ranks.division IS 'Rank division: I, II, III, IV (NULL for MASTER and above)';
