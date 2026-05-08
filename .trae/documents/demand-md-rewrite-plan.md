# `DEMAND.md` 补全与润色计划

## Summary

- 目标：基于用户已写的 `d:\Go\HDU-RIDE\DEMAND.md`、当前仓库实现现状以及已形成的 `CURRENT_DESIGN.md`，重写并补全为一份“可直接指导 AI 实现”的详尽需求书。
- 输出定位：不是单纯润色文字，而是把业务目标、数据模型变更、权限规则、页面功能、正确反馈、范围边界写成一份决策闭合的需求文档。
- 已确认的关键产品决策：
  - 课程应升级为独立业务实体
  - `admin` 应为课程级管理员，而非单纯全局角色
  - 助教仅是班级成员标签，不是全局账号类型
  - 讲义编辑器按 MVP 文件编辑器定义
  - 所有增删改查界面都纳入 Notion Database 风格要求
  - 文档以完整目标态清单输出，不做 P0/P1/P2 分期

## Current State Analysis

### 现有需求稿现状

- 当前 `d:\Go\HDU-RIDE\DEMAND.md` 已写出高层业务方向：
  - root 创建课程并指定 admin
  - admin 属于课程
  - teacher 加入课程并创建班级
  - 助教是班级标签，不是账号类型
  - 学生按课程查看教案、按班级完成作业
- 当前稿件的问题：
  - 结构未闭合，仅有 Root 章节开头，`Admin/Teacher/Student` 几乎未写
  - 关键对象未统一定义，例如课程、课程成员、班级教师、讲义、作业之间的关系
  - 缺少“正确反馈”规范，无法指导接口和前端提示设计
  - 缺少页面、权限、状态、异常、验收标准等可执行细节
  - 存在与当前实现冲突但尚未正式落地成需求规则的地方，例如现有全局 `admin/assistant` 角色模型

### 当前实现与目标需求的主要差异

- 当前实现中：
  - 课程不是数据库实体，只是 `classes.course_id` 绑定内容包 ID
  - `admin` 是全局角色，不是课程级角色
  - `assistant` 既是全局角色又是班级成员角色
  - 讲义编辑仍是文件导入/重载模式，没有在线编辑器
  - 后台管理页已有基础 CRUD，但远未达到“Notion 数据库风格”
- 这些差异意味着新版 `DEMAND.md` 不能只写“加功能”，而必须明确：
  - 新的数据模型
  - 新的权限体系
  - 对旧模型的替代关系
  - 各角色在课程/班级两个层级的真实边界

### 已确认的事实来源

- 现有需求初稿：`d:\Go\HDU-RIDE\DEMAND.md`
- 当前系统现状说明：`d:\Go\HDU-RIDE\CURRENT_DESIGN.md`
- 现有角色与权限实现：`d:\Go\HDU-RIDE\backend\app\models.go`、`d:\Go\HDU-RIDE\backend\app\auth.go`
- 现有数据库 schema：`d:\Go\HDU-RIDE\backend\app\db.go`
- 现有业务接口：`d:\Go\HDU-RIDE\backend\app\routes.go`
- 现有前端页面入口：`d:\Go\HDU-RIDE\frontend\src\router.ts` 与 `frontend/src/pages/*`

## Proposed Changes

### 目标文档结构

重写后的 `DEMAND.md` 将按以下结构组织：

1. 文档目标与适用范围
   - 说明这是“优化项目 + 修复 bug + 重构业务模型”的正式需求书
   - 明确面向 AI 实现，要求描述精确、避免歧义
2. 核心业务背景与设计原则
   - 教学组织结构
   - 课程/班级/成员/讲义/作业的边界
   - “助教不是账号类型”“课程是一级实体”等总原则
3. 角色体系
   - root
   - 课程级 admin
   - teacher
   - student
   - 班级标签 assistant
4. 数据模型需求
   - 新增/重构哪些实体
   - 各实体主键、关系、约束、唯一性要求
   - 现有全局 `assistant/admin` 模型与目标模型的替代关系
5. 权限模型需求
   - 角色能做什么、不能做什么
   - 课程级权限与班级级权限的继承/隔离规则
   - 高级角色能下发哪些低级身份
6. 按角色的功能需求
   - Root
   - Admin
   - Teacher
   - Student
   - Assistant（作为班级标签能力单列说明）
7. 按业务对象的功能需求
   - 课程
   - 课程成员/教学组
   - 班级
   - 班级成员
   - 讲义
   - 作业
   - 提交与批改
