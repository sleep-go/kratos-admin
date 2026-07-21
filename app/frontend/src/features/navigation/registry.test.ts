import { buildNavigationTree, resolveNavigation } from './registry'

describe('动态菜单安全映射', () => {
  it('只接受编译期注册的组件键和路由', () => {
    const result = resolveNavigation([
      { name: '文件管理', routePath: '/console/files', componentKey: 'files' },
      { name: '恶意页面', routePath: 'https://evil.example', componentKey: '../../evil' }
    ])

    expect(result.some((item) => item.to === '/console/files')).toBe(true)
    expect(result.some((item) => item.label === '恶意页面')).toBe(false)
  })

  it('按 parentId 树生成顶栏目录与二级菜单', () => {
    const result = buildNavigationTree([
      { id: '1', parentId: '0', name: '日志中心', componentKey: '', sortOrder: 40 },
      {
        id: '2',
        parentId: '1',
        name: '登录日志',
        routePath: '/console/logs/login',
        componentKey: 'login-logs',
        sortOrder: 41
      },
      {
        id: '3',
        parentId: '1',
        name: '操作审计',
        routePath: '/console/logs/audit',
        componentKey: 'audit-logs',
        sortOrder: 42
      },
      {
        id: '4',
        parentId: '1',
        name: 'API 日志',
        routePath: '/console/logs/api',
        componentKey: 'api-logs',
        sortOrder: 43
      },
      {
        id: '5',
        parentId: '1',
        name: '导出记录',
        routePath: '/console/logs/exports',
        componentKey: 'log-exports',
        sortOrder: 44
      },
      {
        id: '6',
        parentId: '0',
        name: '系统设置',
        routePath: '/console/settings',
        componentKey: 'settings',
        sortOrder: 61
      }
    ])

    expect(result.find((item) => item.label === '日志中心')?.children).toEqual([
      { label: '登录日志', to: '/console/logs/login' },
      { label: '操作审计', to: '/console/logs/audit' },
      { label: 'API 日志', to: '/console/logs/api' },
      { label: '导出记录', to: '/console/logs/exports' }
    ])
    expect(result.find((item) => item.label === '系统设置')).toEqual({
      label: '系统设置',
      to: '/console/settings'
    })
  })

  it('只把已注册路由的菜单放入导航树', () => {
    const result = buildNavigationTree([
      { id: '1', parentId: '0', name: '日志中心', componentKey: '', sortOrder: 40 },
      {
        id: '2',
        parentId: '1',
        name: '操作审计',
        routePath: '/console/logs/audit',
        componentKey: 'audit-logs',
        sortOrder: 42
      },
      {
        id: '3',
        parentId: '1',
        name: '未知日志',
        routePath: '/console/logs/unknown',
        componentKey: 'unknown-logs',
        sortOrder: 43
      }
    ])

    expect(result).toEqual([
      {
        label: '日志中心',
        to: '/console/logs/audit',
        children: [{ label: '操作审计', to: '/console/logs/audit' }]
      }
    ])
  })

  it('平台域菜单按 DB 目录分组', () => {
    const result = buildNavigationTree([
      { id: '1', parentId: '0', name: '租户运营', componentKey: '', sortOrder: 10 },
      {
        id: '2',
        parentId: '1',
        name: '租户管理',
        routePath: '/platform/tenants',
        componentKey: 'tenants',
        sortOrder: 11
      },
      {
        id: '3',
        parentId: '0',
        name: '成员管理',
        routePath: '/console/organization/users',
        componentKey: 'members',
        sortOrder: 21
      }
    ])

    expect(result).toEqual([
      {
        label: '租户运营',
        to: '/platform/tenants',
        children: [{ label: '租户管理', to: '/platform/tenants' }]
      },
      { label: '成员管理', to: '/console/organization/users' }
    ])
  })

  it('maps platform log menus to /platform/logs paths', () => {
    const items = resolveNavigation([
      {
        name: '操作审计',
        routePath: '/platform/logs/audit',
        componentKey: 'platform-audit-logs',
        sortOrder: 172
      },
      {
        name: '登录日志',
        routePath: '/platform/logs/login',
        componentKey: 'platform-login-logs',
        sortOrder: 171
      }
    ])
    expect(items.map((item) => item.to)).toEqual(['/platform/logs/login', '/platform/logs/audit'])

    const top = buildNavigationTree([
      { id: '1', parentId: '0', name: '日志审计', componentKey: '', sortOrder: 40 },
      {
        id: '2',
        parentId: '1',
        name: '登录日志',
        routePath: '/platform/logs/login',
        componentKey: 'platform-login-logs',
        sortOrder: 171
      },
      {
        id: '3',
        parentId: '1',
        name: '操作审计',
        routePath: '/platform/logs/audit',
        componentKey: 'platform-audit-logs',
        sortOrder: 172
      }
    ])
    expect(top.find((item) => item.label === '日志审计')?.children).toEqual([
      { label: '登录日志', to: '/platform/logs/login' },
      { label: '操作审计', to: '/platform/logs/audit' }
    ])
  })

  it('注册 app-users 与平台 RBAC 路由', () => {
    const items = resolveNavigation([
      {
        name: 'App 用户',
        routePath: '/platform/app-users',
        componentKey: 'app-users',
        sortOrder: 21
      },
      {
        name: '平台角色',
        routePath: '/platform/permission/roles',
        componentKey: 'platform-roles',
        sortOrder: 23
      },
      {
        name: '按钮与 API 授权',
        routePath: '/platform/permission/policies',
        componentKey: 'platform-casbin-rules',
        sortOrder: 24
      }
    ])

    expect(items.map((item) => item.to)).toEqual([
      '/platform/app-users',
      '/platform/permission/roles',
      '/platform/permission/policies'
    ])
  })
})
