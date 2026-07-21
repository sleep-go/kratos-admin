<script setup lang="ts">
import { computed, onMounted, reactive, ref, shallowRef } from 'vue'
import { storeToRefs } from 'pinia'
import { useRoute, useRouter } from 'vue-router'

import { useAuthStore } from '@/stores/auth'
import * as authApi from '@/api/auth'
import { Hide, Refresh, View } from '@/components/icons/actions'

const authStore = useAuthStore()
const router = useRouter()
const route = useRoute()
const { loading } = storeToRefs(authStore)
const form = reactive({ identifier: '', password: '', captchaId: '', captchaCode: '' })
const captchaImage = shallowRef('')
const passwordVisible = ref(false)
const isPlatformLogin = computed(() => route.path === '/platform/login')

async function refreshCaptcha() {
  const response = await authApi.getCaptcha()
  form.captchaId = response.captchaId ?? ''
  form.captchaCode = ''
  captchaImage.value = response.imageDataUri ?? ''
}

async function submit() {
  try {
    const response = isPlatformLogin.value
      ? await authStore.platformLogin({ ...form })
      : await authStore.login({ ...form })
    if (response.mfaRequired) {
      await router.push({
        name: 'mfa',
        query: {
          challenge: response.mfaChallengeId,
          ...(isPlatformLogin.value ? { from: 'platform' } : {})
        }
      })
      return
    }
    const defaultRedirect = isPlatformLogin.value ? '/platform/tenants' : '/console'
    await router.replace(
      typeof route.query.redirect === 'string' ? route.query.redirect : defaultRedirect
    )
  } catch {
    await refreshCaptcha()
  }
}

onMounted(refreshCaptcha)
</script>

<template>
  <main
    class="login-page"
    :class="isPlatformLogin ? 'login-page--platform' : 'login-page--tenant'"
  >
    <section v-if="!isPlatformLogin" class="login-atmosphere" aria-label="Kratos Admin 产品介绍">
      <div class="atmosphere-grid"></div>
      <div class="product-mark">KA</div>
      <div class="product-copy">
        <p class="eyebrow">KRATOS ADMINISTRATION SYSTEM</p>
        <h2>秩序，让复杂系统<br />保持清晰。</h2>
        <p>多租户组织、权限与审计能力，由同一套可信边界驱动。</p>
      </div>
    </section>

    <section class="login-panel">
      <div class="login-card">
        <template v-if="isPlatformLogin">
          <div class="platform-brand">
            <span class="platform-mark">PG</span>
            <div>
              <strong>KRATOS PLATFORM</strong>
              <span>Governance Console</span>
            </div>
          </div>
          <p class="login-kicker login-kicker--platform">PLATFORM ACCESS</p>
          <h1>平台治理中心</h1>
          <p class="login-description">使用平台管理员账号进入租户与资源治理界面</p>
        </template>
        <template v-else>
          <div class="login-brand"><strong>KRATOS</strong><span>Administration System</span></div>
          <p class="login-kicker">SECURE CONSOLE</p>
          <h1>欢迎回来</h1>
          <p class="login-description">使用你的管理账号进入控制台</p>
        </template>

        <form class="login-form" @submit.prevent="submit">
          <label class="field">
            <span>账号</span>
            <input
              v-model.trim="form.identifier"
              name="identifier"
              autocomplete="username"
              placeholder="用户名 / 邮箱 / 手机号"
              required
            />
          </label>
          <div class="captcha-row">
            <label class="field">
              <span>图形验证码</span>
              <input
                v-model.trim="form.captchaCode"
                name="captcha"
                inputmode="numeric"
                autocomplete="off"
                maxlength="5"
                placeholder="输入图中数字"
                required
              />
            </label>
            <button
              type="button"
              class="captcha-image"
              aria-label="刷新图形验证码"
              @click="refreshCaptcha"
            >
              <img v-if="captchaImage" :src="captchaImage" alt="图形验证码" />
              <span v-else class="captcha-loading">
                <el-icon :size="16"><Refresh /></el-icon>
                加载中
              </span>
            </button>
          </div>
          <label class="field password-field">
            <span>密码</span>
            <div class="password-input-wrap">
              <input
                v-model="form.password"
                name="password"
                :type="passwordVisible ? 'text' : 'password'"
                autocomplete="current-password"
                placeholder="请输入密码"
                required
              />
              <button
                type="button"
                class="password-toggle"
                :aria-label="passwordVisible ? '隐藏密码' : '显示密码'"
                :aria-pressed="passwordVisible"
                @click="passwordVisible = !passwordVisible"
              >
                <el-icon :size="16">
                  <Hide v-if="passwordVisible" />
                  <View v-else />
                </el-icon>
              </button>
            </div>
          </label>
          <div class="form-meta">
            <label><input type="checkbox" /> 保持登录状态</label>
            <RouterLink v-if="!isPlatformLogin" to="/forgot-password">忘记密码？</RouterLink>
          </div>
          <button class="submit-button" type="submit" :disabled="loading">
            {{ loading ? '正在验证…' : isPlatformLogin ? '进入平台' : '安全登录' }}
          </button>
        </form>

        <p class="security-note">
          <span aria-hidden="true">◆</span>
          {{ isPlatformLogin ? '平台操作将记录审计日志并受 MFA 策略保护' : '登录行为受安全策略和审计日志保护' }}
        </p>

        <p v-if="isPlatformLogin" class="login-switch">
          <RouterLink to="/login">前往租户控制台登录</RouterLink>
        </p>
      </div>
    </section>
  </main>
