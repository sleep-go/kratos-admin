<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { storeToRefs } from 'pinia'
import { ElMessage, ElMessageBox } from 'element-plus'

import * as authApi from '@/api/auth'
import { useAuthStore } from '@/stores/auth'
import type { DeviceSession } from '@/api/auth'
import AppIcon from '@/components/icons/AppIcon.vue'
import { Check, Delete, Refresh } from '@/components/icons/actions'

const authStore = useAuthStore()
const { currentTenant, currentUser } = storeToRefs(authStore)
const sessions = ref<DeviceSession[]>([])
const loading = ref(false)
const saving = ref(false)
const isPlatformAccount = computed(
  () => currentUser.value?.realm === 'platform' && !currentUser.value?.impersonating
)
const form = reactive({ displayName: '', avatarUrl: '', email: '', phone: '' })

async function loadSessions() {
  loading.value = true
  try {
    const response = isPlatformAccount.value
      ? await authApi.platformListSessions()
      : await authApi.listSessions()
    sessions.value = response.items ?? []
  } finally {
    loading.value = false
  }
}

async function saveProfile() {
  if (isPlatformAccount.value) {
    ElMessage.info('平台管理员资料请在平台治理入口维护')
    return
  }
  saving.value = true
  try {
    const response = await authApi.updateProfile(form)
    if (response.user) authStore.applyProfile(response.user)
    ElMessage.success('个人资料已更新')
  } finally {
    saving.value = false
  }
}

async function revoke(session: DeviceSession) {
  if (!session.id || session.current) return
  await ElMessageBox.confirm('撤销后该设备需要重新登录，确认继续？', '撤销设备会话', {
    type: 'warning'
  })
  if (isPlatformAccount.value) {
    await authApi.platformRevokeSession(session.id)
  } else {
    await authApi.revokeSession(session.id)
  }
  ElMessage.success('设备会话已撤销')
  await loadSessions()
}

function formatTime(value?: string) {
  return value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '—'
}

onMounted(() => {
  Object.assign(form, {
    displayName: currentUser.value?.displayName ?? '',
    avatarUrl: currentUser.value?.avatarUrl ?? '',
    email: currentUser.value?.email ?? '',
    phone: currentUser.value?.phone ?? ''
  })
  void loadSessions()
})
</script>

<template>
  <section class="account-page">
    <header class="page-heading">
      <div>
        <p>ACCOUNT SECURITY</p>
        <h1>个人中心</h1>
        <span>维护个人资料并管理已登录设备。</span>
      </div>
      <span class="tenant-chip">{{ currentTenant?.name ?? '未选择租户' }}</span>
    </header>

    <div class="account-grid">
      <article class="panel profile-panel">
        <header>
          <div class="avatar-preview">{{ currentUser?.displayName?.slice(0, 1) || '管' }}</div>
          <div>
            <h2>{{ currentUser?.displayName }}</h2>
            <span>{{ currentUser?.username || `用户 ${currentUser?.id}` }}</span>
          </div>
        </header>
        <dl class="identity-summary">
          <div>
            <dt>邮箱</dt>
            <dd>{{ currentUser?.email || '未设置' }}</dd>
          </div>
          <div>
            <dt>手机号</dt>
            <dd>{{ currentUser?.phone || '未设置' }}</dd>
          </div>
          <div>
            <dt>MFA</dt>
            <dd>{{ currentUser?.mfaEnabled ? `已启用 · ${currentUser.mfaChannel}` : '未启用' }}</dd>
          </div>
        </dl>
        <el-form v-if="!isPlatformAccount" label-position="top" @submit.prevent="saveProfile">
          <div class="form-grid">
            <el-form-item label="显示名称" required>
              <el-input v-model="form.displayName" />
            </el-form-item>
            <el-form-item label="头像地址"><el-input v-model="form.avatarUrl" /></el-form-item>
            <el-form-item label="邮箱"><el-input v-model="form.email" /></el-form-item>
            <el-form-item label="手机号"><el-input v-model="form.phone" /></el-form-item>
          </div>
          <el-button type="danger" native-type="submit" :icon="Check" :loading="saving">保存资料</el-button>
          <router-link to="/forgot-password" class="password-link">通过验证码重置密码</router-link>
        </el-form>
        <p v-else class="platform-note">平台管理员账号资料由平台治理模块统一维护，此处仅展示会话信息。</p>
      </article>

      <article class="panel sessions-panel">
        <div class="panel-title">
          <div>
            <span>DEVICES</span>
            <h2>登录设备</h2>
          </div>
          <el-button :loading="loading" :icon="Refresh" @click="loadSessions">刷新</el-button>
        </div>
        <div v-if="sessions.length" class="session-list">
          <section v-for="session in sessions" :key="session.id" class="session-item">
            <div>
              <strong>{{ session.deviceName || '未知设备' }}</strong>
              <span>{{ session.ip || '未知 IP' }} · {{ formatTime(session.createdAt) }}</span>
              <small>有效至 {{ formatTime(session.expiresAt) }}</small>
            </div>
            <span v-if="session.current" class="current-tag">当前设备</span>
            <el-button v-else link type="danger" :icon="Delete" @click="revoke(session)">撤销</el-button>
          </section>
        </div>
        <div v-else-if="!loading" class="empty-state">
          <AppIcon name="empty" :size="26" />
          <span>暂无有效设备会话</span>
        </div>
      </article>
    </div>
  </section>
