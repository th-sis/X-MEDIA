<script setup lang="ts">
// V7 §28 启动序 7 步：用户进入 dashboard 时若服务刚启动（uptime < 60s），
// 顶部展示倒计时 + 重启原因（graceful/config_change/oom/panic）。
defineProps<{
  seconds: number;
  reason?: string;
}>();
</script>

<template>
  <div v-if="seconds > 0" class="admin-startup-banner" role="status">
    <i class="fas fa-circle-info" />
    <span>
      后端任务启动中，预计 <strong>{{ seconds }}</strong> 秒后才可执行。
      <span v-if="reason" class="reason-tag">重启原因: {{ reason }}</span>
    </span>
  </div>
</template>

<style scoped>
.admin-startup-banner {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 16px;
  border-radius: var(--radius-md);
  font-size: 13px;
  line-height: 1.5;
  background: color-mix(in srgb, var(--brand) 10%, var(--surface));
  border: 1px solid color-mix(in srgb, var(--brand) 24%, var(--border-soft));
  color: color-mix(in srgb, var(--brand) 70%, var(--text));
}

.admin-startup-banner i {
  flex-shrink: 0;
}

.reason-tag {
  margin-left: 8px;
  padding: 2px 8px;
  border-radius: var(--radius-sm);
  background: color-mix(in srgb, var(--brand) 14%, var(--surface));
  font-size: 12px;
  font-weight: 500;
}
</style>