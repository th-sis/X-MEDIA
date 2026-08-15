# X-MEDIA 项目设计文档 v7.0

> **最后更新**: 2026-08-09
> **状态**: 设计阶段（已定稿，待编码）
> **基础**: LitePan 开源项目改造 + Flutter 播放器前端 + PanSou Sidecar 搜索
> **核心理念**: TMDB 元数据驱动，不是"有文件去刮削"，而是"根据元数据去找文件"
> **v7 变更**：架构审查全量落地——补 UX 8 项（继续观看/搜索页/进度可视化/可播放标识/季集可用性/P2后台/骨架屏/搜索引导）+ TECH 8 项（能力预检API/media_library清理/多语言搜索/P0跳过优化/配置验证/并发限流/缓存失效/Design Token）。新增 §17.10 搜索页、§17.11 继续观看行、§17.12 骨架屏规范、附录 C Design Token；修正 §6.7 多语言关键词回退、§6.3 P0 智能跳过、§8.5 PanSou 缓存失效、§11.1 配置验证、§12.1 新增 capabilities/continue-watching 端点。
> **v6 变更**：将 v4 (2719 行) 与 v5 (983 行增量) 合并为单文件独立完整版。保留所有 v5 新增章节（§1.4 Day 1、§6.9 转存路径、§9.7 NAS 扫描、§27.4 健康映射、§28 启动序）和修正（§1.3 驱动命名三层、§5.1 补字段、§6.4 P1/P2 关键词、§11.1 NAS 可关、§15 工期 60-80 天）。v4 文档和 v5 delta 文档保留作为历史归档。
> **v5 变更**: 在 v4 基础上完成 P0/P1/P2 全量补充——补 §1.4 Day 1 旅程、§6.9 转存路径与配额、§9.7 NAS 扫描生命周期、§27.4 健康状态映射、§28 启动序与恢复协议；修正一致性 10 项；工期预估修正至 60-80 天；新增 D29-D40 决策记录
> **v4 变更**: P0-P3 审查修改全部落地（PanSou Sidecar / ShareSaver 接口 / Ticket 机制 / 季集索引 / external_id 统一 / ResolveTask 持久化等）

---

## 目录

