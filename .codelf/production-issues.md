# V7 实测问题清单（持续更新）

每条标注：[S]严重度 / [分类] / [复现] / [设计意图] / [建议处理]

---

## A. 真机实测抓到的 bug（优先级最高）

### BUG-A1  [P0] PanSou CheckLinks 端点不存在 → 静默冒充成功
- [分类] server/resolve/pansearch
- [复现] PanSou 主分支仅暴露 /api/search + /api/health（已抓 model/response.go 验证），但 xmedia.Health/CheckLinks 设计契约里有 /api/check/links（V7 §8.2 文档虚构）
- [现状] 上轮 875fa0c 改了 CheckLinks 让 404 返回 err；但 CheckLinks 函数是否仍被调用要看 e2e 场景
- [设计意图] §8.2 "链接有效性检测"，§6.4 P1 流程依赖该步骤过滤死链
- [建议处理] ①本地启发式（保留给未来真实端点） ②当前直接不传调用、所有链接默认 valid（已是 runP1:209 兜底） ③更新 §8.2 文档删掉这个端点

### BUG-A2  [P1] resolve.runP1 SaveShare 失败全部静默吞掉
- [分类] server/resolve/pan_layer.go:180
- [复现] `if err != nil || savedRes == nil { continue }` — 20 条全失败用户只看到 not_found
- [设计意图] §6.4 P1 应 "找到 N 个 → 检测 → 转存" 阶段全部可见；§27.4 健康检查应能看到 save 失败
- [建议处理] 累计 20 条失败时 push "save_fail" 阶段 + 记录最近错误细节到 task.ErrorMsg

### BUG-A3  [P1] runP1/runP2 账号为空时静默 skip
- [分类] server/resolve/pan_layer.go:114, magnet_layer.go:21
- [复现] 无 active 账号 → return false，无 push → 用户只见 not_found
- [设计意图] §6.4 应让用户看到 "请先登录网盘账号" 提示
- [建议处理] 入口加 push(StageNoAccount, "未配置网盘账号，无法 P1/P2") → 客户端按 §27.4 显示具体操作按钮

### BUG-A4  [P2] capabilities.NASStatus vs WS health_check.nas.status 口径不一致
- [分类] server/resolve/service.go:382, server/internal/websocket/ws.go:91
- [复现] capabilities 报 nas_status=not_accessible(4 sources enabled)，WS 报 nas.status=not_configured(path="")
- [设计意图] §27.2 健康检查首条消息应包含 capabilities 全字段，二者口径一致
- [建议处理] WS healthPayload.nas 改为组装完整三态字段（status + path + file_count），从 Capabilities 取数

### BUG-A5  [P2] demo_fallback 仅覆盖播放引擎，搜索/首页无对应降级
- [分类] server/internal/tmdb (覆盖 search/home 路径)
- [复现] TMDB 上游不可达但有 key → home=0 sections, search=0 results，前端一片空白无任何提示
- [设计意图] §27.4 "对应具体操作按钮" + §6.4 "降级策略"
- [建议处理] 搜索/榜单下游失败时返回明确 error_msg + capabilities.tmdb_status

### BUG-A6  [P2] logx 500 panic 被 Recoverer 吞，无 ERROR 日志
- [分类] server/internal/api/middleware, server/internal/logx
- [复现] 上轮 capabilities 500 时 /api/logs 零 ERROR 条目（grep level=error / level=40 都空）
- [设计意图] §21.4 "错误日志必填字段"，§27.2 健康检查可观测
- [建议处理] ①加 Recoverer 内的 logx.Error ②/或 chi middleware 捕获所有 5xx 写错误日志

> **自查撤回**：上轮 `/api/admin/logs` 报 404 实为误诊——实测实际生效路径是 `/api/logs`（GET 200），是 requireAdmin 保护的正确路径，路径误用不是 bug。B2 (tmdb/config) 同理（PUT-only 是预期设计）。

### BUG-A7  [P1] NAS source 路径视角错位（部分修复）
- [分类] server + 部署
- [复现] 容器内无 SMB 挂载（NAS_MEDIA_PATH 未设），4 个 source 存的宿主路径 /mnt/BTORAGE/* 在容器内不存在
- [现状] 上轮测试 NAS not_accessible
- [设计意图] §9.4 NAS source 应经 ResolveNASPath 自动改写到容器内路径
- [建议处理] ①部署侧 export NAS_MEDIA_PATH ②服务端创建/更新 source 时已 rewrite，但存量 8/21 老数据未改；建议加管理端 "重新解析所有路径" 按钮

---

## B. 设计/UX 文档级问题（V7 文档与代码不一致）

### DOC-B1  V7 §8.2 文档契约失真
- [现象] §8.2 示例写顶层 merged_by_type，实际上游是 data.merged_by_type
- [现状] 上轮 875fa0c commit message 已记录文档同步建议

### DOC-B2  V7 §8.4 CheckLinks 端点虚构
- [现象] §8.4 列 CheckLinks，PanSou 主分支不存在该能力
- [现状] 上轮 875fa0c commit message 已记录
- [建议处理] 在 V7 §8.2 + §8.4 加"上游不存在则视为全部有效"明确条款

### DOC-B3  V7 §17.x.5 primaryFocus==null 文档与 Flutter 框架行为不符
- [现象] 真实 Flutter binding root scope 兜底，页面销毁后 primaryFocus 仍不为 null
- [现状] 已记录在 5b49a51 时间附近的 note；实现改用"恢复时跳过已离树节点"覆盖真实场景

---

## C. 部署/配置问题

### DEPLOY-C1  NAS 上无 SMB 挂载
- [现象] 容器内 /mnt/nas-root 不存在、detected=null
- [建议] 用户侧 docker compose up 前 export NAS_MEDIA_PATH=/mnt/BTORAGE
- [状态] ✅ 已补充第二条路径：管理后台「NAS 配置 → SMB 挂载点」填 smb:// 地址 + 容器内挂载点，
  后端特权 mount.cifs 自动挂载并持久化（V7 §9.4 UI-first，smb_mounts 表 + smbmount 服务）。
  前提：compose 需取消注释 cap_add: SYS_ADMIN + privileged: true，Dockerfile 已装 cifs-utils。

### DEPLOY-C2  V7 文档 NAS 部分被裁剪/对不上当前实现
- [现象] §9.4 多 NAS source 设计是新增（commit 期间），admin 页面对应 G1.C
- [建议] 文档对齐 §9.4 vs §12.2 端点列表