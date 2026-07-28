# ColumbinaHotfix

Genshin LAN dispatch server in Go. Serves hotfix configs, login/auth, region dispatch, admin panel.

## Build

```sh
# interactive menu (platform + debug/release)
./build.sh

# quick local build (linux/arm64, release, named `ColumbinaHotfix`)
env GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o ColumbinaHotfix .
```

`build.sh` output names: linux/arm64 → `ColumbinaHotfix`, others → `ColumbinaHotfix_{GOOS}_{GOARCH}_{MODE}`.

## Run

Place next to `keys/`, `hotfix/`, `lang/` (all relative to binary directory). `config.json`, `genshin.db`, `*.log` auto-created.

```sh
./ColumbinaHotfix
# → http://<ip>:5200/admin
```

## Architecture

Single Go 1.22 binary, no frameworks, pure Go SQLite (`modernc.org/sqlite`).

| File | Role |
|------|------|
| `main.go` | entry, init order: config → chdir → lang → DB → admin → keys → logging → HTTP |
| `server.go` | HTTP router + dispatch handlers + admin API + logging |
| `config.go` | config struct, default, load/save, auto-gen if missing |
| `crypto.go` | RSA keys, dispatch encryption/signing |
| `database.go` | SQLite, auto-creates tables on first run |
| `hotfix.go` | read hotfix JSON configs from `hotfix/{CNREL,OSREL}/{platform}/{version}.json` |
| `admin.go` | `//go:embed static/admin.html` + i18n injection |
| `proto.go` | protobuf builder for dispatch responses |
| `lang.go` | i18n loader, fallback to raw key string if file missing |

### Key runtime files

- `static/admin.html` — embedded via `//go:embed`, recompile to update
- `lang/lang_cn.json`, `lang_en.json` — read-only at runtime
- `keys/dispatchKey.bin`, `dispatchSeed.bin`, `SigningKey.der`, `game_keys/` — required, panics if missing
- `hotfix/CNREL/OSREL/{Android,Win,iOS}/*.json` — loaded per request, silently skipped if not found

### Admin HTML

`admin.html` contains `/*LANG_DATA*/` placeholder. `admin.go:buildAdminHTML()` replaces it with JSON of all i18n keys at serve time.

## Hotfix config format

```json
{
  "ResourceUrl": "https://...",
  "DataUrl": "https://...",
  "ClientDataVersion": 12345,
  "ClientSilenceDataVersion": 12345,
  "ClientDataMd5": "{\"remoteName\":\"data_versions\",\"md5\":\"...\",\"fileSize\":123}",
  "ClientSilenceDataMd5": "{\"remoteName\":\"data_versions\",\"md5\":\"...\",\"fileSize\":456}",
  "ClientVersionSuffix": "...",
  "ClientSilenceVersionSuffix": "...",
  "ResVersionConfig": {
    "Version": 12345,
    "Md5": "json lines of md5 entries",
    "ReleaseTotalSize": "0",
    "VersionSuffix": "...",
    "Branch": "1.0_live"
  }
}
```

## Verify script

```sh
python convert_hotfix.py [source_dir]  # convert ref configs -> hotfix/ dir
python verify.py  # download & validate all configs against servers
python verify.py path/to/version.json # single version
```

## Git tracking

Only `hotfix/*/4.6.0.json` is tracked (see `.gitignore`). All other hotfix JSON is local-only.

## Key endpoints

| Path | Purpose |
|------|---------|
| `/query_region_list` | region list (protobuf base64) |
| `/query_cur_region/{region}` | region info (RSA-encrypted) |
| `/hk4e_*/mdk/shield/api/verify` | login auth |
| `/hk4e_*/combo/granter/login/v2/login` | combo login |
| `/admin` | management panel |
