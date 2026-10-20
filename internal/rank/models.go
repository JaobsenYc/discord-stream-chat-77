package rank

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// RiotBinding represents a user's Riot account binding
type RiotBinding struct {
	ID                    uuid.UUID  `db:"id" json:"id"`
	DiscordID             string     `db:"discord_id" json:"discord_id"`
	RiotPUUID             string     `db:"riot_puuid" json:"riot_puuid"`
	RiotID                string     `db:"riot_id" json:"riot_id"`
	RiotTag               string     `db:"riot_tag" json:"riot_tag"`
	SummonerID            *string    `db:"summoner_id" json:"summoner_id,omitempty"`
	IsVerified            bool       `db:"is_verified" json:"is_verified"`
	VerificationIconID    *int       `db:"verification_icon_id" json:"verification_icon_id,omitempty"`
	VerificationExpiresAt *time.Time `db:"verification_expires_at" json:"verification_expires_at,omitempty"`
	CreatedAt             time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt             time.Time  `db:"updated_at" json:"updated_at"`
}

// RiotRank represents cached rank data for a binding
type RiotRank struct {
	ID            uuid.UUID `db:"id" json:"id"`
	BindingID     uuid.UUID `db:"binding_id" json:"binding_id"`
	QueueType     string    `db:"queue_type" json:"queue_type"`
	Tier          *string   `db:"tier" json:"tier,omitempty"`
	Division      *string   `db:"division" json:"division,omitempty"`
	LeaguePoints  int       `db:"league_points" json:"league_points"`
	Wins          int       `db:"wins" json:"wins"`
	Losses        int       `db:"losses" json:"losses"`
	LastUpdatedAt time.Time `db:"last_updated_at" json:"last_updated_at"`
	CreatedAt     time.Time `db:"created_at" json:"created_at"`
}

// LeaderboardEntry represents a single entry in the leaderboard
type LeaderboardEntry struct {
	Rank         int     `json:"rank" db:"-"`
	DiscordID    string  `json:"discord_id" db:"discord_id"`
	RiotID       string  `json:"riot_id" db:"riot_id"`
	RiotTag      string  `json:"riot_tag" db:"riot_tag"`
	Tier         string  `json:"tier" db:"tier"`
	Division     string  `json:"division" db:"division"`
	LeaguePoints int     `json:"league_points" db:"league_points"`
	Wins         int     `json:"wins" db:"wins"`
	Losses       int     `json:"losses" db:"losses"`
	WinRate      float64 `json:"win_rate" db:"-"`
}

// UserRankInfo combines binding and rank data for user display
type UserRankInfo struct {
	Binding RiotBinding
	Rank    *RiotRank
}

// Constants
const (
	// BindingChannelID is the only channel where binding commands work
	BindingChannelID = "1459129899636756765"

	// MaxBindingsPerUser is the maximum number of Riot accounts a user can bind
	MaxBindingsPerUser = 3

	// VerificationTimeout is how long a verification code is valid
	VerificationTimeout = 10 * time.Minute

	// QueueTypeSoloDuo is the queue type for Solo/Duo ranked
	QueueTypeSoloDuo = "RANKED_SOLO_5x5"
)

// VerificationIcons is a list of safe profile icon IDs that exist in all patches
// These are the default icons available to all accounts
var VerificationIcons = []int{
	0, 1, 2, 3, 4, 5, 6, 7, 8, 9,
	10, 11, 12, 13, 14, 15, 16, 17, 18, 19,
	20, 21, 22, 23, 24, 25, 26, 27, 28, 29,
}

// TierPriority maps tier names to priority values for sorting (higher = better)
var TierPriority = map[string]int{
	"CHALLENGER":  10,
	"GRANDMASTER": 9,
	"MASTER":      8,
	"DIAMOND":     7,
	"EMERALD":     6,
	"PLATINUM":    5,
	"GOLD":        4,
	"SILVER":      3,
	"BRONZE":      2,
	"IRON":        1,
	"":            0, // Unranked
}

// DivisionPriority maps division names to priority values (higher = better)
var DivisionPriority = map[string]int{
	"I":   4,
	"II":  3,
	"III": 2,
	"IV":  1,
	"":    0,
}

// TierEmoji returns an emoji/badge for the tier
func TierEmoji(tier string) string {
	switch tier {
	case "CHALLENGER":
		return "<:challenger:>"
	case "GRANDMASTER":
		return "<:grandmaster:>"
	case "MASTER":
		return "<:master:>"
	case "DIAMOND":
		return "<:diamond:>"
	case "EMERALD":
		return "<:emerald:>"
	case "PLATINUM":
		return "<:platinum:>"
	case "GOLD":
		return "<:gold:>"
	case "SILVER":
		return "<:silver:>"
	case "BRONZE":
		return "<:bronze:>"
	case "IRON":
		return "<:iron:>"
	default:
		return ""
	}
}

// TierDisplay returns a formatted display string for tier
func TierDisplay(tier string) string {
	switch tier {
	case "CHALLENGER":
		return "Challenger"
	case "GRANDMASTER":
		return "Grandmaster"
	case "MASTER":
		return "Master"
	case "DIAMOND":
		return "Diamond"
	case "EMERALD":
		return "Emerald"
	case "PLATINUM":
		return "Platinum"
	case "GOLD":
		return "Gold"
	case "SILVER":
		return "Silver"
	case "BRONZE":
		return "Bronze"
	case "IRON":
		return "Iron"
	default:
		return "Unranked"
	}
}

// RankDisplay returns a formatted rank string like "Diamond I 75 LP"
func RankDisplay(tier, division string, lp int) string {
	if tier == "" {
		return "Unranked"
	}
	tierStr := TierDisplay(tier)
	// Master, Grandmaster, Challenger don't have divisions
	if tier == "MASTER" || tier == "GRANDMASTER" || tier == "CHALLENGER" {
		return fmt.Sprintf("%s %d LP", tierStr, lp)
	}
	return fmt.Sprintf("%s %s %d LP", tierStr, division, lp)
}

// CalculateWinRate calculates win rate percentage
func CalculateWinRate(wins, losses int) float64 {
	total := wins + losses
	if total == 0 {
		return 0
	}
	return float64(wins) / float64(total) * 100
}
