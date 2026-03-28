// API client with automatic JWT injection

let token = localStorage.getItem('token') || null

export function setToken(t) {
  token = t
  if (t) {
    localStorage.setItem('token', t)
  } else {
    localStorage.removeItem('token')
  }
}

export function getToken() {
  return token
}

export function isLoggedIn() {
  return !!token
}

async function request(method, path, body) {
  const headers = {}
  if (token) {
    headers['Authorization'] = `Bearer ${token}`
  }
  if (body && typeof body === 'object') {
    headers['Content-Type'] = 'application/json'
    body = JSON.stringify(body)
  }

  const res = await fetch(path, { method, headers, body })

  if (res.status === 204) return null

  const data = await res.json().catch(() => null)

  if (!res.ok) {
    const msg = data?.error || `HTTP ${res.status}`
    throw new Error(msg)
  }
  return data
}

export const api = {
  get: (path) => request('GET', path),
  post: (path, body) => request('POST', path, body),
  patch: (path, body) => request('PATCH', path, body),
  put: (path, body) => request('PUT', path, body),
  delete: (path) => request('DELETE', path),
}

// Auth
export async function login(username, password) {
  // Use the first available repo for auth, or a dedicated endpoint
  // We send Basic Auth to get a JWT back
  const res = await fetch('/api/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  })
  const data = await res.json()
  if (!res.ok) throw new Error(data.error || 'Login failed')
  setToken(data.token)
  return data
}

// Repos
export const repos = {
  list: () => api.get('/api/repos'),
  get: (name) => api.get(`/api/repos/${name}`),
  create: (body) => api.post('/api/repos', body),
  update: (name, body) => api.patch(`/api/repos/${name}`, body),
  delete: (name, force) => api.delete(`/api/repos/${name}${force ? '?force=true' : ''}`),
}

// Members
export const members = {
  list: (repo) => api.get(`/api/repos/${repo}/members`),
  invite: (repo, body) => api.post(`/api/repos/${repo}/members`, body),
  update: (repo, username, body) => api.put(`/api/repos/${repo}/members/${username}`, body),
  remove: (repo, username) => api.delete(`/api/repos/${repo}/members/${username}`),
}

// Packages (search)
export const packages = {
  search: (repo, query) => api.get(`/api/conan/${repo}/v2/conans/search?q=${encodeURIComponent(query || '*')}`),
}
