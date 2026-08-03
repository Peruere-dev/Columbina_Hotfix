# ColumbinaHotfix

Genshin LAN dispatch server: hotfix configs, login/auth, region dispatch, admin panel.

## Build

```sh
# linux/arm64 (local) — output named `ColumbinaHotfix`
env GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.buildVersion=1.0.0" -o ColumbinaHotfix .

# all 6 platforms (release)
./build.sh            # interactive menu; version arg optional: ./build.sh 1.0.0
# names: linux/arm64 → `ColumbinaHotfix`, others → `ColumbinaHotfix_{GOOS}_{GOARCH}_{MODE}`
```

- `buildVersion` defaults to `1.0.0` in `main.go`, shown in startup box + REPL `status`; override via `-X main.buildVersion=...`.
- Module `columbinahotfix`, Go 1.22, pure Go SQLite (`modernc.org/sqlite`), no libc/CGO.

## Run

```sh
./ColumbinaHotfix    # → http://127.0.0.1:5200/admin
```

Binary alone is enough: `keys/` and `lang/` are **embedded** with disk override fallback (disk takes priority). `config.json`, `genshin.db`, `*.log` auto-created on first run. If `hotfix/` missing, server starts with warning (0 cached configs).

- **Timezone**: `main.go` forces `time.Local = time.FixedZone("CST", 8*3600)` (UTC+8). Termux lacks `zoneinfo`, so without this Go silently uses UTC and every log/timestamp is 8h slow. Never remove it or rely on system TZ.
- **Admin creds**: default `admin`/`123456` are seeded once and printed to console only on first run; login with default creds returns `must_change_password=true`. Login lockout: 3 failed attempts per IP → 10-min block (`adminMaxFails`/`adminLockMin` in database.go). Admin creds are never logged.

## dispatchSeed check (seed.go)

Per-version+platform verification of the `dispatchSeed` query param on `query_cur_region` (mimics official, config-driven). Seed values are **fixed per client version** (16 hex chars, kept out of git — see `dispatchseed_verified.json` locally) and are **unrelated to `dispatchSeed.bin`** (the 2076-byte `client_secret_key` blob sent to clients).

- Config: `config.json` → `"seedCheck": {"enabled": true}` (default on; `false` = test mode: no verify, no collect, no file-corrupt errors).
- Files in binary dir: `dispatchseed_verified.json` (manually-maintained official seeds, **gitignored — sensitive**) and `dispatchseed_collected.json` (runtime auto-collect, **gitignored**).
- Both files share one format: `{"CNREL6.7.0": {"Android": ["e00fa3521af2486e"], "Win": ["9c15fa34e16d014d"]}}` — version key omits the platform word, platform keys are `Android`/`Win`/`iOS`, values are arrays (a version+platform can hold multiple seeds). Request keys are normalized via `canonicalVersion()` (`CNRELAndroid6.7.0` → `CNREL6.7.0`) and `platformName()` (`2`→`Android`, `3`→`Win`, `1`→`iOS`).
- First run auto-creates both as `{}`. A file that exists but fails to parse ⇒ verify disabled, file left untouched, error logged (never rewritten — protects data).
- `seedReject(version, platform, dispatchSeed, regionSuffix)` logic:
  - verified hit → strict whole-string match; missing/mismatch → reject (`retcode=1`)
  - unverified + valid 16-hex seed → pass + record to collected (atomic tmp+rename)
  - invalid seed + probe path (suffix ∈ `config.regions[].Name`) → pass; else reject
- Reject response is `retcode=1`; encrypted or plaintext depends on `key_id` (see below). Invalid seeds are never written to collected.

## Dispatch quirks

