#!/bin/bash

# Discord LFG Bot 运行脚本
# 作者: Discord LFG Bot Team
# 版本: 1.0.0

set -e  # 遇到错误立即退出

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 打印带颜色的消息
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查是否安装了Go
check_go() {
    if ! command -v go &> /dev/null; then
        print_error "Go 未安装，请先安装 Go 1.23 或更高版本"
        exit 1
    fi
    
    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    print_info "检测到 Go 版本: $GO_VERSION"
}

# 检查是否安装了Docker
check_docker() {
    if ! command -v docker &> /dev/null; then
        print_warning "Docker 未安装，将无法使用容器化部署"
        return 1
    fi
    
    if ! command -v docker-compose &> /dev/null; then
        print_warning "Docker Compose 未安装，将无法使用 docker-compose"
        return 1
    fi
    
    print_info "Docker 和 Docker Compose 已安装"
    return 0
}

# 检查环境变量
check_env() {
    if [ ! -f ".env" ]; then
        if [ -f "config/consise.env.example" ]; then
            print_warning ".env 文件不存在，请复制 config/consise.env.example 并配置"
            cp config/consise.env.example .env
            print_info "已创建 .env 文件，请编辑并配置必要的环境变量"
        else
            print_error "config/consise.env.example 文件不存在"
            exit 1
        fi
    fi
    
    # 检查必需的环境变量
    source .env
    
    if [ -z "$DISCORD_TOKEN" ]; then
        print_error "DISCORD_TOKEN 环境变量未设置"
        exit 1
    fi
    
    if [ -z "$GUILD_ID" ]; then
        print_error "GUILD_ID 环境变量未设置"
        exit 1
    fi
    
    print_success "环境变量检查通过"
}

# 安装依赖
install_deps() {
    print_info "正在安装 Go 依赖..."
    go mod download
    go mod tidy
    print_success "依赖安装完成"
}

# 运行测试
run_tests() {
    print_info "正在运行测试..."
    go test -v ./...
    print_success "测试通过"
}

# 构建应用
build() {
    print_info "正在构建应用..."
    go build -o bin/discord-lfg-bot ./cmd/bot
    print_success "构建完成: bin/discord-lfg-bot"
}

# 运行应用 (开发模式)
run_dev() {
    print_info "正在以开发模式运行..."
    export LOG_LEVEL=debug
    go run ./cmd/bot
}

# 运行应用 (生产模式)
run_prod() {
    print_info "正在以生产模式运行..."
    if [ ! -f "bin/discord-lfg-bot" ]; then
        build
    fi
    ./bin/discord-lfg-bot
}

# Docker 构建
docker_build() {
    print_info "正在构建 Docker 镜像..."
    docker build -f docker/Dockerfile -t discord-lfg-bot:latest .
    print_success "Docker 镜像构建完成"
}

# Docker Compose 运行
docker_run() {
    if check_docker; then
        print_info "正在使用 Docker Compose 启动服务..."
        cd docker
        docker-compose up -d
        print_success "服务已启动"
        print_info "使用 'docker-compose logs -f' 查看日志"
        print_info "使用 'docker-compose down' 停止服务"
    else
        print_error "Docker 不可用"
        exit 1
    fi
}

# 清理
clean() {
    print_info "正在清理构建文件..."
    rm -rf bin/
    go clean
    print_success "清理完成"
}

# 显示帮助
show_help() {
    echo "Discord LFG Bot 运行脚本"
    echo ""
    echo "用法: $0 [命令]"
    echo ""
    echo "命令:"
    echo "  check       检查环境和依赖"
    echo "  install     安装Go依赖"
    echo "  test        运行测试"
    echo "  build       构建应用"
    echo "  dev         以开发模式运行"
    echo "  run         以生产模式运行"
    echo "  docker      构建Docker镜像"
    echo "  compose     使用Docker Compose运行"
    echo "  clean       清理构建文件"
    echo "  help        显示此帮助"
    echo ""
    echo "示例:"
    echo "  $0 check    # 检查环境"
    echo "  $0 dev      # 开发模式运行"
    echo "  $0 compose  # Docker部署"
}

# 主函数
main() {
    case "${1:-help}" in
        "check")
            check_go
            check_docker
            check_env
            ;;
        "install")
            check_go
            install_deps
            ;;
        "test")
            check_go
            run_tests
            ;;
        "build")
            check_go
            install_deps
            build
            ;;
        "dev")
            check_go
            check_env
            install_deps
            run_dev
            ;;
        "run")
            check_go
            check_env
            run_prod
            ;;
        "docker")
            check_docker
            docker_build
            ;;
        "compose")
            check_env
            docker_run
            ;;
        "clean")
            clean
            ;;
        "help"|*)
            show_help
            ;;
    esac
}

# 执行主函数
main "$@" 