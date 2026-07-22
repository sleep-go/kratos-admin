<template>
  <div class="admin-list">
    <el-card>
      <template #header>
        <div class="header">
          <span>管理员列表</span>
          <div class="header-right">
            <span class="current-user">{{ auth.user?.name }}</span>
            <el-button size="small" @click="handleLogout">退出登录</el-button>
          </div>
        </div>
      </template>

      <div class="toolbar">
        <el-input v-model="filter" placeholder="过滤条件，如 name=&quot;admin&quot;" clearable style="width: 300px" @clear="fetchAdmins" @keyup.enter="fetchAdmins" />
        <el-button @click="fetchAdmins">查询</el-button>
      </div>

      <el-table :data="admins" v-loading="loading" border style="margin-top: 16px">
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="name" label="用户名" />
        <el-table-column prop="email" label="邮箱" />
        <el-table-column prop="access" label="权限" width="100" />
        <el-table-column label="创建时间" width="180">
          <template #default="{ row }">
            {{ formatTime(row.createTime) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="120">
          <template #default="{ row }">
            <el-button type="danger" size="small" @click="handleDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { listAdmins, deleteAdmin } from '../api/admin'
import { ElMessage, ElMessageBox } from 'element-plus'

const auth = useAuthStore()
const router = useRouter()

const admins = ref([])
const loading = ref(false)
const filter = ref('')

async function fetchAdmins() {
  loading.value = true
  try {
    const params = { page_size: 50 }
    if (filter.value) {
      params.filter = filter.value
    }
    const data = await listAdmins(params)
    admins.value = data.admins || []
  } catch (err) {
    ElMessage.error(err?.response?.data?.message || '获取列表失败')
  } finally {
    loading.value = false
  }
}

function formatTime(ts) {
  if (!ts) return ''
  if (typeof ts === 'string') return ts
  if (ts.seconds) return new Date(Number(ts.seconds) * 1000).toLocaleString()
  return ''
}

async function handleDelete(row) {
  try {
    await ElMessageBox.confirm(`确定删除管理员 ${row.name}?`, '提示', { type: 'warning' })
    await deleteAdmin(row.id)
    ElMessage.success('删除成功')
    fetchAdmins()
  } catch (err) {
    if (err !== 'cancel') {
      ElMessage.error(err?.response?.data?.message || '删除失败')
    }
  }
}

async function handleLogout() {
  await auth.logout()
  router.push('/login')
}

onMounted(fetchAdmins)
</script>

<style scoped>
.admin-list {
  max-width: 1000px;
  margin: 40px auto;
  padding: 0 20px;
}
.header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.header-right {
  display: flex;
  align-items: center;
  gap: 12px;
}
.current-user {
  color: #606266;
  font-size: 14px;
}
.toolbar {
  display: flex;
  gap: 8px;
}
</style>
