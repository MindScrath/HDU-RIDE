# React Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Migrate the entire HDU RIDE frontend from Vue 3 + Element Plus to Next.js + Shadcn/ui + CopilotKit AG-UI.

**Architecture:** Next.js App Router as frontend shell with Middleware auth guard. Go backend APIs kept intact. Next.js API Routes serve as BFF for CopilotKit AG-UI, proxying to Bailian (百炼). Zustand for client state. react-markdown + remark-math + rehype-katex for all Markdown/LaTeX rendering.

**Tech Stack:** Next.js 15, React 19, TypeScript, Shadcn/ui, Tailwind CSS, Zustand, react-markdown, remark-math, rehype-katex, KaTeX, CopilotKit, Bun

---

### Task 1: Project Initialization & Dependencies

**Files:**
- Create: `frontend-react/` (entire Next.js project)

- [ ] **Step 1: Scaffold Next.js project with Bun**

```bash
cd /home/apollo/Desktop/HDU-RIDE
bun create next-app@latest frontend-react --typescript --tailwind --eslint --app --no-src-dir --import-alias="@/*" --turbopack
cd frontend-react
```

- [ ] **Step 2: Initialize Shadcn/ui**

```bash
npx shadcn@latest init -d --style default --base-color blue
```

- [ ] **Step 3: Install all required dependencies**

```bash
# Core
bun add zustand react-markdown remark-math remark-gfm rehype-katex katex dayjs

# Shadcn/ui components (add all needed)
npx shadcn@latest add button input select dialog dropdown-menu table form badge skeleton toast textarea

# CopilotKit
bun add @copilotkit/react-core @copilotkit/react-ui

# Dev
bun add -d @types/katex
```

- [ ] **Step 4: Verify project runs**

```bash
bun run dev
```

Visit http://localhost:3000 — you should see the default Next.js welcome page.

- [ ] **Step 5: Commit**

```bash
git add frontend-react/
git commit -m "feat: scaffold Next.js + Shadcn/ui project"
```

---

### Task 2: Base Infrastructure — Types, API, Utils

**Files:**
- Create: `frontend-react/lib/types.ts`
- Create: `frontend-react/lib/api.ts`
- Create: `frontend-react/lib/utils.ts`

- [ ] **Step 1: Write `lib/types.ts` — Port and extend type definitions from Vue**

```typescript
// lib/types.ts
export type Role = 'root' | 'admin' | 'teacher' | 'assistant' | 'student'

export interface User {
  id: string
  username: string
  displayName: string
  role: Role
  status: string
  createdAt?: string
}

export interface ClassItem {
  id: string
  courseId: string
  name: string
  term: string
  note: string
  createdBy: string
}

export interface Lecture {
  id: string
  file: string
  title: string
  order: number
}

export interface LectureChapter {
  id: string
  title: string
  order: number
  sections: Lecture[]
}

export interface Assignment {
  id: string
  title: string
  openAt: string
  dueAt: string
  rstudioImage: string
  starter: string
  submitPath: string
}

export interface Submission {
  id: string
  classId: string
  assignmentId: string
  userId: string
  textObject: string
  fileObject: string
  attempt: number
  late: boolean
  createdAt: string
}

export interface SubmissionRow {
  submission: Submission
  studentName: string
  grade: {
    id: string | null
    score: number | null
    comment: string
    publishedAt?: string | null
  }
}

export interface MemberRow {
  user: User
  memberRole: 'student' | 'assistant'
  joinedAt: string
}
```

- [ ] **Step 2: Write `lib/api.ts` — Port fetch wrapper**

```typescript
// lib/api.ts
export class ApiError extends Error {
  constructor(
    message: string,
    public status: number
  ) {
    super(message)
  }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  // Use env variable for Go backend, fallback to same-origin (works via Next.js proxy or direct)
  const baseUrl = process.env.NEXT_PUBLIC_GO_API_URL ?? ''
  const response = await fetch(`${baseUrl}${path}`, {
    credentials: 'include',
    headers:
      init.body instanceof FormData
        ? init.headers
        : { 'Content-Type': 'application/json', ...init.headers },
    ...init,
  })
  const data = await response.json().catch(() => ({}))
  if (!response.ok) {
    throw new ApiError(data.error ?? 'request failed', response.status)
  }
  return data as T
}

export const api = {
  get: <T>(path: string) => request<T>(path),
  post: <T>(path: string, body?: unknown) =>
    request<T>(path, {
      method: 'POST',
      body: body instanceof FormData ? body : JSON.stringify(body ?? {}),
    }),
  patch: <T>(path: string, body: unknown) =>
    request<T>(path, { method: 'PATCH', body: JSON.stringify(body) }),
  delete: <T>(path: string) => request<T>(path, { method: 'DELETE' }),
  async download(path: string) {
    const baseUrl = process.env.NEXT_PUBLIC_GO_API_URL ?? ''
    const response = await fetch(`${baseUrl}${path}`, { credentials: 'include' })
    if (!response.ok) {
      const data = await response.json().catch(() => ({}))
      throw new ApiError(data.error ?? 'download failed', response.status)
    }
    return response.blob()
  },
}
```

- [ ] **Step 3: Write `lib/utils.ts` — Shadcn/ui cn helper**

```typescript
// lib/utils.ts
import { type ClassValue, clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function generateId(): string {
  if (typeof crypto !== 'undefined' && crypto.randomUUID) return crypto.randomUUID()
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0
    return (c === 'x' ? r : (r & 0x3) | 0x8).toString(16)
  })
}
```

```bash
bun add clsx tailwind-merge
```

- [ ] **Step 4: Commit**

```bash
git add frontend-react/lib/
git commit -m "feat: add types, API layer, and utils"
```

---

### Task 3: Zustand Session Store & Middleware

**Files:**
- Create: `frontend-react/stores/session.ts`
- Create: `frontend-react/middleware.ts`
- Create: `frontend-react/.env.local`

- [ ] **Step 1: Write `stores/session.ts`**

```typescript
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
```

- [ ] **Step 2: Write `middleware.ts`**

```typescript
// middleware.ts
import { NextResponse } from 'next/server'
import type { NextRequest } from 'next/server'

export function middleware(request: NextRequest) {
  const sessionToken = request.cookies.get('session_token')?.value
  const { pathname } = request.nextUrl

  // Allow API routes to pass through (handled by Go backend or Next.js API routes)
  if (pathname.startsWith('/api/')) {
    return NextResponse.next()
  }

  // Not logged in → redirect to /login (except /login itself)
  if (!sessionToken && pathname !== '/login') {
    return NextResponse.redirect(new URL('/login', request.url))
  }

  // Logged in and on /login → redirect to /classes
  if (sessionToken && pathname === '/login') {
    return NextResponse.redirect(new URL('/classes', request.url))
  }

  return NextResponse.next()
}

export const config = {
  matcher: ['/((?!_next/static|_next/image|favicon.ico).*)'],
}
```

- [ ] **Step 3: Write `.env.local`**

```env
# .env.local
BAILIAN_API_KEY=your_api_key_here
BAILIAN_APP_ID=your_app_id_here
NEXT_PUBLIC_GO_API_URL=http://localhost:8080
```

- [ ] **Step 4: Commit**

```bash
git add frontend-react/stores/session.ts frontend-react/middleware.ts frontend-react/.env.local
git commit -m "feat: add session store, auth middleware, and env config"
```

---

### Task 4: App Layout (RootLayout + Global Styles)

**Files:**
- Modify: `frontend-react/app/globals.css` (replace default Tailwind with custom styles)
- Modify: `frontend-react/app/layout.tsx` (root layout with sidebar + topbar)
- Create: `frontend-react/components/layout/sidebar.tsx`
- Create: `frontend-react/components/layout/topbar.tsx`
- Create: `frontend-react/providers/session-provider.tsx`

- [ ] **Step 1: Replace `app/globals.css` — Port all CSS from Vue `style.css`**

