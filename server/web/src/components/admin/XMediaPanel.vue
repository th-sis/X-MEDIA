<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import SectionTabBar from "@/components/admin/SectionTabBar.vue";
import SettingsCard from "@/components/admin/SettingsCard.vue";
import SettingsRow from "@/components/admin/SettingsRow.vue";
import AppButton from "@/components/base/AppButton.vue";
import AppInput from "@/components/base/AppInput.vue";
import AdminStatusPill from "@/components/admin/AdminStatusPill.vue";
import { getApiErrorMessage } from "@/api/client";
import {
  fetchSnapshot,
  fetchHealth,
  fetchXMediaConfigs,
  saveXMediaConfig,
  startNasScan,
  tmdbEvict,
  testTmdbKey,
  fetchNASSources,
  createNASSource as createNASSourceRequest,
  toggleNASSource,
  deleteNASSource,
  testNASPath,
  bulkNASHealth,
  reresolveNASPaths,
  fetchNASMounts,
  deleteNASMount,
  probeNASMounts,
  resolveNASPath,
  type HealthStatus,
  type StateSnapshot,
  type NASSource,
  type NASTestPathResult,
  type NASMount,
  type NASDetectedMount,
  type NASResolveResult,
} from "@/api/xmedia";
// [P0#2] 健康检查面板补"查看日志"按钮，复用 logs API
import { logsApi, type LogEntry } from "@/api/logs";
import AppModal from "@/components/base/AppModal.vue";
import { useSectionTabRoute } from "@/composables/useSectionTabRoute";
import { toast } from "@/composables/useToast";
import "@/styles/admin-shared.css";

// [P2#7] LRU 配置键（与后端 domain.ConfigMediaLibraryMaxRows / KeepRows 对齐）
const LRU_MAX_ROWS_KEY = "media_library_max_rows";
const LRU_KEEP_ROWS_KEY = "media_library_keep_rows";

// Phase 8 媒体配置页（§12.2/§27.4/§6.9/§9.7.1/§11.1）：
// 能力预检 + 索引状态面板 + TMDB/盘搜/NAS 配置 + 健康检查。
const OVERVIEW_TAB = "overview";
const INDEX_TAB = "index";
const TMDB_TAB = "tmdb";
const PANSEARCH_TAB = "pansearch";
const NAS_TAB = "nas";
const HEALTH_TAB = "health";

const tabs = [
  { key: OVERVIEW_TAB, label: "能力预检" },
  { key: INDEX_TAB, label: "索引状态" },
  { key: TMDB_TAB, label: "TMDB 配置" },
  { key: PANSEARCH_TAB, label: "盘搜配置" },
  { key: NAS_TAB, label: "NAS 配置" },
  { key: HEALTH_TAB, label: "健康检查" },
];

const { activeTab, setActiveTab } = useSectionTabRoute(OVERVIEW_TAB, tabs.map((t) => t.key));

const snapshot = ref<StateSnapshot | null>(null);
const health = ref<HealthStatus | null>(null);
const configs = ref<Record<string, string>>({});
const loading = ref(false);
// [V7 §9.4+ 扩展 G1.C] NAS 媒体源 CRUD 状态
const nasSources = ref<NASSource[]>([]);
const nasLoading = ref(false);
const nasCreating = ref(false);
const nasBulkHealthBusy = ref(false);
const nasTestingPath = ref(false);
const nasBusyId = ref<number | null>(null);
const nasNewName = ref("");
const nasNewPath = ref("");
const nasTestResult = ref<NASTestPathResult | null>(null);
// [V7 §9.4+ 扩展 G18] NAS 主机路径 → 容器路径 映射管理状态
const nasMounts = ref<NASMount[]>([]);
const nasDetected = ref<NASDetectedMount[]>([]);
const nasMountsLoading = ref(false);
const nasProbeBusy = ref(false);
const nasDeleteMountBusy = ref<string | null>(null);
// [76007b2 UI-first] 存量路径批量重映射
const nasReresolveBusy = ref(false);
// 实时预览：用户在 nasNewPath 输入时，debounce 300ms 调一次 resolve
const nasResolvePreview = ref<NASResolveResult | null>(null);
const nasResolveBusy = ref(false);
let nasResolveTimer: number | null = null;
const savingKey = ref("");
const testing = ref("");
const scanning = ref(false);

// [P0#2] 健康检查面板 — 日志弹窗状态
const healthLogModalOpen = ref(false);
const healthLogEntries = ref<LogEntry[]>([]);
const healthLogLoading = ref(false);
const healthLogLevel = ref<number>(30); // 默认 WARNING
// 健康检查"重扫索引"独立 busy 标志，避免与 OVERVIEW/INDEX_TAB 的 scanning 串扰
const healthScanBusy = ref(false);

// [P2#7] media_library LRU 配置 + 立即清理
const lruMaxRows = ref<string>("5000");
const lruKeepRows = ref<string>("3000");
const lruEvictBusy = ref(false);
// 初始化：从 configs 拉取当前值
function syncLRUFromConfigs() {
  if (configs.value?.[LRU_MAX_ROWS_KEY]) {
    lruMaxRows.value = configs.value[LRU_MAX_ROWS_KEY];
  }
  if (configs.value?.[LRU_KEEP_ROWS_KEY]) {
    lruKeepRows.value = configs.value[LRU_KEEP_ROWS_KEY];
  }
}

// [V7 §27.4] NAS 三态：not_configured / ok / not_accessible
const nasStatus = computed(() => snapshot.value?.capabilities.nas_status || "not_configured");
const nasStatusTone = computed(() => {
  if (nasStatus.value === "ok") return "success";
  if (nasStatus.value === "not_accessible") return "danger";
  return "warning";
});
const nasStatusLabel = computed(() => {
  switch (nasStatus.value) {
    case "ok": return "已配置且可访问";
    case "not_accessible": return "路径不可访问";
    default: return "未配置 NAS";
  }
});
const nasIndexComplete = computed(() => !!snapshot.value?.capabilities.nas_index_complete);
const pansearchAvailable = computed(() => !!snapshot.value?.capabilities.pansearch_available);
const indexProgress = computed(() => snapshot.value?.index_progress ?? null);
const indexedTotal = computed(() => snapshot.value?.indexed_total ?? 0);

