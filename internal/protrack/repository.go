package protrack

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jaobsenyc/discord-lfg-bot-go/internal/database"
)

// Repository pro_accounts 表的数据访问
type Repository struct {
	db *database.DB
}

// NewRepository 创建仓储
func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

// GetByRiotID 按 Riot ID(不区分大小写)查询账号,不存在时返回 nil, nil
func (r *Repository) GetByRiotID(ctx context.Context, gameName, tagLine string) (*ProAccount, error) {
	var a ProAccount
	err := r.db.GetContext(ctx, &a, `
		SELECT * FROM pro_accounts
		WHERE LOWER(riot_game_name) = LOWER($1) AND LOWER(riot_tag_line) = LOWER($2)`,
		gameName, tagLine)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("按 Riot ID 查询职业选手账号失败: %w", err)
	}
	return &a, nil
}

// ListByProName 按选手名(不区分大小写)查询其全部账号
func (r *Repository) ListByProName(ctx context.Context, proName string) ([]ProAccount, error) {
	var accounts []ProAccount
	err := r.db.SelectContext(ctx, &accounts, `
		SELECT * FROM pro_accounts WHERE LOWER(pro_name) = LOWER($1) ORDER BY created_at`,
		proName)
	if err != nil {
		return nil, fmt.Errorf("按选手名查询账号失败: %w", err)
	}
	return accounts, nil
}

// Create 新增选手账号,回填 ID 与时间戳
func (r *Repository) Create(ctx context.Context, a *ProAccount) error {
	err := r.db.QueryRowxContext(ctx, `
		INSERT INTO pro_accounts (pro_name, team, riot_game_name, riot_tag_line, riot_puuid)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at`,
		a.ProName, a.Team, a.RiotGameName, a.RiotTagLine, a.RiotPUUID,
	).Scan(&a.ID, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return fmt.Errorf("新增职业选手账号失败: %w", err)
	}
	return nil
}

// UpdateProInfo 更新选手名与战队
func (r *Repository) UpdateProInfo(ctx context.Context, id uuid.UUID, proName string, team *string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE pro_accounts SET pro_name = $2, team = $3, updated_at = NOW() WHERE id = $1`,
		id, proName, team)
	if err != nil {
		return fmt.Errorf("更新选手信息失败: %w", err)
	}
	return nil
}

// SetRiotAccount 写入 PUUID 及 Riot 返回的规范大小写名字
func (r *Repository) SetRiotAccount(ctx context.Context, id uuid.UUID, puuid, gameName, tagLine string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE pro_accounts
		SET riot_puuid = $2, riot_game_name = $3, riot_tag_line = $4, updated_at = NOW()
		WHERE id = $1`,
		id, puuid, gameName, tagLine)
	if err != nil {
		return fmt.Errorf("保存 PUUID 失败: %w", err)
	}
	return nil
}

// SetTracked 开始追踪,播报发往 channelID
func (r *Repository) SetTracked(ctx context.Context, id uuid.UUID, channelID, discordID string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE pro_accounts
		SET is_tracked = TRUE, track_channel_id = $2, tracked_by_discord_id = $3, updated_at = NOW()
		WHERE id = $1`,
		id, channelID, discordID)
	if err != nil {
		return fmt.Errorf("开启追踪失败: %w", err)
	}
	return nil
}

// Untrack 取消追踪并清空对局状态
func (r *Repository) Untrack(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE pro_accounts
		SET is_tracked = FALSE, current_game_id = NULL, current_game_started_at = NULL,
		    current_champion_id = NULL, updated_at = NOW()
		WHERE id = $1`,
		id)
	if err != nil {
		return fmt.Errorf("取消追踪失败: %w", err)
	}
	return nil
}

// ListTracked 查询全部追踪中的账号
func (r *Repository) ListTracked(ctx context.Context) ([]ProAccount, error) {
	var accounts []ProAccount
	err := r.db.SelectContext(ctx, &accounts, `
		SELECT * FROM pro_accounts WHERE is_tracked ORDER BY LOWER(pro_name), created_at`)
	if err != nil {
		return nil, fmt.Errorf("查询追踪列表失败: %w", err)
	}
	return accounts, nil
}

// ListAll 查询选手库全部账号
func (r *Repository) ListAll(ctx context.Context) ([]ProAccount, error) {
	var accounts []ProAccount
	err := r.db.SelectContext(ctx, &accounts, `
		SELECT * FROM pro_accounts ORDER BY LOWER(pro_name), created_at`)
	if err != nil {
		return nil, fmt.Errorf("查询选手库失败: %w", err)
	}
	return accounts, nil
}

// CountTracked 统计追踪中的账号数
func (r *Repository) CountTracked(ctx context.Context) (int, error) {
	var n int
	if err := r.db.GetContext(ctx, &n, `SELECT COUNT(*) FROM pro_accounts WHERE is_tracked`); err != nil {
		return 0, fmt.Errorf("统计追踪数量失败: %w", err)
	}
	return n, nil
}

// SetInGame 记录进行中的单双排对局
func (r *Repository) SetInGame(ctx context.Context, id uuid.UUID, gameID int64, startedAt time.Time, championID int64) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE pro_accounts
		SET current_game_id = $2, current_game_started_at = $3, current_champion_id = $4,
		    last_checked_at = NOW(), updated_at = NOW()
		WHERE id = $1`,
		id, gameID, startedAt, championID)
	if err != nil {
		return fmt.Errorf("记录对局状态失败: %w", err)
	}
	return nil
}

// ClearGame 对局结束:当前对局转存为 last_game_id 后清空
func (r *Repository) ClearGame(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE pro_accounts
		SET last_game_id = current_game_id, current_game_id = NULL, current_game_started_at = NULL,
		    current_champion_id = NULL, last_checked_at = NOW(), updated_at = NOW()
		WHERE id = $1`,
		id)
	if err != nil {
		return fmt.Errorf("清除对局状态失败: %w", err)
	}
	return nil
}

// TouchChecked 更新最后检查时间
func (r *Repository) TouchChecked(ctx context.Context, id uuid.UUID) error {
	if _, err := r.db.ExecContext(ctx, `UPDATE pro_accounts SET last_checked_at = NOW() WHERE id = $1`, id); err != nil {
		return fmt.Errorf("更新检查时间失败: %w", err)
	}
	return nil
}
