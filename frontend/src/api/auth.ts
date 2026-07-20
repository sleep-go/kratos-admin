import { http } from './http'
import type { LoginRequest, LoginResponse, RefreshResponse } from '@/types/auth'

export async function login(request: LoginRequest): Promise<LoginResponse> {
  const response = await http.post<LoginResponse>('/auth/login', request)
  return response.data
}

export async function logout(): Promise<void> {
  await http.post('/auth/logout')
}

export async function refresh(): Promise<RefreshResponse> {
  const response = await http.post<RefreshResponse>('/auth/refresh', {})
  return response.data
}
