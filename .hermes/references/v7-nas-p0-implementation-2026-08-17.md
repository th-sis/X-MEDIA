# X-MEDIA V7 NAS P0 Gap 整改 — 实施档案 (commit b1a2f5a)

> 时间：2026-08-17
> 范围：G7（智能跳过 P0）+ G11（capabilities 三态）+ G14（启动自动扫描）
> 完整 gap 分析见 `v7-nas-index-gap-analysis-2026-08-17.md`

---

## 1. 整改内容

### 1.1 G7 — Resolve 智能跳过 P0（V7 §6.3）

[V7 §6.3] 明示"扫描中跳过 P0"，旧实现缺这一条。

**修改**：
- `internal/resolve/nas_layer.go` `shouldSkipP0()` 加 4 个跳过条件：未配置 / 索引服务不可用 / **扫描中** / 索引为空
- `internal/resolve/service.go` `Options` 加 `IndexScanning func() bool` 回调
- `internal/app/wire_xmedia.go` 接 `indexEngine.IsScanning()`

**测试覆盖**（`nas_layer_test.go`）：
- 未配置 NAS → 跳过（reason: 未配置 NAS 路径）
- 扫描中 → 跳过（reason: NAS 正在扫描（索引不完整））
- 索引为空 → 跳过（reason: NAS 索引为空）
- 正常 → 不跳过

### 1.2 G11 — Capabilities NAS 三态化（V7 §9.4 + §27.4）

[V7 §27.4] 明示 nas 状态值 `ok / not_configured / not_accessible`，旧实现只有 bool。

**修改**：
- `domain/capabilities.go` `Capabilities` 加 `NASStatus string` + `NASScanning bool` 字段
- `internal/resolve/service.go` `Capabilities()` 重写：
  - `not_configured`：未配置路径或 `nas_enabled=false`
  - `not_accessible`：配置路径但 stat 失败
  - `ok`：配置 + 路径可读
- `Options` 加 3 个回调：`NASPathsKnown func() []string`、`PathStat func(path string) bool`、`IndexStatus func() (scanning, phase, processed, total)`

**测试覆盖**：
- not_configured（未配置）
- not_accessible（stat 失败）
- ok（stat 成功）
- 扫描中（NASScanning=true + 进度字段）

### 1.3 G14 — 启动自动触发 NAS 首次扫描（V7 §28.1 步骤5）

[V7 §28.1 步骤5] 明示"启动索引引擎（NAS 首次扫描）"，旧实现**完全不接**。

**修改**：
- `internal/app/wire_xmedia.go` 在 `wireXMedia` 末尾添加 `go func() { indexEngine.ScanNASFull(ctx) }()`
- 仅当 `nasConfiguredFn` 为 true 时才启动；扫描中重复触发被 `indexengine.Service` 内部互斥锁忽略

**验证**：单元测试覆盖（integration 级，启动逻辑不在单元测试范围；部署后实测）。

---

## 2. 索引两个目的（V7 §6.8.3）的设计意图（commit message 强调）

| 目的 | NAS 处理 | 网盘处理 |
|---|---|---|
| **加速** | `media_index.file_path` 命中 → stream 代理直读本地 | `media_index.file_path` + `stream_url` 命中 → 302 重定向 |
| **减少网盘风控** | **不需要**（本地直读） | `media_index.stream_url` 缓存 + `url_expires` 自动刷新 |

NAS 扫描**不写 stream_url/url_expires**（commit `4bbfd29` 已说明）—— 本次 C4 不涉及此字段写入逻辑，仅扩展 Capabilities 三态化。

---

## 3. 三个回调的依赖方向

```
resolve.Options 接收：
  NASPathsKnown / PathStat       (stateless helpers, nil-safe 兜底)
  IndexScanning / IndexStatus    (→ indexEngine)
  NASConfigured / IndexCount     (→ store/configs)

无反向依赖：resolve 不 import indexengine；indexengine 不感知 resolve。
```

---

## 4. 质量门

```
[1] gofmt -l: clean
[2] go build ./...: exit 0
[3] go vet ./...: exit 0
[4] go test ./...: 31/31 PASS（resolve 8.774s，含新 8 个用例）
```

新测试用例：
- `TestShouldSkipP0_V7` 4 子用例
- `TestCapabilities_NASStatus` 4 子用例

---

## 5. 剩余 Gap（不在本轮）

- **G1** NAS 媒体源 CRUD（V7 §9.4+ 多源扩展）—— C5
- **G4** NAS 自动 cron 增量扫描（§9.7.4）—— C6
- **G8** playback/stream.go NAS 分支直读（§6.8.3）—— 单独 audit
- **G13** eventbus.FileMutated → media_index 增量（§9.3）—— 单独 audit
- **G15** stream 时更新 last_played_at（§9.5）—— 同 G8 audit
- **G18** Vue UI "NAS 配置" 改造 —— C7

按 `references/v7-nas-index-gap-analysis-2026-08-17.md` 中的 Round A-G 顺序推进。