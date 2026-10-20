package riotwatch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/jaobsenyc/discord-lfg-bot-go/pkg/logger"
	"golang.org/x/time/rate"
)

// RiotAPIClient Riot API 客户端，带限流功能
type RiotAPIClient struct {
	httpClient *http.Client
	apiKey     string
	region     string // 游戏区域 (na1, euw1, kr, etc.)
	platform   string // 平台路由 (americas, europe, asia)
	limiter    *rate.Limiter

	// Data Dragon 英雄数据缓存
	championCache     map[int64]string // championID -> championName
	championCacheMu   sync.RWMutex
	championCacheTime time.Time
}

// NewRiotAPIClient 创建新的 Riot API 客户端
func NewRiotAPIClient(apiKey, region, platform string) *RiotAPIClient {
	return &RiotAPIClient{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		apiKey:     apiKey,
		region:     region,
		platform:   platform,
		// 限流：每秒 15 个请求，突发 20 个
		limiter:       rate.NewLimiter(rate.Every(67*time.Millisecond), 20),
		championCache: make(map[int64]string),
	}
}

// doRequest 执行 HTTP 请求
func (c *RiotAPIClient) doRequest(ctx context.Context, url string) ([]byte, error) {
	// 等待限流
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter error: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Riot-Token", c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// 处理错误状态码
	switch resp.StatusCode {
	case http.StatusOK:
		return body, nil
	case http.StatusNotFound:
		return nil, ErrNotFound
	case http.StatusTooManyRequests:
		logger.Warnf("Riot API rate limit exceeded")
		return nil, ErrRateLimited
	case http.StatusUnauthorized:
		return nil, ErrUnauthorized
	case http.StatusForbidden:
		return nil, ErrForbidden
	default:
		return nil, fmt.Errorf("API error: status %d, body: %s", resp.StatusCode, string(body))
	}
}

