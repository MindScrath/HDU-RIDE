// app/layout.tsx
import type { Metadata } from 'next'
import './globals.css'
import { SessionProvider } from '@/providers/session-provider'
import { Toaster } from '@/components/ui/sonner'

export const metadata: Metadata = {
  title: 'HDU RIDE',
  description: '金融计量分析教学平台',
}

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="zh-CN">
      <body>
        <SessionProvider>
          {children}
          <Toaster position="top-center" richColors />
        </SessionProvider>
      </body>
    </html>
  )
}
