# HDU RIDE React 前端 — 日常维护与升级指南

本文档面向已经部署好的 HDU RIDE React 前端，讲解日常如何升级代码、更新依赖、新增页面、以及排查常见问题。

文档分工：

- 首次部署看 [REACT-ON.md](REACT-ON.md)
- 日常升级/维护看本文档
- 后端/K8s/PostgreSQL/MinIO 运维看 [INSTRUCTION.md](INSTRUCTION.md)

---

## 1. 项目结构速查

```
frontend-react/
├── app/                           # Next.js App Router 页面
│   ├── layout.tsx                 # 根布局（SessionProvider + Toaster）
│   ├── globals.css                # 所有全局样式
│   ├── login/page.tsx             # 登录页
│   ├── (authenticated)/           # 认证路由组（有 Sidebar + Topbar）
│   │   ├── layout.tsx             # 认证布局（SidebarProvider + Shell）
│   │   ├── classes/page.tsx       # 班级列表
│   │   ├── classes/[classId]/
│   │   │   ├── members/page.tsx   # 班级成员
│   │   │   ├── lectures/[[lectureId]]/page.tsx  # 班级讲义
│   │   │   └── assignments/[[assignmentId]]/page.tsx  # 班级作业
│   │   ├── lectures/[[lectureId]]/page.tsx  # 全局讲义
│   │   ├── assignments/[[assignmentId]]/page.tsx  # 全局作业
│   │   ├── admin/users/page.tsx   # 用户管理
│   │   ├── admin/courses/page.tsx # 课程导入
│   │   └── agui/page.tsx          # AI 助手（CopilotKit）
│   └── api/copilotkit/route.ts    # CopilotKit 百炼 BFF 端点
├── components/
│   ├── ui/                        # Shadcn/ui 基础组件（自动生成）
│   ├── layout/                    # 布局组件
│   │   ├── sidebar.tsx            # 侧边栏导航
│   │   ├── sidebar-context.tsx    # 侧边栏折叠状态 Context
│   │   ├── topbar.tsx             # 顶栏（用户菜单 + 修改密码）
│   │   └── authenticated-shell.tsx # 认证外壳（Topbar + Sidebar + main）
│   ├── markdown/                  # MarkdownRenderer（react-markdown + KaTeX）
│   ├── lectures/                  # 讲义查看器（章节树 + 内容渲染）
│   └── assignments/              # 作业查看器（三面板 + RStudio iframe + 批改）
├── lib/
│   ├── api.ts                     # Go 后端 API 请求封装
│   ├── types.ts                   # TypeScript 类型定义
│   └── utils.ts                   # cn() + generateId()
├── stores/
│   └── session.ts                 # Zustand 认证状态
├── providers/
│   └── session-provider.tsx       # 会话初始化 Provider
├── middleware.ts                  # 路由鉴权（Cookie 校验）
├── next.config.ts                 # Next.js 配置（含 API 代理 rewrites）
├── package.json                   # 依赖声明
└── .env.local                     # 本地环境变量（百炼密钥等）
```

---

## 2. 日常开发流程

### 2.1 启动开发服务器

```bash
cd /opt/hdu-ride/frontend-react
bun run dev
```

浏览器访问 `http://localhost:3000`。

> 如果后端在 K8s 集群内，需要先 `kubectl port-forward -n hdu-ride svc/hdu-ride-backend 8080:8080`。

### 2.2 修改后热更新

Next.js 开发服务器支持 **热模块替换（HMR）**：

- 修改 `.tsx` / `.ts` 文件 → 浏览器自动刷新
- 修改 `globals.css` → 样式即时生效
- 修改 `next.config.ts` → 需要手动重启 `bun run dev`

### 2.3 TypeScript 类型检查

提交前务必检查类型：

```bash
bun x --bun tsc --noEmit
```

零错误才能提交。

---

## 3. 添加新页面

### 3.1 在 `app/(authenticated)/` 下创建路由

Next.js App Router 是**文件系统路由**。例如要添加一个 `/settings` 页面：

```bash
mkdir -p app/\(authenticated\)/settings
```