```css
/* app/globals.css */
@tailwind base;
@tailwind components;
@tailwind utilities;

@layer base {
  :root {
    color: #1f2937;
    background: #f7f9fc;
    font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont,
      "Segoe UI", "Microsoft YaHei", sans-serif;
    font-size: 14px;
  }

  * {
    box-sizing: border-box;
  }

  body {
    margin: 0;
    min-width: 960px;
  }

  button,
  input,
  textarea {
    font: inherit;
  }
}

/* ── Shell Layout ────────────────────────────── */
.app-shell {
  display: grid;
  grid-template-rows: 64px minmax(0, 1fr);
  min-height: 100vh;
  background: #f7f9fc;
}

.app-body {
  display: grid;
  grid-template-columns: 156px minmax(0, 1fr);
  min-height: 0;
  transition: grid-template-columns 0.18s ease;
}

.app-shell.is-collapsed .app-body {
  grid-template-columns: 64px minmax(0, 1fr);
}

/* ── Topbar ──────────────────────────────────── */
.global-topbar {
  display: grid;
  grid-template-columns: 220px minmax(0, 1fr);
  align-items: center;
  gap: 18px;
  height: 64px;
  padding: 0 18px;
  background: #fff;
  border-bottom: 1px solid #dfe5ee;
}

.topbar-brand {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}

.brand-mark {
  display: grid;
  flex: 0 0 auto;
  width: 34px;
  height: 34px;
  place-items: center;
  border-radius: 6px;
  background: #0b5ed7;
  color: #fff;
  font-weight: 780;
}

.brand-copy strong {
  display: block;
  color: #111827;
  font-family: Georgia, "Times New Roman", serif;
  font-size: 24px;
  line-height: 1;
  letter-spacing: 0;
}

.topbar-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 12px;
  min-width: 0;
}

.user-button {
  display: inline-flex;
  gap: 8px;
  align-items: center;
  border: 1px solid #d7dfeb;
  background: #fff;
  color: #273244;
  padding: 7px 10px;
  border-radius: 6px;
  cursor: pointer;
}

/* ── Sidebar ──────────────────────────────────── */
.sidebar {
  display: flex;
  flex-direction: column;
  background: #fff;
  color: #273244;
  border-right: 1px solid #dfe5ee;
  padding: 14px 10px;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 9px;
  height: 38px;
  padding: 0 10px;
  margin-bottom: 4px;
  color: #4b5565;
  text-decoration: none;
  border-radius: 6px;
  font-weight: 560;
  white-space: nowrap;
}

.nav-item.active,
.nav-item:hover {
  background: #eaf2ff;
  color: #0b5ed7;
}

.collapse-button {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  height: 38px;
  margin-top: auto;
  border: 0;
  border-top: 1px solid #edf1f6;
  background: transparent;
  color: #6b7280;
  cursor: pointer;
}

.collapse-button:hover {
  color: #0b5ed7;
}

/* ── Collapsed Sidebar ────────────────────────── */
.app-shell.is-collapsed .sidebar {
  align-items: center;
  padding-inline: 8px;
}

.app-shell.is-collapsed .sidebar .brand-copy,
.app-shell.is-collapsed .sidebar .nav-item span,
.app-shell.is-collapsed .sidebar .collapse-button span {
  display: none;
}

.app-shell.is-collapsed .nav-item,
.app-shell.is-collapsed .collapse-button {
  justify-content: center;
  width: 42px;
  padding: 0;
}

/* ── Workspace ────────────────────────────────── */
.workspace {
  min-width: 0;
  padding: 12px 18px 18px;
}

/* ── Panel ────────────────────────────────────── */
.panel {
  background: #fff;
  border: 1px solid #dde4ee;
  border-radius: 6px;
  box-shadow: 0 8px 22px rgba(30, 41, 59, 0.04);
  min-width: 0;
}

.single-panel {
  width: min(100%, 1180px);
  margin: 0 auto;
}

.page-grid {
  display: grid;
  grid-template-columns: 260px minmax(0, 1fr);
  gap: 12px;
  height: calc(100vh - 94px);
}

.panel-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  min-height: 48px;
  padding: 0 14px;
  border-bottom: 1px solid #e7edf5;
}

.toolbar-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
}

.panel-head h2 {
  font-size: 17px;
  margin: 0;
}

.panel-head h3 {
  font-size: 14px;
  margin: 0;
}

.scroll {
  overflow: auto;
}

.list-item {
  display: block;
  padding: 10px 12px;
  border-bottom: 1px solid #edf1f7;
  color: #202a3a;
  text-decoration: none;
}

.list-item.active,
.list-item:hover {
  background: #eef6ff;
}

.muted {
  color: #687386;
}

/* ── Markdown ──────────────────────────────────── */
.markdown {
  padding: 18px 22px 30px;
  color: #1f2937;
  line-height: 1.68;
}

.markdown h1 {
  font-size: 22px;
  line-height: 1.3;
  margin: 0 0 14px;
}

.markdown h2 {
  font-size: 17px;
  margin-top: 26px;
}

.markdown pre {
  overflow: auto;
  padding: 14px;
  border-radius: 8px;
  background: #0f172a;
  color: #edf2ff;
}

.markdown code {
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", monospace;
}

/* ── Splitter ──────────────────────────────────── */
.splitter {
  position: relative;
  cursor: col-resize;
  touch-action: none;
}

.splitter::before {
  position: absolute;
  top: 10px;
  bottom: 10px;
  left: 50%;
  width: 1px;
  content: "";
  background: #d8e0eb;
}

.splitter:hover::before {
  width: 2px;
  background: #0b5ed7;
}

/* ── Assignment Grid ────────────────────────────── */
.assignment-grid {
  display: grid;
  grid-template-columns:
    minmax(170px, var(--assignment-list-width, 205px))
    10px
    minmax(360px, 1fr)
    10px
    minmax(280px, var(--assignment-ide-width, 360px));
  height: calc(100vh - 94px);
  min-width: 0;
  overflow-x: auto;
}

.workspace-panel {
  overflow: hidden;
}

.workspace-panel.fullscreen {
  position: fixed;
  inset: 0;
  z-index: 1000;
  border-radius: 0;
}

.ide-frame {
  width: 100%;
  height: calc(100% - 48px);
  border: 0;
  background: #fff;
}

/* ── Login ──────────────────────────────────────── */
.login-page {
  display: grid;
  min-height: 100vh;
  place-items: center;
  background: linear-gradient(135deg, #e8f1fb 0%, #f8fbff 58%, #edf6f4 100%);
}

.login-box {
  width: 380px;
  padding: 28px;
}

/* ── Chapter Tree ────────────────────────────────── */
.lecture-tree {
  padding-bottom: 12px;
}

.chapter-block {
  padding: 10px 10px 0;
}

.chapter-title {
  margin: 4px 0 6px;
  color: #202a3a;
  font-size: 13px;
  font-weight: 700;
}

.section-item {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  gap: 8px;
  min-height: 34px;
  padding: 0 10px;
  color: #344054;
  text-decoration: none;
  border-radius: 4px;
}

.section-item span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.section-item.active,
.section-item:hover {
  background: #eaf2ff;
  color: #0b5ed7;
}

/* ── Prompt Block ────────────────────────────────── */
.prompt-block {
  border-bottom: 1px solid #edf1f7;
}

.prompt-block.collapsed {
  background: #fbfcfe;
}

.prompt-toggle {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  height: 40px;
  padding: 0 22px;
  border: 0;
  background: transparent;
  color: #1f2937;
  cursor: pointer;
}

.prompt-toggle span {
  color: #0b5ed7;
  font-size: 12px;
}

/* ── Grading ────────────────────────────────────── */
.grading-overview {
  padding: 14px 18px 22px;
}

.grading-toolbar {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 10px;
}

.metric-row {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  border: 1px solid #e3e9f2;
  border-radius: 6px;
  overflow: hidden;
}

.metric-row div {
  padding: 8px 12px;
  border-right: 1px solid #e3e9f2;
}

.metric-row div:last-child {
  border-right: 0;
}

.metric-row strong {
  display: block;
  color: #0b5ed7;
  font-size: 17px;
  line-height: 1.1;
}

.metric-row span {
  display: block;
  margin-top: 3px;
  color: #667085;
  font-size: 11px;
}

/* ── Student Submit ─────────────────────────────── */
.student-submit {
  padding: 16px 22px 24px;
}

.submit-actions {
  display: flex;
  justify-content: flex-end;
  padding-top: 16px;
}

/* ── Submission Preview ──────────────────────────── */
.submission-preview {
  height: calc(100% - 48px);
  overflow: auto;
  padding: 14px;
}

.preview-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
  color: #4b5565;
}

/* ── Member Layout ───────────────────────────────── */
.member-layout {
  display: grid;
  grid-template-columns: 320px minmax(0, 1fr);
  gap: 18px;
  padding: 18px;
}

.member-import {
  display: grid;
  align-content: start;
  gap: 12px;
}

.member-import h3 {
  margin: 0;
  font-size: 14px;
}

/* ── Context Select ──────────────────────────────── */
.context-select {
  width: min(260px, 42vw);
}
```

- [ ] **Step 2: Write `providers/session-provider.tsx`**

```tsx
// providers/session-provider.tsx
'use client'

import { useEffect } from 'react'
import { useSession } from '@/stores/session'

export function SessionProvider({ children }: { children: React.ReactNode }) {
  const { fetchSession, initialized } = useSession()

  useEffect(() => {
    if (!initialized) {
      fetchSession()
    }
  }, [initialized, fetchSession])

  return <>{children}</>
}
```

- [ ] **Step 3: Write `components/layout/topbar.tsx`**

```tsx
// components/layout/topbar.tsx
'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import { User } from 'lucide-react'
import { useSession } from '@/stores/session'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { toast } from 'sonner'

export function Topbar() {
  const { user, logout, changePassword } = useSession()
  const router = useRouter()
  const [passwordOpen, setPasswordOpen] = useState(false)
  const [passwordSaving, setPasswordSaving] = useState(false)
  const [oldPassword, setOldPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')

  async function handleLogout() {
    await logout()
    router.push('/login')
  }

  async function handleSavePassword() {
    if (newPassword !== confirmPassword) {
      toast.error('两次输入的新密码不一致')
      return
    }
    setPasswordSaving(true)
    try {
      await changePassword(oldPassword, newPassword)
      toast.success('密码已修改')
      setPasswordOpen(false)
      setOldPassword('')
      setNewPassword('')
      setConfirmPassword('')
    } finally {
      setPasswordSaving(false)
    }
  }

  return (
    <>
      <header className="global-topbar">
        <div className="topbar-brand">
          <span className="brand-mark">R</span>
          <div className="brand-copy">
            <strong>HDU RIDE</strong>
          </div>
        </div>
        <div className="topbar-actions">
          {user && (
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <button className="user-button">
                  <User size={16} />
                  {user.displayName}
                </button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem disabled>{user.role}</DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem onClick={() => setPasswordOpen(true)}>
                  修改密码
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem onClick={handleLogout}>退出</DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          )}
        </div>
      </header>

      <Dialog open={passwordOpen} onOpenChange={setPasswordOpen}>
        <DialogContent className="sm:max-w-[420px]">
          <DialogHeader>
            <DialogTitle>修改密码</DialogTitle>
          </DialogHeader>
          <div className="grid gap-3">
            <div>
              <Label>当前密码</Label>
              <Input
                type="password"
                autoComplete="current-password"
                value={oldPassword}
                onChange={(e) => setOldPassword(e.target.value)}
              />
            </div>
            <div>
              <Label>新密码</Label>
              <Input
                type="password"
                autoComplete="new-password"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
              />
            </div>
            <div>
              <Label>确认新密码</Label>
              <Input
                type="password"
                autoComplete="new-password"
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setPasswordOpen(false)}>
              取消
            </Button>
            <Button onClick={handleSavePassword} disabled={passwordSaving}>
              保存
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
```

- [ ] **Step 4: Write `components/layout/sidebar.tsx`**

```tsx
// components/layout/sidebar.tsx
'use client'

import { useState } from 'react'
import { usePathname } from 'next/navigation'
import Link from 'next/link'
import {
  Grid,
  Notebook,
  FileText,
  Settings,
  MessageCircle,
  Expand,
  Fold,
} from 'lucide-react'
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
  const { isAdmin } = useSession()
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
            <Link
              key={item.key}
              href={item.path}
              className={cn('nav-item', isActive && 'active')}
            >
              <Icon size={18} />
              <span>{item.label}</span>
            </Link>
          )
        })}
      </nav>
      <button
        className="collapse-button"
        title={collapsed ? '展开侧栏' : '收起侧栏'}
        onClick={() => setCollapsed(!collapsed)}
      >
        {collapsed ? <Expand size={16} /> : <Fold size={16} />}
        <span>{collapsed ? '展开' : '收起'}</span>
      </button>
    </aside>
  )
}
```

Note: The sidebar component receives `collapsed` state via a context or prop. We'll wire this up in the layout via React context. For now, manage `collapsed` internally and expose via a small context.

- [ ] **Step 5: Rewrite `app/layout.tsx`**

```tsx
// app/layout.tsx
import type { Metadata } from 'next'
import './globals.css'
import { SessionProvider } from '@/providers/session-provider'
import { Toaster } from '@/components/ui/sonner'

export const metadata: Metadata = {
  title: 'HDU RIDE',
  description: '金融计量分析教学平台',
}

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
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
```

Note: We need to add sonner. Run `npx shadcn@latest add sonner`.

