export interface TenantSummary {
  id: string
  name: string
}

export interface CurrentUser {
  id: string
  displayName: string
  avatarUrl?: string
  platformAdmin: boolean
}

export interface LoginRequest {
  identifier: string
  password: string
}

export interface LoginResponse {
  accessToken: string
  expiresAt: string
  user: CurrentUser
  tenants: TenantSummary[]
  currentTenant?: TenantSummary
  mfaChallengeId?: string
}
