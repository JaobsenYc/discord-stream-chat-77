package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConfig_GetMappingByTextChannel(t *testing.T) {
	cfg := &Config{
		ChannelMappings: []ChannelMapping{
			{
				TextChannelID:   "text1",
				VoiceCategoryID: "voice1",
				GameName:        "LOL",
			},
			{
				TextChannelID:   "text2",
				VoiceCategoryID: "voice2",
				GameName:        "Valorant",
			},
		},
	}

	tests := []struct {
		name      string
		channelID string
		expected  *ChannelMapping
	}{
		{
			name:      "找到匹配的映射",
			channelID: "text1",
			expected: &ChannelMapping{
				TextChannelID:   "text1",
				VoiceCategoryID: "voice1",
				GameName:        "LOL",
			},
		},
		{
			name:      "未找到匹配的映射",
			channelID: "text3",
			expected:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cfg.GetMappingByTextChannel(tt.channelID)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestConfig_IsMonitoredChannel(t *testing.T) {
	cfg := &Config{
		DefaultTextChannelID: "default_text",
		ChannelMappings: []ChannelMapping{
			{TextChannelID: "text1", VoiceCategoryID: "voice1", GameName: "LOL"},
			{TextChannelID: "text2", VoiceCategoryID: "voice2", GameName: "Valorant"},
		},
	}

	tests := []struct {
		name      string
		channelID string
		expected  bool
	}{
		{
			name:      "映射中的频道",
			channelID: "text1",
			expected:  true,
		},
		{
			name:      "默认文字频道",
			channelID: "default_text",
			expected:  true,
		},
		{
			name:      "未监听的频道",
			channelID: "text3",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cfg.IsMonitoredChannel(tt.channelID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConfig_GetVoiceCategoryForTextChannel(t *testing.T) {
	cfg := &Config{
		DefaultCategoryID: "default_category",
		ChannelMappings: []ChannelMapping{
			{TextChannelID: "text1", VoiceCategoryID: "voice1", GameName: "LOL"},
			{TextChannelID: "text2", VoiceCategoryID: "voice2", GameName: "Valorant"},
		},
	}

	tests := []struct {
		name      string
		channelID string
		expected  string
	}{
		{
			name:      "映射中的频道",
			channelID: "text1",
			expected:  "voice1",
		},
		{
			name:      "未映射的频道 - 返回默认分类",
			channelID: "text3",
			expected:  "default_category",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cfg.GetVoiceCategoryForTextChannel(tt.channelID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConfig_GetGameNameForTextChannel(t *testing.T) {
	cfg := &Config{
		DefaultTextChannelID: "default_text",
		ChannelMappings: []ChannelMapping{
			{TextChannelID: "text1", VoiceCategoryID: "voice1", GameName: "LOL"},
			{TextChannelID: "text2", VoiceCategoryID: "voice2", GameName: "Valorant"},
		},
	}

	tests := []struct {
		name      string
		channelID string
		expected  string
	}{
		{
			name:      "映射中的频道",
			channelID: "text1",
			expected:  "LOL",
		},
		{
			name:      "默认文字频道",
			channelID: "default_text",
			expected:  "通用寻组",
		},
		{
			name:      "未知频道",
			channelID: "text3",
			expected:  "未知游戏",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cfg.GetGameNameForTextChannel(tt.channelID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConfig_GetAllMonitoredTextChannels(t *testing.T) {
	cfg := &Config{
		DefaultTextChannelID: "default_text",
		ChannelMappings: []ChannelMapping{
			{TextChannelID: "text1", VoiceCategoryID: "voice1", GameName: "LOL"},
			{TextChannelID: "text2", VoiceCategoryID: "voice2", GameName: "Valorant"},
		},
	}

	result := cfg.GetAllMonitoredTextChannels()
	expected := []string{"text1", "text2", "default_text"}

	assert.ElementsMatch(t, expected, result)
}

func TestConfig_GetAllMonitoredTextChannels_NoDuplicates(t *testing.T) {
	cfg := &Config{
		DefaultTextChannelID: "text1", // 与映射中的频道重复
		ChannelMappings: []ChannelMapping{
			{TextChannelID: "text1", VoiceCategoryID: "voice1", GameName: "LOL"},
			{TextChannelID: "text2", VoiceCategoryID: "voice2", GameName: "Valorant"},
		},
	}

	result := cfg.GetAllMonitoredTextChannels()
	expected := []string{"text1", "text2"}

	assert.ElementsMatch(t, expected, result)
	assert.Len(t, result, 2) // 确保没有重复
}

func TestConfig_GetAllMonitoredVoiceChannels(t *testing.T) {
	cfg := &Config{
		DefaultCategoryID: "default_category",
		ChannelMappings: []ChannelMapping{
			{TextChannelID: "text1", VoiceCategoryID: "voice1", GameName: "LOL"},
			{TextChannelID: "text2", VoiceCategoryID: "voice2", GameName: "Valorant"},
		},
	}

	result := cfg.GetAllMonitoredVoiceChannels()
	expected := []string{"voice1", "voice2", "default_category"}

	assert.ElementsMatch(t, expected, result)
}

func TestConfig_GetAllMonitoredVoiceChannels_NoDuplicates(t *testing.T) {
	cfg := &Config{
		DefaultCategoryID: "voice1", // 与映射中的分类重复
		ChannelMappings: []ChannelMapping{
			{TextChannelID: "text1", VoiceCategoryID: "voice1", GameName: "LOL"},
			{TextChannelID: "text2", VoiceCategoryID: "voice2", GameName: "Valorant"},
		},
	}

	result := cfg.GetAllMonitoredVoiceChannels()
	expected := []string{"voice1", "voice2"}

	assert.ElementsMatch(t, expected, result)
	assert.Len(t, result, 2) // 确保没有重复
}

func TestConfig_parseChannelMappings(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		expected    []ChannelMapping
	}{
		{
			name:        "正常解析",
			input:       "LOL:text1:voice1,Valorant:text2:voice2",
			expectError: false,
			expected: []ChannelMapping{
				{TextChannelID: "text1", VoiceCategoryID: "voice1", GameName: "LOL"},
				{TextChannelID: "text2", VoiceCategoryID: "voice2", GameName: "Valorant"},
			},
		},
		{
			name:        "带空格的解析",
			input:       " LOL : text1 : voice1 , Valorant : text2 : voice2 ",
			expectError: false,
			expected: []ChannelMapping{
				{TextChannelID: "text1", VoiceCategoryID: "voice1", GameName: "LOL"},
				{TextChannelID: "text2", VoiceCategoryID: "voice2", GameName: "Valorant"},
			},
		},
		{
			name:        "无效格式 - 缺少部分",
			input:       "LOL:text1,Valorant:text2:voice2",
			expectError: true,
			expected:    nil,
		},
		{
			name:        "无效格式 - 空游戏名",
			input:       ":text1:voice1,Valorant:text2:voice2",
			expectError: true,
			expected:    nil,
		},
		{
			name:        "无效格式 - 空频道ID",
			input:       "LOL::voice1,Valorant:text2:voice2",
			expectError: true,
			expected:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{ChannelMappings: make([]ChannelMapping, 0)}

			err := cfg.parseChannelMappings(tt.input)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, cfg.ChannelMappings)
			}
		})
	}
}

func TestConfig_validate(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
	}{
		{
			name: "有效配置",
			config: &Config{
				MinTeamSize:           2,
				MaxTeamSize:           10,
				ChannelCleanupTimeout: 5 * time.Minute,
				ChannelMappings: []ChannelMapping{
					{TextChannelID: "text1", VoiceCategoryID: "voice1", GameName: "LOL"},
				},
			},
			expectError: false,
		},
		{
			name: "最小人数大于等于最大人数",
			config: &Config{
				MinTeamSize:           10,
				MaxTeamSize:           5,
				ChannelCleanupTimeout: 5 * time.Minute,
				ChannelMappings: []ChannelMapping{
					{TextChannelID: "text1", VoiceCategoryID: "voice1", GameName: "LOL"},
				},
			},
			expectError: true,
		},
		{
			name: "清理超时时间太短",
			config: &Config{
				MinTeamSize:           2,
				MaxTeamSize:           10,
				ChannelCleanupTimeout: 30 * time.Second,
				ChannelMappings: []ChannelMapping{
					{TextChannelID: "text1", VoiceCategoryID: "voice1", GameName: "LOL"},
				},
			},
			expectError: true,
		},
		{
			name: "没有频道映射",
			config: &Config{
				MinTeamSize:           2,
				MaxTeamSize:           10,
				ChannelCleanupTimeout: 5 * time.Minute,
				ChannelMappings:       []ChannelMapping{},
			},
			expectError: true,
		},
		{
			name: "重复的文字频道ID",
			config: &Config{
				MinTeamSize:           2,
				MaxTeamSize:           10,
				ChannelCleanupTimeout: 5 * time.Minute,
				ChannelMappings: []ChannelMapping{
					{TextChannelID: "text1", VoiceCategoryID: "voice1", GameName: "LOL"},
					{TextChannelID: "text1", VoiceCategoryID: "voice2", GameName: "Valorant"},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.validate()

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestLoad_RequiredEnvironmentVariables(t *testing.T) {
	// 保存原有环境变量
	originalVars := map[string]string{
		"DISCORD_TOKEN":           os.Getenv("DISCORD_TOKEN"),
		"GUILD_ID":                os.Getenv("GUILD_ID"),
		"CHANNEL_MAPPINGS":        os.Getenv("CHANNEL_MAPPINGS"),
		"DEFAULT_CATEGORY_ID":     os.Getenv("DEFAULT_CATEGORY_ID"),
		"DEFAULT_TEXT_CHANNEL_ID": os.Getenv("DEFAULT_TEXT_CHANNEL_ID"),
	}

	// 清理环境变量
	for key := range originalVars {
		os.Unsetenv(key)
	}

	// 延迟恢复原有环境变量
	defer func() {
		for key, value := range originalVars {
			if value != "" {
				os.Setenv(key, value)
			} else {
				os.Unsetenv(key)
			}
		}
	}()

	tests := []struct {
		name        string
		envVars     map[string]string
		expectError bool
	}{
		{
			name: "缺少 DISCORD_TOKEN",
			envVars: map[string]string{
				"GUILD_ID":                "test_guild",
				"CHANNEL_MAPPINGS":        "LOL:text1:voice1",
				"DEFAULT_CATEGORY_ID":     "default_category",
				"DEFAULT_TEXT_CHANNEL_ID": "default_text",
			},
			expectError: true,
		},
		{
			name: "缺少 GUILD_ID",
			envVars: map[string]string{
				"DISCORD_TOKEN":           "test_token",
				"CHANNEL_MAPPINGS":        "LOL:text1:voice1",
				"DEFAULT_CATEGORY_ID":     "default_category",
				"DEFAULT_TEXT_CHANNEL_ID": "default_text",
			},
			expectError: true,
		},
		{
			name: "缺少 CHANNEL_MAPPINGS",
			envVars: map[string]string{
				"DISCORD_TOKEN":           "test_token",
				"GUILD_ID":                "test_guild",
				"DEFAULT_CATEGORY_ID":     "default_category",
				"DEFAULT_TEXT_CHANNEL_ID": "default_text",
			},
			expectError: true,
		},
		{
			name: "所有必需变量都存在",
			envVars: map[string]string{
				"DISCORD_TOKEN":           "test_token",
				"GUILD_ID":                "test_guild",
				"CHANNEL_MAPPINGS":        "LOL:text1:voice1",
				"DEFAULT_CATEGORY_ID":     "default_category",
				"DEFAULT_TEXT_CHANNEL_ID": "default_text",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 设置测试环境变量
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}

			cfg, err := Load()

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, cfg)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, cfg)
			}

			// 清理测试环境变量
			for key := range tt.envVars {
				os.Unsetenv(key)
			}
		})
	}
}

func TestLoad_DevelopmentModeEnvironmentVariables(t *testing.T) {
	// 保存原有环境变量
	originalVars := map[string]string{
		"DISCORD_TOKEN":                   os.Getenv("DISCORD_TOKEN"),
		"GUILD_ID":                        os.Getenv("GUILD_ID"),
		"CHANNEL_MAPPINGS":                os.Getenv("CHANNEL_MAPPINGS"),
		"DEFAULT_CATEGORY_ID":             os.Getenv("DEFAULT_CATEGORY_ID"),
		"DEFAULT_TEXT_CHANNEL_ID":         os.Getenv("DEFAULT_TEXT_CHANNEL_ID"),
		"DEVELOPMENT_MODE":                os.Getenv("DEVELOPMENT_MODE"),
		"CLEANUP_TEXT_CHANNELS_ON_START":  os.Getenv("CLEANUP_TEXT_CHANNELS_ON_START"),
		"CLEANUP_VOICE_CHANNELS_ON_START": os.Getenv("CLEANUP_VOICE_CHANNELS_ON_START"),
	}

	// 清理环境变量
	for key := range originalVars {
		os.Unsetenv(key)
	}

	// 延迟恢复原有环境变量
	defer func() {
		for key, value := range originalVars {
			if value != "" {
				os.Setenv(key, value)
			} else {
				os.Unsetenv(key)
			}
		}
	}()

	// 设置基础必需环境变量
	baseEnvVars := map[string]string{
		"DISCORD_TOKEN":           "test_token",
		"GUILD_ID":                "test_guild",
		"CHANNEL_MAPPINGS":        "LOL:text1:voice1",
		"DEFAULT_CATEGORY_ID":     "default_category",
		"DEFAULT_TEXT_CHANNEL_ID": "default_text",
	}

	tests := []struct {
		name                         string
		additionalEnvVars            map[string]string
		expectedDevelopmentMode      bool
		expectedCleanupTextChannels  bool
		expectedCleanupVoiceChannels bool
	}{
		{
			name:                         "默认值 - 所有清理选项为false",
			additionalEnvVars:            map[string]string{},
			expectedDevelopmentMode:      false,
			expectedCleanupTextChannels:  false,
			expectedCleanupVoiceChannels: false,
		},
		{
			name: "开发模式启用",
			additionalEnvVars: map[string]string{
				"DEVELOPMENT_MODE": "true",
			},
			expectedDevelopmentMode:      true,
			expectedCleanupTextChannels:  false,
			expectedCleanupVoiceChannels: false,
		},
		{
			name: "启用文字频道清理",
			additionalEnvVars: map[string]string{
				"DEVELOPMENT_MODE":               "true",
				"CLEANUP_TEXT_CHANNELS_ON_START": "true",
			},
			expectedDevelopmentMode:      true,
			expectedCleanupTextChannels:  true,
			expectedCleanupVoiceChannels: false,
		},
		{
			name: "启用语音频道清理",
			additionalEnvVars: map[string]string{
				"DEVELOPMENT_MODE":                "true",
				"CLEANUP_VOICE_CHANNELS_ON_START": "true",
			},
			expectedDevelopmentMode:      true,
			expectedCleanupTextChannels:  false,
			expectedCleanupVoiceChannels: true,
		},
		{
			name: "启用所有清理选项",
			additionalEnvVars: map[string]string{
				"DEVELOPMENT_MODE":                "true",
				"CLEANUP_TEXT_CHANNELS_ON_START":  "true",
				"CLEANUP_VOICE_CHANNELS_ON_START": "true",
			},
			expectedDevelopmentMode:      true,
			expectedCleanupTextChannels:  true,
			expectedCleanupVoiceChannels: true,
		},
		{
			name: "非true值应该为false",
			additionalEnvVars: map[string]string{
				"DEVELOPMENT_MODE":                "false",
				"CLEANUP_TEXT_CHANNELS_ON_START":  "false",
				"CLEANUP_VOICE_CHANNELS_ON_START": "no",
			},
			expectedDevelopmentMode:      false,
			expectedCleanupTextChannels:  false,
			expectedCleanupVoiceChannels: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 设置基础环境变量
			for key, value := range baseEnvVars {
				os.Setenv(key, value)
			}

			// 设置测试特定的环境变量
			for key, value := range tt.additionalEnvVars {
				os.Setenv(key, value)
			}

			cfg, err := Load()

			assert.NoError(t, err)
			assert.NotNil(t, cfg)
			assert.Equal(t, tt.expectedDevelopmentMode, cfg.DevelopmentMode)
			assert.Equal(t, tt.expectedCleanupTextChannels, cfg.CleanupTextChannelsOnStart)
			assert.Equal(t, tt.expectedCleanupVoiceChannels, cfg.CleanupVoiceChannelsOnStart)

			// 清理测试环境变量
			for key := range baseEnvVars {
				os.Unsetenv(key)
			}
			for key := range tt.additionalEnvVars {
				os.Unsetenv(key)
			}
		})
	}
}

func TestLoad_ProTrackConfig(t *testing.T) {
	baseEnvVars := map[string]string{
		"DISCORD_TOKEN":           "test_token",
		"GUILD_ID":                "test_guild",
		"CHANNEL_MAPPINGS":        "LOL:text1:voice1",
		"DEFAULT_CATEGORY_ID":     "default_category",
		"DEFAULT_TEXT_CHANNEL_ID": "default_text",
	}

	tests := []struct {
		name             string
		envVars          map[string]string
		expectedMax      int
		expectedInterval time.Duration
	}{
		{
			name:             "默认值",
			envVars:          map[string]string{},
			expectedMax:      10,
			expectedInterval: 20 * time.Second,
		},
		{
			name: "自定义上限与间隔",
			envVars: map[string]string{
				"PRO_TRACK_MAX":      "30",
				"PRO_TRACK_INTERVAL": "30s",
			},
			expectedMax:      30,
			expectedInterval: 30 * time.Second,
		},
		{
			name: "非法值回退默认",
			envVars: map[string]string{
				"PRO_TRACK_MAX":      "-1",
				"PRO_TRACK_INTERVAL": "abc",
			},
			expectedMax:      10,
			expectedInterval: 20 * time.Second,
		},
		{
			name: "间隔过短回退默认",
			envVars: map[string]string{
				"PRO_TRACK_INTERVAL": "1s",
			},
			expectedMax:      10,
			expectedInterval: 20 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, key := range []string{"PRO_TRACK_CHANNEL_ID", "PRO_TRACK_MENTION_ROLE_ID", "PRO_TRACK_MAX", "PRO_TRACK_INTERVAL"} {
				t.Setenv(key, "")
			}
			for key, value := range baseEnvVars {
				t.Setenv(key, value)
			}
			for key, value := range tt.envVars {
				t.Setenv(key, value)
			}

			cfg, err := Load()

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedMax, cfg.ProTrackMax)
			assert.Equal(t, tt.expectedInterval, cfg.ProTrackInterval)
		})
	}

	t.Run("频道与身份组", func(t *testing.T) {
		for key, value := range baseEnvVars {
			t.Setenv(key, value)
		}
		t.Setenv("PRO_TRACK_CHANNEL_ID", "chan1")
		t.Setenv("PRO_TRACK_MENTION_ROLE_ID", "role1")

		cfg, err := Load()

		assert.NoError(t, err)
		assert.Equal(t, "chan1", cfg.ProTrackChannelID)
		assert.Equal(t, "role1", cfg.ProTrackMentionRoleID)
	})
}
