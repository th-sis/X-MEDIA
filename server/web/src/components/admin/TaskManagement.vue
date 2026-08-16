<script setup lang="ts">
import {
  computed,
  defineAsyncComponent,
  reactive,
  ref,
  watch,
  watchEffect,
} from "vue";
import AppButton from "@/components/base/AppButton.vue";
import AdminTaskTabHeader from "@/components/admin/AdminTaskTabHeader.vue";
import SectionTabBar from "@/components/admin/SectionTabBar.vue";
import AdminSettingsDrawer from "@/components/admin/AdminSettingsDrawer.vue";
import CacheRuntimeStats from "@/components/admin/CacheRuntimeStats.vue";
// 重面板仅在对应 Tab 或抽屉首次打开时加载。
import type CacheRetentionPanelComponent from "@/components/admin/CacheRetentionPanel.vue";
const CacheRetentionPanel = defineAsyncComponent(() => import("@/components/admin/CacheRetentionPanel.vue"));
const CacheSettingsPanel = defineAsyncComponent(() => import("@/components/admin/CacheSettingsPanel.vue"));
const AutomationPanel = defineAsyncComponent(() => import("@/components/admin/AutomationPanel.vue"));
import { useSectionTabRoute } from "@/composables/useSectionTabRoute";
import { useSettingsPageDirty } from "@/composables/useSettingsPageDirty";
import { readPanelSaving, type SettingsPanelExpose } from "@/composables/useSettingsForm";
import { useAdminPageLoading } from "@/composables/useAdminLoadingBar";
import { toast } from "@/composables/useToast";
import "@/styles/admin-shared.css";
import "@/styles/admin-table.css";

// §13.1 裁剪：STRM / 目录整理 tab 已随后端模块移除。
const CACHE_TAB = "cache";
const AUTOMATION_TAB = "automation";
const tabs = [
  { key: CACHE_TAB, label: "缓存任务" },
  { key: AUTOMATION_TAB, label: "自动联动" },
];

type DrawerKind = "cache";

const settingsDrawerOpen = ref(false);
const drawerKind = ref<DrawerKind>("cache");
// 抽屉内设置面板按 kind 首次打开才挂载，之后保持挂载（保留已加载数据与脏状态语义）。
const drawerKindsVisited = reactive<Record<DrawerKind, boolean>>({
  cache: false,
});

const retentionPanelRef = ref<InstanceType<typeof CacheRetentionPanelComponent> | null>(null);
const cacheSettingsRef = ref<SettingsPanelExpose | null>(null);
const automationPanelRef = ref<{ openCreate: () => void } | null>(null);

const cachePanelDirty = ref(false);

watchEffect(() => {
  cachePanelDirty.value = (cacheSettingsRef.value as SettingsPanelExpose | null)?.isDirty?.() ?? false;
});

const drawerDirty = computed(() => {
  if (drawerKind.value === "cache") return cachePanelDirty.value;
  return false;
});

const settingsPageDirty = computed(() => settingsDrawerOpen.value && drawerDirty.value);

function revertDrawerSettings() {
  if (drawerKind.value === "cache") cacheSettingsRef.value?.revert?.();
}

const { confirmDiscardChanges } = useSettingsPageDirty(settingsPageDirty, revertDrawerSettings);

const { activeTab, setActiveTab } = useSectionTabRoute(
  CACHE_TAB,
  [CACHE_TAB, AUTOMATION_TAB],
  {
    beforeTabChange: async () => {
      if (!settingsDrawerOpen.value) return true;
      const ok = await confirmDiscardChanges(() => drawerDirty.value);
      if (!ok) return false;
      settingsDrawerOpen.value = false;
      return true;
    },
  },
);
useAdminPageLoading(
  "tasks",
  computed(() => activeTab.value === CACHE_TAB && !tabsVisited[CACHE_TAB]),
);

// 面板首次激活后保持挂载，避免初次进入时并发加载全部接口。
const tabsVisited = reactive<Record<string, boolean>>({});
watch(
  activeTab,
  (tab) => {
    tabsVisited[tab] = true;
  },
  { immediate: true },
);

const drawerSaving = computed(() => readPanelSaving(cacheSettingsRef.value?.saving));

function openSettingsDrawer(kind: DrawerKind) {
  drawerKind.value = kind;
  drawerKindsVisited[kind] = true;
  settingsDrawerOpen.value = true;
}

function closeSettingsDrawer() {
  settingsDrawerOpen.value = false;
}

async function handleDrawerSave() {
  if (drawerKind.value === "cache") {
    await cacheSettingsRef.value?.save?.();
    toast.success("缓存设置已保存");
  }
  settingsDrawerOpen.value = false;
}
</script>

<template>
  <div class="settings">
    <SectionTabBar :model-value="activeTab" :tabs="tabs" @update:model-value="setActiveTab">
      <template #actions>
        <AppButton
          v-if="activeTab === CACHE_TAB"
          type="button"
          variant="primary"
          @click="retentionPanelRef?.openCreate()"
        >
          添加任务
        </AppButton>
        <AppButton
          v-else-if="activeTab === AUTOMATION_TAB"
          type="button"
          variant="primary"
          @click="automationPanelRef?.openCreate()"
        >
          新增联动
        </AppButton>
      </template>
    </SectionTabBar>

    <div v-if="tabsVisited[CACHE_TAB]" v-show="activeTab === CACHE_TAB">
      <AdminTaskTabHeader
        settings-title="缓存设置"
        settings-hint="通用缓存 · WebDAV"
        @open-settings="openSettingsDrawer('cache')"
      >
        <CacheRuntimeStats />
      </AdminTaskTabHeader>
      <CacheRetentionPanel ref="retentionPanelRef" hide-stats />
    </div>

    <div v-if="tabsVisited[AUTOMATION_TAB]" v-show="activeTab === AUTOMATION_TAB">
      <AutomationPanel ref="automationPanelRef" />
    </div>

    <AdminSettingsDrawer
      :open="settingsDrawerOpen"
      :title="drawerKind === 'cache' ? '缓存设置' : '设置'"
      :saving="drawerSaving"
      @close="closeSettingsDrawer"
      @save="handleDrawerSave"
    >
      <CacheSettingsPanel
        v-if="drawerKindsVisited.cache"
        v-show="drawerKind === 'cache'"
        ref="cacheSettingsRef"
      />
    </AdminSettingsDrawer>
  </div>
</template>
