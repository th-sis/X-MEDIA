// X-MEDIA 管理 API 客户端（§12.2 / Phase 8）：
// 能力预检 / 索引引擎 / X-MEDIA 配置 / 健康检查。
import { http, ApiError } from "./client";

export interface Capabilities {
  nas_available: boolean;
  nas_index_complete: boolean;
  nas_index_count: number;
  pansearch_available: boolean;
  logged_in_drivers: string[];
  magnet_enabled: boolean;
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
export interface NASDetectedMount {
  mount_point: string;
  fs_type: string;
  source: string;
  super_options: string;
}

export interface NASMountListView {
  configured: NASMount[];
  detected: NASDetectedMount[];
}

export type NASResolveSource = "explicit" | "auto_detected" | "passthrough";

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