- [ ] **Step 6: Commit**

```bash
git add frontend-react/app/globals.css frontend-react/app/layout.tsx frontend-react/components/layout/ frontend-react/providers/
git commit -m "feat: add root layout, sidebar, topbar, and session provider"
```

---

### Task 5: Markdown Renderer Component

**Files:**
- Create: `frontend-react/components/markdown/MarkdownRenderer.tsx`

- [ ] **Step 1: Install KaTeX and add to tailwind config**

```bash
bun add katex
bun add -d @types/katex
```

Add to `app/globals.css`:
```css
@import 'katex/dist/katex.min.css';
```

- [ ] **Step 2: Write `components/markdown/MarkdownRenderer.tsx`**

```tsx
// components/markdown/MarkdownRenderer.tsx
'use client'

import ReactMarkdown from 'react-markdown'
import remarkMath from 'remark-math'
import remarkGfm from 'remark-gfm'
import rehypeKatex from 'rehype-katex'
import type { Components } from 'react-markdown'

function CodeBlock({ className, children, ...props }: React.HTMLAttributes<HTMLElement>) {
  const match = /language-(\w+)/.exec(className || '')
  const language = match ? match[1] : ''
  return (
    <pre className="overflow-auto p-4 rounded-lg bg-slate-900 text-slate-100">
      {language && (
        <div className="text-xs text-slate-400 mb-1">{language}</div>
      )}
      <code className={className} {...props}>
        {children}
      </code>
    </pre>
  )
}

const components: Components = {
  code({ className, children, ...props }) {
    // Inline code (no language- prefix in className from react-markdown)
    const isInline = !className?.includes('language-')
    if (isInline) {
      return (
        <code
          className="bg-slate-100 text-rose-600 rounded px-1 py-0.5 font-mono text-[0.9em]"
          {...props}
        >
          {children}
        </code>
      )
    }
    return <CodeBlock className={className} {...props}>{children}</CodeBlock>
  },
}

export function MarkdownRenderer({ content }: { content: string }) {
  if (!content) return null
  return (
    <ReactMarkdown
      remarkPlugins={[remarkGfm, remarkMath]}
      rehypePlugins={[rehypeKatex]}
      components={components}
    >
      {content}
    </ReactMarkdown>
  )
}
```

- [ ] **Step 3: Commit**

```bash
git add frontend-react/components/markdown/
git commit -m "feat: add MarkdownRenderer with LaTeX support"
```

---

### Task 6: Login Page

**Files:**
- Create: `frontend-react/app/login/page.tsx`

- [ ] **Step 1: Write `app/login/page.tsx`**

```tsx
// app/login/page.tsx
'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useSession } from '@/stores/session'
import { toast } from 'sonner'

export default function LoginPage() {
  const { login } = useSession()
  const router = useRouter()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [loading, setLoading] = useState(false)

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setLoading(true)
    try {
      await login(username, password)
      router.push('/classes')
    } catch (err) {
      toast.error(err instanceof Error ? err.message : '登录失败')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="login-page">
      <section className="panel login-box">
        <div className="flex items-center gap-3 mb-5" style={{ color: '#172033' }}>
          <span className="brand-mark">R</span>
          <div>
            <strong className="block text-[#101827] font-serif text-lg leading-tight">
              HDU RIDE
            </strong>
            <small className="text-[#6c7787] text-[11px]">
              金融计量分析教学平台
            </small>
          </div>
        </div>
        <form onSubmit={handleSubmit} className="grid gap-3">
          <div>
            <Label>账号</Label>
            <Input
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              autoComplete="username"
            />
          </div>
          <div>
            <Label>密码</Label>
            <Input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete="current-password"
            />
          </div>
          <Button type="submit" className="w-full" disabled={loading}>
            登录
          </Button>
        </form>
      </section>
    </div>
  )
}
```

- [ ] **Step 2: Verify login page renders at /login**

`bun run dev` and visit http://localhost:3000/login

- [ ] **Step 3: Commit**

```bash
git add frontend-react/app/login/
git commit -m "feat: add login page"
```

---

### Task 7: Authenticated Layout Wrapper

**Files:**
- Create: `frontend-react/app/(authenticated)/layout.tsx`
- Move/Create: `frontend-react/app/(authenticated)/page.tsx` (redirect)
- Create: `frontend-react/components/layout/sidebar-context.tsx`

We use a route group `(authenticated)` so that the sidebar+topbar shell only wraps authenticated pages. Login stands alone.

- [ ] **Step 1: Write `components/layout/sidebar-context.tsx`**

```tsx
// components/layout/sidebar-context.tsx
'use client'

import { createContext, useContext, useState, type ReactNode } from 'react'

const SidebarContext = createContext<{
  collapsed: boolean
  setCollapsed: (v: boolean) => void
}>({ collapsed: false, setCollapsed: () => {} })

export function SidebarProvider({ children }: { children: ReactNode }) {
  const [collapsed, setCollapsed] = useState(false)
  return (
    <SidebarContext.Provider value={{ collapsed, setCollapsed }}>
      {children}
    </SidebarContext.Provider>
  )
}

export function useSidebar() {
  return useContext(SidebarContext)
}
```

- [ ] **Step 2: Update `components/layout/sidebar.tsx` — use `useSidebar`**

Replace the internal `collapsed` state with:
```tsx
import { useSidebar } from './sidebar-context'

// inside component:
const { collapsed, setCollapsed } = useSidebar()
```

- [ ] **Step 3: Write `app/(authenticated)/layout.tsx`**

```tsx
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
```

- [ ] **Step 4: Write `components/layout/authenticated-shell.tsx`**

```tsx
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
```

- [ ] **Step 5: Write `app/(authenticated)/page.tsx` — redirect to /classes**

```tsx
// app/(authenticated)/page.tsx
import { redirect } from 'next/navigation'

export default function Home() {
  redirect('/classes')
}
```

- [ ] **Step 6: Verify shell renders for authenticated routes**

Visit `/classes` (should show empty main area with sidebar + topbar).

- [ ] **Step 7: Commit**

```bash
git add frontend-react/app/\(authenticated\)/ frontend-react/components/layout/
git commit -m "feat: add authenticated layout shell with route group"
```

---

### Task 8: Classes Page (Class List)

**Files:**
- Create: `frontend-react/app/(authenticated)/classes/page.tsx`

- [ ] **Step 1: Write `app/(authenticated)/classes/page.tsx`**

```tsx
// app/(authenticated)/classes/page.tsx
'use client'

import { useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Checkbox } from '@/components/ui/checkbox'
import { api, ApiError } from '@/lib/api'
import { useSession } from '@/stores/session'
import { toast } from 'sonner'
import type { ClassItem } from '@/lib/types'

export default function ClassesPage() {
  const { user } = useSession()
  const router = useRouter()
  const [classes, setClasses] = useState<ClassItem[]>([])
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [createOpen, setCreateOpen] = useState(false)
  const [form, setForm] = useState({ courseId: 'intro-r', name: '', term: '2026 春', note: '' })

  const canManage = ['root', 'admin', 'teacher'].includes(user?.role ?? '')

  async function load() {
    try {
      const data = await api.get<{ classes: ClassItem[] }>('/api/classes')
      setClasses(data.classes)
    } catch (err) {
      if ((err as ApiError).status === 401) router.push('/login')
    }
  }

  useEffect(() => { load() }, [])

  async function handleCreate() {
    await api.post('/api/classes', form)
    toast.success('班级已创建')
    setCreateOpen(false)
    setForm({ courseId: 'intro-r', name: '', term: '2026 春', note: '' })
    await load()
  }

  async function handleDelete(ids: string[]) {
    if (!ids.length) return
    if (!confirm(`确定删除 ${ids.length} 个班级？关联成员、提交和成绩会一并删除。`)) return
    await api.post('/api/classes/bulk', { action: 'delete', ids })
    toast.success('班级已删除')
    setSelectedIds(new Set())
    await load()
  }

  function toggleSelect(id: string) {
    const next = new Set(selectedIds)
    next.has(id) ? next.delete(id) : next.add(id)
    setSelectedIds(next)
  }

  return (
    <>
      <section className="panel single-panel">
        <div className="panel-head">
          <div>
            <h2>班级</h2>
            <span className="muted">班级成员从这里进入，讲义和作业也可从左侧直接打开</span>
          </div>
          <div className="toolbar-actions">
            {canManage && selectedIds.size > 0 && (
              <Button variant="destructive" onClick={() => handleDelete(Array.from(selectedIds))}>
                删除选中
              </Button>
            )}
            {canManage && (
              <Button onClick={() => setCreateOpen(true)}>新建班级</Button>
            )}
          </div>
        </div>
        <Table>
          <TableHeader>
            <TableRow>
              {canManage && <TableHead className="w-[44px]" />}
              <TableHead>班级</TableHead>
              <TableHead className="w-[150px]">课程 ID</TableHead>
              <TableHead className="w-[130px]">学期</TableHead>
              <TableHead>备注</TableHead>
              <TableHead className="w-[320px]">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {classes.map((klass) => (
              <TableRow key={klass.id}>
                {canManage && (
                  <TableCell>
                    <Checkbox
                      checked={selectedIds.has(klass.id)}
                      onCheckedChange={() => toggleSelect(klass.id)}
                    />
                  </TableCell>
                )}
                <TableCell>{klass.name}</TableCell>
                <TableCell>{klass.courseId}</TableCell>
                <TableCell>{klass.term}</TableCell>
                <TableCell>{klass.note}</TableCell>
                <TableCell>
                  <div className="flex gap-2">
                    <Button variant="outline" size="sm" onClick={() => router.push(`/classes/${klass.id}/lectures`)}>讲义</Button>
                    <Button size="sm" onClick={() => router.push(`/classes/${klass.id}/assignments`)}>作业</Button>
                    {(canManage || useSession.getState().canTeach()) && (
                      <Button variant="outline" size="sm" onClick={() => router.push(`/classes/${klass.id}/members`)}>成员</Button>
                    )}
                    {canManage && (
                      <Button variant="destructive" size="sm" onClick={() => handleDelete([klass.id])}>删除</Button>
                    )}
                  </div>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </section>

      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent className="sm:max-w-[420px]">
          <DialogHeader>
            <DialogTitle>新建班级</DialogTitle>
          </DialogHeader>
          <div className="grid gap-3">
            <div><Label>课程 ID</Label><Input value={form.courseId} onChange={(e) => setForm({ ...form, courseId: e.target.value })} /></div>
            <div><Label>班级名称</Label><Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} /></div>
            <div><Label>学期</Label><Input value={form.term} onChange={(e) => setForm({ ...form, term: e.target.value })} /></div>
            <div><Label>备注</Label><Input value={form.note} onChange={(e) => setForm({ ...form, note: e.target.value })} /></div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateOpen(false)}>取消</Button>
            <Button onClick={handleCreate}>创建</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
```

