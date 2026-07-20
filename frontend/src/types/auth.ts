import type {
  AdminV1CurrentUser,
  AdminV1GetCaptchaResponse,
  AdminV1ForgotPasswordRequest,
  AdminV1ForgotPasswordResponse,
  AdminV1LoginRequest,
  AdminV1LoginResponse,
  AdminV1RefreshResponse,
  AdminV1SwitchTenantResponse,
  AdminV1TenantSummary,
  AdminV1ResetPasswordRequest,
  AdminV1VerifyMfaRequest,
  AdminV1VerifyMfaResponse
} from '@/api/generated'

export type TenantSummary = AdminV1TenantSummary
export type CurrentUser = AdminV1CurrentUser
export type CaptchaResponse = AdminV1GetCaptchaResponse
export type ForgotPasswordRequest = AdminV1ForgotPasswordRequest
export type ForgotPasswordResponse = AdminV1ForgotPasswordResponse
export type ResetPasswordRequest = AdminV1ResetPasswordRequest
export type VerifyMfaRequest = AdminV1VerifyMfaRequest
export type VerifyMfaResponse = AdminV1VerifyMfaResponse
export type LoginRequest = AdminV1LoginRequest
export type LoginResponse = AdminV1LoginResponse
export type RefreshResponse = AdminV1RefreshResponse
export type SwitchTenantResponse = AdminV1SwitchTenantResponse
