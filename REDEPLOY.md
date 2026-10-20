# 全新服务器重新部署指南

> 背景:旧服务器过期,数据放弃。本次为全新部署 + rank 绑定手动导入 + 自动备份。

## 0. 先换掉所有泄露过的凭证(重要)

旧服务器整机丢失,且 `config/development.env` 里曾把真实 token 提交到过 GitHub,以下全部要换新:

- [ ] **Discord Bot Token** — [Discord Developer Portal](https://discord.com/developers/applications) → Bot → Reset Token
- [ ] **Riot API Key** — [Riot Developer Portal](https://developer.riotgames.com/)(dev key 本来就会过期,production key 建议 regenerate)
- [ ] **POSTGRES_PASSWORD / REDIS_PASSWORD** — 直接生成新的:`openssl rand -base64 24`
- [ ] `config/development.env`/`production.env` 已从 git 跟踪移除并加入 .gitignore(本地文件保留);历史提交里已泄露的值以轮换为准

## 1. 新服务器准备

```bash
# 安装 docker + compose 插件 (Ubuntu/Debian)
curl -fsSL https://get.docker.com | sh

git clone https://github.com/JaobsenYc/discord-lfg-bot-go.git
cd discord-lfg-bot-go
```

把本机的 `.env` 拷到服务器仓库根目录(scp 或手动粘贴),然后**把上面第 0 步的新凭证填进去**。

## 2. 启动

```bash
docker compose up -d --build
docker compose ps          # 4 个容器: lfg-bot / lfg-postgres / lfg-redis / lfg-db-backup
```

数据库 schema **无需任何手动步骤**:Postgres 容器首次启动(数据卷为空)时会按顺序自动执行
`sql/init/` 下的全部 SQL(001–006 + 迁移基线登记),完整建好所有表。
只有上线后要改表结构时才需要 `sql/migrations/`(从 007 起,见其中 README)。

验证:

```bash
docker logs -f discord-lfg-bot                        # 看到 bot 上线、slash 命令注册成功
docker compose ps                                     # lfg-bot 应显示 healthy
docker exec discord-lfg-bot /discord-lfg-bot -health-check && echo OK   # 手动探测
```

## 3. 导入 rank leaderboard 绑定

把绑定信息整理成 CSV(格式见 [scripts/bindings.example.csv](scripts/bindings.example.csv)):

```csv
discord_id,RiotName#TAG
```

然后在服务器上执行:

```bash
python3 scripts/import_bindings.py bindings.csv          # 先 dry run,检查解析结果
python3 scripts/import_bindings.py bindings.csv --apply  # 确认无误后写入数据库
```

脚本会通过 Riot API 把每个 `RiotName#TAG` 解析成 PUUID 并以「已验证」状态写入
`user_riot_bindings`;段位数据由 bot 的 rank scheduler 在下个周期自动抓取,无需手动操作。
解析失败的行会在末尾汇总,修正后重跑即可(脚本是 upsert,重复执行安全)。

## 4. 备份(这次必须做好)

三层防护:

1. **容器内每日自动备份**(已随 compose 启动):`lfg-db-backup` 容器每天
   `pg_dump` 一份到 `./backups/db_auto_*.sql.gz`,保留 30 天。零配置,但和服务器同生共死。
2. **异地备份(关键!上次丢数据就是因为没有这个)**:`scripts/backup.sh`
   支持推到任意 rclone 远端(S3 / Cloudflare R2 / Google Drive)或另一台机器:

   ```bash
   # 服务器上装 rclone 并配置一个远端后,在 .env 里加:
   RCLONE_REMOTE=r2:lfg-backups
   # 或者推回你自己的电脑/另一台机器:
   BACKUP_SSH_TARGET=user@host:/backups/lfg

   # 加到 crontab (crontab -e):
   0 5 * * * /root/discord-lfg-bot-go/scripts/backup.sh >> /root/discord-lfg-bot-go/logs/backup.log 2>&1
   ```

3. **续费日历提醒**:给服务器到期日设个提前 7 天的提醒——上次的根因是过期,不是故障。

恢复演练(建议部署完做一次):

```bash
gunzip -c backups/db_auto_XXXX.sql.gz | docker exec -i lfg-postgres psql -U lfguser -d lfgbot
```

## 常见问题

- **bot 起不来 / 数据库连不上**:`docker logs lfg-postgres` 看初始化是否完成;compose 内部走
  `postgres:5432`,宿主机侧映射为 `5433`(避免与其它服务冲突)。
- **旧备份**:`backup_20260106.sql` / `redis_backup.rdb`(2026-01-06 的旧数据)已决定放弃,
  但文件仍在 git 历史里;如介意其中的用户数据,可 `git rm` 并改写历史,或保持仓库私有。
