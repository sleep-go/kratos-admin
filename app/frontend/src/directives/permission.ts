import type { Directive, DirectiveBinding } from 'vue'

import { useAuthStore } from '@/stores/auth'

export function hasPermission(
  granted: string[] | undefined,
  required: string,
  platformContext = false
) {
  if (platformContext) return true
  if (!required) return true
  const [resource] = required.split(':', 1)
  return Boolean(
    granted?.includes('*:*') || granted?.includes(required) || granted?.includes(`${resource}:*`)
  )
}

function applyPermission(element: HTMLElement, binding: DirectiveBinding<string>) {
  const authStore = useAuthStore()
  const platformContext =
    authStore.currentUser?.realm === 'platform' && Number(authStore.currentTenant?.id ?? 0) === 0
  element.hidden = !hasPermission(
    authStore.currentUser?.permissions,
    binding.value,
    platformContext
  )
}

// permissionDirective 根据服务端下发的权限集合控制按钮等操作元素的可见性。
export const permissionDirective: Directive<HTMLElement, string> = {
  mounted: applyPermission,
  updated: applyPermission
}
