<script setup lang="ts">
import "@/styles/settings-panel.css";

withDefaults(
  defineProps<{
    label?: string;
    changed?: boolean;
    showChangedBadge?: boolean;
  }>(),
  { showChangedBadge: false },
);
</script>

<template>
  <div class="settings-row" :class="{ 'settings-row--changed': changed }">
    <div class="settings-row__info">
      <slot name="info" />
      <span v-if="label" class="settings-row__label">{{ label }}</span>
      <span v-if="changed && showChangedBadge" class="settings-row__badge">已修改</span>
    </div>
    <div v-if="$slots.control || $slots.default" class="settings-row__control">
      <slot name="control" />
      <slot />
    </div>
  </div>
</template>
