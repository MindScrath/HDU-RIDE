// app/(authenticated)/layout.tsx
import { SidebarProvider } from '@/components/layout/sidebar-context'
import { AuthenticatedShell } from '@/components/layout/authenticated-shell'
import { ConfirmProvider } from '@/components/ui/confirm-dialog'

export default function AuthenticatedLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <ConfirmProvider>
      <SidebarProvider>
        <AuthenticatedShell>{children}</AuthenticatedShell>
      </SidebarProvider>
    </ConfirmProvider>
  )
}
