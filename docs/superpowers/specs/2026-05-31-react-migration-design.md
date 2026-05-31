# HDU RIDE 前端 React 迁移设计

日期: 2026-05-31
状态: 设计已确认，待制定实施计划

## 1. 迁移动机

1. **引入 AG-UI 生态**：使用 CopilotKit 的 AG-UI 组件，替代当前自定义 AI 聊天实现
2. **修复 Markdown 公式渲染**：当前 markdown-it + @traptitech/markdown-it-katex 方案存在公式渲染问题，迁移到 react-markdown + remark-math + rehype-katex
3. **技术栈统一**：React 生态拥有更丰富的 AG-UI / AI 框架支持

## 2. 目标技术栈

| 层面 | Vue 当前 | React 目标 |
|------|----------|-----------|
| 框架 | Vue 3 + Vue Router | Next.js (App Router) |
| UI 组件 | Element Plus | Shadcn/ui + Tailwind CSS |
| Markdown | markdown-it + markdown-it-katex | react-markdown + remark-math + rehype-katex |
| AI 聊天 | 自定义 SSE（AguiPage.vue） | CopilotKit AG-UI + 百炼适配器 |
| 状态管理 | composables (ref/reactive) | Zustand |
| 构建 | Vite + Bun | Next.js (Turbopack) + Bun |
| 后端 API | Go REST | Go 业务 API 保留 + Next.js API Routes（AI/BFF） |

## 3. 整体架构

```
┌─────────────────────────────────────────────────┐
│                    Browser                        │
├─────────────────────────────────────────────────┤
│  Next.js App (React)                             │
│  ┌────────────────────┐ ┌────────────────────┐  │
│  │  Pages & Layouts    │ │  Zustand Stores    │  │
│  │  (App Router)       │ │  (auth, chat, ...) │  │
│  ├────────────────────┤ └────────────────────┘  │
│  │  Shadcn/ui          │                         │
│  │  + Tailwind         │  react-markdown         │
│  │                     │  + remark-math          │
│  ├────────────────────┤  + rehype-katex          │
│  │  CopilotKit         │                         │
│  │  AG-UI Components   │                         │
│  └─────────┬───────────┘                         │
│            │                                      │
│  ┌─────────▼───────────┐                         │
│  │  API Layer (fetch)   │  ─── Go REST API        │
│  │  + CopilotKit Runtime│  ─── Next.js API Routes │
│  └─────────┬───────────┘        (AI / AG-UI)     │
│            │                                      │
└────────────┼──────────────────────────────────────┘
             │
    ┌────────▼────────┐    ┌───────────────┐
    │  Go Backend      │    │  百炼 API      │
    │  (业务 CRUD)      │    │  (通义千问)     │
    │  (K8s 工作区)     │    │               │
    └─────────────────┘    └───────────────┘
```

- **Next.js App Router** 负责前端路由、页面渲染、Middleware 鉴权
- **Go 后端** 保持所有业务 API（classes, lectures, assignments, submissions, workspaces 等），逻辑不动
- **Next.js API Routes** 运行 CopilotKit runtime，作为百炼 API 的 BFF 层，管理 API Key，处理 AG-UI 协议
- **Zustand** 管理客户端全局状态（session、聊天会话等），替代当前 composables
- **react-markdown** 统一处理讲义渲染、题面渲染、AI 消息中的 Markdown + LaTeX

## 4. 路由与页面映射

当前 9 个 Vue 页面平移到 Next.js App Router：

```
frontend-react/
├── app/
│   ├── layout.tsx              # 根布局（Sidebar + Topbar + session provider）
│   ├── page.tsx                # redirect -> /classes
│   ├── login/page.tsx          # Login.vue → 登录页
│   ├── classes/
│   │   ├── page.tsx            # ClassHome.vue → 班级列表
│   │   └── [classId]/
│   │       ├── members/page.tsx      # ClassMembers.vue → 班级成员
│   │       ├── lectures/[[lectureId]]/page.tsx  # LecturePage.vue
│   │       └── assignments/[[assignmentId]]/page.tsx  # AssignmentPage.vue
│   ├── lectures/[[lectureId]]/page.tsx  # 全局讲义
│   ├── assignments/[[assignmentId]]/page.tsx  # 全局作业（学生多班级视图）
│   ├── admin/
│   │   ├── users/page.tsx      # AdminUsers.vue → 用户管理
│   │   └── courses/page.tsx    # AdminCourseImport.vue → 课程管理
│   └── agui/
│       └── page.tsx            # AguiPage.vue → AG-UI 重构版
├── components/
│   ├── ui/                     # Shadcn/ui 生成的基础组件
│   ├── layout/                 # AppShell, Sidebar, Topbar
│   ├── markdown/               # MarkdownRenderer
│   └── shared/                 # 通用业务组件（EmptyState, ConfirmDialog 等）
├── lib/
│   ├── api.ts                  # fetch wrapper（平移自当前 api.ts）
│   ├── types.ts                # TypeScript 类型（平移 + 扩展）
│   └── utils.ts                # 工具函数
├── stores/                     # Zustand stores
│   ├── session.ts              # 登录态（平移 useSession）
│   └── chat.ts                 # 聊天会话状态
├── middleware.ts               # Next.js Middleware（路由守卫）
└── app/api/                    # Next.js API Routes（BFF）
    └── copilotkit/route.ts     # CopilotKit AG-UI endpoint
```

