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
