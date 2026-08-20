# X-MEDIA v7 NAS 整改 — 2026-08-17 Part 1 工作纪要

> 适用：明天接手 / 复盘 / 跨会话延续
> 状态：当天工作暂停，待用户"明天继续"指令
> 全部 commit 已落地但**未 push**（按用户 B3 纪律）

---

## 1. 当天进度（4 commit + 1 amend + 2 docs）

### 1.1 commit 链
```
d0ab693  feat(server,domain):     NAS 多媒体源配置键 + 解析工具 + compose 路径   [C1+C2]
4bbfd29  feat(server,indexengine): NAS 多媒体源多路径遍历扫描 [V7 §9.4+]      [C3 amended]
2f44d15  feat(server,resolve):    NAS P0 智能跳过 + Capabilities 三态化 + 启动自动扫描 [V7 §6.3+§9.4+§28.1]  [C4]
```

主分支：
```
3af253d  fix(web,dashboard): 修正 StateSnapshot 属性访问路径，修复 CI TS2339
1542edc  fix(web,dashboard): 仪表盘状态优先读 capabilities 实时值，配置保存后自动刷新
```

### 1.2 文档沉淀（`.hermes/references/`）
- `v7-nas-index-gap-analysis-2026-08-17.md` (12 KB) — **V7 全章节精读 + 19 gap 总表**（下次会话接手 NAS 索引工作前必读）
- `v7-nas-p0-implementation-2026-08-17.md` (4 KB) — C4 实施档案
- `session-summary-2026-08-17-part1.md` (本文) — 今日工作总览

---

## 2. 业务 / 技术关键决策

### 2.1 NAS 多挂载点 — V7 增强需求（不是 bug fix）
- **V7 §9.4** 仅说明"单路径 `nas_local_path`"——用户实际需求是TrueNAS 25.10.13 多 SMB 共享
- **实施方式**：容器 bind mount 单根目录 + admin 界面增删子路径
- **commit 标注**：`[V7 §9.4+]` 表示 V7 扩展需求

### 2.2 索引两个目的（V7 §6.8.3）— 核心设计意图
| 目的 | NAS | 网盘 |
|---|---|---|
| 加速（命中直接读） | ✅ | ✅ |
| 减少网盘风控（直链缓存 + 过期刷新） | ❌ 不需要 | ✅ 必须 |

**NAS 扫描不写 stream_url/url_expires**（C3 commit message 明确）

### 2.3 P0 智能跳过条件（V7 §6.3）
4 个条件，全部在 `domain/resolve/nas_layer.go:shouldSkipP0()` 实现：
1. NAS 未配置
2. 索引服务不可用
3. NAS 处于扫描中（Phase A/B 索引不完整）**[C4 新增]**
4. 索引为空

### 2.4 Capabilities 三态（V7 §27.4）
`domain.Capabilities.NASStatus`：
- `not_configured` — 未配置路径或 `nas_enabled=false`
- `not_accessible` — 配置路径但 stat 失败
- `ok` — 配置 + 路径可读

---

## 3. Gap 路线图（剩余 6 个 P0/P1/P2）

| # | gap | V7 条款 | 严重度 | 预期战役 |
|---|---|---|---|---|
| G1 | NAS 媒体源 CRUD（多挂载点） | §9.4+ | P1 | **C5** |
| G2 | NAS 路径 vs 媒体源分离 | §9.4+ | P1 | C5 |
| G4 | NAS 自动 cron 增量扫描 | §9.7.4 | P1 | C6 |
| G8 | playback/stream.go NAS 分支直读 | §6.8.3 | P0 | Round A |
| G13 | eventbus.FileMutated → media_index 增量 | §9.3 | P0 | Round B |
| G15 | stream 时更新 last_played_at | §9.5 | P0 | Round A |
| G10 | 月度全盘校验 | §9.7.5 | P2 | C6 后置 |
| G18 | Vue UI "媒体配置 → NAS" 改造 | §12.2 Phase 8 | P1 | C7 |