// [P0#2] NAS 媒体源不可访问计数
const nasInaccessibleCount = computed(
  () => nasSources.value.filter((s) => s.last_accessibility === "not_accessible").length,
);

let pollTimer: number | null = null;

async function loadAll() {
  loading.value = true;
  nasLoading.value = true;
  // [V7 整改 E3] fetchSnapshot 必须自带 .catch(null) —— 否则任何 /api/state/snapshot
  // 500 都让整页 reject, 其它 tab 跟着 "请求失败 (500)" 假象拖垮 (Promise.all 短路语义)。
  // 正确做法: 每个 promise 各自 catch 返回零值, 这里只用 Promise.all 拿顺序, 不让它 reject。
  // 对比: DashboardManagement.vue 用 Promise.allSettled 走同条接口, 没有这个 bug。
  try {
    const [snap, h, cfg, nasList, mountsView] = await Promise.all([
      fetchSnapshot().catch(() => null),
      fetchHealth().catch(() => null),
      fetchXMediaConfigs().catch(() => ({})),
      fetchNASSources().catch(() => []),
      // [V7 §9.4+ 扩展 G18] mount map 拉取（容错，失败不阻塞主流程）
      fetchNASMounts().catch(() => null),
    ]);
    // null-safe 赋值: 任一接口失败时不污染其它状态
    if (snap) snapshot.value = snap;
    if (h) health.value = h;
    if (cfg) configs.value = cfg;
    if (nasList) nasSources.value = nasList;
    if (mountsView) {
      // 服务端无 configured 字段时容错
      nasMounts.value = mountsView.configured ?? [];
      nasDetected.value = mountsView.detected ?? [];
    }
  } catch (e) {
    toast.error(getApiErrorMessage(e, "状态读取失败"));
  } finally {
    loading.value = false;
    nasLoading.value = false;
    // [P2#7] 同步 LRU 配置到本地 ref
    syncLRUFromConfigs();
  }
}

// ===== [V7 §9.4+ 扩展 G1.C] NAS 媒体源 CRUD 操作 =====

async function testNewPath() {
  const path = nasNewPath.value.trim();
  if (!path) return;
  nasTestingPath.value = true;
  nasTestResult.value = null;
  try {
    const result = await testNASPath(path);
    nasTestResult.value = result;
    if (result.exists && result.is_dir && result.readable) {
      toast.success(`路径可读，浅层文件数 ${result.file_count}`);
    } else if (!result.exists) {
      toast.error("路径不存在");
    } else if (!result.is_dir) {
      toast.error("不是目录");
    } else {
      toast.error("无读取权限");
    }
  } catch (e) {
    toast.error(getApiErrorMessage(e, "路径测试失败"));
  } finally {
    nasTestingPath.value = false;
  }
}

async function createNASSource() {
  const name = nasNewName.value.trim();
  const path = nasNewPath.value.trim();
  if (!name || !path) {
    toast.error("名称和路径都不能为空");
    return;
  }
  nasCreating.value = true;
  try {
    const src = await createNASSourceRequest({ name, path });
    nasSources.value = [...nasSources.value, src];
    nasNewName.value = "";
    nasNewPath.value = "";
    nasTestResult.value = null;
    toast.success("已添加");
  } catch (e) {
    toast.error(getApiErrorMessage(e, "添加失败"));
  } finally {
    nasCreating.value = false;
  }
}

async function toggleNAS(id: number) {
  nasBusyId.value = id;
  try {
    const updated = await toggleNASSource(id);
    nasSources.value = nasSources.value.map((s) => (s.id === id ? updated : s));
  } catch (e) {
    toast.error(getApiErrorMessage(e, "切换启用状态失败"));
  } finally {
    nasBusyId.value = null;
  }
}

async function deleteNAS(id: number, name: string) {
  if (!confirm(`确认删除 "${name}"？`)) return;
  nasBusyId.value = id;
  try {
    await deleteNASSource(id);
    nasSources.value = nasSources.value.filter((s) => s.id !== id);
    toast.success("已删除");
  } catch (e) {
    toast.error(getApiErrorMessage(e, "删除失败"));
  } finally {
    nasBusyId.value = null;
  }
}

async function runBulkHealth() {
  nasBulkHealthBusy.value = true;
  try {
    await bulkNASHealth();
    const list = await fetchNASSources();
    nasSources.value = list;
    toast.success("全部 NAS 源可访问性检测完成");
  } catch (e) {
    toast.error(getApiErrorMessage(e, "批量检测失败"));
  } finally {
    nasBulkHealthBusy.value = false;
  }
}

// ===== [V7 §9.4+ 扩展 G18] NAS 主机路径 → 容器路径 映射操作 =====

async function probeNASMountsNow() {
  nasProbeBusy.value = true;
  try {
    const res = await probeNASMounts();
    nasDetected.value = res.detected ?? [];
    toast.success(`已重新探测 ${res.count} 个挂载点`);
  } catch (e) {
    toast.error(getApiErrorMessage(e, "重新探测失败"));
  } finally {
    nasProbeBusy.value = false;
  }
}

async function deleteNASMountByHost(hostPath: string) {
  if (!confirm(`确认删除主机路径映射 "${hostPath}"？`)) return;
  nasDeleteMountBusy.value = hostPath;
  try {
    await deleteNASMount(hostPath);
    nasMounts.value = nasMounts.value.filter((m) => m.host_path !== hostPath);
    toast.success("已删除映射");
  } catch (e) {
    toast.error(getApiErrorMessage(e, "删除映射失败"));
  } finally {
    nasDeleteMountBusy.value = null;
  }
}

