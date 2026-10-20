package rank

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jaobsenyc/discord-lfg-bot-go/internal/riotwatch"
)

// Summoner represents Riot Summoner API response
type Summoner struct {
	ID            string `json:"id"`            // Encrypted summoner ID (region-specific)
	AccountID     string `json:"accountId"`     // Encrypted account ID
	PUUID         string `json:"puuid"`         // Permanent unique ID
	ProfileIconID int    `json:"profileIconId"` // Current profile icon ID
	RevisionDate  int64  `json:"revisionDate"`  // Date of last modification (epoch ms)
	SummonerLevel int64  `json:"summonerLevel"` // Summoner level
}

// LeagueEntry represents a single ranked queue entry from Riot League API
type LeagueEntry struct {
	LeagueID     string `json:"leagueId"`
	SummonerID   string `json:"summonerId"`
	QueueType    string `json:"queueType"`    // RANKED_SOLO_5x5, RANKED_FLEX_SR, etc.
	Tier         string `json:"tier"`         // IRON, BRONZE, SILVER, GOLD, etc.
	Rank         string `json:"rank"`         // I, II, III, IV (this is actually the division)
	LeaguePoints int    `json:"leaguePoints"` // LP
	Wins         int    `json:"wins"`
	Losses       int    `json:"losses"`
	HotStreak    bool   `json:"hotStreak"`
	Veteran      bool   `json:"veteran"`
	FreshBlood   bool   `json:"freshBlood"`
	Inactive     bool   `json:"inactive"`
}

// RankAPIClient wraps RiotAPIClient with rank-specific methods
type RankAPIClient struct {
	client *riotwatch.RiotAPIClient
}

// NewRankAPIClient creates a new RankAPIClient wrapping the existing RiotAPIClient
func NewRankAPIClient(client *riotwatch.RiotAPIClient) *RankAPIClient {
	return &RankAPIClient{client: client}
}

// GetAccountByRiotID fetches account info by Riot ID (game name + tag)
func (c *RankAPIClient) GetAccountByRiotID(ctx context.Context, gameName, tagLine string) (*riotwatch.Account, error) {
	return c.client.GetAccountByRiotID(ctx, gameName, tagLine)
}

// GetSummonerByPUUID fetches summoner data including profile icon ID
// This uses the region endpoint (na1, euw1, etc.), not platform (americas, europe)
func (c *RankAPIClient) GetSummonerByPUUID(ctx context.Context, puuid string) (*Summoner, error) {
	apiURL := fmt.Sprintf(
		"https://%s.api.riotgames.com/lol/summoner/v4/summoners/by-puuid/%s",
		c.client.Region(), puuid,
	)

	body, err := c.client.DoRequest(ctx, apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get summoner: %w", err)
	}

	var summoner Summoner
	if err := json.Unmarshal(body, &summoner); err != nil {
		return nil, fmt.Errorf("failed to parse summoner: %w", err)
	}

	return &summoner, nil
}

// GetLeagueEntriesByPUUID fetches ranked data for a player using PUUID
// Returns entries for all queues (Solo/Duo, Flex, etc.)
func (c *RankAPIClient) GetLeagueEntriesByPUUID(ctx context.Context, puuid string) ([]LeagueEntry, error) {
	apiURL := fmt.Sprintf(
		"https://%s.api.riotgames.com/lol/league/v4/entries/by-puuid/%s",
		c.client.Region(), puuid,
	)

	body, err := c.client.DoRequest(ctx, apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get league entries: %w", err)
	}

	var entries []LeagueEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Errorf("failed to parse league entries: %w", err)
	}

	return entries, nil
}

// GetSoloDuoRankByPUUID fetches only the Solo/Duo queue rank for a player using PUUID
func (c *RankAPIClient) GetSoloDuoRankByPUUID(ctx context.Context, puuid string) (*LeagueEntry, error) {
	entries, err := c.GetLeagueEntriesByPUUID(ctx, puuid)
	if err != nil {
		return nil, err
	}

	for i := range entries {
		if entries[i].QueueType == QueueTypeSoloDuo {
			return &entries[i], nil
		}
	}

	// No Solo/Duo rank found (unranked)
	return nil, nil
}

// IsNotFound checks if the error is a not found error
func IsNotFound(err error) bool {
	return riotwatch.IsNotFound(err)
}

// IsRateLimited checks if the error is a rate limit error
func IsRateLimited(err error) bool {
	return riotwatch.IsRateLimited(err)
}

// GetAccountByPUUID fetches account info by PUUID (used to refresh Riot ID)
func (c *RankAPIClient) GetAccountByPUUID(ctx context.Context, puuid string) (*riotwatch.Account, error) {
	return c.client.GetAccountByPUUID(ctx, puuid)
}

// GetLatestDataDragonVersion returns the latest Data Dragon version
// Falls back to a default version if the API call fails
func (c *RankAPIClient) GetLatestDataDragonVersion() string {
	version := c.client.GetLatestVersion()
	if version == "" {
		return "14.24.1" // Fallback default
	}
	return version
}
