// components/layout/authenticated-shell.tsx
'use client'

import { Topbar } from './topbar'
import { Sidebar } from './sidebar'
import { useSidebar } from './sidebar-context'

export function AuthenticatedShell({ children }: { children: React.ReactNode }) {
  const { collapsed } = useSidebar()

  return (
    <div className={`app-shell ${collapsed ? 'is-collapsed' : ''}`}>
      <Topbar />
      <div className="app-body">
        <Sidebar />
        <main className="workspace">{children}</main>
      </div>
    </div>
  )
}