- [ ] **Step 2: Add Checkbox Shadcn component**

```bash
npx shadcn@latest add checkbox
```

- [ ] **Step 3: Verify classes page works**

Visit `/classes` — should load class list from Go backend (if running).

- [ ] **Step 4: Commit**

```bash
git add frontend-react/app/\(authenticated\)/classes/
git commit -m "feat: add classes page"
```

---

### Task 9: Class Members Page

**Files:**
- Create: `frontend-react/app/(authenticated)/classes/[classId]/members/page.tsx`

- [ ] **Step 1: Write `app/(authenticated)/classes/[classId]/members/page.tsx`**

```tsx
// app/(authenticated)/classes/[classId]/members/page.tsx
'use client'

import { useEffect, useState } from 'react'
import { useParams, useRouter } from 'next/navigation'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Checkbox } from '@/components/ui/checkbox'
import { api } from '@/lib/api'
import { useSession } from '@/stores/session'
import { toast } from 'sonner'
import type { ClassItem, MemberRow } from '@/lib/types'

export default function ClassMembersPage() {
  const { classId } = useParams<{ classId: string }>()
  const { user } = useSession()
  const router = useRouter()
  const [klass, setKlass] = useState<ClassItem | null>(null)
  const [members, setMembers] = useState<MemberRow[]>([])
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [importText, setImportText] = useState('username,displayName,password\nstudent001,学生一,student123')
  const [passwordOpen, setPasswordOpen] = useState(false)
  const [passwordTarget, setPasswordTarget] = useState<MemberRow | null>(null)
  const [newPassword, setNewPassword] = useState('')

  const canManage = ['root', 'admin', 'teacher'].includes(user?.role ?? '')

  async function load() {
    const klassData = await api.get<{ class: ClassItem }>(`/api/classes/${classId}`)
    setKlass(klassData.class)
    const memData = await api.get<{ members: MemberRow[] }>(`/api/classes/${classId}/members`)
    setMembers(memData.members)
  }

  useEffect(() => { load() }, [])

  async function handleImport() {
    const students = importText
      .split(/\r?\n/)
      .slice(1)
      .map((line) => line.split(',').map((item) => item.trim()))
      .filter((row) => row[0] && row[1] && row[2])
      .map(([username, displayName, password]) => ({ username, displayName, password }))
    await api.post(`/api/classes/${classId}/members/import`, { students })
    toast.success('成员已导入')
    await load()
  }

  async function handleRemove(ids: string[]) {
    if (!ids.length) return
    if (!confirm(`确定移除 ${ids.length} 个班级成员？账号本身会保留。`)) return
    await api.post(`/api/classes/${classId}/members/bulk`, { action: 'remove', userIds: ids })
    toast.success('成员已移除')
    setSelectedIds(new Set())
    await load()
  }

  async function handleSetRole(ids: string[], memberRole: 'student' | 'assistant') {
    if (!ids.length) return
    await api.post(`/api/classes/${classId}/members/bulk`, { action: 'setMemberRole', userIds: ids, memberRole })
    toast.success(memberRole === 'assistant' ? '已设为助教' : '已设为学生')
    setSelectedIds(new Set())
    await load()
  }

  async function handleSavePassword() {
    if (!passwordTarget) return
    await api.post(`/api/classes/${classId}/members/${passwordTarget.user.id}/password`, { password: newPassword })
    toast.success('密码已重置')
    setPasswordOpen(false)
  }

  function toggleSelect(id: string) {
    const next = new Set(selectedIds)
    next.has(id) ? next.delete(id) : next.add(id)
    setSelectedIds(next)
  }

  return (
    <>
      <section className="panel single-panel">
        <div className="panel-head">
          <div>
            <h2>{klass?.name ?? '成员'}</h2>
            <span className="muted">学生与助教绑定在当前班级</span>
          </div>
          <div className="toolbar-actions">
            {canManage && selectedIds.size > 0 && (
              <>
                <Button variant="outline" onClick={() => handleSetRole(Array.from(selectedIds), 'assistant')}>设为助教</Button>
                <Button variant="outline" onClick={() => handleSetRole(Array.from(selectedIds), 'student')}>设为学生</Button>
                <Button variant="destructive" onClick={() => handleRemove(Array.from(selectedIds))}>移除选中</Button>
              </>
            )}
            <Button variant="outline" onClick={() => router.push('/classes')}>返回班级</Button>
          </div>
        </div>
        <div className="member-layout">
          {canManage && (
            <div className="member-import">
              <h3>导入学生</h3>
              <Textarea value={importText} onChange={(e) => setImportText(e.target.value)} rows={8} />
              <Button onClick={handleImport}>导入</Button>
            </div>
          )}
          <Table>
            <TableHeader>
              <TableRow>
                {canManage && <TableHead className="w-[44px]" />}
                <TableHead className="w-[160px]">账号</TableHead>
                <TableHead className="w-[160px]">姓名</TableHead>
                <TableHead className="w-[130px]">班级角色</TableHead>
                <TableHead className="w-[110px]">状态</TableHead>
                <TableHead>加入时间</TableHead>
                {canManage && <TableHead className="w-[260px]">操作</TableHead>}
              </TableRow>
            </TableHeader>
            <TableBody>
              {members.map((row) => (
                <TableRow key={row.user.id}>
                  {canManage && (
                    <TableCell>
                      <Checkbox
                        checked={selectedIds.has(row.user.id)}
                        onCheckedChange={() => toggleSelect(row.user.id)}
                      />
                    </TableCell>
                  )}
                  <TableCell>{row.user.username}</TableCell>
                  <TableCell>{row.user.displayName}</TableCell>
                  <TableCell>{row.memberRole}</TableCell>
                  <TableCell>{row.user.status}</TableCell>
                  <TableCell>{row.joinedAt}</TableCell>
                  {canManage && (
                    <TableCell>
                      <div className="flex gap-1">
                        <Button
                          variant="link"
                          size="sm"
                          onClick={() => handleSetRole([row.user.id], row.memberRole === 'assistant' ? 'student' : 'assistant')}
                        >
                          {row.memberRole === 'assistant' ? '设为学生' : '设为助教'}
                        </Button>
                        {row.memberRole === 'student' && (
                          <Button
                            variant="link"
                            size="sm"
                            onClick={() => { setPasswordTarget(row); setNewPassword(''); setPasswordOpen(true) }}
                          >
                            重置密码
                          </Button>
                        )}
                        <Button variant="link" size="sm" className="text-red-500" onClick={() => handleRemove([row.user.id])}>移除</Button>
                      </div>
                    </TableCell>
                  )}
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      </section>

      <Dialog open={passwordOpen} onOpenChange={setPasswordOpen}>
        <DialogContent className="sm:max-w-[420px]">
          <DialogHeader><DialogTitle>重置学生密码</DialogTitle></DialogHeader>
          <div className="grid gap-3">
            <div>
              <Label>学生</Label>
              <Input value={passwordTarget?.user.displayName ?? ''} disabled />
            </div>
            <div>
              <Label>新密码</Label>
              <Input type="password" value={newPassword} onChange={(e) => setNewPassword(e.target.value)} />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setPasswordOpen(false)}>取消</Button>
            <Button onClick={handleSavePassword}>保存</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
```

- [ ] **Step 2: Verify**

Visit `/classes/<id>/members` — should display member list.

- [ ] **Step 3: Commit**

```bash
git add frontend-react/app/\(authenticated\)/classes/\[classId\]/members/
git commit -m "feat: add class members page"
```

---

### Task 10: Admin Users Page

**Files:**
- Create: `frontend-react/app/(authenticated)/admin/users/page.tsx`

- [ ] **Step 1: Write `app/(authenticated)/admin/users/page.tsx`**

