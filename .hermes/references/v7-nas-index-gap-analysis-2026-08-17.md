# X-MEDIA V7 NAS 多媒体源 / 索引 / 清理策略 — Gap 分析
> 状态：2026-08-17 V7 全链路精读后编写
> 作者：Hermes Agent（基于 V7 文档 §6/§9/§11/§16/§28 实读）
> 用途：新会话接手 NAS / 索引 / 转存相关工作时**先读本档案**，避免重蹈"猜设计"覆辙

---

## 1. V7 设计意图（不可改写为代码假设的"为什么"）

### 1.1 索引的双重目的（V7 §6.8.3）

`media_index` 表**不只是一个文件列表**——它在播放链路上有两个明确目的：

| 目的 | 描述 | 实现 | 适用 |
|---|---|---|---|
| **加速** | 路径索引命中 → 直接读 | `media_index.file_path` + `media_index.external_id` 等字段 | **所有 source_type** |
| **减少网盘风控** | **网盘直链缓存 + 过期刷新**——避免每次播放都调网盘 API | `media_index.stream_url` + `media_index.url_expires` | **仅网盘 source_type** |

**关键证据（V7 §6.8.3 Stream 代理处理流程）**：

```text
网盘（source=pan115/quark/...）：
  a. 从 media_index 查询缓存的直链 URL          ← 目的2 命中
  b. 直链未过期 -> 302 重定向到真实直链
  c. 直链已过期 -> 调用 driver.GetFile() 刷新直链
     -> 更新 media_index.stream_url
  d. 刷新失败 -> 502 Bad Gateway

NAS（source=nas）：
  直接读取本地文件 -> 代理 Range 请求          ← 目的1 命中（无目的2）
```

### 1.2 NAS 索引 vs 网盘索引的本质区别

| 维度 | NAS (LocalFs) | 网盘 (pan115/quark/...) |
|---|---|---|
| **索引字段** | `file_path`（本地路径） | `file_path` + `stream_url` + `url_expires`（直链缓存） |
| **直链来源** | 本地文件系统直接读 | **网盘 API 实时获取** |
| **直链有效期** | 永久（路径不变） | **短时**（115:2h, 夸克: 1h, §6.8.1） |
| **目的1: 加速** | ✅ O(1) 路径定位 | ✅ 命中直接 302 重定向 |
| **目的2: 减少风控** | ❌ 不需要（本地） | ✅ **核心目的**——避免每次播放都调网盘 API |
| **增删触发** | 文件名变更 → Upsert；文件删除 → DeleteBySourcePath | **网盘事件**（转存完成 → 调 GetFile 拿直链 → Upsert stream_url）；直链过期 → 刷新 |

**写入策略**：
- NAS 扫描 → 只写路径相关字段（`file_path` / `file_size` / `file_format` / `match_status` / `match_score`）
- 网盘扫描/事件 → 上述 + `stream_url` + `url_expires` + `file_id` + `account_id`

### 1.3 不同索引有不同增删策略（V7 §11.2 + §9.5）

| source_type | 清理频率 | 清理范围 | 实现位置 |
|---|---|---|---|
| **NAS** | 增量扫描（§9.7.4 每周）+ Phase D（消失文件检测）+ §9.7.5 月度校验（默认关闭） | 仅删除**索引条目**（文件本身不动——OS 层管理） | `internal/indexengine/service.go:scanNAS` |
| **115** | 通常不清理（容量大如 VIP 28TB） | 用户手动删除 → 索引自动修正（§9.3 eventbus） | 用户操作触发 |
| **夸克** | 每周清理转存目录所有文件 + 重建索引 | 删除网盘文件 + 删除索引条目 + 跳过最近 2h 播放过的（§9.5 `last_played_at` 保护） | `internal/indexengine/service.go:Cleanup`（含 driver.DeleteFile） |

**关键**：`Cleanup(ctx, sourceType, accountID)` 当前实现是**网盘清理逻辑**（含 driver.DeleteFile）。**NAS 路径不走 Cleanup，走 §9.7.4 增量扫描的 Phase D**——两者分离。

### 1.4 NAS 扫描三阶段（V7 §9.7.1）

```text
Phase A：路径发现（秒级）
  - 递归遍历 nas_local_path，产出候选文件路径（不读内容）
  - 仅写路径到内存队列
  - 用户 HTTP 健康检查可立即收到 nas_scan: phase=A, total_files=125000

Phase B：元数据提取（分钟~小时）
  - worker pool（默认 8 worker，可配 nas_scan_worker_count）
  - 每个 worker: 读取文件大小 + 文件名清洗 + 调用 media_library 查询 TMDB
  - 批量写 media_index（每 1000 条一次事务）
  - 进度通过 WS index_status 推送

Phase C：孤儿标记（秒级）
  - 标记 match_status='unconfirmed' 中超过 30 天的为 orphaned
  - 不删除，仅状态变化

Phase D（增量扫描专属）：消失文件清理
  - 删除 media_index 中实际已不存在的 file_path
```

