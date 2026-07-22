<template>
  <div class="login-container">
    <el-card class="login-card">
      <template #header>
        <h2>管理员登录</h2>
      </template>
      <el-form :model="form" @submit.prevent="handleLogin">
        <el-form-item>
          <el-input v-model="form.username" placeholder="用户名" :prefix-icon="User" />
        </el-form-item>
        <el-form-item>
          <el-input
            v-model="form.password"
            type="password"
            placeholder="密码"
            :prefix-icon="Lock"
            show-password
            @keyup.enter="handleLogin"
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="auth.loading" style="width: 100%" @click="handleLogin">
            登录
          </el-button>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>

<script setup>
import { reactive } from 'vue'
import { useRouter } from 'vue-router'
import { User, Lock } from '@element-plus/icons-vue'
import { useAuthStore } from '../stores/auth'
import { ElMessage } from 'element-plus'

const auth = useAuthStore()
const router = useRouter()

const form = reactive({ username: '', password: '' })

async function handleLogin() {
  if (!form.username || !form.password) {
    ElMessage.warning('请输入用户名和密码')
    return
  }
  try {
    await auth.login(form.username, form.password)
    router.push('/')
  } catch (err) {
    ElMessage.error(err?.response?.data?.message || '登录失败')
  }
}
</script>

<style scoped>
.login-container {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 100vh;
  background: #f5f5f5;
}
.login-card {
  width: 400px;
}
.login-card h2 {
  text-align: center;
  margin: 0;
}
</style>
