# Docker Compose 设置指南

本指南将帮助你使用 Docker Compose 快速设置和运行 Discord LFG Bot，包括 PostgreSQL 数据库和 Redis 缓存。

## 📋 前置要求

- Docker 和 Docker Compose 已安装
- Discord Bot Token 和服务器权限
- 基本的 Docker 知识

## 🚀 快速开始

### 1. 环境变量配置

首先复制环境变量模板：

```bash
cp config/consise.env.example .env
```

编辑 `.env` 文件，配置以下关键变量：

```bash
# Discord 配置
DISCORD_TOKEN=你的_discord_bot_token
GUILD_ID=你的_discord_服务器_id

# 数据库密码 (重要: 请使用强密码,生成: openssl rand -base64 24)
POSTGRES_PASSWORD=你的_安全_postgres_密码
REDIS_PASSWORD=你的_安全_redis_密码
```

### 2. 启动服务

使用 Docker Compose 启动所有服务：

```bash
# 启动所有服务 (后台运行)
docker-compose up -d

# 查看服务状态
docker-compose ps

# 查看日志
docker-compose logs -f lfg-bot
```

### 3. 数据库初始化

数据库会自动初始化，但你可以手动运行迁移：

```bash
# 进入应用容器
docker-compose exec lfg-bot /bin/sh

# 运行数据库迁移
./migrate up

# 检查数据库健康状态
./migrate health
```

## 🏗️ 服务架构

Docker Compose 包含以下服务：

### 核心服务

| 服务名 | 端口 | 描述 |
|--------|------|------|
| `lfg-bot` | - | Discord LFG Bot 主应用 |
| `postgres` | 5432 | PostgreSQL 数据库 |
| `redis` | 6379 | Redis 缓存和会话存储 |

## 🔧 详细配置

### 数据库配置

PostgreSQL 配置在 `docker-compose.yml` 中：

```yaml
postgres:
  image: postgres:15-alpine
  environment:
    - POSTGRES_DB=lfgbot
    - POSTGRES_USER=lfguser
    - POSTGRES_PASSWORD=${POSTGRES_PASSWORD}
  volumes:
    - postgres_data:/var/lib/postgresql/data
    - ./sql/init:/docker-entrypoint-initdb.d  # 自动执行初始化脚本
```

### Redis 配置

Redis 配置包含密码保护和数据持久化：

```yaml
redis:
  image: redis:7-alpine
  command: redis-server --appendonly yes --requirepass ${REDIS_PASSWORD}
  volumes:
    - redis_data:/data
```

### 应用配置

Bot 应用的环境变量：

```yaml
lfg-bot:
  environment:
    # 数据库连接 (注意使用容器名作为主机名)
    - DATABASE_URL=postgres://lfguser:${POSTGRES_PASSWORD}@postgres:5432/lfgbot?sslmode=disable
    - REDIS_URL=redis://redis:6379
    # 其他 Discord 和应用配置...
```

## 📊 监控和管理

### 健康检查

所有服务都配置了健康检查：

```bash
# 查看所有服务健康状态
docker-compose ps

# 查看特定服务的健康检查日志
docker-compose logs postgres
```

### 健康检查端点

- **Bot 健康检查**: http://localhost:8080/health(校验数据库连通,返回 200 = 正常)

### 数据库管理

```bash
# 连接到 PostgreSQL
docker-compose exec postgres psql -U lfguser -d lfgbot

# 连接到 Redis
docker-compose exec redis redis-cli -a 你的_redis_密码

# 查看数据库日志
docker-compose logs postgres

# 备份数据库
docker-compose exec postgres pg_dump -U lfguser lfgbot > backup.sql
```

## 🛠️ 常用命令

### 服务管理

```bash
# 启动所有服务
docker-compose up -d

# 停止所有服务
docker-compose down

# 重启特定服务
docker-compose restart lfg-bot

# 查看服务状态
docker-compose ps

# 查看资源使用情况
docker-compose top
```

### 日志管理

```bash
# 查看所有服务日志
docker-compose logs

# 查看特定服务日志
docker-compose logs lfg-bot

# 实时跟踪日志
docker-compose logs -f lfg-bot

# 查看最近的日志
docker-compose logs --tail=100 lfg-bot
```

