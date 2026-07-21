<script setup lang="ts">
import { computed, reactive } from 'vue'
import { storeToRefs } from 'pinia'
import { useRoute, useRouter } from 'vue-router'

import { useAuthStore } from '@/stores/auth'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const { loading } = storeToRefs(authStore)
const isPlatformFlow = computed(() => route.query.from === 'platform')
const challengeId = computed(() =>
  typeof route.query.challenge === 'string' ? route.query.challenge : ''
)
const form = reactive({ code: '' })

async function submit() {
  if (!challengeId.value) {
    await router.replace(isPlatformFlow.value ? '/platform/login' : '/login')
    return
  }
  try {
    await authStore.verifyMfa(challengeId.value, form.code)
    await router.replace(isPlatformFlow.value ? '/platform/tenants' : '/console')
  } catch {
    form.code = ''
  }
}
</script>

<template>
  <main class="security-page">
    <section class="security-card">
      <p>MULTI-FACTOR AUTHENTICATION</p>
      <h1>完成二次验证</h1>
      <span>验证码已发送至账号配置的安全渠道，5 分钟内有效。</span>
      <form @submit.prevent="submit">
        <label>
          <strong>六位验证码</strong>
          <input
            v-model.trim="form.code"
            inputmode="numeric"
            maxlength="6"
            autocomplete="one-time-code"
            required
          />
        </label>
        <button type="submit" :disabled="loading || !challengeId">
          {{ loading ? '验证中…' : '验证并登录' }}
        </button>
      </form>
      <RouterLink :to="isPlatformFlow ? '/platform/login' : '/login'">返回登录</RouterLink>
    </section>
  </main>
</template>

<style scoped lang="scss">
.security-page {
  min-height: 100vh;
  display: grid;
  place-items: center;
  padding: 24px;
  background: linear-gradient(145deg, #767676, #282828);
}
.security-card {
  width: min(100%, 430px);
  box-sizing: border-box;
  padding: 46px;
  border-top: 3px solid var(--ka-accent);
  background: #fff;
  box-shadow: 0 20px 60px rgb(0 0 0 / 24%);
}
.security-card > p {
  margin: 0 0 12px;
  color: var(--ka-accent);
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 0.16em;
}
h1 {
  margin: 0 0 10px;
  font-size: 30px;
}
.security-card > span {
  color: var(--ka-muted);
  font-size: 13px;
  line-height: 1.7;
}
form {
  display: grid;
  gap: 22px;
  margin: 30px 0 22px;
}
label {
  display: grid;
  gap: 9px;
}
strong {
  font-size: 12px;
}
input {
  height: 48px;
  padding: 0 15px;
  border: 1px solid var(--ka-border);
  font-size: 20px;
  letter-spacing: 0.3em;
  outline: none;
}
input:focus {
  border-color: var(--ka-accent);
}
button {
  height: 48px;
  border: 0;
  color: #fff;
  background: var(--ka-accent);
  font-weight: 750;
  cursor: pointer;
}
a {
  color: var(--ka-muted);
  font-size: 12px;
  text-decoration: none;
}
@media (max-width: 520px) {
  .security-card {
    padding: 32px 24px;
  }
}
</style>
