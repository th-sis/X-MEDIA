// X-MEDIA 管理 API 客户端（§12.2 / Phase 8）：
// 能力预检 / 索引引擎 / X-MEDIA 配置 / 健康检查。
import { http, ApiError } from "./client";

export interface Capabilities {
  nas_available: boolean;
  nas_status: string; // [V7 §27.4] not_configured / ok / not_accessible
  nas_index_complete: boolean;
  nas_index_count: number;
  nas_phase: string;
  nas_processed_files: number;
  nas_total_files: number;
  nas_scanning: boolean;
  nas_total_sources: number;
  nas_enabled_sources: number;
  pansearch_available: boolean;
  logged_in_drivers: string[];
  magnet_enabled: boolean;
  p0_min_score: number;
  demo_fallback: boolean;
  server_version: string;
}

export interface IndexProgress {
  scope: string;
  phase: string;
  status: string;
  processed: number;
  total: number;
  matched: number;
  unconfirmed: number;
  orphaned: number;
  rate_per_sec: number;
  error_msg: string;
}

export interface StateSnapshot {
  active_resolve_tasks: number;
  capabilities: Capabilities;
  index_progress: IndexProgress;
  index_scanning: boolean;
  indexed_total: number;
  server_started_at: string;
  server_uptime_secs: number;
  server_version: string;
  // V7 §28.3：客户端感知重启（graceful / config_change / oom / panic）
  last_restart_reason?: string;
}
export interface HealthStatus {
  status: string;
  message?: string;
  validation: {
    ok: boolean;
    tmdb_key?: { status: string; message?: string };
    pansearch_url?: { status: string; message?: string };
    nas?: { status: string; message?: string };
    has_any_account?: { status: string; message?: string };
  };
}

export function fetchSnapshot(): Promise<StateSnapshot> {
  return http.get<StateSnapshot>("/state/snapshot");
}

export function fetchHealth(): Promise<HealthStatus> {
  return http.get<HealthStatus>("/health");
}

export async function fetchXMediaConfigs(): Promise<Record<string, string>> {
  const items = await http.get<{ items: Record<string, string> }>("/admin/configs/");
  return items?.items ?? {};
}

export function saveXMediaConfig(key: string, value: string): Promise<unknown> {
  return http.put("/admin/configs/", { key, value });
}

export function startNasScan(mode: "full" | "incremental"): Promise<unknown> {
  return http.post(`/admin/index/nas/${mode}`);
}

// [P2#7] 手动触发 media_library LRU 淘汰, 返回淘汰数量 + 当前阈值.
export interface TmdbEvictResult {
  removed: number;
  max_rows?: string;
  keep_rows?: string;
}
export function tmdbEvict(): Promise<TmdbEvictResult> {
  return http.post<TmdbEvictResult>("/admin/tmdb/evict", {});
}

// TMDB 配置连通性测试：走搜索端点（有 key 时真实 API，无 key 时演示数据）。
// 注意：/api/tmdb/search 返回裸列表（无 success 包装），不能走 http 的标准
// 解包逻辑，这里直接 fetch 并解析。
export async function testTmdbKey(): Promise<{ item_count: number }> {
  const resp = await fetch(`/api/tmdb/search?q=${encodeURIComponent("阿凡达")}`, {
    credentials: "include",
  });
  if (!resp.ok) {
    throw new ApiError(`TMDB 连通失败（HTTP ${resp.status}）`, "http_error", resp.status);
  }
  const body = (await resp.json()) as { items?: unknown[]; total?: number };
  return { item_count: body.items?.length ?? 0 };
}

// ===== [V7 §9.4+ 扩展 G1.C/G18] NAS 媒体源 CRUD =====

export interface NASSource {
  id: number;
  name: string;
  path: string;
  enabled: boolean;
  file_count: number;
  last_accessibility: "ok" | "not_accessible" | "unknown";
  last_checked_at?: string;
  created_at: string;
  updated_at: string;
}

