// app/(authenticated)/layout.tsx
import { SidebarProvider } from '@/components/layout/sidebar-context'
import { AuthenticatedShell } from '@/components/layout/authenticated-shell'

export default function AuthenticatedLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <SidebarProvider>
      <AuthenticatedShell>{children}</AuthenticatedShell>
    </SidebarProvider>
  )
}
