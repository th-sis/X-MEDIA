<script setup lang="ts">
// V7 §11.1 网盘优先级配置：NAS 可关闭 + 网盘排序（↑/↓）+ 已登录校验 + 磁力兜底。
import { computed, onMounted, ref } from "vue";
import SettingsCard from "@/components/admin/SettingsCard.vue";
import SettingsRow from "@/components/admin/SettingsRow.vue";
import AppButton from "@/components/base/AppButton.vue";
import AppSelect from "@/components/base/AppSelect.vue";
import { http } from "@/api/client";
import { getApiErrorMessage } from "@/api/client";
import { driversApi } from "@/api/drivers";
import { accountsApi } from "@/api/accounts";
import type { DriverInfo } from "@/api/types";
import type { Account } from "@/api/types";
import { fetchXMediaConfigs } from "@/api/xmedia";
import { useAdminPageLoading } from "@/composables/useAdminLoadingBar";
import { toast } from "@/composables/useToast";
import "@/styles/admin-shared.css";

// V7 §11.1:网盘优先级默认顺序。NAS 在最前,5 个网盘驱动随后。
const DRIVER_ORDER = ["nas", "pan115", "quark", "pan123", "baidu", "guangya"];

interface PriorityRow {
  key: string;
  label: string;
  type: "nas" | "driver";
  loggedIn: boolean;
}

const configs = ref<Record<string, string>>({});
const drivers = ref<DriverInfo[]>([]);
const accounts = ref<Account[]>([]);
const loading = ref(false);
const saving = ref(false);
useAdminPageLoading("priority", loading);

const NAS_ENABLED_KEY = "nas_enabled";
const PRIORITY_KEY = "resolve_priority";
const MAGNET_ENABLED_KEY = "resolve_magnet_enabled";
const MAGNET_TARGET_KEY = "resolve_magnet_target";

const nasEnabled = computed({
  get: () => (configs.value[NAS_ENABLED_KEY] ?? "true") === "true",
  set: (v) => { configs.value[NAS_ENABLED_KEY] = v ? "true" : "false"; },
});
const magnetEnabled = computed({
  get: () => (configs.value[MAGNET_ENABLED_KEY] ?? "true") === "true",
  set: (v) => { configs.value[MAGNET_ENABLED_KEY] = v ? "true" : "false"; },
});
const magnetTarget = ref(configs.value[MAGNET_TARGET_KEY] ?? "pan115");

const driverLabel = (key: string) => {
  if (key === "nas") return "NAS 本地";
  const driver = drivers.value.find((d) => d.name === key);
  return driver?.display_name ?? key;
};

const driverKind = (key: string) => drivers.value.find((d) => d.name === key)?.auth_type ?? "";

const loggedInDrivers = computed(() =>
  accounts.value
    .filter((a) => a.is_active)
    .map((a) => a.driver_type),
);

// §11.1：构造可编辑的优先级行。NAS 受开关控制;网盘按服务端返回的已登录状态灰显。
const priorityRows = computed<PriorityRow[]>(() => {
  const raw = configs.value[PRIORITY_KEY];
  let list: string[] = [];
  try {
    list = JSON.parse(raw || "[]");
  } catch {
    list = [...DRIVER_ORDER];
  }
  return list.map((key) => {
    const isLogged = key === "nas" ? true : loggedInDrivers.value.includes(key);
    return {
      key,
      label: driverLabel(key),
      type: key === "nas" ? "nas" : "driver",
      loggedIn: isLogged,
    };
  });
});

// 网盘下拉（磁力兜底下载到哪）
const magnetTargetOptions = computed(() => {
  return priorityRows.value
    .filter((row) => row.type === "driver" && row.loggedIn)
    .map((row) => ({ value: row.key, label: row.label }));
});

function moveUp(idx: number) {
  if (idx <= 0) return;
  const rows = [...priorityRows.value];
  const tmp = rows[idx - 1];
  rows[idx - 1] = rows[idx];
  rows[idx] = tmp;
  configs.value[PRIORITY_KEY] = JSON.stringify(rows.map((r) => r.key));
}

function moveDown(idx: number) {
  const rows = [...priorityRows.value];
  if (idx >= rows.length - 1) return;
  const tmp = rows[idx + 1];
  rows[idx + 1] = rows[idx];
  rows[idx] = tmp;
  configs.value[PRIORITY_KEY] = JSON.stringify(rows.map((r) => r.key));
}

function resetToDefault() {
  configs.value[PRIORITY_KEY] = JSON.stringify([...DRIVER_ORDER]);
}

async function loadAll() {
  loading.value = true;
  try {
    const [cfg, drv, accts] = await Promise.all([
      fetchXMediaConfigs().catch(() => ({})),
      driversApi.list().catch(() => []),
      accountsApi.list().catch(() => []),
    ]);
    configs.value = cfg;
    drivers.value = drv;
    accounts.value = accts;
    magnetTarget.value = configs.value[MAGNET_TARGET_KEY] ?? "pan115";
  } catch (e) {
    toast.error(getApiErrorMessage(e, "读取优先级失败"));
  } finally {
    loading.value = false;
  }
}