export interface NASTestPathResult {
  path: string;
  exists: boolean;
  is_dir: boolean;
  readable?: boolean;
  file_count: number;
  sample?: string[];
}

export interface NASBulkHealth {
  checked: number;
  results: Array<{
    id: number;
    name: string;
    path: string;
    status: string;
    count: number;
    persisted: boolean;
  }>;
}

export function fetchNASSources(): Promise<NASSource[]> {
  return http.get<NASSource[]>("/admin/nas-sources/");
}
export function createNASSource(payload: { name: string; path: string; enabled?: boolean }): Promise<NASSource> {
  return http.post<NASSource>("/admin/nas-sources/", payload);
}
export function updateNASSource(id: number, payload: { name?: string; path?: string; enabled?: boolean }): Promise<NASSource> {
  return http.put<NASSource>(`/admin/nas-sources/${id}`, payload);
}
export function deleteNASSource(id: number): Promise<{ deleted: number }> {
  return http.del<{ deleted: number }>(`/admin/nas-sources/${id}`);
}
export function toggleNASSource(id: number): Promise<NASSource> {
  return http.post<NASSource>(`/admin/nas-sources/${id}/toggle`, {});
}
export function testNASPath(path: string): Promise<NASTestPathResult> {
  return http.get<NASTestPathResult>("/admin/nas-sources/test-path", { path });
}
export function bulkNASHealth(): Promise<NASBulkHealth> {
  return http.post<NASBulkHealth>("/admin/nas-sources/bulk-health", {});
}

// [76007b2 UI-first] 存量 source 路径批量重映射：
// 历史版本入库的主机视角路径 (/mnt/BTORAGE/*) 一键改写为容器内路径。
export interface NASReresolveItem {
  id: number;
  name: string;
  old_path: string;
  new_path: string;
  source: NASResolveSource;
  changed: boolean;
  // [实测增强] 改写后的即时可达性 — 点击后无需等周期监测即可看状态变化
  accessible?: "ok" | "not_accessible";
  error?: string;
}
export interface NASReresolveResult {
  total: number;
  changed: number;
  results: NASReresolveItem[];
  // 容器无 SMB 挂载时非空：精确部署指引（Docker volume 只能创建时挂载）
  deploy_hint?: string;
}
export function reresolveNASPaths(): Promise<NASReresolveResult> {
  return http.post<NASReresolveResult>("/admin/nas-sources/reresolve", {});
}

// ===== [V7 §9.4+ 扩展 G18] NAS 主机路径 → 容器路径 映射管理 =====
//
// commit #4 (e914ebc) 添加后端 6 个端点：
//   GET    /api/admin/nas-mounts                  configured + detected
//   POST   /api/admin/nas-mounts                  body {host_path, container_path}
//   PUT    /api/admin/nas-mounts/{host_path}    body {container_path}
//   DELETE /api/admin/nas-mounts/{host_path}    body {container_path}
//   POST   /api/admin/nas-mounts/probe           强制重新探测 /proc/self/mountinfo
//   POST   /api/admin/nas-mounts/resolve         body {path} → {resolved, source}
//
// 映射存储在 configs[nas_mount_<host_path>] = container_path。
// 创建/删除/更新只动 configured（手动），detected 来自 mountinfo 自动探测。

export interface NASMount {
  host_path: string;
  container_path: string;
}

// 探测结果（来自 /proc/self/mountinfo SMB/cifs 挂载）
// [76007b2 对齐] 后端 domain.MountInfoEntry JSON tag: filesystem/mount_target/source
export interface NASDetectedMount {
  filesystem: string;
  mount_target: string;
  source: string;
}

export interface NASMountListView {
  configured: NASMount[];
  detected: NASDetectedMount[];
}

export type NASResolveSource = "explicit" | "auto_detected" | "smb_alias" | "passthrough";

export interface NASResolveResult {
  input: string;
  resolved: string;
  source: NASResolveSource;
}