完整 gap 表见 `v7-nas-index-gap-analysis-2026-08-17.md` §2。

---

## 4. 实施过程的非显性教训

### 4.1 Patch 工具陷阱
- Patch 工具多行替换 ≥ 2 次容易吃错缩进（出现 `if err != nil` 多一层 tab）
- **纪律**：≥ 2 次 patch 失败立即切换 Python 脚本（write_file + 完整替换）
- 详见 memory：`patch→script 退出条件`

### 4.2 PowerShell 编码 / 转义陷阱
- `Get-Content -Raw` + `Set-Content` 默认 GBK 写回 → 破坏 UTF-8 中文注释（导致 `illegal UTF-8 encoding`）
- **修复**：用 `write_file`（write_file 强制 UTF-8），不混用 PowerShell 文件 I/O
- `cmd /c "go.exe test ... -run 'A|B'"` 单引号在 cmd 中被截断
- **修复**：直接 PowerShell 原生调用 `& go.exe`，不用 cmd /c
- `go test -o $tmpOut` 是写测试二进制，不是 stdout 重定向
- **修复**：用 `*> $tmpOut 2>&1` 重定向

### 4.3 LSP 诊断的价值
- 每次重构后 `/worker_count` 等变量引用错误能被 LSP 立即捕获（`Root in struct literal of type NASProgress`）
- **纪律**：看 LSP 红色诊断优先于跑 `go build`

### 4.4 测试覆盖盲区
- 单测前一定要 stub 所有 service 方法（包括 `pansearchHealth` / `loggedInDrivers` / `magnetEnabledFn` 等回调）
- Capabilities() 实际调用链上**每个**回调都可能 nil panic
- **纪律**：写测试时列出生产代码调用的所有回调，全部 stub

### 4.5 `os` 包导入遗漏
- refactor 时 patch 在 import 中加 `os` 但失败（2 处匹配），后续补加能漏
- **纪律**：refactor 后**立刻** `go build ./...` 检查，不要等 5 个 patch 一起

---

## 5. 当前仓库状态

### 5.1 git log（main 分支 HEAD）
```
2f44d15  feat(server,resolve): NAS P0 智能跳过 + Capabilities 三态化 + 启动自动扫描 [V7 §6.3+§9.4+§28.1]
4bbfd29  feat(server,indexengine): NAS 多媒体源多路径遍历扫描 [V7 §9.4+]
d0ab693  feat(server,domain): NAS 多媒体源配置键 + 解析工具 + compose 路径
3af253d  fix(web,dashboard): 修正 StateSnapshot 属性访问路径，修复 CI TS2339
```

### 5.2 git status
工作树干净（未提交 C4 改动）：
- `M server/go.mod`（历史 go 模块更新，不是本轮改动）
- `D server/internal/api/web/assets/index-*.gz` (D=Deleted) — 历史遗留 web asset
- `M server/internal/api/web/index.html` — 历史遗留
- `?? server/internal/api/web/assets/index-*.gz` (Untracked) — 历史遗留

**这些都不是 C1-C4 改动**，是 commit 3af253d 之前的快照差异（web asset 重建）

### 5.3 NAS 部署状态
- NAS=192.168.7.154:38088 跑通
- admin 密码=12345678 (form-data POST)
- PanSou 内部地址 http://pansou:8888/ 已配
- pan115 + quark 已登录
- **NAS 路径未生效**（容器没 bind mount 真 SMB 路径）—— 部署测试需用户手动配 `NAS_MEDIA_PATH` env

---

## 6. 决策门（明天继续时需要先对齐）

### 6.1 G14 启动扫描如何部署验证
- 已实现 `go func() { indexEngine.ScanNASFull(ctx) }()` 在 `wireXMedia` 末尾
- **单元测试未覆盖**（启动逻辑不在单元测试范围）
- 部署后需用户重启 NAS 上的 xmedia 容器，加 `NAS_MEDIA_PATH=/mnt/BTORAGE` env 才能验证

