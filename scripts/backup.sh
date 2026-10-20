#!/usr/bin/env bash
# Backup postgres + redis from the running docker-compose stack.
#
# Local backups go to ./backups (30-day rotation). Off-server copies are
# strongly recommended — set ONE of these before running (or in .env):
#   RCLONE_REMOTE=remote:bucket/lfg-backups   # any rclone remote (S3/R2/GDrive/...)
#   BACKUP_SSH_TARGET=user@host:/path         # rsync over ssh (e.g. your own machine)
#
# Suggested cron on the server (daily at 05:00):
#   0 5 * * * /path/to/repo/scripts/backup.sh >> /path/to/repo/logs/backup.log 2>&1
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKUP_DIR="${BACKUP_DIR:-$REPO_ROOT/backups}"
RETENTION_DAYS="${RETENTION_DAYS:-30}"
PG_CONTAINER="${PG_CONTAINER:-lfg-postgres}"
REDIS_CONTAINER="${REDIS_CONTAINER:-lfg-redis}"
STAMP="$(date +%Y%m%d_%H%M%S)"

# pull passwords from .env if not already exported
if [[ -f "$REPO_ROOT/.env" ]]; then
    set -a; source <(grep -E '^(REDIS_PASSWORD|RCLONE_REMOTE|BACKUP_SSH_TARGET)=' "$REPO_ROOT/.env" || true); set +a
fi

mkdir -p "$BACKUP_DIR"

# --- postgres ---
PG_FILE="$BACKUP_DIR/db_${STAMP}.sql.gz"
docker exec "$PG_CONTAINER" pg_dump -U lfguser -d lfgbot | gzip > "$PG_FILE"
gzip -t "$PG_FILE"
[[ $(stat -f%z "$PG_FILE" 2>/dev/null || stat -c%s "$PG_FILE") -gt 1024 ]] \
    || { echo "ERROR: postgres dump suspiciously small: $PG_FILE"; exit 1; }
echo "postgres -> $PG_FILE"

# --- redis (best effort; cache only) ---
if docker ps --format '{{.Names}}' | grep -qx "$REDIS_CONTAINER"; then
    docker exec "$REDIS_CONTAINER" redis-cli -a "${REDIS_PASSWORD:-}" --no-auth-warning BGSAVE >/dev/null
    sleep 5
    docker cp "$REDIS_CONTAINER:/data/dump.rdb" "$BACKUP_DIR/redis_${STAMP}.rdb" \
        && echo "redis    -> $BACKUP_DIR/redis_${STAMP}.rdb" \
        || echo "WARN: redis copy failed (cache only, continuing)"
fi

# --- rotation ---
find "$BACKUP_DIR" -name 'db_*.sql.gz' -mtime +"$RETENTION_DAYS" -delete
find "$BACKUP_DIR" -name 'redis_*.rdb' -mtime +"$RETENTION_DAYS" -delete

# --- offsite ---
if [[ -n "${RCLONE_REMOTE:-}" ]] && command -v rclone >/dev/null; then
    rclone copy "$BACKUP_DIR" "$RCLONE_REMOTE" --include 'db_*.sql.gz' --include 'redis_*.rdb'
    echo "offsite  -> $RCLONE_REMOTE"
elif [[ -n "${BACKUP_SSH_TARGET:-}" ]]; then
    rsync -az "$BACKUP_DIR/" "$BACKUP_SSH_TARGET/"
    echo "offsite  -> $BACKUP_SSH_TARGET"
else
    echo "WARN: no offsite target configured (RCLONE_REMOTE / BACKUP_SSH_TARGET) — backups are on this server only"
fi

echo "backup complete: $STAMP"
