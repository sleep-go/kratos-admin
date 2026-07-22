import axios from 'axios'

const api = axios.create({
  baseURL: '/v1/admins',
  withCredentials: true,
})

export async function login(username, password) {
  const { data } = await api.post('/login', { username, password })
  return data
}

export async function logout() {
  await api.post('/logout', {})
}

export async function getCurrent() {
  const { data } = await api.get('/current')
  return data
}

export async function listAdmins(params = {}) {
  const { data } = await api.get('/list', { params })
  return data
}

export async function createAdmin(admin) {
  const { data } = await api.post('/create', { admin })
  return data
}

export async function deleteAdmin(id) {
  await api.delete(`/${id}`)
}

export default api