### 1.5 智能跳过 P0（V7 §6.3）

```text
在执行 P0 查询前，引擎先检查 index_status：
- NAS 处于 Phase A/B 扫描中 -> 跳过 P0，直接进入 P1
  （索引不完整，查询必然 miss）
- NAS 未配置（nas.status = "not_configured"）-> 跳过 P0
- NAS 索引为空（index.total_files = 0）-> 跳过 P0
- 仅当 NAS 已配置 + 可用 + 索引完成时，才执行 P0 查询
```

跳过 P0 时 Resolve Modal 阶段不显示 `nas_lookup`，直接显示 `pan_searching`。

### 1.6 启动顺序 7 步（V7 §28.1）

```text
1. 加载 configs（缺失用 ConfigDefaults 补）       阻塞，失败退出
2. 打开 SQLite + 执行 pending migrations          阻塞，失败退出
3. 启动 EventBus + 订阅 FileMutated 事件          异步，失败退出
4. 启动 ResolveTask 恢复协程（§28.2）              异步，仅日志
5. 启动索引引擎（NAS 首次扫描）                    异步，仅日志    ← P0 改造点
6. 启动 WebSocket Hub + HTTP 监听                 阻塞，失败退出
7. 启动 PanSou 健康检查（5s 间隔）                异步，仅标记 unavailable
```

NAS 首次扫描**必须自动启动**（§28.1 步骤5），**不是手动触发**。当前实现是手动触发——gap。

### 1.7 健康检查状态机（V7 §27.4）

```text
nas 状态值：
  ok              绿  NAS 可访问（{file_count} 个文件）
  not_configured  黄  未配置 NAS 路径     [去配置]
  not_accessible  红  NAS 路径不可访问   [检查路径] [查看日志]
```

当前 capabilities.nas 是**二态 boolean**（`nas_available: false`），**不是三态**——gap。

### 1.8 多媒体源扩展（V7 §9.4+）

V7 §9.4 仅明示**单字符串 `nas_local_path`**。**多挂载点动态增删是 V7 扩展需求**（用户实情：TrueNAS 25.10.13 已挂载 SMB 共享，需 admin 界面增删子目录）。

实现方案：
- 容器 bind mount 单根目录 `${NAS_MEDIA_PATH:-...}:/mnt/nas-root:ro`
- admin 界面动态增删"容器内子路径"作为媒体源
- 后端 `nas_local_paths` JSON 数组（兼容旧 `nas_local_path` 单字符串）

---

## 2. Gap 总表（实现 vs V7）

> 严重度：**P0** = 影响播放链路核心 / **P1** = 影响 UX + 索引完整性 / **P2** = 增强

| # | gap | V7 条款 | 现状 | 严重度 | 修复位置 |
|---|---|---|---|---|---|
| **G3** | C3 commit message 标 NAS 不写 stream_url | §6.8.3 NAS 分支 | ✅ C3 已 amend（commit `4bbfd29`） | OK | — |
| **G7** | resolve 引擎 P0 智能跳过 | §6.3 | 需 audit resolve.go | **P0** | `internal/resolve/engine.go` |
| **G8** | playback/stream.go NAS 分支直读 | §6.8.3 NAS 分支 | 需 audit stream.go | **P0** | `internal/playback/stream.go` |
| **G13** | eventbus.FileMutated → media_index 增量 | §9.3 | 需 audit indexengine + eventbus | **P0** | `internal/indexengine/event.go` (新建) |
| **G14** | 启动时自动 NAS 首次扫描 | §28.1 步骤5 | 当前手动 | **P0** | `internal/app/wire_xmedia.go` |
| **G15** | stream 时更新 last_played_at | §9.5 | 需 audit playback | **P0** | `internal/playback/service.go` |
| **G16** | Stream 完整流程 audit | §6.8.3 流程图 | 需 audit | **P0** | 同 G8 |
| **G1** | NAS 媒体源 CRUD（多挂载点） | §9.4 扩展 | C3 已实现解析，**API/UI 待做** | **P1** | C4 |
| **G2** | NAS 路径 vs 媒体源分离 | §9.4 扩展 | C1 已改 compose bind mount | **P1** | C4 |
| **G4** | NAS 自动增量扫描（cron） | §9.7.4 | 未实现 | **P1** | C5 |
| **G11** | nas 三态健康 | §9.4 + §27.4 | 当前二态 boolean | **P0** | C4 改造 capabilities |
| **G10** | 月度全盘校验 | §9.7.5 + §11.3 | 未实现 | **P2** | C5 后置 |
| **G18** | Vue UI "媒体配置 → NAS" 改造 | §12.2 Phase 8 | 当前单输入框 | **P1** | C7 |
| **G5** | NAS 匹配阈值 | §9.2 (0.85/0.6) | ✅ 已实现 | OK | — |
| **G6** | Phase A/B/C/D 实现 | §9.7.1 | ✅ 已实现 | OK | — |
| **G9** | nas_index_incremental_day 配置 | §9.7.4 + §11.3 | ✅ 配置键已定义 | OK | — |
| **G12** | resolve 引擎索引命中 SQL | §6.3 | ✅ 已实现 | OK | — |
| **G17** | nas_root_path 配置键 | §9.4 | ✅ C2 已加白名单 | OK | — |
| **G19** | 启动 PanSou 健康检查 | §28.1 步骤7 | ✅ 已实现 | OK | — |

