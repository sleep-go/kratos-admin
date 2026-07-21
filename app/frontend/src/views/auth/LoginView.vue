<script setup lang="ts">
import { onMounted, reactive, shallowRef } from 'vue'
import { storeToRefs } from 'pinia'
import { useRoute, useRouter } from 'vue-router'

import { useAuthStore } from '@/stores/auth'
import * as authApi from '@/api/auth'
import { Refresh } from '@/components/icons/actions'

const authStore = useAuthStore()
const router = useRouter()
const route = useRoute()
const { loading } = storeToRefs(authStore)
const form = reactive({ identifier: '', password: '', captchaId: '', captchaCode: '' })
const captchaImage = shallowRef('')

async function refreshCaptcha() {
  const response = await authApi.getCaptcha()
  form.captchaId = response.captchaId ?? ''
  form.captchaCode = ''
  captchaImage.value = response.imageDataUri ?? ''
}

async function submit() {
  try {
    const isPlatformLogin = route.path === '/platform/login'
    const response = isPlatformLogin
      ? await authStore.platformLogin({ ...form })
      : await authStore.login({ ...form })
    if (response.mfaRequired) {
      await router.push({ name: 'mfa', query: { challenge: response.mfaChallengeId } })
      return
    }
    const defaultRedirect = isPlatformLogin ? '/platform/tenants' : '/console'
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
  <main class="login-page">
    <section class="login-atmosphere" aria-label="Kratos Admin 产品介绍">
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
        <div class="login-brand"><strong>KRATOS</strong><span>Administration System</span></div>
        <p class="login-kicker">SECURE CONSOLE</p>
        <h1>欢迎回来</h1>
        <p class="login-description">使用你的管理账号进入控制台</p>

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
          <label class="field">
            <span>密码</span>
            <input
              v-model="form.password"
              name="password"
              type="password"
              autocomplete="current-password"
              placeholder="请输入密码"
              required
            />
          </label>
          <div class="form-meta">
            <label><input type="checkbox" /> 保持登录状态</label>
            <RouterLink to="/forgot-password">忘记密码？</RouterLink>
          </div>
          <button class="submit-button" type="submit" :disabled="loading">
            {{ loading ? '正在验证…' : '安全登录' }}
          </button>
        </form>

        <p class="security-note">
          <span aria-hidden="true">◆</span> 登录行为受安全策略和审计日志保护
        </p>
      </div>
    </section>
  </main>
</template>

<style scoped lang="scss">
.login-page {
  min-height: 100vh;
  display: grid;
  grid-template-columns: minmax(420px, 1.15fr) minmax(420px, 0.85fr);
  background: #ededed;
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

.login-panel {
  display: grid;
  place-items: center;
  padding: 40px;
}

.login-card {
  width: min(100%, 430px);
  padding: clamp(30px, 4vw, 54px);
  border-top: 3px solid var(--ka-accent);
  background: #fff;
  box-shadow: 0 18px 48px rgb(0 0 0 / 10%);
  animation: card-in 420ms ease-out both;
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

.login-description {
  margin: 0 0 34px;
  color: #888;
  font-size: 13px;
}

.login-form {
  display: grid;
  gap: 20px;
}

.field {
  display: grid;
  gap: 8px;

  span {
    color: #333;
    font-size: 12px;
    font-weight: 700;
  }

  input {
    width: 100%;
    height: 46px;
    box-sizing: border-box;
    border: 1px solid #d8d8d8;
    border-radius: 2px;
    padding: 0 14px;
    outline: none;
    transition:
      border 150ms ease,
      box-shadow 150ms ease;

    &:focus {
      border-color: var(--ka-accent);
      box-shadow: 0 0 0 3px rgb(237 21 21 / 8%);
    }
  }
}

.form-meta {
  display: flex;
  justify-content: space-between;
  color: #777;
  font-size: 11px;

  a {
    color: #333;
    text-decoration: none;
  }
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
  background: linear-gradient(100deg, #f12626, #c80808);
  box-shadow: 0 9px 20px rgb(220 12 12 / 20%);
  font-weight: 750;
  letter-spacing: 0.08em;
  cursor: pointer;

  &:disabled {
    opacity: 0.6;
    cursor: wait;
  }
}

.security-note {
  margin: 28px 0 0;
  color: #9a9a9a;
  font-size: 10px;
  text-align: center;

  span {
    color: var(--ka-accent);
  }
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
  .login-page {
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
}
</style>
