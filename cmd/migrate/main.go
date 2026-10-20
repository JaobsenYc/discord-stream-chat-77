package main

import (
	"database/sql"
	"fmt"

	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type Migration struct {
	Version int
	Name    string
	Path    string
}

func main() {
	// 加载环境变量(与主程序一致,读取根目录 .env)
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found")
	}

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "up":
		runMigrations()
	case "down":
		if len(os.Args) < 3 {
			log.Fatal("Please specify the number of migrations to rollback")
		}
		steps, err := strconv.Atoi(os.Args[2])
		if err != nil {
			log.Fatal("Invalid number of steps")
		}
		rollbackMigrations(steps)
	case "status":
		showMigrationStatus()
	case "create":
		if len(os.Args) < 3 {
			log.Fatal("Please specify migration name")
		}
		createMigration(os.Args[2])
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  migrate up                    - Run all pending migrations")
	fmt.Println("  migrate down <steps>          - Rollback <steps> migrations")
	fmt.Println("  migrate status                - Show migration status")
	fmt.Println("  migrate create <name>         - Create a new migration file")
}

func connectDB() *sql.DB {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	return db
}

func ensureMigrationsTable(db *sql.DB) {
	query := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			applied_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`

	if _, err := db.Exec(query); err != nil {
		log.Fatalf("Failed to create migrations table: %v", err)
	}
}

func getMigrationFiles() ([]Migration, error) {
	var migrations []Migration

	// 检查 sql/init 目录
	initFiles, err := filepath.Glob("sql/init/*.sql")
	if err != nil {
		return nil, err
	}

	// sql/init 是完整基线（001-006，docker 部署时由 Postgres 容器自动执行），
	// 这里也全部纳入，供本地开发在裸数据库上用 migrate up 建库
	for _, file := range initFiles {
		name := filepath.Base(file)
		parts := strings.SplitN(name, "_", 2)
		if len(parts) < 2 {
			continue
		}
		version, err := strconv.Atoi(parts[0])
		if err != nil {
			// 跳过非编号文件（如 zzz_migration_baseline.sql，由容器 init 专用）
			continue
		}
		migrations = append(migrations, Migration{
			Version: version,
			Name:    strings.TrimSuffix(name, ".sql"),
			Path:    file,
		})
	}

	// 检查 sql/migrations 目录
	migrationFiles, err := filepath.Glob("sql/migrations/*.sql")
	if err != nil {
		return nil, err
	}

	for _, file := range migrationFiles {
		name := filepath.Base(file)
		parts := strings.SplitN(name, "_", 2)
		if len(parts) < 2 {
			continue
		}

		version, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}

		migrations = append(migrations, Migration{
			Version: version,
			Name:    strings.TrimSuffix(name, ".sql"),
			Path:    file,
		})
	}

	// 按版本排序
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

func getAppliedMigrations(db *sql.DB) (map[int]bool, error) {
	applied := make(map[int]bool)

	rows, err := db.Query("SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		applied[version] = true
	}

	return applied, nil
}

func runMigrations() {
	db := connectDB()
	defer db.Close()

	ensureMigrationsTable(db)

	migrations, err := getMigrationFiles()
	if err != nil {
		log.Fatalf("Failed to get migration files: %v", err)
	}

	applied, err := getAppliedMigrations(db)
	if err != nil {
		log.Fatalf("Failed to get applied migrations: %v", err)
	}

	var executed int
	for _, migration := range migrations {
		if applied[migration.Version] {
			continue
		}

		fmt.Printf("Running migration %d: %s\n", migration.Version, migration.Name)

		content, err := os.ReadFile(migration.Path)
		if err != nil {
			log.Fatalf("Failed to read migration file %s: %v", migration.Path, err)
		}

		// 开始事务
		tx, err := db.Begin()
		if err != nil {
			log.Fatalf("Failed to begin transaction: %v", err)
		}

		// 执行迁移
		if _, err := tx.Exec(string(content)); err != nil {
			tx.Rollback()
			log.Fatalf("Failed to execute migration %d: %v", migration.Version, err)
		}

		// 记录迁移
		if _, err := tx.Exec(
			"INSERT INTO schema_migrations (version, name) VALUES ($1, $2)",
			migration.Version, migration.Name,
		); err != nil {
			tx.Rollback()
			log.Fatalf("Failed to record migration %d: %v", migration.Version, err)
		}

		// 提交事务
		if err := tx.Commit(); err != nil {
			log.Fatalf("Failed to commit migration %d: %v", migration.Version, err)
		}

		executed++
	}

	if executed == 0 {
		fmt.Println("No pending migrations")
	} else {
		fmt.Printf("Successfully executed %d migrations\n", executed)
	}
}

func rollbackMigrations(steps int) {
	db := connectDB()
	defer db.Close()

	ensureMigrationsTable(db)

	// 获取最近应用的迁移
	rows, err := db.Query("SELECT version, name FROM schema_migrations ORDER BY version DESC LIMIT $1", steps)
	if err != nil {
		log.Fatalf("Failed to get migrations to rollback: %v", err)
	}
	defer rows.Close()

	var toRollback []Migration
	for rows.Next() {
		var version int
		var name string
		if err := rows.Scan(&version, &name); err != nil {
			log.Fatalf("Failed to scan migration: %v", err)
		}
		toRollback = append(toRollback, Migration{Version: version, Name: name})
	}

	if len(toRollback) == 0 {
		fmt.Println("No migrations to rollback")
		return
	}

	for _, migration := range toRollback {
		fmt.Printf("Rolling back migration %d: %s\n", migration.Version, migration.Name)

		// 查找对应的 down 文件
		downFile := fmt.Sprintf("sql/migrations/%03d_%s_down.sql", migration.Version,
			strings.TrimPrefix(migration.Name, fmt.Sprintf("%03d_", migration.Version)))

		if _, err := os.Stat(downFile); os.IsNotExist(err) {
			log.Printf("Warning: No down migration found for %s, skipping", migration.Name)
			continue
		}

		content, err := os.ReadFile(downFile)
		if err != nil {
			log.Fatalf("Failed to read down migration file %s: %v", downFile, err)
		}

		// 开始事务
		tx, err := db.Begin()
		if err != nil {
			log.Fatalf("Failed to begin transaction: %v", err)
		}

		// 执行回滚
		if _, err := tx.Exec(string(content)); err != nil {
			tx.Rollback()
			log.Fatalf("Failed to execute rollback %d: %v", migration.Version, err)
		}

		// 删除迁移记录
		if _, err := tx.Exec("DELETE FROM schema_migrations WHERE version = $1", migration.Version); err != nil {
			tx.Rollback()
			log.Fatalf("Failed to remove migration record %d: %v", migration.Version, err)
		}

		// 提交事务
		if err := tx.Commit(); err != nil {
			log.Fatalf("Failed to commit rollback %d: %v", migration.Version, err)
		}
	}

	fmt.Printf("Successfully rolled back %d migrations\n", len(toRollback))
}

func showMigrationStatus() {
	db := connectDB()
	defer db.Close()

	ensureMigrationsTable(db)

	migrations, err := getMigrationFiles()
	if err != nil {
		log.Fatalf("Failed to get migration files: %v", err)
	}

	applied, err := getAppliedMigrations(db)
	if err != nil {
		log.Fatalf("Failed to get applied migrations: %v", err)
	}

	fmt.Println("Migration Status:")
	fmt.Println("================")

	for _, migration := range migrations {
		status := "Pending"
		if applied[migration.Version] {
			status = "Applied"
		}
		fmt.Printf("Version %d: %s [%s]\n", migration.Version, migration.Name, status)
	}
}

func createMigration(name string) {
	// 获取下一个版本号
	migrations, err := getMigrationFiles()
	if err != nil {
		log.Fatalf("Failed to get migration files: %v", err)
	}

	nextVersion := 1
	if len(migrations) > 0 {
		nextVersion = migrations[len(migrations)-1].Version + 1
	}

	// 创建迁移文件名
	timestamp := time.Now().Format("20060102150405")
	filename := fmt.Sprintf("%03d_%s_%s.sql", nextVersion, timestamp, strings.ReplaceAll(name, " ", "_"))
	filepath := fmt.Sprintf("sql/migrations/%s", filename)

	// 创建目录
	if err := os.MkdirAll("sql/migrations", 0755); err != nil {
		log.Fatalf("Failed to create migrations directory: %v", err)
	}

	// 创建迁移文件
	content := fmt.Sprintf(`-- =====================================================
-- Migration: %s
-- Version: %d
-- Created: %s
-- =====================================================

-- Add your migration SQL here


`, name, nextVersion, time.Now().Format("2006-01-02 15:04:05"))

	if err := os.WriteFile(filepath, []byte(content), 0644); err != nil {
		log.Fatalf("Failed to create migration file: %v", err)
	}

	// 创建对应的 down 文件
	downFilename := fmt.Sprintf("%03d_%s_%s_down.sql", nextVersion, timestamp, strings.ReplaceAll(name, " ", "_"))
	downFilepath := fmt.Sprintf("sql/migrations/%s", downFilename)

	downContent := fmt.Sprintf(`-- =====================================================
-- Rollback Migration: %s
-- Version: %d
-- Created: %s
-- =====================================================

-- Add your rollback SQL here


`, name, nextVersion, time.Now().Format("2006-01-02 15:04:05"))

	if err := os.WriteFile(downFilepath, []byte(downContent), 0644); err != nil {
		log.Fatalf("Failed to create down migration file: %v", err)
	}

	fmt.Printf("Created migration files:\n")
	fmt.Printf("  %s\n", filepath)
	fmt.Printf("  %s\n", downFilepath)
}
