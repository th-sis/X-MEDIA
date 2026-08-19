<script setup lang="ts">
// V7 整改 (commit #3 中): nav-item icons 从 fontawesome 改成内联 SVG
// 根因: fontawesome webfont .woff2 加载链有未知问题 (浏览器渲染失败, 显示空心圆占位)
// 改法: 8 个 nav icon 全用 inline SVG path (viewBox 24x24 stroke="currentColor"),
//       风格与 sidebar-toggle 一致 (Lucide-style)
// 参考: https://lucide.dev/icons/
defineProps<{
  name: string;
}>();
</script>

<template>
  <!-- 仪表盘: tachometer-alt -->
  <svg v-if="name === 'tachometer-alt'" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
    <path d="M12 14m-4 0a4 4 0 1 0 8 0a4 4 0 1 0 -8 0" />
    <path d="M12 10l3 -3" />
    <path d="M12 7v-3" />
    <path d="M5.6 5.6l-2 -2" />
    <path d="M18.4 5.6l2 -2" />
  </svg>
  <!-- 存储管理: hdd (database/server) -->
  <svg v-else-if="name === 'hdd'" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
    <ellipse cx="12" cy="5" rx="9" ry="3" />
    <path d="M3 5v14a9 3 0 0 0 18 0v-14" />
    <path d="M3 12a9 3 0 0 0 18 0" />
  </svg>
  <!-- 系统设置: cogs (settings/gear) -->
  <svg v-else-if="name === 'cogs'" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
    <path d="M10.325 4.317c.426 -1.756 2.924 -1.756 3.35 0a1.724 1.724 0 0 0 2.573 1.066c1.543 -.94 3.31 .826 2.37 2.37a1.724 1.724 0 0 0 1.065 2.572c1.756 .426 1.756 2.924 0 3.35a1.724 1.724 0 0 0 -1.066 2.573c.94 1.543 -.826 3.31 -2.37 2.37a1.724 1.724 0 0 0 -2.572 1.065c-.426 1.756 -2.924 1.756 -3.35 0a1.724 1.724 0 0 0 -2.573 -1.066c-1.543 .94 -3.31 -.826 -2.37 -2.37a1.724 1.724 0 0 0 -1.065 -2.572c-1.756 -.426 -1.756 -2.924 0 -3.35a1.724 1.724 0 0 0 1.066 -2.573c-.94 -1.543 .826 -3.31 2.37 -2.37a1.724 1.724 0 0 0 2.572 -1.065z" />
    <path d="M15 12a3 3 0 1 1 -6 0a3 3 0 0 1 6 0z" />
  </svg>
  <!-- 播放优先级: layer-group (stack layers) -->
  <svg v-else-if="name === 'layer-group'" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
    <path d="M12 2l-9 5l9 5l9 -5l-9 -5" />
    <path d="M3 12l9 5l9 -5" />
    <path d="M3 17l9 5l9 -5" />
  </svg>
  <!-- 转存设置: folder-tree -->
  <svg v-else-if="name === 'folder-tree'" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
    <path d="M20 10a1 1 0 0 0 0 -2h-2a1 1 0 0 1 0 -2h-3l-2 2h-7a1 1 0 0 0 -1 1v8a1 1 0 0 0 1 1h2" />
    <path d="M3 19a2 2 0 0 0 2 2h3l3 -3h6a1 1 0 0 0 1 -1v-4" />
    <path d="M3 14v-3" />
    <path d="M11 14h5l3 3h3a1 1 0 0 0 1 -1v-4a1 1 0 0 0 -1 -1h-7" />
  </svg>
  <!-- 任务管理: tasks (clipboard checklist) -->
  <svg v-else-if="name === 'tasks'" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
    <path d="M9 5h-2a2 2 0 0 0 -2 2v12a2 2 0 0 0 2 2h10a2 2 0 0 0 2 -2v-12a2 2 0 0 0 -2 -2h-2" />
    <path d="M9 3m0 2h6l-6 -2z" />
    <path d="M9 12l2 2l4 -4" />
  </svg>
  <!-- 媒体配置: clapperboard -->
  <svg v-else-if="name === 'clapperboard'" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
    <path d="M3 8h18v11a2 2 0 0 1 -2 2h-14a2 2 0 0 1 -2 -2v-11z" />
    <path d="M3 8l2 -4h14l2 4" />
    <path d="M9 8v-3" />
    <path d="M13 8v-3" />
    <path d="M17 8v-3" />
  </svg>
  <!-- 跨盘秒传: right-left (transfer) -->
  <svg v-else-if="name === 'right-left'" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
    <path d="M21 7l-18 0" />
    <path d="M3 7l4 -4l-4 -4" />
    <path d="M21 17l-18 0" />
    <path d="M3 17l4 4l-4 4" />
  </svg>
  <!-- 返回首页: home -->
  <svg v-else width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
    <path d="M3 9l9 -7l9 7v11a2 2 0 0 1 -2 2h-14a2 2 0 0 1 -2 -2z" />
    <path d="M9 22l0 -6h6l0 6" />
  </svg>
</template>

<style scoped>
.admin-nav-icon {
  flex: none;
  width: 24px;
  height: 24px;
  text-align: center;
  font-size: 18px;
  line-height: 1;
}
</style>