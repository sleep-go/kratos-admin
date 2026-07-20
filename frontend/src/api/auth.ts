import { http } from './http'
import type { LoginRequest, LoginResponse } from '@/types/auth'

export async function login(request: LoginRequest): Promise<LoginResponse> {
  const response = await http.post<LoginResponse>('/auth/login', request)
  return response.data
}

export async function logout(): Promise<void> {
  await http.post('/auth/logout')
}