创建 `app/(authenticated)/settings/page.tsx`：

```tsx
// app/(authenticated)/settings/page.tsx
export default function SettingsPage() {
  return (
    <section className="panel single-panel">
      <div className="panel-head">
        <h2>设置</h2>
      </div>
      <div className="p-6">
        {/* 页面内容 */}
      </div>
    </section>
  )
}
```

文件创建后路由自动生效 — 不需要手动注册路由。

### 3.2 在侧边栏添加导航项

编辑 `components/layout/sidebar.tsx`，在 `navItems` 数组中加一项：

```tsx
const navItems = [
  // ... 现有项
  { key: 'settings', label: '设置', path: '/settings', icon: Settings },
]
```

从 `lucide-react` 选择合适的图标：

```tsx
import { ..., Settings } from 'lucide-react'
```

### 3.3 页面样式约定

| 场景 | CSS 类名 | 说明 |
|------|---------|------|
| 单面板页面 | `panel single-panel` | 最大宽度 1180px，居中 |
| 双面板页面 | `page-grid` | 左侧 260px + 右侧自适应 |
| 三面板页面 | `assignment-grid` | 作业页专用，可拖拽调整宽度 |
| 面板头部 | `panel-head` | 标题 + 操作按钮区 |
| 操作按钮区 | `toolbar-actions` | 右对齐的按钮组 |
| 灰色辅助文字 | `muted` | 面板标题下方的说明文字 |

这些类全部定义在 `app/globals.css`，可直接使用。

---

## 4. 修改现有页面

### 4.1 修改 API 调用

所有后端 API 调用统一通过 `lib/api.ts`：

```tsx
import { api } from '@/lib/api'

// GET
const data = await api.get<{ classes: ClassItem[] }>('/api/classes')

// POST
await api.post('/api/classes', { name: '新班级', courseId: 'intro-r' })

// PATCH
await api.patch('/api/admin/users/123', { displayName: '新名字' })

// DELETE
await api.delete('/api/admin/users/123')

// 文件下载
const blob = await api.download('/api/path/to/file')
```

### 4.2 添加/修改 TypeScript 类型

在 `lib/types.ts` 中添加新接口。**注意**：类型名和字段名必须与 Go 后端返回的 JSON 字段一致（小驼峰）。

```typescript
// 示例：添加新类型
export interface NewFeature {
  id: string
  name: string
  createdAt: string
}
```

### 4.3 使用 Zustand Store 管理状态

当前只有一个 Store：`stores/session.ts`。如需添加全局状态：

```typescript
// stores/new-feature.ts
import { create } from 'zustand'

interface NewFeatureState {
  data: SomeType[]
  loading: boolean
  fetchData: () => Promise<void>
}

export const useNewFeature = create<NewFeatureState>((set) => ({
  data: [],
  loading: false,
  fetchData: async () => {
    set({ loading: true })
    const result = await api.get<{ items: SomeType[] }>('/api/path')
    set({ data: result.items, loading: false })
  },
}))
```

页面中使用（注意用 selector 避免不必要渲染）：

```tsx
const data = useNewFeature((s) => s.data)
const fetchData = useNewFeature((s) => s.fetchData)
```

---

## 5. 更新依赖

### 5.1 检查过期依赖

```bash
cd frontend-react
bun outdated
```

### 5.2 更新所有依赖

```bash
bun update
```

### 5.3 更新特定依赖

```bash
bun update @copilotkit/react-core @copilotkit/react-ui @copilotkit/runtime
```

### 5.4 更新后的验证步骤

```bash
# 1. 类型检查
bun x --bun tsc --noEmit

# 2. 构建检查
bun run build

# 3. 启动验证
bun run dev
# 手动检查登录、讲义（Markdown 渲染）、AI 助手等功能
```

---

## 6. CopilotKit / 百炼 AI 维护

### 6.1 切换百炼模型

编辑 `app/api/copilotkit/route.ts`：

