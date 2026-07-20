import { http } from './http'
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

export async function refresh(): Promise<RefreshResponse> {
  const response = await http.post<RefreshResponse>('/auth/refresh', {})
  return response.data
}

export async function switchTenant(tenantId: string): Promise<SwitchTenantResponse> {
  const response = await http.post<SwitchTenantResponse>('/auth/switch-tenant', { tenantId })
  return response.data
}