export function fetchNASMounts(): Promise<NASMountListView> {
  return http.get<NASMountListView>("/admin/nas-mounts");
}

export function createNASMount(payload: {
  host_path: string;
  container_path: string;
}): Promise<NASMount> {
  return http.post<NASMount>("/admin/nas-mounts", payload);
}

export function updateNASMount(
  hostPath: string,
  payload: { container_path: string }
): Promise<NASMount> {
  // route.go 用 chi.URLParam(r, "host_path")，传值需要 url-encode 但 client 已经走在
  // buildURL(path) 里走 URLSearchParams，这里直接拼接即可
  return http.put<NASMount>(
    `/admin/nas-mounts/${encodeURIComponent(hostPath)}`,
    payload
  );
}

export function deleteNASMount(hostPath: string): Promise<{ host_path: string; deleted: boolean }> {
  return http.del<{ host_path: string; deleted: boolean }>(
    `/admin/nas-mounts/${encodeURIComponent(hostPath)}`
  );
}

export function probeNASMounts(): Promise<{ detected: NASDetectedMount[]; count: number }> {
  return http.post<{ detected: NASDetectedMount[]; count: number }>(
    "/admin/nas-mounts/probe",
    {}
  );
}

export function resolveNASPath(path: string): Promise<NASResolveResult> {
  return http.post<NASResolveResult>("/admin/nas-mounts/resolve", { path });
}

// ===== [V7 §9.4 UI-first] 容器内 SMB 挂载点管理（特权 mount.cifs）=====
//
// 后端端点（api/smb_mounts_admin.go）：
//   GET    /api/admin/smb-mounts                列表（含实时状态）
//   POST   /api/admin/smb-mounts                新增并立即挂载
//   PUT    /api/admin/smb-mounts/{id}           更新并重新挂载
//   DELETE /api/admin/smb-mounts/{id}           卸载并删除
//   POST   /api/admin/smb-mounts/{id}/mount     手动（重新）挂载
//   POST   /api/admin/smb-mounts/{id}/unmount   手动卸载
//   POST   /api/admin/smb-mounts/refresh        全量按 /proc/self/mounts 校准状态
//
// 安全：后端返回的 smb_url 密码已脱敏（user:***@host/share）。

export type SMBMountStateType = "unmounted" | "mounting" | "mounted" | "error";

export interface SMBMount {
  id: number;
  name: string;
  smb_url: string; // 脱敏后展示（密码为 ***）
  remote_path: string;
  mount_point: string;
  uid: number;
  gid: number;
  state: SMBMountStateType;
  last_error?: string;
  last_checked_at?: string;
  created_at: string;
  updated_at: string;
}

export interface SMBMountCreatePayload {
  name: string;
  smb_url: string;
  remote_path?: string;
  mount_point: string;
  uid?: number;
  gid?: number;
}

export function fetchSMBMounts(): Promise<SMBMount[]> {
  return http.get<SMBMount[]>("/admin/smb-mounts");
}

export function createSMBMount(payload: SMBMountCreatePayload): Promise<SMBMount> {
  return http.post<SMBMount>("/admin/smb-mounts", payload);
}

export function updateSMBMount(
  id: number,
  payload: Partial<SMBMountCreatePayload>
): Promise<SMBMount> {
  return http.put<SMBMount>(`/admin/smb-mounts/${id}`, payload);
}

export function deleteSMBMount(id: number): Promise<{ deleted: number }> {
  return http.del<{ deleted: number }>(`/admin/smb-mounts/${id}`);
}

export function mountSMBMount(id: number): Promise<SMBMount> {
  return http.post<SMBMount>(`/admin/smb-mounts/${id}/mount`, {});
}

export function unmountSMBMount(id: number): Promise<SMBMount> {
  return http.post<SMBMount>(`/admin/smb-mounts/${id}/unmount`, {});
}

export function refreshSMBMounts(): Promise<SMBMount[]> {
  return http.post<SMBMount[]>("/admin/smb-mounts/refresh", {});
}
