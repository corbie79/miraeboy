// Svelte 5 reactive auth state
import { getToken, setToken } from './api.js'

function parseJwt(token) {
  try {
    const parts = token.split('.')
    if (parts.length !== 3) return null
    const base64 = parts[1].replace(/-/g, '+').replace(/_/g, '/')
    const claims = JSON.parse(atob(base64))
    // Reject expired tokens
    if (claims.exp && claims.exp * 1000 < Date.now()) return null
    return claims
  } catch {
    return null
  }
}

function buildState() {
  const token = getToken()
  if (!token) {
    return { loggedIn: false, username: '', admin: false, groups: {} }
  }
  const claims = parseJwt(token)
  if (!claims) {
    return { loggedIn: false, username: '', admin: false, groups: {} }
  }
  return {
    loggedIn: true,
    username: claims.username || '',
    admin: claims.admin || false,
    groups: claims.groups || {},
  }
}

// Reactive state using $state rune
export let auth = $state(buildState())

export function refreshAuth() {
  const s = buildState()
  auth.loggedIn = s.loggedIn
  auth.username = s.username
  auth.admin = s.admin
  auth.groups = s.groups
}

export function logout() {
  setToken(null)
  refreshAuth()
}
