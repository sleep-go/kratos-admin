<script setup lang="ts">
import { onMounted, ref, shallowRef } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'

import * as fileApi from '@/api/files'
import * as managementApi from '@/api/management'

type FileRow = Record<string, unknown> & { id?: string }
type BrowserFile = InstanceType<typeof globalThis.File>
type FileChangeEvent = {
  target: { files?: { 0?: BrowserFile }; value: string }
}

const items = ref<FileRow[]>([])
const total = shallowRef(0)
const page = shallowRef(1)
const pageSize = 20
const loading = shallowRef(false)
const uploading = shallowRef(false)
const uploadProgress = shallowRef(0)

async function load() {
  loading.value = true
  try {
    const response = await managementApi.listResources('files', {
      page: page.value,
      pageSize
    })
    items.value = (response.items ?? []) as FileRow[]
    total.value = Number(response.total ?? 0)
  } finally {
    loading.value = false
  }
}

async function selectFile(event: unknown) {
  const input = (event as FileChangeEvent).target
  const file = input.files?.[0]
  if (!file) return
  uploading.value = true
  uploadProgress.value = 0
  try {
    const digest = await globalThis.crypto.subtle.digest('SHA-256', await file.arrayBuffer())
    const sha256 = Array.from(new Uint8Array(digest))
      .map((byte) => byte.toString(16).padStart(2, '0'))
      .join('')
    const created = await fileApi.createUpload({
      originalName: file.name,
      contentType: file.type || 'application/octet-stream',
      sizeBytes: String(file.size),
      sha256
    })
    if (!created.fileId || !created.upload) throw new Error('文件预登记响应不完整')
    await fileApi.putSignedFile(created.upload, file, (percent) => {
      uploadProgress.value = percent
    })
    await fileApi.confirmUpload(created.fileId)
    ElMessage.success('上传并校验成功')
    await load()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '上传失败')
  } finally {
    uploading.value = false
    input.value = ''
  }
}

async function download(row: FileRow) {
  const fileId = String(row.id ?? '')
  const response = await fileApi.getDownloadURL(fileId)
  if (!response.download?.url) throw new Error('下载地址为空')
  globalThis.location.assign(response.download.url)
}

async function remove(row: FileRow) {
  const fileId = String(row.id ?? '')
  await ElMessageBox.confirm('删除后文件对象和元数据均不可恢复，确认继续？', '危险操作确认', {
    type: 'warning'
  })
  await fileApi.deleteFile(fileId)
  ElMessage.success('文件已删除')
  await load()
}

function statusText(value: unknown) {
  return (
    ({ 1: '待确认', 2: '可用', 3: '已删除', 4: '清理失败' } as Record<number, string>)[
      Number(value)
    ] ?? '未知'
  )
}

function formatSize(value: unknown) {
  const bytes = Number(value)
  if (!Number.isFinite(bytes)) return '—'
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  return (bytes / 1024 / 1024).toFixed(1) + ' MB'
}

onMounted(load)
</script>

<template>
  <section class="file-page">
    <header class="file-heading">
      <div>
        <p>STORAGE</p>
        <h1>文件管理</h1>
        <span>使用短期签名直传，并由服务端复核对象元数据。</span>
      </div>
      <label v-permission="'files:create'" class="upload-trigger">
        <input type="file" aria-label="选择文件直传" :disabled="uploading" @change="selectFile" />
        {{ uploading ? '上传中' : '选择文件直传' }}
      </label>
    </header>

    <el-progress v-if="uploading" :percentage="uploadProgress" :stroke-width="5" status="success" />

    <div class="file-panel">
      <el-table v-loading="loading" :data="items" stripe>
        <el-table-column prop="original_name" label="文件名" min-width="190" />
        <el-table-column prop="content_type" label="类型" min-width="160" />
        <el-table-column label="大小" width="110">
          <template #default="scope">{{ formatSize(scope.row.size_bytes) }}</template>
        </el-table-column>
        <el-table-column prop="provider_name" label="存储" width="120" />
        <el-table-column label="状态" width="100">
          <template #default="scope">{{ statusText(scope.row.status) }}</template>
        </el-table-column>
        <el-table-column label="操作" fixed="right" width="148">
          <template #default="scope">
            <el-button
              v-permission="'files:download'"
              link
              :disabled="Number(scope.row.status) !== 2"
              @click="download(scope.row)"
            >
              下载
            </el-button>
            <el-button v-permission="'files:delete'" link type="danger" @click="remove(scope.row)">
              删除
            </el-button>
          </template>
        </el-table-column>
      </el-table>
      <div v-if="!loading && items.length === 0" class="empty-state">
        <strong>还没有文件</strong>
        <span>选择文件后将按“预登记 → 直传 → 服务端确认”的链路保存。</span>
      </div>
      <el-pagination
        v-model:current-page="page"
        :page-size="pageSize"
        :total="total"
        layout="total, prev, pager, next"
        @change="load"
      />
    </div>
  </section>
</template>

<style scoped lang="scss">
.file-page {
  display: grid;
  gap: 18px;
}
.file-heading {
  display: flex;
  align-items: end;
  justify-content: space-between;
  gap: 20px;
}
.file-heading p {
  margin: 0 0 5px;
  color: var(--ka-accent);
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 0.18em;
}
.file-heading h1 {
  margin: 0;
  font-size: clamp(26px, 4vw, 38px);
}
.file-heading span {
  display: block;
  margin-top: 8px;
  color: var(--ka-muted);
}
.upload-trigger {
  position: relative;
  min-height: 40px;
  display: inline-flex;
  align-items: center;
  padding: 0 18px;
  color: #fff;
  background: var(--ka-accent);
  box-shadow: 0 8px 18px rgb(237 21 21 / 18%);
  cursor: pointer;
  overflow: hidden;
}
.upload-trigger input {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  opacity: 0;
  cursor: pointer;
}
.file-panel {
  padding: 18px;
  background: #fff;
  box-shadow: var(--ka-shadow);
}
.empty-state {
  min-height: 180px;
  display: grid;
  place-content: center;
  gap: 7px;
  color: var(--ka-muted);
  text-align: center;
}
.empty-state strong {
  color: var(--ka-text);
}
.el-pagination {
  justify-content: flex-end;
  margin-top: 18px;
}
@media (max-width: 640px) {
  .file-heading {
    align-items: start;
    flex-direction: column;
  }
  .upload-trigger {
    width: 100%;
    justify-content: center;
  }
  .file-panel {
    padding: 12px;
    overflow-x: auto;
  }
}
</style>