8. 前端页面与交互需求
   - 菜单结构
   - 页面布局
   - Notion 风格的 CRUD 统一要求
   - 讲义在线编辑器 MVP
9. 正确反馈规范
   - 成功反馈
   - 校验失败反馈
   - 权限不足反馈
   - 空状态、加载态、危险操作确认
10. 与现有实现的差异说明
   - 哪些现有逻辑必须被替换
   - 哪些旧行为不可继续沿用
11. 验收标准
   - 业务验收
   - 页面验收
   - 权限验收
   - 正确反馈验收

### 本次将如何改写 `DEMAND.md`

- 保留用户原文中已明确表达的核心意图，但统一术语和层次。
- 把当前零散叙述重构成“实体 -> 角色 -> 功能 -> 反馈 -> 验收”的实现导向文档。
- 明确以下高风险需求点：
  - 课程必须成为独立实体
  - root 仍然唯一
  - `admin` 不再是纯全局角色，而是课程管理员
  - 助教不再作为账号类型存在，只作为班级标签存在
  - 一个用户可以在多个班级同时被标记为助教
  - 班级至少有一位授课教师
  - 课程讲义编辑器为在线 MVP 文件编辑器
  - 所有 CRUD 页面统一采用仿 Notion 数据库体验
- 为每类动作补充“正确反馈”：
  - 成功提示
  - 失败提示
  - 权限拒绝提示
  - 空数据提示
  - 二次确认提示

### 具体文件变更范围

- 主要编辑文件：
  - `d:\Go\HDU-RIDE\DEMAND.md`
- 仅作为事实依据参考的文件：
  - `d:\Go\HDU-RIDE\CURRENT_DESIGN.md`
  - `d:\Go\HDU-RIDE\backend\app\models.go`
  - `d:\Go\HDU-RIDE\backend\app\auth.go`
  - `d:\Go\HDU-RIDE\backend\app\db.go`
  - `d:\Go\HDU-RIDE\backend\app\routes.go`
  - `d:\Go\HDU-RIDE\frontend\src\router.ts`
  - `d:\Go\HDU-RIDE\frontend\src\pages\ClassHome.vue`
  - `d:\Go\HDU-RIDE\frontend\src\pages\ClassMembers.vue`
  - `d:\Go\HDU-RIDE\frontend\src\pages\LecturePage.vue`
  - `d:\Go\HDU-RIDE\frontend\src\pages\AssignmentPage.vue`
  - `d:\Go\HDU-RIDE\frontend\src\pages\AdminUsers.vue`
  - `d:\Go\HDU-RIDE\frontend\src\pages\AdminCourseImport.vue`

### 执行步骤

1. 读取 `DEMAND.md` 与 `CURRENT_DESIGN.md`，提取“用户目标”和“当前系统现状”的冲突点。
2. 重写 `DEMAND.md` 的整体目录与文风，使其从草稿升级为正式需求书。
3. 补写数据模型与权限模型章节，明确新旧模型差异。
4. 按角色和按业务对象两条线补齐功能需求，避免只写角色视角而缺少对象视角。
5. 补写前端风格与交互要求，特别是 Notion 风格 CRUD 和讲义编辑器 MVP。
6. 补写“正确反馈”章节，统一前后端行为约束。
7. 补写验收标准，使需求书可直接作为 AI 实现的输入。

## Assumptions & Decisions

- 假设用户希望这份 `DEMAND.md` 直接面向后续 AI 编码，而不是仅供人工讨论。
- 决定不沿用当前系统中的 `assistant` 全局角色语义，而在需求书中明确废弃为“班级标签式助教”。
- 决定把“课程”提升为一等实体，并围绕它重写课程成员、课程讲义和课程权限。
- 决定文档中同时写“业务逻辑”和“正确反馈”，避免后续实现时只完成功能，不完成交互结果约束。
- 决定不做需求分期，而以完整目标态输出。
- 决定保留用户原始表达的业务方向，但统一成更严谨、更可实现的术语体系。

## Verification

- 校验新计划中的所有现状判断都能在当前仓库文件中找到依据。
- 校验最终需求书会覆盖当前草稿未完成的 `Admin/Teacher/Student` 章节。
- 校验最终需求书会明确“课程实体化、课程级 admin、班级标签助教”三项核心改造。
- 校验最终需求书会包含页面、权限、数据模型、反馈、验收五类实现必需信息。
- 校验最终需求书不会把当前实现误写成目标设计，而是明确区分“现状”和“目标”。
