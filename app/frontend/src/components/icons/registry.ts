import type { Component } from 'vue'
import {
  ArrowDown,
  Box,
  Close,
  Connection,
  Document,
  DocumentCopy,
  Folder,
  Key,
  Lock,
  Menu,
  Monitor,
  Odometer,
  OfficeBuilding,
  Setting,
  Switch,
  SwitchButton,
  Upload,
  User,
  UserFilled,
  WarningFilled
} from '@element-plus/icons-vue'

/** 语义化图标名称，供 AppIcon 与导航映射使用 */
export type IconName =
  | 'dashboard'
  | 'platform'
  | 'organization'
  | 'permission'
  | 'logs'
  | 'files'
  | 'settings'
  | 'account'
  | 'members'
  | 'roles'
  | 'tenant'
  | 'arrow-down'
  | 'close'
  | 'menu'
  | 'switch'
  | 'logout'
  | 'upload'
  | 'empty'
  | 'security'
  | 'provider'
  | 'connection'

/** 图标组件注册表 */
export const iconRegistry: Record<IconName, Component> = {
  dashboard: Odometer,
  platform: OfficeBuilding,
  organization: User,
  permission: Lock,
  logs: Document,
  files: Folder,
  settings: Setting,
  account: UserFilled,
  members: User,
  roles: Key,
  tenant: OfficeBuilding,
  'arrow-down': ArrowDown,
  close: Close,
  menu: Menu,
  switch: Switch,
  logout: SwitchButton,
  upload: Upload,
  empty: Box,
  security: WarningFilled,
  provider: Connection,
  connection: Monitor
}

/** 顶栏导航分组标签 → 图标 */
const navigationLabelIcons: Record<string, IconName> = {
  工作台: 'dashboard',
  平台管理: 'platform',
  组织管理: 'organization',
  权限中心: 'permission',
  日志中心: 'logs',
  文件管理: 'files',
  系统设置: 'settings',
  个人中心: 'account'
}

/** 根据导航文案解析图标名称 */
export function resolveNavigationIconName(label: string): IconName | undefined {
  return navigationLabelIcons[label]
}

/** 根据导航文案解析图标组件 */
export function resolveNavigationIcon(label: string): Component | undefined {
  const name = resolveNavigationIconName(label)
  return name ? iconRegistry[name] : undefined
}