```tsx
// app/(authenticated)/admin/users/page.tsx
'use client'

import { useEffect, useState, useMemo } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Checkbox } from '@/components/ui/checkbox'
import { api } from '@/lib/api'
import { useSession } from '@/stores/session'
import { toast } from 'sonner'
import type { Role, User } from '@/lib/types'

export default function AdminUsersPage() {
  const { user: currentUser } = useSession()
  const [users, setUsers] = useState<User[]>([])
  const [selectedUsers, setSelectedUsers] = useState<User[]>([])
  const [createOpen, setCreateOpen] = useState(false)
  const [editOpen, setEditOpen] = useState(false)
  const [passwordOpen, setPasswordOpen] = useState(false)
  const [createForm, setCreateForm] = useState({ username: '', displayName: '', password: '', role: 'student' as Role })
  const [editForm, setEditForm] = useState({ id: '', displayName: '', role: 'student' as Role, status: 'active' })
  const [passwordForm, setPasswordForm] = useState({ id: '', displayName: '', password: '' })

  const roleOptions = useMemo(() => {
    const roles: Array<{ label: string; value: Role }> = [
      { label: 'Root', value: 'root' },
      { label: 'Admin', value: 'admin' },
      { label: 'Teacher', value: 'teacher' },
      { label: 'Assistant', value: 'assistant' },
      { label: 'Student', value: 'student' },
    ]
    return currentUser?.role === 'root' ? roles : roles.filter((r) => r.value !== 'root')
  }, [currentUser])

  function canManage(user: User) {
    if (currentUser?.role === 'root') return true
    return currentUser?.role === 'admin' && user.role !== 'root'
  }

  function canMutateIdentity(user: User) {
    return canManage(user) && user.id !== currentUser?.id
  }

  const manageableSelection = selectedUsers.filter(canManage)

  async function load() {
    const data = await api.get<{ users: User[] }>('/api/admin/users')
    setUsers(data.users)
  }

  useEffect(() => { load() }, [])

  async function handleCreate() {
    await api.post('/api/admin/users', createForm)
    toast.success('用户已创建')
    setCreateOpen(false)
    setCreateForm({ username: '', displayName: '', password: '', role: 'student' })
    await load()
  }

  async function handleEdit(user: User) {
    setEditForm({ id: user.id, displayName: user.displayName, role: user.role, status: user.status })
    setEditOpen(true)
  }

  async function handleSaveEdit() {
    await api.patch(`/api/admin/users/${editForm.id}`, {
      displayName: editForm.displayName,
      role: editForm.role,
      status: editForm.status,
    })
    toast.success('用户已更新')
    setEditOpen(false)
    await load()
  }

  async function handlePassword(user: User) {
    setPasswordForm({ id: user.id, displayName: user.displayName, password: '' })
    setPasswordOpen(true)
  }

  async function handleSavePassword() {
    await api.post(`/api/admin/users/${passwordForm.id}/password`, { password: passwordForm.password })
    toast.success('密码已重置')
    setPasswordOpen(false)
  }

  async function handleDisable(user: User) {
    if (!confirm(`确定禁用账号 ${user.username}？`)) return
    await api.delete(`/api/admin/users/${user.id}`)
    toast.success('账号已禁用')
    await load()
  }

  async function handleBulk(action: 'disable' | 'activate' | 'setRole', role?: Role) {
    const rows = manageableSelection.filter((u) => action === 'activate' || canMutateIdentity(u))
    if (!rows.length) return
    if (action === 'disable') {
      if (!confirm(`确定禁用 ${rows.length} 个账号？`)) return
    }
    await api.post('/api/admin/users/bulk', { action, ids: rows.map((u) => u.id), role })
    toast.success('批量操作已完成')
    setSelectedUsers([])
    await load()
  }

  function toggleSelect(user: User) {
    setSelectedUsers((prev) =>
      prev.includes(user) ? prev.filter((u) => u.id !== user.id) : [...prev, user]
    )
  }

  return (
    <>
      <section className="panel single-panel">
        <div className="panel-head">
          <h2>用户管理</h2>
          <div className="toolbar-actions">
            {manageableSelection.length > 0 && (
              <>
                <Button variant="outline" onClick={() => handleBulk('activate')}>启用选中</Button>
                <Button variant="destructive" onClick={() => handleBulk('disable')}>禁用选中</Button>
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button variant="outline">批量角色</Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent>
                    {roleOptions.map((role) => (
                      <DropdownMenuItem key={role.value} onClick={() => handleBulk('setRole', role.value)}>
                        {role.label}
                      </DropdownMenuItem>
                    ))}
                  </DropdownMenuContent>
                </DropdownMenu>
              </>
            )}
            <Button onClick={() => setCreateOpen(true)}>新建用户</Button>
          </div>
        </div>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-[44px]" />
              <TableHead className="w-[180px]">账号</TableHead>
              <TableHead className="w-[180px]">姓名</TableHead>
              <TableHead className="w-[130px]">角色</TableHead>
              <TableHead className="w-[120px]">状态</TableHead>
              <TableHead>创建时间</TableHead>
              <TableHead className="w-[260px]">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {users.map((u) => (
              <TableRow key={u.id}>
                <TableCell>
                  <Checkbox
                    checked={selectedUsers.includes(u)}
                    disabled={!canManage(u)}
                    onCheckedChange={() => toggleSelect(u)}
                  />
                </TableCell>
                <TableCell>{u.username}</TableCell>
                <TableCell>{u.displayName}</TableCell>
                <TableCell>{u.role}</TableCell>
                <TableCell>{u.status}</TableCell>
                <TableCell>{u.createdAt}</TableCell>
                <TableCell>
                  <div className="flex gap-1">
                    <Button variant="link" size="sm" disabled={!canManage(u)} onClick={() => handleEdit(u)}>编辑</Button>
                    <Button variant="link" size="sm" disabled={!canManage(u)} onClick={() => handlePassword(u)}>重置密码</Button>
                    <Button variant="link" size="sm" className="text-red-500" disabled={!canMutateIdentity(u)} onClick={() => handleDisable(u)}>禁用</Button>
                  </div>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </section>

      {/* Create Dialog */}
      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent className="sm:max-w-[420px]">
          <DialogHeader><DialogTitle>新建用户</DialogTitle></DialogHeader>
          <div className="grid gap-3">
            <div><Label>账号</Label><Input value={createForm.username} onChange={(e) => setCreateForm({ ...createForm, username: e.target.value })} /></div>
            <div><Label>姓名</Label><Input value={createForm.displayName} onChange={(e) => setCreateForm({ ...createForm, displayName: e.target.value })} /></div>
            <div><Label>密码</Label><Input type="password" value={createForm.password} onChange={(e) => setCreateForm({ ...createForm, password: e.target.value })} /></div>
            <div>
              <Label>角色</Label>
              <Select value={createForm.role} onValueChange={(v) => setCreateForm({ ...createForm, role: v as Role })}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  {roleOptions.map((r) => <SelectItem key={r.value} value={r.value}>{r.label}</SelectItem>)}
                </SelectContent>
              </Select>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateOpen(false)}>取消</Button>
            <Button onClick={handleCreate}>创建</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Edit Dialog */}
      <Dialog open={editOpen} onOpenChange={setEditOpen}>
        <DialogContent className="sm:max-w-[420px]">
          <DialogHeader><DialogTitle>编辑用户</DialogTitle></DialogHeader>
          <div className="grid gap-3">
            <div><Label>姓名</Label><Input value={editForm.displayName} onChange={(e) => setEditForm({ ...editForm, displayName: e.target.value })} /></div>
            <div>
              <Label>角色</Label>
              <Select value={editForm.role} onValueChange={(v) => setEditForm({ ...editForm, role: v as Role })}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  {roleOptions.map((r) => <SelectItem key={r.value} value={r.value}>{r.label}</SelectItem>)}
                </SelectContent>
              </Select>
            </div>
            <div>
              <Label>状态</Label>
              <Select value={editForm.status} onValueChange={(v) => setEditForm({ ...editForm, status: v })}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="active">Active</SelectItem>
                  <SelectItem value="disabled">Disabled</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setEditOpen(false)}>取消</Button>
            <Button onClick={handleSaveEdit}>保存</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Password Dialog */}
      <Dialog open={passwordOpen} onOpenChange={setPasswordOpen}>
        <DialogContent className="sm:max-w-[420px]">
          <DialogHeader><DialogTitle>重置密码</DialogTitle></DialogHeader>
          <div className="grid gap-3">
            <div><Label>用户</Label><Input value={passwordForm.displayName} disabled /></div>
            <div><Label>新密码</Label><Input type="password" value={passwordForm.password} onChange={(e) => setPasswordForm({ ...passwordForm, password: e.target.value })} /></div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setPasswordOpen(false)}>取消</Button>
            <Button onClick={handleSavePassword}>保存</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
```

- [ ] **Step 2: Verify**

Visit `/admin/users` — should show user management table.

- [ ] **Step 3: Commit**

```bash
git add frontend-react/app/\(authenticated\)/admin/users/
git commit -m "feat: add admin users page"
```

---

### Task 11: Admin Courses Page

**Files:**
- Create: `frontend-react/app/(authenticated)/admin/courses/page.tsx`

- [ ] **Step 1: Write `app/(authenticated)/admin/courses/page.tsx`**

```tsx
// app/(authenticated)/admin/courses/page.tsx
'use client'

import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { api } from '@/lib/api'
import { toast } from 'sonner'

export default function AdminCoursesPage() {
  const [courseId, setCourseId] = useState('intro-r')
  const [file, setFile] = useState<File | null>(null)
  const [loading, setLoading] = useState(false)

  async function handleImport() {
    if (!file) {
      toast.error('请选择课程包')
      return
    }
    setLoading(true)
    try {
      const payload = new FormData()
      payload.append('courseId', courseId)
      payload.append('file', file)
      await api.post('/api/admin/courses/import', payload)
      toast.success('课程已导入')
    } finally {
      setLoading(false)
    }
  }

  async function handleReload() {
    await api.post('/api/admin/courses/reload')
    toast.success('课程已重新加载')
  }

  return (
    <section className="panel single-panel !max-w-[720px]">
      <div className="panel-head">
        <h2>课程内容</h2>
        <span className="muted">上传 course.yml + chapters + assignments 课程包</span>
      </div>
      <div className="p-5">
        <div className="grid gap-3">
          <div>
            <Label>课程 ID</Label>
            <Input value={courseId} onChange={(e) => setCourseId(e.target.value)} />
          </div>
          <div>
            <Label>课程包 zip</Label>
            <Input type="file" accept=".zip" onChange={(e) => setFile(e.target.files?.[0] ?? null)} />
          </div>
          <div className="flex gap-2">
            <Button onClick={handleImport} disabled={loading}>导入课程</Button>
            <Button variant="outline" onClick={handleReload}>重新加载</Button>
          </div>
        </div>
      </div>
    </section>
  )
}
```

- [ ] **Step 2: Verify**

Visit `/admin/courses` — should show course import form.

- [ ] **Step 3: Commit**

```bash
git add frontend-react/app/\(authenticated\)/admin/courses/
git commit -m "feat: add admin courses page"
```

---

### Task 12: Lecture Pages

**Files:**
- Create: `frontend-react/app/(authenticated)/lectures/[[lectureId]]/page.tsx`
- Create: `frontend-react/app/(authenticated)/classes/[classId]/lectures/[[lectureId]]/page.tsx`

Both share the same logic — use a shared client component.

- [ ] **Step 1: Write `components/lectures/LectureViewer.tsx`**

```tsx
// components/lectures/LectureViewer.tsx
'use client'

import { useEffect, useState, useCallback } from 'react'
import { useRouter } from 'next/navigation'
import Link from 'next/link'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { MarkdownRenderer } from '@/components/markdown/MarkdownRenderer'
import { api } from '@/lib/api'
import type { ClassItem, LectureChapter, Lecture } from '@/lib/types'

interface Props {
  classId?: string
  lectureId?: string
}

export function LectureViewer({ classId, lectureId }: Props) {
  const router = useRouter()
  const [classes, setClasses] = useState<ClassItem[]>([])
  const [chapters, setChapters] = useState<LectureChapter[]>([])
  const [raw, setRaw] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const basePath = classId ? `/classes/${classId}/lectures` : '/lectures'
  const sections = chapters.flatMap((ch) => ch.sections)
  const selected = lectureId ?? sections[0]?.id ?? ''
  const selectedSection = sections.find((s) => s.id === selected)

  // Load classes
  useEffect(() => {
    api.get<{ classes: ClassItem[] }>('/api/classes').then((data) => {
      setClasses(data.classes)
      if (!classId && data.classes[0]) {
        router.replace(`/classes/${data.classes[0].id}/lectures`)
      }
    })
  }, [])

  // Load chapters
  const loadChapters = useCallback(async () => {
    const path = classId ? `/api/classes/${classId}/lectures` : '/api/lectures'
    const data = await api.get<{ lectures: LectureChapter[] }>(path)
    setChapters(data.lectures)
    if (!lectureId && data.lectures[0]?.sections[0]) {
      router.replace(`${basePath}/${data.lectures[0].sections[0].id}`)
    }
  }, [classId])

  useEffect(() => { loadChapters() }, [loadChapters])

  // Load lecture content
  useEffect(() => {
    if (!selected) return
    const controller = new AbortController()
    setLoading(true)
    setError('')
    setRaw('')

    const url = `${classId ? `/api/classes/${classId}` : '/api'}/lectures/${selected}`
    api.get<{ markdown: string }>(url)
      .then((data) => setRaw(data.markdown))
      .catch((err) => {
        if (err.name === 'AbortError') return
        setError(err instanceof Error ? err.message : '讲义加载失败')
      })
      .finally(() => setLoading(false))

    return () => controller.abort()
  }, [selected])

  return (
    <div className="page-grid">
      <aside className="panel scroll lecture-tree">
        <div className="panel-head"><h3>章节</h3></div>
        {chapters.map((ch) => (
          <div key={ch.id} className="chapter-block">
            <div className="chapter-title">{ch.title}</div>
            {ch.sections.map((section) => (
              <Link
                key={section.id}
                href={`${basePath}/${section.id}`}
                className={`section-item ${section.id === selected ? 'active' : ''}`}
              >
                <span>{section.title}</span>
                <small className="text-green-600 text-[11px]">已发布</small>
              </Link>
            ))}
          </div>
        ))}
      </aside>
      <article className="panel scroll">
        <div className="panel-head">
          <h2>{selectedSection?.title ?? '讲义'}</h2>
          {classes.length > 0 && (
            <Select
              value={classId ?? ''}
              onValueChange={(v) => router.push(v ? `/classes/${v}/lectures` : '/lectures')}
            >
              <SelectTrigger className="context-select">
                <SelectValue placeholder="选择班级" />
              </SelectTrigger>
              <SelectContent>
                {classes.map((klass) => (
                  <SelectItem key={klass.id} value={klass.id}>{klass.name}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
        </div>
        {loading && (
          <div className="p-6">
            <Skeleton className="h-4 w-full mb-2" />
            <Skeleton className="h-4 w-3/4 mb-2" />
            <Skeleton className="h-4 w-5/6 mb-2" />
          </div>
        )}
        {error && (
          <div className="p-10 text-center text-red-500">{error}</div>
        )}
        {!loading && !error && (
          <div className="markdown">
            <MarkdownRenderer content={raw} />
          </div>
        )}
      </article>
    </div>
  )
}
```

