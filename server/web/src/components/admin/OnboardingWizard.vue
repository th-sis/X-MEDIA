<script setup lang="ts">
// V7 §1.4 Step 2 初始化向导：首次登录（默认口令 admin/admin）时按顺序引导
// 1) 设置管理员密码  2) 配置 TMDB API Key（保存即测试）  3) 添加网盘账号。
// 每步完成自动进入下一步；全部完成关闭并刷新会话状态。
import { computed, onMounted, ref } from "vue";
import { updateCredentials } from "@/api/auth";
import { toast } from "@/composables/useToast";
import { useAuthStore } from "@/stores/auth";
import AppButton from "@/components/base/AppButton.vue";
import AppInput from "@/components/base/AppInput.vue";
import { getApiErrorMessage } from "@/api/client";

const emit = defineEmits<{ done: [] }>();
const auth = useAuthStore();

const step = ref(1);
const steps = [
  { key: 1, label: "设置管理员密码" },
  { key: 2, label: "配置 TMDB API Key" },
  { key: 3, label: "添加网盘账号" },
];

// Step 1: 改密
const newPassword = ref("");
const confirmPassword = ref("");
const changing = ref(false);

// Step 2: TMDB
const tmdbKey = ref("");
const testingTmdb = ref(false);
const tmdbStatus = ref<"idle" | "ok" | "fail">("idle");
const tmdbMessage = ref("");

const canNext1 = computed(() => {
  if (newPassword.value.length < 8) return false;
  return newPassword.value === confirmPassword.value;
});

async function submitPassword() {
  if (!canNext1.value) return;
  changing.value = true;
  try {
    await updateCredentials({
      admin_username: auth.username || "admin",
      admin_password: newPassword.value,
    });
    auth.mustChangePassword = false;
    toast.success("管理员密码已更新");
    step.value = 2;
  } catch (e) {
    toast.error(getApiErrorMessage(e, "密码修改失败"));
  } finally {
    changing.value = false;
  }
}

async function testAndSaveTmdb() {
  const key = tmdbKey.value.trim();
  if (!key) {
    tmdbStatus.value = "fail";
    tmdbMessage.value = "请输入 TMDB API Key";
    return;
  }
  testingTmdb.value = true;
  tmdbStatus.value = "idle";
  try {
    const res = await fetch("/api/admin/tmdb/config", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      credentials: "include",
      body: JSON.stringify({ api_key: key }),
    });
    const body = await res.json();
    if (!res.ok || !body.success) {
      throw new Error(body.message || `HTTP ${res.status}`);
    }
    tmdbStatus.value = "ok";
    tmdbMessage.value = `TMDB 连通正常，测试搜索返回 ${body.data?.test_count ?? 0} 条结果`;
    toast.success("TMDB API Key 已保存并验证通过");
  } catch (e) {
    tmdbStatus.value = "fail";
    tmdbMessage.value = getApiErrorMessage(e, "TMDB 测试失败");
  } finally {
    testingTmdb.value = false;
  }
}

function goToAccounts() {
  step.value = 3;
}

function finish() {
  emit("done");
}

onMounted(() => {
  // 预读已保存的 TMDB key（如果有）
  fetch("/api/admin/configs/", { credentials: "include" })
    .then((r) => r.json())
    .then((d) => {
      const items = d?.data?.items ?? {};
      if (items.tmdb_api_key) tmdbKey.value = items.tmdb_api_key;
    })
    .catch(() => undefined);
});
</script>