### 6.2 G7 实现完整性
- ✅ `shouldSkipP0` 4 个条件全部实现
- ⚠️ **`runEngine` 调 `shouldSkipP0` 后**只更新了 `t.StageDetail = "P0 已跳过：..."`，**没真正去 P1**——让我回去看代码确认

### 6.3 C5 NAS 媒体源 CRUD 的存储格式选择
- **A**：新建 DB 表 `nas_sources`（id, name, container_path, enabled, created_at）
- **B**：configs 表 JSON 数组（沿用 C3 解析的 `nas_local_paths`）
- **倾向 A**（多挂载点有独立生命周期，CRUD 友好）

### 6.4 Round A (playback stream audit) 范围
- §6.8.3 流程图完整覆盖：NAS 直读 / 网盘 302 / 过期刷新
- 涉及 `internal/playback/stream.go`（待 audit）
- 涉及 `internal/playback/service.go`（last_played_at 更新）

---

## 7. 明日开工 quickstart

### 7.1 第一步：核对 git 状态
```powershell
cd D:\DSH-NEW
git log --oneline -5
git status -s
```

### 7.2 第二步：跑 canonical gate（验证 HEAD 2f44d15 仍然干净）
```powershell
cd D:\DSH-NEW\server
go.exe build ./...
go.exe vet ./...
go.exe test ./internal/resolve/... -run 'TestShouldSkipP0_V7|TestCapabilities_NASStatus' -count=1 -v
```

### 7.3 第三步：读 gap 文档
- `references/v7-nas-index-gap-analysis-2026-08-17.md` — 剩余 6 个 gap 详细
- `references/v7-nas-p0-implementation-2026-08-17.md` — C4 实施档案

### 7.4 第四步：根据用户拍板推进
- 选项 A：直接开 C5（NAS 媒体源 CRUD）
- 选项 B：先做 Round A（playback stream audit + G8/G15 修复）
- 选项 C：先手动部署验证 C4（包括 G14 启动扫描 + G7 智能跳过 + G11 三态健康）

---

## 8. 已知 blocker / 注意事项

### 8.1 部署版 vs 源码不一致
- 之前实测发现 NAS 部署版二进制白名单比源码宽（如 `nas_local_path` 源码不在白名单但部署版放行）
- C4 后**需重新部署**才能端到端验证
- 按 B3 纪律：AI 不 push，需要用户触发

### 8.2 路线 P0 实战阻塞
- **真正能播放的链路**：需要 NAS 路径真挂载 + 索引能扫到文件
- 当前 NAS 容器 bind mount 是占位 `/tmp/...` —— 扫描结果永远 0
- **必须用户操作**：在 NAS 主机或 docker-compose 环境变量 `NAS_MEDIA_PATH=/mnt/BTORAGE` 重启

### 8.3 next 推进节奏
- **每改一轮 → 实地测一轮 → commit 一轮**
- 严禁"改完没测就 commit"（多文件改动测试盲区未被捕获过的教训）
- 实战改动 gap 后，**必须用 NAS 部署版本真实测试**（不能再继续依赖单测）

---

## 9. 明天恢复建议

如果用户让继续：
1. 先确认是否要"实测 C4 部署" 还是"继续 C5"
2. 实测 C4 的话，先给用户 `set $NAS_MEDIA_PATH=/path/to/real/smb` 的 docker-compose 例子
3. 继续 C5 的话，先问存储格式（A 新表 vs B JSON 数组）
4. Round A 的话，先 audit `internal/playback/stream.go` 完整覆盖 V7 §6.8.3 流程图

强制早 break：每完成 1 个真好单元测试 + 1 个 commit 就停下来汇报，不要等用户说"继续"才停。

---

