<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { storeToRefs } from 'pinia'
import { BarChart } from 'echarts/charts'
import { GridComponent, TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import { init, use, type ECharts } from 'echarts/core'

import * as managementApi from '@/api/management'
import AppIcon from '@/components/icons/AppIcon.vue'
import { Refresh } from '@/components/icons/actions'
import type { IconName } from '@/components/icons/registry'
import { useAuthStore } from '@/stores/auth'
import type { ResourceRow } from '@/api/management'

use([BarChart, GridComponent, TooltipComponent, CanvasRenderer])

const authStore = useAuthStore()
const { currentTenant } = storeToRefs(authStore)
const loading = ref(false)
const memberTotal = ref(0)
const roleTotal = ref(0)
const requestTotal = ref(0)
const exceptionTotal = ref(0)
const activities = ref<ResourceRow[]>([])
const dailyCounts = ref<number[]>(Array.from({ length: 7 }, () => 0))
const chartElement = ref<globalThis.HTMLElement>()
let chart: ECharts | undefined

const perspective = computed(() =>
  String(currentTenant.value?.id ?? '0') === '0' ? '平台治理视角' : '当前租户运行视角'
)
const metrics = computed(() => [
  {
    label: '成员总数',
    value: memberTotal.value.toLocaleString('zh-CN'),
    note: '有效组织成员',
    icon: 'members' as IconName
  },
  {
    label: '角色数量',
    value: roleTotal.value.toLocaleString('zh-CN'),
    note: '当前权限角色',
    icon: 'roles' as IconName
  },
  {
    label: 'API 日志',
    value: requestTotal.value.toLocaleString('zh-CN'),
    note: '当前可见记录',
    icon: 'logs' as IconName
  },
  {
    label: '异常请求',
    value: exceptionTotal.value.toLocaleString('zh-CN'),
    note: '最近 200 条样本',
    icon: 'security' as IconName
  }
])

function dateKey(date: Date) {
  return `${date.getFullYear()}-${date.getMonth() + 1}-${date.getDate()}`
}

function buildTrend(rows: ResourceRow[]) {
  const days = Array.from({ length: 7 }, (_, index) => {
    const date = new Date()
    date.setDate(date.getDate() - (6 - index))
    return date
  })
  const counts = new Map(days.map((day) => [dateKey(day), 0]))
  for (const row of rows) {
    const createdAt = String(row.created_at ?? '')
    if (!createdAt) continue
    const key = dateKey(new Date(createdAt))
    if (counts.has(key)) counts.set(key, (counts.get(key) ?? 0) + 1)
  }
  dailyCounts.value = days.map((day) => counts.get(dateKey(day)) ?? 0)
  return days.map((day) => `${day.getMonth() + 1}/${day.getDate()}`)
}

function renderChart(labels: string[]) {
  if (!chartElement.value || chartElement.value.clientWidth === 0) return
  chart ??= init(chartElement.value)
  chart.setOption({
    animationDuration: 420,
    grid: { left: 12, right: 12, top: 20, bottom: 24, containLabel: true },
    tooltip: { trigger: 'axis' },
    xAxis: {
      type: 'category',
      data: labels,
      axisTick: { show: false },
      axisLine: { lineStyle: { color: '#d7d7d7' } }
    },
    yAxis: { type: 'value', minInterval: 1, splitLine: { lineStyle: { color: '#eeeeee' } } },
    series: [
      {
        type: 'bar',
        data: dailyCounts.value,
        barMaxWidth: 34,
        itemStyle: {
          color: (value: { dataIndex: number }) => (value.dataIndex === 6 ? '#ed1515' : '#464646'),
          borderRadius: [2, 2, 0, 0]
        }
      }
    ]
  })
}

async function load() {
  loading.value = true
  try {
    const [members, roles, apiLogs, auditLogs] = await Promise.all([
      managementApi.listResources('members', { page: 1, page_size: 1 }),
      managementApi.listResources('roles', { page: 1, page_size: 1 }),
      managementApi.listResources('api-logs', { page: 1, page_size: 200, sort: 'created_at:desc' }),
      managementApi.listResources('audit-logs', { page: 1, page_size: 6, sort: 'created_at:desc' })
    ])
    memberTotal.value = Number(members.total ?? 0)
    roleTotal.value = Number(roles.total ?? 0)
    requestTotal.value = Number(apiLogs.total ?? 0)
    const apiItems = apiLogs.items ?? []
    exceptionTotal.value = apiItems.filter((item) => Number(item.status_code ?? 0) >= 400).length
    activities.value = auditLogs.items ?? []
    const labels = buildTrend(apiItems)
    await nextTick()
    renderChart(labels)
  } finally {
    loading.value = false
  }
}

function formatActivityTime(value: unknown) {
  if (!value) return '—'
  return new Date(String(value)).toLocaleString('zh-CN', {
    month: 'numeric',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false
  })
}

function resizeChart() {
  chart?.resize()
}
onMounted(() => {
  globalThis.addEventListener('resize', resizeChart)
  void load()
})
onBeforeUnmount(() => {
  globalThis.removeEventListener('resize', resizeChart)
  chart?.dispose()
})
</script>

<template>
  <section v-loading="loading" class="dashboard">
    <header class="page-heading">
      <div>
        <p class="eyebrow">OVERVIEW</p>
        <h1>工作台</h1>
        <p>{{ perspective }}，展示真实业务数据与安全动态。</p>
      </div>
      <el-button :icon="Refresh" @click="load">刷新数据</el-button>
    </header>

    <div class="metric-grid">
      <article v-for="metric in metrics" :key="metric.label" class="metric-card">
        <AppIcon class="metric-card__icon" :name="metric.icon" :size="20" />
        <span>{{ metric.label }}</span><strong>{{ metric.value }}</strong><small>{{ metric.note }}</small>
      </article>
    </div>

    <div class="dashboard-grid">
      <article class="panel trend-panel">
        <div class="panel-title">
          <div>
            <span>REQUESTS</span>
            <h2>请求趋势</h2>
          </div>
          <small>最近 7 天可见日志</small>
        </div>
        <div ref="chartElement" class="chart" aria-label="最近七天请求趋势图"></div>
      </article>
      <article class="panel activity-panel">
        <div class="panel-title">
          <div>
            <span>SECURITY</span>
            <h2>安全动态</h2>
          </div>
        </div>
        <ul v-if="activities.length">
          <li v-for="activity in activities" :key="String(activity.id)">
            <time>{{ formatActivityTime(activity.created_at) }}</time>
            <span>{{
              activity.summary || `${activity.action ?? '操作'} ${activity.resource_type ?? ''}`
            }}</span>
          </li>
        </ul>
        <div v-else class="empty-state">
          <AppIcon name="empty" :size="28" />
          <span>暂无可见安全动态</span>
        </div>
      </article>
    </div>
  </section>
</template>

<style scoped lang="scss">
.dashboard {
  display: grid;
  gap: 24px;
}
.page-heading {
  display: flex;
  align-items: end;
  justify-content: space-between;
  gap: 18px;
}
.eyebrow,
.panel-title span {
  margin: 0 0 5px;
  color: var(--ka-accent);
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 0.18em;
}
h1 {
  margin: 0;
  font-size: clamp(28px, 4vw, 42px);
  letter-spacing: -0.04em;
}
.page-heading p:last-child {
  margin: 8px 0 0;
  color: var(--ka-muted);
}
.metric-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
}
.metric-card,
.panel {
  border: 1px solid rgb(0 0 0 / 4%);
  background: #fff;
  box-shadow: var(--ka-shadow);
}
.metric-card {
  position: relative;
  min-height: 142px;
  display: grid;
  align-content: center;
  gap: 7px;
  padding: 24px;
  overflow: hidden;
}
.metric-card__icon {
  position: absolute;
  top: 18px;
  right: 18px;
  color: rgb(0 0 0 / 14%);
}
.metric-card::before {
  position: absolute;
  inset: 0 auto 0 0;
  width: 3px;
  background: var(--ka-accent);
  content: '';
}
.metric-card span,
.metric-card small,
.panel-title small {
  color: var(--ka-muted);
  font-size: 12px;
}
.metric-card strong {
  font-size: 30px;
  letter-spacing: -0.03em;
}
.dashboard-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.7fr) minmax(300px, 1fr);
  gap: 16px;
}
.panel {
  min-height: 310px;
  padding: 25px;
}
.panel-title {
  display: flex;
  align-items: start;
  justify-content: space-between;
}
.panel-title h2 {
  margin: 0;
  font-size: 18px;
}
.chart {
  height: 230px;
  margin-top: 18px;
}
ul {
  margin: 16px 0 0;
  padding: 0;
  list-style: none;
}
li {
  display: grid;
  grid-template-columns: 92px 1fr;
  gap: 10px;
  padding: 14px 0;
  border-bottom: 1px solid #eee;
  font-size: 13px;
  line-height: 1.5;
}
time {
  color: var(--ka-muted);
  font-size: 11px;
}
.empty-state {
  min-height: 210px;
  display: grid;
  place-items: center;
  gap: 10px;
  color: var(--ka-muted);
}
@media (max-width: 900px) {
  .metric-grid {
    grid-template-columns: repeat(2, 1fr);
  }
  .dashboard-grid {
    grid-template-columns: 1fr;
  }
}
@media (max-width: 520px) {
  .page-heading {
    align-items: start;
  }
  .page-heading .el-button {
    display: none;
  }
  .metric-grid {
    gap: 10px;
  }
  .metric-card {
    min-height: 112px;
    padding: 17px;
  }
  .metric-card strong {
    font-size: 23px;
  }
  .panel {
    padding: 18px;
  }
}
</style>