- [ ] **Step 2: Write `app/(authenticated)/lectures/[[lectureId]]/page.tsx`**

```tsx
// app/(authenticated)/lectures/[[lectureId]]/page.tsx
import { LectureViewer } from '@/components/lectures/LectureViewer'

export default async function GlobalLecturePage({
  params,
}: {
  params: Promise<{ lectureId?: string }>
}) {
  const { lectureId } = await params
  return <LectureViewer lectureId={lectureId} />
}
```

- [ ] **Step 3: Write `app/(authenticated)/classes/[classId]/lectures/[[lectureId]]/page.tsx`**

```tsx
// app/(authenticated)/classes/[classId]/lectures/[[lectureId]]/page.tsx
import { LectureViewer } from '@/components/lectures/LectureViewer'

export default async function ClassLecturePage({
  params,
}: {
  params: Promise<{ classId: string; lectureId?: string }>
}) {
  const { classId, lectureId } = await params
  return <LectureViewer classId={classId} lectureId={lectureId} />
}
```

- [ ] **Step 4: Add Skeleton component**

```bash
npx shadcn@latest add skeleton
```

- [ ] **Step 5: Verify**

Visit `/lectures` or `/classes/<id>/lectures` — should show chapter tree + markdown content.

- [ ] **Step 6: Commit**

```bash
git add frontend-react/app/\(authenticated\)/lectures/ frontend-react/app/\(authenticated\)/classes/\[classId\]/lectures/ frontend-react/components/lectures/
git commit -m "feat: add lecture pages with MarkdownRenderer"
```

---

### Task 13: Assignment Page (Most Complex)

**Files:**
- Create: `frontend-react/components/assignments/AssignmentViewer.tsx`
- Create: `frontend-react/app/(authenticated)/assignments/[[assignmentId]]/page.tsx`
- Create: `frontend-react/app/(authenticated)/classes/[classId]/assignments/[[assignmentId]]/page.tsx`

This is the most complex page — handles iframe RStudio, resizable panels, submissions, grading, history.

- [ ] **Step 1: Write `components/assignments/AssignmentViewer.tsx`**

This is a large component. We'll implement it faithfully porting AssignmentPage.vue:

