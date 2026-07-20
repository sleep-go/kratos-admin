import axios from 'axios'

export const http = axios.create({
  baseURL: '/api/v1',
  timeout: 15_000,
  withCredentials: true
})

export function setAccessToken(token: string | null) {
  if (token) {
    http.defaults.headers.common.Authorization = `Bearer ${token}`
    return
  }
  delete http.defaults.headers.common.Authorization
}