- Single-host, multi-region: `dispatch_url` = `…/query_cur_region/{region}` suffix; official uses one host per region with **no suffix** (region encoded in hostname). Client requests the `dispatch_url` verbatim — both forms must keep working.
- `key_id` decides encryption: valid key (in `encryptionKeys`) → RSA-encrypted JSON `{content,sign}`; missing/`0`/invalid → base64 plaintext protobuf. There is **no default key_id** and **no dispatchSeed-triggered encryption** (official behavior, `crypto.go` `encryptAndSignRegionData`).
- Empty `version` → always base64 protobuf `retcode=1 "Not Found version config"`, regardless of suffix (official: no version ⇒ no client config).
- Unknown paths → `{"retcode":"-1","msg":"system error"}` HTTP 200; the server never 404s (official behavior).
- `platform` is read from the **query** param, not the path (path segment 2 is not platform).

## Tests

```sh
go test -run 'TestIsValidHex16|TestCanonicalKeys|TestSeedReject|TestEncryptAndSignKeyID' ./...
```

Single-package Go tests in `seed_test.go` (hex-format, canonical key normalization, verified strict, whitelist, corrupt-file fallback, key_id encryption). `TestSeedReject` writes collected file to `t.TempDir()`.

## Key files

| File | Role |
|------|------|
| `main.go` | entry, init order: config → chdir → lang → logging → DB → admin → keys → startup box → **initSeedCheck** → hotfix cache → HTTP → REPL + signal; `buildVersion` var |
| `server.go` | HTTP router + dispatch handlers + admin API + colored logRequest |
| `config.go` | Config struct (incl. `seedCheck`), load/save, auto-gen if missing |
| `seed.go` | dispatchSeed verify/collect, probe-path whitelist, atomic writes |
| `seed_test.go` | unit tests for seed check + key_id encryption |
| `crypto.go` | RSA keys, dispatch encryption/signing. Embedded keys with disk override |
| `database.go` | SQLite, session expiry (24h TTL), user CRUD |
| `hotfix.go` | hotfix JSON cache (`sync.RWMutex` map, all `hotfix/{region}/{plat}/{ver}.json` loaded at startup, no cap) |
| `admin.go` | `//go:embed static/admin.html` + i18n injection (`/*LANG_DATA*/` → JSON) |
| `proto.go` | protobuf builder for dispatch responses |
| `lang.go` | i18n loader, embedded with disk override fallback |
| `repl.go` | stdin REPL (help/status/stats/reload/stop), ANSI color constants, logInfo/Warn/Error/Request |
| `signal.go` | SIGINT/SIGTERM → graceful shutdown, SIGHUP → config + hotfix reload |
| `build.sh` | interactive platform + mode selection, optional version arg |

## Admin panel

Route configurable via `admin.route` in config.json (default `/admin`). Default creds `admin`/`123456` seeded once and printed to console at first startup only; default creds trigger `must_change_password` on login. Login via POST `/api/login` with `{username, password}`, token stored in localStorage.

## REPL commands

Type at running server terminal:
- `help` — list commands
- `status` — version, PID, uptime, request count, user stats
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

`hotfix/*/4.6.0.json` tracked (6 files); all other hotfix JSON gitignored. `config.json`, both `dispatchseed_*.json`, binaries, logs, DB gitignored.

## Key endpoints

| Path | Purpose |
|------|---------|
| `GET /query_region_list` | region list (protobuf base64) |
| `GET /query_cur_region/{region}` | region info (RSA-encrypted, dispatchSeed-checked) |
| `GET /query_gateserver` | same handler as query_cur_region |
| `GET /hk4e_{cn,global}/combo/granter/api/getConfig` | granter config (both prefixes) |
| `GET /hk4e_{cn,global}/mdk/shield/api/loadConfig` | load config (both prefixes) |
| `POST /hk4e_{cn,global}/mdk/shield/api/verify` | login auth |
| `POST /hk4e_{cn,global}/combo/granter/login/v2/login` | combo login |
| `POST /{admin}/api/login` | admin login |
| anything else | `{"retcode":"-1","msg":"system error"}` HTTP 200 |

## Verify script

```sh
python convert_hotfix.py [source_dir]  # convert ref configs → hotfix/ dir
python verify.py                       # download & validate all configs against servers
python verify.py path/to/version.json  # single version
```
