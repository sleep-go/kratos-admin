<script setup lang="ts">
const metrics = [
  { label: '成员总数', value: '1,286', trend: '+8.2%', tone: 'red' },
  { label: '角色数量', value: '24', trend: '+2', tone: 'teal' },
  { label: '今日请求', value: '84,932', trend: '+12.5%', tone: 'blue' },
  { label: '异常请求', value: '36', trend: '-18.3%', tone: 'amber' }
]

const activities = [
  ['10:42', '管理员更新了「财务主管」角色权限'],
  ['09:18', '检测到新设备登录，已完成 MFA 验证'],
  ['昨天', 'API 日志导出任务执行完成']
]
</script>

<template>
  <section class="dashboard">
    <header class="page-heading">
      <div>
        <p class="eyebrow">OVERVIEW</p>
        <h1>工作台</h1>
        <p>查看当前租户的运行状态与安全动态。</p>
      </div>
      <button type="button">刷新数据</button>
    </header>

    <div class="metric-grid">
      <article v-for="metric in metrics" :key="metric.label" class="metric-card">
        <span class="metric-label">{{ metric.label }}</span>
        <strong>{{ metric.value }}</strong>
        <span class="metric-trend" :class="`metric-trend--${metric.tone}`">{{ metric.trend }}</span>
      </article>
    </div>

    <div class="dashboard-grid">
      <article class="panel trend-panel">
        <div class="panel-title">
          <div>
            <span>REQUESTS</span>
            <h2>请求趋势</h2>
          </div>
          <small>最近 7 天</small>
        </div>
        <div class="chart" aria-label="最近七天请求趋势图">
          <i
            v-for="height in [38, 56, 44, 72, 62, 86, 74]"
            :key="height"
            :style="{ height: `${height}%` }"
          ></i>
        </div>
      </article>

      <article class="panel activity-panel">
        <div class="panel-title">
          <div>
            <span>SECURITY</span>
            <h2>安全动态</h2>
          </div>
        </div>
        <ul>
          <li v-for="activity in activities" :key="activity[0]">
            <time>{{ activity[0] }}</time><span>{{ activity[1] }}</span>
          </li>
        </ul>
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
.page-heading button {
  padding: 10px 17px;
  border: 1px solid #ccc;
  border-radius: 3px;
  background: #fff;
  cursor: pointer;
}
.metric-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
}
.metric-card,
.panel {
  border: 1px solid rgb(0 0 0 / 4%);
  border-radius: 5px;
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
.metric-card::before {
  position: absolute;
  inset: 0 auto 0 0;
  width: 3px;
  background: var(--ka-accent);
  content: '';
}
.metric-label {
  color: var(--ka-muted);
  font-size: 13px;
}
.metric-card strong {
  font-size: 30px;
  letter-spacing: -0.03em;
}
.metric-trend {
  position: absolute;
  right: 20px;
  bottom: 23px;
  color: #008f82;
  font-size: 12px;
}
.metric-trend--red {
  color: var(--ka-accent);
}
.metric-trend--amber {
  color: #b97800;
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
  justify-content: space-between;
  align-items: start;
}
.panel-title h2 {
  margin: 0;
  font-size: 18px;
}
.panel-title small {
  color: var(--ka-muted);
}
.chart {
  height: 210px;
  display: flex;
  align-items: end;
  gap: clamp(8px, 2vw, 24px);
  padding: 34px 10px 0;
  border-bottom: 1px solid #ddd;
  background: repeating-linear-gradient(to bottom, transparent 0 51px, #eee 52px);
}
.chart i {
  flex: 1;
  min-width: 16px;
  border-radius: 2px 2px 0 0;
  background: linear-gradient(#777, #252525);
}
.chart i:last-child {
  background: linear-gradient(#f25b5b, #d60000);
}
ul {
  margin: 24px 0 0;
  padding: 0;
  list-style: none;
}
li {
  display: grid;
  grid-template-columns: 48px 1fr;
  gap: 12px;
  padding: 15px 0;
  border-bottom: 1px solid #eee;
  font-size: 13px;
  line-height: 1.5;
}
time {
  color: var(--ka-muted);
  font-size: 11px;
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
  .page-heading button {
    display: none;
  }
  .metric-grid {
    grid-template-columns: 1fr 1fr;
    gap: 10px;
  }
  .metric-card {
    min-height: 112px;
    padding: 17px;
  }
  .metric-card strong {
    font-size: 23px;
  }
  .metric-trend {
    position: static;
  }
  .panel {
    padding: 18px;
  }
}
</style>
