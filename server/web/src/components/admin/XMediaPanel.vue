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
  testTmdbKey,
  fetchNASSources,
  createNASSource as createNASSourceRequest,
  toggleNASSource,
  deleteNASSource,
  testNASPath,
  bulkNASHealth,
  type HealthStatus,
  type StateSnapshot,
  type NASSource,
  type NASTestPathResult,
} from "@/api/xmedia";
import { useSectionTabRoute } from "@/composables/useSectionTabRoute";
import { toast } from "@/composables/useToast";
import "@/styles/admin-shared.css";

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
const savingKey = ref("");
const testing = ref("");
const scanning = ref(false);

const nasAvailable = computed(() => !!snapshot.value?.capabilities.nas_available);
const nasIndexComplete = computed(() => !!snapshot.value?.capabilities.nas_index_complete);
const pansearchAvailable = computed(() => !!snapshot.value?.capabilities.pansearch_available);
const indexProgress = computed(() => snapshot.value?.index_progress ?? null);
const indexedTotal = computed(() => snapshot.value?.indexed_total ?? 0);

let pollTimer: number | null = null;

async function loadAll() {
  loading.value = true;
  nasLoading.value = true;
  // [V7 整改 E3] fetchSnapshot 必须自带 .catch(null) —— 否则任何 /api/state/snapshot
  // 500 都让整页 reject, 其它 tab 跟着 "请求失败 (500)" 假象拖垮 (Promise.all 短路语义)。
  // 正确做法: 每个 promise 各自 catch 返回零值, 这里只用 Promise.all 拿顺序, 不让它 reject。
  // 对比: DashboardManagement.vue 用 Promise.allSettled 走同条接口, 没有这个 bug。
  try {
    const [snap, h, cfg, nasList] = await Promise.all([
      fetchSnapshot().catch(() => null),
      fetchHealth().catch(() => null),
      fetchXMediaConfigs().catch(() => ({})),
      fetchNASSources().catch(() => []),
    ]);
    // null-safe 赋值: 任一接口失败时不污染其它状态
    if (snap) snapshot.value = snap;
    if (h) health.value = h;
    if (cfg) configs.value = cfg;
    if (nasList) nasSources.value = nasList;
  } catch (e) {
    toast.error(getApiErrorMessage(e, "状态读取失败"));
  } finally {
    loading.value = false;
    nasLoading.value = false;
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

async function saveConfig(key: string, value: string, label: string) {
  savingKey.value = key;
  try {
    await saveXMediaConfig(key, value.trim());
    configs.value = { ...configs.value, [key]: value.trim() };
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
        <SettingsRow label="NAS 可用">
          <AdminStatusPill :tone="nasAvailable ? 'success' : 'warning'">
            {{ nasAvailable ? "可用" : "不可用" }}
          </AdminStatusPill>
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
      <SettingsCard title="NAS 媒体源（[V7 §9.4+ 扩展 G1.C]）" :loading="nasLoading">
        <SettingsRow label="添加媒体源">
          <div class="nas-add-row">
            <AppInput v-model="nasNewName" placeholder="名称（可读标识）" class="config-input" />
            <AppInput v-model="nasNewPath" placeholder="路径（容器内绝对路径，如 /mnt/nas-root/Asia-Movie）" class="config-input" />
            <AppButton type="button" variant="primary" :disabled="nasCreating" @click="createNASSource">
              {{ nasCreating ? "添加中…" : "添加" }}
            </AppButton>
            <AppButton type="button" variant="ghost" :disabled="!nasNewPath || nasTestingPath" @click="testNewPath">
              {{ nasTestingPath ? "检测中…" : "测试路径" }}
            </AppButton>
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
          </div>
          <p class="row-hint" v-if="nasSources.length === 0">
            ⚠ 当前没有 NAS 媒体源。请先在上方表单添加主机路径（后端会自动映射到容器内路径）后再点击扫描。
          </p>
          <p class="row-hint" v-else>
            提示：NAS 媒体源独立启停；启用后才会参与扫描、P0 智能跳过命中、P1 转存候选。
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
        </SettingsRow>
        <SettingsRow label="PanSou">
          <AdminStatusPill :tone="toneFor(health?.validation?.pansearch_url?.status)">
            {{ health?.validation?.pansearch_url?.message ?? "未知" }}
          </AdminStatusPill>
        </SettingsRow>
        <SettingsRow label="操作">
          <AppButton type="button" variant="ghost" :disabled="loading" @click="loadAll">重新检查</AppButton>
        </SettingsRow>
      </SettingsCard>
    </div>
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
.settings-help {
  color: var(--color-text-secondary, #6b7280);
  font-size: 13px;
}
/* [V7 §9.4+ 扩展 G18] NAS 媒体源 CRUD 表格样式 */
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
</style>
