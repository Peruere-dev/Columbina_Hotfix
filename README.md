# ColumbinaHotfix

原神 LAN 派发服务器（Dispatch Server），基于 Go 实现，支持热更新文件分发、登录验证、区域调度及网页管理后台。

> 项目名 **Columbina**（哥伦比娅），源自《原神》中的角色「 Columbina 」。

## 功能

- 热更新文件分发（按版本和平台：Android / iOS / Win）
- 官服兼容的 dispatch 协议（RSA 加密 + SHA256 签名）
- 游戏客户端登录 / 验证（shield / combo / granter）
- 多区域（region）查询与调度
- SQLite 用户管理（自动注册、封禁、UID 修改）
- 网页管理后台（仪表盘、用户管理）
- 多语言支持（中文 / English）
- 请求日志（data.log）

## 快速开始

### 构建

```bash
# ARM64（手机/服务器）
/usr/lib/go-1.22/bin/go build -o columbina-hotfix .

# 交叉编译 x86_64
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o columbina-hotfix .
```

### 配置

首次运行自动生成 `config.json`，按需修改：

```json
{
    "server": {
        "bindAddress": "0.0.0.0",
        "bindPort": 5200,
        "accessAddress": "127.0.0.1",
        "accessPort": 5200
    },
    "gameServer": {
        "accessAddress": "0.0.0.0",
        "accessPort": 22101
    },
    "admin": {
        "route": "/admin",
        "username": "admin",
        "password": "123456"
    },
    "regions": [
        {
            "Name": "hotfix",
            "Title": "<color=#AAFFFF>全版本热更新</color>",
            "Ip": "0.0.0.0",
            "Port": 22101
        }
    ],
    "unsupportedVersion": {
        "message": "暂不支持当前版本",
        "url": "https://ys.mihoyo.com"
    }
}
```

| 字段 | 说明 |
|------|------|
| `server.bindAddress` | 监听地址 |
| `server.bindPort` | 监听端口 |
| `server.accessAddress` | 对外访问地址（影响 dispatch URL） |
| `admin.username / password` | 管理后台登录凭据 |
| `regions` | 区域列表，决定 query_region_list 返回的服务器条目 |
| `autoCreateAccount` | 是否自动注册不存在的账号 |
| `unsupportedVersion` | 版本不支持时的提示信息和跳转链接 |
| `language` | 0=中文, 1=English |

### 运行

```bash
./columbina-hotfix
```

服务默认监听 `:5200`，管理后台 `http://<ip>:5200/admin`。

### 目录结构

```
ColumbinaHotfix/
├── columbina-hotfix    # 编译产物
├── config.json         # 配置文件
├── keys/               # 密钥（dispatchKey.bin, dispatchSeed.bin, game_keys/）
│   ├── dispatchKey.bin       # XOR 加密密钥 (4096 B)
│   ├── dispatchSeed.bin      # EC2B 格式分发种子 (2076 B)
│   ├── game_keys/            # RSA 密钥对（key_id 管理）
│   └── SigningKey.der
├── hotfix/             # 热更新版本配置
│   ├── Android/
│   │   ├── 1.0.0.json
│   │   ├── ...
│   │   └── 6.7.0.json
│   ├── iOS/
│   └── Win/
├── lang/               # 多语言翻译文件
├── static/admin.html   # 管理后台页面（Go embed）
├── genshin.db          # SQLite 数据库（自动创建）
└── data.log            # 请求日志
```

## 管理后台

- **仪表盘**：用户数、请求统计、版本分布
- **用户管理**：增删改查、封禁/解封、修改 UID/密码
- **语言切换**：中文 / English

## Dispatch 协议兼容

| 端点 | 说明 |
|------|------|
| `GET /query_region_list` | 返回 region 列表（protobuf base64） |
| `GET /query_cur_region/{region}` | 返回指定 region 信息（RSA 加密） |
| `POST /hk4e_cn/mdk/shield/api/verify` | 登录验证 |
| `POST /hk4e_cn/combo/granter/login/v2/login` | combo 登录 |
| `GET /combo/box/api/config/sdk/combo` | SDK 配置 |
| `GET /admin/mi18n/...` | 多语言资源 |

### 返回值格式

带 `dispatchSeed` 参数时返回 JSON：

```json
{
    "content": "RSA 加密后的 protobuf（base64）",
    "sign": "RSA-SHA256 签名（base64）"
}
```

不带 `dispatchSeed` 参数时返回裸 base64（兼容 hotfix.nyakya.com）。

## 技术栈

- Go 1.22
- SQLite（modernc.org/sqlite，纯 Go，无外部依赖）
- RSA-OAEP / RSA-PKCS1v15 加密
- 无第三方 HTTP 框架
- 纯文本管理界面（无前端构建工具）

## 参考项目

- [nod-krai-gi](https://github.com/mjolsic/nod-krai-gi) — Rust 实现的原神 Dispatch / Game Server
- [LunaGC](https://github.com/LunaGC/LunaGC-6.5.0) — Java (Grasscutter) 原神服务端
