package bot

import (
	"github.com/bwmarrin/discordgo"
	"github.com/jaobsenyc/discord-lfg-bot-go/pkg/logger"
)

// commandRegistrar 一个模块的 slash 命令注册单元
type commandRegistrar struct {
	name     string
	register func(s *discordgo.Session) error
}

// commandRegistrars 返回全部模块的命令注册表,handleReady 统一循环执行。
// 新增模块的 slash 命令在这里登记一行即可。
func (b *Bot) commandRegistrars() []commandRegistrar {
	// registerAll 逐条注册命令定义;单条失败记录日志并继续,返回首个错误
	registerAll := func(cmds func() []*discordgo.ApplicationCommand) func(*discordgo.Session) error {
		return func(s *discordgo.Session) error {
			var firstErr error
			for _, cmd := range cmds() {
				if _, err := s.ApplicationCommandCreate(s.State.User.ID, b.config.GuildID, cmd); err != nil {
					logger.Errorf("注册 Slash Command %s 失败: %v", cmd.Name, err)
					if firstErr == nil {
						firstErr = err
					}
				}
			}
			return firstErr
		}
	}

	regs := []commandRegistrar{
		{"签到命令", registerAll(b.registerCheckInCommands)},
		{"成就系统命令", b.achievementCommands.RegisterCommands},
		{"抽奖系统命令", b.lotteryCommands.RegisterCommands},
		{"频道管理命令", registerAll(b.channelCommands.GetCommands)},
		{"LOL内战命令", func(*discordgo.Session) error { return b.lolCommands.RegisterCommands() }},
		{"竞猜系统命令", func(*discordgo.Session) error { return b.bettingCommands.RegisterCommands() }},
		{"积分系统命令", func(*discordgo.Session) error { return b.pointsCommands.RegisterCommands() }},
	}

	if b.riotWatchCommands != nil {
		regs = append(regs, commandRegistrar{"Riot Watch 命令",
			func(*discordgo.Session) error { return b.riotWatchCommands.RegisterCommands() }})
	}
	if b.rankCommands != nil {
		regs = append(regs, commandRegistrar{"Rank 绑定系统命令",
			func(*discordgo.Session) error { return b.rankCommands.RegisterCommands() }})
	}
	if b.proTrackCommands != nil {
		regs = append(regs, commandRegistrar{"职业选手追踪命令",
			func(*discordgo.Session) error { return b.proTrackCommands.RegisterCommands() }})
	}

	return regs
}
