#!/usr/bin/env python3
"""Import rank leaderboard bindings into the bot database.

Input CSV, one binding per line (header row optional, '#' comments allowed):

    discord_id,RiotName#TAG
    # or three columns:
    discord_id,RiotName,TAG

For each row the script resolves the Riot PUUID and summoner ID via the
Riot API, then upserts a verified row into user_riot_bindings. Rank data
is filled in automatically by the bot's rank scheduler after import.

Usage:
    ./scripts/import_bindings.py bindings.csv            # resolve + write bindings.sql (dry run)
    ./scripts/import_bindings.py bindings.csv --apply    # also apply into the lfg-postgres container

Config (env vars, falling back to .env in the repo root):
    RIOT_API_KEY    required
    RIOT_PLATFORM   regional route for account-v1 (default: americas)
    RIOT_REGION     platform route for summoner-v4 (default: na1)
    PG_CONTAINER    postgres container name (default: lfg-postgres)
"""

import json
import os
import ssl
import subprocess
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

REPO_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))


def make_ssl_context():
    """macOS 自带 Python 常缺 CA 证书;默认上下文为空时回退到系统证书文件"""
    ctx = ssl.create_default_context()
    if ctx.cert_store_stats().get("x509_ca", 0) == 0:
        for cafile in ("/etc/ssl/cert.pem", "/etc/ssl/certs/ca-certificates.crt"):
            if os.path.exists(cafile):
                return ssl.create_default_context(cafile=cafile)
    return ctx


SSL_CTX = make_ssl_context()


def load_env_fallback():
    env_path = os.path.join(REPO_ROOT, ".env")
    if not os.path.exists(env_path):
        return
    with open(env_path, encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if not line or line.startswith("#") or "=" not in line:
                continue
            key, _, value = line.partition("=")
            key, value = key.strip(), value.strip().strip('"').strip("'")
            os.environ.setdefault(key, value)


def riot_get(url, api_key, max_retries=5):
    for _ in range(max_retries):
        # Riot 边缘的 Cloudflare 会按客户端特征拦截无 UA 的请求(error 1010),必须带 User-Agent
        req = urllib.request.Request(
            url, headers={"X-Riot-Token": api_key, "User-Agent": "lfg-bot-import/1.0"}
        )
        try:
            with urllib.request.urlopen(req, timeout=15, context=SSL_CTX) as resp:
                return json.load(resp)
        except urllib.error.HTTPError as e:
            if e.code == 429:
                wait = int(e.headers.get("Retry-After", "10"))
                print(f"    rate limited, waiting {wait}s...")
                time.sleep(wait)
                continue
            if e.code == 404:
                return None
            raise
    raise RuntimeError(f"still rate limited after {max_retries} retries: {url}")


def parse_line(line):
    parts = [p.strip() for p in line.split(",")]
    if len(parts) == 2 and "#" in parts[1]:
        name, _, tag = parts[1].rpartition("#")
        return parts[0], name.strip(), tag.strip()
    if len(parts) == 3:
        return parts[0], parts[1], parts[2].lstrip("#")
    raise ValueError(f"unrecognized line format: {line!r}")


def sql_quote(s):
    return "'" + s.replace("'", "''") + "'"


def main():
    args = [a for a in sys.argv[1:] if not a.startswith("--")]
    apply_sql = "--apply" in sys.argv
    if len(args) != 1:
        print(__doc__)
        sys.exit(1)

    load_env_fallback()
    api_key = os.environ.get("RIOT_API_KEY")
    if not api_key:
        sys.exit("RIOT_API_KEY not set (env or .env)")
    platform = os.environ.get("RIOT_PLATFORM", "americas")
    region = os.environ.get("RIOT_REGION", "na1")
    container = os.environ.get("PG_CONTAINER", "lfg-postgres")

    rows, failures = [], []
    with open(args[0], encoding="utf-8") as f:
        lines = [l.strip() for l in f if l.strip() and not l.strip().startswith("#")]

    for i, line in enumerate(lines):
        try:
            discord_id, name, tag = parse_line(line)
        except ValueError as e:
            if i == 0:  # tolerate a header row
                continue
            failures.append((line, str(e)))
            continue
        if not discord_id.isdigit():
            if i == 0:
                continue
            failures.append((line, "discord_id is not numeric"))
            continue

        print(f"[{i + 1}/{len(lines)}] {name}#{tag} (discord {discord_id})")
        try:
            acct_url = (
                f"https://{platform}.api.riotgames.com/riot/account/v1/accounts/by-riot-id/"
                f"{urllib.parse.quote(name)}/{urllib.parse.quote(tag)}"
            )
            account = riot_get(acct_url, api_key)
            if account is None:
                failures.append((line, "riot account not found (404)"))
                print("    NOT FOUND")
                continue
            time.sleep(1.5)

            summ_url = (
                f"https://{region}.api.riotgames.com/lol/summoner/v4/summoners/by-puuid/"
                f"{urllib.parse.quote(account['puuid'])}"
            )
            summoner = riot_get(summ_url, api_key)
            time.sleep(1.5)

            rows.append(
                {
                    "discord_id": discord_id,
                    "puuid": account["puuid"],
                    "riot_id": account.get("gameName", name),
                    "riot_tag": account.get("tagLine", tag),
                    "summoner_id": (summoner or {}).get("id"),
                }
            )
            print(f"    ok  puuid={account['puuid'][:12]}...")
        except Exception as e:  # keep going, report at the end
            failures.append((line, str(e)))
            print(f"    ERROR: {e}")

    if not rows:
        sys.exit(f"no bindings resolved ({len(failures)} failures)")

    stmts = ["BEGIN;"]
    for r in rows:
        summoner_sql = sql_quote(r["summoner_id"]) if r["summoner_id"] else "NULL"
        stmts.append(
            "INSERT INTO user_riot_bindings\n"
            "    (discord_id, riot_puuid, riot_id, riot_tag, summoner_id, is_verified)\n"
            f"VALUES ({sql_quote(r['discord_id'])}, {sql_quote(r['puuid'])}, "
            f"{sql_quote(r['riot_id'])}, {sql_quote(r['riot_tag'])}, {summoner_sql}, TRUE)\n"
            "ON CONFLICT (riot_puuid) DO UPDATE SET\n"
            "    discord_id = EXCLUDED.discord_id,\n"
            "    riot_id = EXCLUDED.riot_id,\n"
            "    riot_tag = EXCLUDED.riot_tag,\n"
            "    summoner_id = EXCLUDED.summoner_id,\n"
            "    is_verified = TRUE,\n"
            "    updated_at = NOW();"
        )
    stmts.append("COMMIT;")
    sql = "\n".join(stmts) + "\n"

    out_path = os.path.join(os.path.dirname(os.path.abspath(args[0])), "bindings.sql")
    with open(out_path, "w", encoding="utf-8") as f:
        f.write(sql)

    print(f"\nresolved {len(rows)}/{len(lines)} bindings -> {out_path}")
    if failures:
        print(f"{len(failures)} failed:")
        for line, err in failures:
            print(f"  {line}  ({err})")

    if apply_sql:
        print(f"\napplying into container {container}...")
        subprocess.run(
            ["docker", "exec", "-i", container, "psql", "-U", "lfguser", "-d", "lfgbot",
             "-v", "ON_ERROR_STOP=1"],
            input=sql.encode(),
            check=True,
        )
        print("done — the bot's rank scheduler will pull ranks on its next cycle.")
    else:
        print(f"\ndry run only. Apply with:\n  docker exec -i {container} psql -U lfguser -d lfgbot < {out_path}")


if __name__ == "__main__":
    main()