1. [项目定位与设计哲学](#1-项目定位与设计哲学)
2. [整体架构](#2-整体架构)
3. [后端改造](#3-后端改造)
4. [前端设计](#4-前端设计)
5. [数据库设计](#5-数据库设计)
6. [四层播放引擎](#6-四层播放引擎)
7. [TMDB 服务](#7-tmdb-服务)
8. [盘搜引擎（PanSearch）](#8-盘搜引擎pansearch)
9. [索引引擎](#9-索引引擎)
10. [WebSocket](#10-websocket)
11. [策略配置](#11-策略配置)
12. [HTTP API 契约](#12-http-api-契约)
13. [模块裁剪清单](#13-模块裁剪清单)
14. [旧项目排坑清单](#14-旧项目排坑清单)
15. [开发路线图](#15-开发路线图)
16. [详细数据结构规范](#16-详细数据结构规范)
17. [前端界面与交互规范](#17-前端界面与交互规范)
18. [HTTP API 契约详细规范](#18-http-api-契约详细规范)
19. [异常处理与容错规范](#19-异常处理与容错规范)
20. [状态管理与数据流](#20-状态管理与数据流)
21. [性能与缓存规范](#21-性能与缓存规范)
22. [安全规范](#22-安全规范)
23. [开发路线图细化（TDD 驱动）](#23-开发路线图细化tdd-驱动)
24. [外部媒体库集成（v1.1 储备）](#24-外部媒体库集成v11-储备)
25. [字幕自动搜索与索引系统（v1.1 储备）](#25-字幕自动搜索与索引系统v11-储备)
26. [播放器防崩溃规范](#26-播放器防崩溃规范)
27. [启动健康检查与状态机增强](#27-启动健康检查与状态机增强)
28. [启动序与恢复协议](#28-启动序与恢复协议)（v5 新增）
29. [附录 A: 项目目录结构](#附录-a-项目目录结构)
30. [附录 B: 关键设计决策记录](#附录-b-关键设计决策记录)
31. [附录 C: Design Token 设计系统](#附录-c-design-token-设计系统)（v7 新增）

---

## 1. 项目定位与设计哲学

### 1.1 核心理念：TMDB 元数据驱动

X-MEDIA 与传统媒体中心的根本区别：

| 维度 | 传统媒体中心（Emby/Jellyfin/Plex） | X-MEDIA |
|---|---|---|
| 起点 | 有文件 -> 刮削元数据 | 看元数据 -> 去找文件 |
| 用户体验 | 浏览本地文件库 -> 播放 | 浏览 TMDB 榜单/分类 -> 点击播放 -> 后端自动找资源 |
| 文件管理 | 用户手动管理目录结构 | 后端自动转存/索引，用户无感 |

**项目灵魂**：先把 TMDB 展示给用户，让用户找感兴趣的内容，然后项目负责实现最快播放。

### 1.2 适用场景

单用户/单家庭局域网。不同用户的主网盘可能不同（有人 115 为主，有人 123 为主），因此网盘优先级和索引策略必须用户可配，不硬编码。

### 1.3 技术栈总览（v5 修正：增驱动命名规范）

| 层 | 技术 | 说明 |
|---|---|---|
| 前端播放器 | Flutter 3.27 + fvp (mdk-sdk) | 弃 libmpv（DV 色彩问题），TV 遥控器适配 |
| 后端 | Go（基于 LitePan 改造） | 单二进制，HTTP + WebSocket |
| 盘搜服务 | PanSou（Go 独立服务） | Sidecar HTTP 模式，端口 8888 |
| 管理后台 | Vue3（保留 LitePan 原有） | 多用户局域网管理，热更新不重编译 Flutter |
| 数据库 | SQLite | 轻量，单文件，够用 |

**驱动命名规范（v5 统一）**：

| 用途 | 命名 | 示例 |
|---|---|---|
| Go 包路径 | `{vendor}_Open` 或 `{vendor}` | `115_Open`, `Quark`, `123_Open`, `Baidu_Open`, `Guangya`, `LocalFs` |
| `media_index.source_type` | `pan{vendor}` 或 `nas` | `pan115`, `quark`, `pan123`, `baidu`, `guangya`, `nas` |
| `resolve_tasks.result_source` | 同 source_type | 同上 |
| `pansearch` cloud_type | 数字/原名（PanSou 协议） | `115`, `quark`, `123`, `baidu`, `guangya`, `magnet`, `ed2k` |
| WebSocket `accounts[].driver` | source_type 命名 | 同 source_type |

**包路径 vs 数据库字段 vs PanSou 协议** 三层命名各司其职，编码时不得混用。

### 1.4 用户首次启动旅程（Day 1，v5 新增，v7 修订）

本节描述一个全新用户从安装到第一次成功播放的完整路径，用作编码前自检清单。

#### Step 1: 部署（用户视角）

1. 用户从 GitHub release 下载 `x-media.zip` 或 `docker-compose.yml`
2. 解压/启动两个容器：x-media + pansou
3. 浏览器打开 `http://localhost:8080` -> Vue 管理后台（首次启动显示初始化向导）

#### Step 2: 后端配置（必须完成的 3 项）

管理后台首屏"初始化向导"按顺序要求：

1. **设置管理员密码**（`POST /api/auth/init`）
   - 至少 8 位
   - 写入 `configs.admin_password_hash`（bcrypt）
   - 后续所有管理操作需 JWT

2. **配置 TMDB API Key**（`PUT /api/admin/tmdb/config`）
   - 用户去 tmdb.com 申请 v3 auth key（界面提供链接 + 截图引导）
   - 后端立即测试（`POST /api/admin/tmdb/test`）
   - 成功 -> 跳转下一步

3. **添加至少一个网盘账号**（`POST /api/admin/accounts`）
   - 115 OAuth：浏览器跳转授权页 -> 回调
   - 夸克扫码：二维码展示 -> 用户扫码 -> 后台 cookie 检测
   - 成功 -> 账号状态变 green

#### Step 3: Flutter 端配置

1. 安装 `x-media.apk` / `x-media.dmg` / `x-media.exe`
2. 打开 App -> 设置页 -> 输入后端地址（支持 mDNS 发现 `x-media.local`，fallback 手动 IP）
3. 输入管理员密码登录 -> 拿到 JWT
4. WebSocket 自动连接 -> 收到 `health_check` 首条消息
5. 显示"系统状态"页：
   - ✅ TMDB 已配置
   - ✅ 115 网盘已登录
   - ⚠️ NAS 路径未配置（[去设置] 按钮）
   - ⚠️ PanSou 健康

#### Step 4: 第一次播放

1. 探索页 -> 12 个榜单已加载（首页顶部展示"继续观看"行，首次为空隐藏）
2. 点阿凡达 -> 详情页加载
3. 点播放 -> Resolve Modal 弹出：
   - `nas_lookup` (0ms) -> not found（首次无索引）
   - `pan_searching` (3s) -> 找到 5 个
   - `transferring` (8s) -> 转存到 115
   - `resolving_link` (1s) -> `play_ready`
   - 自动跳转播放器
4. 播放过程中每 10s 自动上报进度 -> 首页"继续观看"行出现

总耗时：~12 秒。

#### Step 5: 后续日常

- 第二次播放同一部 -> P1 索引命中，< 100ms
- 季集下钻：点 S01E03 -> 同样的 4 层流程（season/episode 参数），已缓存的集显示绿色✓角标
- 没找到的剧 -> 自动订阅 -> 下周收到 `notification`
- 首页顶部「继续观看」行：显示上次未看完的内容，一键续播

#### 编码前自检问题（v5 必答，v7 补 1 项）

| 问题 | 答案必须在文档中明确 |
|---|---|
| 用户没有 TMDB Key 能否进入 App？ | 能，但首页报错并引导去申请 |
| 用户没有网盘账号能否播放？ | 能走 P0 NAS（如果配了），否则引导订阅 |
| NAS 路径不存在能否启动？ | 能，启动时 health_check `nas=not_accessible` |
| 用户忘记 admin 密码？ | 通过配置文件 `x-media.reset-password=true` 重置（启动时检测） |
| Flutter 找不到后端？ | 设置页明确显示"无法连接 x-media.local:8080" + 提供 ping 工具 |
| mDNS 不通时如何发现？ | 手动输入 IP，保存到 settings |
| Docker 部署下 mDNS 工作吗？ | 取决于网络模式，建议 docker-compose bridge + 手动 IP |
| 用户打开 App，首页能看到什么？ | [v7 新增] 顶部"继续观看"行（有播放记录时）+ 12 个榜单行；无记录时仅榜单；NAS 未索引时搜索页/详情页卡片无绿色可播放角标 |

### 1.5 本文档演进史（v5 新增，v7 修订）

- v3：初版架构设计
- v4：增加 Sidecar PanSou / ShareSaver / Ticket / external_id / ResolveTask 持久化
- v5：在 v4 基础上完成 P0/P1/P2 全量补充——补齐启动序、转存路径、NAS 扫描生命周期、健康状态映射、Day 1 旅程；修正一致性 10 项；工期预估调整
- v6：将 v4 与 v5 delta 合并为单文件独立完整版
- v7（本版）：架构审查全量落地——补 UX 8 项 + TECH 8 项（继续观看/搜索页/进度可视化/可播放标识/季集可用性/P2后台行为/骨架屏/搜索引导 + 能力预检API/media_library清理/多语言搜索/P0跳过/配置验证/并发限流/缓存失效/Design Token）

---

## 2. 整体架构

```
┌─────────────────────────────────────────────────────┐
│                   用户终端                            │
│                                                      │
│  ┌──────────────┐        ┌──────────────────────┐   │
│  │ Flutter 播放器 │        │ Vue3 管理后台(浏览器)  │   │
│  │ (TV/桌面/移动) │        │ (网盘配置/索引/策略)   │   │
│  └──────┬───────┘        └──────────┬───────────┘   │
│         │ HTTP + WebSocket           │ HTTP           │
└─────────┼────────────────────────────┼───────────────┘
          │                            │
          ▼                            ▼
┌─────────────────────────────────────────────────────┐
│              X-MEDIA 后端（Go 单二进制）               │
│                                                      │
│  ┌─────────┐ ┌──────────┐ ┌───────────┐            │
│  │ TMDB代理 │ │ Resolve  │ │ WebSocket │            │
│  │ +Bangumi│ │ 四层引擎  │ │   Hub     │            │
│  └─────────┘ └──────────┘ └───────────┘            │
│  ┌─────────┐ ┌──────────┐ ┌───────────┐            │
│  │ 索引引擎 │ │ PanSearch│ │ Playback  │            │
│  │         │ │ Service  │ │ + Ticket  │            │
│  └─────────┘ └─────┬────┘ └───────────┘            │
│  ┌─────────┐       │        ┌───────────┐            │
│  │ SQLite  │       │        │ EventBus  │            │
│  └─────────┘       │        └───────────┘            │
│                    │ HTTP                               │
└────────────────────┼──────────────────────────────────┘
                     │ localhost:8888
                     ▼
              ┌──────────────┐
              │  PanSou 服务  │
              │ (TG+插件搜索) │
              └──────────────┘
```

### 2.1 通信协议

| 通道 | 协议 | 用途 |
|---|---|---|
| Flutter <-> 后端 | HTTP（REST JSON） | TMDB 浏览/搜索/详情、Resolve 触发/查询、媒体库 CRUD |
| Flutter <-> 后端 | WebSocket | Resolve 阶段实时推送、订阅通知、健康检查、心跳 |
| Vue 后台 <-> 后端 | HTTP（REST JSON） | 网盘配置/索引管理/策略设置/TMDB 配置 |
| 后端 <-> PanSou | HTTP（REST JSON） | 搜索请求/链接检测/健康检查 |
| 后端 <-> 网盘 | 各驱动 API | 文件列表/直链解析/分享转存/离线下载 |

WebSocket 重连策略：指数退避 1s -> 2s -> 4s -> 8s -> 16s -> max 30s，重连后 HTTP 补刷一次状态快照。

### 2.2 单实例约束（v5 明确）

v1.0 **不支持多实例**：WebSocket Hub、内存缓存、ResolveTask 状态机均为单进程设计。未来如需高可用，需重构为：
- WebSocket Hub 替换为 Redis Pub/Sub
- L1 缓存替换为 Redis
- L2/L3 SQLite 替换为主从或 PostgreSQL

---

## 3. 后端改造

### 3.1 LitePan 源码分析摘要

LitePan（github.com/Ponphil/LitePan）是 Go 编写的网盘聚合管理器。关键发现：

- **Store** 聚合 14 个 Repository（Accounts/Configs/Notifications/StrmTasks/OfflineDownloads/AutomationRules 等），基于 SQLite + 版本化 Migration
- **crosstransfer** 模块做的是基于文件 hash 的**跨盘秒传**，不是分享链接转存
- **115_Open 驱动**使用 OAuth 认证（access_token + refresh_token），无扫码
- **Quark 驱动**有完善扫码（361 行，CAS 流程，cookieCollector 多域 Set-Cookie 收集器）
- **playback** 模块有 Range 代理和 302 Redirect，但无 ticket 概念
- **eventbus** 定义了 OfflineDownloadCompleted/NotificationCreated 等事件类型
- **automation** 模块有规则引擎和定时调度器（每 10 秒扫描规则表）

### 3.2 改造原则（v5 修正：v1.0/v1.1 embyproxy 隔离）

1. **保留复用**：account/auth/api/app/automation/cache/cacheretention/config/core/crosstransfer/domain/driver/eventbus/file/offlinedownload/playback/store/upload/notification
2. **砍除（v1.0）**：fusemount/fusereadcache/fnosproxy/strm/strmscrape/mediaorganize/share/favorites(旧)/多余驱动/WebAPI/OneDrive/template/apikey
3. **新增**：tmdb/pansearch/media/indexengine/resolve/websocket/playback(ticket 扩展)
4. **保留改造（v1.1 重新引入）**：embyproxy 改造为可选外部媒体库模块（v1.1 储备，v5 明确：v1.0 砍除 embyproxy 仅指本版本，v1.1 通过 §24 重新引入并扩展）

---

## 4. 前端设计

### 4.1 页面结构（v7 修订：新增搜索页）

| 页面 | 说明 |
|---|---|
| 探索页（首页） | TMDB 直送 12 个榜单 + **顶部「继续观看」横向行**（v7 新增），横向卡片行 |
| 搜索页 | [v7 新增] 全局搜索：TMDB 搜索 + 搜索历史 + 无结果引导 |
| 分类页 | 5 Tab（电影/电视/综艺/动漫/纪录），网格式瀑布流 |
| 详情页 | TMDB 直送，含播放/收藏/订阅按钮，电视剧含季集列表（**已索引集显示绿色✓角标**，v7 新增） |
| 播放器页 | fvp 播放器，含进度上报/字幕/倍速 |
| 历史页 | 播放历史列表，同 TMDB ID 去重 |
| 订阅页 | 订阅列表，状态徽章 |
| 设置页 | 后端连接/播放器配置/系统状态 |

### 4.2 技术选型

- Flutter 3.27.1 + Dart 3.6.0
- fvp（mdk-sdk）替代 libmpv，解决 Dolby Vision 色彩问题
- TV 焦点管理：全局 TvButton 组件，四方向焦点导航
- 图片缓存：cached_network_image（200MB LRU）
- WebSocket 客户端：指数退避重连 + HTTP 补刷
- [v7 新增] Design Token 系统：附录 C 定义，Flutter + Vue 共享变量

---

## 5. 数据库设计

### 5.1 Migration 清单（v5 修正：0022/0020 补字段；v7 新增：0026 media_library 清理策略）

LitePan 现有 migration 0015 及之前。新增：

#### 0016_media_library.sql

```sql
CREATE TABLE media_library (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    external_id     INTEGER NOT NULL,             -- TMDB ID 或 Bangumi ID
    external_source TEXT NOT NULL DEFAULT 'tmdb', -- tmdb / bangumi
    media_type      TEXT NOT NULL,                -- movie/tv/anime/variety/documentary
    title           TEXT NOT NULL,
    title_orig      TEXT NOT NULL DEFAULT '',
    poster_url      TEXT NOT NULL DEFAULT '',
    backdrop_url    TEXT NOT NULL DEFAULT '',
    overview        TEXT NOT NULL DEFAULT '',
    year            INTEGER NOT NULL DEFAULT 0,
    vote_avg        REAL NOT NULL DEFAULT 0,
    vote_count      INTEGER NOT NULL DEFAULT 0,
    genres          TEXT NOT NULL DEFAULT '[]',   -- JSON: [{"id":28,"name":"动作"},...]
    runtime         INTEGER NOT NULL DEFAULT 0,
    seasons         INTEGER NOT NULL DEFAULT 0,
    episodes        INTEGER NOT NULL DEFAULT 0,
    seasons_json    TEXT NOT NULL DEFAULT '[]',   -- JSON: 季集结构缓存
    cast            TEXT NOT NULL DEFAULT '[]',   -- JSON: [{"name":"演员","character":"角色","profile_url":"..."},...]
    extra           TEXT NOT NULL DEFAULT '{}',   -- JSON: 扩展字段 (含 stale 标记)
    cached_at       TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_accessed_at TIMESTAMP,                   -- [v7 新增] 最后访问时间（LRU 淘汰用）
    UNIQUE(external_id, external_source)
);
CREATE INDEX idx_media_library_ext ON media_library(external_id, external_source);
CREATE INDEX idx_media_library_accessed ON media_library(last_accessed_at); -- [v7 新增]
```

#### 0017_media_index.sql

```sql
CREATE TABLE media_index (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    external_id     INTEGER NOT NULL,             -- TMDB ID 或 Bangumi ID
    external_source TEXT NOT NULL DEFAULT 'tmdb', -- tmdb / bangumi
    season          INTEGER NOT NULL DEFAULT 0,   -- 季号，0=电影/整季
    episode         INTEGER NOT NULL DEFAULT 0,   -- 集号，0=电影/整季
    media_type      TEXT NOT NULL,
    title           TEXT NOT NULL,
    original_name   TEXT NOT NULL DEFAULT '',
    year            INTEGER NOT NULL DEFAULT 0,
    source_type     TEXT NOT NULL,                -- nas/pan115/quark/pan123/baidu/guangya
    account_id      INTEGER NOT NULL DEFAULT 0,   -- NAS 时为 0
    file_path       TEXT NOT NULL,
    file_id         TEXT NOT NULL DEFAULT '',     -- 网盘文件 ID（NAS 为空）
    file_size       INTEGER NOT NULL DEFAULT 0,
    file_format     TEXT NOT NULL DEFAULT '',
    match_status    TEXT NOT NULL DEFAULT 'unconfirmed', -- matched/unconfirmed/orphaned
    match_score     REAL NOT NULL DEFAULT 0,
    stream_url      TEXT NOT NULL DEFAULT '',
    url_expires     TIMESTAMP,
    last_played_at  TIMESTAMP,                    -- 最后播放时间（清理策略用）
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(source_type, file_path)
);
CREATE INDEX idx_media_index_ext ON media_index(external_id, external_source, season, episode);
CREATE INDEX idx_media_index_source ON media_index(source_type, account_id);
```

#### 0018_play_history.sql

```sql
CREATE TABLE play_history (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    external_id     INTEGER NOT NULL,
    external_source TEXT NOT NULL DEFAULT 'tmdb',
    media_type      TEXT NOT NULL,
    title           TEXT NOT NULL,
    poster_url      TEXT NOT NULL DEFAULT '',
    source_type     TEXT NOT NULL,                -- 播放来源
    season          INTEGER NOT NULL DEFAULT 0,  -- [v7 新增] 季号
    episode         INTEGER NOT NULL DEFAULT 0,  -- [v7 新增] 集号
    position_ms     INTEGER NOT NULL DEFAULT 0,   -- 上次播放位置（毫秒）
    duration_ms     INTEGER NOT NULL DEFAULT 0,   -- 总时长（毫秒）
    played_at       TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(external_id, external_source, season, episode) -- [v7 修正] 支持季集维度
);
```

#### 0019_favorites.sql

```sql
CREATE TABLE favorites (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    external_id     INTEGER NOT NULL,
    external_source TEXT NOT NULL DEFAULT 'tmdb',
    media_type      TEXT NOT NULL,
    title           TEXT NOT NULL,
    poster_url      TEXT NOT NULL DEFAULT '',
    year            INTEGER NOT NULL DEFAULT 0,
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(external_id, external_source)
);
```

#### 0020_subscriptions.sql（v5 补字段：result_account_id）

```sql
CREATE TABLE subscriptions (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    external_id     INTEGER NOT NULL,
    external_source TEXT NOT NULL DEFAULT 'tmdb',
    media_type      TEXT NOT NULL,
    title           TEXT NOT NULL,
    year            INTEGER NOT NULL DEFAULT 0,
    poster_url      TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'watching', -- watching/found/downloaded/failed
    auto_rule_id    INTEGER NOT NULL DEFAULT 0,
    last_search_at  TIMESTAMP,
    search_count    INTEGER NOT NULL DEFAULT 0,
    max_searches    INTEGER NOT NULL DEFAULT 12,  -- 约 12 周
    result_source   TEXT NOT NULL DEFAULT '',
    result_account_id INTEGER NOT NULL DEFAULT 0,   -- [v5 新增] 订阅结果归属哪个网盘
    result_path     TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(external_id, external_source)
);
```

#### 0021_pansearch_cache.sql

```sql
CREATE TABLE pansearch_cache (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    keyword     TEXT NOT NULL,
    cloud_types TEXT NOT NULL DEFAULT '',         -- 逗号分隔
    results     TEXT NOT NULL,                    -- JSON 序列化的 PanSearchResult 列表
    link_count  INTEGER NOT NULL DEFAULT 0,       -- [v7 新增] 有效链接数（0 = stale）
    cached_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(keyword, cloud_types)
);
```

#### 0022_resolve_tasks.sql（v5 补字段：result_account_id, result_file_path）

```sql
CREATE TABLE resolve_tasks (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    external_id    INTEGER NOT NULL,
    external_source TEXT NOT NULL DEFAULT 'tmdb',
    media_type     TEXT NOT NULL,
    title          TEXT NOT NULL,
    year           INTEGER NOT NULL DEFAULT 0,
    season         INTEGER NOT NULL DEFAULT 0,
    episode        INTEGER NOT NULL DEFAULT 0,
    status         TEXT NOT NULL DEFAULT 'pending', -- pending/running/done/failed
    stage          TEXT NOT NULL DEFAULT '',
    stage_detail   TEXT NOT NULL DEFAULT '',
    progress_pct   INTEGER NOT NULL DEFAULT 0,
    result_source  TEXT NOT NULL DEFAULT '',
    result_file_id TEXT NOT NULL DEFAULT '',
    result_account_id INTEGER NOT NULL DEFAULT 0,   -- [v5 新增] 网盘账号 ID
    result_file_path TEXT NOT NULL DEFAULT '',      -- [v5 新增] 文件路径
    error_msg      TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_resolve_tasks_active ON resolve_tasks(external_id, external_source, season, episode)
    WHERE status IN ('pending', 'running');
```

#### 0023_external_media_servers.sql（v1.1 储备）

```sql
CREATE TABLE external_media_servers (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    server_type   TEXT NOT NULL,              -- emby / jellyfin / plex
    name          TEXT NOT NULL,
    base_url      TEXT NOT NULL,
    username      TEXT NOT NULL DEFAULT '',
    password      TEXT NOT NULL DEFAULT '',   -- AES 加密存储
    api_key       TEXT NOT NULL DEFAULT '',
    is_enabled    INTEGER NOT NULL DEFAULT 1,
    last_test_at  TIMESTAMP,
    test_status   TEXT NOT NULL DEFAULT 'untested',
    test_error    TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(server_type, base_url)
);
```

#### 0024_external_media_cache.sql（v1.1 储备）

```sql
CREATE TABLE external_media_cache (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id      INTEGER NOT NULL,
    external_id    TEXT NOT NULL,
    title          TEXT NOT NULL,
    media_type     TEXT NOT NULL,
    year           INTEGER DEFAULT 0,
    poster_url     TEXT NOT NULL DEFAULT '',
    backdrop_url   TEXT NOT NULL DEFAULT '',
    overview       TEXT NOT NULL DEFAULT '',
    rating         REAL DEFAULT 0,
    extra          TEXT NOT NULL DEFAULT '{}',
    cached_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(server_id, external_id)
);
```

#### 0025_subtitle_index.sql（v1.1 储备）

```sql
CREATE TABLE subtitle_index (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    external_id   INTEGER NOT NULL,
    external_source TEXT NOT NULL DEFAULT 'tmdb',
    media_type    TEXT NOT NULL,
    season        INTEGER NOT NULL DEFAULT 0,
    episode       INTEGER NOT NULL DEFAULT 0,
    language      TEXT NOT NULL,
    filename      TEXT NOT NULL,
    local_path    TEXT NOT NULL,
    file_size     INTEGER NOT NULL DEFAULT 0,
    source        TEXT NOT NULL,              -- opensubtitles / sibling / manual
    source_id     TEXT NOT NULL DEFAULT '',
    format        TEXT NOT NULL DEFAULT 'srt',
    rating        REAL NOT NULL DEFAULT 0,
    download_count INTEGER NOT NULL DEFAULT 0,
    created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(external_id, external_source, season, episode, language, source, source_id)
);
CREATE INDEX idx_subtitle_ext ON subtitle_index(external_id, external_source, language);
```

#### 0026_resolve_rate_limits.sql（v7 新增）

```sql
-- [v7 新增] Resolve 请求频率限制表
CREATE TABLE resolve_rate_limits (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    client_ip   TEXT NOT NULL,
    window_start TIMESTAMP NOT NULL,
    request_count INTEGER NOT NULL DEFAULT 1,
    UNIQUE(client_ip, window_start)
);
CREATE INDEX idx_rate_limits_window ON resolve_rate_limits(window_start);
```

---

## 6. 四层播放引擎

### 6.1 引擎概述

用户在详情页点击 [播放] 按钮 -> `POST /api/resolve` -> 后端创建 ResolveTask -> 四层优先级依次尝试：

| 层 | 名称 | 来源 | 目标 | 典型耗时 |
|---|---|---|---|---|
| P0 | NAS 索引秒播 | 本地 SMB 挂载盘 | 直接读文件 | <100ms |
| P1 | 盘搜转存 | PanSou 搜索 -> 分享转存 | 网盘直链 | 3-15s |
| P2 | 磁力兜底 | PanSou 搜 magnet -> 115 离线下载 | 115 网盘 | 分钟~小时 |
| P3 | 订阅等待 | 全部失败 -> 自动创建订阅 | 等待后续搜寻 | N/A |

### 6.2 Resolve 输入

```json
{
    "external_id": 19995,
    "external_source": "tmdb",
    "media_type": "movie",
    "title": "阿凡达",
    "year": 2009,
    "season": 0,
    "episode": 0
}
```

电视剧场景：`season=1, episode=3` 表示 S01E03。

### 6.3 P0 NAS 索引查询（v7 修订：智能跳过）

```sql
SELECT * FROM media_index
WHERE external_id = ?
  AND external_source = ?
  AND (season = ? OR season = 0)
  AND (episode = ? OR episode = 0)
ORDER BY season DESC, episode DESC
LIMIT 1
```

命中 -> 生成 ticket -> 返回 `/api/stream?ticket=xxx`。

**[v7 新增] P0 智能跳过逻辑**：

在执行 P0 查询前，引擎先检查 `index_status`：
- NAS 处于 Phase A/B 扫描中 -> 跳过 P0，直接进入 P1（索引不完整，查询必然 miss）
- NAS 未配置（`nas.status = "not_configured"`）-> 跳过 P0
- NAS 索引为空（`index.total_files = 0`）-> 跳过 P0
- 仅当 NAS 已配置 + 可用 + 索引完成时，才执行 P0 查询

跳过 P0 时 Resolve Modal 阶段不显示 `nas_lookup`，直接显示 `pan_searching`，避免用户看到无意义的"查询本地索引"闪过。

### 6.4 P1 盘搜 + 分享转存（v5 明确：P2 永走 magnet/ed2k）

**重要**：转存使用各驱动的 `ShareSaver` 接口（`SaveShare` 方法），不使用 crosstransfer 模块。crosstransfer 仅保留用于同盘内文件的移动/复制操作。

流程：
1. 调用 PanSearchService.Search（关键词构建见 §6.7）
2. 按用户配置的网盘优先级排序结果
3. 逐个尝试：CheckLinks 检测有效性 -> ShareSaver.SaveShare 转存 -> 索引增量 -> 解析直链 -> 返回
4. 转存成功后触发 eventbus.FileMutated 事件，索引引擎自动增量

**v5 修正**：明确 P2 磁力兜底与 P1 盘搜的关键词构建差异：
- **P1 盘搜关键词**：见 §6.7，保留季集信息（如 `阿凡达 S01E03`），用于精确定位单集
- **P2 磁力关键词**（v5 新增）：去掉季集，加 `磁力 高清` 后缀，例如：
  - P1: `权力的游戏 S01E03`
  - P2: `权力的游戏 磁力 高清`

**P2 永远走 `magnet/ed2k` cloud_type，不受用户网盘优先级影响**。这一点在 §8.3 中也有呼应。

### 6.5 P2 磁力兜底（v7 修订：后台行为明确）

1. 调用 PanSou 搜索 `cloud_types: ["magnet", "ed2k"]`
2. 选最佳磁力链接（按文件大小/seeders 排序）
3. 调用 115 驱动的 `OfflineDownloader.AddOfflineTask()`
4. 轮询下载进度 -> WebSocket 推送 `download_progress` 事件
5. 下载完成 -> 触发索引增量 -> 解析直链 -> 返回

**注意**：115 云下载非秒下载，前端需有预期管理（进度条 + 速度显示 + 取消按钮）。

**[v7 新增] P2 后台行为规范**：

| 场景 | 行为 |
|---|---|
| 用户关闭 Resolve Modal | 下载继续（后台 115 云下载不中断），ResolveTask 保持 running |
| 用户退出 Flutter App | 同上，下载继续。下次打开 App 时 WebSocket 重连后 push 最新 download_progress |
| 用户关闭并重新打开同一影片 | `POST /api/resolve` 返回 `reused=true`，前端接入已有 P2 任务，显示下载进度 |
| 下载完成后用户不在线 | ResolveTask 标记 done，写入 media_index。用户下次点击播放时 P0/P1 命中 |
| 115 离线下载本身失败 | ResolveTask 标记 failed，WS 推送 `resolve_failed`，前端下次查询时展示错误 |
| P2 下载中用户按 [取消] | 调用 `CancelOfflineTask`，ResolveTask 标记 failed，自动创建订阅（进入 P3） |

### 6.6 P3 订阅等待

全部失败 -> 自动创建 Subscription 记录 -> 返回 `not_found` + 引导订阅 -> automation scheduler 每周搜寻。

### 6.7 搜索关键词构建（v7 修订：多语言回退链）

```go
// v7: 多语言关键词回退链
func buildSearchKeywords(task *ResolveTask, media *MediaLibrary) []string {
    var keywords []string

    // 主关键词：中文标题 + 季集信息
    primary := task.Title
    if task.MediaType == "tv" && task.Season > 0 {
        if task.Episode > 0 {
            primary = fmt.Sprintf("%s S%02dE%02d", task.Title, task.Season, task.Episode)
        } else {
            primary = fmt.Sprintf("%s S%02d", task.Title, task.Season)
        }
    }
    keywords = append(keywords, primary)

    // 回退 1：原始标题（英文/罗马音）
    if media != nil && media.TitleOrig != "" && media.TitleOrig != task.Title {
        orig := media.TitleOrig
        if task.MediaType == "tv" && task.Season > 0 {
            if task.Episode > 0 {
                orig = fmt.Sprintf("%s S%02dE%02d", media.TitleOrig, task.Season, task.Episode)
            } else {
                orig = fmt.Sprintf("%s S%02d", media.TitleOrig, task.Season)
            }
        }
        keywords = append(keywords, orig)
    }

    // 回退 2：中文 + 英文混合（如 "阿凡达 Avatar"）
    if media != nil && media.TitleOrig != "" && media.TitleOrig != task.Title {
        mixed := fmt.Sprintf("%s %s", task.Title, media.TitleOrig)
        if task.MediaType == "tv" && task.Season > 0 {
            if task.Episode > 0 {
                mixed = fmt.Sprintf("%s %s S%02dE%02d", task.Title, media.TitleOrig, task.Season, task.Episode)
            } else {
                mixed = fmt.Sprintf("%s %s S%02d", task.Title, media.TitleOrig, task.Season)
            }
        }
        keywords = append(keywords, mixed)
    }

    return keywords
}
```

**搜索策略**：按 `keywords` 顺序依次尝试，找到 ≥1 个有效结果即停止。所有关键词均无结果才进入 P2 磁力兜底。

### 6.8 Ticket 与 Stream 代理机制

#### 6.8.1 Ticket 生成

Resolve 成功后，后端生成 ticket。Ticket 是一个 HMAC-SHA256 签名的 base64url 字符串：

```go
// internal/playback/ticket.go

type TicketClaims struct {
    TaskID         int64  `json:"t"`   // ResolveTask ID
    AccountID      int64  `json:"a"`   // 网盘账号 ID（NAS 为 0）
    FileID         string `json:"f"`   // 网盘文件 ID（NAS 为文件路径）
    Source         string `json:"s"`   // nas/pan115/quark/...
    ExternalID     int64  `json:"e"`   // 媒体 ID
    ExpiresAt      int64  `json:"x"`   // 过期时间戳（Unix）
}
```

Ticket 有效期：
- NAS 文件：24 小时（文件路径不变）
- 115 网盘：2 小时（115 直链有效期通常 2 小时）
- 夸克网盘：1 小时
- 其他网盘：1 小时

Ticket 签名密钥存储在 `configs` 表 key=`ticket_signing_secret`，后端启动时若不存在则随机生成。

#### 6.8.2 Stream 代理端点

前端拿到的 `stream_url` 格式永远为：
```
/api/stream?ticket={ticket_string}
```

前端将此 URL 直接喂给 fvp 播放器，不需要知道真实直链。

#### 6.8.3 Stream 代理处理流程

```
fvp 请求 /api/stream?ticket=xxx
  │
  ├── 1. 验证 ticket：HMAC 签名校验 + 过期检查
  │     └── 无效/过期 -> 401 Unauthorized
  │
  ├── 2. 从 ticket 提取 {source, account_id, file_id}
  │
  ├── 3. 根据 source 分支：
  │     ├── NAS（source=nas）：
  │     │     直接读取本地文件 -> 代理 Range 请求
  │     │     支持 Range: bytes=start-end -> 206 Partial Content
  │     │
  │     ├── 网盘（source=pan115/quark/...）：
  │     │     a. 从 media_index 查询缓存的直链 URL
  │     │     b. 直链未过期 -> 302 重定向到真实直链
  │     │     c. 直链已过期 -> 调用 driver.GetFile() 刷新直链
  │     │        -> 更新 media_index.stream_url -> 302 重定向
  │     │     d. 刷新失败 -> 502 Bad Gateway
  │     │
  │     └── 外部媒体库（source=emby/jellyfin/plex，v1.1）：
  │           调用对应适配器获取播放 URL -> 302 重定向
  │
  └── 4. 响应头设置：
        Access-Control-Allow-Origin: *
        Cache-Control: no-store
```

#### 6.8.4 安全保证

- 真实网盘直链（含 sign/token 参数）**永远不出后端进程**
- 前端只持有 ticket，ticket 中不含真实 URL
- 日志中 ticket 相关的 URL 使用 `redactURL()` 脱敏

#### 6.8.5 驱动接口扩展

在 `internal/driver/driver.go` 新增接口：

```go
// ShareSaver 支持分享链接转存的驱动实现此接口
type ShareSaver interface {
    SaveShare(ctx context.Context, req ShareRequest) (*SaveResult, error)
}

type ShareRequest struct {
    ShareURL       string // 分享链接
    Password       string // 提取码（可为空）
    TargetParentID string // 保存到网盘的指定目录 ID
}

type SaveResult struct {
    FileID    string // 转存后的文件 ID
    FileName  string // 文件名
    FileSize  int64  // 文件大小（字节）
    FileCount int    // 转存的文件数量
}

// OfflineDownloader 支持离线下载的驱动实现此接口
type OfflineDownloader interface {
    AddOfflineTask(ctx context.Context, magnetURL string) (taskID string, err error)
    GetOfflineTaskStatus(ctx context.Context, taskID string) (*OfflineTaskStatus, error)
    CancelOfflineTask(ctx context.Context, taskID string) error
}

type OfflineTaskStatus struct {
    State       string // downloading/completed/failed
    ProgressPct int
    Speed       string // "12.5 MB/s"
    FileID      string // 下载完成后的文件 ID
    FileName    string
}
```

各驱动在 `drivers/{driver}/` 下新增 `share.go` 实现 `ShareSaver` 接口。115 驱动同时实现 `OfflineDownloader` 接口。

### 6.9 转存路径与配额管理（v5 新增）

#### 6.9.1 转存根目录配置

每个网盘账号独立配置 `save_root_folder_id`：
- 默认值：首次添加账号时调用 `driver.ListRoot()`，自动创建 `X-MEDIA/` 目录，存其 FolderID
- 存储：`configs` 表 key=`pan_{driver}_save_root_{account_id}`
- Vue 管理后台：账号详情页可查看/修改"转存目标目录"

X-MEDIA 在 save_root 下按规则创建子目录：

```
X-MEDIA/
├── movies/                 按 external_id 散列分桶
│   ├── 19995-阿凡达/       单部电影独立目录
│   └── ...
├── tv/                     按剧集分目录
│   ├── 1399-权力的游戏/
│   │   ├── Season 01/      每季独立子目录
│   │   │   ├── S01E01.mkv
│   │   │   └── ...
│   │   └── Season 02/
│   └── ...
└── other/                  综艺/动漫/纪录
```

子目录命名规则由 matcher 复用文件名清洗结果。

#### 6.9.2 转存后重命名策略

`configs` 表 key=`pan_rename_enabled`（默认 true），启用时：
- 电影：`{title} ({year}).{ext}` -> 如 `阿凡达 (2009).mkv`
- 电视剧：`{title} S{season:02d}E{episode:02d}.{ext}` -> 如 `权力的游戏 S01E03.mkv`
- 综艺/动漫/纪录：电影规则 + 类型前缀

驱动调用 `driver.Rename()` 实现（115/夸克原生支持）。

#### 6.9.3 配额预警与自动清理

每个网盘账号独立配置：
- `pan_{driver}_quota_warning_gb`：剩余空间 < 该值时触发通知（默认 5GB）
- `pan_{driver}_cleanup_mode`：none / periodic / lru
- `pan_{driver}_cleanup_keep_recent_days`：periodic 模式下保留最近 N 天播放的（默认 7）

清理实现见 §9.5，调用 `driver.DeleteFile()` 删除 + eventbus.FileMutated 触发索引删除。

前端通知：WS `notification` payload：

```json
{
  "level": "warning",
  "title": "115 网盘空间不足",
  "message": "剩余3.2GB，建议清理转存内容",
  "action": "open_cleanup_settings"
}
```

#### 6.9.4 转存失败的级联处理

| 失败原因 | 后端行为 |
|---|---|
| TargetParentID 不存在 | slog.Error + 重试 1 次（重新 ListRoot 查找） |
| 容量不足 | 跳过当前源 + WS 推送 `quota_warning` + 继续下一个优先级源 |
| 分享已失效 | 标记搜索结果 invalid + 继续下一个搜索结果 |
| 驱动 API 401/403 | 触发 account_auth_failed + 跳过当前账号 |

---

## 7. TMDB 服务

### 7.1 后端代理模式

TMDB API Key 仅存在于后端 `configs` 表 + 内存，不暴露前端。Flutter 通过后端代理访问 TMDB。

### 7.2 首页 12 个榜单

| 榜单 | TMDB API | 参数 |
|---|---|---|
| 热门电影 | /trending/movie/week | - |
| 热门电视 | /trending/tv/week | - |
| 今日热播 | /trending/all/day | - |
| 即将上映 | /movie/upcoming | region=CN |
| 评分最高 | /movie/top_rated | - |
| 动作电影 | /discover/movie | with_genres=28&sort_by=popularity.desc |
| 科幻电影 | /discover/movie | with_genres=878 |
| 喜剧电影 | /discover/movie | with_genres=35 |
| 热播剧集 | /tv/popular | - |
| 评分最高剧集 | /tv/top_rated | - |
| 综艺 | /discover/tv | with_genres=10764 |
| 纪录片 | /discover/movie | with_genres=99 |

### 7.3 动漫走 Bangumi API

TMDB 中文动漫覆盖不如 Bangumi。动漫分类页使用 Bangumi 排行 API。动漫的 `external_source = "bangumi"`。

### 7.4 缓存策略

| 数据 | 存储 | TTL |
|---|---|---|
| TMDB 详情 | SQLite media_library | 7 天 |
| TMDB 榜单 | Go 内存 | 6 小时 |
| TMDB 搜索 | Go 内存 | 5 分钟 |
| 海报图片 | CDN 直连 | 永久（TMDB CDN） |

TMDB API 不可用时返回过期缓存（`extra` 字段标记 `"stale":true`）。

#### 7.4.1 media_library 清理策略（v7 新增）

`media_library` 表会随用户浏览不断增长。清理策略：

- **保留规则**：已收藏 + 已订阅 + 有播放记录 + 最近 30 天访问过的条目永久保留
- **淘汰规则**：其余条目按 `last_accessed_at` 排序，LRU 淘汰
- **触发时机**：表行数超过 5000 时自动清理至 3000 条
- **实现**：`configs` 表 key=`media_library_max_rows`（默认 5000）、`media_library_keep_rows`（默认 3000）

清理时更新 `last_accessed_at`：每次通过 `GET /api/tmdb/detail/` 访问都更新该字段。

---

## 8. 盘搜引擎（PanSearch）

### 8.1 架构：Sidecar HTTP 模式

PanSou（github.com/fish2018/pansou）是纯 Go 编写的独立 HTTP 搜索服务，支持 TG 频道搜索 + 自定义插件搜索。作为独立进程与 X-MEDIA 后端并行运行，通过 HTTP 通信。

```
┌─────────────────────────────────────────┐
│              X-MEDIA 后端                 │
│                                          │
│  internal/pansearch/                     │
│  ┌─────────────────────────────────┐    │
│  │ PanSearchService                │    │
│  │  - HTTP client -> PanSou API     │    │
│  │  - 结果解析 -> PanSearchResult   │    │
│  │  - 质量排序 + 过滤              │    │
│  │  - 本地缓存（SQLite）           │    │
│  └─────────────────────────────────┘    │
└──────────────┬───────────────────────────┘
               │ HTTP (localhost:8888)
               ▼
┌─────────────────────────────────────────┐
│         PanSou 服务（Sidecar）            │
│  - TG 频道搜索 + 插件搜索（16+ 插件）      │
│  - 二级缓存（内存+磁盘）                  │
│  - 链接有效性检测                         │
│  - 端口 8888                             │
└─────────────────────────────────────────┘
```

**为什么不内嵌为 Go 包**：PanSou 是完整服务（TG 爬虫 + 插件系统 + 二级缓存），拆包嵌入成本远大于收益，且 PanSou 独立升级更灵活。

### 8.2 PanSou API 调用

#### 搜索 API

```
POST /api/search
Content-Type: application/json

请求体：
{
    "kw": "阿凡达",                    // 搜索关键词（必填）
    "cloud_types": ["115", "quark"], // 限定网盘类型（可选）
    "res": "merge",                   // 返回格式：merge=按网盘类型分组
    "src": "all",                     // 数据来源：all/tg/plugin
    "filter": {                       // 关键词过滤（可选）
        "include": ["4K", "1080P"],
        "exclude": ["CAM", "预告"]
    }
}
```

PanSou 响应（`res=merge` 格式）：
```json
{
    "total": 15,
    "merged_by_type": {
        "quark": [
            {
                "url": "https://pan.quark.cn/s/xxxx",
                "password": "1234",
                "note": "阿凡达 4K HDR",
                "datetime": "2023-06-10T14:23:45Z",
                "source": "tg:频道名"
            }
        ],
        "115": [...],
        "baidu": [...]
    }
}
```

#### 链接检测 API

```
POST /api/check/links
Content-Type: application/json

请求体：
{
    "items": [
        {"disk_type": "quark", "url": "https://pan.quark.cn/s/xxx", "password": "1234"},
        {"disk_type": "115", "url": "https://115cdn.com/s/xxx?password=1234"}
    ]
}

响应：
{
    "results": [
        {"disk_type": "quark", "url": "...", "state": "ok", "summary": "链接有效"},
        {"disk_type": "115", "url": "...", "state": "bad", "summary": "链接失效"}
    ]
}
```

#### 健康检查 API

```
GET /api/health
响应：{"status": "ok", "auth_enabled": false, "plugin_count": 16, ...}
```

### 8.3 cloud_types 映射（v5 修正：magnet/ed2k 是 P2 专用）

X-MEDIA 驱动名 -> PanSou cloud_types：

| X-MEDIA 驱动名 | PanSou 类型 |
|---|---|
| pan115 | 115 |
| quark | quark |
| pan123 | 123 |
| baidu | baidu |
| guangya | guangya |
| (磁力兜底 P2) | magnet, ed2k（专用，P2 永走，**不受用户网盘优先级影响**） |

搜索时根据用户配置的优先级列表，只搜索已登录网盘对应的 cloud_types。

### 8.4 PanSearchService 接口

```go
// internal/pansearch/service.go

type Service struct {
    httpClient *http.Client
    baseURL    string          // 从 configs 表读取，默认 http://localhost:8888
    authToken  string          // 可选 JWT token
    store      *store.Store    // 本地缓存
    log        *slog.Logger
}

// Search 调用 PanSou /api/search，返回按网盘类型分组的结果
func (s *Service) Search(ctx context.Context, req SearchRequest) (*SearchResponse, error)

// CheckLinks 调用 PanSou /api/check/links，检测分享链接有效性
func (s *Service) CheckLinks(ctx context.Context, items []CheckItem) ([]CheckResult, error)

// Health 检测 PanSou 服务可用性
func (s *Service) Health(ctx context.Context) error
```

### 8.5 结果解析与排序（v7 修订：缓存失效机制）

PanSou 返回 `merged_by_type` 格式，X-MEDIA 解析为 `PanSearchResult` 列表：

```go
func parseMergedResults(merged map[string][]pansouLink) []PanSearchResult {
    var results []PanSearchResult
    for diskType, links := range merged {
        for _, link := range links {
            results = append(results, PanSearchResult{
                Title:     link.Note,
                Source:    mapDiskType(diskType),   // 115 -> pan115, quark -> quark
                ShareURL:  link.URL,
                Password:  link.Password,
                Datetime:  link.Datetime,
                Quality:   detectQuality(link.Note), // 从标题推断 4K/1080P/720P/CAM
                Format:    detectFormat(link.Note),  // 从标题推断 mkv/mp4
                MagnetURL: "",                       // 仅 magnet 类型有值
            })
        }
    }
    return results
}
```

质量排序器逻辑：
1. 排除 CAM（枪版），除非用户在配置中关闭过滤
2. 4K 优先 > 1080P > 720P > 未知
3. 同质量下按 datetime 降序（越新越优先）
4. 按用户配置的网盘优先级排序

#### 8.5.1 PanSou 缓存失效机制（v7 新增）

`pansearch_cache` 表新增 `link_count` 字段（§5.1 0021），记录缓存时有效链接数。

**缓存写入**：每次 PanSou 搜索成功，写入缓存时附带 `link_count = len(validResults)`。

**缓存读取失效**：
- TTL 内命中缓存 -> 返回缓存结果
- P1 转存阶段逐个尝试时，若所有 CheckLinks 返回 `bad`（有效链接数降为 0）-> 标记该缓存条目 `link_count = 0`（stale）
- 下次同一关键词搜索时：先查缓存，若 `link_count == 0` 且 `cached_at < 30 分钟前`，则**跳过缓存直接调用 PanSou 重新搜索**

**缓存写入时更新**：重新搜索成功后，UPDATE 覆盖旧缓存（含新的 `link_count`）。

### 8.6 部署方式

**docker-compose.yml**：

```yaml
services:
  xmedia:
    build: .
    ports:
      - "8080:8080"
    environment:
      - PANSOU_URL=http://pansou:8888
    depends_on:
      - pansou
    volumes:
      - /mnt/nas/media:/media/nas:ro  # NAS 只读挂载
  pansou:
    image: ghcr.io/fish2018/pansou:latest
    ports:
      - "8888"  # 仅内部网络可达，不暴露到宿主机
    # 如需代理访问 TG：
    # environment:
    #   - PROXY=socks5://host.docker.internal:1080
```

**裸机部署**：PanSou 源码编译为单二进制（`go build -o pansou .`），与 X-MEDIA 后端同机运行。

### 8.7 新增 ConfigKey

```go
const (
    ConfigPansearchURL      = "pansearch_url"          // 默认 http://localhost:8888
    ConfigPansearchAuthOn   = "pansearch_auth_enabled"  // 默认 false
    ConfigPansearchToken    = "pansearch_token"         // JWT token（可选）
    ConfigPansearchCAMBlock = "pansearch_cam_block"     // 默认 true
    ConfigPansearch4KPriority = "pansearch_4k_priority" // 默认 true
)
```

### 8.8 降级策略

X-MEDIA 后端启动时调用 `GET /api/health` 检测 PanSou 可达性。不可达时：
- 盘搜功能降级：P1 层跳过，直接进 P2 磁力兜底
- WebSocket 健康检查中标记 `pansearch: "unavailable"`
- Vue 管理后台显示 PanSou 状态红灯

---

## 9. 索引引擎

### 9.1 索引策略

| 来源 | 索引方式 | 频率 |
|---|---|---|
| NAS (LocalFs) | 首次全盘扫描 | 启动后自动 1 次 |
| NAS (LocalFs) | 增量扫描 | 每周 1 次 |
| NAS (LocalFs) | 全盘校验 | 每月 1 次（可关闭） |
| 网盘 | 事件驱动增量 | 转存/下载完成时自动 |
| 网盘 | 删除联动 | 用户删文件时自动 |

### 9.2 NAS 文件名匹配

1. 文件名清洗：去除编码组、分辨率、来源等无关信息，提取标题+年份/季集
2. TMDB 匹配：用清洗后的标题查询 media_library 表
3. 匹配阈值：score >= 0.85 -> matched，0.6-0.85 -> unconfirmed，<0.6 -> orphaned

### 9.3 网盘索引增量

监听 eventbus.FileMutated 事件：
- 文件新增（转存/下载完成）-> 写入 media_index
- 文件删除 -> 删除 media_index 对应条目

### 9.4 NAS 部署前置

NAS SMB 挂载是 OS 层操作，不由 X-MEDIA 后端负责。

**裸机部署**：管理员在 OS 层挂载 SMB 共享到本地路径
- Linux: `mount -t cifs //nas-ip/media /mnt/nas/media -o username=xxx,password=xxx,ro`
- 写入 `/etc/fstab` 实现开机自动挂载

**Docker 部署**：在 docker-compose.yml 中挂载已挂载的 SMB 路径
```yaml
volumes:
  - /mnt/nas/media:/media/nas:ro  # 只读挂载
```

**Vue 管理后台配置**：
- NAS 本地路径：文本输入框（如 `/mnt/nas/media` 或 Docker 内 `/media/nas`）
- [测试路径可读性] 按钮：后端检查路径是否存在 + 是否可读
- 配置存储在 `configs` 表 key=`nas_local_path`

**健康检查**：检查 `nas_local_path` 配置的路径是否存在 + 是否可读。状态：`ok` / `not_configured` / `not_accessible`。

### 9.5 清理策略

`media_index` 表有 `last_played_at` 字段。播放时（`/api/stream` 被请求）更新该字段。

清理逻辑：跳过最近 2 小时内播放过的文件，避免清理正在播放的内容。

```go
func (s *Service) Cleanup(ctx context.Context, accountID int64, driverName string) error {
    items, _ := s.store.MediaIndex.ListBySource(driverName, accountID)
    cutoff := time.Now().Add(-2 * time.Hour)
    var toDelete []MediaIndex
    for _, item := range items {
        if item.LastPlayedAt != nil && item.LastPlayedAt.After(cutoff) {
            continue // 正在播放或刚播放过，跳过
        }
        toDelete = append(toDelete, item)
    }
    // 删除网盘文件 + 删除索引条目
}
```

### 9.6 索引校验

索引校验默认关闭。用户可在管理后台开启后按周执行（对比 media_index 和网盘实际文件列表，发现孤儿记录自动标记删除）。

### 9.7 NAS 扫描生命周期（v5 新增）

#### 9.7.1 扫描阶段

启动 NAS 索引分三个阶段，独立可中断：

**Phase A：路径发现**（秒级）
- 递归遍历 `nas_local_path`，产出所有候选文件路径（不读文件内容）
- 仅写路径到内存队列
- 用户 HTTP 健康检查可立即收到 `nas_scan: phase=A, total_files=125000`

**Phase B：元数据提取**（分钟~小时）
- worker pool（默认 8 worker，可配 `nas_scan_worker_count`）
- 每个 worker 读取文件大小 + 文件名清洗 + 调用 media_library 查询 TMDB
- 批量写 media_index（每 1000 条一次事务）
- 进度通过 WS `index_status` 推送：

```json
{
  "type": "index_status",
  "payload": {
    "scope": "nas",
    "phase": "B",
    "processed": 45000,
    "total": 125000,
    "matched": 38000,
    "unconfirmed": 5000,
    "orphaned": 2000,
    "rate_per_sec": 230
  }
}
```

**Phase C：孤儿标记**（秒级）
- 标记 `match_status='unconfirmed'` 中超过 30 天的为 `orphaned`
- 不删除，仅状态变化

#### 9.7.2 与 HTTP 服务的并发

- Phase A/B 在独立 goroutine 池中运行，不阻塞 HTTP listener
- HTTP handler 查询 media_index 看到的是已写入的子集（增量可见）
- 用户在扫描期间点击播放：P0 查询可能 0 命中 -> 自动降级到 P1（已有逻辑）

#### 9.7.3 取消与重启恢复

- 进程收到 SIGTERM -> 当前批次完成后停止 Phase B
- 重启后从 `media_index.created_at` 最新值之后继续（增量扫描逻辑）

#### 9.7.4 增量扫描策略

每周一次（`nas_index_incremental_day`）：
- 仅扫描 `mtime > 上次扫描时间` 的文件
- 用 `find -newer` 或 `os.ReadDir` 的 `FileInfo.ModTime()`
- 删除 media_index 中对应不存在的（Phase D：孤儿清理）

#### 9.7.5 月度全盘校验

`nas_index_full_validation_day`（默认关闭）：
- 对比 media_index.file_path 与实际文件系统
- 不存在的标 orphaned（保留 30 天后清理）

---

## 10. WebSocket

### 10.1 事件类型（v7 新增：capabilities）

| 类型 | 说明 |
|---|---|
| `health_check` | 连接建立后首条消息，后端全量自检结果（含 capabilities，v7 新增） |
| `capabilities` | [v7 新增] 能力变更推送（网盘登录/退出、NAS 索引完成时主动推） |
| `resolve_stage` | Resolve 阶段推进 |
| `resolve_complete` | Resolve 成功 |
| `resolve_failed` | Resolve 失败 |
| `download_progress` | P2 磁力下载进度 |
| `subscription_ready` | 订阅找到资源 |
| `index_status` | 索引状态变更 |
| `notification` | 通用通知 |
| `account_auth_failed` | 网盘账号认证失效 |

### 10.2 认证

WebSocket URL：`/ws?token={JWT}`

### 10.3 心跳

- Flutter 每 30 秒发送 ping
- 后端 90 秒未收到 ping -> 关闭连接
- Flutter 60 秒未收到 pong -> 视为断线 -> 启动重连
- 心跳超时不在健康检查面板显示（静默重连）

---

## 11. 策略配置

### 11.1 网盘优先级配置（v5 修正：NAS 可关闭，默认开启；v7 新增配置验证）

v5 修正 §11.1 UI 图中"NAS 本地 (固定最高)"的暗示——NAS 可关闭，默认开启：

```
┌─────────────────────────────────────────────┐
│  播放优先级设置                               │
│                                               │
│  [✓] 启用 NAS 本地索引（推荐）                │
│                                               │
│  网盘拖拽排序（数字越小优先级越高）:           │
│  ┌───┐                                        │
│  │ 1 │ 115 网盘          [↑] [↓]              │
│  ├───┤                                        │
│  │ 2 │ 夸克网盘          [↑] [↓]              │
│  ├───┤                                        │
│  │ 3 │ 123 网盘          [↑] [↓]              │
│  ├───┤                                        │
│  │ 4 │ 百度网盘          [↑] [↓]              │
│  ├───┤                                        │
│  │ 5 │ 光鸭网盘          [↑] [↓]              │
│  └───┘                                        │
│                                               │
│  ⚙ 磁力兜底: [✓] 启用  下载到: [115 网盘 ▼]   │
│                                               │
│  💡 提示: 优先级决定播放引擎搜索转存的顺序。    │
│  建议将容量大、API 稳定的网盘设为高优先级。     │
│  磁力兜底仅在前述所有网盘均无资源时触发。       │
└─────────────────────────────────────────────┘
```

#### 11.1.1 配置验证（v7 新增）

保存优先级配置时，后端执行以下验证：

1. **已登录校验**：若优先级列表中出现未登录的网盘 driver，前端展示 ⚠️ 警告图标 + 提示"{网盘名} 未登录，将被自动跳过"
2. **运行时动态跳过**：Resolve 引擎在遍历优先级时，自动跳过 `account.status != "ok"` 的网盘，不等待超时
3. **配置保存触发**：优先级变更后触发 WS `capabilities` 推送，通知前端重新获取能力信息

**前端展示**：未登录的网盘在拖拽列表中灰显 + ⚠️，提示"请先在账号管理中添加"。

### 11.2 各盘索引与清理策略

每个网盘账号独立配置，hint 提示文案说明逻辑：

- **115 网盘**：`💡 115 通常容量较大(如 VIP 28TB)，建议不清理。转存的文件会持续累积，用户可在文件管理中手动删除。删除后索引自动修正。`
- **夸克网盘**：`💡 夸克免费空间通常 10GB，建议每周清理一次转存内容。清理时会删除转存目录下的所有文件并重建索引。正在播放或最近 2 小时内播放过的内容不会被清理。`

### 11.3 配置存储

所有配置存储在 `configs` 表（key-value），JSON 格式：

```json
// resolve_priority
["nas","pan115","quark","pan123","baidu","guangya"]

// pan_quark_cleanup_mode
"periodic"

// pan_quark_cleanup_days
"7"

// 每个网盘一组 key: pan_{driver}_xxx
```

---

## 12. HTTP API 契约

### 12.1 Flutter 播放器 API（v7 新增 capabilities / continue-watching / search-history）

#### TMDB 相关

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/tmdb/trending` | 首页 12 个榜单（?type=movie&window=week） |
| GET | `/api/tmdb/discover` | 分类页（?type=movie&genre=28&page=1） |
| GET | `/api/tmdb/search` | 搜索（?q=阿凡达&page=1） |
| GET | `/api/tmdb/detail/{external_id}` | 详情页（?type=movie&source=tmdb） |
| GET | `/api/tmdb/seasons/{external_id}` | 电视剧季集列表 |

#### 播放引擎

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/resolve` | 触发播放引擎 `{external_id, external_source, title, year, media_type, season, episode}` |
| GET | `/api/resolve/result/{task_id}` | 查询解析结果 |
| GET | `/api/stream` | 播放流代理 `?ticket=xxx` |
| GET | `/api/capabilities` | [v7 新增] 能力预检（无需 resolve 即可查） |

#### 媒体库

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/media/continue-watching` | [v7 新增] 继续观看列表（最近 20 条未看完的） |
| GET | `/api/media/history` | 播放历史列表 |
| POST | `/api/media/history` | 上报播放进度 `{external_id, external_source, season, episode, position_ms, duration_ms}` |
| DELETE | `/api/media/history/{external_id}` | 清除单条历史 |
| DELETE | `/api/media/history` | 清空全部历史 |
| GET | `/api/media/favorites` | 收藏列表 |
| POST | `/api/media/favorites` | 添加收藏 `{external_id, external_source, media_type, title, year}` |
| DELETE | `/api/media/favorites/{external_id}` | 取消收藏 |
| GET | `/api/media/subscriptions` | 订阅列表 |
| POST | `/api/media/subscriptions` | 添加订阅 `{external_id, external_source, media_type, title, year}` |
| DELETE | `/api/media/subscriptions/{external_id}` | 取消订阅 |
| GET | `/api/media/search-history` | [v7 新增] 搜索历史（最近 20 条） |
| DELETE | `/api/media/search-history` | [v7 新增] 清空搜索历史 |

#### 系统

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/auth/login` | 登录 `{password}` -> 返回 JWT |
| GET | `/api/state/snapshot` | 系统状态快照 |
| GET | `/ws` | WebSocket 升级端点（?token=JWT） |
| GET | `/api/health` | 健康检查（无需认证） |

### 12.2 Vue 管理后台 API

复用 LitePan 现有 API，新增：

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/admin/index/status` | 索引引擎状态 |
| POST | `/api/admin/index/nas/full` | 触发 NAS 全盘索引 |
| POST | `/api/admin/index/nas/incremental` | 触发 NAS 增量索引 |
| POST | `/api/admin/index/rebuild/{account_id}` | 重建指定网盘索引 |
| GET | `/api/admin/resolve/config` | 获取播放引擎配置 |
| PUT | `/api/admin/resolve/config` | 更新播放引擎配置 |
| GET | `/api/admin/tmdb/config` | 获取 TMDB 配置 |
| PUT | `/api/admin/tmdb/config` | 更新 TMDB 配置 |
| POST | `/api/admin/tmdb/test` | 测试 TMDB API Key |
| GET | `/api/admin/pansearch/config` | 获取盘搜服务配置 |
| PUT | `/api/admin/pansearch/config` | 更新盘搜服务配置 |
| POST | `/api/admin/pansearch/test` | 测试 PanSou 连通性 |

---

## 13. 模块裁剪清单

### 13.1 砍掉的模块

| 模块 | 路径 | 行数(估) | 理由 |
|---|---|---|---|
| FUSE 挂载 | `internal/fusemount` | ~1500 | fvp 直播直链，不挂本地磁盘 |
| FUSE 读缓存 | `internal/fusereadcache` | ~800 | 同上 |
| FNOS 代理 | `internal/fnosproxy` | ~300 | 飞牛 OS 专属 |
| STRM 生成 | `internal/strm` | ~1000 | 不用 STRM 文件 |
| STRM 刮削 | `internal/strmscrape` | ~800 | 不刮削 |
| Media Organize | `internal/mediaorganize` | ~1200 | TMDB 驱动 |
| 文件分享 | `internal/share` | ~500 | 不需要 |
| 收藏(旧) | `internal/favorites` | ~300 | 移入 media 模块 |
| 多余驱动 | `drivers/WebDAV`, `OneDrive`, `template` | ~2000 | 只留 5 网盘+LocalFs |
| API Key | `internal/apikey` | ~400 | 简化为单 admin token |
| 115 扫码移植 | 旧项目 QR 登录逻辑 | ~800 | 采用 LitePan 115_Open OAuth 认证，不需要扫码 |

### 13.2 保留的模块

| 模块 | 路径 | 理由 |
|---|---|---|
| account / auth | 网盘账号管理 + 认证状态 | 核心 |
| api | HTTP 路由 | 核心 |
| app | 应用装配（含 §28 启动序） | 核心 |
| automation | 订阅调度器（改造 trigger） | 复用 scheduler |
| cache | 内存缓存（目录列表/直链） | 减少扫盘 |
| cacheretention | 目录列表缓存保鲜 | 减少扫盘，防风控 |
| config / settings | 配置管理 | 核心 |
| core | 驱动执行器 | 核心 |
| crosstransfer | 跨盘转存 | 仅同盘文件移动/复制场景，非分享转存 |
| domain | 领域模型 | 核心 |
| driver | 驱动接口 | 核心 |
| eventbus | 事件总线 | WebSocket 数据源 |
| file | 文件列表服务 | 核心 |
| offlinedownload | 离线下载 | 115 云下载兜底 |
| playback | 播放供流 + Ticket 代理 | 核心 |
| store | SQLite | 核心 |
| upload | 上传 | 跨盘转存依赖 |
| notification | 通知 | 订阅完成通知 |

### 13.3 新增的模块

| 模块 | 路径 | 说明 |
|---|---|---|
| tmdb | `internal/tmdb` | TMDB API 代理 + 缓存 |
| pansearch | `internal/pansearch` | PanSou HTTP 客户端 + 结果解析 |
| media | `internal/media` | 媒体库（历史/收藏/订阅/继续观看/搜索历史） |
| indexengine | `internal/indexengine` | 索引引擎（NAS + 网盘） |
| resolve | `internal/resolve` | 四层播放引擎（含并发限流，v7 新增） |
| websocket | `internal/websocket` | WebSocket 状态推送 |
| externalmedia | `internal/externalmedia` | 外部媒体库集成（v1.1 储备） |
| subtitle | `internal/subtitle` | 字幕搜索/下载/索引（v1.1 储备） |

---

## 14. 旧项目排坑清单

### 14.1 架构层面

| 坑 | 旧项目教训 | 新项目对策 |
|---|---|---|
| gRPC Stream 闪断 | ControlStream 依赖 HTTP context，handler 返回后 goroutine 被杀 | 改用 HTTP + WebSocket，WebSocket 断线自动重连 |
| 边车强耦合 | OpenList/AList 进程死亡导致 X-Media 瘫痪 | PanSou 不可用时降级跳过 P1，不瘫痪 |
| gRPC 契约缺口 | MediaService 无 ListMedia RPC，前端无法获取数据 | HTTP API 自由定义，无 proto 约束 |
| 跨盘秒传不存在 | 夸克->115 跨网盘秒传在物理上无通用方案 | 只做同盘分享转存，不跨盘 |

### 14.2 网盘协议层面

| 坑 | 旧项目教训 | 新项目对策 |
|---|---|---|
| 扫码 context 早夭 | `PollQRConfirm(r.Context())` handler 返回后 context 取消 | Quark 驱动已有成熟扫码，用 opaque token 无服务端状态 |
| 扫码并发安全 | 裸 `map[string]*QRResult` 多 goroutine 写 race | LitePan Quark 驱动用 opaque token（客户端持有），无服务端状态 |

（115 扫码相关问题已删除——采用 OAuth 认证，不存在扫码问题。）

### 14.3 前端层面

| 坑 | 旧项目教训 | 新项目对策 |
|---|---|---|
| libmpv DV 色彩 | 反复修改无法解决 Dolby Vision 色彩问题 | 改用 fvp (mdk-sdk) |
| TV 焦点混乱 | Flutter 默认为触摸设计，遥控器焦点乱跳 | 第一天起封装 `TvButton`，全局焦点管理（详见 §17.x） |
| go:embed 缓存 | 重建容器后浏览器显示旧版 admin.html | Vue Web 管理后台独立部署，不 embed |

### 14.4 工程纪律层面

| 坑 | 旧项目教训 | 新项目对策 |
|---|---|---|
| 按了葫芦起了瓢 | 每次修补引入新 bug，最终失去信心 | 砍掉不需要的模块，减少代码量，降低复杂度 |
| 表象修补 | 跑通特例 ≠ 完成设计意图 | 设计文档先行，AI 编码时按文档执行 |
| 降级红线 | 遇到问题就降级实现，项目成妥协产物 | 物理限制（如跨盘秒传）分层设计，不妥协核心体验 |

---

## 15. 开发路线图（v5 修正工期预估：60-80 天；v7 微调）

**v5 修正**：原 v4 预估 30-40 天偏乐观。v5 拆 Phase 7 为 7a/7b（驱动接口 vs 驱动实现），拆 Phase 9 为 9a/9b/9c（基础 vs TV 焦点 vs 播放器），加入真实工程问题（NAS 扫描、启动序、转存路径）的编码时间，调整为 **60-80 天**。

**v7 微调**：新增 v7 UX/TECH 任务（继续观看 API、搜索页、能力预检、骨架屏、多语言搜索、并发限流、Design Token 等），Phase 1/2/5/9a/9c 各增加 0.5-1 天。

### Phase 0: 基础裁剪（1-2 天）

- [ ] clone LitePan 到 `D:\CodelfWorkspace\x-media`
- [ ] 删除砍掉的模块（§13.1）
- [ ] 删除多余驱动，只注册 6 个
- [ ] 改包名 `litepan` -> `xmedia`
- [ ] 确认 `go build` 通过
- [ ] 确认现有 Vue 管理后台能正常启动

### Phase 1: 数据库 + 核心模型（v7：2-3 天）

- [ ] 新增 migration 0016-0026（§5，含 v7 新增 0026 rate_limits + media_library last_accessed_at 字段 + pansearch_cache link_count 字段）
- [ ] 新增 domain 模型（含 v5 补字段 + v7 季集维度 play_history）
- [ ] 新增 store 层 Repository
- [ ] [v7 新增] media_library LRU 清理逻辑

### Phase 2: TMDB 代理（v7：1-2 天）

- [ ] 实现 `internal/tmdb` Service
- [ ] 实现 TMDB API 代理端点
- [ ] 实现 Bangumi API 集成
- [ ] [v7 新增] 搜索历史存储/查询端点

### Phase 3: 索引引擎（v5：2-3 天）

- [ ] 实现 `internal/indexengine` Service
- [ ] NAS 三阶段扫描（§9.7）：Phase A 路径发现、Phase B 元数据提取（worker pool）、Phase C 孤儿标记
- [ ] 网盘增量索引（监听 eventbus）
- [ ] 索引查询接口

### Phase 4: 盘搜引擎（v7：2-3 天）

- [ ] 编写 `internal/pansearch` HTTP 客户端
- [ ] 实现 SearchRequest -> PanSou API -> PanSearchResult 解析链
- [ ] 实现 CheckLinks 集成
- [ ] 实现质量排序 + 过滤
- [ ] 实现本地缓存（SQLite，1 小时 TTL）
- [ ] [v7 新增] 缓存失效机制（link_count 跟踪 + stale 重搜）
- [ ] docker-compose 集成 PanSou 服务

### Phase 5: 四层播放引擎（v7：3-4 天）

- [ ] 实现 `internal/resolve` Service
- [ ] P0 NAS 索引查询（含 v7 智能跳过逻辑 §6.3）
- [ ] P1 盘搜 + ShareSaver 转存（含 §6.9 转存路径 + 配额预警 + v7 多语言关键词回退 §6.7）
- [ ] P2 磁力兜底（关键词去季集 + "磁力 高清" 后缀，§6.4）+ 115 离线下载（含 v7 后台行为 §6.5）
- [ ] Ticket 生成 + Stream 代理
- [ ] ResolveTask 持久化 + 启动恢复（§28.2）
- [ ] [v7 新增] 并发限流（每 30s 最多 3 个 resolve 请求）

### Phase 6: WebSocket + 订阅（v5：1-2 天）

- [ ] 实现 `internal/websocket` Hub
- [ ] Resolve 阶段推送
- [ ] 健康检查首条消息（含 server_started_at + v7 capabilities，§28.3）
- [ ] 订阅调度（复用 automation scheduler）
- [ ] [v7 新增] capabilities 变更推送（网盘登录/退出、NAS 索引完成时）

### Phase 7: 驱动 ShareSaver 实现（v5 拆分为 7a/7b）

#### Phase 7a: 接口定义 + 115/夸克实现（2-3 天）

- [ ] `internal/driver/driver.go` 定义 ShareSaver/OfflineDownloader 接口
- [ ] 115 驱动 `share.go` 实现 ShareSaver
- [ ] 115 驱动 `offline.go` 实现 OfflineDownloader
- [ ] Quark 驱动 `share.go` 实现 ShareSaver
- [ ] 单元测试覆盖接口签名

#### Phase 7b: 123/百度/光鸭 ShareSaver 实现（2-3 天）

- [ ] 123 驱动 `share.go`
- [ ] 百度驱动 `share.go`
- [ ] 光鸭驱动 `share.go`
- [ ] 各驱动 Rename 实现（§6.9.2）

### Phase 8: Vue 管理后台扩展（v5：3-4 天）

- [ ] 网盘优先级配置页（含 NAS 可关闭开关 + v7 已登录校验提示，§11.1）
- [ ] 各盘索引策略配置页（含配额预警阈值，§6.9.3）
- [ ] 索引状态面板（含 NAS 扫描进度，§9.7.1）
- [ ] TMDB 配置页 + API Key 测试
- [ ] 盘搜服务配置页 + 连通性测试
- [ ] NAS 路径配置页 + 可读性测试
- [ ] 健康检查面板（含 §27.4 状态映射按钮）
- [ ] 转存路径/配额配置页（§6.9.1）

### Phase 9: Flutter 前端（v5 拆分为 9a/9b/9c；v7 调整）

#### Phase 9a: 基础脚手架（v7：9-11 天）

- [ ] 项目脚手架 + 路由
- [ ] 探索页（12 榜单 + [v7 新增] 顶部「继续观看」行 + 骨架屏加载状态）
- [ ] [v7 新增] 搜索页（TMDB 搜索 + 搜索历史 + 无结果引导）
- [ ] 分类页（5 Tab 瀑布流 + 骨架屏）
- [ ] 详情页 + 电视剧季集下钻（[v7 新增] 已索引集绿色✓角标）
- [ ] 历史页 / 订阅页
- [ ] 设置页
- [ ] [v7 新增] Design Token 应用（附录 C 变量到所有组件）

#### Phase 9b: TV 焦点组件库（4-5 天）

- [ ] 焦点归属栈（§17.x.1）
- [ ] 焦点丢失恢复机制（§17.x.2）
- [ ] 弹窗与通知的焦点策略（§17.x.3）
- [ ] 长按与确认键标准（§17.x.4）
- [ ] 焦点可视化（§17.x.5）

#### Phase 9c: fvp 播放器 + WebSocket + 健康检查（v7：8-10 天）

- [ ] fvp 播放器集成
- [ ] 播放加载弹窗（[v7 新增] 分层进度指示器）
- [ ] WebSocket 客户端 + 指数退避重连 + HTTP 补刷
- [ ] 健康检查面板（§27.4 状态映射）
- [ ] 启动健康状态显示（§1.4 Day 1 Step 3）
- [ ] [v7 新增] P2 后台下载状态恢复（App 重启后接入已有 P2 任务）

### Phase 10: 集成测试 + E2E（3-5 天）

- [ ] 端到端播放流程测试（P0/P1/P2 全路径）
- [ ] NAS 索引 + 播放测试
- [ ] 盘搜 + 转存 + 播放测试
- [ ] 订阅全流程测试
- [ ] 启动健康检查测试
- [ ] TV 遥控器操作测试
- [ ] Day 1 启动旅程端到端（§1.4）
- [ ] [v7 新增] 继续观看流程测试
- [ ] [v7 新增] 搜索页全流程
- [ ] [v7 新增] 多语言搜索回退测试
- [ ] [v7 新增] 并发限流测试

### Phase 拆分总表（v7）

| Phase | 任务 | 天数（v7） |
|---|---|---|
| 0 | 基础裁剪 | 2-3 |
| 1 | 数据库 + 核心模型（含 v7 rate_limits/media_library 清理） | 3-4 |
| 2 | TMDB 代理（含搜索历史） | 1-2 |
| 3 | 索引引擎（含 §9.7 NAS 扫描生命周期） | 4-5 |
| 4 | 盘搜引擎（含 v7 缓存失效） | 3-4 |
| 5 | 四层播放引擎（含 v7 多语言搜索/P0跳过/并发限流/P2后台） | 5-6 |
| 6 | WebSocket + 订阅（含 v7 capabilities 推送） | 2-3 |
| 7a | 驱动 ShareSaver/OfflineDownloader 接口 + 115/夸克实现 | 3-4 |
| 7b | 123/百度/光鸭 ShareSaver 实现 | 2-3 |
| 8 | Vue 管理后台扩展（含 §6.9/§27.4 配置） | 3-4 |
| 9a | Flutter 基础脚手架 + 继续观看/搜索页/骨架屏/Design Token | 9-11 |
| 9b | TV 焦点组件库 + 遥控器适配（§17.x） | 4-5 |
| 9c | fvp 播放器 + WebSocket + 健康检查 + P2 恢复 | 8-10 |
| 10 | 集成测试 + E2E | 4-6 |
| **合计** | | **53-70 天**（不含 bug 修复 + 联调缓冲 15-20 天） |

**预计总工期**：70-90 天（v1.0 核心）。含 v1.1 储备（外部媒体库 + 字幕）：90-110 天。

### 关键风险与缓冲

| 风险 | 影响 | 缓冲策略 |
|---|---|---|
| 115/夸克 OpenAPI 变更 | 7a/7b 延期 | 接口层封装 + mock 测试，必要时降级为只支持已验证 API |
| fvp 在 TV 上的稳定性 | 9c 延期 | 9c 提前预研（与 9a 并行）；准备 fallback 到 video_player |
| PanSou 插件失效 | 4 延期 | PanSou 二级缓存兜底；用户可手填 PanSou 镜像 URL |
| TMDB 配额 | 2 延期 | L2 缓存 7 天 TTL 减少 API 调用；批量请求合并 |
| 家庭 NAS 兼容性 | 3 延期 | 假设 SMB 挂载可用，其他协议（iSCSI/NFS）v1.1 处理 |

---

## 16. 详细数据结构规范

### 16.1 Go 后端领域模型（internal/domain/）

#### 16.1.1 MediaIndex（文件索引条目）

```go
// domain/media_index.go

type MatchStatus string

const (
    MatchMatched     MatchStatus = "matched"
    MatchUnconfirmed MatchStatus = "unconfirmed"
    MatchOrphaned    MatchStatus = "orphaned"
)

type MediaIndex struct {
    ID              int64       `json:"id"`
    ExternalID      int64       `json:"external_id"`           // TMDB ID 或 Bangumi ID
    ExternalSource  string      `json:"external_source"`       // tmdb / bangumi
    Season          int         `json:"season"`                // 季号，0=电影/整季
    Episode         int         `json:"episode"`               // 集号，0=电影/整季
    MediaType       string      `json:"media_type"`            // movie/tv/anime/variety/documentary
    Title           string      `json:"title"`
    OriginalName    string      `json:"original_name"`
    Year            int         `json:"year"`
    SourceType      string      `json:"source_type"`           // nas/pan115/quark/pan123/baidu/guangya
    AccountID       int64       `json:"account_id"`            // NAS 时为 0
    FilePath        string      `json:"file_path"`
    FileID          string      `json:"file_id"`               // 网盘文件 ID（NAS 为空）
    FileSize        int64       `json:"file_size"`
    FileFormat      string      `json:"file_format"`           // mkv/mp4/ts/avi
    MatchStatus     MatchStatus `json:"match_status"`
    MatchScore      float64     `json:"match_score"`           // 0.0-1.0
    StreamURL       string      `json:"stream_url,omitempty"`
    URExpires       time.Time   `json:"url_expires,omitempty"`
    LastPlayedAt    *time.Time  `json:"last_played_at,omitempty"`
    CreatedAt       time.Time   `json:"created_at"`
    UpdatedAt       time.Time   `json:"updated_at"`
}
```

#### 16.1.2 MediaLibrary（TMDB 元数据缓存，v7 增 last_accessed_at）

```go
// domain/media_library.go

type MediaLibrary struct {
    ID              int64           `json:"id"`
    ExternalID      int64           `json:"external_id"`
    ExternalSource  string          `json:"external_source"`       // tmdb / bangumi
    MediaType       string          `json:"media_type"`
    Title           string          `json:"title"`
    TitleOrig       string          `json:"title_orig"`
    PosterURL       string          `json:"poster_url"`
    BackdropURL     string          `json:"backdrop_url"`
    Overview        string          `json:"overview"`
    Year            int             `json:"year"`
    VoteAvg         float64         `json:"vote_avg"`
    VoteCount       int             `json:"vote_count"`
    Genres          json.RawMessage `json:"genres"`               // JSON: [{"id":28,"name":"动作"},...]
    Runtime         int             `json:"runtime"`
    Seasons         int             `json:"seasons"`
    Episodes        int             `json:"episodes"`
    SeasonsJSON     json.RawMessage `json:"seasons_json"`         // JSON: 季集结构
    Cast            json.RawMessage `json:"cast"`                 // JSON: [{"name":"演员","character":"角色","profile_url":"..."},...]
    Extra           json.RawMessage `json:"extra"`                // JSON: 扩展字段 (含 stale 标记)
    CachedAt        time.Time       `json:"cached_at"`
    LastAccessedAt  *time.Time      `json:"last_accessed_at"`     // [v7 新增] LRU 淘汰用
}
```

#### 16.1.3 PlayHistory（播放历史，v7 增 season/episode）

```go
// domain/play_history.go

type PlayHistory struct {
    ID              int64     `json:"id"`
    ExternalID      int64     `json:"external_id"`
    ExternalSource  string    `json:"external_source"`
    MediaType       string    `json:"media_type"`
    Title           string    `json:"title"`
    PosterURL       string    `json:"poster_url"`
    SourceType      string    `json:"source_type"`
    Season          int       `json:"season"`          // [v7 新增]
    Episode         int       `json:"episode"`         // [v7 新增]
    PositionMs      int64     `json:"position_ms"`
    DurationMs      int64     `json:"duration_ms"`
    PlayedAt        time.Time `json:"played_at"`
}
```

#### 16.1.4 Favorite（收藏）

```go
// domain/favorite.go

type Favorite struct {
    ID              int64     `json:"id"`
    ExternalID      int64     `json:"external_id"`
    ExternalSource  string    `json:"external_source"`
    MediaType       string    `json:"media_type"`
    Title           string    `json:"title"`
    PosterURL       string    `json:"poster_url"`
    Year            int       `json:"year"`
    CreatedAt       time.Time `json:"created_at"`
}
```

#### 16.1.5 Subscription（订阅）+ 状态机（v5 补 result_account_id）

```go
// domain/subscription.go

type SubStatus string

const (
    SubWatching   SubStatus = "watching"
    SubFound      SubStatus = "found"
    SubDownloaded SubStatus = "downloaded"
    SubFailed     SubStatus = "failed"
)

var subTransitions = map[SubStatus][]SubStatus{
    SubWatching:   {SubFound, SubFailed},
    SubFound:      {SubDownloaded, SubFailed, SubWatching},
    SubDownloaded: {},
    SubFailed:     {SubWatching},
}

type Subscription struct {
    ID              int64      `json:"id"`
    ExternalID      int64      `json:"external_id"`
    ExternalSource  string     `json:"external_source"`
    MediaType       string     `json:"media_type"`
    Title           string     `json:"title"`
    Year            int        `json:"year"`
    PosterURL       string     `json:"poster_url"`
    Status          SubStatus  `json:"status"`
    AutoRuleID      int64      `json:"auto_rule_id"`
    LastSearchAt    *time.Time `json:"last_search_at"`
    SearchCount     int        `json:"search_count"`
    MaxSearches     int        `json:"max_searches"`
    ResultSource    string     `json:"result_source"`
    ResultAccountID int64      `json:"result_account_id"`     // [v5 新增] 结果归属哪个网盘
    ResultPath      string     `json:"result_path"`
    CreatedAt       time.Time  `json:"created_at"`
    UpdatedAt       time.Time  `json:"updated_at"`
}

func (s *Subscription) CanTransitionTo(target SubStatus) bool {
    for _, t := range subTransitions[s.Status] {
        if t == target { return true }
    }
    return false
}
```

#### 16.1.6 ResolveTask（播放解析任务，v5 补 result_account_id/result_file_path）

```go
// domain/resolve_task.go

type ResolveStage string

const (
    StageResolveStart   ResolveStage = "resolve_start"
    StageNASLookup      ResolveStage = "nas_lookup"
    StageNASHit         ResolveStage = "nas_hit"
    StagePanSearching   ResolveStage = "pan_searching"
    StagePanSearched    ResolveStage = "pan_searched"
    StageTransferring   ResolveStage = "transferring"
    StageResolvingLink  ResolveStage = "resolving_link"
    StagePlayReady      ResolveStage = "play_ready"
    StageMagnetDownload ResolveStage = "magnet_downloading"
    StageNotFound       ResolveStage = "not_found"
    StageError          ResolveStage = "error"
)

type ResolveTask struct {
    ID              int64        `json:"id"`
    ExternalID      int64        `json:"external_id"`
    ExternalSource  string       `json:"external_source"`
    MediaType       string       `json:"media_type"`
    Title           string       `json:"title"`
    Year            int          `json:"year"`            // int，非 string
    Season          int          `json:"season"`
    Episode         int          `json:"episode"`
    Status          string       `json:"status"`          // pending/running/done/failed
    Stage           ResolveStage `json:"stage"`
    StageDetail     string       `json:"stage_detail"`
    ProgressPct     int          `json:"progress_pct"`
    ResultSource    string       `json:"result_source"`
    ResultFileID    string       `json:"result_file_id"`  // string，非 int64（网盘文件 ID 是字符串）
    ResultAccountID int64        `json:"result_account_id"` // [v5 新增]
    ResultFilePath  string       `json:"result_file_path"`  // [v5 新增]
    ErrorMsg        string       `json:"error_msg"`
    CreatedAt       time.Time    `json:"created_at"`
    UpdatedAt       time.Time    `json:"updated_at"`
}
```

#### 16.1.7 PanSearchResult（盘搜结果）

```go
// domain/pansearch_result.go

type PanSearchResult struct {
    Title     string  `json:"title"`
    Source    string  `json:"source"`        // pan115/quark/pan123/baidu/guangya/magnet
    ShareURL  string  `json:"share_url"`     // 分享链接
    Password  string  `json:"password"`      // 提取码
    MagnetURL string  `json:"magnet_url"`    // 磁力链接（仅 source=magnet 时有值）
    Datetime  string  `json:"datetime"`      // 发布时间（用于排序）
    Quality   string  `json:"quality"`       // 4K/1080P/720P/CAM
    Format    string  `json:"format"`        // mkv/mp4
    Score     float64 `json:"score"`         // 综合评分 0-100
}
```

#### 16.1.8 ConfigKey 常量（v5 增：save_root / rename / quota；v7 增：rate_limit / media_library）

```go
// domain/config_keys.go

const (
    ConfigTMDBAPIKey           = "tmdb_api_key"
    ConfigTMDBLanguage         = "tmdb_language"           // 默认 zh-CN
    ConfigResolvePriority      = "resolve_priority"        // JSON: ["nas","pan115",...]
    ConfigResolveMagnetEnabled = "resolve_magnet_enabled"
    ConfigResolveMagnetTarget  = "resolve_magnet_target"   // pan115
    ConfigPansearchURL         = "pansearch_url"           // 默认 http://localhost:8888
    ConfigPansearchAuthOn      = "pansearch_auth_enabled"
    ConfigPansearchToken       = "pansearch_token"
    ConfigPansearchCAMBlock    = "pansearch_cam_block"
    ConfigPansearch4KPriority  = "pansearch_4k_priority"
    ConfigNASLocalPath         = "nas_local_path"
    ConfigNASFullScanDay       = "nas_index_full_scan_day"
    ConfigNASIncrementalDay    = "nas_index_incremental_day"
    ConfigWebSocketEnabled     = "websocket_enabled"
    ConfigBangumiAPIBase       = "bangumi_api_base"
    ConfigTicketSigningSecret  = "ticket_signing_secret"   // HMAC 签名密钥
    ConfigPanRenameEnabled     = "pan_rename_enabled"      // [v5 新增] 转存重命名开关
    ConfigNASEnabled           = "nas_enabled"             // [v5 新增] NAS 启用开关
    ConfigResolveRateLimitMax  = "resolve_rate_limit_max"  // [v7 新增] 每窗口最大请求数（默认 3）
    ConfigResolveRateLimitSec  = "resolve_rate_limit_sec"  // [v7 新增] 限流窗口秒数（默认 30）
    ConfigMediaLibraryMaxRows  = "media_library_max_rows"  // [v7 新增] LRU 淘汰阈值（默认 5000）
    ConfigMediaLibraryKeepRows = "media_library_keep_rows" // [v7 新增] LRU 保留行数（默认 3000）
    // 各盘清理策略：pan_{driver}_cleanup_mode, pan_{driver}_cleanup_days, pan_{driver}_space_warning_gb
    // 各盘转存目录：pan_{driver}_save_root_{account_id}
)

var ConfigDefaults = map[string]string{
    ConfigTMDBLanguage:           "zh-CN",
    ConfigResolvePriority:        `["nas","pan115","quark","pan123","baidu","guangya"]`,
    ConfigResolveMagnetEnabled:   "true",
    ConfigResolveMagnetTarget:    "pan115",
    ConfigPansearchURL:           "http://localhost:8888",
    ConfigPansearchAuthOn:        "false",
    ConfigPansearchCAMBlock:      "true",
    ConfigPansearch4KPriority:    "true",
    ConfigNASFullScanDay:         "1",
    ConfigNASIncrementalDay:      "7",
    ConfigWebSocketEnabled:       "true",
    ConfigPanRenameEnabled:       "true",                  // [v5 新增]
    ConfigNASEnabled:             "true",                  // [v5 新增]
    ConfigResolveRateLimitMax:    "3",                     // [v7 新增]
    ConfigResolveRateLimitSec:    "30",                    // [v7 新增]
    ConfigMediaLibraryMaxRows:    "5000",                  // [v7 新增]
    ConfigMediaLibraryKeepRows:   "3000",                  // [v7 新增]
}
```

### 16.2 Flutter 前端数据模型（lib/models/）

（本节与 v6 一致，仅增 v7 相关字段注释。核心变更：ResolveTaskState 增 season/episode、PlayHistory 增 season/episode、新增 ContinueWatchingItem/SearchHistoryItem 模型。）

```dart
// lib/models/resolve_task.dart

enum ResolveStage {
  resolveStart,
  nasLookup,
  nasHit,
  panSearching,
  panSearched,
  transferring,
  resolvingLink,
  playReady,
  magnetDownloading,
  notFound,
  error;

  factory ResolveStage.fromString(String s) {
    return ResolveStage.values.firstWhere(
      (e) => e.name == s,
      orElse: () => ResolveStage.error,
    );
  }

  String get displayText {
    switch (this) {
      case ResolveStage.resolveStart: return '准备中...';
      case ResolveStage.nasLookup: return '查询本地索引...';
      case ResolveStage.nasHit: return '找到本地文件';
      case ResolveStage.panSearching: return '搜索全网盘资源...';
      case ResolveStage.panSearched: return '分析搜索结果...';
      case ResolveStage.transferring: return '转存中...';
      case ResolveStage.resolvingLink: return '获取播放链接...';
      case ResolveStage.playReady: return '播放就绪';
      case ResolveStage.magnetDownloading: return '云下载中...';
      case ResolveStage.notFound: return '暂无可用资源';
      case ResolveStage.error: return '出错了';
    }
  }
}

class ResolveTaskState {
  final int taskId;
  final ResolveStage stage;
  final String stageDetail;
  final int progressPct;
  final String? resultTitle;
  final String? resultSource;
  final String? streamUrl;    // 内部代理 URL: /api/stream?ticket=xxx
  final String? errorMsg;
  final bool reused;

  const ResolveTaskState({
    required this.taskId,
    required this.stage,
    this.stageDetail = '',
    this.progressPct = 0,
    this.resultTitle,
    this.resultSource,
    this.streamUrl,
    this.errorMsg,
    this.reused = false,
  });

  factory ResolveTaskState.fromResolve(Map<String, dynamic> json) {
    return ResolveTaskState(
      taskId: json['task_id'] as int? ?? 0,
      stage: ResolveStage.resolveStart,
      stageDetail: '正在创建任务...',
      reused: json['reused'] == true,
    );
  }

  factory ResolveTaskState.fromWebSocket(Map<String, dynamic> json) {
    final payload = json['payload'] as Map<String, dynamic>? ?? {};
    return ResolveTaskState(
      taskId: 0,
      stage: ResolveStage.fromString(payload['stage'] as String? ?? ''),
      stageDetail: payload['detail'] as String? ?? '',
      progressPct: payload['progress_pct'] as int? ?? 0,
    );
  }

  factory ResolveTaskState.fromComplete(Map<String, dynamic> json) {
    final payload = json['payload'] as Map<String, dynamic>? ?? {};
    return ResolveTaskState(
      taskId: 0,
      stage: ResolveStage.playReady,
      streamUrl: payload['stream_url'] as String?,  // /api/stream?ticket=xxx
      resultTitle: payload['file_name'] as String?,
      resultSource: payload['source'] as String?,
    );
  }

  factory ResolveTaskState.fromFailed(Map<String, dynamic> json) {
    final payload = json['payload'] as Map<String, dynamic>? ?? {};
    return ResolveTaskState(
      taskId: 0,
      stage: ResolveStage.error,
      errorMsg: payload['reason'] as String? ?? '未知错误',
    );
  }

  bool get isTerminal =>
      stage == ResolveStage.playReady ||
      stage == ResolveStage.notFound ||
      stage == ResolveStage.error;

  bool get isSuccess => stage == ResolveStage.playReady;
}
```

```dart
// lib/models/play_result.dart

class PlayResult {
  final String streamUrl;       // 内部代理 URL: /api/stream?ticket=xxx
  final String? ticket;
  final String fileId;          // string，非 int
  final String? source;
  final String? title;
  final int? transferId;
  final int? size;
  final List<SubtitleInfo> subtitles;
  final bool cached;

  const PlayResult({
    required this.streamUrl,
    this.ticket,
    required this.fileId,
    this.source,
    this.title,
    this.transferId,
    this.size,
    this.subtitles = const [],
    this.cached = false,
  });

  factory PlayResult.fromJson(Map<String, dynamic> json) {
    return PlayResult(
      streamUrl: json['stream_url'] as String? ?? '',
      ticket: json['ticket'] as String?,
      fileId: json['file_id'] as String? ?? '',
      source: json['source'] as String?,
      title: json['title'] as String?,
      transferId: json['transfer_id'] as int?,
      size: json['size'] as int?,
      cached: json['cached'] == true,
    );
  }
}

class SubtitleInfo {
  final String name;
  final String url;

  const SubtitleInfo({required this.name, required this.url});

  factory SubtitleInfo.fromJson(Map<String, dynamic> json) {
    return SubtitleInfo(
      name: json['name'] as String? ?? '',
      url: json['url'] as String? ?? '',
    );
  }
}
```

```dart
// lib/models/subscription.dart

enum SubStatus { watching, found, downloaded, failed }

class SubscriptionItem {
  final int id;
  final int externalId;
  final String externalSource;
  final String mediaType;
  final String title;
  final int year;
  final String? posterPath;
  final SubStatus status;
  final int searchCount;
  final int maxSearches;
  final DateTime createdAt;

  const SubscriptionItem({
    required this.id,
    required this.externalId,
    required this.externalSource,
    required this.mediaType,
    required this.title,
    required this.year,
    this.posterPath,
    required this.status,
    required this.searchCount,
    required this.maxSearches,
    required this.createdAt,
  });

  factory SubscriptionItem.fromJson(Map<String, dynamic> json) {
    return SubscriptionItem(
      id: json['id'] as int? ?? 0,
      externalId: json['external_id'] as int? ?? 0,
      externalSource: json['external_source'] as String? ?? 'tmdb',
      mediaType: json['media_type'] as String? ?? 'movie',
      title: json['title'] as String? ?? '',
      year: json['year'] as int? ?? 0,
      posterPath: json['poster_path'] as String?,
      status: _parseStatus(json['status'] as String?),
      searchCount: json['search_count'] as int? ?? 0,
      maxSearches: json['max_searches'] as int? ?? 12,
      createdAt: DateTime.tryParse(json['created_at'] as String? ?? '') ?? DateTime.now(),
    );
  }

  static SubStatus _parseStatus(String? s) {
    switch (s) {
      case 'found': return SubStatus.found;
      case 'downloaded': return SubStatus.downloaded;
      case 'failed': return SubStatus.failed;
      default: return SubStatus.watching;
    }
  }

  String get statusText {
    switch (status) {
      case SubStatus.watching: return '搜寻中';
      case SubStatus.found: return '已找到';
      case SubStatus.downloaded: return '可观看';
      case SubStatus.failed: return '搜寻失败';
    }
  }
}
```

```dart
// lib/models/continue_watching.dart  [v7 新增]

class ContinueWatchingItem {
  final int externalId;
  final String externalSource;
  final String mediaType;
  final String title;
  final String posterUrl;
  final int season;
  final int episode;
  final int positionMs;
  final int durationMs;
  final DateTime playedAt;

  const ContinueWatchingItem({
    required this.externalId,
    required this.externalSource,
    required this.mediaType,
    required this.title,
    required this.posterUrl,
    required this.season,
    required this.episode,
    required this.positionMs,
    required this.durationMs,
    required this.playedAt,
  });

  double get progress => durationMs > 0 ? positionMs / durationMs : 0;
  bool get isFinished => durationMs > 0 && (durationMs - positionMs) < 120000; // 2 分钟内

  factory ContinueWatchingItem.fromJson(Map<String, dynamic> json) {
    return ContinueWatchingItem(
      externalId: json['external_id'] as int? ?? 0,
      externalSource: json['external_source'] as String? ?? 'tmdb',
      mediaType: json['media_type'] as String? ?? '',
      title: json['title'] as String? ?? '',
      posterUrl: json['poster_url'] as String? ?? '',
      season: json['season'] as int? ?? 0,
      episode: json['episode'] as int? ?? 0,
      positionMs: json['position_ms'] as int? ?? 0,
      durationMs: json['duration_ms'] as int? ?? 0,
      playedAt: DateTime.tryParse(json['played_at'] as String? ?? '') ?? DateTime.now(),
    );
  }
}
```

```dart
// lib/models/capabilities.dart  [v7 新增]

class Capabilities {
  final bool nasAvailable;
  final bool nasIndexComplete;
  final bool pansearchAvailable;
  final List<String> loggedInDrivers; // e.g. ["pan115", "quark"]
  final String nasPhase;              // "" / "A" / "B" / "C"
  final int nasProcessedFiles;
  final int nasTotalFiles;

  const Capabilities({
    required this.nasAvailable,
    required this.nasIndexComplete,
    required this.pansearchAvailable,
    required this.loggedInDrivers,
    this.nasPhase = '',
    this.nasProcessedFiles = 0,
    this.nasTotalFiles = 0,
  });

  bool get canInstantPlay => nasAvailable && nasIndexComplete;

  factory Capabilities.fromJson(Map<String, dynamic> json) {
    return Capabilities(
      nasAvailable: json['nas_available'] == true,
      nasIndexComplete: json['nas_index_complete'] == true,
      pansearchAvailable: json['pansearch_available'] == true,
      loggedInDrivers: (json['logged_in_drivers'] as List<dynamic>?)
              ?.map((e) => e.toString())
              .toList() ??
          [],
      nasPhase: json['nas_phase'] as String? ?? '',
      nasProcessedFiles: json['nas_processed_files'] as int? ?? 0,
      nasTotalFiles: json['nas_total_files'] as int? ?? 0,
    );
  }
}
```

### 16.3 WebSocket 消息结构（Go + Dart 共用契约，v7 增 capabilities）

```go
// websocket/messages.go

type WSMsgType string

const (
    WSHealthCheck      WSMsgType = "health_check"
    WSResolveStage     WSMsgType = "resolve_stage"
    WSResolveComplete  WSMsgType = "resolve_complete"
    WSResolveFailed    WSMsgType = "resolve_failed"
    WSDownloadProgress WSMsgType = "download_progress"
    WSSubReady         WSMsgType = "subscription_ready"
    WSIndexStatus      WSMsgType = "index_status"
    WSNotification     WSMsgType = "notification"
    WSAccountAuthFail  WSMsgType = "account_auth_failed"
    WSServerStopping   WSMsgType = "server_stopping"    // [v5 新增] §28.4 优雅退出
    WSCapabilities     WSMsgType = "capabilities"       // [v7 新增] 能力变更推送
)

type WSMessage struct {
    Type    WSMsgType       `json:"type"`
    Payload json.RawMessage `json:"payload"`
}

// resolve_stage payload
type WSResolveStagePayload struct {
    Stage       ResolveStage `json:"stage"`
    Detail      string       `json:"detail"`
    ProgressPct int          `json:"progress_pct"`
}

// resolve_complete payload
type WSResolveCompletePayload struct {
    StreamURL string `json:"stream_url"`   // 内部代理 URL: /api/stream?ticket=xxx
    Source    string `json:"source"`
    FileName  string `json:"file_name"`
    FileID    string `json:"file_id"`      // string，非 int64
    Ticket    string `json:"ticket"`
}

// resolve_failed payload
type WSResolveFailedPayload struct {
    Reason     string `json:"reason"`
    Suggestion string `json:"suggestion"`
    Stage      string `json:"stage"`
}

// download_progress payload
type WSDownloadProgressPayload struct {
    TaskID      int64  `json:"task_id"`
    ProgressPct int    `json:"progress_pct"`
    Speed       string `json:"speed"`
}

// subscription_ready payload
type WSSubReadyPayload struct {
    ExternalID int64  `json:"external_id"`
    Title      string `json:"title"`
    Source     string `json:"source"`
}

// index_status payload（v5 扩展：scope/phase/processed/total/matched/orphaned/rate_per_sec）
type WSIndexStatusPayload struct {
    AccountID  int64  `json:"account_id,omitempty"`
    Scope      string `json:"scope,omitempty"`        // nas / pan{xx}
    Phase      string `json:"phase,omitempty"`        // A / B / C
    Status     string `json:"status"`
    Processed  int    `json:"processed,omitempty"`
    Total      int    `json:"total,omitempty"`
    Matched    int    `json:"matched,omitempty"`
    Unconfirmed int   `json:"unconfirmed,omitempty"`
    Orphaned   int    `json:"orphaned,omitempty"`
    RatePerSec int    `json:"rate_per_sec,omitempty"`
    FileCount  int    `json:"file_count,omitempty"`
    ErrorMsg   string `json:"error_msg,omitempty"`
}

// notification payload
type WSNotificationPayload struct {
    Level   string `json:"level"`
    Title   string `json:"title"`
    Message string `json:"message"`
    Action  string `json:"action,omitempty"`  // [v5 新增] e.g. "open_cleanup_settings"
}

// account_auth_failed payload
type WSAccountAuthFailedPayload struct {
    AccountID  int64  `json:"account_id"`
    DriverType string `json:"driver_type"`
    Reason     string `json:"reason"`
}

// [v7 新增] capabilities payload
type WSCapabilitiesPayload struct {
    NASAvailable        bool     `json:"nas_available"`
    NASIndexComplete    bool     `json:"nas_index_complete"`
    PansearchAvailable  bool     `json:"pansearch_available"`
    LoggedInDrivers     []string `json:"logged_in_drivers"`
    NASPhase            string   `json:"nas_phase"`
    NASProcessedFiles   int      `json:"nas_processed_files"`
    NASTotalFiles       int      `json:"nas_total_files"`
}
```

---

## 17. 前端界面与交互规范

### 17.1 通用交互约定（v7 修订：骨架屏）

**所有页面必须实现的 4 种状态**：

| 状态 | 触发条件 | UI 表现 |
|---|---|---|
| loading | 首次加载数据 | 骨架屏（skeleton screen）：灰色占位卡片模拟内容布局，带 shimmer 动画（v7 修订） |
| data | 数据加载成功 | 正常内容展示 |
| empty | 数据为空（非错误） | 居中图标 + 提示文案 + [可选]引导按钮 |
| error | 网络/服务端错误 | 居中错误图标 + 错误信息 + [重试] 按钮 |

**骨架屏规范**（v7 新增，见 §17.12）：
- 探索页榜单行：每行 5 张横向占位卡片（宽 140px × 高 210px，圆角 $--radius）
- 分类页网格：3 列网格占位卡片（宽 160px × 高 240px）
- 详情页：横幅占位 + 元数据行占位
- 所有骨架屏统一使用 Design Token `$--skeleton` 颜色 + shimmer 动画

**TV 遥控器通用规则**：
- 所有可交互组件统一使用 `TvButton`（自动管理 FocusNode）
- 方向键导航：上/下/左/右自动跳转最近可聚焦节点
- [返回] 键：弹出当前页面（Navigator.pop），在根页面弹出确认退出对话框
- 弹窗出现时强制接管焦点；关闭后归还到触发按钮

### 17.2 探索页（首页，v7 修订：继续观看行 + 可播放标识）

**布局顺序（从上到下）**：

1. **[v7 新增] 「继续观看」行**（仅在有历史记录时显示）
   - 标题："继续观看" + 右箭头"全部" -> 跳转历史页
   - 内容：横向 ListView，最多 20 张卡片
   - 每张卡片：海报图（圆角 8px）+ 底部进度条（`position_ms / duration_ms`）+ 季集信息（如 "S01E03 · 剩余 23 分钟"）
   - 点击：直接进入播放器（复用已有 ticket 或重新 resolve）
   - 数据源：`GET /api/media/continue-watching`

2. **12 行横向卡片榜单**
   - 每行：标题 + 右箭头"更多" -> 横向 ListView
   - 每张卡片：海报图（圆角 8px）+ 评分角标，获焦时白色 2px 边框 + scale 1.05
   - **[v7 新增] 可播放角标**：若该内容在 media_index 中存在（P0 可秒播），海报右上角显示绿色 ✓ 角标（使用 `$--success` 色）
   - 点击：导航到详情页
   - 刷新策略：每个榜单独立缓存（内存，6 小时过期），海报用 `cached_network_image`（磁盘 7 天）

**可播放角标的获取**：探索页加载时并行调用 `GET /api/capabilities`，若 `nas_index_complete=true` 且 `nas_available=true`，前端批量查询 `POST /api/media/check-availability`（传入 film_ids[]），后端返回有索引的 ID 列表。

### 17.3 分类页（5 Tab）

| Tab | 标题 | 后端 API | external_source |
|---|---|---|---|
| 1 | 电影 | `GET /api/tmdb/discover?type=movie` | tmdb |
| 2 | 电视 | `GET /api/tmdb/discover?type=tv` | tmdb |
| 3 | 综艺 | `GET /api/tmdb/discover?type=variety` | tmdb |
| 4 | 动漫 | Bangumi 排行 API | bangumi |
| 5 | 纪录 | `GET /api/tmdb/discover?type=documentary` | tmdb |

无限滚动网格：距底部 200px 触发加载下一页。首次加载使用骨架屏（3 列网格占位卡片）。

### 17.4 详情页（v7 修订：季集可用性标识）

**播放按钮行为**：
1. 用户点击 [播放] -> 显示全屏加载弹窗（resolve 进度，含分层进度指示器 v7）
2. 请求 `POST /api/resolve`，同时建立 WebSocket 监听
3. 状态演进（见 §17.5 播放加载弹窗）
4. 成功 -> 跳转播放器页面（stream_url = `/api/stream?ticket=xxx`）；失败 -> 弹窗提示 + 引导订阅

**电视剧季集交互**（v7 修订）：
1. 进入详情页时自动加载 `GET /api/tmdb/seasons/{external_id}`
2. 每季显示为可展开的折叠面板
3. 展开后显示剧集列表（E01-NN），每集可独立点击播放
4. 点击某集 -> resolve 时额外携带 `season` + `episode` 参数
5. **[v7 新增] 季集可用性标识**：
   - 页面加载时并行调用 `POST /api/media/check-availability`（传入 season/episode 范围）
   - 已索引的集：显示**绿色 ✓ 角标** + 点击直接秒播（P0 命中）
   - 未索引的集：无角标，点击进入正常 resolve 流程
   - 实现：`GET /api/tmdb/seasons/{id}` 响应中每个 episode 增加 `available: bool` 字段

**订阅按钮显示条件**：当 resolve 返回 `not_found` 后显示；否则隐藏。

### 17.5 播放加载弹窗（Resolve Modal，v7 修订：分层进度指示器）

**[v7 新增] 分层进度指示器**：Modal 顶部显示四层引擎的步骤指示条：

```
┌──────────────────────────────────┐
│  ○ NAS 本地  →  ○ 盘搜转存  →  ○ 磁力下载  →  ○ 订阅   │
│  (当前层高亮，已完成层 ✓，未开始层灰色)            │
├──────────────────────────────────┤
│  阶段文字 + 详情                                  │
└──────────────────────────────────┘
```

| 阶段 | 指示器状态 | UI 展示 | 用户可操作 |
|---|---|---|---|
| resolve_start | P0 高亮 | "正在准备..." | 等待 |
| nas_lookup | P0 高亮（呼吸动画） | "查询本地索引..." | 等待 |
| nas_hit | P0 ✓ 绿色 | "找到本地文件 ✓" | 自动 0.5s 后进入播放 |
| pan_searching | P1 高亮（呼吸动画） | "搜索全网盘资源..." + 搜索中动画 | 等待 |
| pan_searched | P1 ✓ | "找到 N 个资源，分析中..." | 等待 |
| transferring | P1 高亮 | "正在转存到 {盘名}..." | 等待 |
| resolving_link | P1 高亮 | "获取播放链接..." | 等待 |
| play_ready | P0/P1/P2 ✓ | "播放就绪 ✓" | 自动进入播放 |
| magnet_downloading | P2 高亮（进度条） | "正在云端下载..." + 进度% + 速度 | 显示 [取消] 按钮 |
| not_found | P3 高亮 | "暂无可用资源" + 引导订阅 | [订阅] [关闭] |
| error | 当前层 ✗ 红色 | "出错了: {原因}" | [重试] [关闭] |

**P0 跳过时不显示**：当 NAS 未配置或索引未完成时，指示器直接从 P1 开始，P0 灰显 + 跳过标记。

取消行为：P0/P1 阶段取消按钮禁用（操作太快无法取消）；P2 磁力下载阶段取消按钮可用。[v7 新增] 用户关闭 Modal 后 P2 继续后台下载（§6.5）。

### 17.6 播放器页

- 控制层（触摸/点击显示，5 秒无操作自动隐藏）：返回/标题/播放暂停/seek/倍速/字幕/全屏
- 播放进度上报：每 10 秒自动 `POST /api/media/history`（含 season/episode，v7 新增）
- 播放中 WebSocket 断线：不中断播放，静默重连
- 播放 URL = `/api/stream?ticket=xxx`，直接喂给 fvp

### 17.7-17.9 历史页/订阅页/设置页

（与 v3 相同，不赘述。核心变更：所有 `tmdb_id` 字段统一为 `external_id` + `external_source`。）

### 17.x TV 焦点完整规范（v5 P1 新增）

> v4 §17.1 只提了"四方向导航"。v5 补充焦点丢失恢复、弹窗归还、后台通知防打扰等真实 TV 场景。

#### 17.x.1 焦点归属栈

每个页面维护一个 `_focusStack: List<FocusNode>`，记录焦点历史：

```dart
class _ExplorePageState extends State<ExplorePage> {
  final FocusNode _rootFocus = FocusNode(debugLabel: 'explore_root');
  final List<FocusNode> _focusHistory = [];

  void _pushFocus(FocusNode node) {
    _focusHistory.add(node);
  }

  void _restoreFocus() {
    if (_focusHistory.isNotEmpty) {
      _focusHistory.last.requestFocus();
    } else {
      _rootFocus.requestFocus();
    }
  }
}
```

#### 17.x.2 焦点丢失恢复

**问题**：TV 遥控器按 [返回] 关闭弹窗后，焦点可能丢失到屏幕外，用户陷入"按什么键都没反应"。

**对策**：
1. 弹窗关闭时调用 `_restoreFocus()`，强制把焦点还给触发弹窗的按钮
2. 监听 `FocusManager.instance.primaryFocusChanged`，若变为 null 立即恢复
3. 全局 `_rootFocus` 兜底：若焦点栈为空，请求 `_rootFocus`

#### 17.x.3 弹窗与通知的焦点策略

| 元素类型 | 焦点策略 | 用户行为 |
|---|---|---|
| Resolve Modal | 全屏接管 | 用户必须看到结果才能继续；[取消] 按钮在 P2 之前禁用 |
| 健康检查通知（顶部 banner） | 不抢焦点 | 仅信息展示，[点击] 才进设置页 |
| 订阅 ready 通知 | 不抢焦点 | 弹 toast 3s 自动消失，[点击] 进详情页 |
| 错误弹窗（如账号过期） | 必须可见但不抢焦点 | Banner 形式 + 健康面板永久展示 |

#### 17.x.4 长按与确认键标准

| 平台 | 长按 | 确认 |
|---|---|---|
| Android TV | `onLongPress` = 显示详情 | D-pad Center / Enter = onPressed |
| Apple TV | Siri Remote 点击 = 确认 | Siri Remote 触摸 = 不响应（避免误触） |
| Web TV 模拟 | 鼠标长按 | Enter / Space |

#### 17.x.5 焦点可视化

- 获焦组件：2px 白色边框 + scale 1.05（已有）
- 焦点状态监听：`FocusManager.instance.addListener` 通知健康面板（用于"当前页面"展示）
- 焦点丢失检测：每 60s 主动检查 `primaryFocus`，若为 null 触发 `_restoreFocus`

---

### 17.10 搜索页（v7 新增）

**页面入口**：探索页顶部导航栏右侧搜索图标 🔍，TV 端通过遥控器导航至搜索入口按钮。

**页面结构**：

```
┌─────────────────────────────────────┐
│  🔍 [________________]  [取消]      │  ← 搜索栏（自动获焦）
├─────────────────────────────────────┤
│  搜索历史（无输入时显示）              │
│  ┌─────────────────────────────┐    │
│  │ 🕐 阿凡达                    │    │
│  │ 🕐 权力的游戏                 │    │
│  │ 🕐 星际穿越                   │    │
│  │ [清空历史]                   │    │
│  └─────────────────────────────┘    │
├─────────────────────────────────────┤
│  搜索结果（输入时实时显示）            │
│  ┌──────┐ ┌──────┐ ┌──────┐        │
│  │ 海报  │ │ 海报  │ │ 海报  │        │
│  │ 标题  │ │ 标题  │ │ 标题  │        │
│  └──────┘ └──────┘ └──────┘        │
│  ...更多结果（无限滚动）              │
└─────────────────────────────────────┘
```

**交互规范**：
- 输入框 debounce 300ms 后发起 `GET /api/tmdb/search?q=xxx`
- TV 端：获焦搜索栏 -> 按确认键 -> 弹出虚拟键盘（系统键盘或自定义 T9 键盘）
- 结果卡片：海报 + 标题 + 年份 + 评分 + [v7] 可播放 ✓ 角标（调用 capabilities 批量查询）
- 点击结果 -> 导航到详情页
- 搜索历史：本地 SQLite 存储（Flutter 端 `shared_preferences`），最多 20 条

**无结果状态**（v7 新增引导）：
```
┌─────────────────────────────────────┐
│          🔍                          │
│     未找到"{关键词}"相关内容           │
│                                      │
│  建议：                               │
│  · 尝试使用英文名搜索（如 "Avatar"）   │
│  · 尝试简化的关键词（如 "avatar"）     │
│  · 检查拼写是否正确                   │
│                                      │
│  [直接盘搜 "{关键词}"] ← 跳过 TMDB    │
│         直接调用 PanSou 搜索          │
└─────────────────────────────────────┘
```

**直接盘搜按钮**：当 TMDB 搜索无结果时，用户可点击 [直接盘搜] 按钮 -> 跳转到 PanSou 直接搜索结果页（复用盘搜结果展示组件）。

---

### 17.11 继续观看行规范（v7 新增）

**数据来源**：`GET /api/media/continue-watching`

**后端查询**：
```sql
SELECT DISTINCT ph.external_id, ph.external_source, ph.media_type,
       ph.title, ph.poster_url, ph.season, ph.episode,
       ph.position_ms, ph.duration_ms, ph.played_at
FROM play_history ph
WHERE ph.position_ms > 0
  AND (ph.duration_ms = 0 OR (ph.duration_ms - ph.position_ms) > 120000)  -- 剩余 > 2 分钟
ORDER BY ph.played_at DESC
LIMIT 20
```

**卡片设计**：
- 宽度 200px，高度 120px（16:9 横版）
- 背景：海报图（模糊或暗化处理）
- 叠加层：标题（粗体）+ 季集信息（如 "S01E03"）+ 进度条（`$--accent` 色，高度 3px）
- 右上角：剩余时长（如 "剩余 23 分钟"）或进度百分比

**显示条件**：列表非空时才渲染该行。

---

### 17.12 骨架屏规范（v7 新增）

**通用规则**：
- 颜色：`$--skeleton`（浅灰，附录 C Design Token）
- 动画：shimmer 效果（从左到右渐变，周期 1.5s）
- 圆角：卡片占位使用 `$--radius`（默认 12px）

**各页面骨架屏**：

| 页面 | 骨架屏结构 |
|---|---|
| 探索页 | 继续观看行：5 张 200×120 占位卡片；每榜单行：标题占位（120×20）+ 5 张 140×210 占位卡片 |
| 分类页 | 3 列网格，每格 160×240 占位卡片 × 9 行 |
| 搜索页 | 搜索栏占位 + 3 列网格（同分类页） |
| 详情页 | 横幅占位（全宽 × 200px）+ 标题占位（60%宽 × 28px）+ 元数据行占位 × 4 |
| 历史/订阅页 | 列表项占位（全宽 × 80px）× 10 行 |

**Flutter 实现**：使用 `shimmer` 包 + 自定义 `SkeletonCard` widget，通过 `SkeletonConfig` 全局配置颜色和动画参数。

---

## 18. HTTP API 契约详细规范

### 18.1 通用响应格式

**成功响应**：
```json
{
  "items": [...],
  "page": 1,
  "has_more": true,
  "total": 150
}
```
单个对象直接在顶层返回字段（非 `{data:...}` 包装）。

**错误响应**：
```json
{
  "error": "可读的错误信息",
  "code": "RESOLVE_NOT_FOUND",
  "action": "建议订阅，系统将每周自动搜寻"
}
```

### 18.2 完整端点 Schema

<details>
<summary><b>GET /api/capabilities</b> - [v7 新增] 能力预检</summary>

Response:
```json
{
  "nas_available": true,
  "nas_index_complete": false,
  "nas_phase": "B",
  "nas_processed_files": 45000,
  "nas_total_files": 125000,
  "pansearch_available": true,
  "logged_in_drivers": ["pan115", "quark"],
  "server_version": "7.0.0"
}
```

前端据此决定：
- 探索页/分类页是否显示可播放 ✓ 角标（需 `nas_available && nas_index_complete`）
- 详情页季集列表是否查询 availability
- Resolve Modal 是否跳过 P0（`nas_index_complete=false` 时跳）
</details>

<details>
<summary><b>POST /api/media/check-availability</b> - [v7 新增] 批量检查内容可用性</summary>

Request:
```json
{
  "items": [
    {"external_id": 19995, "external_source": "tmdb", "season": 0, "episode": 0},
    {"external_id": 1399, "external_source": "tmdb", "season": 1, "episode": 1}
  ]
}
```

Response:
```json
{
  "available": [
    {"external_id": 19995, "external_source": "tmdb", "season": 0, "episode": 0},
    {"external_id": 1399, "external_source": "tmdb", "season": 1, "episode": 1}
  ]
}
```

仅返回已索引的条目。前端用返回的 ID 列表显示绿色 ✓ 角标。
</details>

<details>
<summary><b>GET /api/media/continue-watching</b> - [v7 新增] 继续观看</summary>

Response:
```json
{
  "items": [
    {
      "external_id": 19995,
      "external_source": "tmdb",
      "media_type": "movie",
      "title": "阿凡达",
      "poster_url": "https://image.tmdb.org/t/p/w500/...",
      "season": 0,
      "episode": 0,
      "position_ms": 5400000,
      "duration_ms": 9720000,
      "played_at": "2026-08-09T10:30:00Z"
    }
  ]
}
```
</details>

<details>
<summary><b>POST /api/resolve</b> - 触发播放引擎</summary>

Request:
```json
{
  "external_id": 19995,
  "external_source": "tmdb",
  "media_type": "movie",
  "title": "阿凡达",
  "year": 2009,
  "season": 0,
  "episode": 0
}
```

Response (201):
```json
{ "task_id": 42, "reused": false }
```
`reused: true` 表示已有同 `(external_id, external_source, season, episode)` 的运行中任务。

Response (200, 复用):
```json
{ "task_id": 42, "reused": true }
```

Response (429, v7 新增):
```json
{
  "error": "请求过于频繁，请稍后再试",
  "code": "RATE_LIMITED",
  "retry_after_sec": 12
}
```
</details>

<details>
<summary><b>GET /api/resolve/result/{task_id}</b> - 查询解析结果</summary>

Response (done):
```json
{
  "stream_url": "/api/stream?ticket=eyJhbGci...",
  "source": "pan115",
  "file_id": "abc123",
  "title": "阿凡达.mkv",
  "year": 2009,
  "transfer_id": 0
}
```
`stream_url` 始终为内部代理 URL，真实直链不出后端。
`transfer_id > 0` 表示仍在转存中，前端应轮询。

Response (running):
```json
{
  "status": "running",
  "stage": "pan_searching",
  "stage_detail": "正在搜索全网盘资源...",
  "progress_pct": 35
}
```
</details>

<details>
<summary><b>GET /api/stream?ticket=xxx</b> - 播放流代理</summary>

- 验证 ticket HMAC 签名 + 过期检查
- NAS 文件：代理 Range 请求（206 Partial Content）
- 网盘文件：302 重定向到真实直链（直链过期自动刷新）
- 无效 ticket：401 Unauthorized
- 直链刷新失败：502 Bad Gateway
</details>

### 18.3 错误码体系（v5 P2 扩展：15 个；v7 新增 2 个）

| HTTP Code | Error Code | 触发条件 | 前端处理 |
|---|---|---|---|
| 400 | `INVALID_PARAM` | 缺少/无效参数 | 显示错误信息，不重试 |
| 401 | `AUTH_REQUIRED` | JWT 过期/无效 | 跳转设置页重新登录 |
| 404 | `NOT_FOUND` | 资源不存在 | 显示"未找到" |
| 409 | `TASK_RUNNING` | 同 ID 已有运行中任务 | 返回已有 task_id，前端接入 |
| 429 | `RATE_LIMITED` | [v7 新增] Resolve 请求频率超限 | 显示"请求过于频繁" + 自动 `retry_after_sec` 后重试 |
| 502 | `UPSTREAM_FAILED` | TMDB/网盘 API 不可用 | 显示"服务暂时不可用"+ 重试 |
| 503 | `SERVICE_UNAVAILABLE` | PanSou/数据库不可用 | 显示错误 + 自动 30s 重试 |
| 200+error | `RESOLVE_NOT_FOUND` | 四层均无资源 | 引导订阅 |
| 200+error | `RESOLVE_PARTIAL` | P0/P1 失败，P2 下载中 | 展示下载进度 |
| 408 / 200+error | `RESOLVE_TIMEOUT` | [v5 新增] 单层超时（如 PanSou 搜索 10s 无响应） | 阶段文字更新为"搜索超时，正在尝试下一层..." |
| 503 | `INDEX_EMPTY` | [v5 新增] NAS 还没扫完，P0 索引为空 | 顶部 Banner 提示"索引尚未完成，部分功能受限" |
| 507 / 200+error | `QUOTA_EXCEEDED` | [v5 新增] 网盘满了，无法转存 | 弹通知 + 跳转"清理设置"页 |
| 429 / 200+error | `DRIVER_RATE_LIMITED` | [v5 新增] 115/夸克 API 风控 | 阶段文字更新为"网盘限流，稍后重试..." + 自动 60s 退避 |
| 502 | `NETWORK_UNREACHABLE` | [v5 新增] 家庭网络断（驱动 API 超时） | 健康检查 nas/pansearch 标 error + 播放流程返回 NOT_FOUND |
| 401 | `TICKET_EXPIRED` | [v5 新增] Ticket HMAC 签名过期（用于播放器上报） | fvp 收到后自动调用 `/api/stream?refresh=true` 重发 |
| 503 | `CAPABILITY_DEGRADED` | [v7 新增] NAS 扫描中 / PanSou 不可用，能力受限 | 前端按 capabilities 返回值调整 UI |

**错误码总数**：v4 (9 个) + v5 P2 (6 个) + v7 (2 个) = **17 个错误码**，覆盖正常/异常/恢复三类场景。

---

## 19. 异常处理与容错规范（v7 修订：P2 后台 + 并发限流）

### 19.1 后端分层错误处理

```
第 1 层：驱动层（driver/*）
  ↓ 返回 *DriveError（Code + Message + Action）
第 2 层：服务层（internal/resolve, pansearch, indexengine）
  ↓ 包装业务上下文 + slog.Error
第 3 层：Handler 层（internal/api/*）
  ↓ 映射为 HTTP 状态码 + JSON error response
第 4 层：前端（Flutter）
  ↓ 展示 error/action + 根据 code 执行恢复动作
```

### 19.2 关键场景异常处理矩阵（v7 增 2 行）

| 场景 | 异常 | 后端行为 | 前端行为 |
|---|---|---|---|
| TMDB API 不可用 | 网络超时/500 | 返回 media_library 过期缓存（带 stale 标记） | 展示缓存数据 + 顶部 Banner 提示 |
| PanSou 不可用 | 连接超时 | P1 层跳过，直接进 P2 磁力兜底 | 阶段文字不显示"搜索网盘" |
| 盘搜全部源不可用 | 所有源超时 | 返回空结果 + slog.Warn | P1 失败，进入 P2 |
| 网盘 cookie/token 过期 | 401/403 | 触发 eventbus.AccountAuthFailed -> WS 推送 | 弹通知"网盘 {name} 登录已失效" |
| 转存失败（容量不足） | DriveError | 跳过当前源，尝试下一个优先级源（§6.9.4） | 阶段文字更新为"正在尝试其他来源..." |
| 转存失败（分享失效） | DriveError | 标记该搜索结果无效 -> 尝试下一个结果 | 同上 |
| 直链过期（播放中断） | sign 过期 401 | stream 代理自动 GetFile 刷新（对前端透明） | 无感知，正常播放 |
| 数据库写入失败 | SQLite error | 事务回滚 + slog.Error | 本次操作返回 500 |
| NAS SMB 挂载断开 | 文件不可访问 | P0 查询返回空 | 静默降级到 P1 盘搜 |
| WebSocket 断线 | 网络断开 | Hub 自动移除死连接 | 指数退避重连 + HTTP 补刷 |
| 并发 resolve 同一 ID | 重复请求 | 返回已有 task_id（reused=true） | 接入已有任务 |
| 服务重启 | 运行中任务中断 | 启动恢复：P2 下载中任务检查 115 状态，其他标记 failed（§28.2） | 用户重试 |
| 网盘配额预警 | 剩余空间 < 阈值 | WS 推送 quota_warning（§6.9.3） | 弹通知 + [去设置] |
| 115 API 风控 | 429 限流 | 退避 60s 后重试 | 阶段文字提示"网盘限流" |
| **[v7] P2 下载中用户离线** | 用户关闭 App | 下载继续后台运行，完成后写入 media_index | 下次打开 App 时 WS 重连推送状态 |
| **[v7] Resolve 并发超限** | 同 IP 30s 超 3 次 | 返回 429 + retry_after_sec | 显示"请求过频" + 自动倒计时重试 |

### 19.3 数据一致性保障

**索引一致性**：
- 事件驱动（eventbus.FileMutated）实时更新
- 索引校验默认关闭，用户开启后按周执行（对比 media_index 和实际文件列表）

**订阅状态一致性**：
- 订阅状态转换需通过 `CanTransitionTo()` 校验
- 定时任务执行前获取分布式锁（`automation_runs` 表唯一约束）
- 失败自动记录 `search_count`，达到 `max_searches` 后标记 `failed`

**直链缓存一致性**：
- URL 缓存后如果播放 404 -> 自动将 media_index 标记 stale
- 下一次 resolve 不再命中 stale 索引，强制重新搜索+转存

**ResolveTask 一致性（v5 新增）**：
- pending 状态超过 30 天自动清理
- 启动恢复按 stage 分级处理（§28.2）
- 单事务批量更新避免锁竞争

**PanSou 缓存一致性（v7 新增）**：
- `pansearch_cache.link_count` 跟踪有效链接数
- 全部链接失效时标记 stale（link_count=0），30 分钟后强制重搜（§8.5.1）

---

## 20. 状态管理与数据流

### 20.1 WebSocket 客户端状态机（Flutter 端）

```dart
enum WsConnectionState {
  disconnected,
  connecting,
  connected,
  reconnecting,
}

// 重连策略
class WsReconnectPolicy {
  static const List<int> delays = [1, 2, 4, 8, 16, 30]; // 秒
  static const int maxDelay = 30;

  int attempt = 0;

  int nextDelay() {
    final d = delays[attempt.clamp(0, delays.length - 1)];
    attempt++;
    return d;
  }

  void reset() => attempt = 0;
}
```

连接生命周期：
1. Flutter 启动 -> 读取设置页保存的后端地址
2. `disconnected` -> `connecting`：发起 WebSocket 连接
3. 成功 -> `connected`：注册消息处理器
4. 断线 -> `reconnecting`：启动重连计时器
5. 重连成功 -> `connected` -> `GET /api/state/snapshot` HTTP 补刷（§28.3：对比 server_started_at 触发强制刷新）
6. 重连失败（超过最大重试）-> `disconnected`：弹通知"与服务器断开连接"

### 20.2 播放引擎任务去重

去重 key = `(external_id, external_source, season, episode)`：

```
用户点击播放 (external_id=19995, source=tmdb, season=1, episode=3)
  │
  ├── 前端: POST /api/resolve
  │     └── 后端: 检查是否有运行中的 (19995, tmdb, 1, 3) 任务
  │           ├── 有 -> 返回 {task_id, reused: true}
  │           └── 无 -> 创建新任务，返回 {task_id, reused: false}
  │
  ├── 前端: 收到 reused=true -> 不创建新 loading，接入已有任务
  │     通过 WebSocket 监听 resolve_stage 更新
  │
  └── 前端: 收到 reused=false -> 创建 loading 弹窗
        同时建立 WebSocket 对该 task_id 的监听
```

后端查询：

```sql
SELECT * FROM resolve_tasks
WHERE external_id = ? AND external_source = ? AND season = ? AND episode = ?
  AND status IN ('pending', 'running')
LIMIT 1
```

### 20.3 并发限流（v7 新增）

`POST /api/resolve` 端点限制同 IP 在滑动窗口内的请求频率：

```go
// internal/resolve/rate_limiter.go

type RateLimiter struct {
    maxRequests int           // 默认 3
    windowSec   int           // 默认 30
    store       *store.Store
}

func (rl *RateLimiter) Allow(ctx context.Context, clientIP string) (bool, int) {
    windowStart := time.Now().Truncate(time.Duration(rl.windowSec) * time.Second)

    count, err := rl.store.GetRateLimitCount(clientIP, windowStart)
    if err != nil {
        return true, 0 // 降级：DB 故障时放行
    }

    if count >= rl.maxRequests {
        remaining := int(windowStart.Add(time.Duration(rl.windowSec) * time.Second).Sub(time.Now()).Seconds())
        return false, remaining
    }

    rl.store.IncrementRateLimit(clientIP, windowStart)
    return true, 0
}
```

**清理**：`resolve_rate_limits` 表中超过 1 小时的记录自动清理（定时任务，每小时一次）。

---

## 21. 性能与缓存规范

### 21.1 后端缓存分层

| 层 | 存储 | 数据 | TTL | 说明 |
|---|---|---|---|---|
| L1 | Go 内存 map | 网盘目录列表 | 按 cacheretention 配置 | LitePan 现有机制 |
| L2 | SQLite media_library | TMDB 详情/元数据 | 7 天（+ v7 LRU 淘汰） | 减少 TMDB API 调用 |
| L3 | SQLite pansearch_cache | 盘搜结果 | 1 小时（+ v7 link_count 失效） | 同一关键词不重复搜索 |
| L4 | SQLite media_index.stream_url | 直链 URL | 2 小时（115）/ 1 小时（夸克） | 直链过期自动刷新 |

### 21.2 前端图片缓存

使用 `cached_network_image` 包：
- 磁盘缓存：最多 200MB，LRU 淘汰
- 海报默认显示占位图（灰色圆角卡片）
- TMDB 图片 URL 基础：`https://image.tmdb.org/t/p/w500{poster_path}`

### 21.3 前端数据缓存

- 探索页 12 个榜单独立缓存（内存，6 小时）
- [v7] 继续观看行：每次进入首页强制刷新（实时性要求高），缓存 1 分钟兜底
- [v7] capabilities 数据：内存缓存，WS `capabilities` 推送时更新
- 分类页各 Tab 首页缓存（5 分钟）
- 切换 Tab 时优先显示缓存，后台静默刷新
- 详情页不缓存（实时性要求高）

### 21.4 日志规范（v5 P1 新增）

> v4 §21 没有日志约定。v5 补 slog 结构化字段规范，覆盖生产调试必需场景。

#### 21.4.1 slog 字段约定

所有日志必须使用 `slog.Default()` + 结构化 key-value，禁止裸 `fmt.Println` 或 `log.Println`：

```go
slog.Info("resolve stage advanced",
    "task_id", task.ID,
    "external_id", task.ExternalID,
    "stage", newStage,
    "prev_stage", prevStage,
    "duration_ms", time.Since(start).Milliseconds(),
    "source", "resolve.engine",
)
```

#### 21.4.2 必填字段

| 字段 | 类型 | 说明 |
|---|---|---|
| `task_id` | int64 | ResolveTask ID（resolve 相关日志必填） |
| `external_id` | int64 | TMDB/Bangumi ID（resolve/订阅相关） |
| `source` | string | 日志来源模块，如 `resolve.engine` / `pansearch.service` / `indexengine.nas` |
| `duration_ms` | int64 | 操作耗时（IO/API 调用日志必填） |
| `error_code` | string | 错误码（错误日志必填） |
| `account_id` | int64 | 网盘账号 ID（驱动调用日志必填） |

#### 21.4.3 脱敏规则

- 网盘直链 URL（含 sign/token 参数）：`redactURL()` 脱敏，输出 `<url-redacted>`
- TMDB API Key：不写入日志（configs 读取时也不记录明文）
- JWT token：不写入日志
- 用户密码：不写入日志

#### 21.4.4 日志级别策略

| 级别 | 用途 | 示例 |
|---|---|---|
| `Debug` | 详细的内部状态（默认不输出） | 单文件元数据提取、单个 PanSou 解析结果 |
| `Info` | 关键业务事件 | resolve stage 推进、账号登录成功、订阅找到资源 |
| `Warn` | 可恢复的错误 | PanSou 搜索无结果、转存失败后降级 |
| `Error` | 不可恢复的错误 | DB 写入失败、driver API 401、ticket 验证失败 |

#### 21.4.5 日志输出

- 开发环境：stdout（人类可读）+ 文件（JSON）
- 生产环境：仅文件（JSON），路径 `/var/log/xmedia/xmedia.log`
- 日志轮转：lumberjack，按 100MB 切分，保留 10 个备份

### 21.5 性能预算（v5 P1 新增，v7 增 2 项）

> v4 §15 没有性能 SLO。v5 补充验收数字，避免"跑通 = 完成"的假象。

| 场景 | 性能指标 | 测量方法 |
|---|---|---|
| P0 NAS 命中 | 播放首帧 < 1s | DB 查询 + ticket 签发 + Range 206 |
| P1 盘搜 + 转存 | 找到资源到能播 < 15s（10GB 内） | End-to-end 计时 |
| P1 命中缓存 | 同上 < 3s | L3 pansearch_cache 命中 |
| P2 磁力下载启动 | < 5s（115 离线任务提交） | AddOfflineTask 响应时间 |
| 详情页加载 | TMDB + Bangumi 并行 < 1.5s | HTTP 调用计时 |
| 健康检查首条消息 | WS 连接建立 < 2s | 连接握手 + self-check |
| WS 重连恢复 | 断线到重连完成 < 30s（平均） | 指数退避 + HTTP 补刷 |
| NAS 扫描吞吐 | > 200 文件/秒（8 worker） | Phase B rate_per_sec |
| 探索页 12 榜单加载 | < 3s（缓存命中）< 8s（冷启动） | 12 个 API 并行 |
| 启动时间 | DB ready 到 HTTP 监听 < 5s | Phase 1-6 总耗时 |
| [v7] capabilities 查询 | < 50ms | 纯内存计算 |
| [v7] 继续观看加载 | < 500ms | SQLite 查询 + media_library join |

---

## 22. 安全规范（v7 修订：并发限流）

### 22.1 敏感信息保护

| 数据 | 保护措施 |
|---|---|
| TMDB API Key | 仅存在于 `configs` 表 + 后端内存，不经过前端传输 |
| 网盘 Cookie/Token | LitePan 原有 `account_auth_states` 表加密存储 |
| 115 OAuth Token | 同上，`account_auth_states` 表加密存储 |
| 网盘直链 sign URL | **绝不出后端**：前端只拿 ticket -> /api/stream 代理内部解析 |
| Ticket 签名密钥 | `configs` 表 key=`ticket_signing_secret`，启动时随机生成 |
| 日志中的 URL | `redactURL()` 脱敏 sign/token 参数（§21.4.3） |
| JWT Token | 仅通过 POST /api/auth/login 获取，WebSocket URL query 传递 |
| Admin 密码 | `configs.admin_password_hash`（bcrypt） |

### 22.2 API 安全（v7 新增：并发限流）

- 所有管理后台 API 需要 JWT 认证（LitePan 已有 adminauth）
- 播放器 API（TMDB/Resolve/Play）需要有效 JWT
- `GET /api/health` 无需认证
- Body 大小限制：`resolve` 和 `media` 端点 4KB
- **[v7 新增] Resolve 并发限流**：同一 IP 每 30 秒最多 3 次 `/api/resolve` 请求（§20.3）
- **[v7 新增] check-availability 批量上限**：单次请求最多 100 个 item，防止恶意批量查询

### 22.3 启动安全（v5 新增，§28 联动）

- 首次启动无 admin 密码时强制进入初始化向导（§1.4 Step 2）
- `x-media.reset-password=true` 配置启动时重置 admin 密码
- 启动 30s 内必须通过健康检查，否则告警（运维侧）

---

## 23. 开发路线图细化（TDD 驱动）（v7 修订：增 v7 专项验收项）

### 23.0 TDD 开发铁律

| 原则 | 说明 |
|---|---|
| 测试先行 | 每个函数/方法在实现前必须先有测试用例 |
| 红灯-绿灯-重构 | `go test ./...` 先红后绿，不允许跳过红灯直接提交 |
| 核心业务逻辑覆盖率 | 文件名清洗器、质量排序器、状态机转换、Resolve 阶段调度 > 80% |
| API handler 覆盖率 | > 50% |
| 驱动层覆盖率 | > 30% |
| 契约测试 | API handler 的请求/响应格式必须用 table-driven test 覆盖正常+异常路径 |
| 回归保护 | 任何 bug 修复必须先写能复现该 bug 的测试 |
| 前端测试 | Flutter widget 测试覆盖所有页面 4 状态 + TV 焦点导航逻辑（§17.x） |

### 23.1 各 Phase 详细计划 + 验证标准（v7 增项标 [v7]）

#### Phase 0: 基础裁剪（1-2 天）

| 任务 | 验收标准 |
|---|---|
| clone LitePan | 仓库完整，go.mod 可解析 |
| 删除砍掉的模块 | `go build ./...` 通过 |
| 删除多余驱动 | `drivers/all.go` 只 import 6 个驱动 |
| 改包名 | `go test ./...` 全部绿 |
| Vue 管理后台可启动 | `npm run dev` 成功 |

#### Phase 1: 数据库 + 核心模型（v7：2-3 天）

| 任务 | 验收标准 |
|---|---|
| 新增 migration 0016-0026 | 所有 migration 可执行和回滚 |
| 新增 domain 模型（含 v5 补字段 + v7 play_history season/episode + media_library last_accessed_at） | 所有 struct 的 json tag 一致 |
| 新增 store Repository | CRUD + 唯一约束冲突测试通过 |
| 状态机转换测试 | 合法/非法转换全覆盖 |
| [v7] media_library LRU 清理 | 触发阈值测试 + 保留规则测试（收藏/订阅/播放记录保留） |

#### Phase 2: TMDB 代理（v7：1-2 天）

| 任务 | 验收标准 |
|---|---|
| `internal/tmdb` Service | mock HTTP，测试搜索/详情/榜单 |
| TMDB API 代理端点 | 200/401/404/超时场景全覆盖 |
| Bangumi API 集成 | mock 响应，测试动漫搜索和排行 |
| 缓存逻辑 | 缓存命中/过期/回退 stale 数据全覆盖 |
| 日志字段（§21.4）| resolve/tmdb 日志含 external_id/duration_ms 必填字段 |
| [v7] 搜索历史端点 | 存储/查询/清空/20 条上限 |

#### Phase 3: 索引引擎（v5：2-3 天）

| 任务 | 验收标准 |
|---|---|
| 文件名清洗器 | 100+ 真实文件名样本，准确率 > 90% |
| TMDB 匹配器 | matched/unconfirmed/orphaned 分类准确 |
| NAS 三阶段扫描（§9.7） | Phase A < 5s/万文件；Phase B > 200 文件/秒（8 worker）；Phase C 准确标记 |
| 网盘增量索引 | FileMutated -> 索引写入/删除 |
| 索引查询接口 | 按 external_id / 标题查询正确 |
| 进度推送（§9.7.1） | WS index_status payload 含 phase/processed/total/matched/orphaned/rate_per_sec |

#### Phase 4: 盘搜引擎（v7：2-3 天）

| 任务 | 验收标准 |
|---|---|
| PanSearch HTTP 客户端 | mock PanSou API，测试搜索/检测 |
| 结果解析 | merged_by_type -> PanSearchResult 正确转换 |
| 质量排序器 | CAM 排除 + 4K 优先 + 优先级排序正确 |
| 本地缓存 | 1 小时 TTL 命中/过期正确 |
| [v7] 缓存失效机制 | link_count=0 且 30 分钟后跳过缓存重搜 |
| 只搜已登录网盘 | 未登录网盘不出现在搜索范围 |
| docker-compose 集成 | PanSou + X-MEDIA 双服务启动正常 |

#### Phase 5: 四层播放引擎（v7：3-4 天）

| 任务 | 验收标准 |
|---|---|
| P0 NAS 查询 | mock DB 返回索引记录，命中 -> 返回 ticket |
| [v7] P0 智能跳过 | NAS 扫描中/未配置/空索引时跳过 P0，不显示 nas_lookup 阶段 |
| P1 盘搜 + ShareSaver 转存（含 §6.9） | mock pansearch + mock ShareSaver，全流程 + 转存路径正确 + [v7] 多语言关键词回退 |
| P2 磁力兜底 | 关键词去季集 + "磁力 高清" 后缀（§6.4）；mock offline download，无盘搜结果 -> 触发磁力下载；[v7] P2 后台行为测试 |
| P3 无资源 | mock 全部失败，返回 not_found + 自动创建订阅 |
| Ticket 生成 + Stream 代理 | HMAC 签名/验证/过期/Range 代理/302 重定向 |
| ResolveTask 持久化 + 启动恢复（§28.2） | pending/running 非 magnet 直接 failed；magnet 调用 115 检查后分流 |
| 并发去重 | 并发请求同 (external_id, season, episode) -> reused=true |
| 转存配额预警（§6.9.3） | 配额低于阈值触发 WS notification |
| [v7] 并发限流 | 超过 3 次/30s 返回 429 |

#### Phase 6: WebSocket + 订阅（v7：2-3 天）

| 任务 | 验收标准 |
|---|---|
| WebSocket Hub | register/broadcast/unregister 正确 |
| Resolve 阶段推送 | 每个阶段的消息 type/payload 正确 |
| 健康检查首条消息 | health_check 包含所有子系统状态 + server_started_at + [v7] capabilities |
| 订阅调度器 | watching->found->downloaded 全流程 |
| 断线重连 | 模拟断线 -> 验证重连 + HTTP 补刷 |
| 启动恢复（§28.2） | mock DB 重启场景，验证 magnet 任务 115 状态查询 |
| [v7] capabilities 推送 | 网盘登录/退出/NAS 扫描完成时主动推送 capabilities 变更 |

#### Phase 7a: 驱动接口 + 115/夸克 ShareSaver（2-3 天）

| 任务 | 验收标准 |
|---|---|
| ShareSaver 接口签名 | table-driven test 覆盖 4 种 share URL 格式 |
| OfflineDownloader 接口签名 | table-driven test 覆盖状态机转换 |
| 115 share.go | SaveShare 调用 115 OpenAPI 保存分享接口 |
| 115 offline.go | AddOfflineTask + GetOfflineTaskStatus + CancelOfflineTask |
| Quark share.go | SaveShare 调用夸克 share/save 接口 |

#### Phase 7b: 123/百度/光鸭 ShareSaver（2-3 天）

| 任务 | 验收标准 |
|---|---|
| 123 share.go | 各驱动 SaveShare 流程跑通 |
| 百度 share.go | 同上 |
| 光鸭 share.go | 同上 |
| Rename 实现（§6.9.2） | 各驱动 Rename 调用成功 + 文件名规则测试 |

#### Phase 8: Vue 管理后台扩展（v5：3-4 天）

| 任务 | 验收标准 |
|---|---|
| 网盘优先级配置页（含 NAS 可关闭开关 + [v7] 已登录校验提示） | 拖拽排序 + 保存 + NAS 启用开关（§11.1） |
| 各盘索引策略配置页（含配额阈值） | 表单验证 + quota_warning 阈值配置 |
| 索引状态面板（含 NAS 扫描进度） | 各盘文件数/最后扫描时间正确 + NAS Phase 显示 |
| TMDB 配置页 + API Key 测试 | 有效 Key=绿，无效=红 |
| 盘搜服务配置页 + 连通性测试 | PanSou 可达=绿，不可达=红 |
| NAS 路径配置页 + 可读性测试 | 路径可读=绿 |
| 健康检查面板（§27.4） | 每个 status 对应具体操作按钮 + overall 颜色 |
| 转存路径/配额配置页（§6.9） | save_root 修改 + quota_warning_gb 配置 |

#### Phase 9a: Flutter 基础脚手架（v7：9-11 天）

| 任务 | 验收标准 |
|---|---|
| 项目脚手架 + 路由 | 所有页面可导航 |
| 探索页（12 榜单 + [v7] 继续观看行 + [v7] 骨架屏 + [v7] 可播放 ✓ 角标） | 4 状态 + 横向滚动 + capabilities 联动 |
| [v7] 搜索页（TMDB 搜索 + 历史 + 无结果引导） | debounce 300ms + 直接盘搜兜底 |
| 分类页（5 Tab 瀑布流 + [v7] 骨架屏） | Tab 切换不丢状态 |
| 详情页 + 电视剧季集下钻（[v7] 已索引集绿色 ✓ 角标） | 电影/电视剧详情正确 + availability 联动 |
| 历史页 / 订阅页 | 4 状态 + 操作 |
| 设置页 | 表单保存 + 连接测试 |
| [v7] Design Token 应用 | 附录 C 变量到所有组件（颜色/间距/圆角/字体） |

#### Phase 9b: TV 焦点组件库（4-5 天）

| 任务 | 验收标准 |
|---|---|
| 焦点归属栈（§17.x.1） | _focusStack push/pop 正确 |
| 焦点丢失恢复（§17.x.2） | 弹窗关闭后焦点正确归还 |
| 弹窗与通知的焦点策略（§17.x.3） | 各类 UI 元素焦点行为符合规范 |
| 长按与确认键标准（§17.x.4） | 三平台（Android TV / Apple TV / Web）表现一致 |
| 焦点可视化（§17.x.5） | primaryFocus 监听 + 健康面板同步 |

#### Phase 9c: fvp 播放器 + WebSocket + 健康检查（v7：8-10 天）

| 任务 | 验收标准 |
|---|---|
| fvp 播放器集成 | 播放/暂停/seek/倍速/字幕 |
| 播放加载弹窗（[v7] 分层进度指示器） | 10 个阶段 UI 状态 + 四层步骤条 |
| WebSocket 客户端 | 指数退避重连 + HTTP 补刷（§20.1） |
| 健康检查面板 | server_started_at 变化触发强制刷新（§28.3） |
| 启动健康状态显示 | Day 1 Step 3 系统状态页正确（§1.4） |
| [v7] P2 后台下载恢复 | App 重启后 WS 重连自动接入 P2 任务 |

#### Phase 10: 集成测试 + E2E（v7：4-6 天）

| 任务 | 验收标准 |
|---|---|
| 端到端播放流程 | TMDB 浏览 -> 详情 -> 播放 -> P0/P1/P2 全路径 |
| NAS 索引 + 播放 | 索引完成 -> 搜索正确 -> 秒开 |
| 盘搜 + 转存 + 播放 | 盘搜命中 -> 转存 -> 播放正常 |
| 订阅全流程 | 订阅 -> 定时搜寻 -> found -> downloaded -> 播放 |
| 启动健康检查 | 模拟各种启动失败场景，验证 §28 启动序 |
| TV 遥控器操作 | 方向键导航 + 播放控制全路径 + 焦点丢失恢复 |
| Day 1 启动旅程端到端 | §1.4 Step 1-5 全链路验证 |
| 性能预算（§21.5） | 12 个场景全部达标 |
| [v7] 继续观看流程 | 播放 -> 上报进度 -> 首页继续观看行 -> 续播 |
| [v7] 搜索页全流程 | 搜索 -> 结果 -> 无结果 -> 直接盘搜 |
| [v7] 多语言搜索回退 | 中文无结果 -> 英文回退 -> 混合回退 |
| [v7] 并发限流 | 超频 -> 429 -> 倒计时重试 |
| [v7] P2 后台 | 启动 P2 -> 关闭 App -> 重新打开 -> 接入已有任务 |

### 23.2 编码顺序约束

1. Phase 1 必须先于 Phase 2-6（数据库是基础）
2. Phase 3 与 Phase 4 可并行
3. Phase 5 依赖 Phase 3 + Phase 4
4. Phase 6 依赖 Phase 5
5. Phase 7a 依赖 Phase 5（Resolve 引擎需要 ShareSaver 接口）
6. Phase 7b 依赖 Phase 7a
7. Phase 8 依赖 Phase 1
8. Phase 9a 与 Phase 2-8 可并行（前后端只需约定好 API 契约）
9. Phase 9b 依赖 Phase 9a
10. Phase 9c 依赖 Phase 9a + Phase 9b
11. Phase 10 必须在所有 Phase 完成后执行

---

## 24. 外部媒体库集成（v1.1 储备）

> ⚠️ **v1.0 不实现。本章节为 v1.1 设计储备，编码时不纳入 Phase 计划。**

详见 §13.1 模块裁剪说明：v1.0 砍除 embyproxy，v1.1 通过本章节重新引入并扩展。

设计要点（v1.1）：
- 复用 LitePan 原有 `internal/embyproxy` 改造为 `internal/externalmedia`
- 支持 Emby/Jellyfin/Plex 三类外部媒体库
- `external_media_servers` 表（§5.1 0023）存服务器配置
- `external_media_cache` 表（§5.1 0024）缓存外部库元数据
- API 端点：`GET/POST /api/admin/external/servers`、`GET /api/external/{server_id}/library` 等
- Vue 管理后台：服务器配置页 + 库浏览
- Flutter 前端：分类页新增"外部库"Tab（仅在至少一个外部服务器 enabled 时显示）
- 隔离原则：外部媒体库不参与 Resolve 四层引擎；仅作为元数据补充来源

---

## 25. 字幕自动搜索与索引系统（v1.1 储备）

> ⚠️ **v1.0 不实现。本章节为 v1.1 设计储备，编码时不纳入 Phase 计划。**

设计要点（v1.1）：
- opensubtitles.com REST API 集成
- `subtitle_index` 表（§5.1 0025）缓存已下载字幕
- 触发时机：播放时按需搜 + 索引时预搜（季集级别）
- 字幕来源：opensubtitles + 同目录兄弟文件 + 手动上传
- 字幕文件存后端本地，fvp 通过 `/api/subtitle?ticket=xxx` 代理加载
- 下载去重：同 `(external_id, season, episode, language, source, source_id)` UNIQUE 约束
- API 端点：`GET /api/subtitle/search`、`GET /api/subtitle/download/{id}` 等

---

## 26. 播放器防崩溃规范

### 26.1 防抖与互斥锁定

| 场景 | 问题 | 对策 |
|---|---|---|
| 用户快速连点 [播放] | 多个 resolve 请求 | 按钮 debounce 500ms + resolve 进行中禁用 + 全局互斥锁 |
| 用户连点 [收藏]/[订阅] | 重复写入 | 按钮 debounce 300ms，操作中显示 loading |
| resolve 弹窗中按 [返回] | 弹窗未关但页面已回退 | 拦截系统返回键，弹窗关闭后才允许返回 |
| 播放中快速按 [返回] | fvp controller 未 dispose | 重写 dispose()，pause() -> dispose() 顺序执行 |
| 播放中切换到其他 App | 后台继续播放占资源 | WidgetsBindingObserver 监听 paused -> 暂停 |
| 快速切换详情页 -> 播放页 | 上一个播放页未释放 | dispose() 必须在 pushReplacement 前完成 |

### 26.2 播放器生命周期管理

```dart
class _PlayerPageState extends State<PlayerPage> with WidgetsBindingObserver {
  VideoPlayerController? _controller;
  bool _disposed = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.paused && _controller != null) {
      _controller!.pause();
      _reportProgress();
    } else if (state == AppLifecycleState.resumed && _controller != null) {
      _controller!.play();
    }
  }

  @override
  void dispose() {
    _disposed = true;
    WidgetsBinding.instance.removeObserver(this);
    _controller?.pause();
    _controller?.dispose();
    _controller = null;
    super.dispose();
  }

  Future<void> _safeAsync(Future<void> Function() fn) async {
    if (_disposed) return;
    await fn();
  }
}
```

### 26.3 全局状态守卫

```dart
class AppState {
  static bool _isResolving = false;
  static bool get isResolving => _isResolving;
  static bool tryStartResolve() {
    if (_isResolving) return false;
    _isResolving = true;
    return true;
  }
  static void endResolve() { _isResolving = false; }
}
```

### 26.4 错误边界

- fvp 初始化失败 -> 降级为系统默认 video_player + Toast
- 播放 URL 返回 404/502/503 -> 错误页面 + [重试] + [返回详情页]
- seek 越界 -> 静默限制到 duration
- Ticket 过期（§18.3 `TICKET_EXPIRED`） -> fvp 自动调用 `/api/stream?refresh=true` 重发

### 26.5 内存保护

- 探索页海报图片：cached_network_image 限制 200MB LRU
- 详情页背景图：gaplessPlayback: true
- 播放器页面进入时释放详情页大图内存
- GridView/ListView 使用 addAutomaticKeepAlives: false

---

## 27. 启动健康检查与状态机增强

### 27.1 后端自检清单

| 检查项 | 检查方式 | 状态 |
|---|---|---|
| SQLite 数据库 | `PRAGMA integrity_check` | ok / error |
| TMDB API Key | `GET /3/configuration` | ok / not_configured / auth_failed / timeout |
| Bangumi API | `GET /v0/search/subjects?limit=1` | ok / timeout / error |
| PanSou 服务 | `GET /api/health`（localhost:8888） | ok / unavailable |
| 115 网盘账号 | 检查 OAuth token 有效性 | ok / not_logged_in / expired |
| 夸克网盘账号 | 检查 cookie 有效性 | ok / not_logged_in / expired |
| 123 网盘账号 | 同上 | ok / not_logged_in / expired |
| 百度网盘账号 | 同上 | ok / not_logged_in / expired |
| 光鸭网盘账号 | 同上 | ok / not_logged_in / expired |
| NAS (LocalFs) | 检查配置路径是否可读 | ok / not_configured / not_accessible |
| 索引引擎 | 检查 `media_index` 表行数 + [v7] 扫描阶段 | file_count: N / empty / phase: A/B/C |
| 订阅引擎 | 检查活跃订阅数 | active: N / none |

### 27.2 WebSocket 健康检查协议（v7 修订：增加 capabilities 字段）

Flutter WebSocket 连接建立后，后端主动推送首条消息：

```json
{
  "type": "health_check",
  "payload": {
    "db": "ok",
    "tmdb": "ok",
    "bangumi": "ok",
    "pansearch": "ok",
    "accounts": [
      {"driver": "pan115", "status": "ok", "label": "115 网盘"},
      {"driver": "quark", "status": "expired", "label": "夸克网盘", "action": "请重新扫码登录"},
      {"driver": "pan123", "status": "not_logged_in", "label": "123 网盘", "action": "请添加账号"}
    ],
    "nas": {"status": "ok", "path": "/mnt/nas/media", "file_count": 1234},
    "index": {"total_files": 5678, "last_scan": "2026-08-08T02:00:00Z", "orphaned": 3, "nas_phase": "C", "nas_processed": 125000, "nas_total": 125000},
    "subscriptions": {"active": 5, "found": 2, "downloaded": 1},
    "capabilities": {
      "nas_available": true,
      "nas_index_complete": true,
      "pansearch_available": true,
      "logged_in_drivers": ["pan115", "quark"]
    },
    "ws": "ok",
    "version": "7.0.0",
    "server_started_at": "2026-08-09T03:15:22Z",
    "overall": "warning"
  }
}
```

`overall` 取值：
- `ok`：所有正常 -> 绿色"系统就绪"
- `warning`：非关键问题 -> 黄色"部分服务异常" + 可展开详情
- `error`：关键问题 -> 红色"系统异常" + 问题列表 + [去设置] 按钮

### 27.3 心跳

- Flutter 每 30 秒发送 ping
- 后端 90 秒未收到 ping -> 关闭连接
- Flutter 60 秒未收到 pong -> 断线重连
- 心跳超时静默处理，不在健康面板显示

### 27.4 健康状态到用户操作映射（v5 新增，v7 修订）

下表是健康面板每个状态在前端的展示和推荐操作。

| 子系统 | 状态值 | 健康 overall | UI 颜色 | 文本 | 推荐操作 |
|---|---|---|---|---|---|
| db | ok | - | 绿 | 数据库正常 | - |
| db | error | error | 红 | 数据库异常 | [查看日志] [重启服务] |
| tmdb | ok | - | 绿 | TMDB 已连接 | - |
| tmdb | not_configured | error | 红 | 未配置 TMDB API Key | [去配置]（跳转管理后台） |
| tmdb | auth_failed | error | 红 | TMDB API Key 无效 | [重新配置] |
| tmdb | timeout | warning | 黄 | TMDB 响应慢 | [重试]（功能仍可用，使用缓存） |
| bangumi | ok | - | 绿 | Bangumi 正常 | - |
| bangumi | timeout | warning | 黄 | Bangumi 不可达 | （动漫分类可能不全） |
| pansearch | ok | - | 绿 | 盘搜服务正常 | - |
| pansearch | unavailable | warning | 黄 | 盘搜服务不可用 | （播放会跳过网盘搜索，仅走磁力兜底） |
| accounts[i] | ok | - | 绿 | {label} 已登录 | - |
| accounts[i] | not_logged_in | warning | 黄 | {label} 未登录 | [去登录] |
| accounts[i] | expired | error | 红 | {label} 登录已失效 | [重新登录] |
| nas | ok | - | 绿 | NAS 可访问（{file_count} 个文件） | - |
| nas | not_configured | warning | 黄 | 未配置 NAS 路径 | [去配置] |
| nas | not_accessible | error | 红 | NAS 路径不可访问 | [检查路径] [查看日志] |
| index | empty | warning | 黄 | 索引为空（首次启动？） | [触发全盘扫描] |
| index | has_orphaned | warning | 黄 | {N} 个孤立文件 | [查看详情] [清理] |
| index | scanning | - | 蓝 | 索引中 Phase {phase}（{pct}%） | （后台任务，不阻塞） |
| subscriptions | none | - | 灰 | 无活跃订阅 | - |
| subscriptions | has_active | - | 绿 | {N} 个订阅搜寻中 | - |
| subscriptions | has_failed | warning | 黄 | {N} 个订阅已停止 | [查看详情] |
| [v7] capabilities | degraded | warning | 黄 | 部分能力受限（NAS 扫描中/盘搜不可用） | 点击查看详情 |

#### 27.4.1 overall 计算规则

```
error：包含任一 error 状态
warning：包含任一 warning 但无 error
ok：全部 ok 或 scanning/none
```

#### 27.4.2 批量操作

健康面板提供两个全局按钮：
- [一键重新检测] -> `POST /api/admin/health/refresh` -> 后端重新执行所有检查
- [修复常见问题] -> 自动重试 PanSou 连通性、刷新所有账号 token 状态

---

## 28. 启动序与恢复协议（v5 新增）

### 28.1 启动顺序（7 步强约束）

后端进程必须按以下顺序启动，任意一步失败 -> 进程退出并 `slog.Error` + exit code != 0：

| 步骤 | 模块 | 阻塞/异步 | 失败策略 |
|---|---|---|---|
| 1 | 加载 configs（缺失用 ConfigDefaults 补） | 阻塞 | 退出（configs 表不存在时） |
| 2 | 打开 SQLite + 执行 pending migrations | 阻塞 | 退出（DB 损坏时） |
| 3 | 启动 EventBus + 订阅 FileMutated 事件 | 异步 | 退出 |
| 4 | 启动 ResolveTask 恢复协程（§28.2） | 异步 | 仅日志，HTTP 继续启动 |
| 5 | 启动索引引擎（NAS 首次扫描） | 异步 | 仅日志，HTTP 继续启动 |
| 6 | 启动 WebSocket Hub + HTTP 监听 | 阻塞 | 退出 |
| 7 | 启动 PanSou 健康检查（5s 间隔） | 异步 | 标记 pansearch=unavailable |

**总结**：
- configs 加载、SQLite/migrations、EventBus、WebSocket+HTTP 监听失败 -> 进程退出
- ResolveTask 恢复、索引引擎启动失败 -> 仅记日志，允许 HTTP 继续启动
- PanSou 健康检查失败 -> 仅标记 unavailable

### 28.2 ResolveTask 启动恢复

启动时查询：

```sql
SELECT * FROM resolve_tasks WHERE status IN ('pending','running')
```

恢复策略：
- `pending`：标记 failed（用户已断开）
- `running` 且 `stage IN (resolve_start, nas_lookup, pan_searching, transferring, resolving_link)`：标记 failed（断点不可恢复）
- `running` 且 `stage IN (magnet_downloading)`：调用 115 OfflineDownloader.GetOfflineTaskStatus()
  - `completed`：推进 stage=resolving_link -> 解析直链 -> 写 media_index -> 标记 done
  - `downloading`：保留 running 状态，继续轮询
  - `failed/cancelled`：标记 failed

恢复操作必须用单事务批量更新 + 加锁：

```sql
BEGIN IMMEDIATE;
UPDATE resolve_tasks SET status='failed', error_msg='服务重启中断'
  WHERE status='pending'
  OR (status='running' AND stage NOT IN ('magnet_downloading'));
-- 处理 magnet 任务
UPDATE resolve_tasks SET status='failed', error_msg='服务重启中断'
  WHERE status='running' AND stage='magnet_downloading'
  AND id NOT IN (...in_progress_task_ids);
COMMIT;
```

### 28.3 客户端感知重启

WebSocket 重连后调用 `GET /api/state/snapshot`，响应新增字段：

```json
{
  "server_started_at": "2026-08-09T03:15:22Z",
  "last_restart_reason": "config_change|oom|panic|graceful"
}
```

Flutter 端对比上次记录：若 `server_started_at` 变化 -> 显示通知"后端已重启，正在重新加载" + 强制刷新所有页面。

### 28.4 优雅退出（SIGTERM）

收到 SIGTERM 时：

1. 停止接收新 Resolve 请求（HTTP handler 返回 503）
2. 等待进行中 HTTP 请求完成（30s 超时）
3. 关闭 WebSocket Hub（发送 `server_stopping` 消息后关闭）
4. 标记所有 `running` ResolveTask 为 pending（下次启动恢复）
5. 关闭 SQLite
6. exit 0

WS `server_stopping` payload：
```json
{
  "type": "server_stopping",
  "payload": {
    "reason": "graceful",
    "grace_period_sec": 30
  }
}
```

Flutter 收到后：暂停播放器 + 提示用户"服务器正在关闭"。

---

## 附录 A: 项目目录结构（最终态，v7 修订）

```
D:\CodelfWorkspace\x-media\
├── cmd\xmedia\main.go              ← 入口
├── internal\
│   ├── account\                    ← 网盘账号管理
│   ├── adminauth\                  ← 管理鉴权
│   ├── api\                        ← HTTP 路由
│   │   ├── tmdb.go                 ← [新] TMDB 代理端点
│   │   ├── resolve.go              ← [新] 播放引擎端点（含 v7 并发限流）
│   │   ├── stream.go               ← [新] Stream 代理端点（ticket）
│   │   ├── media.go                ← [新] 媒体库端点（含 v7 继续观看/搜索历史/availability）
│   │   ├── capabilities.go         ← [v7 新增] 能力预检端点
│   │   ├── index_admin.go          ← [新] 索引管理端点
│   │   ├── pansearch_admin.go      ← [新] 盘搜配置端点
│   │   ├── files.go                ← 文件管理
│   │   ├── accounts.go             ← 账号管理
│   │   ├── offline_download.go     ← 离线下载
│   │   ├── settings.go             ← 系统设置
│   │   └── health.go               ← 健康检查
│   ├── app\                        ← 应用装配（含 7 步启动序 §28）
│   │   └── startup.go              ← [v5 新增] 启动序实现
│   ├── automation\                 ← 订阅调度器
│   ├── cache\                      ← 内存缓存
│   ├── cacheretention\             ← 缓存保鲜
│   ├── config\                     ← 配置
│   ├── core\                       ← 驱动执行器
│   ├── crosstransfer\              ← 跨盘转存（同盘移动/复制）
│   ├── domain\                     ← 领域模型
│   │   ├── account.go
│   │   ├── file.go
│   │   ├── media_index.go          ← [新]
│   │   ├── media_library.go        ← [新]（含 v7 last_accessed_at）
│   │   ├── play_history.go         ← [新]（含 v7 season/episode）
│   │   ├── subscription.go         ← [新] 含 result_account_id
│   │   ├── resolve_task.go         ← [新] 含 result_file_path/result_account_id
│   │   ├── pansearch_result.go     ← [新]
│   │   ├── config_keys.go          ← [新] 含 v7 rate_limit/media_library 配置
│   │   ├── capabilities.go         ← [v7 新增] Capabilities 模型
│   │   └── repository.go
│   ├── driver\                     ← 驱动接口
│   │   ├── driver.go               ← Driver + ShareSaver + OfflineDownloader + Rename 接口
│   │   └── qrlogin.go              ← QRLoginProvider 接口
│   ├── eventbus\                   ← 事件总线
│   ├── file\                       ← 文件列表
│   ├── httpx\                      ← HTTP 工具
│   ├── indexengine\                ← [新] 索引引擎
│   │   ├── service.go
│   │   ├── nas_index.go            ← 含 §9.7 三阶段扫描 + v7 P0 跳过检查
│   │   ├── pan_index.go
│   │   ├── matcher.go
│   │   └── cleanup.go              ← 配额清理 §6.9.3
│   ├── logx\                       ← 日志（slog 结构化 §21.4）
│   ├── media\                      ← [新] 媒体库
│   │   ├── service.go
│   │   ├── history.go
│   │   ├── favorite.go
│   │   ├── continue_watching.go    ← [v7 新增]
│   │   └── search_history.go       ← [v7 新增]
│   ├── notification\               ← 通知（含配额预警 §6.9.3）
│   ├── offlinedownload\            ← 离线下载
│   ├── pansearch\                  ← [新] PanSou HTTP 客户端
│   │   ├── service.go
│   │   ├── client.go
│   │   └── filter.go               ← 含 v7 缓存失效逻辑
│   ├── playback\                   ← 播放供流 + Ticket 代理
│   │   ├── service.go
│   │   ├── ticket.go               ← [新] Ticket 生成/验证
│   │   └── stream.go               ← [新] Stream 代理 handler
│   ├── resolve\                    ← [新] 四层播放引擎
│   │   ├── service.go
│   │   ├── rate_limiter.go         ← [v7 新增] 并发限流
│   │   ├── nas_layer.go            ← P0（含 v7 智能跳过）
│   │   ├── pan_layer.go            ← P1（含 §6.9 转存路径 + v7 多语言关键词）
│   │   ├── magnet_layer.go         ← P2（关键词构造见 §6.4 + v7 后台行为）
│   │   ├── notfound_layer.go       ← P3
│   │   └── recovery.go             ← [v5 新增] 启动恢复 §28.2
│   ├── settings\                   ← 设置
│   ├── store\                      ← SQLite
│   │   ├── media_index_repo.go     ← [新]
│   │   ├── media_library_repo.go   ← [新]（含 v7 LRU 清理）
│   │   ├── play_history_repo.go    ← [新]（含 v7 继续观看查询）
│   │   ├── favorite_repo.go        ← [新]
│   │   ├── subscription_repo.go    ← [新]
│   │   ├── pansearch_cache_repo.go ← [新]（含 v7 link_count 更新）
│   │   ├── resolve_task_repo.go    ← [新]
│   │   ├── search_history_repo.go  ← [v7 新增]
│   │   └── rate_limit_repo.go      ← [v7 新增]
│   ├── tmdb\                       ← [新] TMDB 代理
│   │   ├── service.go
│   │   ├── tmdb_client.go
│   │   └── bangumi_client.go
│   ├── upload\                     ← 上传
│   └── websocket\                  ← [新] WebSocket
│       ├── hub.go
│       ├── client.go
│       └── messages.go             ← 含 v7 capabilities/server_started_at/server_stopping
├── drivers\
│   ├── 115_Open\                   ← 115（OAuth 认证，无扫码）
│   │   ├── auth.go
│   │   ├── share.go                ← [新] ShareSaver 实现
│   │   ├── offline.go              ← [新] OfflineDownloader 实现
│   │   └── rename.go               ← [v5 新增] 转存重命名 §6.9.2
│   ├── Quark\                      ← 夸克（已有扫码）
│   │   ├── qrlogin.go
│   │   ├── share.go                ← [新] ShareSaver 实现
│   │   └── rename.go               ← [v5 新增]
│   ├── 123_Open\
│   │   ├── share.go                ← [新] ShareSaver 实现
│   │   └── rename.go               ← [v5 新增]
│   ├── Baidu_Open\
│   │   ├── share.go                ← [新] ShareSaver 实现
│   │   └── rename.go               ← [v5 新增]
│   ├── Guangya\
│   │   ├── share.go                ← [新] ShareSaver 实现
│   │   └── rename.go               ← [v5 新增]
│   ├── LocalFs\                    ← NAS 本地文件
│   └── all.go
├── web\                            ← Vue3 管理后台
│   └── src\
│       └── views\
│           ├── AdminView.vue       ← 扩展管理页面
│           ├── HealthPanel.vue     ← [v5 新增] 健康状态映射 §27.4
│           ├── TransferSettings.vue← [v5 新增] 转存路径/配额配置 §6.9
│           └── PriorityConfig.vue  ← [v5 新增] 网盘优先级（NAS 可关闭 + v7 登录校验）§11.1
├── pkg\
├── go.mod
├── Dockerfile
└── docker-compose.yml              ← X-MEDIA + PanSou 双服务
```

---

## 附录 B: 关键设计决策记录（v7 新增 D42-D57）

| # | 决策 | 理由 | 日期 |
|---|---|---|---|
| D1 | 摒弃 gRPC，改用 HTTP + WebSocket | 消除 Stream 闪断/证书/代码生成摩擦 | 2026-08-07 |
| D2 | 摒弃 libmpv，改用 fvp | 解决 DV 色彩问题 | 2026-08-07 |
| D3 | 基于 LitePan 改造而非从零开发 | 复用成熟的驱动/缓存/转存/调度基础设施 | 2026-08-07 |
| D4 | TMDB 元数据驱动而非刮削 | 项目灵魂：先看元数据再找文件 | 2026-08-07 |
| D5 | 保留 Vue Web 管理后台 | 多用户场景，后端改动不要求 Flutter 重编译 | 2026-08-07 |
| D6 | 保留 cacheretention 模块 | 网盘目录列表缓存保鲜，减少扫盘防风控 | 2026-08-07 |
| D7 | 不跨盘秒传，只同盘分享转存 | 跨网盘 hash 算法不同，物理上无通用方案 | 2026-08-07 |
| D8 | 索引引擎作为第一公民 | 减少扫盘 = 减少风控，索引命中即秒播 | 2026-08-07 |
| D9 | 网盘优先级/清理策略用户可配 | 不硬编码任何用户偏好，不同用户主盘不同 | 2026-08-07 |
| D10 | 磁力兜底走 115 云下载 + 订阅机制 | 非秒下载，用户需有预期 | 2026-08-07 |
| D11 | 盘搜内嵌 Library 模式 | ~~已修正~~ -> 见 D24 | 2026-08-07 |
| D12 | 历史记录同一 external_id 只保留最后一条 | 避免历史列表出现重复卡片；[v7 修正] 扩展到季集维度 | 2026-08-07 |
| D13 | 动漫走 Bangumi API | TMDB 中文动漫覆盖不如 Bangumi | 2026-08-07 |
| D14 | 保留 5 个网盘驱动 | 115/夸克/123/百度/光鸭 | 2026-08-07 |
| D15 | WebSocket 指数退避重连 + HTTP 补刷 | TV 端 WiFi 不稳定场景 | 2026-08-07 |
| D16 | 外部媒体库作为可选补充模块（v1.1） | 不影响主体 TMDB 驱动流程，严格隔离 | 2026-08-07 |
| D17 | 字幕自动搜索使用 opensubtitles.com（v1.1） | 最大字幕库，稳定 REST API | 2026-08-07 |
| D18 | 字幕本地索引复用 | 同 ID+季+集只下载一次 | 2026-08-07 |
| D19 | TDD 全流程强制 | 先写测试再编码，防止"跑通特例=完成" | 2026-08-07 |
| D20 | WebSocket 启动首条消息为 health_check | 用户启动 App 第一时间知道系统状态 | 2026-08-07 |
| D21 | Flutter 前端 UI 布局为暂定结构 | 视觉设计将另行产出 | 2026-08-07 |
| D22 | 所有元数据和图片仅来源于 TMDB + Bangumi | 不使用豆瓣/IMDb 等第三方数据源 | 2026-08-07 |
| D23 | 115 认证采用 OAuth，不移植扫码 | LitePan 115_Open 驱动已有成熟 OAuth 实现，扫码 Cookie 维护成本高 | 2026-08-08 |
| D24 | PanSou 以 Sidecar HTTP 模式运行，不内嵌为 Go 包 | PanSou 是完整服务（TG 爬虫+插件系统+二级缓存），拆包嵌入成本远大于收益 | 2026-08-08 |
| D25 | 分享转存通过 ShareSaver 驱动接口实现，不用 crosstransfer | crosstransfer 是跨盘 hash 秒传，分享转存是调用网盘"保存分享"API，两者完全不同 | 2026-08-08 |
| D26 | Ticket 机制：HMAC 签名 token + /api/stream 代理端点 | 真实直链永不出后端，安全且支持自动刷新 | 2026-08-08 |
| D27 | tmdb_id 改为 external_id + external_source | 避免动漫 Bangumi ID 与电影 TMDB ID 冲突 | 2026-08-08 |
| D28 | 磁力链接来源复用 PanSou 的 magnet/ed2k 搜索 | 不引入额外 BT 搜索 API，减少外部依赖 | 2026-08-08 |
| **D29** | **明确 v1.0 不支持多实例** | WebSocket Hub / L1 内存缓存 / ResolveTask 状态机均单进程设计，文档首声明避免误判 | 2026-08-09 |
| **D30** | **驱动命名三层规范** | 包路径 / DB source_type / PanSou cloud_type 三层命名混用导致代码不一致，统一约束 | 2026-08-09 |
| **D31** | **启动序 7 步强约束** | 启动顺序决定系统能否上线，启动失败策略统一 | 2026-08-09 |
| **D32** | **ResolveTask 按 stage 分级恢复** | pending/running 非 magnet 阶段直接 failed；magnet_downloading 调用 115 接口查询后按状态分流，避免断点不可恢复导致的用户困惑 | 2026-08-09 |
| **D33** | **7 步启动顺序强约束（configs -> DB -> EventBus -> ResolveTask 恢复 -> 索引引擎 -> WS+HTTP -> PanSou 健康检查）** | 启动顺序决定模块依赖关系；DB 没准备好前不能启动 EventBus/索引引擎；HTTP 监听前必须先启动 WS Hub | 2026-08-09 |
| **D34** | **SIGTERM 优雅退出协议** | 容器编排（k8s/docker-compose down）会发送 SIGTERM；必须留出 30s 让 HTTP 请求收尾，否则用户正播放中会断流 | 2026-08-09 |
| **D35** | **每个网盘账号独立的 save_root_folder_id + 配额预警阈值** | 不配置 save_root 会导致转存到网盘根目录污染用户数据；不预警配额会导致转存失败但用户不知道为什么 | 2026-08-09 |
| **D36** | **转存后重命名规则用户可关** | 115 转存后原文件名通常带广告或分辨率标识，重命名可改善观感；但部分用户希望保留原文件名（如下到收藏），故提供开关 | 2026-08-09 |
| **D37** | **NAS 扫描分三阶段（路径发现 -> 元数据提取 -> 孤儿标记）** | 100TB 全盘扫描不能阻塞 HTTP；元数据提取是 IO+CPU+网络混合，必须用 worker pool；孤儿标记独立可中断 | 2026-08-09 |
| **D38** | **NAS 扫描进度通过 WS index_status 实时推送** | 用户看到 100TB 扫描数小时必须有进度反馈，否则以为卡死；进度格式含 phase/processed/total/matched/rate | 2026-08-09 |
| **D39** | **健康状态每个 status 必须映射到前端具体操作按钮** | 健康面板不能只是装饰；status=error 必须有 [查看日志]/[重启服务]/[去配置] 等具体按钮引导用户 | 2026-08-09 |
| **D40** | **Day 1 启动旅程作为编码前自检清单** | 文档全是技术细节时容易漏掉用户真实路径；首次启动必须能跑通：部署 -> 配置 -> 添加账号 -> 播放 | 2026-08-09 |
| D41 | v6 = v4 + v5 单文件合并 | 用户阅读偏好单文件独立完整版；v4 和 v5 保留为历史归档 | 2026-08-09 |
| **D42** | **[v7] 新增「继续观看」作为首页第一行** | 媒体 App 最高频入口；play_history 已有数据基础；Netflix/Emby/Plex 均将 Resume 放首屏 | 2026-08-09 |
| **D43** | **[v7] 新增搜索页** | §12.1 已有搜索 API 但 §4.1 无对应页面，属于功能缺口；TMDB 搜索结果为空时引导直接 PanSou 盘搜 | 2026-08-09 |
| **D44** | **[v7] 新增 GET /api/capabilities 能力预检端点** | 前端需在用户点击播放前就知道系统能力（NAS 是否就绪/哪些盘已登录/PanSou 是否可用），据此调整 UI 和跳过无意义操作 | 2026-08-09 |
| **D45** | **[v7] P0 查询前增加智能跳过检查** | NAS 扫描中/未配置/空索引时 P0 必然 miss，跳过避免无意义延迟；Resolve Modal 不再闪现 nas_lookup 阶段 | 2026-08-09 |
| **D46** | **[v7] 多语言搜索关键词回退链（中文 -> 原文 -> 混合）** | PanSou TG 频道资源常用英文/罗马音命名；单中文搜索命中率低；多关键词回退提升 P1 成功率 | 2026-08-09 |
| **D47** | **[v7] Resolve Modal 增加分层进度指示器** | 纯文字等待 3-12s 用户心理感知差；四层步骤条让用户知道进度 + 剩余层数，降低焦虑 | 2026-08-09 |
| **D48** | **[v7] P2 磁力下载明确后台行为** | 分钟~小时级操作不能阻塞用户；关闭 Modal/退出 App 后下载继续，下次打开自动接入已有任务 | 2026-08-09 |
| **D49** | **[v7] PanSou 缓存增加 link_count 失效机制** | 缓存中所有链接失效后，1 小时内后续请求仍命中坏缓存；link_count=0 + 30 分钟阈值触发强制重搜 | 2026-08-09 |
| **D50** | **[v7] 网盘优先级保存时校验已登录状态** | 用户拖拽的网盘可能未登录，运行时跳过但保存时不提示会导致困惑；保存时前端 ⚠️ 提示 + 运行时跳过 | 2026-08-09 |
| **D51** | **[v7] Resolve 并发限流（30s 内最多 3 次）** | 防止恶意/意外高并发触发网盘 API 风控；DB 故障时降级放行保可用 | 2026-08-09 |
| **D52** | **[v7] media_library 增加 LRU 淘汰策略** | 用户浏览过的所有元数据永久保留导致 DB 膨胀；5000 行阈值 + 收藏/订阅/播放记录保护 + LRU 淘汰 | 2026-08-09 |
| **D53** | **[v7] 探索页/详情页增加可播放 ✓ 角标** | 用户浏览时无法区分已缓存（秒播）和未缓存（需搜索 3-12s）的内容；通过 check-availability API 批量查询 + 视觉区分 | 2026-08-09 |
| **D54** | **[v7] 电视剧季集列表显示 availability** | 用户点击某集前不知道是否有资源，导致"点击 -> 12s 等待 -> not_found"挫败；已索引集绿色 ✓ 角标，未索引集无角标 | 2026-08-09 |
| **D55** | **[v7] 全页面骨架屏替代纯 spinner** | CircularProgressIndicator 在 TV 大屏上体验差；shimmer 骨架屏模拟内容布局减少感知等待 | 2026-08-09 |
| **D56** | **[v7] 引入 Design Token 系统（附录 C）** | 文档中散落的颜色/间距/圆角值不一致；Flutter + Vue 双端需要共享设计变量确保视觉一致 | 2026-08-09 |
| **D57** | **[v7] v7 = v6 + UX 8 项 + TECH 8 项全量落地** | 架构审查 16 项全部采纳；工期调整至 70-90 天 | 2026-08-09 |

---

## 附录 C: Design Token 设计系统（v7 新增）

> 本附录定义 X-MEDIA 的跨平台设计变量。Flutter 端和 Vue 管理后台共享同一套 Token 定义，确保视觉一致。

### C.1 颜色系统

```yaml
# Light Mode（默认）
colors:
  primary:        "#6E6CF0"    # --accent，主色调（紫蓝）
  primary_hover:  "#5B59D4"    # 主色调悬停
  primary_bg:     "#F0EFFF"    # 主色调浅底

  surface:        "#FFFFFF"    # 卡片背景
  surface_hover:  "#F5F5FF"    # 卡片悬停
  background:     "#F8F9FC"    # 页面背景
  sidebar:        "#1A1A2E"    # 侧栏背景（Material 实心填充）

  text_primary:   "#1A1A2E"    # 主文字
  text_secondary: "#6B7280"    # 次要文字
  text_muted:     "#9CA3AF"    # 弱化文字
  text_on_dark:   "#FFFFFF"    # 深色背景上的文字

  success:        "#22C55E"    # --green, 成功/可播放标识
  warning:        "#F59E0B"    # --yellow, 警告
  error:          "#FF4D4D"    # --red, 错误
  info:           "#00E5FF"    # --cyan, 信息/进度
  skeleton:       "#E5E7EB"    # 骨架屏灰色

  border:         "#E5E7EB"    # 边框
  divider:        "#F3F4F6"    # 分割线

  # 语义色别名（指向基础色）
  resolve_p0:     "#22C55E"    # P0 NAS 层（绿色，秒播）
  resolve_p1:     "#6E6CF0"    # P1 盘搜层（紫色）
  resolve_p2:     "#F59E0B"    # P2 磁力层（黄色）
  resolve_p3:     "#9CA3AF"    # P3 订阅层（灰色）
```

### C.2 间距系统

```yaml
spacing:
  xs:   4px
  sm:   8px
  md:   12px
  lg:   16px
  xl:   24px
  2xl:  32px
  3xl:  48px
```

### C.3 圆角

```yaml
radius:
  none:   0px
  sm:     4px
  md:     8px
  lg:     12px     # 默认卡片圆角
  xl:     16px
  full:   9999px   # 胶囊/圆形
```

### C.4 毛玻璃效果

```yaml
glass:
  blur:       12px
  opacity:    0.85
  bg:         "rgba(255, 255, 255, 0.85)"
```

### C.5 字体层级

```yaml
typography:
  family:
    display:  "Inter, SF Pro Display, system-ui"
    body:     "Inter, SF Pro Text, system-ui"
    mono:     "JetBrains Mono, Fira Code, monospace"

  size:
    xs:     11px      # 角标/标签
    sm:     13px      # 辅助文字
    base:   15px      # 正文
    lg:     18px      # 小标题
    xl:     22px      # 标题
    2xl:    28px      # 大标题
    3xl:    36px      # 页面标题

  weight:
    regular:  400
    medium:   500
    semibold: 600
    bold:     700
```

### C.6 阴影层级

```yaml
elevation:
  none:    "none"
  sm:      "0 1px 2px rgba(0,0,0,0.06)"
  md:      "0 4px 6px rgba(0,0,0,0.08)"
  lg:      "0 10px 25px rgba(0,0,0,0.1)"
  xl:      "0 20px 50px rgba(0,0,0,0.15)"
  modal:   "0 25px 60px rgba(0,0,0,0.2)"
```

### C.7 动画

```yaml
animation:
  duration:
    fast:     150ms
    normal:   250ms
    slow:     400ms
  easing:
    default:  "cubic-bezier(0.4, 0, 0.2, 1)"     # ease-in-out
    decelerate: "cubic-bezier(0.0, 0, 0.2, 1)"   # ease-out
    accelerate: "cubic-bezier(0.4, 0, 1, 1)"      # ease-in
  skeleton_shimmer:
    duration: 1500ms
    gradient: "linear-gradient(90deg, #E5E7EB 0%, #F3F4F6 50%, #E5E7EB 100%)"
```

### C.8 Flutter 实现

```dart
// lib/theme/design_tokens.dart  [v7 新增]

class AppColors {
  static const primary = Color(0xFF6E6CF0);
  static const primaryHover = Color(0xFF5B59D4);
  static const surface = Color(0xFFFFFFFF);
  static const background = Color(0xFFF8F9FC);
  static const textPrimary = Color(0xFF1A1A2E);
  static const textSecondary = Color(0xFF6B7280);
  static const success = Color(0xFF22C55E);
  static const warning = Color(0xFFF59E0B);
  static const error = Color(0xFFFF4D4D);
  static const info = Color(0xFF00E5FF);
  static const skeleton = Color(0xFFE5E7EB);
}

class AppSpacing {
  static const xs = 4.0;
  static const sm = 8.0;
  static const md = 12.0;
  static const lg = 16.0;
  static const xl = 24.0;
  static const xxl = 32.0;
}

class AppRadius {
  static const sm = 4.0;
  static const md = 8.0;
  static const lg = 12.0;
  static const xl = 16.0;
}

class AppTextSize {
  static const xs = 11.0;
  static const sm = 13.0;
  static const base = 15.0;
  static const lg = 18.0;
  static const xl = 22.0;
  static const xxl = 28.0;
}

class AppDuration {
  static const fast = Duration(milliseconds: 150);
  static const normal = Duration(milliseconds: 250);
  static const slow = Duration(milliseconds: 400);
}
```

### C.9 Vue 管理后台实现

```css
/* web/src/assets/tokens.css  [v7 新增] */
:root {
  --color-primary: #6E6CF0;
  --color-surface: #FFFFFF;
  --color-background: #F8F9FC;
  --color-text-primary: #1A1A2E;
  --color-text-secondary: #6B7280;
  --color-success: #22C55E;
  --color-warning: #F59E0B;
  --color-error: #FF4D4D;
  --color-info: #00E5FF;
  --color-skeleton: #E5E7EB;

  --radius-sm: 4px;
  --radius-md: 8px;
  --radius-lg: 12px;

  --spacing-sm: 8px;
  --spacing-md: 12px;
  --spacing-lg: 16px;
  --spacing-xl: 24px;

  --glass-blur: 12px;

  --font-family: "Inter", "SF Pro Display", system-ui;
  --font-size-sm: 13px;
  --font-size-base: 15px;
  --font-size-lg: 18px;

  --shadow-md: 0 4px 6px rgba(0,0,0,0.08);

  --transition-normal: 250ms cubic-bezier(0.4, 0, 0.2, 1);
}
```

---

*文档结束。本文档为 X-MEDIA 项目的权威设计参考，编码时以此为准。v7 是 v6 + 架构审查 16 项全量落地版，独立完整；v4/v5/v6 保留作为历史归档在 git 中查阅。*