**总结**：13 个 gap 待办，6 个 P0 + 4 个 P1 + 1 个 P2 + 2 个已完成。

---

## 3. P0 整改路线（按依赖顺序）

### Round A：playback stream 完整流程（独立模块）

**G8 + G15 + G16 一起做**——都属于 playback 服务，串行依赖：
1. audit 当前 `internal/playback/stream.go`
2. 对照 §6.8.3 流程图确认每个分支实现
3. 增加 `last_played_at` 更新（§9.5）— 在 stream 成功响应后异步更新
4. 新增测试覆盖 NAS 直读 + 网盘 302 + 过期刷新

### Round B：indexengine event-driven 增量（G13）

1. audit 当前 `eventbus` 订阅 + `indexengine.IndexFile` / `RemoveFile`（已存在）
2. 确认 `crosstransfer.SaveShare` / `offlinedownload.Complete` 等事件触发后调用 `IndexFile`
3. 确保直链缓存写入 `media_index.stream_url` + `url_expires`（§6.8.3 第 3 步 c）

### Round C：app 启动 + capabilities（G14 + G11）

1. `internal/app/wire_xmedia.go` 步骤5 添加自动触发 `indexEngine.ScanNASFull()`
2. `internal/app/state.go` (或 wire) capabilities.nas 三态化（not_configured/not_accessible/ok）
3. `internal/api/health.go` + `state.go` 输出三态

### Round D：resolve 智能跳过（G7）

1. audit `internal/resolve/engine.go` 看是否检查 `index_status`
2. 在 Resolve 入口加 §6.3 智能跳过逻辑
3. 测试：NAS扫描中 → P0 跳过 → 直接 P1

---

## 4. P1 整改路线（Round E 起）

### Round E：NAS 媒体源 CRUD（G1 + G2）

1. 新增 DB 表 `nas_sources` (id, name, container_path, enabled, created_at, updated_at)
   - 或存 `configs.nas_local_paths` JSON 数组（C3 已实现解析，但无 UI/API）
2. `/api/admin/nas-sources/*` CRUD
3. Vue UI "NAS 配置" tab 重写：列表 + 表单（容器内路径 + 浏览子目录）
4. capabilities.nas 三态化（与 G11 同步）

### Round F：自动 cron 增量扫描（G4）

1. `internal/cron` 或 `internal/automation` 增加 NAS 增量扫描 cron
2. 默认每周一次（`nas_index_incremental_day`）
3. WS `index_status` 推送进度

### Round G：Vue UI（G18）

1. NAS tab 列表 + 添加/编辑/删除/启停
2. 设置页集成 NAS 三态健康

---

## 5. 设计陷阱（不要犯）

1. **不要给 NAS 索引写 `stream_url`** — V7 §6.8.3 NAS 分支直读本地文件，无意义
2. **不要用同一个 `Cleanup()` 处理 NAS** — §11.2 网盘清理含 driver.DeleteFile，NAS 应走 §9.7.4 Phase D
3. **不要让 P0 在扫描中跑查询** — §6.3 智能跳过：扫描中 P0 必然 miss，应跳过直接 P1
4. **不要把"启动首次扫描"作为手动操作** — §28.1 步骤5 自动启动
5. **不要假设网盘清理逻辑适用于 NAS** — §9.5 + §11.2 + §9.7.4 是三条独立路径
6. **不要在 commit message 里省略设计意图** — 标明 V7 条款出处 + 索引两目的

---

## 6. 关联文档

- `X-MEDIA-Design-Doc-v7.md` — 设计文档权威版（V7）
- `server/internal/store/migrations/0017_media_index.sql` — media_index 表结构（22 列含 stream_url/url_expires）
- `server/internal/indexengine/` — 扫描 + 匹配实现
- `server/internal/playback/` — stream 代理实现
- `server/internal/resolve/` — 四层播放引擎
- `server/internal/app/wire_xmedia.go` — 启动顺序

---

## 7. commit message 模板（含 V7 出处）

```
feat(<scope>): <动作> [V7 §<章节>]

[V7 §X.Y 明示条款]<原文引用>

[本次实现]
- 改动 1
- 改动 2

[设计意图]
- 索引目的1（加速）：<如何实现>
- 索引目的2（减少网盘风控）：<如何实现或不适用>

quality gate:
- go build: exit 0
- go vet: exit 0
- go test: N/N PASS
- gofmt: clean

下一步: <下一战役目标>
```

---

> 文档版本：2026-08-17 v1.0
> 下次会话接手 NAS / 索引 / 转存相关工作时**先读本档案**。