</template>

<style scoped lang="scss">
.login-page {
  min-height: 100vh;
  display: grid;
  background: #ededed;
}

.login-page--tenant {
  grid-template-columns: minmax(420px, 1.15fr) minmax(420px, 0.85fr);
}

.login-page--platform {
  place-items: center;
  padding: clamp(24px, 4vw, 48px);
  background:
    radial-gradient(circle at 12% 18%, rgb(212 168 83 / 14%), transparent 34%),
    radial-gradient(circle at 88% 82%, rgb(56 189 248 / 10%), transparent 30%),
    linear-gradient(160deg, #0b1220 0%, #111827 42%, #1a2332 100%);
}

.login-atmosphere {
  position: relative;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  overflow: hidden;
  padding: clamp(32px, 5vw, 72px);
  color: #fff;
  background: linear-gradient(145deg, #767676 0%, #3f3f3f 48%, #202020 100%);
}

.atmosphere-grid {
  position: absolute;
  inset: 0;
  opacity: 0.16;
  background-image:
    linear-gradient(rgb(255 255 255 / 16%) 1px, transparent 1px),
    linear-gradient(90deg, rgb(255 255 255 / 16%) 1px, transparent 1px);
  background-size: 54px 54px;
  mask-image: linear-gradient(145deg, #000, transparent 75%);
}

.product-mark {
  position: relative;
  width: 62px;
  height: 62px;
  display: grid;
  place-items: center;
  border: 2px solid rgb(255 255 255 / 75%);
  font-family: 'Arial Black', sans-serif;
  font-size: 22px;
}

.product-copy {
  position: relative;
  max-width: 650px;

  h2 {
    margin: 18px 0 24px;
    font-size: clamp(40px, 5vw, 72px);
    line-height: 1.16;
    letter-spacing: -0.055em;
  }

  > p:last-child {
    max-width: 460px;
    color: rgb(255 255 255 / 64%);
    line-height: 1.8;
  }
}

.eyebrow,
.login-kicker {
  color: #ef1b1b;
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0.18em;
}

.login-kicker--platform {
  color: #d4a853;
}

.login-panel {
  display: grid;
  place-items: center;
  padding: 40px;
}

.login-page--platform .login-panel {
  width: min(100%, 480px);
  padding: 0;
}

.login-card {
  width: min(100%, 430px);
  padding: clamp(30px, 4vw, 54px);
  background: #fff;
  animation: card-in 420ms ease-out both;
}

.login-page--tenant .login-card {
  border-top: 3px solid var(--ka-accent);
  box-shadow: 0 18px 48px rgb(0 0 0 / 10%);
}

.login-page--platform .login-card {
  border: 1px solid rgb(212 168 83 / 28%);
  border-radius: 16px;
  color: #e5e7eb;
  background: rgb(15 23 42 / 88%);
  box-shadow:
    0 24px 60px rgb(0 0 0 / 35%),
    inset 0 1px 0 rgb(255 255 255 / 6%);
  backdrop-filter: blur(12px);
}

.platform-brand {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-bottom: 42px;
}

.platform-mark {
  width: 52px;
  height: 52px;
  display: grid;
  place-items: center;
  border: 1px solid rgb(212 168 83 / 55%);
  border-radius: 12px;
  color: #f3d08a;
  background: rgb(212 168 83 / 8%);
  font-family: 'Arial Black', sans-serif;
  font-size: 18px;
}

.platform-brand strong {
  display: block;
  color: #f8fafc;
  font-size: 15px;
  letter-spacing: 0.08em;
}

.platform-brand span {
  color: rgb(148 163 184);
  font-size: 11px;
}

.login-brand {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 56px;

  strong {
    font-family: 'Arial Black', sans-serif;
    font-size: 20px;
    letter-spacing: -1px;
  }

  span {
    width: 80px;
    color: #777;
    font-size: 8px;
    line-height: 1.2;
  }
}

.login-card h1 {
  margin: 10px 0 8px;
  font-size: 34px;
  letter-spacing: -0.04em;
}

.login-page--platform .login-card h1 {
  color: #f8fafc;
}

.login-description {
  margin: 0 0 34px;
  font-size: 13px;
}

.login-page--tenant .login-description {
  color: #888;
}

.login-page--platform .login-description {
  color: rgb(148 163 184);
}

.login-form {
  display: grid;
  gap: 20px;
}

.field {
  display: grid;
  gap: 8px;

  span {
    font-size: 12px;
    font-weight: 700;
  }

  input {
    width: 100%;
    height: 46px;
    box-sizing: border-box;
    border-radius: 2px;
    padding: 0 14px;
    outline: none;
    transition:
      border 150ms ease,
      box-shadow 150ms ease;
  }
}

.login-page--tenant .field span {
  color: #333;
}

.login-page--tenant .field input {
  border: 1px solid #d8d8d8;

  &:focus {
    border-color: var(--ka-accent);
    box-shadow: 0 0 0 3px rgb(237 21 21 / 8%);
  }
}

.login-page--platform .field span {
  color: rgb(203 213 225);
}

.login-page--platform .field input {
  border: 1px solid rgb(148 163 184 / 28%);
  color: #f8fafc;
  background: rgb(15 23 42 / 72%);

  &::placeholder {
    color: rgb(100 116 139);
  }

  &:focus {
    border-color: #d4a853;
    box-shadow: 0 0 0 3px rgb(212 168 83 / 14%);
  }
}

.password-input-wrap {
  position: relative;

  input {
    padding-right: 44px;
  }
}

.password-toggle {
  position: absolute;
  top: 0;
  right: 0;
  width: 44px;
  height: 46px;
  display: grid;
  place-items: center;
  border: 0;
  background: transparent;
  color: #888;
  cursor: pointer;

  &:hover {
    color: #333;
  }
}

.login-page--platform .password-toggle {
  color: rgb(148 163 184);

  &:hover {
    color: #f8fafc;
  }
}

.form-meta {
  display: flex;
  justify-content: space-between;
  font-size: 11px;

  a {
    text-decoration: none;
  }
}

.login-page--tenant .form-meta {
  color: #777;

  a {
    color: #333;
  }
}

.login-page--platform .form-meta {
  color: rgb(148 163 184);
}

.captcha-row {
  display: grid;
  grid-template-columns: 1fr 150px;
  align-items: end;
  gap: 12px;
}

.captcha-image {
  height: 46px;
  padding: 0;
  border: 1px solid #d8d8d8;
  background: #f5f5f5;
  cursor: pointer;

  img {
    width: 100%;
    height: 100%;
    display: block;
    object-fit: cover;
  }
}

.login-page--platform .captcha-image {
  overflow: hidden;
  border-color: rgb(148 163 184 / 28%);
  border-radius: 2px;
  background: rgb(15 23 42 / 72%);
}

.captcha-loading {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 0 12px;
  color: var(--ka-muted);
  font-size: 12px;
}

.submit-button {
  height: 48px;
  border: 0;
  border-radius: 2px;
  color: #fff;
  font-weight: 750;
  letter-spacing: 0.08em;
  cursor: pointer;

  &:disabled {
    opacity: 0.6;
    cursor: wait;
  }
}

.login-page--tenant .submit-button {
  background: linear-gradient(100deg, #f12626, #c80808);
  box-shadow: 0 9px 20px rgb(220 12 12 / 20%);
}

.login-page--platform .submit-button {
  border-radius: 8px;
  background: linear-gradient(100deg, #e8c068, #c8943a);
  box-shadow: 0 10px 24px rgb(200 148 58 / 28%);
  color: #1a1205;
}

.security-note {
  margin: 28px 0 0;
  font-size: 10px;
  text-align: center;

  span {
    color: var(--ka-accent);
  }
}

.login-page--platform .security-note {
  color: rgb(100 116 139);

  span {
    color: #d4a853;
  }
}

.login-switch {
  margin: 18px 0 0;
  font-size: 12px;
  text-align: center;

  a {
    text-decoration: none;
  }
}

.login-page--tenant .login-switch a {
  color: #666;
}

.login-page--platform .login-switch a {
  color: #d4a853;
}

@keyframes card-in {
  from {
    opacity: 0;
    transform: translateY(14px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@media (max-width: 900px) {
  .login-page--tenant {
    grid-template-columns: 1fr;
  }

  .login-atmosphere {
    min-height: 220px;
  }

  .product-copy h2 {
    margin-bottom: 0;
    font-size: 38px;
  }

  .product-copy > p:last-child {
    display: none;
  }

  .login-panel {
    padding: 24px 16px 48px;
  }
}

@media (max-width: 520px) {
  .login-atmosphere {
    min-height: 160px;
    padding: 24px;
  }

  .product-mark {
    width: 44px;
    height: 44px;
    font-size: 16px;
  }

  .product-copy h2 {
    margin-top: 10px;
    font-size: 27px;
  }

  .eyebrow {
    display: none;
  }

  .login-card {
    padding: 30px 24px;
  }

  .login-brand {
    margin-bottom: 36px;
  }

  .platform-brand {
    margin-bottom: 28px;
  }
}
</style>