</template>

<style scoped lang="scss">
.account-page {
  display: grid;
  gap: 20px;
}
.page-heading {
  display: flex;
  align-items: end;
  justify-content: space-between;
  gap: 18px;
}
.page-heading p,
.panel-title span {
  margin: 0 0 5px;
  color: var(--ka-accent);
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 0.18em;
}
.page-heading h1 {
  margin: 0;
  font-size: clamp(28px, 4vw, 40px);
}
.page-heading > div > span {
  display: block;
  margin-top: 8px;
  color: var(--ka-muted);
}
.tenant-chip {
  padding: 8px 12px;
  color: #555;
  background: #fff;
  box-shadow: var(--ka-shadow);
  font-size: 12px;
}
.account-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.15fr) minmax(330px, 0.85fr);
  gap: 16px;
}
.panel {
  padding: 24px;
  background: #fff;
  box-shadow: var(--ka-shadow);
}
.profile-panel > header {
  display: flex;
  align-items: center;
  gap: 14px;
  padding-bottom: 20px;
  border-bottom: 1px solid var(--ka-border);
}
.profile-panel h2,
.panel-title h2 {
  margin: 0;
  font-size: 19px;
}
.profile-panel header span {
  color: var(--ka-muted);
  font-size: 12px;
}
.avatar-preview {
  width: 52px;
  height: 52px;
  display: grid;
  place-items: center;
  border-radius: 50%;
  color: #fff;
  background: linear-gradient(135deg, #777, #292929);
  font-size: 20px;
}
.identity-summary {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 10px;
  margin: 20px 0;
}
.identity-summary div {
  min-width: 0;
  padding: 12px;
  background: #f5f5f5;
}
.identity-summary dt {
  color: var(--ka-muted);
  font-size: 11px;
}
.identity-summary dd {
  margin: 5px 0 0;
  overflow: hidden;
  font-size: 13px;
  text-overflow: ellipsis;
}
.form-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0 14px;
}
.password-link {
  margin-left: 14px;
  color: var(--ka-muted);
  font-size: 12px;
}
.platform-note {
  margin: 0;
  color: var(--ka-muted);
  font-size: 13px;
  line-height: 1.7;
}
.panel-title {
  display: flex;
  align-items: start;
  justify-content: space-between;
}
.session-list {
  margin-top: 12px;
}
.session-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 16px 0;
  border-bottom: 1px solid var(--ka-border);
}
.session-item > div {
  min-width: 0;
  display: grid;
  gap: 4px;
}
.session-item span,
.session-item small {
  color: var(--ka-muted);
  font-size: 11px;
  overflow-wrap: anywhere;
}
.current-tag {
  flex: none;
  padding: 5px 8px;
  color: #08786f !important;
  background: #e6f5f3;
}
.empty-state {
  min-height: 180px;
  display: grid;
  place-items: center;
  gap: 8px;
  color: var(--ka-muted);
}
@media (max-width: 900px) {
  .account-grid {
    grid-template-columns: 1fr;
  }
}
@media (max-width: 560px) {
  .page-heading {
    align-items: start;
    flex-direction: column;
  }
  .identity-summary,
  .form-grid {
    grid-template-columns: 1fr;
  }
  .panel {
    padding: 18px;
  }
}
</style>