```tsx
// components/assignments/AssignmentViewer.tsx
'use client'

import { useEffect, useState, useCallback, useMemo, useRef } from 'react'
import { useRouter } from 'next/navigation'
import dayjs from 'dayjs'
import Link from 'next/link'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { MarkdownRenderer } from '@/components/markdown/MarkdownRenderer'
import { api } from '@/lib/api'
import { useSession } from '@/stores/session'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import type { Assignment, ClassItem, Submission, SubmissionRow } from '@/lib/types'
import { Maximize2, Minimize2 } from 'lucide-react'

interface AssignmentEntry {
  assignment: Assignment
  classId: string
  className: string
  courseId: string
}

interface Props {
  classId?: string
  assignmentId?: string
}

const ASSIGNMENT_LIST_WIDTH = 205
const ASSIGNMENT_IDE_WIDTH = 360

function sleep(ms: number) { return new Promise((r) => setTimeout(r, ms)) }

export function AssignmentViewer({ classId, assignmentId }: Props) {
  const router = useRouter()
  const { user } = useSession()
  const isStudent = user?.role === 'student'

  const [classes, setClasses] = useState<ClassItem[]>([])
  const [entries, setEntries] = useState<AssignmentEntry[]>([])
  const [raw, setRaw] = useState('')
  const [status, setStatus] = useState<Record<string, unknown>>({})
  const [rows, setRows] = useState<SubmissionRow[]>([])
  const [workspaceURL, setWorkspaceURL] = useState('')
  const [workspaceLoading, setWorkspaceLoading] = useState(false)
  const [workspaceFullscreen, setWorkspaceFullscreen] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [promptOpen, setPromptOpen] = useState(true)
  const [classFilter, setClassFilter] = useState(classId ?? '')
  const [gradeDialog, setGradeDialog] = useState(false)
  const [currentSubmission, setCurrentSubmission] = useState<Submission | null>(null)
  const [selectedSubmission, setSelectedSubmission] = useState<SubmissionRow | null>(null)
  const [gradeForm, setGradeForm] = useState({ score: 0, comment: '' })
  const [historyOpen, setHistoryOpen] = useState(false)
  const [historyStudent, setHistoryStudent] = useState('')
  const [listWidth, setListWidth] = useState(
    () => Number(typeof window !== 'undefined' ? localStorage.getItem('hdu.assignment.listWidth') : null) || ASSIGNMENT_LIST_WIDTH
  )
  const [ideWidth, setIdeWidth] = useState(
    () => Number(typeof window !== 'undefined' ? localStorage.getItem('hdu.assignment.ideWidth') : null) || ASSIGNMENT_IDE_WIDTH
  )

  const gridRef = useRef<HTMLDivElement>(null)

  // Filtered entries
  const filteredEntries = useMemo(
    () => entries.filter((e) => !classFilter || e.classId === classFilter),
    [entries, classFilter]
  )

  // Selected entry
  const activeClassId = classId || filteredEntries[0]?.classId || ''
  const selected = assignmentId ?? filteredEntries[0]?.assignment.id ?? ''
  const selectedEntry = filteredEntries.find(
    (e) => e.assignment.id === selected && (!classFilter || e.classId === classFilter)
  ) ?? filteredEntries[0]
  const selectedAssignment = selectedEntry?.assignment

  // Latest rows (one per user, highest attempt)
  const latestRows = useMemo(() => {
    const byUser = new Map<string, SubmissionRow>()
    for (const row of rows) {
      const prev = byUser.get(row.submission.userId)
      if (!prev || row.submission.attempt > prev.submission.attempt) {
        byUser.set(row.submission.userId, row)
      }
    }
    return Array.from(byUser.values()).sort((a, b) =>
      a.studentName.localeCompare(b.studentName, 'zh-CN')
    )
  }, [rows])

  const historyRows = useMemo(
    () =>
      rows
        .filter((r) => r.submission.userId === historyStudent)
        .sort((a, b) => b.submission.attempt - a.submission.attempt),
    [rows, historyStudent]
  )

  const reviewedCount = latestRows.filter((r) => r.grade.score !== null).length
  const pendingCount = latestRows.filter((r) => r.grade.score === null).length
  const averageScore = useMemo(() => {
    const scores = latestRows
      .map((r) => r.grade.score)
      .filter((s): s is number => s !== null)
    return scores.length ? (scores.reduce((a, b) => a + b, 0) / scores.length).toFixed(1) : '-'
  }, [latestRows])

  function assignmentPath(entry: AssignmentEntry) {
    if (classId) return `/classes/${entry.classId}/assignments/${entry.assignment.id}`
    return { pathname: `/assignments/${entry.assignment.id}`, query: { classId: entry.classId } }
  }

  // Load assignments
  useEffect(() => {
    async function load() {
      const data = await api.get<{ classes: ClassItem[] }>('/api/classes')
      setClasses(data.classes)
      const visibleClasses = classId ? data.classes.filter((c) => c.id === classId) : data.classes
      const loaded = await Promise.all(
        visibleClasses.map(async (klass) => {
          const d = await api.get<{ assignments: Assignment[] }>(`/api/classes/${klass.id}/assignments`)
          return d.assignments.map((a) => ({
            assignment: a,
            classId: klass.id,
            className: klass.name,
            courseId: klass.courseId,
          }))
        })
      )
      setEntries(loaded.flat())
      if (!classId) setClassFilter(new URLSearchParams(window.location.search).get('classId') ?? '')
    }
    load()
  }, [])

  // Load assignment detail
  const loadAssignment = useCallback(async () => {
    if (!selected || !activeClassId) return
    const data = await api.get<{ assignment: Assignment; markdown: string; status: Record<string, unknown> }>(
      `/api/classes/${activeClassId}/assignments/${selected}`
    )
    setRaw(data.markdown)
    setStatus(data.status)
    setPromptOpen(isStudent)
    setSelectedSubmission(null)
    setWorkspaceURL('')
    setWorkspaceFullscreen(false)
    if (!isStudent) {
      const subData = await api.get<{ submissions: SubmissionRow[] }>(
        `/api/classes/${activeClassId}/assignments/${selected}/submissions`
      )
      setRows(subData.submissions)
    } else {
      setRows([])
    }
  }, [selected, activeClassId, isStudent])

  useEffect(() => { loadAssignment() }, [loadAssignment])

  // Workspace
  async function waitForGateway(url: string) {
    for (let i = 0; i < 30; i++) {
      try {
        const resp = await fetch(url, { credentials: 'include', cache: 'no-store' })
        if (resp.ok) return
      } catch { /* retry */ }
      await sleep(700)
    }
    throw new Error('RStudio 尚未就绪')
  }

  async function startWorkspace() {
    setWorkspaceLoading(true)
    try {
      const data = await api.post<{ workspace: { id: string; ideURL: string } }>(
        `/api/classes/${activeClassId}/assignments/${selected}/workspace`
      )
      await waitForGateway(data.workspace.ideURL)
      setWorkspaceURL(data.workspace.ideURL)
    } finally {
      setWorkspaceLoading(false)
    }
  }

  async function handleSubmit() {
    setSubmitting(true)
    try {
      await api.post(`/api/classes/${activeClassId}/assignments/${selected}/submit`)
      toast.success('工作区已提交')
      await loadAssignment()
    } finally {
      setSubmitting(false)
    }
  }

  async function selectSubmissionRow(row: SubmissionRow) {
    setSelectedSubmission(row)
    setWorkspaceURL('')
    setWorkspaceLoading(true)
    try {
      const data = await api.post<{ workspace: { id: string; ideURL: string } }>(
        `/api/submissions/${row.submission.id}/workspace`
      )
      await waitForGateway(data.workspace.ideURL)
      setWorkspaceURL(data.workspace.ideURL)
    } finally {
      setWorkspaceLoading(false)
    }
  }

  function openGrade(row: SubmissionRow) {
    selectSubmissionRow(row)
    setCurrentSubmission(row.submission)
    setGradeForm({ score: row.grade.score ?? 0, comment: row.grade.comment ?? '' })
    setGradeDialog(true)
  }

  async function saveGrade() {
    if (!currentSubmission) return
    const data = await api.post<{ id: string }>(`/api/submissions/${currentSubmission.id}/grade`, gradeForm)
    await api.post(`/api/grades/${data.id}/publish`)
    toast.success('成绩已发布')
    setGradeDialog(false)
    await loadAssignment()
  }

  function showHistory(row: SubmissionRow) {
    setHistoryStudent(row.submission.userId)
    setHistoryOpen(true)
  }

  async function exportGrades() {
    const blob = await api.download(
      `/api/classes/${activeClassId}/assignments/${selected}/grades/export?format=csv`
    )
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `${selectedEntry?.className ?? 'class'}-${selected}-grades.csv`
    document.body.appendChild(link)
    link.click()
    link.remove()
    URL.revokeObjectURL(url)
  }

  // Resize handling
  function clamp(v: number, min: number, max: number) {
    return Math.min(Math.max(v, min), Math.max(min, max))
  }

  function startResize(target: 'list' | 'ide', e: React.PointerEvent) {
    const rect = gridRef.current?.getBoundingClientRect()
    if (!rect) return
    e.preventDefault()
    const move = (ev: PointerEvent) => {
      if (target === 'list') {
        setListWidth(clamp(ev.clientX - rect.left, 170, 280))
      } else {
        setIdeWidth(clamp(rect.right - ev.clientX, 280, rect.width - listWidth - 430))
      }
    }
    const stop = () => {
      localStorage.setItem('hdu.assignment.listWidth', String(listWidth))
      localStorage.setItem('hdu.assignment.ideWidth', String(ideWidth))
      window.removeEventListener('pointermove', move)
      window.removeEventListener('pointerup', stop)
    }
    window.addEventListener('pointermove', move)
    window.addEventListener('pointerup', stop, { once: true })
  }

  const gridStyle = {
    '--assignment-list-width': `${listWidth}px`,
    '--assignment-ide-width': `${ideWidth}px`,
  } as React.CSSProperties

  return (
    <>
      <div ref={gridRef} className="assignment-grid" style={gridStyle}>
        {/* Left: Assignment List */}
        <aside className="panel scroll">
          <div className="panel-head"><h3>作业</h3></div>
          {!classId && (
            <div className="p-2 border-b">
              <Select value={classFilter} onValueChange={(v) => { setClassFilter(v); router.push(v ? `?classId=${v}` : '/assignments') }}>
                <SelectTrigger><SelectValue placeholder="选择班级" /></SelectTrigger>
                <SelectContent>
                  {classes.map((klass) => (
                    <SelectItem key={klass.id} value={klass.id}>{klass.name}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          )}
          {filteredEntries.map((entry) => {
            const href = assignmentPath(entry)
            const isActive = entry.assignment.id === selected && entry.classId === activeClassId
            return (
              <Link
                key={`${entry.classId}-${entry.assignment.id}`}
                href={typeof href === 'string' ? href : { pathname: href.pathname, query: href.query }}
                className={cn('list-item', isActive && 'active')}
              >
                <strong>{entry.assignment.title}</strong>
                <div className="muted">{entry.className}</div>
                <div className="muted">截止 {dayjs(entry.assignment.dueAt).format('MM-DD HH:mm')}</div>
              </Link>
            )
          })}
        </aside>

        <div className="splitter" onPointerDown={(e) => startResize('list', e)} />

        {/* Center: Assignment Detail */}
        <section className="panel scroll">
          <div className="panel-head">
            <div>
              <h2>{selectedAssignment?.title ?? '作业'}</h2>
              <span className="muted">{selectedEntry?.className}</span>
            </div>
          </div>

          <div className={cn('prompt-block', !promptOpen && 'collapsed')}>
            <button className="prompt-toggle" onClick={() => setPromptOpen(!promptOpen)}>
              <strong>作业题面</strong>
              <span>{promptOpen ? '收起' : '展开'}</span>
            </button>
            {promptOpen && (
              <div className="markdown">
                <MarkdownRenderer content={raw} />
              </div>
            )}
          </div>

          {isStudent ? (
            <div className="student-submit">
              <div className="grid grid-cols-2 border rounded-md overflow-hidden mb-4">
                <div className="p-2 border-r"><span className="text-xs text-gray-500">提交次数</span><div className="font-bold">{String(status.attempts ?? 0)}</div></div>
                <div className="p-2"><span className="text-xs text-gray-500">最近提交</span><div className="font-bold">{String(status.latestSubmittedAt ?? '未提交')}</div></div>
              </div>
              <div className="submit-actions">
                <Button onClick={handleSubmit} disabled={submitting}>提交 RStudio 工作区</Button>
              </div>
            </div>
          ) : (
            <div className="grading-overview">
              <div className="grading-toolbar">
                <Button variant="outline" onClick={exportGrades}>导出成绩 CSV</Button>
              </div>
              <div className="metric-row">
                <div><strong>{latestRows.length}</strong><span>提交数</span></div>
                <div><strong>{pendingCount}</strong><span>待批改</span></div>
                <div><strong>{reviewedCount}</strong><span>已批改</span></div>
                <div><strong>{averageScore}</strong><span>平均分</span></div>
              </div>
              <div className="mt-3">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead className="w-[110px]">学生</TableHead>
                      <TableHead>提交</TableHead>
                      <TableHead className="w-[82px]">成绩</TableHead>
                      <TableHead className="w-[112px]">操作</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {latestRows.map((row) => (
                      <TableRow
                        key={row.submission.id}
                        className={cn(row.submission.id === selectedSubmission?.submission.id && 'bg-blue-50')}
                        onClick={() => selectSubmissionRow(row)}
                      >
                        <TableCell>{row.studentName}</TableCell>
                        <TableCell>
                          <Button variant="link" size="sm" onClick={(e) => { e.stopPropagation(); showHistory(row) }}>
                            第 {row.submission.attempt} 次
                          </Button>
                          {row.submission.late && <Badge variant="warning">补交</Badge>}
                        </TableCell>
                        <TableCell>{row.grade.score ?? '未评分'}</TableCell>
                        <TableCell>
                          <div className="flex gap-1">
                            <Button variant="link" size="sm" onClick={(e) => { e.stopPropagation(); selectSubmissionRow(row) }}>查看</Button>
                            <Button variant="link" size="sm" onClick={(e) => { e.stopPropagation(); openGrade(row) }}>批改</Button>
                          </div>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            </div>
          )}
        </section>

        <div className="splitter" onPointerDown={(e) => startResize('ide', e)} />

        {/* Right: RStudio Workspace */}
        <section className={cn('panel workspace-panel', workspaceFullscreen && 'fullscreen')}>
          <div className="panel-head">
            <h3>{selectedSubmission ? '批改工作区' : 'RStudio 工作区'}</h3>
            <div className="flex items-center gap-2">
              {workspaceURL && (
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => setWorkspaceFullscreen(!workspaceFullscreen)}
                >
                  {workspaceFullscreen ? <Minimize2 size={16} /> : <Maximize2 size={16} />}
                  <span className="ml-1">{workspaceFullscreen ? '退出全屏' : '全屏'}</span>
                </Button>
              )}
              {isStudent ? (
                <Button onClick={startWorkspace} disabled={workspaceLoading}>打开 RStudio</Button>
              ) : selectedSubmission ? (
                <Button onClick={() => selectSubmissionRow(selectedSubmission)} disabled={workspaceLoading}>重新打开</Button>
              ) : null}
            </div>
          </div>
          {workspaceURL ? (
            <iframe src={workspaceURL} className="ide-frame" />
          ) : !isStudent && selectedSubmission ? (
            <div className="submission-preview">
              <div className="preview-meta">
                <strong>{selectedSubmission.studentName}</strong>
                <span>第 {selectedSubmission.submission.attempt} 次提交</span>
                {selectedSubmission.grade.score !== null ? (
                  <Badge variant="success">{selectedSubmission.grade.score}</Badge>
                ) : (
                  <Badge variant="warning">未评分</Badge>
                )}
              </div>
              {workspaceLoading && <Skeleton className="h-20 w-full" />}
            </div>
          ) : (
            <div className="flex items-center justify-center h-[calc(100%-52px)] text-gray-400">
              {isStudent ? '点击打开后创建独立 RStudio Pod' : '选择提交后在这里查看内容'}
            </div>
          )}
        </section>
      </div>

      {/* History Dialog */}
      <Dialog open={historyOpen} onOpenChange={setHistoryOpen}>
        <DialogContent className="sm:max-w-[520px]">
          <DialogHeader><DialogTitle>提交历史</DialogTitle></DialogHeader>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="w-[100px]">版本</TableHead>
                <TableHead>提交时间</TableHead>
                <TableHead className="w-[90px]">成绩</TableHead>
                <TableHead className="w-[90px]">操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {historyRows.map((row) => (
                <TableRow key={row.submission.id}>
                  <TableCell>第 {row.submission.attempt} 次</TableCell>
                  <TableCell>{dayjs(row.submission.createdAt).format('MM-DD HH:mm')}</TableCell>
                  <TableCell>{row.grade.score ?? '未评分'}</TableCell>
                  <TableCell>
                    <Button
                      variant="link"
                      size="sm"
                      onClick={() => { setHistoryOpen(false); selectSubmissionRow(row) }}
                    >
                      查看
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </DialogContent>
      </Dialog>

      {/* Grade Dialog */}
      <Dialog open={gradeDialog} onOpenChange={setGradeDialog}>
        <DialogContent className="sm:max-w-[440px]">
          <DialogHeader><DialogTitle>评分</DialogTitle></DialogHeader>
          <div className="grid gap-3">
            <div>
              <Label>分数</Label>
              <Input
                type="number"
                min={0}
                max={100}
                value={gradeForm.score}
                onChange={(e) => setGradeForm({ ...gradeForm, score: Number(e.target.value) })}
              />
            </div>
            <div>
              <Label>评语</Label>
              <textarea
                className="flex min-h-[80px] w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm"
                rows={5}
                value={gradeForm.comment}
                onChange={(e) => setGradeForm({ ...gradeForm, comment: e.target.value })}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setGradeDialog(false)}>取消</Button>
            <Button onClick={saveGrade}>发布成绩</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
```

