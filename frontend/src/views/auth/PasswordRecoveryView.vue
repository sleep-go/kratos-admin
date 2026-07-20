<script setup lang="ts">
import { reactive, shallowRef } from 'vue'
import { ElMessage } from 'element-plus'
import { useRouter } from 'vue-router'

import * as authApi from '@/api/auth'

const router = useRouter()
const stage = shallowRef<'request' | 'reset'>('request')
const loading = shallowRef(false)
const form = reactive({
  identifier: '',
  channel: 'email',
  challengeId: '',
  code: '',
  newPassword: ''
})

async function requestCode() {
  loading.value = true
  try {
    const response = await authApi.forgotPassword({
      identifier: form.identifier,
      channel: form.channel
    })
    form.challengeId = response.challengeId ?? ''
    stage.value = 'reset'
    ElMessage.success('若账号和渠道有效，验证码已发送')
  } finally {
    loading.value = false
  }
}

async function resetPassword() {
  loading.value = true
  try {
    await authApi.resetPassword({
      challengeId: form.challengeId,
      code: form.code,
      newPassword: form.newPassword
    })
    ElMessage.success('密码已重置，请重新登录')
    await router.replace('/login')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <main class="recovery-page">
    <section class="recovery-card">
      <p>ACCOUNT RECOVERY</p>
      <h1>{{ stage === 'request' ? '找回密码' : '设置新密码' }}</h1>
      <span>{{
        stage === 'request'
          ? '输入账号并选择已验证的安全渠道。'
          : '验证码 5 分钟内有效，最多可尝试 5 次。'
      }}</span>
      <form v-if="stage === 'request'" @submit.prevent="requestCode">
        <label><strong>账号</strong><input v-model.trim="form.identifier" placeholder="用户名 / 邮箱 / 手机号" required /></label>
        <label><strong>验证渠道</strong><select v-model="form.channel">
          <option value="email">邮箱</option>
          <option value="sms">短信</option>
        </select></label>
        <button type="submit" :disabled="loading">{{ loading ? '发送中…' : '发送验证码' }}</button>
      </form>
      <form v-else @submit.prevent="resetPassword">
        <label><strong>验证码</strong><input v-model.trim="form.code" inputmode="numeric" maxlength="6" required /></label>
        <label><strong>新密码</strong><input
          v-model="form.newPassword"
          type="password"
          autocomplete="new-password"
          minlength="12"
          required
        /></label>
        <button type="submit" :disabled="loading">{{ loading ? '提交中…' : '重置密码' }}</button>
      </form>
      <RouterLink to="/login">返回登录</RouterLink>
    </section>
  </main>
</template>

<style scoped lang="scss">
.recovery-page {
  min-height: 100vh;
  display: grid;
  place-items: center;
  padding: 24px;
  background: linear-gradient(145deg, #767676, #282828);
}
.recovery-card {
  width: min(100%, 450px);
  box-sizing: border-box;
  padding: 46px;
  border-top: 3px solid var(--ka-accent);
  background: #fff;
  box-shadow: 0 20px 60px rgb(0 0 0 / 24%);
}
.recovery-card > p {
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
.recovery-card > span {
  color: var(--ka-muted);
  font-size: 13px;
}
form {
  display: grid;
  gap: 18px;
  margin: 30px 0 22px;
}
label {
  display: grid;
  gap: 8px;
}
strong {
  font-size: 12px;
}
input,
select {
  height: 46px;
  box-sizing: border-box;
  padding: 0 14px;
  border: 1px solid var(--ka-border);
  background: #fff;
  outline: none;
}
input:focus,
select:focus {
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
  .recovery-card {
    padding: 32px 24px;
  }
}
</style>
