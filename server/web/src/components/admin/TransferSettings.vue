<script setup lang="ts">
// V7 §6.9 转存路径与配额配置：
// - 全局：pan_rename_enabled 转存后重命名开关
// - 按账号：pan_save_root_{account_id} 转存根目录 ID
// - 按 driver：pan_quota_warning_{driver} / pan_cleanup_mode_{driver} / pan_cleanup_keep_recent_days_{driver}
import { computed, onMounted, ref } from "vue";
import SettingsCard from "@/components/admin/SettingsCard.vue";
import SettingsRow from "@/components/admin/SettingsRow.vue";
import AppButton from "@/components/base/AppButton.vue";
import AppInput from "@/components/base/AppInput.vue";
import AppSelect from "@/components/base/AppSelect.vue";
import { http, getApiErrorMessage } from "@/api/client";
import { driversApi } from "@/api/drivers";
import { accountsApi } from "@/api/accounts";
import type { DriverInfo } from "@/api/types";
import type { Account } from "@/api/types";
import { fetchXMediaConfigs } from "@/api/xmedia";
import { useAdminPageLoading } from "@/composables/useAdminLoadingBar";
import { toast } from "@/composables/useToast";
import "@/styles/admin-shared.css";

const PREFIX_SAVE_ROOT = "pan_save_root_";
const PREFIX_QUOTA = "pan_quota_warning_";
const PREFIX_CLEANUP_MODE = "pan_cleanup_mode_";
const PREFIX_CLEANUP_KEEP = "pan_cleanup_keep_recent_days_";
const KEY_RENAME_ENABLED = "pan_rename_enabled";

const configs = ref<Record<string, string>>({});
const drivers = ref<DriverInfo[]>([]);
const accounts = ref<Account[]>([]);
const loading = ref(false);
const saving = ref(false);
useAdminPageLoading("transfer", loading);

const renameEnabled = computed({
  get: () => (configs.value[KEY_RENAME_ENABLED] ?? "true") === "true",
  set: (v) => { configs.value[KEY_RENAME_ENABLED] = v ? "true" : "false"; },
});

// 按账号维度聚合
const accountRows = computed(() =>
  accounts.value.map((account) => {
    const driver = drivers.value.find((d) => d.name === account.driver_type);
    return {
      id: account.id,
      name: account.name,
      driverType: account.driver_type,
      driverLabel: driver?.display_name ?? account.driver_type,
      saveRoot: configs.value[`${PREFIX_SAVE_ROOT}${account.id}`] ?? "",
    };
  }),
);

// 按 driver 维度聚合
const driverRows = computed(() => {
  const seen = new Set<string>();
  for (const acc of accounts.value) seen.add(acc.driver_type);
  return Array.from(seen).map((driverType) => {
    const driver = drivers.value.find((d) => d.name === driverType);
    return {
      driverType,
      driverLabel: driver?.display_name ?? driverType,
      quotaGb: configs.value[`${PREFIX_QUOTA}${driverType}`] ?? "5",
      cleanupMode: configs.value[`${PREFIX_CLEANUP_MODE}${driverType}`] ?? "none",
      cleanupKeepDays: configs.value[`${PREFIX_CLEANUP_KEEP}${driverType}`] ?? "7",
    };
  });
});

const cleanupModeOptions = [
  { value: "none", label: "不清理" },
  { value: "periodic", label: "定时清理" },
  { value: "lru", label: "LRU 智能清理" },
];

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
  } catch (e) {
    toast.error(getApiErrorMessage(e, "读取转存配置失败"));
  } finally {
    loading.value = false;
  }
}

async function saveOne(key: string, value: string) {
  return http.put("/admin/configs/", { key, value });
}