- [ ] **Step 2: Write page wrappers**

`app/(authenticated)/assignments/[[assignmentId]]/page.tsx`:
```tsx
import { AssignmentViewer } from '@/components/assignments/AssignmentViewer'

export default async function GlobalAssignmentPage({
  params,
  searchParams,
}: {
  params: Promise<{ assignmentId?: string }>
  searchParams: Promise<{ classId?: string }>
}) {
  const { assignmentId } = await params
  const { classId } = await searchParams
  return <AssignmentViewer assignmentId={assignmentId} classId={classId} />
}
```

`app/(authenticated)/classes/[classId]/assignments/[[assignmentId]]/page.tsx`:
```tsx
import { AssignmentViewer } from '@/components/assignments/AssignmentViewer'

export default async function ClassAssignmentPage({
  params,
}: {
  params: Promise<{ classId: string; assignmentId?: string }>
}) {
  const { classId, assignmentId } = await params
  return <AssignmentViewer classId={classId} assignmentId={assignmentId} />
}
```

- [ ] **Step 3: Add Badge component**

```bash
npx shadcn@latest add badge
```

- [ ] **Step 4: Verify**

Visit `/classes/<id>/assignments/<id>` or `/assignments` — test: list, RStudio iframe, resizing, grading, submit.

- [ ] **Step 5: Commit**

```bash
git add frontend-react/components/assignments/ frontend-react/app/\(authenticated\)/assignments/ frontend-react/app/\(authenticated\)/classes/\[classId\]/assignments/
git commit -m "feat: add assignment page with workspace, submissions, grading"
```

---

### Task 14: AG-UI Chat Page with CopilotKit + Bailian

**Files:**
- Create: `frontend-react/app/api/copilotkit/route.ts`
- Create: `frontend-react/app/(authenticated)/agui/page.tsx`

- [ ] **Step 1: Write `app/api/copilotkit/route.ts` — CopilotKit runtime with Bailian adapter**

```typescript
// app/api/copilotkit/route.ts
import {
  CopilotRuntime,
  OpenAIAdapter,
  copilotRuntimeNextJSAppRouterEndpoint,
} from '@copilotkit/runtime'
import { NextRequest } from 'next/server'

// Bailian (百炼) uses OpenAI-compatible API format
const serviceAdapter = new OpenAIAdapter({
  apiKey: process.env.BAILIAN_API_KEY!,
  baseURL: 'https://dashscope.aliyuncs.com/compatible-mode/v1',
  defaultHeaders: {
    'X-DashScope-AppId': process.env.BAILIAN_APP_ID!,
  },
})

const runtime = new CopilotRuntime()

export const POST = async (req: NextRequest) => {
  const { handleRequest } = copilotRuntimeNextJSAppRouterEndpoint({
    runtime,
    serviceAdapter,
    endpoint: '/api/copilotkit',
  })
  return handleRequest(req)
}
```

Note: Install the runtime package if needed:
```bash
bun add @copilotkit/runtime
```

If `@copilotkit/runtime` API differs, check CopilotKit docs for the exact adapter configuration. The key point is Bailian's OpenAI-compatible endpoint: `https://dashscope.aliyuncs.com/compatible-mode/v1` with the `X-DashScope-AppId` header.

- [ ] **Step 2: Write `app/(authenticated)/agui/page.tsx`**

```tsx
// app/(authenticated)/agui/page.tsx
'use client'

import { CopilotKit } from '@copilotkit/react-core'
import { CopilotChat } from '@copilotkit/react-ui'
import '@copilotkit/react-ui/styles.css'

export default function AguiPage() {
  return (
    <div className="h-[calc(100vh-94px)] bg-[#f7f9fc] rounded-lg overflow-hidden shadow-sm">
      <CopilotKit runtimeUrl="/api/copilotkit">
        <CopilotChat
          labels={{
            title: 'AI 助手',
            initial: '你好！我是 HDU-RIDE AI 助手，基于通义千问。有什么可以帮你？',
            placeholder: '输入消息…（Enter 发送）',
          }}
          className="h-full"
        />
      </CopilotKit>
    </div>
  )
}
```

- [ ] **Step 3: Verify CopilotKit integration**

1. Set `BAILIAN_API_KEY` and `BAILIAN_APP_ID` in `.env.local`
2. Visit `/agui`
3. Type a message and verify streaming response from Bailian
4. Verify Markdown rendering in chat messages (formulas included)

- [ ] **Step 4: If CopilotKit Bailian compatibility issue**

If the `OpenAIAdapter` approach doesn't work directly with Bailian's specific headers, create a custom adapter:

```typescript
// lib/bailian-adapter.ts
// Custom fetch-based adapter for CopilotKit runtime
// Maps OpenAI-compatible requests to Bailian API
export async function bailianChatCompletion(body: {
  messages: Array<{ role: string; content: string }>
  stream?: boolean
}) {
  const response = await fetch(
    'https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions',
    {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${process.env.BAILIAN_API_KEY}`,
        'X-DashScope-AppId': process.env.BAILIAN_APP_ID!,
      },
      body: JSON.stringify({
        model: 'qwen-plus',
        messages: body.messages,
        stream: body.stream ?? true,
      }),
    }
  )
  return response
}
```

- [ ] **Step 5: Commit**

```bash
git add frontend-react/app/api/copilotkit/ frontend-react/app/\(authenticated\)/agui/
git commit -m "feat: add AG-UI chat page with CopilotKit + Bailian integration"
```

---

### Task 15: Final Verification & Cleanup

**Files:**
- Create/Modify: `frontend-react/next.config.ts` (add Go backend proxy if needed)
- Modify: `.gitignore` (ensure .env.local is ignored)

- [ ] **Step 1: Configure Next.js to proxy Go backend API in dev**

Update `next.config.ts`:
```typescript
import type { NextConfig } from 'next'

const nextConfig: NextConfig = {
  async rewrites() {
    return [
      {
        source: '/api/:path*',
        destination: `${process.env.NEXT_PUBLIC_GO_API_URL ?? 'http://localhost:8080'}/api/:path*`,
      },
      {
        source: '/ide/:path*',
        destination: `${process.env.NEXT_PUBLIC_GO_API_URL ?? 'http://localhost:8080'}/ide/:path*`,
      },
    ]
  },
}

export default nextConfig
```

Note: With this proxy, `lib/api.ts` no longer needs `NEXT_PUBLIC_GO_API_URL` prefix — all API calls go through Next.js which forwards to Go backend. But CopilotKit's `/api/copilotkit` route must NOT be forwarded. The proxy handles this naturally since the Next.js API route takes priority over rewrites.

- [ ] **Step 2: Update `lib/api.ts` to remove baseUrl (use relative paths)**

Since Next.js will proxy `/api/*` to Go backend, simplify `request()`:
```typescript
async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(path, {
    credentials: 'include',
    headers: init.body instanceof FormData ? init.headers : { 'Content-Type': 'application/json', ...init.headers },
    ...init,
  })
  // ... rest unchanged
}
```

- [ ] **Step 3: End-to-end verification checklist**

Run `bun run dev` and verify all routes:
- [x] `/login` — login page renders, auth works
- [x] `/classes` — class list loads
- [x] `/classes/<id>/members` — member management works
- [x] `/classes/<id>/lectures/<id>` — lecture with math formulas renders correctly
- [x] `/classes/<id>/assignments/<id>` — RStudio iframe, submit, grading work
- [x] `/admin/users` — user CRUD works
- [x] `/admin/courses` — course import/reload works
- [x] `/agui` — CopilotKit chat works with Bailian
- [x] Sidebar navigation and collapse
- [x] Topbar user dropdown, password change

- [ ] **Step 4: Verify LaTeX formula rendering specifically**

Create a test lecture/assignment with formulas like:
```
Inline: $E = mc^2$

Block:
$$
\begin{aligned}
\hat{\beta} &= (X^TX)^{-1}X^Ty \\
R^2 &= 1 - \frac{SS_{res}}{SS_{tot}}
\end{aligned}
$$
```
Verify both render correctly with KaTeX.

- [ ] **Step 5: Commit**

```bash
git add frontend-react/
git commit -m "feat: finalize React migration — Next.js proxy, all pages verified"
```

---

### Task 16: Remove Old Vue Frontend

**Files:**
- Delete: `frontend/`

- [ ] **Step 1: Remove old frontend directory**

```bash
git rm -r frontend/
```

- [ ] **Step 2: Update any references in deploy/docs/scripts**

Check if `deploy/`, `DEPLOY.md`, `README.md`, or `scripts/` reference `frontend/` and update to `frontend-react/`.

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "chore: remove old Vue frontend, migration complete"
```

---

## Summary

| Task | Description | Files Created |
|------|-------------|---------------|
| 1 | Project init & deps | Next.js + Shadcn/ui scaffold |
| 2 | Base infrastructure | lib/types.ts, lib/api.ts, lib/utils.ts |
| 3 | Session store & middleware | stores/session.ts, middleware.ts, .env.local |
| 4 | App layout & global styles | globals.css, layout.tsx, sidebar, topbar |
| 5 | Markdown renderer | components/markdown/MarkdownRenderer.tsx |
| 6 | Login page | app/login/page.tsx |
| 7 | Authenticated layout shell | Route group layout + sidebar context |
| 8 | Classes page | app/classes/page.tsx |
| 9 | Class members page | app/classes/[classId]/members/page.tsx |
| 10 | Admin users page | app/admin/users/page.tsx |
| 11 | Admin courses page | app/admin/courses/page.tsx |
| 12 | Lecture pages | Shared LectureViewer + 2 page wrappers |
| 13 | Assignment page | Shared AssignmentViewer + 2 page wrappers |
| 14 | AG-UI CopilotKit chat | CopilotKit runtime + agui page |
| 15 | Final verification & proxy | next.config.ts rewrite, E2E check |
| 16 | Remove old Vue frontend | Delete frontend/ |