### 路由守卫

`middleware.ts` 检查 cookie session → 未登录重定向 `/login`，已登录访问 `/login` 则重定向 `/classes`。逻辑平直当前 `router.beforeEach`。

### 全局布局

`app/layout.tsx` 渲染侧栏导航 + 顶栏用户菜单 + `<main>` 内容区，替代当前 `App.vue` 中的 `<div class="app-shell">` 结构。

## 5. 状态管理与数据流

### Zustand Store 设计

**Session Store**（平移自 `useSession.ts`）：

```
stores/session.ts
  - user: User | null
  - initialized: boolean
  - fetchSession()
  - login(username, password)
  - logout()
  - changePassword(oldPw, newPw)
  - isAdmin (computed)
  - canTeach (computed)
```

**Chat Store**（平移自 `AguiPage.vue` conversation 逻辑，适配 CopilotKit）：

```
stores/chat.ts
  - conversations: Conversation[]
  - activeId: string | null
  - newConversation()
  - deleteConversation(id)
  - selectConversation(id)
```

### 数据流

```
Middleware (auth check)
  └─> Layout (SessionProvider, 注入 session store)
        └─> Page (Server Component wrapper)
              └─> Client Components
                    ├── 读 Zustand store
                    ├── 调 lib/api.ts → Go Backend API
                    └── 调 CopilotKit hooks → /api/copilotkit → 百炼
```

- **Go API**：`lib/api.ts`（fetch wrapper，平移当前逻辑），所有 CRUD 操作不变
- **AI 交互**：CopilotKit 的 `useCopilotChat` / `useCopilotAction` 等 hooks，通过 Next.js API Routes 代理到百炼
- **Middleware**：纯服务端 cookie 读取，不依赖 Zustand

### CopilotKit + 百炼集成

- `app/api/copilotkit/route.ts` 中配置 CopilotKit runtime，`remoteEndpoints` 指向百炼 API
- 百炼兼容 OpenAI 格式（`/v1/chat/completions`），CopilotKit 原生支持
- `BAILIAN_API_KEY` + `BAILIAN_APP_ID` 存 Next.js 环境变量，服务端使用
- 前端 `<CopilotKit>` provider 包裹 `/agui` 路由，`<CopilotChat>` 替代自定义聊天 UI

## 6. Markdown 渲染

统一使用 `react-markdown` + 插件体系：

```tsx
// components/markdown/MarkdownRenderer.tsx
import ReactMarkdown from 'react-markdown'
import remarkMath from 'remark-math'
import rehypeKatex from 'rehype-katex'
import remarkGfm from 'remark-gfm'
import 'katex/dist/katex.min.css'
```

- **使用场景**：讲义渲染、题面渲染、AI 消息中的 Markdown 内容
- **代码块**：自定义 `CodeBlock` 组件（语法高亮 + 复制按钮）
- **公式**：KaTeX 服务端+客户端双方案，块级公式和行内公式均支持

## 7. AG-UI 聊天实现

### 组件结构

```tsx
// app/agui/page.tsx
<CopilotKit runtimeUrl="/api/copilotkit">
  <ChatLayout>
    <ConversationList />       {/* 左侧会话列表 */}
    <CopilotChat />            {/* 右侧聊天区，内置 AG-UI 协议 */}
  </ChatLayout>
</CopilotKit>
```

### 与当前的差异

| 功能 | 当前 (Vue AguiPage) | 目标 (React CopilotKit) |
|------|---------------------|--------------------------|
| 聊天 UI | 自定义 400 行 | CopilotKit `<CopilotChat>` |
| SSE 解析 | 手动 TextDecoder + 行解析 | CopilotKit runtime 处理 |
| Markdown 渲染 | 简单 regex | `MarkdownRenderer` 统一渲染 |
| 持久化 | localStorage 手动管理 | CopilotKit 内置持久化 |
| 文件上传 | 自定义 /api/ai/upload | 通过 CopilotKit actions 或独立 endpoint |
| AG-UI 协议 | 无 | 原生支持 |

## 8. 组件库迁移映射

Element Plus → Shadcn/ui 的主要组件映射：

