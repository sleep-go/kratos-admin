import { resolveNavigation, resolveTopNavigation } from './registry'

describe('动态菜单安全映射', () => {
  it('只接受编译期注册的组件键和路由', () => {
    const result = resolveNavigation([
      { name: '文件管理', routePath: '/console/files', componentKey: 'files' },
      { name: '恶意页面', routePath: 'https://evil.example', componentKey: '../../evil' }
    ])

    expect(result.some((item) => item.to === '/console/files')).toBe(true)
    expect(result.some((item) => item.label === '恶意页面')).toBe(false)
  })

  it('按顶部导航分组生成已授权的二级菜单', () => {
    const result = resolveTopNavigation(
      resolveNavigation([
        { name: '登录日志', routePath: '/console/logs/login', componentKey: 'login-logs', sortOrder: 41 },
        { name: '操作审计', routePath: '/console/logs/audit', componentKey: 'audit-logs', sortOrder: 42 },
        { name: 'API 日志', routePath: '/console/logs/api', componentKey: 'api-logs', sortOrder: 43 },
        {
          name: '导出记录',
          routePath: '/console/logs/exports',
          componentKey: 'log-exports',
          sortOrder: 44
        },
        { name: '系统设置', routePath: '/console/settings', componentKey: 'settings', sortOrder: 61 }
      ])
    )

    expect(result.find((item) => item.label === '日志中心')?.children).toEqual([
      { label: '登录日志', to: '/console/logs/login' },
      { label: '操作审计', to: '/console/logs/audit' },
      { label: 'API 日志', to: '/console/logs/api' },
      { label: '导出记录', to: '/console/logs/exports' }
    ])
    expect(result.find((item) => item.label === '系统设置')?.children).toEqual([
      { label: '系统设置', to: '/console/settings' }
    ])
  })

  it('只把当前账号实际拥有的页面放入二级菜单', () => {
    const result = resolveTopNavigation(
      resolveNavigation([
        { name: '操作审计', routePath: '/console/logs/audit', componentKey: 'audit-logs', sortOrder: 42 },
        {
          name: '未知日志',
          routePath: '/console/logs/unknown',
          componentKey: 'unknown-logs',
          sortOrder: 43
        }
      ])
    )

    expect(result.find((item) => item.label === '日志中心')).toEqual({
      label: '日志中心',
      to: '/console/logs/audit',
      children: [{ label: '操作审计', to: '/console/logs/audit' }]
    })
  })

  it('平台域菜单只展示平台路由', () => {
    const resolved = resolveNavigation([
      { name: '租户管理', routePath: '/platform/tenants', componentKey: 'tenants', sortOrder: 12 },
      { name: '成员管理', routePath: '/console/organization/users', componentKey: 'members', sortOrder: 21 }
    ])
    const result = resolveTopNavigation(resolved, 'platform')

    expect(result).toEqual([
      {
        label: '平台治理',
        to: '/platform/tenants',
        children: [{ label: '租户管理', to: '/platform/tenants' }]
      }
    ])
  })
})
