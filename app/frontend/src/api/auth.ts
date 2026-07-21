import { http } from './http'
import type {
  AdminV1ListSessionsResponse,
  AdminV1ListNavigationResponse,
  AdminV1Session,
  AdminV1UpdateProfileRequest,
  AdminV1UpdateProfileResponse
} from './generated'
import type {
  CaptchaResponse,
  ForgotPasswordRequest,
  ForgotPasswordResponse,
  LoginRequest,
  LoginResponse,
  RefreshResponse,
  ResetPasswordRequest,
  SwitchTenantResponse,
  VerifyMfaRequest,
  VerifyMfaResponse
} from '@/types/auth'

export async function getCaptcha(): Promise<CaptchaResponse> {
  const response = await http.get<CaptchaResponse>('/auth/captcha')
  return response.data
}

export async function login(request: LoginRequest): Promise<LoginResponse> {
  const response = await http.post<LoginResponse>('/auth/login', request)
  return response.data
}

export async function platformLogin(request: LoginRequest): Promise<LoginResponse> {
  const response = await http.post<LoginResponse>('/platform/auth/login', request)
  return response.data
}

export async function verifyMfa(request: VerifyMfaRequest): Promise<VerifyMfaResponse> {
  const response = await http.post<VerifyMfaResponse>('/auth/mfa/verify', request)
  return response.data
}

export async function forgotPassword(
  request: ForgotPasswordRequest
): Promise<ForgotPasswordResponse> {
  const response = await http.post<ForgotPasswordResponse>('/auth/forgot-password', request)
  return response.data
}

export async function resetPassword(request: ResetPasswordRequest): Promise<void> {
  await http.post('/auth/reset-password', request)
}

export async function logout(): Promise<void> {
  await http.post('/auth/logout')
}

export async function platformLogout(): Promise<void> {
  await http.post('/platform/auth/logout')
}

export async function refresh(): Promise<RefreshResponse> {
  const response = await http.post<RefreshResponse>('/auth/refresh', {})
  return response.data
}

export async function platformRefresh(): Promise<RefreshResponse> {
  const response = await http.post<RefreshResponse>('/platform/auth/refresh', {})
  return response.data
}

export async function switchTenant(tenantId: string): Promise<SwitchTenantResponse> {
  const response = await http.post<SwitchTenantResponse>('/auth/switch-tenant', { tenantId })
  return response.data
}

export async function exitImpersonation(): Promise<RefreshResponse> {
  const response = await http.post<RefreshResponse>('/auth/exit-impersonation', {})
  return response.data
}

export async function impersonate(tenantId: string): Promise<LoginResponse> {
  const response = await http.post<LoginResponse>('/platform/auth/impersonate', { tenantId })
  return response.data
}

export async function listSessions(): Promise<AdminV1ListSessionsResponse> {
  const response = await http.get<AdminV1ListSessionsResponse>('/auth/sessions')
  return response.data
}

export async function platformListSessions(): Promise<AdminV1ListSessionsResponse> {
  const response = await http.get<AdminV1ListSessionsResponse>('/platform/auth/sessions')
  return response.data
}

export async function listNavigation(): Promise<AdminV1ListNavigationResponse> {
  const response = await http.get<AdminV1ListNavigationResponse>('/auth/navigation')
  return response.data
}

export async function platformListNavigation(): Promise<AdminV1ListNavigationResponse> {
  const response = await http.get<AdminV1ListNavigationResponse>('/platform/auth/navigation')
  return response.data
}

export async function revokeSession(sessionId: string): Promise<void> {
  await http.delete('/auth/sessions/' + encodeURIComponent(sessionId))
}

export async function platformRevokeSession(sessionId: string): Promise<void> {
  await http.delete('/platform/auth/sessions/' + encodeURIComponent(sessionId))
}

export async function updateProfile(
  request: AdminV1UpdateProfileRequest
): Promise<AdminV1UpdateProfileResponse> {
  const response = await http.put<AdminV1UpdateProfileResponse>('/auth/profile', request)
  return response.data
}

export type DeviceSession = AdminV1Session