async function saveAll() {
  saving.value = true;
  try {
    configs.value[MAGNET_TARGET_KEY] = magnetTarget.value;
    await Promise.all([
      http.put("/admin/configs/", { key: NAS_ENABLED_KEY, value: configs.value[NAS_ENABLED_KEY] }),
      http.put("/admin/configs/", { key: PRIORITY_KEY, value: configs.value[PRIORITY_KEY] }),
      http.put("/admin/configs/", { key: MAGNET_ENABLED_KEY, value: configs.value[MAGNET_ENABLED_KEY] }),
      http.put("/admin/configs/", { key: MAGNET_TARGET_KEY, value: configs.value[MAGNET_TARGET_KEY] }),
    ]);
    toast.success("网盘优先级已保存，能力预检将通过 WS 推送给客户端");
  } catch (e) {
    toast.error(getApiErrorMessage(e, "保存失败"));
  } finally {
    saving.value = false;
  }
}

const hasUnloggedInDriver = computed(() =>
  priorityRows.value.some((row) => row.type === "driver" && !row.loggedIn),
);

const priorityHasNas = computed(() => priorityRows.value.some((r) => r.key === "nas"));

onMounted(() => {
  void loadAll();
});
</script>

<template>
  <div class="settings">
    <SettingsCard title="网盘优先级（§11.1）" :loading="loading">
      <SettingsRow label="启用 NAS 本地索引">
        <label class="toggle">
          <input v-model="nasEnabled" type="checkbox" />
          <span>{{ nasEnabled ? "已启用" : "已关闭" }}</span>
        </label>
      </SettingsRow>

      <SettingsRow label="网盘优先级">
        <p class="settings-help">数字越小优先级越高。运行时若网盘未登录则自动跳过（§11.1.1）。</p>
        <ul v-if="priorityRows.length" class="priority-list">
          <li
            v-for="(row, idx) in priorityRows"
            :key="row.key"
            class="priority-row"
            :class="{ 'is-disabled': row.type === 'driver' && !row.loggedIn }"
          >
            <span class="priority-row__index">{{ idx + 1 }}</span>
            <span class="priority-row__label">
              {{ row.label }}
              <span v-if="row.type === 'driver' && !row.loggedIn" class="priority-row__warn" :title="`${row.label} 未登录，运行时将被自动跳过`">
                ⚠️ 未登录
              </span>
            </span>
            <span class="priority-row__type">{{ driverKind(row.key) || (row.type === "nas" ? "本地" : "") }}</span>
            <span class="priority-row__actions">
              <button type="button" :disabled="idx === 0" @click="moveUp(idx)">↑</button>
              <button type="button" :disabled="idx === priorityRows.length - 1" @click="moveDown(idx)">↓</button>
            </span>
          </li>
        </ul>
        <p v-else class="settings-help">未配置优先级</p>
        <p v-if="hasUnloggedInDriver" class="settings-help is-warn">
          ⚠️ 优先级列表中存在未登录的网盘，请先在「存储管理」添加账号，否则运行时会被跳过。
        </p>
        <p v-if="!priorityHasNas" class="settings-help is-warn">
          ⚠️ 当前未启用 NAS 本地索引，P0 秒播将不可用。
        </p>
      </SettingsRow>

      <SettingsRow label="操作">
        <div class="row-actions">
          <AppButton type="button" variant="ghost" :disabled="loading || saving" @click="loadAll">刷新</AppButton>
          <AppButton type="button" variant="ghost" :disabled="saving" @click="resetToDefault">恢复默认顺序</AppButton>
          <AppButton type="button" variant="primary" :disabled="saving" @click="saveAll">
            {{ saving ? "保存中..." : "保存" }}
          </AppButton>
        </div>
      </SettingsRow>
    </SettingsCard>

    <SettingsCard title="磁力兜底（§6.5 P2）" :loading="loading">
      <SettingsRow label="启用磁力兜底">
        <label class="toggle">
          <input v-model="magnetEnabled" type="checkbox" />
          <span>{{ magnetEnabled ? "已启用" : "已关闭" }}</span>
        </label>
      </SettingsRow>

      <SettingsRow label="磁力下载到">
        <AppSelect
          v-model="magnetTarget"
          :disabled="!magnetEnabled || magnetTargetOptions.length === 0"
          :options="magnetTargetOptions"
          placeholder="请先在优先级列表中保留至少一个已登录网盘"
        />
      </SettingsRow>
    </SettingsCard>
  </div>
</template>

<style scoped>
.settings {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.priority-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
  max-width: 520px;
}

.priority-row {
  display: grid;
  grid-template-columns: 32px 1fr auto auto;
  gap: 10px;
  align-items: center;
  padding: 8px 12px;
  border: 1px solid var(--border-soft);
  border-radius: var(--radius-md);
  background: var(--surface);
}

.priority-row.is-disabled {
  opacity: 0.55;
  border-style: dashed;
}

.priority-row__index {
  font-weight: 600;
  color: var(--brand);
  text-align: center;
}

.priority-row__label {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 500;
}

.priority-row__warn {
  color: var(--warn, #f59e0b);
  font-size: 12px;
  font-weight: 500;
}

.priority-row__type {
  font-size: 12px;
  color: var(--color-text-secondary, #6b7280);
}

.priority-row__actions {
  display: flex;
  gap: 4px;
}

.priority-row__actions button {
  width: 28px;
  height: 28px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--border-soft);
  background: var(--surface);
  color: var(--color-text-primary);
  cursor: pointer;
}

.priority-row__actions button:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.toggle {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  font-size: 13px;
}

.toggle input {
  width: 16px;
  height: 16px;
}

.settings-help {
  font-size: 12px;
  color: var(--color-text-secondary, #6b7280);
  margin: 0 0 6px;
}

.settings-help.is-warn {
  color: var(--warn, #f59e0b);
}

.row-actions {
  display: flex;
  gap: 8px;
}
</style>