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
  type HealthStatus,
  type StateSnapshot,
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
  try {
    const [snap, h, cfg] = await Promise.all([
      fetchSnapshot(),
      fetchHealth().catch(() => null),
      fetchXMediaConfigs().catch(() => ({})),
    ]);
    snapshot.value = snap;
    health.value = h;
    configs.value = cfg;
  } catch (e) {
    toast.error(getApiErrorMessage(e, "状态读取失败"));
  } finally {
    loading.value = false;
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

async function testNasReadable() {
  testing.value = "nas";
  try {
    await loadAll();
    if (snapshot.value?.capabilities.nas_available) toast.success("NAS 路径可读，能力预检通过");
    else toast.error("NAS 路径不可读，请检查路径与权限");
  } catch (e) {
    toast.error(getApiErrorMessage(e, "NAS 测试失败"));
  } finally {
    testing.value = "";
  }
}

async function triggerScan(mode: "full" | "incremental") {
  scanning.value = true;
  try {
    await startNasScan(mode);
    toast.success(mode === "full" ? "全量扫描已启动" : "增量扫描已启动");
  } catch (e) {
    toast.error(getApiErrorMessage(e, "扫描启动失败"));
  } finally {
    scanning.value = false;
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
              全量扫描
            </AppButton>
            <AppButton type="button" variant="ghost" :disabled="scanning" @click="triggerScan('incremental')">
              增量扫描
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
      <SettingsCard title="NAS 路径配置（Phase 8）" :loading="loading">
        <SettingsRow label="本地媒体路径">
          <AppInput v-model="configs.nas_local_path" placeholder="例如 D:\Media" class="config-input" />
        </SettingsRow>
        <SettingsRow label="操作">
          <div class="row-actions">
            <AppButton
              type="button"
              variant="primary"
              :disabled="savingKey === 'nas'"
              @click="saveConfig('nas_local_path', configs.nas_local_path ?? '', 'NAS 路径')"
            >
              保存
            </AppButton>
            <AppButton type="button" variant="ghost" :disabled="testing === 'nas'" @click="testNasReadable">
              测试可读性
            </AppButton>
          </div>
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
</style>