## 10. 写到 memory 的核心事实

以下 3 条已 / 待更新到 `memory`（保证跨会话保留）：
- 用户工程纪律（B3：AI 不 push / 每改一轮 commit 一轮 / 单测不放过）
- X-MEDIA v7 当前 HEAD（2f44d15）+ 部署端口 38088 + admin 密码 12345678
- NAS 多挂载点 = V7 §9.4+ 扩展需求（不要当 bug fix 处理）

---

> 文档自动生成：2026-08-17 23:50
> 写入 `.hermes/references/session-summary-2026-08-17-part1.md`
> 明天恢复工作先读本文档 §7 + §9

---

# Part 2 — 2026-08-18 → 2026-08-20 增量(append,part1 决策链保留)

> 适用：跨会话恢复时,先读 part1 §7+§9,再读本 part2
> 状态：HEAD `1ac966b` = `main` = `origin/main`(已 push,无遗留)
> 增量 commit 范围：`2f44d15..1ac966b` 共 13 commit

## P2.1 增量 commit 链

### C5 NAS 媒体源 CRUD(`d572e2b / 62f9cbc / 7e23dac`)
- `d572e2b` feat(server,web,api): NAS 媒体源 CRUD [V7 §9.4+ 多源扩展] — C5 主体
- `62f9cbc` fix(server,app,api): NAS source 接线补漏 + 移除 debug 日志 [V7 §9.4+]
- `7e23dac` chore(server,gitignore): C5 收尾清理
- **G1/G2 已 close** —— V7 §9.4+ 多源扩展落地

### E3 + V7 §9.7 修复(`48777e0 / daac473`)
- `48777e0` fix(web,media-config): XMediaPanel loadAll 单接口 500 拖垮全 tab [V7 整改 E3]
- `daac473` fix(server,resolve): NewService 漏赋 indexStatusFn/indexScanningFn + Capabilities 调用 nil-safe [V7 §9.7]
- **3 类 panic 全部捕获**(详见 memory):构造漏赋 func-typed / wiring 漏 binding / Promise.all 缺 .catch

### 5+ 实战反馈清单(`28e16c2 / 8232377 / 153c922 / 96a18c0 / e914ebc / c915e87 / 1ac966b`)
按 D 方案拆 7 commit 滚动,具体见各 commit message:

| # | commit | 主题 | 实际根因 |
|---|---|---|---|
| 1 | 28e16c2 | Logo 替换 LitePan → X-MEDIA AI | — |
| 1' | 8232377 | Logo 浅色/深色双变体 + prefers-color-scheme | — |
| 2 | 153c922 | Dashboard hero-metric 文案"接入/在线" → "网盘接入/网盘在线" | 用户语义混淆 |
| 3 | 96a18c0 | nav-icon fontawesome → 内联 SVG | fontawesome CSS 没打包 |
| 4 | e914ebc | NAS host-path → container-path auto mapping | 用户不该知道容器内路径(V7 §13.1 用户友好违反) |
| 5 | c915e87 | NAS 扫描按钮 UX feedback | 扫描异步,无 UI 状态显示 |
| 6 | 1ac966b | nav-icon SVG 撑爆 sidebar + CRLF 修复 | SVG 默认 width=100% 撑爆 220px sidebar |

**commit #6 复盘(写入 SKILL §0)**:
- SVG/Image 默认尺寸 > 父容器 CSS 限定 —— 必须双保险
- AdminNavIcon.vue 9 个 SVG 都漏 width/height,Vue 3 class 传递没用
- PowerShell 终端 commit 引入 CRLF,未 normalize
- 验证脚本只 grep 字符串,未做渲染测试 → 必修"心理模拟:width/height 显式还是 auto"
- canonical gates：`npm run type-check` + `npm run build` 双 0

## P2.2 仍未关闭的 gap(来自 part1 §3)