// 实时预览：用户输入 nasNewPath 时 debounce 300ms 调一次 resolve
function scheduleResolvePreview() {
  if (nasResolveTimer !== null) {
    window.clearTimeout(nasResolveTimer);
  }
  const path = nasNewPath.value.trim();
  if (!path) {
    nasResolvePreview.value = null;
    nasResolveBusy.value = false;
    return;
  }
  nasResolveBusy.value = true;
  nasResolveTimer = window.setTimeout(async () => {
    try {
      const res = await resolveNASPath(path);
      // 防止 debounce 期间用户又改输入：用最新的 nasNewPath 比对
      if (nasNewPath.value.trim() === path) {
        nasResolvePreview.value = res;
      }
    } catch {
      // 失败静默——预览是辅助，不弹错
      if (nasNewPath.value.trim() === path) {
        nasResolvePreview.value = null;
      }
    } finally {
      if (nasNewPath.value.trim() === path) {
        nasResolveBusy.value = false;
      }
    }
  }, 300);
}

function sourceLabel(source: NASResolveResult["source"] | undefined): string {
  if (source === "explicit") return "手动映射";
  if (source === "auto_detected") return "自动探测";
  // [76007b2] SMB 源别名推导：命中后后端已自动持久化映射到 configs
  if (source === "smb_alias") return "SMB 别名映射（已自动保存）";
  return "原样透传";
}

// [76007b2 UI-first] 存量路径批量重映射：历史宿主视角路径一键改写。
async function runReresolve() {
  nasReresolveBusy.value = true;
  try {
    const res = await reresolveNASPaths();
    const list = await fetchNASSources();
    nasSources.value = list;
    if (res.changed > 0) {
      toast.success(`已重映射 ${res.changed}/${res.total} 个源路径，列表已刷新`);
    } else {
      toast.info(`全部 ${res.total} 个源路径无需改写`);
    }
  } catch (e) {
    toast.error(getApiErrorMessage(e, "批量重映射失败"));
  } finally {
    nasReresolveBusy.value = false;
  }
}

async function saveConfig(key: string, value: string, label: string) {
  savingKey.value = key;
  try {
    await saveXMediaConfig(key, value.trim());
    configs.value = { ...configs.value, [key]: value.trim() };
    // [P2#7] 保存 LRU 配置后, 同步本地 ref
    if (key === LRU_MAX_ROWS_KEY || key === LRU_KEEP_ROWS_KEY) {
      syncLRUFromConfigs();
    }
    toast.success(`${label}已保存`);
    await loadAll();
  } catch (e) {
    toast.error(getApiErrorMessage(e, `${label}保存失败`));
  } finally {
    savingKey.value = "";
  }
}

async function testTmdb() {
  testing.value = "tmdb";
  try {
    const res = await testTmdbKey();
    toast.success(`TMDB 连通正常，搜索返回 ${res.item_count} 条结果`);
  } catch (e) {
    toast.error(getApiErrorMessage(e, "TMDB 测试失败"));
  } finally {
    testing.value = "";
  }
}

async function testPansearch() {
  testing.value = "pansearch";
  try {
    const h = await fetchHealth();
    health.value = h;
    const st = h.validation?.pansearch_url;
    if (st?.status === "ok") toast.success("盘搜服务连通正常");
    else toast.error(st?.message ?? "盘搜服务不可达");
  } catch (e) {
    toast.error(getApiErrorMessage(e, "盘搜测试失败"));
  } finally {
    testing.value = "";
  }
}

async function triggerScan(mode: "full" | "incremental") {
  // [V7 整改 commit #5] NAS 扫描按钮 feedback 改进:
  // 1. 启动后立即 fetchSnapshot, 让 UI 立即看到 scanning=true
  // 2. 延迟 2 秒重置 scanning, 避免连击
  scanning.value = true;
  try {
    await startNasScan(mode);
    toast.success(mode === "full" ? "全量扫描已启动" : "增量扫描已启动");
    fetchSnapshot().then((s) => { if (s) snapshot.value = s; }).catch(() => undefined);
  } catch (e) {
    toast.error(getApiErrorMessage(e, "扫描启动失败"));
  } finally {
    window.setTimeout(() => { scanning.value = false; }, 2000);
  }
}

// ===== [P0#2] 健康检查面板补操作按钮 =====

// 跳转到指定 Tab（用于"去配置"按钮）
function goToTab(tab: string) {
  void setActiveTab(tab);
}

// 健康检查面板里的"触发全盘扫描"——独立 busy 标志
async function triggerHealthFullScan() {
  healthScanBusy.value = true;
  try {
    await startNasScan("full");
    toast.success("全盘扫描已启动");
    fetchSnapshot().then((s) => { if (s) snapshot.value = s; }).catch(() => undefined);
  } catch (e) {
    toast.error(getApiErrorMessage(e, "全盘扫描启动失败"));
  } finally {
    window.setTimeout(() => { healthScanBusy.value = false; }, 2000);
  }
}

// 打开健康检查"查看日志"弹窗，按当前级别过滤最近 50 条
async function openHealthLogs() {
  healthLogModalOpen.value = true;
  healthLogLoading.value = true;
  try {
    const entries = await logsApi().list({
      level: healthLogLevel.value,
      limit: 50,
    });
    healthLogEntries.value = entries ?? [];
  } catch (e) {
    toast.error(getApiErrorMessage(e, "日志加载失败"));
    healthLogEntries.value = [];
  } finally {
    healthLogLoading.value = false;
  }
}

function closeHealthLogs() {
  healthLogModalOpen.value = false;
}

// ===== [P2#7] LRU 立即清理 =====

