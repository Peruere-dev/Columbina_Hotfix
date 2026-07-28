# ColumbinaHotfix

Genshin LAN dispatch server: hotfix configs, login/auth, region dispatch, admin panel.

## Build

```sh
# linux/arm64 (local) — output named `ColumbinaHotfix`
env GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o ColumbinaHotfix .

# all 6 platforms
./build.sh          # interactive menu (linux/amd64,arm64  darwin/amd64,arm64  windows/amd64,arm64)
# build.sh names: linux/arm64 → `ColumbinaHotfix`, others → `ColumbinaHotfix_{GOOS}_{GOARCH}_{MODE}`
```

Module: `columbinahotfix`, Go 1.22, pure Go SQLite (`modernc.org/sqlite`), no libc/CGO.

## Run

```sh
./ColumbinaHotfix    # → http://127.0.0.1:5200/admin
```

Runtime requirements: the binary alone is enough. `keys/` and `lang/` are **embedded** with disk override fallback (disk takes priority). `config.json`, `genshin.db`, `*.log` auto-created on first run. If `hotfix/` missing, server starts with warning (0 cached configs).

## Key files

| File | Role |
|------|------|
| `main.go` | entry, init order: config → chdir → lang → logging → DB → admin → keys → startup box → hotfix cache → HTTP → REPL + signal |
| `server.go` | HTTP router + dispatch handlers + admin API + colored logRequest |
| `config.go` | Config struct, load/save, auto-gen if missing |
| `crypto.go` | RSA keys, dispatch encryption/signing. Embedded keys with disk override |
| `database.go` | SQLite, session expiry (24h TTL), user CRUD |
| `hotfix.go` | hotfix JSON cache (`sync.RWMutex` map, 300 configs loaded at startup) |
| `admin.go` | `//go:embed static/admin.html` + i18n injection (`/*LANG_DATA*/` → JSON) |
| `proto.go` | protobuf builder for dispatch responses |
| `lang.go` | i18n loader, embedded with disk override fallback |
| `repl.go` | stdin REPL (help/status/stats/reload/stop), ANSI color constants, logInfo/Warn/Error/Request |
| `signal.go` | SIGINT/SIGTERM → graceful shutdown, SIGHUP → config + hotfix reload |
| `build.sh` | interactive platform + mode selection |

## Admin panel

Route configurable via `admin.route` in config.json (default `/admin`). Default creds: `admin`/`123456` printed once at first startup. Login via POST `/api/login` with `{username, password}`, token stored in localStorage.

## REPL commands

Type at running server terminal:
- `help` — list commands
- `status` — PID, uptime, request count, user stats
- `stats` — version request counts
- `reload` — re-read config.json + hotfix cache (equivalent to SIGHUP)
- `stop` — graceful shutdown

## Hotfix config format (`hotfix/{CNREL,OSREL}/{Android,Win,iOS}/{version}.json`)

```json
{"ResourceUrl":"...","DataUrl":"...","ClientDataVersion":12345,"ClientSilenceDataVersion":12345,
 "ClientDataMd5":"...","ClientSilenceDataMd5":"...","ClientVersionSuffix":"...",
 "ClientSilenceVersionSuffix":"...","ResVersionConfig":{"Version":12345,"Md5":"...",
 "ReleaseTotalSize":"0","VersionSuffix":"...","Branch":"1.0_live"}}
```

## Git tracking

Only `hotfix/*/4.6.0.json` tracked (6 files for CNREL/OSREL × Android/Win/iOS). All other hotfix JSON gitignored.

## Key endpoints

| Path | Purpose |
|------|---------|
| `GET /query_region_list` | region list (protobuf base64) |
| `GET /query_cur_region/{region}` | region info (RSA-encrypted) |
| `POST /hk4e_{cn,global}/mdk/shield/api/verify` | login auth |
| `POST /hk4e_{cn,global}/combo/granter/login/v2/login` | combo login |
| `POST /{admin}/api/login` | admin login |

## Verify script

```sh
python convert_hotfix.py [source_dir]  # convert ref configs → hotfix/ dir
python verify.py                       # download & validate all configs against servers
python verify.py path/to/version.json  # single version
```