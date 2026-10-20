package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

func main() {
	apiKey := os.Getenv("RIOT_API_KEY")
	region := "na1" // 可以根据需要修改

	if len(os.Args) > 1 {
		apiKey = os.Args[1]
	}
	if len(os.Args) > 2 {
		region = os.Args[2]
	}

	if apiKey == "" {
		fmt.Println("用法: test_api_key [API_KEY] [REGION]，或设置 RIOT_API_KEY 环境变量")
		os.Exit(1)
	}

	fmt.Printf("测试 API Key: 已提供 (长度 %d)\n", len(apiKey))
	fmt.Printf("区域: %s\n\n", region)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 测试 1: Account API (通常都有权限)
	fmt.Println("1. 测试 Account API...")
	testAccountAPI(ctx, apiKey, "americas")
	fmt.Println()

	// 测试 2: Summoner API
	fmt.Println("2. 测试 Summoner API...")
	// 需要一个有效的 PUUID 来测试，这里先测试一个示例
	// 实际使用时需要先通过 Account API 获取 PUUID
	fmt.Println("   (需要有效的 PUUID，跳过)")
	fmt.Println()

	// 测试 3: League API - 这是关键测试
	fmt.Println("3. 测试 League API (关键测试)...")
	// 需要一个有效的 Summoner ID 来测试
	// 这里我们尝试一个已知的格式来测试权限
	testLeagueAPI(ctx, apiKey, region)
	fmt.Println()

	// 测试 4: 检查 API Key 基本信息
	fmt.Println("4. 检查 API Key 状态...")
	checkAPIKeyStatus(ctx, apiKey)
}

func testAccountAPI(ctx context.Context, apiKey, platform string) {
	// 使用一个真实的 Riot ID 进行测试（如果知道的话）
	// 或者使用一个不存在的 ID 来测试权限
	url := "https://americas.api.riotgames.com/riot/account/v1/accounts/by-riot-id/Riot/Test"

	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("X-Riot-Token", apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("   ❌ 请求失败: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	switch resp.StatusCode {
	case http.StatusOK:
		fmt.Printf("   ✅ Account API 可用\n")
	case http.StatusNotFound:
		fmt.Printf("   ✅ Account API 可用 (404 是正常的，账号不存在但权限正常)\n")
	case http.StatusUnauthorized:
		fmt.Printf("   ❌ API Key 无效或已过期\n")
	case http.StatusForbidden:
		fmt.Printf("   ❌ Account API 权限不足\n")
		fmt.Printf("   响应: %s\n", string(body))
	default:
		fmt.Printf("   ⚠️  状态码: %d\n", resp.StatusCode)
		if len(body) > 0 {
			fmt.Printf("   响应: %s\n", string(body))
		}
	}
}

func testLeagueAPI(ctx context.Context, apiKey, region string) {
	// 尝试访问 League API
	// 使用一个无效的 Summoner ID 来测试权限（而不是数据存在性）
	// 如果返回 403 Forbidden，说明没有权限
	// 如果返回 404 或 200（空数组），说明有权限但数据不存在
	testSummonerID := "invalid_summoner_id_for_permission_test"

	url := fmt.Sprintf("https://%s.api.riotgames.com/lol/league/v4/entries/by-summoner/%s", region, testSummonerID)

	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("X-Riot-Token", apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("   ❌ 请求失败: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	switch resp.StatusCode {
	case http.StatusOK:
		fmt.Printf("   ✅ League API 可用 (返回空数组是正常的)\n")
		var entries []interface{}
		if err := json.Unmarshal(body, &entries); err == nil {
			fmt.Printf("   📊 返回了 %d 条记录\n", len(entries))
		}
	case http.StatusNotFound:
		fmt.Printf("   ✅ League API 可用 (404 表示 Summoner ID 不存在，但权限正常)\n")
	case http.StatusUnauthorized:
		fmt.Printf("   ❌ API Key 无效或已过期\n")
		fmt.Printf("   响应: %s\n", string(body))
	case http.StatusForbidden:
		fmt.Printf("   ❌ League API 权限不足！\n")
		fmt.Printf("   ⚠️  这就是问题所在 - API Key 没有访问 League API 的权限\n")
		fmt.Printf("   💡 解决方案:\n")
		fmt.Printf("      1. 访问 https://developer.riotgames.com/ 检查 API Key 权限\n")
		fmt.Printf("      2. 确保 API Key 有 lol/league/v4 权限\n")
		fmt.Printf("      3. 如果是个人开发密钥，考虑申请生产环境 API Key\n")
		if len(body) > 0 {
			fmt.Printf("   响应: %s\n", string(body))
		}
	default:
		fmt.Printf("   ⚠️  状态码: %d\n", resp.StatusCode)
		if len(body) > 0 {
			fmt.Printf("   响应: %s\n", string(body))
		}
	}
}

func checkAPIKeyStatus(ctx context.Context, apiKey string) {
	// 尝试访问一个不需要特定资源的端点来检查 API Key 状态
	url := "https://na1.api.riotgames.com/lol/status/v4/platform-data"

	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("X-Riot-Token", apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("   ❌ 请求失败: %v\n", err)
		return
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		fmt.Printf("   ✅ API Key 有效\n")
	case http.StatusUnauthorized:
		fmt.Printf("   ❌ API Key 无效或已过期\n")
	case http.StatusForbidden:
		fmt.Printf("   ❌ API Key 权限不足\n")
	default:
		fmt.Printf("   ⚠️  状态码: %d\n", resp.StatusCode)
	}
}