| # | gap | V7 条款 | 严重度 | 现状 |
|---|---|---|---|---|
| G4 | NAS 自动 cron 增量扫描 | §9.7.4 | P1 | 未开始 |
| G8 | playback/stream.go NAS 分支直读 | §6.8.3 | P0 | 未开始 |
| G13 | eventbus.FileMutated → media_index 增量 | §9.3 | P0 | 未开始 |
| G15 | stream 时更新 last_played_at | §9.5 | P0 | 未开始 |
| G10 | 月度全盘校验 | §9.7.5 | P2 | 未开始 |
| G18 | Vue UI 媒体配置 → NAS 改造 | §12.2 Phase 8 | P1 | 未开始 |

**G1 / G2 在 C5 关闭**(清单更新)

## P2.3 8/18–8/20 几天沉淀的核心教训

### 纪律强化
- **commit 后 `git show --stat` + 实际拉起 + curl 三件套不可省**
- **chore/cleanup commit 更高危**(7e23dac 用"清理"掩盖了 NewService 漏赋 func-typed 字段)
- **Go func 类型零值是 nil**(编译/单测不抓,必须 curl 端点触达)
- **前端 Promise.all 每个 fetch 自带 .catch**(或用 Promise.allSettled)—— 单接口 500 拖垮全 tab

### 部署端教训(8/18 实测)
- NAS 部署版二进制白名单比源码宽 → C4 必须重部署才能端到端验证
- 容器 bind mount 路径若不对,扫描永远 0 条 → 必须真 SMB 路径

### Commit 风格纪律
- 实测反馈多问题 → 按 D 方案拆 N commit,每项独立 commit + 独立 audit + 三件套验证
- 严禁"一锅端"改 5 个文件 = 1 commit

## P2.4 仓库当前事实(2026-08-20 校正)

| 维度 | 当前真实状态 |
|---|---|
| HEAD | `1ac966b` |
| 工作树 | 仅 1 个未追踪:`.hermes/references/session-summary-2026-08-17-part1.md`(本文件) |
| Go build / vet | ✅ 双 0 |
| Web 目录 | `D:\DSH-NEW\server\web\`(**part1 假设的 `D:\DSH-NEW\web/` 不存在**) |
| NAS 部署 | 192.168.7.154:38088,admin 密码 12345678;PanSou http://pansou:8888/ 已配 |
| pan115 + quark | 已登录 |
| NAS 路径 | 仍需用户手动配 `NAS_MEDIA_PATH` env + 重启容器才生效 |

## P2.5 候选下一步(等用户拍板)

### A. 文档收尾(已完成,见本 part2)
### B. 跑剩余 web canonical(下个响应执行)
### C. 继续 V7 gap 路线
- **C6=G4** NAS 自动 cron 增量扫描(V7 §9.7.4)
- **Round A** playback stream audit(G8+G15)涉及 `internal/playback/{stream,service}.go`
- **Round B** eventbus 增量(G13)涉及 §9.3
- **G18** Vue UI 媒体配置 NAS 改造(§12.2 Phase 8)

### 节奏
- 每改一轮 → 实地测一轮 → commit 一轮(同 part1 §8.3)
- 严禁"改完没测就 commit"
- B3=AI 不 push,本地 commit 由用户终端 push

## P2.6 跨会话恢复 quickstart

```powershell
cd D:\DSH-NEW
git log --oneline -3                                    # 确认 HEAD = 1ac966b
git status -s                                          # 应仅 1 行 .hermes/references/session-summary-2026-08-17-part1.md
cd server; go.exe build ./...; go.exe vet ./...         # 双 0 expected
```

读本文档顺序：**part1 §7+§9**(决策门)→ **part2 §P2.5**(候选下一步)。

---
> 文档增量生成：2026-08-20(part1 生成于 2026-08-17 23:50)
> 写入 `.hermes/references/session-summary-2026-08-17-part1.md`(同文件 append,不另起 part2 文件)