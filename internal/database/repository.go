package database

// Repository 数据访问层管理器
type Repository struct {
	db      *DB
	User    *UserRepository
	CheckIn *CheckInRepository
	Team    *TeamRepository
	Voice   *VoiceRepository
	Lottery *LotteryRepository
}

// NewRepository 创建新的 Repository 管理器
func NewRepository(db *DB) *Repository {
	return &Repository{
		db:      db,
		User:    NewUserRepository(db),
		CheckIn: NewCheckInRepository(db),
		Team:    NewTeamRepository(db),
		Voice:   NewVoiceRepository(db),
		Lottery: NewLotteryRepository(db),
	}
}

// DB 获取数据库连接
func (r *Repository) DB() *DB {
	return r.db
}

// Close 关闭数据库连接
func (r *Repository) Close() error {
	return r.db.Close()
}

// Ping 测试数据库连接
func (r *Repository) Ping() error {
	return r.db.Ping()
}
