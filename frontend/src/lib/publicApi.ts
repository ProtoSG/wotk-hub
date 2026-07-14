import axios from 'axios'

// Deliberately separate from the main `api` instance (src/lib/axios.ts): this
// one backs the token-gated public ytdlp page and must never send auth
// cookies or go through the login-refresh/redirect interceptor — a wrong or
// expired token should just show an error, not bounce the visitor to /login.
const publicApi = axios.create({
  baseURL: import.meta.env.VITE_API_URL ?? 'http://localhost:3001',
  timeout: 30000,
})

export default publicApi
