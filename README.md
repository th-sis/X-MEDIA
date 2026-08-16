# X-MEDIA

> 以 TMDB 元数据驱动的私人影视中心：先看元数据、再自动找资源、秒级开播。
> 客户端采用 **Kodi 风格 10 英尺 TV 界面**，后端基于 [LitePan](https://github.com/Ponphil/LitePan) 改造。

本仓库实现了设计文档《X-MEDIA-Design-Doc-v7.md》的架构骨架与可运行闭环：Go 单二进制后端（TMDB 代理 / 媒体库 / 四层播放引擎 / WebSocket / Ticket 流代理）+ Flutter 桌面客户端（Kodi 风格）+ Vue3 管理后台（复用 LitePan，内嵌）。

## 架构

```
┌──────────────────────────────────────────────┐
│  Flutter 客户端（Kodi 风格 TV UI，可编译 exe） │
│        HTTP + WebSocket                       │
└───────────────────┬──────────────────────────┘
                    ▼
┌──────────────────────────────────────────────┐
│  X-MEDIA 后端（Go 单二进制，基于 LitePan）      │
│  TMDB 代理 / 媒体库 / 四层播放引擎              │
│  Ticket 流代理 / WebSocket Hub / SQLite        │
└───────────────────┬──────────────────────────┘
                    ▼
   PanSou 盘搜（Sidecar，可选）· 网盘驱动（115/夸克/…）
```

- **四层播放引擎**：P0 NAS 本地秒播 → P1 盘搜转存 → P2 磁力云下载 → P3 订阅等待。
- **Ticket 流代理**：真实网盘直链永不出后端，客户端只持有 `/api/stream?ticket=xxx`。
- **演示模式**：未配置 TMDB Key / 网盘账号 / PanSou 时，后端以内置演示目录 + 演示视频兜底，**开箱即测**。

## 目录结构

```
├── X-MEDIA-Design-Doc-v7.md # 设计文档
├── server/                  # Go 后端（cmd/xmedia）+ Vue3 管理后台（server/web/，构建产物内嵌）
├── client/                  # Flutter 客户端（Kodi 风格）
├── docker-compose.yml       # 后端 + PanSou
└── Dockerfile               # 后端镜像
```

## 快速开始

### 方式一：Docker Compose 部署（推荐，NAS 一键部署）

x-media 镜像已发布到 GitHub Container Registry（私有包），由 GitHub Actions 在每次 push to main 时自动构建并推送。

```bash
# 1. 登录 GHCR（拉取私有镜像需要）
echo $GITHUB_TOKEN | docker login ghcr.io -u th-sis --password-stdin

# 2. 设置 NAS 路径（如无 NAS 可跳过，compose 会用占位目录）
export NAS_MEDIA_PATH=/mnt/nas/media

# 3. 启动（两个容器：xmedia + pansou）
docker compose up -d
```

- 管理后台：<http://your-host:38088>（**首次启动后必须立即修改默认管理员密码**；初始化向导 `OnboardingWizard` 会强制引导改密，见 `server/README.md`）
- 健康检查：`GET /api/health`
- 能力预检：`GET /api/capabilities`
- 镜像 tag：`latest`（默认）/ `7.0.0`（固定版本便于回滚）/ `sha-xxxxxxx`（特定 commit）

无需任何配置即可进入演示模式：`/api/tmdb/home` 返回 12 个榜单，`POST /api/resolve` 返回演示播放流。

### 方式二：本地开发（裸 Go 运行）

需要 Go 1.26+。

```bash
cd server
go run ./cmd/xmedia
# 默认监听 :38088，数据目录 ./data
```

如需 PanSou 盘搜，本地跑：

```bash
docker run -d --name pansou -p 8888:8888 ghcr.io/fish2018/pansou:latest
# 然后管理后台把 PanSou URL 改为 http://localhost:8888
```

### 2. 运行客户端（开发）

需要 Flutter 3.27+。

```bash
cd client
flutter run -d windows
```

### 3. 编译客户端 exe

```bash
cd client
flutter build windows --release
# 产物：client/build/windows/x64/runner/Release/xmedia_client.exe
```

### 4. 配置真实服务（可选）

| 项 | 说明 |
|---|---|
| TMDB API Key | 后端 `configs` 表 `tmdb_api_key`，或管理后台设置 |
| 网盘账号 | 管理后台添加 115/夸克/123/百度/光鸭 账号 |
| PanSou | 设置 `pansearch_url`（默认 `http://localhost:8888`） |

## 客户端操作

- **鼠标**：悬停高亮、点击确认。
- **键盘 / 遥控器**：方向键导航、回车/空格确认、Esc/返回键回退。
- 首页：顶部「继续观看」行 + 12 个横向榜单。
- 详情页：[播放] 弹出四层解析进度（NAS → 盘搜 → 磁力 → 订阅），就绪后自动进入播放器。

## API 契约

播放器 API（开放，无鉴权）与设计文档 §18 一致：

- `GET /api/tmdb/home|discover|search|detail/{id}|seasons/{id}`
- `POST /api/resolve`、`GET /api/resolve/result/{task_id}`、`GET /api/stream?ticket=xxx`
- `GET /api/capabilities`
- `GET/POST/DELETE /api/media/{continue-watching,history,favorites,subscriptions,search-history,check-availability}`
- `GET /ws`（WebSocket：`health_check` 首条消息 + `resolve_*`/`capabilities` 推送）

## 许可

后端基于 LitePan，遵循其 [PolyForm Noncommercial 1.0.0](server/LICENSE) 许可（个人学习与非商业使用，禁止商用）。详见 `server/THIRD_PARTY_NOTICES.md`。

## 说明

本项目为设计文档 v7 的可运行实现。相比完整 70-90 天路线图，当前实现聚焦可运行闭环与演示体验：网盘驱动的 ShareSaver/OfflineDownloader 接口与 P1/P2 真实转存链路以接口 + 演示数据呈现，真实网盘联调需接入对应驱动实现（见 `server/drivers/`）。
