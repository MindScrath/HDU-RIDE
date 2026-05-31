// stores/session.ts
import { create } from 'zustand'
import { api } from '@/lib/api'
import type { User } from '@/lib/types'

interface SessionState {
  user: User | null
  initialized: boolean
  loading: boolean
  fetchSession: () => Promise<void>
  login: (username: string, password: string) => Promise<void>
  logout: () => Promise<void>
  changePassword: (oldPassword: string, newPassword: string) => Promise<void>
  isAdmin: () => boolean
  canTeach: () => boolean
}

export const useSession = create<SessionState>((set, get) => ({
  user: null,
  initialized: false,
  loading: false,

  fetchSession: async () => {
    set({ loading: true })
    try {
      const data = await api.get<{ user: User }>('/api/session')
      set({ user: data.user, initialized: true, loading: false })
    } catch {
      set({ user: null, initialized: true, loading: false })
    }
  },

  login: async (username: string, password: string) => {
    const data = await api.post<{ user: User }>('/api/login', { username, password })
    set({ user: data.user })
  },

  logout: async () => {
    await api.post('/api/logout')
    set({ user: null })
  },

  changePassword: async (oldPassword: string, newPassword: string) => {
    await api.patch('/api/me/password', { oldPassword, newPassword })
  },

  isAdmin: () => {
    const role = get().user?.role
    return role === 'root' || role === 'admin'
  },

  canTeach: () => {
    const role = get().user?.role
    return ['root', 'admin', 'teacher', 'assistant'].includes(role ?? '')
  },
}))
