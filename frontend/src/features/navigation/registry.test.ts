import { resolveNavigation } from './registry'

describe('动态菜单安全映射', () => {
  it('只接受编译期注册的组件键和路由', () => {
    const result = resolveNavigation([
      { name: '文件管理', routePath: '/files', componentKey: 'files' },
      { name: '恶意页面', routePath: 'https://evil.example', componentKey: '../../evil' }
    ])

    expect(result.some((item) => item.to === '/files')).toBe(true)
    expect(result.some((item) => item.label === '恶意页面')).toBe(false)
  })
})
