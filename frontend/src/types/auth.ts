import type {
  AdminV1CurrentUser,
  AdminV1LoginRequest,
  AdminV1LoginResponse,
  AdminV1RefreshResponse,
  AdminV1SwitchTenantResponse,
  AdminV1TenantSummary
} from '@/api/generated'

export type TenantSummary = AdminV1TenantSummary
export type CurrentUser = AdminV1CurrentUser
export type LoginRequest = AdminV1LoginRequest
export type LoginResponse = AdminV1LoginResponse
export type RefreshResponse = AdminV1RefreshResponse
export type SwitchTenantResponse = AdminV1SwitchTenantResponse