| Element Plus | Shadcn/ui / Radix |
|-------------|-------------------|
| el-table | Shadcn Table (TanStack Table) |
| el-dialog | Dialog (Radix) |
| el-form + el-form-item | Form (React Hook Form + Zod) |
| el-button | Button |
| el-input / el-input-number | Input |
| el-select | Select (Radix) |
| el-dropdown | Dropdown Menu (Radix) |
| el-tag | Badge |
| el-empty | 自定义 EmptyState |
| el-skeleton | Skeleton |
| el-message | Toast (Sonner) |
| el-descriptions | 自定义 Descriptions 组件 |
| el-icon | Lucide React icons |

## 9. 迁移执行范围

### 保留不变
- Go 后端所有代码和 API 接口
- 数据库 schema
- 课程内容包（content/ 目录）
- K8s 工作区机制
- 认证 cookie-session 机制

### 需要重写
- 所有前端页面（9 个）
- API 层（lib/api.ts）
- 状态管理（composables → Zustand stores）
- 路由系统（Vue Router → Next.js App Router）
- Markdown 渲染方案
- AI 聊天实现

### 新增
- CopilotKit AG-UI 集成
- Next.js API Routes（BFF 层）
- Middleware 鉴权
- Tailwind CSS 配置
- Shadcn/ui 组件库

## 10. 项目初始化

### 目录位置
新前端代码建在仓库根目录 `frontend-react/`，与当前 `frontend/` 并行存在。迁移完成后删除 `frontend/`。

### 初始化命令
```bash
npx create-next-app@latest frontend-react --typescript --tailwind --eslint --app --src-dir=false --import-alias="@/*"
cd frontend-react && npx shadcn@latest init
npx shadcn@latest add button input select dialog dropdown-menu table form badge skeleton toast
npm install zustand react-markdown remark-math remark-gfm rehype-katex katex
npm install @copilotkit/react-core @copilotkit/react-ui
```

### 环境变量
```env
# .env.local (frontend-react/)
BAILIAN_API_KEY=xxx
BAILIAN_APP_ID=xxx
NEXT_PUBLIC_GO_API_URL=http://localhost:8080    # Go 后端地址
```

## 11. 迁移执行顺序

按优先级依次重写：

1. **基础骨架**：`layout.tsx`（Sidebar + Topbar）、`middleware.ts`、`lib/api.ts`、`lib/types.ts`、`stores/session.ts`
2. **登录页**：`login/page.tsx` — 验证 auth 流程可通
3. **班级列表**：`classes/page.tsx` — 首个 CRUD 页面，验证 Element Plus → Shadcn/ui 迁移模式
4. **班级成员**：`classes/[classId]/members/page.tsx`
5. **用户管理**：`admin/users/page.tsx`
6. **课程管理**：`admin/courses/page.tsx`
7. **讲义页**：`lectures/[[lectureId]]/page.tsx` + `classes/[classId]/lectures/[[lectureId]]/page.tsx` — 验证 MarkdownRenderer
8. **作业页**：`assignments/[[assignmentId]]/page.tsx` + `classes/[classId]/assignments/[[assignmentId]]/page.tsx` — 最复杂页面（iframe + workspace + 提交 + 批改），验证所有核心交互
9. **AG-UI 聊天**：`agui/page.tsx` + `app/api/copilotkit/route.ts` — 接入百炼，验证 CopilotKit 集成

## 12. 测试策略

- **API 层**（`lib/api.ts`）：Go 后端已有 `auth_test.go`、`content_test.go`、`workspace_test.go`、`gateway_test.go`，前端不再重复测试 API 逻辑
- **组件级**：React Testing Library + Vitest，关键页面（Login、AssignmentPage、ClassMembers）需覆盖核心交互路径
- **端到端**：Playwright 验证核心用户流程（登录 → 查看班级 → 打开作业 → 查看讲义）
- **AG-UI 集成测试**：手动 + CopilotKit devtools 验证百炼 API 连通性和流式响应

## 13. 风险与注意事项

1. **AG-UI 兼容性**：CopilotKit 对百炼的兼容性需在实施中验证。百炼支持 OpenAI 兼容格式，理论上 CopilotKit 的 `remoteEndpoints` 可直接配置。若遇问题，备选方案为自定义 runtime adapter
2. **Element Plus 组件迁移**：shadcn/ui 偏向无头组件库，部分高级组件（如 Table 筛选、排序）需 TanStack Table 额外实现
3. **iframe RStudio 交互**：AssignmentPage 中的 RStudio iframe、resize handle、workspace 状态轮询逻辑需完整保留
4. **文件上传**：当前 AI 聊天支持文件上传到 `/api/ai/upload`，需在 CopilotKit 方案中保留此能力
5. **样式一致性**：迁移后需确保所有页面满足 DEMAND.md 7.1 要求的 "Notion Database 风格" 统一视觉
