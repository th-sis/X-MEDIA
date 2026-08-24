# Phase 10 集成测试落地计划（方案 B）

## 目标

对照 V7 §23.1 Phase 10 的 13 项验收标准，在 **Go httptest 全链路层**（真实 app 装配 + 真实 SQLite + 假外部服务）落地可自动化部分；无法脱离真机/真账号的项目明确标注待验条件。

## 分层策略

### A 层：Go API 级集成测试（本次交付）

落点：`server/internal/app/e2e_phase10_test.go`（package app，可直接访问私有 `httpSrv.Handler`，零生产代码改动）。

环境构造：
- `t.TempDir()` 作 DataDir（真实 SQLite 迁移全跑）
- `app.New(ctx, Options{Config, Logs})` 完整装配
- `httptest.NewServer(a.httpSrv.Handler)` 挂路由
- 假 PanSou：`POST /api/search` → 200 `{"merged_by_type":{}}`（合法空结果），配置写入 `pansearch_url`
- TMDB base URL 硬编码 → 只能验证无 key 降级路径（设计内行为）

| 用例 | 对应清单项 | 断言要点 |
|---|---|---|
| TestE2E_Phase10_HealthSnapshot | #5 启动健康检查 | /api/health 200 + subsystems + server_started_at；/api/state/snapshot 同源 |
| TestE2E_Phase10_Day1Chain | #7 Day 1 §1.4 Step2-4 | health→capabilities→tmdb/home→tmdb/search 无 key 全程不 5xx、响应结构合法 |
| TestE2E_Phase10_ContinueWatching | #9 继续观看 | POST history → GET continue-watching 含该条 → 更新 position 续播点变化 → DELETE 清空 |
| TestE2E_Phase10_SearchHistory | #10 搜索页(服务端) | tmdb/search 触发记录 → search-history 可查 → DELETE 清空 |
| TestE2E_Phase10_RateLimit429 | #12 并发限流 [v7] | 默认 3 次/30s：前 3 个非 429，第 4 个 429 + retry_after_sec |
| TestE2E_Phase10_NASIndexP0Play | #2 NAS 索引+播放 | 本地目录当 NAS root → 插 source+media_library 条目 → ScanNASFull 轮询完成 → POST resolve P0 done → GET /api/stream?ticket= 200 |
| TestE2E_Phase10_P2MagnetRecovery | #13 P2 后台恢复 §28.2 | 写 magnet running 任务 → Shutdown → 同 DB 重开 app → 任务保留且被恢复流程接管（状态合法） |
| TestE2E_Phase10_AllLayersMissSubscribe | #4/#11 部分 | 假 PanSou 空 + 无账号 + magnet 关 → resolve failed(P3) + 自动创建订阅 |

### B 层：真机/真账号待验（文件头注释标注，不入 CI）

- #1 P0/P1/P2 全路径中的 **P1 转存成功链路**（需 115/夸克等真实 OpenAPI 凭据）
- #3 盘搜+转存+播放成功链路（同上）
- #6 TV 遥控器物理操作（方向键导航/焦点丢失恢复——Flutter widget 层已有单测覆盖逻辑）
- #8 性能预算 §21.5（12 场景需真实数据量与硬件）
- Flutter UI 端到端（Day 1 Step 5 播放器画面、§17.5 分层指示器视觉验证）

## 执行步骤

1. 写 plan 文档（本文件）
2. 实现 e2e_phase10_test.go（8 用例 + helper）
3. `go test ./internal/app/ -run TestE2E_Phase10` 迭代至绿
4. 全量回归 `go test ./...` + `flutter test`
5. commit + push

## 已确认的契约细节（实现依据）

- `App.Shutdown(ctx)` 存在（app.go:184）；`httpSrv.Handler = router`（wire_http.go:77）
- RateLimiter 默认 `ConfigResolveRateLimitMax=3` / `Sec=30`（wire_xmedia.go:145-147）
- historyUpsertReq JSON 字段：external_id/title/media_type/season/episode/position_ms/duration_ms...
- ScanNASFull 内部 `go s.scanNAS` 异步 → 测试轮询 IsScanning()/Progress() 至完成
- NAS 匹配基于 media_library 已有条目（TitleOrig/Year/ExternalID），不依赖外网 TMDB
- pansearch.Service.Search 解析 `merged_by_type` map；不可达时静默返回空（§8.8）
- stream 端点：`GET /api/stream?ticket=xxx`