async function triggerEvictNow() {
  lruEvictBusy.value = true;
  try {
    const res = await tmdbEvict();
    if (res.removed > 0) {
      toast.success(`已淘汰 ${res.removed} 条（当前阈值: max=${res.max_rows ?? "?"} / keep=${res.keep_rows ?? "?"}）`);
    } else {
      toast.success("无需淘汰，库内条目未超过 max_rows");
    }
  } catch (e) {
    toast.error(getApiErrorMessage(e, "LRU 淘汰失败"));
  } finally {
    lruEvictBusy.value = false;
  }
}

function toneFor(status: string | undefined): "success" | "warning" | "danger" {
  if (status === "ok") return "success";
  if (status === "error") return "danger";
  return "warning";
}

onMounted(() => {
  void loadAll();
  pollTimer = window.setInterval(() => {
    void fetchSnapshot()
      .then((s) => (snapshot.value = s))
      .catch(() => undefined);
  }, 3000);
});
onUnmounted(() => {
  if (pollTimer !== null) window.clearInterval(pollTimer);
  // [V7 §9.4+ 扩展 G18] 清理实时预览 debounce timer，防止内存泄漏
  if (nasResolveTimer !== null) window.clearTimeout(nasResolveTimer);
});
</script>

<template>
  <div class="settings">
    <SectionTabBar :model-value="activeTab" :tabs="tabs" @update:model-value="setActiveTab">
      <template #actions>
        <AppButton type="button" variant="ghost" :disabled="loading" @click="loadAll">刷新</AppButton>
      </template>
    </SectionTabBar>

    <div v-show="activeTab === OVERVIEW_TAB">
      <SettingsCard title="能力预检（§6.3）" :loading="loading">
        <SettingsRow label="NAS 状态（§27.4）">
          <div class="nas-status-line">
            <AdminStatusPill :tone="nasStatusTone">{{ nasStatusLabel }}</AdminStatusPill>
            <span class="settings-help">
              已配置 {{ snapshot?.capabilities.nas_total_sources ?? 0 }} 个源 · 启用 {{ snapshot?.capabilities.nas_enabled_sources ?? 0 }} 个 · 可达性每 5 分钟自动复测
            </span>
          </div>
          <!-- [76007b2] not_accessible 时给出新逻辑下的自愈操作：批量重映射 → 去 NAS 配置 -->
          <div v-if="nasStatus === 'not_accessible'" class="row-actions" style="margin-top: 8px;">
            <AppButton type="button" variant="ghost" :disabled="nasReresolveBusy" @click="runReresolve">
              {{ nasReresolveBusy ? "重映射中…" : "批量重新映射存量路径" }}
            </AppButton>
            <AppButton type="button" variant="ghost" @click="goToTab(NAS_TAB)">去 NAS 配置</AppButton>
          </div>
        </SettingsRow>
        <SettingsRow label="NAS 索引完成">
          <AdminStatusPill :tone="nasIndexComplete ? 'success' : 'warning'">
            {{ nasIndexComplete ? "已完成" : "未完成" }}
          </AdminStatusPill>
        </SettingsRow>
        <SettingsRow label="PanSou 可用">
          <AdminStatusPill :tone="pansearchAvailable ? 'success' : 'warning'">
            {{ pansearchAvailable ? "可用" : "不可达（P1 降级）" }}
          </AdminStatusPill>
        </SettingsRow>
        <SettingsRow label="磁力兜底（P2）">
          <AdminStatusPill :tone="snapshot?.capabilities.magnet_enabled ? 'success' : 'warning'">
            {{ snapshot?.capabilities.magnet_enabled ? "已启用" : "已关闭" }}
          </AdminStatusPill>
        </SettingsRow>
        <SettingsRow label="已登录网盘">
          <span class="settings-help">
            {{ (snapshot?.capabilities.logged_in_drivers ?? []).join("、") || "无（请到存储管理添加账号）" }}
          </span>
        </SettingsRow>
        <SettingsRow label="索引条目总数">
          <span>{{ indexedTotal }}</span>
        </SettingsRow>
        <SettingsRow label="活跃解析任务">
          <span>{{ snapshot?.active_resolve_tasks ?? 0 }}</span>
        </SettingsRow>
        <SettingsRow label="服务版本">
          <span>v{{ snapshot?.server_version ?? "?" }} · 运行 {{ snapshot?.server_uptime_secs ?? 0 }}s</span>
        </SettingsRow>
      </SettingsCard>
    </div>

    <div v-show="activeTab === INDEX_TAB">
      <SettingsCard title="索引引擎（§9.7.1）" :loading="loading">
        <SettingsRow label="当前状态">
          <AdminStatusPill :tone="snapshot?.index_scanning ? 'warning' : 'success'">
            {{ snapshot?.index_scanning ? "扫描中" : indexProgress?.status === "done" ? "空闲" : "待扫描" }}
          </AdminStatusPill>
        </SettingsRow>
        <template v-if="indexProgress && indexProgress.total">
          <SettingsRow label="阶段">
            <span>Phase {{ indexProgress.phase }} · {{ indexProgress.status }}</span>
          </SettingsRow>
          <SettingsRow label="处理进度">
            <span>{{ indexProgress.processed }} / {{ indexProgress.total }}（{{ indexProgress.rate_per_sec }}/s）</span>
          </SettingsRow>
          <SettingsRow label="匹配 / 待确认 / 孤儿">
            <span>{{ indexProgress.matched }} / {{ indexProgress.unconfirmed }} / {{ indexProgress.orphaned }}</span>
          </SettingsRow>
        </template>
        <SettingsRow label="入库总数">
          <span>{{ indexedTotal }}</span>
        </SettingsRow>
        <SettingsRow label="操作">
          <div class="row-actions">
            <AppButton type="button" variant="primary" :disabled="scanning" @click="triggerScan('full')">
              {{ scanning ? "扫描中…" : "全量扫描" }}
            </AppButton>
            <AppButton type="button" variant="ghost" :disabled="scanning" @click="triggerScan('incremental')">
              {{ scanning ? "扫描中…" : "增量扫描" }}
            </AppButton>
          </div>
        </SettingsRow>
      </SettingsCard>

      <!-- [P2#7] media_library LRU 配置 + 立即清理 -->
      <SettingsCard title="TMDB 媒体库 LRU 管理（§15.2）" :loading="loading">
        <SettingsRow label="当前条目数">
          <span class="settings-help">{{ indexedTotal }}</span>
        </SettingsRow>
        <SettingsRow label="上限 (max_rows)">
          <AppInput
            :model-value="lruMaxRows"
            class="config-input"
            placeholder="5000"
            @update:model-value="lruMaxRows = String($event ?? '')"
          />
          <AppButton
            type="button" size="sm" variant="ghost" class="row-action-btn"
            :disabled="savingKey === LRU_MAX_ROWS_KEY"
            @click="saveConfig(LRU_MAX_ROWS_KEY, lruMaxRows, '上限')"
          >
            {{ savingKey === LRU_MAX_ROWS_KEY ? "保存中…" : "保存" }}
          </AppButton>
        </SettingsRow>
        <SettingsRow label="保留 (keep_rows)">
          <AppInput
            :model-value="lruKeepRows"
            class="config-input"
            placeholder="3000"
            @update:model-value="lruKeepRows = String($event ?? '')"
          />
          <AppButton
            type="button" size="sm" variant="ghost" class="row-action-btn"
            :disabled="savingKey === LRU_KEEP_ROWS_KEY"
            @click="saveConfig(LRU_KEEP_ROWS_KEY, lruKeepRows, '保留')"
          >
            {{ savingKey === LRU_KEEP_ROWS_KEY ? "保存中…" : "保存" }}
          </AppButton>
        </SettingsRow>
        <SettingsRow label="操作">
          <AppButton
            type="button" variant="primary"
            :disabled="lruEvictBusy" @click="triggerEvictNow"
          >
            {{ lruEvictBusy ? "清理中…" : "立即清理" }}
          </AppButton>
          <span class="settings-help row-action-btn">手动触发 LRU 淘汰（保护收藏/订阅/有播放记录）</span>
        </SettingsRow>
      </SettingsCard>
    </div>

    <div v-show="activeTab === TMDB_TAB">
      <SettingsCard title="TMDB 配置（Phase 8）" :loading="loading">
        <SettingsRow label="API Key">
          <AppInput v-model="configs.tmdb_api_key" placeholder="未配置（演示模式）" class="config-input" />
        </SettingsRow>
        <SettingsRow label="语言">
          <AppInput v-model="configs.tmdb_language" placeholder="zh-CN" class="config-input" />
        </SettingsRow>
        <SettingsRow label="操作">
          <div class="row-actions">
            <AppButton
              type="button"
              variant="primary"
              :disabled="savingKey === 'tmdb'"
              @click="saveConfig('tmdb_api_key', configs.tmdb_api_key ?? '', 'TMDB Key')"
            >
              保存
            </AppButton>
            <AppButton type="button" variant="ghost" :disabled="testing === 'tmdb'" @click="testTmdb">
              测试连通
            </AppButton>
          </div>
        </SettingsRow>
      </SettingsCard>
    </div>

    <div v-show="activeTab === PANSEARCH_TAB">
      <SettingsCard title="盘搜服务配置（Phase 8）" :loading="loading">
        <SettingsRow label="服务地址">
          <AppInput v-model="configs.pansearch_url" placeholder="http://localhost:8888" class="config-input" />
        </SettingsRow>
        <SettingsRow label="操作">
          <div class="row-actions">
            <AppButton
              type="button"
              variant="primary"
              :disabled="savingKey === 'pansearch'"
              @click="saveConfig('pansearch_url', configs.pansearch_url ?? '', '盘搜地址')"
            >
              保存
            </AppButton>
            <AppButton type="button" variant="ghost" :disabled="testing === 'pansearch'" @click="testPansearch">
              测试连通
            </AppButton>
          </div>
        </SettingsRow>
      </SettingsCard>
    </div>

    <div v-show="activeTab === NAS_TAB">
      <!-- [V7 §9.4+ 扩展 G18] 主机路径 → 容器路径 映射管理卡片 -->
      <SettingsCard title="NAS 主机路径映射（[V7 §9.4+ 扩展 G18]）" :loading="nasMountsLoading">
        <SettingsRow label="操作">
          <div class="row-actions">
            <AppButton type="button" variant="ghost" :disabled="nasProbeBusy" @click="probeNASMountsNow">
              {{ nasProbeBusy ? "探测中…" : "重新探测挂载" }}
            </AppButton>
            <AppButton type="button" variant="ghost" :disabled="nasMountsLoading" @click="loadAll">
              刷新列表
            </AppButton>
          </div>
          <p class="row-hint">
            主机路径由后端按以下优先级自动映射：① 手动添加的映射（见下表）②
            <code>/proc/self/mountinfo</code> SMB/cifs 探测 ③
            SMB 源别名推导（从 //内网IP/share 自动识别，命中后自动保存为手动映射）④
            原样透传（容器内路径）。新增媒体源时，输入主机路径即可，下方有实时预览。
          </p>
        </SettingsRow>

        <SettingsRow label="手动映射">
          <div v-if="nasMounts.length === 0" class="nas-empty">
            暂无手动映射。主机路径会按"自动探测"规则处理。
          </div>
          <table v-else class="nas-table">
            <thead>
              <tr>
                <th>主机路径</th>
                <th>容器内路径</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="m in nasMounts" :key="m.host_path">
                <td class="path-cell" :title="m.host_path">{{ m.host_path }}</td>
                <td class="path-cell" :title="m.container_path">{{ m.container_path }}</td>
                <td class="actions-cell">
                  <AppButton
                    type="button"
                    variant="ghost"
                    :disabled="nasDeleteMountBusy === m.host_path"
                    @click="deleteNASMountByHost(m.host_path)"
                  >
                    {{ nasDeleteMountBusy === m.host_path ? "删除中…" : "删除" }}
                  </AppButton>
                </td>
              </tr>
            </tbody>
          </table>
        </SettingsRow>

        <SettingsRow label="自动探测（仅展示）">
          <div v-if="nasDetected.length === 0" class="nas-empty">
            暂无自动探测到的 SMB/cifs 挂载。点击"重新探测挂载"试试。
          </div>
          <table v-else class="nas-table">
            <thead>
              <tr>
                <th>挂载点</th>
                <th>文件系统</th>
                <th>源</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(d, idx) in nasDetected" :key="`${d.mount_target}-${idx}`">
                <td class="path-cell" :title="d.mount_target">{{ d.mount_target }}</td>
                <td>{{ d.filesystem }}</td>
                <td class="path-cell" :title="d.source">{{ d.source }}</td>
              </tr>
            </tbody>
          </table>
        </SettingsRow>
      </SettingsCard>

      <SettingsCard title="NAS 媒体源（[V7 §9.4+ 扩展 G1.C]）" :loading="nasLoading">
        <SettingsRow label="添加媒体源">
          <div class="nas-add-row">
            <AppInput v-model="nasNewName" placeholder="名称（可读标识）" class="config-input" />
            <AppInput
              v-model="nasNewPath"
              placeholder="主机路径（绝对路径，如 /mnt/BTORAGE，后端自动映射）"
              class="config-input"
              @input="scheduleResolvePreview"
            />
            <AppButton type="button" variant="primary" :disabled="nasCreating" @click="createNASSource">
              {{ nasCreating ? "添加中…" : "添加" }}
            </AppButton>
            <AppButton type="button" variant="ghost" :disabled="!nasNewPath || nasTestingPath" @click="testNewPath">
              {{ nasTestingPath ? "检测中…" : "测试路径" }}
            </AppButton>
          </div>
          <!-- [V7 §9.4+ 扩展 G18] 实时预览：debounce 300ms 后展示"映射成什么" -->
          <div
            v-if="nasNewPath.trim()"
            class="nas-resolve-preview"
            :class="{
              'is-explicit': nasResolvePreview?.source === 'explicit',
              'is-auto_detected': nasResolvePreview?.source === 'auto_detected',
              'is-smb_alias': nasResolvePreview?.source === 'smb_alias',
              'is-passthrough': nasResolvePreview?.source === 'passthrough',
            }"
          >
            <span v-if="nasResolveBusy">⏳ 解析中…</span>
            <span v-else-if="nasResolvePreview">
              预览 →
              <code class="path-snippet">{{ nasResolvePreview.resolved }}</code>
              <span class="source-tag">[{{ sourceLabel(nasResolvePreview.source) }}]</span>
              <span v-if="nasResolvePreview.resolved !== nasResolvePreview.input" class="diff-marker">
                （重写过）
              </span>
            </span>
            <span v-else class="dim">输入合法路径后展示映射预览</span>
          </div>
          <div v-if="nasTestResult" class="nas-test-result" :class="nasTestResult.exists && nasTestResult.is_dir && nasTestResult.readable ? 'is-ok' : 'is-not_accessible'">
            <span v-if="nasTestResult.exists && nasTestResult.is_dir && nasTestResult.readable">
              ✅ 可读 · 浅层文件数 {{ nasTestResult.file_count }}
              <span v-if="nasTestResult.sample && nasTestResult.sample.length"> · 子目录: {{ nasTestResult.sample.join(", ") }}</span>
            </span>
            <span v-else-if="!nasTestResult.exists">❌ 路径不存在</span>
            <span v-else-if="!nasTestResult.is_dir">⚠ 不是目录</span>
            <span v-else>⚠ 无读取权限</span>
          </div>
        </SettingsRow>

        <SettingsRow label="媒体源列表">
          <div v-if="nasSources.length === 0" class="nas-empty">
            尚未配置 NAS 媒体源。在上方添加后点击"扫描"将进入索引引擎。
          </div>
          <table v-else class="nas-table">
            <thead>
              <tr>
                <th>名称</th>
                <th>路径</th>
                <th>可访问</th>
                <th>文件数</th>
                <th>启用</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="src in nasSources" :key="src.id" :class="{ 'is-disabled': !src.enabled }">
                <td>{{ src.name }}</td>
                <td class="path-cell" :title="src.path">{{ src.path }}</td>
                <td>
                  <AdminStatusPill
                    :status="src.last_accessibility === 'ok' ? 'ok' : src.last_accessibility === 'not_accessible' ? 'error' : 'pending'"
                    :label="src.last_accessibility === 'ok' ? '可读' : src.last_accessibility === 'not_accessible' ? '不可读' : '未知'"
                  />
                </td>
                <td>{{ src.file_count }}</td>
                <td>
                  <AppButton type="button" variant="ghost" :disabled="nasBusyId === src.id" @click="toggleNAS(src.id)">
                    {{ src.enabled ? "已启用" : "已禁用" }}
                  </AppButton>
                </td>
                <td class="actions-cell">
                  <AppButton type="button" variant="ghost" :disabled="nasBusyId === src.id" @click="deleteNAS(src.id, src.name)">删除</AppButton>
                </td>
              </tr>
            </tbody>
          </table>
        </SettingsRow>

        <SettingsRow label="扫描与健康检查">
          <div class="row-actions">
            <AppButton type="button" variant="primary" :disabled="scanning" @click="triggerScan('full')">
              {{ scanning ? "扫描中…" : "全量扫描" }}
            </AppButton>
            <AppButton type="button" variant="ghost" :disabled="scanning" @click="triggerScan('incremental')">
              {{ scanning ? "扫描中…" : "增量扫描" }}
            </AppButton>
            <AppButton type="button" variant="ghost" :disabled="nasBulkHealthBusy" @click="runBulkHealth">
              {{ nasBulkHealthBusy ? "检测中…" : "全部可访问性检测" }}
            </AppButton>
            <!-- [76007b2 UI-first] 历史宿主视角路径一键改写为容器内路径 -->
            <AppButton type="button" variant="ghost" :disabled="nasReresolveBusy" @click="runReresolve">
              {{ nasReresolveBusy ? "重映射中…" : "批量重新映射路径" }}
            </AppButton>
          </div>
          <p class="row-hint" v-if="nasSources.length === 0">
            ⚠ 当前没有 NAS 媒体源。请先在上方表单填入真实 SMB 路径（如 /mnt/BTORAGE/Asia-Movie），系统会自动识别内网挂载并完成映射入库。
          </p>
          <p class="row-hint" v-else>
            可达性每 5 分钟自动复测并刷新「可访问」列；「全部可访问性检测」会同时统计文件数。「批量重新映射路径」用于把历史版本保存的主机视角路径改写为容器内路径。
          </p>
        </SettingsRow>
      </SettingsCard>
    </div>

    <div v-show="activeTab === HEALTH_TAB">
      <SettingsCard title="健康检查（§27.4）" :loading="loading">
        <SettingsRow label="总体状态">
          <AdminStatusPill :tone="health?.validation?.ok ? 'success' : 'warning'">
            {{ health?.validation?.ok ? "全部通过" : "存在警告" }}
          </AdminStatusPill>
        </SettingsRow>
        <SettingsRow label="TMDB Key">
          <AdminStatusPill :tone="toneFor(health?.validation?.tmdb_key?.status)">
            {{ health?.validation?.tmdb_key?.message ?? "未知" }}
          </AdminStatusPill>
          <!-- [P0#2] 异常时引导用户去 TMDB 配置 Tab -->
          <AppButton
            v-if="health?.validation?.tmdb_key?.status !== 'ok'"
            type="button" size="sm" variant="ghost" class="row-action-btn"
            @click="goToTab(TMDB_TAB)"
          >
            去配置
          </AppButton>
        </SettingsRow>
        <SettingsRow label="PanSou">
          <AdminStatusPill :tone="toneFor(health?.validation?.pansearch_url?.status)">
            {{ health?.validation?.pansearch_url?.message ?? "未知" }}
          </AdminStatusPill>
          <!-- [P0#2] 异常时引导用户去盘搜配置 Tab -->
          <AppButton
            v-if="health?.validation?.pansearch_url?.status !== 'ok'"
            type="button" size="sm" variant="ghost" class="row-action-btn"
            @click="goToTab(PANSEARCH_TAB)"
          >
            去配置
          </AppButton>
        </SettingsRow>
        <SettingsRow label="NAS 状态（§27.4）">
          <AdminStatusPill :tone="nasStatusTone">
            {{ nasStatusLabel }}
          </AdminStatusPill>
          <AppButton type="button" size="sm" variant="ghost" class="row-action-btn" @click="goToTab(NAS_TAB)">
            去配置
          </AppButton>
        </SettingsRow>
        <SettingsRow label="NAS 媒体源">
          <span class="settings-help">
            <template v-if="nasInaccessibleCount > 0">
              ⚠ {{ nasInaccessibleCount }} 个不可访问（共 {{ nasSources.length }} 个）
            </template>
            <template v-else-if="nasSources.length > 0">
              全部可访问（共 {{ nasSources.length }} 个）
            </template>
            <template v-else>
              尚未添加媒体源
            </template>
          </span>
          <!-- [P0#2] 引导用户去 NAS 配置 Tab 排查 -->
          <AppButton type="button" size="sm" variant="ghost" class="row-action-btn" @click="goToTab(NAS_TAB)">
            查看
          </AppButton>
          <AppButton
            type="button" size="sm" variant="ghost" class="row-action-btn"
            :disabled="nasProbeBusy" @click="probeNASMountsNow"
          >
            {{ nasProbeBusy ? "探测中…" : "重新探测" }}
          </AppButton>
        </SettingsRow>
        <SettingsRow label="索引">
          <span class="settings-help">
            <template v-if="snapshot?.index_scanning">扫描中…</template>
            <template v-else-if="nasIndexComplete">已完成（{{ indexedTotal }} 条）</template>
            <template v-else>未完成</template>
          </span>
          <!-- [P0#2] 触发全盘扫描 -->
          <AppButton
            type="button" size="sm" variant="ghost" class="row-action-btn"
            :disabled="healthScanBusy" @click="triggerHealthFullScan"
          >
            {{ healthScanBusy ? "启动中…" : "触发全盘扫描" }}
          </AppButton>
        </SettingsRow>
        <SettingsRow label="日志">
          <span class="settings-help">查看最近告警与错误</span>
          <!-- [P0#2] 打开日志弹窗 -->
          <AppButton
            type="button" size="sm" variant="ghost" class="row-action-btn"
            @click="openHealthLogs"
          >
            查看日志
          </AppButton>
        </SettingsRow>
        <SettingsRow label="操作">
          <AppButton type="button" variant="ghost" :disabled="loading" @click="loadAll">重新检查</AppButton>
        </SettingsRow>
      </SettingsCard>
    </div>

    <!-- [P0#2] 健康检查"查看日志"弹窗 -->
    <AppModal
      :open="healthLogModalOpen"
      title="最近日志（最多 50 条）"
      size="lg"
      @close="closeHealthLogs"
    >
      <div class="health-log-filters">
        <span class="settings-help">级别过滤：</span>
        <AppButton
          type="button" size="sm" variant="ghost"
          :class="{ 'is-active': healthLogLevel === 0 }"
          @click="healthLogLevel = 0; openHealthLogs()"
        >全部</AppButton>
        <AppButton
          type="button" size="sm" variant="ghost"
          :class="{ 'is-active': healthLogLevel === 30 }"
          @click="healthLogLevel = 30; openHealthLogs()"
        >WARNING</AppButton>
        <AppButton
          type="button" size="sm" variant="ghost"
          :class="{ 'is-active': healthLogLevel === 40 }"
          @click="healthLogLevel = 40; openHealthLogs()"
        >ERROR</AppButton>
      </div>
      <div v-if="healthLogLoading" class="health-log-loading">加载中…</div>
      <div v-else-if="healthLogEntries.length === 0" class="health-log-empty">该级别暂无日志</div>
      <table v-else class="health-log-table">
        <thead>
          <tr>
            <th class="col-time">时间</th>
            <th class="col-level">级别</th>
            <th class="col-module">模块</th>
            <th class="col-msg">消息</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="e in healthLogEntries" :key="e.id">
            <td class="col-time">{{ e.timestamp }}</td>
            <td class="col-level">{{ e.level_emoji }} {{ e.level_name }}</td>
            <td class="col-module">{{ e.module_name }}</td>
            <td class="col-msg">{{ e.message }}</td>
          </tr>
        </tbody>
      </table>
    </AppModal>
  </div>
</template>

<style scoped>
.config-input {
  max-width: 420px;
}
.row-actions {
  display: flex;
  gap: 8px;
}

/* [P0#2] 健康检查面板行内引导按钮（去配置/查看/重新探测/触发扫描/查看日志） */
.row-action-btn {
  margin-left: 8px;
}
/* [P0#2] 日志弹窗样式 */
.health-log-filters {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 12px;
  padding-bottom: 10px;
  border-bottom: 1px solid var(--color-border, rgba(0, 0, 0, 0.06));
}
.health-log-filters .is-active {
  background: var(--brand, #3b82f6);
  color: #fff;
  border-color: var(--brand, #3b82f6);
}
.health-log-loading,
.health-log-empty {
  padding: 24px;
  text-align: center;
  color: var(--color-text-secondary, #6b7280);
  font-size: 13px;
}
.health-log-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 12px;
  font-family: ui-monospace, SFMono-Regular, monospace;
}
.health-log-table th,
.health-log-table td {
  padding: 6px 10px;
  text-align: left;
  border-bottom: 1px solid var(--color-border, rgba(0, 0, 0, 0.06));
  vertical-align: top;
}
.health-log-table th {
  font-weight: 600;
  color: var(--color-text-secondary, #6b7280);
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}
.health-log-table .col-time { white-space: nowrap; width: 160px; }
.health-log-table .col-level { white-space: nowrap; width: 110px; }
.health-log-table .col-module { white-space: nowrap; width: 80px; }
.health-log-table .col-msg { word-break: break-word; }
.settings-help {
  color: var(--color-text-secondary, #6b7280);
  font-size: 13px;
}
/* [V7 §9.4+ 扩展 G18] NAS 媒体源 CRUD 表格样式 */
.nas-status-line {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}
.nas-add-row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}
.nas-add-row .config-input {
  flex: 1 1 200px;
  min-width: 160px;
}
.nas-test-result {
  margin-top: 8px;
  padding: 8px 12px;
  border-radius: 6px;
  font-size: 13px;
}
.nas-test-result.is-ok {
  background: rgba(0, 200, 100, 0.12);
  color: #047857;
}
.nas-test-result.is-not_accessible {
  background: rgba(220, 38, 38, 0.12);
  color: #b91c1c;
}
.nas-test-result.is-unknown {
  background: rgba(120, 120, 120, 0.08);
  color: #4b5563;
}
.nas-empty {
  padding: 12px;
  color: var(--color-text-secondary, #6b7280);
  font-size: 13px;
}
.nas-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}
.nas-table th,
.nas-table td {
  padding: 8px 10px;
  text-align: left;
  border-bottom: 1px solid var(--color-border, rgba(0, 0, 0, 0.06));
}
.nas-table tr.is-disabled {
  opacity: 0.55;
}
.nas-table .path-cell {
  max-width: 360px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: ui-monospace, SFMono-Regular, monospace;
  font-size: 12px;
}
.nas-table .actions-cell {
  display: flex;
  gap: 6px;
}
.row-hint {
  margin: 6px 0 0;
  font-size: 12px;
  color: var(--color-text-secondary, #6b7280);
}
/* [V7 §9.4+ 扩展 G18] 实时预览 + 路径片段样式 */
.nas-resolve-preview {
  margin-top: 8px;
  padding: 8px 12px;
  border-radius: 6px;
  font-size: 13px;
  background: rgba(120, 120, 120, 0.08);
  color: var(--color-text-secondary, #4b5563);
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
}
.nas-resolve-preview.is-explicit {
  background: rgba(0, 200, 100, 0.12);
  color: #047857;
}
.nas-resolve-preview.is-auto_detected {
  background: rgba(110, 108, 240, 0.12);
  color: #4338ca;
}
/* [76007b2] SMB 别名映射：青色系，与"已自动保存"语义对应 */
.nas-resolve-preview.is-smb_alias {
  background: rgba(14, 165, 160, 0.12);
  color: #0f766e;
}
.nas-resolve-preview.is-passthrough {
  background: rgba(120, 120, 120, 0.08);
  color: #4b5563;
}
.nas-resolve-preview .path-snippet {
  font-family: ui-monospace, SFMono-Regular, monospace;
  font-size: 12px;
  background: rgba(0, 0, 0, 0.06);
  padding: 1px 6px;
  border-radius: 4px;
}
.nas-resolve-preview .source-tag {
  font-size: 11px;
  padding: 1px 6px;
  border-radius: 999px;
  background: rgba(0, 0, 0, 0.08);
}
.nas-resolve-preview .diff-marker {
  font-size: 11px;
  color: var(--color-text-secondary, #6b7280);
}
.nas-resolve-preview .dim {
  color: var(--color-text-secondary, #9ca3af);
  font-size: 12px;
}
</style>
