# HDU RIDE 全面审计与实现方案

## 审计结论

当前代码库是从 Vue 迁移到 React 的功能性教学平台，核心工作流（工作区、提交、评分、班级管理）基本可用。但与 DEMAND.md 定义的目标架构存在根本性偏差：系统围绕"文件系统内容包 + 全局角色"构建，目标需求是"数据库课程实体 + 课程范围角色 + 班级归属作业 + 在线讲义编辑"。

## 目标技术栈

- 前端：Next.js 16 (App Router) + React 19 + Tailwind CSS v4 + shadcn/ui + Zustand
- 后端：Go 1.26 + Gin + PostgreSQL (pgx) + MinIO + Kubernetes
- AI 助手：CopilotKit + 阿里云百炼 App API (自定义 SSE 代理)

## 实施策略

模块化渐进式：每个模块独立交付并验证后再进行下一个。

---

## 模块 1：数据库基础 — courses + course_members

### 新增表

```sql
CREATE TABLE courses (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name         text NOT NULL,
  code         text NOT NULL UNIQUE,
  description  text DEFAULT '',
  status       text NOT NULL DEFAULT 'active',  -- active | archived
  content_root text,
  created_by   uuid REFERENCES users(id),
  created_at   timestamptz DEFAULT now(),
  updated_at   timestamptz DEFAULT now()
);

CREATE TABLE course_members (
  course_id   uuid REFERENCES courses(id) ON DELETE CASCADE,
  user_id     uuid REFERENCES users(id) ON DELETE CASCADE,
  member_role text NOT NULL,  -- 'admin' | 'teacher'
  joined_at   timestamptz DEFAULT now(),
  invited_by  uuid REFERENCES users(id),
  PRIMARY KEY (course_id, user_id)
);
```

### 数据迁移

- 从现有 `classes.course_id` 提取唯一课程标识，自动创建 courses 行
- `classes.course_id` 改为 FK → `courses.id`
- 现有课程的班级创建者自动成为该课程的 teacher

### 后端文件

- `backend/app/models.go` — 新增 Course、CourseMember 结构体
- `backend/app/db.go` — 新增 DDL 迁移逻辑

---

## 模块 2：权限模型重构

### 规则变更

| 操作 | 旧规则 | 新规则 |
|------|---------|---------|
| 创建课程 | 无此概念 | root 专属 |
| 管理课程（成员/内容） | 无此概念 | 课程 admin |
| 创建班级 | teacher 全局角色 | 课程 teacher 或课程 admin |
| 班级管理 | created_by | 班级所属课程的 teacher/admin |
| 助教 | 全局角色 + 班级标签 | 仅班级标签 `class_members.member_role = 'assistant'` |

### 后端变更

- `backend/app/auth.go` — 新增 `isCourseAdmin()`、`isCourseTeacher()`、`canCreateClassInCourse()`
- `backend/app/routes.go` — 所有权限检查点更新
- 移除 `users.role = 'assistant'` 作为全局角色的使用
- `setClassMemberRole` 不再修改 `users.role`

---

## 模块 3：课程管理 API + 前端

### 后端新增端点

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| GET | /api/admin/courses | root/admin/teacher | 列出用户可访问的课程 |
| POST | /api/admin/courses | root | 创建课程 |
| PATCH | /api/admin/courses/:id | course admin | 编辑课程信息 |
| DELETE | /api/admin/courses/:id | root | 归档课程 |
| GET | /api/admin/courses/:id/members | course admin | 列出课程成员 |
| POST | /api/admin/courses/:id/members | course admin | 添加成员 |
| DELETE | /api/admin/courses/:id/members/:userId | course admin | 移除成员 |

### 前端新增页面

- `/admin/courses` — 课程列表（表格：名称、代码、状态、成员数、班级数、操作）、创建/编辑对话框
- `/admin/courses/[courseId]/members` — 课程成员管理（添加教师/管理员、移除）

### 侧边栏更新

- "管理" 下拉子菜单：用户管理、课程管理、课程内容

---

## 模块 4：班级教师管理

### 后端变更

- `POST /api/classes` 新增必填 `teacherIds` 字段
- `GET /api/classes/:classId/teachers` — 列出教师
- `POST /api/classes/:classId/teachers` — 添加教师
- `DELETE /api/classes/:classId/teachers/:userId` — 移除教师
- `POST /api/classes/bulk` 新增 `action: 'add-teachers'`

### 前端变更

- 创建班级对话框新增教师选择器
- 班级成员页面新增"教师"标签页

---

## 模块 5：作业入库

### 新增表 / 后端

```sql
ALTER TABLE submissions ADD COLUMN assignment_db_id uuid;
-- 新增 assignments 表用于存储班级级别作业元数据
```

- 作业数据从课程内容初始化，教师可覆盖
- `POST /api/classes/:classId/assignments` — 创建作业
- `PATCH /api/classes/:classId/assignments/:id` — 编辑作业（标题、截止日期、镜像、starter）

### 前端变更

- 作业列表新增"新建作业"按钮（教师/admin 可见）
- 创建/编辑作业对话框

---

## 模块 6：讲义编辑器

### 后端新增端点

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/admin/courses/:id/lectures | 列出章节树 |
| POST | /api/admin/courses/:id/lectures | 新建章节/讲义 |
| PATCH | /api/admin/courses/:id/lectures/:lectureId | 重命名/移动/排序 |
| DELETE | /api/admin/courses/:id/lectures/:lectureId | 删除 |
| PATCH | /api/admin/courses/:id/lectures/:lectureId/content | 编辑 Markdown 内容 |

### 前端新增页面

- `/admin/courses/[courseId]/lectures` — 章节树 + Markdown 编辑器（Monaco Editor）

---

## 模块 7：质量收尾

### 修复清单

- 危险操作使用 shadcn AlertDialog 替代 `confirm()`
- 成员导入：已存在账号不覆盖密码
- API 集成测试覆盖所有 CRUD 端点
- 空状态/错误状态/权限提示统一完善
- 清理旧代码：`deploy/docker/nginx.conf`、旧 `.env` 中的 `API_KEY`/`APP_ID`

### 测试目标

- 每个模块的 API 端点至少覆盖：成功路径、权限拒绝、参数校验失败
- 目标：后端测试覆盖率 > 60%

---

## 依赖关系

```
模块 1 (数据库) → 模块 2 (权限) → 模块 3 (课程管理)
                                 → 模块 4 (班级教师)
                                 → 模块 5 (作业入库)
                                 → 模块 6 (讲义编辑器)
模块 7 (质量) → 所有模块完成后执行
```
