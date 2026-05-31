// components/layout/sidebar.tsx
'use client'

import { useState } from 'react'
import { usePathname } from 'next/navigation'
import Link from 'next/link'
import { Grid, Notebook, FileText, Settings, MessageCircle, Expand, PanelLeftClose } from 'lucide-react'
import { useSession } from '@/stores/session'
import { cn } from '@/lib/utils'

const navItems = [
  { key: 'classes', label: '班级', path: '/classes', icon: Grid },
  { key: 'lectures', label: '讲义', path: '/lectures', icon: Notebook },
  { key: 'assignments', label: '作业', path: '/assignments', icon: FileText },
  { key: 'agui', label: 'AI 助手', path: '/agui', icon: MessageCircle },
  { key: 'admin', label: '管理', path: '/admin/users', icon: Settings, adminOnly: true },
]

export function Sidebar() {
  const pathname = usePathname()
  const isAdmin = useSession((s) => s.isAdmin)
  const [collapsed, setCollapsed] = useState(false)

  const activeNav = (() => {
    if (pathname.startsWith('/admin')) return 'admin'
    if (pathname.includes('/lectures')) return 'lectures'
    if (pathname.includes('/assignments')) return 'assignments'
    if (pathname.startsWith('/agui')) return 'agui'
    return 'classes'
  })()

  return (
    <aside className="sidebar">
      <nav>
        {navItems.map((item) => {
          if (item.adminOnly && !isAdmin()) return null
          const Icon = item.icon
          const isActive = activeNav === item.key
          return (
            <Link key={item.key} href={item.path}
              className={cn('nav-item', isActive && 'active')}>
              <Icon size={18} />
              <span>{item.label}</span>
            </Link>
          )
        })}
      </nav>
      <button className="collapse-button"
        title={collapsed ? '展开侧栏' : '收起侧栏'}
        onClick={() => setCollapsed(!collapsed)}>
        {collapsed ? <Expand size={16} /> : <PanelLeftClose size={16} />}
        <span>{collapsed ? '展开' : '收起'}</span>
      </button>
    </aside>
  )
}
