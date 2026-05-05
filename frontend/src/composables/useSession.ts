import { computed, reactive } from 'vue'
import { api } from '../api'
import type { User } from '../types'

const state = reactive({
  user: null as User | null,
  loading: false,
  initialized: false
})

export function useSession() {
  const signedIn = computed(() => Boolean(state.user))
  const isAdmin = computed(() => state.user?.role === 'root' || state.user?.role === 'admin')
  const canTeach = computed(() => ['root', 'admin', 'teacher', 'assistant'].includes(state.user?.role ?? ''))

  async function fetchSession() {
    state.loading = true
    try {
      const data = await api.get<{ user: User }>('/api/session')
      state.user = data.user
    } catch {
      state.user = null
    } finally {
      state.loading = false
      state.initialized = true
    }
  }

  async function login(username: string, password: string) {
    const data = await api.post<{ user: User }>('/api/login', { username, password })
    state.user = data.user
  }

  async function logout() {
    await api.post('/api/logout')
    state.user = null
  }

  return { state, signedIn, isAdmin, canTeach, fetchSession, login, logout }
}
