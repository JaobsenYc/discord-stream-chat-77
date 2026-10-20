package role

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/config"
	"github.com/jaobsenyc/discord-lfg-bot-go/pkg/logger"
)

// Manager Discord角色管理器
type Manager struct {
	session *discordgo.Session
	guildID string
	config  *config.Config
}

// NewManager 创建新的角色管理器
func NewManager(session *discordgo.Session, guildID string, cfg *config.Config) *Manager {
	return &Manager{
		session: session,
		guildID: guildID,
		config:  cfg,
	}
}

// IsAdmin 检查用户是否是管理员
func (m *Manager) IsAdmin(userID string) (bool, error) {
	logger.Debugf("检查管理员权限: 用户ID=%s, 配置的管理员ID列表=%v", userID, m.config.AdminUserIDs)

	// 首先检查用户ID白名单
	for _, adminID := range m.config.AdminUserIDs {
		if userID == adminID {
			logger.Debugf("用户 %s 在管理员白名单中", userID)
			return true, nil
		}
	}

	// 如果不在白名单中，检查Discord权限
	member, err := m.session.GuildMember(m.guildID, userID)
	if err != nil {
		return false, fmt.Errorf("获取用户信息失败: %w", err)
	}

	// 检查用户是否有管理员权限
	for _, roleID := range member.Roles {
		role, err := m.session.State.Role(m.guildID, roleID)
		if err != nil {
			continue
		}
		if role.Permissions&discordgo.PermissionAdministrator != 0 {
			return true, nil
		}
	}

	return false, nil
}

// IsAdminChannel 检查频道是否允许使用管理员命令
func (m *Manager) IsAdminChannel(channelID string) bool {
	// 如果没有配置管理员频道，则允许在所有频道使用
	if len(m.config.AdminChannelIDs) == 0 {
		return true
	}

	// 检查频道是否在允许列表中
	for _, adminChannelID := range m.config.AdminChannelIDs {
		if channelID == adminChannelID {
			return true
		}
	}

	return false
}

// CreateRole 创建新角色
func (m *Manager) CreateRole(name string, color string, permissions int64) (*discordgo.Role, error) {
	// 将十六进制颜色转换为整数
	colorInt64, err := strconv.ParseInt(strings.TrimPrefix(color, "#"), 16, 64)
	if err != nil {
		return nil, fmt.Errorf("颜色格式无效: %w", err)
	}

	// 转换为int类型
	colorInt := int(colorInt64)

	// 创建角色
	role, err := m.session.GuildRoleCreate(m.guildID, &discordgo.RoleParams{
		Name:        name,
		Color:       &colorInt,
		Permissions: &permissions,
	})
	if err != nil {
		return nil, fmt.Errorf("创建角色失败: %w", err)
	}

	logger.Infof("创建角色成功: %s (ID: %s)", name, role.ID)
	return role, nil
}

// DeleteRole 删除角色
func (m *Manager) DeleteRole(name string) error {
	// 获取所有角色
	roles, err := m.session.GuildRoles(m.guildID)
	if err != nil {
		return fmt.Errorf("获取角色列表失败: %w", err)
	}

	// 查找指定名称的角色
	var roleID string
	for _, role := range roles {
		if role.Name == name {
			roleID = role.ID
			break
		}
	}

	if roleID == "" {
		return fmt.Errorf("未找到角色: %s", name)
	}

	// 删除角色
	err = m.session.GuildRoleDelete(m.guildID, roleID)
	if err != nil {
		return fmt.Errorf("删除角色失败: %w", err)
	}

	logger.Infof("删除角色成功: %s (ID: %s)", name, roleID)
	return nil
}