<template>
  <div class="wizard-overlay">
    <div class="wizard" role="dialog" aria-modal="true" aria-label="初始化向导">
      <div class="wizard__head">
        <h3>欢迎使用 X-MEDIA · 初始化向导</h3>
        <p>首次使用需要完成 3 个步骤（§1.4 Day 1 旅程）</p>
      </div>

      <div class="wizard__steps">
        <div
          v-for="s in steps"
          :key="s.key"
          class="wizard__step"
          :class="{ 'is-active': step === s.key, 'is-done': step > s.key }"
        >
          <span class="wizard__step-index">{{ step > s.key ? "✓" : s.key }}</span>
          <span class="wizard__step-label">{{ s.label }}</span>
        </div>
      </div>

      <!-- Step 1: 改密 -->
      <div v-if="step === 1" class="wizard__body">
        <p class="wizard__hint">
          当前仍在使用默认管理员口令（admin/admin）。请设置至少 8 位的新密码。
        </p>
        <label class="wizard__field">
          <span>新密码（至少 8 位）</span>
          <AppInput v-model="newPassword" type="password" placeholder="输入新密码" />
        </label>
        <label class="wizard__field">
          <span>确认新密码</span>
          <AppInput v-model="confirmPassword" type="password" placeholder="再次输入新密码" />
        </label>
        <p v-if="newPassword.length >= 8 && newPassword !== confirmPassword" class="wizard__error">
          两次输入的密码不一致
        </p>
        <div class="wizard__actions">
          <AppButton type="button" variant="primary" :disabled="!canNext1 || changing" @click="submitPassword">
            {{ changing ? "提交中..." : "下一步 →" }}
          </AppButton>
        </div>
      </div>

      <!-- Step 2: TMDB -->
      <div v-else-if="step === 2" class="wizard__body">
        <p class="wizard__hint">
          配置 TMDB API Key（v3 auth key）。可到
          <a href="https://www.themoviedb.org/settings/api" target="_blank" rel="noopener">tmdb.com 申请</a>。
          保存时将自动测试连通性。
        </p>
        <label class="wizard__field">
          <span>TMDB API Key</span>
          <AppInput v-model="tmdbKey" placeholder="粘贴你的 TMDB v3 API Key" />
        </label>
        <p v-if="tmdbStatus === 'ok'" class="wizard__ok">✓ {{ tmdbMessage }}</p>
        <p v-else-if="tmdbStatus === 'fail'" class="wizard__error">✕ {{ tmdbMessage }}</p>
        <div class="wizard__actions">
          <AppButton type="button" variant="ghost" :disabled="testingTmdb" @click="testAndSaveTmdb">
            {{ testingTmdb ? "测试中..." : "保存并测试" }}
          </AppButton>
          <AppButton
            type="button"
            variant="primary"
            :disabled="tmdbStatus !== 'ok'"
            @click="goToAccounts"
          >
            下一步 →
          </AppButton>
        </div>
      </div>

      <!-- Step 3: 网盘账号 -->
      <div v-else class="wizard__body">
        <p class="wizard__hint">
          添加至少一个网盘账号后即可开始播放。您也可以稍后到「存储管理」添加。
        </p>
        <div class="wizard__actions">
          <AppButton type="button" variant="primary" @click="finish">去添加网盘账号 →</AppButton>
          <AppButton type="button" variant="ghost" @click="finish">稍后再说</AppButton>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.wizard-overlay {
  position: fixed;
  inset: 0;
  z-index: 1000;
  background: rgba(0, 0, 0, 0.45);
  display: grid;
  place-items: center;
  padding: 24px;
}
.wizard {
  width: 480px;
  max-width: 100%;
  background: var(--surface);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-pop);
  padding: 24px;
}
.wizard__head h3 {
  margin: 0 0 4px;
  font-size: 17px;
}
.wizard__head p {
  margin: 0 0 16px;
  font-size: 13px;
  color: var(--text-muted);
}
.wizard__steps {
  display: flex;
  gap: 8px;
  margin-bottom: 20px;
}
.wizard__step {
  flex: 1;
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 10px;
  border-radius: var(--radius-sm);
  background: var(--surface-soft);
  color: var(--text-muted);
  font-size: 12px;
}
.wizard__step.is-active {
  background: color-mix(in srgb, var(--brand) 14%, var(--surface));
  color: var(--brand);
  font-weight: 600;
}
.wizard__step.is-done {
  color: var(--success);
}
.wizard__step-index {
  width: 20px;
  height: 20px;
  display: grid;
  place-items: center;
  border-radius: 50%;
  background: currentColor;
  color: var(--surface);
  font-size: 11px;
  font-weight: 700;
  flex-shrink: 0;
}
.wizard__body {
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.wizard__hint {
  margin: 0;
  font-size: 13px;
  line-height: 1.6;
  color: var(--text-muted);
}
.wizard__field {
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 13px;
}
.wizard__actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  margin-top: 8px;
}
.wizard__error {
  margin: 0;
  font-size: 13px;
  color: var(--danger);
}
.wizard__ok {
  margin: 0;
  font-size: 13px;
  color: var(--success);
}
</style>
