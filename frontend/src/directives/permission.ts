import type { Directive, DirectiveBinding } from 'vue'

import { useAuthStore } from '@/stores/auth'

export function hasPermission(
  granted: string[] | undefined,
  required: string,
  platformAdmin = false
) {
  if (platformAdmin) return true
  if (!required) return true
  const [resource] = required.split(':', 1)
  return Boolean(
    granted?.includes('*:*') || granted?.includes(required) || granted?.includes(`${resource}:*`)
  )
}

function applyPermission(element: HTMLElement, binding: DirectiveBinding<string>) {
  const authStore = useAuthStore()
  element.hidden = !hasPermission(
    authStore.currentUser?.permissions,
    binding.value,
    authStore.currentUser?.platformAdmin
  )
}

// permissionDirective 根据服务端下发的权限集合控制按钮等操作元素的可见性。
export const permissionDirective: Directive<HTMLElement, string> = {
  mounted: applyPermission,
  updated: applyPermission
}