### 数据管理

```bash
# 清理未使用的容器和镜像
docker system prune

# 查看数据卷
docker volume ls

# 备份数据卷
docker run --rm -v lfg-bot-go_postgres_data:/data -v $(pwd):/backup alpine tar czf /backup/postgres_backup.tar.gz -C /data .

# 恢复数据卷
docker run --rm -v lfg-bot-go_postgres_data:/data -v $(pwd):/backup alpine tar xzf /backup/postgres_backup.tar.gz -C /data
```

## 🔒 安全配置

### 密码安全

确保在 `.env` 文件中使用强密码：

```bash
# 生成安全密码
openssl rand -base64 32

# 或使用 pwgen (如果已安装)
pwgen -s 32 1
```

### 网络安全

默认配置创建了隔离的 Docker 网络：

```yaml
networks:
  lfg-network:
    driver: bridge
```

### 数据持久化

重要数据存储在 Docker 卷中：

```yaml
volumes:
  postgres_data:    # PostgreSQL 数据
  redis_data:       # Redis 数据
```

## 🐛 故障排除

### 常见问题

1. **服务启动失败**
   ```bash
   # 查看详细错误信息
   docker-compose logs 服务名
   
   # 检查配置文件语法
   docker-compose config
   ```

2. **数据库连接失败**
   ```bash
   # 检查数据库是否就绪
   docker-compose exec postgres pg_isready -U lfguser
   
   # 测试连接
   docker-compose exec lfg-bot ping postgres
   ```

3. **端口冲突**
   ```bash
   # 查看端口使用情况
   netstat -tulpn | grep :5432
   
   # 修改 docker-compose.yml 中的端口映射
   ```

4. **权限问题**
   ```bash
   # 检查文件权限
   ls -la sql/init/
   
   # 修复权限
   chmod 644 sql/init/*.sql
   ```

### 重置环境

如果需要完全重置：

```bash
# 停止并删除所有容器、网络
docker-compose down

# 删除数据卷 (注意: 这会删除所有数据!)
docker-compose down -v

# 清理镜像
docker-compose down --rmi all

# 重新构建和启动
docker-compose up --build -d
```

## 📈 性能优化

### 资源限制

在 `docker-compose.yml` 中添加资源限制：

```yaml
lfg-bot:
  deploy:
    resources:
      limits:
        cpus: '1.0'
        memory: 512M
      reservations:
        cpus: '0.5'
        memory: 256M
```

### 数据库优化

PostgreSQL 性能调优：

```yaml
postgres:
  environment:
    - POSTGRES_INITDB_ARGS=--encoding=UTF-8 --lc-collate=C --lc-ctype=C
  command: >
    postgres
    -c shared_preload_libraries=pg_stat_statements
    -c max_connections=200
    -c shared_buffers=256MB
    -c effective_cache_size=1GB
```

## 🔄 更新和维护

### 更新应用

```bash
# 拉取最新代码
git pull

# 重新构建并启动
docker-compose up --build -d

# 运行数据库迁移 (如果有新的)
docker-compose exec lfg-bot ./migrate up
```

### 定期维护

```bash
# 清理日志 (定期执行)
docker system prune -f

# 更新基础镜像
docker-compose pull
docker-compose up -d

# 备份数据 (建议每日执行)
./scripts/backup.sh
```

## 📝 生产环境建议

1. **使用外部数据库**: 生产环境建议使用托管的 PostgreSQL 和 Redis 服务
2. **SSL/TLS**: 配置 HTTPS 和数据库 SSL 连接
3. **监控**: 利用 `/health` 端点接入外部拨测/告警(如 UptimeRobot)
4. **备份**: 实施自动化备份策略
5. **安全**: 定期更新镜像和依赖项

## 🆘 获取帮助

如果遇到问题：

1. 查看日志: `docker-compose logs 服务名`
2. 检查健康状态: `docker-compose ps`
3. 验证配置: `docker-compose config`
4. 查看文档: 本项目的其他 `.md` 文件

---

**注意**: 请确保 `.env` 文件不要提交到版本控制系统中，它包含敏感信息。 