```typescript
const openai = new OpenAI({
  apiKey: process.env.BAILIAN_API_KEY!,
  baseURL: 'https://dashscope.aliyuncs.com/compatible-mode/v1',
  defaultHeaders: {
    'X-DashScope-AppId': process.env.BAILIAN_APP_ID!,
  },
})

const serviceAdapter = new OpenAIAdapter({
  openai,
  model: 'qwen-plus',    // 可改为 qwen-turbo, qwen-max, qwen-plus-latest 等
})
```

百炼支持的模型：`qwen-turbo`（快/便宜）、`qwen-plus`（均衡）、`qwen-max`（最强）。

### 6.2 调试 AI 响应

开启 CopilotKit 调试模式（开发环境）：

在 `app/(authenticated)/agui/page.tsx` 中临时添加：

```tsx
<CopilotKit runtimeUrl="/api/copilotkit" publicApiKey="ck_pub_...">
```

或查看 Next.js 服务端日志：

```bash
# 查看 /api/copilotkit 的请求日志
curl -X POST http://localhost:3000/api/copilotkit \
  -H "Content-Type: application/json" \
  -d '{"messages":[{"role":"user","content":"你好"}]}'
```

### 6.3 百炼 API Key 轮换

1. 在阿里云百炼控制台生成新 Key
2. 更新 `frontend-react/.env.local`（开发）或 K8s Secret（生产）
3. 重启前端服务

```bash
# 开发环境
bun run dev

# 生产环境（Docker）
docker restart hdu-ride-frontend

# 生产环境（K8s）
kubectl rollout restart deployment/hdu-ride-frontend -n hdu-ride
```

---

## 7. 样式修改

### 7.1 Shadcn/ui 主题

在 `app/globals.css` 中通过 CSS 变量调整主题色：

```css
@layer base {
  :root {
    --primary: 222.2 47.4% 11.2%;
    /* Shadcn/ui 主题变量 */
  }
}
```

### 7.2 修改全局样式

业务相关样式全部在 `app/globals.css` 中，使用传统 CSS（非 Tailwind 原子类）。原因是这些样式从 Vue 版精确迁移而来，保持一致性。

- `.panel` — 白色卡片容器
- `.panel-head` — 面板标题栏
- `.markdown` — Markdown 内容排版
- `.assignment-grid` — 三面板作业布局
- 等等

修改这些样式后，开发服务器会**即时热更新**。

### 7.3 添加 Shadcn/ui 组件

```bash
npx shadcn@latest add <component-name>
```

常用组件：`accordion`, `alert-dialog`, `card`, `popover`, `tabs`, `tooltip`。

---

## 8. 生产部署升级流程

### 8.1 代码更新

```bash
cd /opt/hdu-ride
git pull
```

### 8.2 Docker 升级（推荐）

```bash
cd /opt/hdu-ride

# 重新构建镜像
docker build -t hdu-ride-frontend:latest -f deploy/docker/frontend.Dockerfile .

# 重启容器
docker stop hdu-ride-frontend
docker rm hdu-ride-frontend
docker run -d \
  --name hdu-ride-frontend \
  --network host \
  -e BAILIAN_API_KEY=sk-xxx \
  -e BAILIAN_APP_ID=xxx \
  -e NEXT_PUBLIC_GO_API_URL=http://localhost:8080 \
  hdu-ride-frontend:latest
```

### 8.3 Kubernetes 升级

```bash
# 构建 + 推送镜像（根据实际镜像仓库调整）
docker build -t your-registry/hdu-ride-frontend:v1.1 -f deploy/docker/frontend.Dockerfile .
docker push your-registry/hdu-ride-frontend:v1.1

# 更新 Deployment 镜像
kubectl set image deployment/hdu-ride-frontend \
  frontend=your-registry/hdu-ride-frontend:v1.1 \
  -n hdu-ride

# 等待滚动更新完成
kubectl rollout status deployment/hdu-ride-frontend -n hdu-ride
```

### 8.4 零停机部署

Next.js standalone 模式支持优雅关闭。K8s 默认滚动更新策略（`maxUnavailable: 25%`）确保升级过程中始终有 Pod 在服务。

---

## 9. 常见问题排查

