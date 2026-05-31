// providers/session-provider.tsx
'use client'

import { useEffect } from 'react'
import { useSession } from '@/stores/session'

export function SessionProvider({ children }: { children: React.ReactNode }) {
  const fetchSession = useSession((s) => s.fetchSession)
  const initialized = useSession((s) => s.initialized)

  useEffect(() => {
    if (!initialized) {
      fetchSession()
    }
  }, [initialized, fetchSession])

  return <>{children}</>
}