// GetAccountByRiotID 通过 Riot ID 获取账号信息
func (c *RiotAPIClient) GetAccountByRiotID(ctx context.Context, gameName, tagLine string) (*Account, error) {
	// URL 编码处理特殊字符
	encodedName := url.PathEscape(gameName)
	encodedTag := url.PathEscape(tagLine)

	apiURL := fmt.Sprintf(
		"https://%s.api.riotgames.com/riot/account/v1/accounts/by-riot-id/%s/%s",
		c.platform, encodedName, encodedTag,
	)

	body, err := c.doRequest(ctx, apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	var account Account
	if err := json.Unmarshal(body, &account); err != nil {
		return nil, fmt.Errorf("failed to parse account: %w", err)
	}

	return &account, nil
}

// GetAccountByPUUID 通过 PUUID 获取账号信息（可用于刷新显示名）
func (c *RiotAPIClient) GetAccountByPUUID(ctx context.Context, puuid string) (*Account, error) {
	apiURL := fmt.Sprintf(
		"https://%s.api.riotgames.com/riot/account/v1/accounts/by-puuid/%s",
		c.platform, puuid,
	)

	body, err := c.doRequest(ctx, apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get account by puuid: %w", err)
	}

	var account Account
	if err := json.Unmarshal(body, &account); err != nil {
		return nil, fmt.Errorf("failed to parse account: %w", err)
	}

	return &account, nil
}

// GetActiveGame 检查玩家是否在游戏中
// 如果玩家不在游戏中，返回 ErrNotFound
func (c *RiotAPIClient) GetActiveGame(ctx context.Context, puuid string) (*ActiveGame, error) {
	apiURL := fmt.Sprintf(
		"https://%s.api.riotgames.com/lol/spectator/v5/active-games/by-summoner/%s",
		c.region, puuid,
	)

	body, err := c.doRequest(ctx, apiURL)
	if err != nil {
		return nil, err // ErrNotFound 表示不在游戏中
	}

	var game ActiveGame
	if err := json.Unmarshal(body, &game); err != nil {
		return nil, fmt.Errorf("failed to parse active game: %w", err)
	}

	return &game, nil
}

// GetMatch 获取已结束对局的详细信息
func (c *RiotAPIClient) GetMatch(ctx context.Context, matchID string) (*Match, error) {
	apiURL := fmt.Sprintf(
		"https://%s.api.riotgames.com/lol/match/v5/matches/%s",
		c.platform, matchID,
	)

	body, err := c.doRequest(ctx, apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get match: %w", err)
	}

	var match Match
	if err := json.Unmarshal(body, &match); err != nil {
		return nil, fmt.Errorf("failed to parse match: %w", err)
	}

	return &match, nil
}

// GetRecentMatches 获取玩家最近的对局列表
func (c *RiotAPIClient) GetRecentMatches(ctx context.Context, puuid string, count int) ([]string, error) {
	if count <= 0 {
		count = 5
	}
	if count > 20 {
		count = 20
	}

	apiURL := fmt.Sprintf(
		"https://%s.api.riotgames.com/lol/match/v5/matches/by-puuid/%s/ids?count=%d",
		c.platform, puuid, count,
	)

	body, err := c.doRequest(ctx, apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent matches: %w", err)
	}

	var matchIDs []string
	if err := json.Unmarshal(body, &matchIDs); err != nil {
		return nil, fmt.Errorf("failed to parse match ids: %w", err)
	}

	return matchIDs, nil
}

// ============================================================================
// Errors
// ============================================================================

// ErrNotFound 表示资源不存在（如玩家不在游戏中）
var ErrNotFound = fmt.Errorf("not found")

// ErrRateLimited 表示 API 限流
var ErrRateLimited = fmt.Errorf("rate limited")

// ErrUnauthorized 表示 API Key 无效
var ErrUnauthorized = fmt.Errorf("unauthorized: invalid API key")

// ErrForbidden 表示没有权限
var ErrForbidden = fmt.Errorf("forbidden: insufficient permissions")

// IsNotFound 检查是否是 NotFound 错误
func IsNotFound(err error) bool {
	return err == ErrNotFound
}

// IsRateLimited 检查是否是限流错误
func IsRateLimited(err error) bool {
	return err == ErrRateLimited
}

// ============================================================================
// Data Dragon API - 英雄数据
// ============================================================================

// DataDragonChampion Data Dragon 英雄数据结构
type DataDragonChampion struct {
	Key  string `json:"key"`  // Champion ID as string
	Name string `json:"name"` // 英雄名称
}

// DataDragonResponse Data Dragon API 响应
type DataDragonResponse struct {
	Data map[string]DataDragonChampion `json:"data"`
}

// GetChampionName 通过英雄ID获取英雄名称
// 使用 Data Dragon API，带24小时缓存
func (c *RiotAPIClient) GetChampionName(championID int64) string {
	// 先检查缓存
	c.championCacheMu.RLock()
	if name, ok := c.championCache[championID]; ok {
		c.championCacheMu.RUnlock()
		return name
	}
	c.championCacheMu.RUnlock()

	// 检查是否需要刷新缓存（24小时过期）
	c.championCacheMu.Lock()
	defer c.championCacheMu.Unlock()

	// 双重检查
	if name, ok := c.championCache[championID]; ok {
		return name
	}

	// 如果缓存为空或过期，重新加载
	if len(c.championCache) == 0 || time.Since(c.championCacheTime) > 24*time.Hour {
		c.loadChampionData()
	}

	// 再次查找
	if name, ok := c.championCache[championID]; ok {
		return name
	}

	// 找不到则返回 ID
	return fmt.Sprintf("Champion %d", championID)
}

// loadChampionData 从 Data Dragon 加载英雄数据
func (c *RiotAPIClient) loadChampionData() {
	// 获取最新版本
	version := c.getLatestVersion()
	if version == "" {
		version = "14.24.1" // 默认版本
	}

	// 使用中文数据
	url := fmt.Sprintf("https://ddragon.leagueoflegends.com/cdn/%s/data/zh_CN/champion.json", version)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		logger.Errorf("创建 Data Dragon 请求失败: %v", err)
		return
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		logger.Errorf("请求 Data Dragon 失败: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Errorf("Data Dragon 返回错误状态码: %d", resp.StatusCode)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Errorf("读取 Data Dragon 响应失败: %v", err)
		return
	}

	var ddResp DataDragonResponse
	if err := json.Unmarshal(body, &ddResp); err != nil {
		logger.Errorf("解析 Data Dragon 响应失败: %v", err)
		return
	}

	// 构建 ID -> Name 映射
	for _, champ := range ddResp.Data {
		var id int64
		if _, err := fmt.Sscanf(champ.Key, "%d", &id); err == nil {
			c.championCache[id] = champ.Name
		}
	}

	c.championCacheTime = time.Now()
	logger.Infof("已加载 %d 个英雄数据 (版本: %s)", len(c.championCache), version)
}

// ============================================================================
// Public Accessors (for use by other packages like rank)
// ============================================================================

// Region returns the game region (na1, euw1, kr, etc.)
func (c *RiotAPIClient) Region() string {
	return c.region
}

// Platform returns the platform routing value (americas, europe, asia)
func (c *RiotAPIClient) Platform() string {
	return c.platform
}

// DoRequest executes an HTTP request with rate limiting (public wrapper)
func (c *RiotAPIClient) DoRequest(ctx context.Context, url string) ([]byte, error) {
	return c.doRequest(ctx, url)
}

// GetLatestVersion returns the latest Data Dragon version (public wrapper)
func (c *RiotAPIClient) GetLatestVersion() string {
	return c.getLatestVersion()
}

// getLatestVersion 获取 Data Dragon 最新版本
func (c *RiotAPIClient) getLatestVersion() string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://ddragon.leagueoflegends.com/api/versions.json", nil)
	if err != nil {
		return ""
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}

	var versions []string
	if err := json.Unmarshal(body, &versions); err != nil || len(versions) == 0 {
		return ""
	}

	return versions[0] // 第一个是最新版本
}
