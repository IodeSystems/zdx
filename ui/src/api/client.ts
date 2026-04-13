// Typed HTTP client generated from /openapi.json via openapi-typescript + openapi-fetch.
// Regenerate with: bin/api-types
import createClient from 'openapi-fetch'
import type { paths } from '../api.gen'

export const TOKEN_KEY = 'zdx_api_token'

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

export function setToken(token: string) {
  localStorage.setItem(TOKEN_KEY, token)
}

export function clearToken() {
  localStorage.removeItem(TOKEN_KEY)
}

export const client = createClient<paths>({
  baseUrl: '/',
  headers: {},
  fetch: (req) => {
    const token = getToken()
    if (token) {
      (req as Request).headers.set('X-Api-Key', token)
    }
    return fetch(req)
  },
})