async function saveAll() {
  saving.value = true;
  try {
    const ops: Promise<unknown>[] = [
      saveOne(KEY_RENAME_ENABLED, configs.value[KEY_RENAME_ENABLED] ?? "true"),
    ];
    for (const row of accountRows.value) {
      ops.push(saveOne(`${PREFIX_SAVE_ROOT}${row.id}`, row.saveRoot));
    }
    for (const row of driverRows.value) {
      ops.push(saveOne(`${PREFIX_QUOTA}${row.driverType}`, row.quotaGb));
      ops.push(saveOne(`${PREFIX_CLEANUP_MODE}${row.driverType}`, row.cleanupMode));
      ops.push(saveOne(`${PREFIX_CLEANUP_KEEP}${row.driverType}`, row.cleanupKeepDays));
    }
    await Promise.all(ops);
    toast.success("转存/配额配置已保存");
  } catch (e) {
    toast.error(getApiErrorMessage(e, "保存失败"));
  } finally {
    saving.value = false;
  }
}

onMounted(() => {
  void loadAll();
});
</script>

<template>
  <div class="settings">
    <SettingsCard title="转存设置（§6.9）" :loading="loading">
      <SettingsRow label="转存后重命名">
        <label class="toggle">
          <input v-model="renameEnabled" type="checkbox" />
          <span>{{ renameEnabled ? "已启用" : "已关闭" }}</span>
        </label>
        <p class="settings-help">§6.9.2：启用后驱动调用 driver.Rename()，按 {title} ({year}).{ext} 或 S{season:02d}E{episode:02d}.{ext} 命名。</p>
      </SettingsRow>

      <SettingsRow v-if="accountRows.length" label="按账号配置转存根目录">
        <table class="kv-table">
          <thead>
            <tr><th>账号</th><th>驱动</th><th>转存目标 Folder ID</th></tr>
          </thead>
          <tbody>
            <tr v-for="row in accountRows" :key="row.id">
              <td>{{ row.name }}</td>
              <td>{{ row.driverLabel }}</td>
              <td>
                <AppInput
                  v-model="row.saveRoot"
                  :placeholder="`例如 folder_id（首次添加账号时自动创建 X-MEDIA/）`"
                />
              </td>
            </tr>
          </tbody>
        </table>
      </SettingsRow>

      <SettingsRow v-if="driverRows.length" label="按驱动配置配额与清理">
        <table class="kv-table">
          <thead>
            <tr><th>驱动</th><th>配额预警 (GB)</th><th>清理模式</th><th>保留天数</th></tr>
          </thead>
          <tbody>
            <tr v-for="row in driverRows" :key="row.driverType">
              <td>{{ row.driverLabel }}</td>
              <td>
                <AppInput v-model="row.quotaGb" type="number" placeholder="5" />
              </td>
              <td>
                <AppSelect v-model="row.cleanupMode" :options="cleanupModeOptions" />
              </td>
              <td>
                <AppInput v-model="row.cleanupKeepDays" type="number" placeholder="7" />
              </td>
            </tr>
          </tbody>
        </table>
        <p class="settings-help">§6.9.3：剩余空间低于配额阈值时触发 WS notification；清理时会跳过最近 N 天播放过的内容。</p>
      </SettingsRow>

      <SettingsRow v-if="!accountRows.length && !driverRows.length" label="尚未配置网盘账号">
        <p class="settings-help">先到「存储管理」添加账号，再回到此页面配置转存根目录与配额。</p>
      </SettingsRow>

      <SettingsRow label="操作">
        <div class="row-actions">
          <AppButton type="button" variant="ghost" :disabled="loading || saving" @click="loadAll">刷新</AppButton>
          <AppButton type="button" variant="primary" :disabled="saving" @click="saveAll">
            {{ saving ? "保存中..." : "保存" }}
          </AppButton>
        </div>
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
  margin: 4px 0 0;
}

.kv-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

.kv-table th,
.kv-table td {
  padding: 8px 10px;
  border-bottom: 1px solid var(--border-soft);
  text-align: left;
}

.kv-table th {
  font-weight: 600;
  color: var(--color-text-secondary, #6b7280);
}

.row-actions {
  display: flex;
  gap: 8px;
}
</style>