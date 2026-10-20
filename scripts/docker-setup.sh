#!/bin/bash

# Discord LFG Bot Docker Compose 快速设置脚本
# 使用方法: ./scripts/docker-setup.sh

set -e

echo "🚀 Discord LFG Bot Docker Compose 设置"
echo "======================================"

# 检查 Docker 和 Docker Compose
if ! command -v docker &> /dev/null; then
    echo "❌ Docker 未安装，请先安装 Docker"
    exit 1
fi

if ! command -v docker-compose &> /dev/null; then
    echo "❌ Docker Compose 未安装，请先安装 Docker Compose"
    exit 1
fi

echo "✅ Docker 和 Docker Compose 已安装"

# 检查 .env 文件
if [ ! -f ".env" ]; then
    echo "📝 创建环境变量文件..."
    cp config/consise.env.example .env
    echo "⚠️  请编辑 .env 文件，配置以下必需的变量:"
    echo "   - DISCORD_TOKEN"
    echo "   - GUILD_ID"
    echo "   - POSTGRES_PASSWORD"
    echo "   - REDIS_PASSWORD"
    echo "   - REDIS_PASSWORD"
    echo ""
    echo "是否现在编辑 .env 文件? (y/n)"
    read -r response
    if [[ "$response" =~ ^[Yy]$ ]]; then
        ${EDITOR:-nano} .env
    else
        echo "⚠️  请手动编辑 .env 文件后重新运行此脚本"
        exit 1
    fi
fi

echo "✅ 环境变量文件已存在"

# 验证关键环境变量
source .env
if [ -z "$DISCORD_TOKEN" ] || [ "$DISCORD_TOKEN" = "your_discord_bot_token_here" ]; then
    echo "❌ 请在 .env 文件中设置有效的 DISCORD_TOKEN"
    exit 1
fi

if [ -z "$GUILD_ID" ] || [ "$GUILD_ID" = "your_discord_server_id_here" ]; then
    echo "❌ 请在 .env 文件中设置有效的 GUILD_ID"
    exit 1
fi

if [ -z "$POSTGRES_PASSWORD" ] || [ "$POSTGRES_PASSWORD" = "your_secure_postgres_password" ]; then
    echo "❌ 请在 .env 文件中设置安全的 POSTGRES_PASSWORD"
    exit 1
fi

if [ -z "$REDIS_PASSWORD" ] || [ "$REDIS_PASSWORD" = "your_secure_redis_password" ]; then
    echo "❌ 请在 .env 文件中设置安全的 REDIS_PASSWORD"
    exit 1
fi

echo "✅ 关键环境变量已配置"

# 创建必要的目录
echo "📁 创建必要的目录..."
mkdir -p logs backups

echo "✅ 目录创建完成"

# 检查配置文件语法
echo "🔍 验证 Docker Compose 配置..."
if ! docker-compose config > /dev/null 2>&1; then
    echo "❌ Docker Compose 配置文件有错误"
    docker-compose config
    exit 1
fi

echo "✅ Docker Compose 配置验证通过"

# 构建和启动服务
echo "🏗️  构建和启动服务..."
echo "这可能需要几分钟时间..."

# 拉取基础镜像
echo "📥 拉取 Docker 镜像..."
echo "使用 Redis 8 最新版本..."
docker-compose pull postgres redis

# 构建应用镜像
echo "🔨 构建应用镜像..."
docker-compose build lfg-bot

# 启动核心服务 (数据库优先)
echo "🗄️  启动数据库服务..."
docker-compose up -d postgres redis

# 等待数据库就绪
echo "⏳ 等待数据库启动..."
sleep 10

# 检查数据库健康状态
echo "🏥 检查数据库健康状态..."
timeout=60
counter=0
while [ $counter -lt $timeout ]; do
    if docker-compose exec -T postgres pg_isready -U lfguser -d lfgbot > /dev/null 2>&1; then
        echo "✅ PostgreSQL 已就绪"
        break
    fi
    echo "⏳ 等待 PostgreSQL 启动... ($counter/$timeout)"
    sleep 2
    counter=$((counter + 2))
done

if [ $counter -ge $timeout ]; then
    echo "❌ PostgreSQL 启动超时"
    docker-compose logs postgres
    exit 1
fi

# 检查 Redis
if docker-compose exec -T redis redis-cli -a "$REDIS_PASSWORD" ping > /dev/null 2>&1; then
    echo "✅ Redis 已就绪"
else
    echo "❌ Redis 连接失败"
    docker-compose logs redis
    exit 1
fi

# 启动主应用
echo "🤖 启动 Discord Bot..."
docker-compose up -d lfg-bot

# 等待应用启动
echo "⏳ 等待应用启动..."
sleep 5

# 数据库 schema 由 Postgres 容器首次启动时自动执行 sql/init/ 全量建好，无需迁移步骤
echo "🗄️  数据库 schema 已由容器初始化自动完成 (sql/init/)"

# 所有核心服务已启动

echo ""
echo "🎉 设置完成!"
echo "==============="
echo ""
echo "📊 服务状态:"
docker-compose ps

echo ""
echo "🔗 访问链接:"
echo "   - PostgreSQL: localhost:5432 (lfguser / $POSTGRES_PASSWORD)"
echo "   - Redis: localhost:6379"

echo ""
echo "📝 常用命令:"
echo "   - 查看日志: docker-compose logs -f lfg-bot"
echo "   - 重启服务: docker-compose restart lfg-bot"
echo "   - 停止所有服务: docker-compose down"
echo "   - 查看服务状态: docker-compose ps"

echo ""
echo "📚 更多信息请查看: docs/DOCKER_SETUP.md"

# 检查服务健康状态
echo ""
echo "🏥 最终健康检查..."
sleep 5

healthy_services=0
total_services=0

for service in lfg-bot postgres redis; do
    total_services=$((total_services + 1))
    if docker-compose ps -q $service | xargs docker inspect --format='{{.State.Health.Status}}' 2>/dev/null | grep -q "healthy\|starting"; then
        echo "✅ $service: 健康"
        healthy_services=$((healthy_services + 1))
    else
        echo "⚠️  $service: 状态未知或不健康"
    fi
done

echo ""
if [ $healthy_services -eq $total_services ]; then
    echo "🎉 所有核心服务都已正常启动!"
else
    echo "⚠️  部分服务可能需要更多时间启动，请使用 'docker-compose logs' 检查"
fi

echo ""
echo "✨ Discord LFG Bot 已准备就绪!" 