### 9.1 页面空白 / 白屏

**可能原因**：JavaScript 报错导致 React 未挂载。

**排查**：

```bash
# 开发模式看终端输出
bun run dev

# 生产模式看容器日志
docker logs hdu-ride-frontend

# 浏览器 F12 → Console 查看 JS 报错
```

常见根因：
- `.env.local` 中百炼 API Key 缺失导致构建期报错
- 后端 API 不可达导致 `fetchSession` 无限重试

### 9.2 登录后仍然跳回登录页

**原因**：Cookie 未正确设置（Go 后端设置 `session_token` cookie，前端 middleware 检查该 cookie）。

**排查**：
- Go 后端是否在运行且 `/api/login` 返回了 `Set-Cookie` 头
- 前端 `lib/api.ts` 中 `credentials: 'include'` 是否生效
- 浏览器 DevTools → Application → Cookies 检查 `session_token` 是否存在

### 9.3 Markdown / LaTeX 公式不渲染

**原因**：KaTeX CSS 未加载或 remark/rehype 插件未生效。

**排查**：

1. 确认 `app/globals.css` 中有 `@import 'katex/dist/katex.min.css';`
2. 确认 `components/markdown/MarkdownRenderer.tsx` 中插件配置正确
3. 确认内容中的公式格式正确：
   - 行内公式：`$E = mc^2$`
   - 块级公式：`$$\hat{\beta} = (X^TX)^{-1}X^Ty$$`
   - 对齐环境：需要用 `\begin{aligned}` 包裹

### 9.4 AI 助手无响应

**原因**：百炼 API 连接失败。

**排查步骤**：

1. 检查 `.env.local` 中的密钥是否正确
2. 直接测试百炼 API：

```bash
curl -X POST https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions \
  -H "Authorization: Bearer $BAILIAN_API_KEY" \
  -H "X-DashScope-AppId: $BAILIAN_APP_ID" \
  -H "Content-Type: application/json" \
  -d '{"model":"qwen-plus","messages":[{"role":"user","content":"你好"}]}'
```

3. 检查 CopilotKit 服务端日志（`bun run dev` 终端输出）

### 9.5 RStudio iframe 无法加载

**原因**：Go 后端的工作区代理未正确转发。

**排查**：
- Next.js rewrites 将 `/ide/*` 代理到 Go 后端（`next.config.ts`）
- Go 后端在 K8s 中创建 Pod/PVC/Service 后，工作区需要 10-30 秒就绪
- 前端 `waitForGateway()` 会重试 30 次（每次 700ms），超时后提示 "RStudio 尚未就绪"

### 9.6 修改代码后页面不更新

**开发模式**：确认 `bun run dev` 正在运行，HMR 应自动生效。

**生产模式**：必须重新构建 + 重启服务：

```bash
bun run build && bun run start
```

---

## 10. 安全注意事项

1. **`.env.local` 绝不提交到 Git**（已在 `.gitignore` 中）
2. **百炼 API Key** 在前端代码中不可见，只在 Next.js 服务端 `app/api/copilotkit/route.ts` 中使用
3. **Go 后端 API** 通过 Next.js rewrites 代理，不直接暴露给浏览器
4. **生产环境**建议使用 K8s Secrets 管理敏感配置，而非 `.env.local` 文件
5. **定期轮换**百炼 API Key

---

## 11. 快速参考

| 操作 | 命令 |
|------|------|
| 启动开发 | `cd frontend-react && bun run dev` |
| 类型检查 | `bun x --bun tsc --noEmit` |
| 生产构建 | `bun run build` |
| 生产启动 | `bun run start` |
| 安装新依赖 | `bun add <package>` |
| 更新依赖 | `bun update` |
| 添加 shadcn 组件 | `npx shadcn@latest add <name>` |
| Docker 构建 | `docker build -t hdu-ride-frontend -f deploy/docker/frontend.Dockerfile .` |
| 查看运行日志 | `docker logs hdu-ride-frontend` |
| K8s 重启 | `kubectl rollout restart deployment/hdu-ride-frontend -n hdu-ride` |
