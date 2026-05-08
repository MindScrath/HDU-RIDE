# 当前系统数据模型与功能设计梳理计划

## Summary

- 目标：在编写后续 `DEMAND.md` 之前，先基于当前仓库实现产出一份“现状导向”的系统说明，覆盖数据模型设计、功能设计、权限边界、核心业务流与明显的结构性问题。
- 输出用途：作为需求前置分析材料，不只盘点已有实现，还要提炼后续优化和修复需求的切入点。
- 输出粒度：以业务模块级梳理为主，必要时补充“页面/API/数据表”的关键映射关系，但不展开成逐接口逐字段字典。

## Current State Analysis

### 仓库与架构边界

- 项目是一个教学平台，整合课程内容、作业提交、评分、班级管理以及按学生/作业创建的 RStudio 工作区，定位见 `README.md`。
- 技术架构为：
  - 后端：Go + Gin + PostgreSQL，核心业务位于 `backend/app/`
  - 前端：Vue 3 + Vite + Element Plus，页面与状态位于 `frontend/src/`
  - 课程内容：文件式 Markdown/YAML，位于 `content/courses/`
  - 工作区：通过 Kubernetes 创建 RStudio Pod/PVC/Service，并经 `/ide/s/:workspaceID/` 反向代理访问

### 数据模型现状

- 持久化模型集中在 `backend/app/db.go` 的内联 SQL 中，没有独立 migration 目录，也没有 ORM。
- 主要业务实体定义在 `backend/app/models.go`，包含：
  - `User`
  - `Class`
  - `Submission`
  - `Grade`
  - `Workspace`
  - `Event`
- 实际数据库表共 8 张，位于 `backend/app/db.go`：
  - `users`
  - `sessions`
  - `classes`
  - `class_members`
  - `submissions`
  - `grades`
  - `workspaces`
  - `events`
- 数据模型呈现出三层来源：
  - 关系型持久化：用户、班级、成员关系、提交、评分、工作区、会话、事件
  - 文件型课程域模型：`CourseBundle`、`ChapterMeta`、`LectureMeta`、`AssignmentMeta`，位于 `backend/app/content.go`
  - 前端消费类型：位于 `frontend/src/types.ts`，是后端响应的简化投影

### 功能设计现状

- 后端统一路由入口在 `backend/app/routes.go`，功能按以下模块组织：
  - 认证与会话
  - 班级管理
  - 班级成员管理
  - 讲义与作业内容读取
  - 作业提交、提交预览、评分、成绩导出与发布
  - RStudio 工作区创建、恢复、停止、心跳
  - 用户管理与课程导入/重载
- 前端路由集中在 `frontend/src/router.ts`，页面主要包括：
  - 登录
  - 班级首页
  - 班级成员
  - 讲义页
  - 作业页
  - 用户管理
  - 课程导入
- 顶部导航与权限感知 UI 在 `frontend/src/App.vue`，会话状态与角色判断在 `frontend/src/composables/useSession.ts`。

### 权限与业务约束现状

- 全局角色定义于 `backend/app/models.go`：`root`、`admin`、`teacher`、`assistant`、`student`。
- 权限判断集中于 `backend/app/auth.go`，核心规则包括：
  - `root/admin` 拥有后台管理能力
  - `teacher` 可创建并管理自己创建的班级
  - `assistant/student` 只能访问自己加入的班级
  - `assistant` 可在其被标记为助教的班级执行评分相关动作
- 班级成员关系与全局角色存在耦合：
  - `class_members.member_role` 管理班级内身份
  - 班级助教会同步影响 `users.role` 在 `student/assistant` 间切换

### 已识别的需求前置关注点

- 数据定义分散：数据库表、Go struct、文件课程模型、前端 TypeScript 类型分别存在，存在后续需求梳理时口径不统一风险。
- 课程内容不在数据库中：课程、章节、作业元数据来自文件系统，业务设计需要明确“数据库域”和“内容域”是两套模型。
- 权限是规则驱动而非策略配置：角色与班级成员关系交织，后续需求应重点审视权限边界和可维护性。
- 工作区是外部基础设施资源映射：`workspaces` 只是 Kubernetes 资源记录，不等同于完整业务状态机。

## Proposed Changes

### 计划产出一份现状分析文档，结构如下

1. 项目目标与系统边界
   - 总结平台服务对象、核心业务目标、系统外部依赖
2. 当前数据模型设计
   - 区分“数据库模型”“课程内容模型”“运行时/外部资源模型”
   - 给出主要实体、关键字段、实体关系与约束
   - 明确哪些数据在 PostgreSQL、哪些在对象存储、哪些在文件系统、哪些映射到 Kubernetes
3. 当前功能设计
   - 按业务模块梳理：认证、用户管理、班级、成员、课程内容、作业、提交、评分、工作区、课程导入
   - 每个模块说明：目标用户、主要页面/API、核心业务流程、权限边界、关键状态变化
4. 当前系统的结构性问题与需求切入点
   - 提炼对后续 `DEMAND.md` 有价值的问题，不直接给实现方案
   - 例如模型分散、权限耦合、状态表达不完整、内容域与业务域边界模糊等

### 产出时将重点引用的实际文件

- 项目概览：
  - `README.md`
- 后端核心：
  - `backend/app/models.go`
  - `backend/app/db.go`
  - `backend/app/auth.go`
  - `backend/app/routes.go`
  - `backend/app/content.go`
  - `backend/app/workspace.go`
  - `backend/app/gateway.go`
- 前端核心：
  - `frontend/src/router.ts`
  - `frontend/src/App.vue`
  - `frontend/src/composables/useSession.ts`
  - `frontend/src/types.ts`
  - `frontend/src/pages/ClassHome.vue`
  - `frontend/src/pages/ClassMembers.vue`
  - `frontend/src/pages/LecturePage.vue`
  - `frontend/src/pages/AssignmentPage.vue`
  - `frontend/src/pages/AdminUsers.vue`
  - `frontend/src/pages/AdminCourseImport.vue`
- 内容样例：
  - `content/courses/intro-r/course.yml`
  - `content/courses/intro-r/assignments/hw01/assignment.yml`

### 执行步骤

1. 继续补读上述关键文件，完成所有主要业务模块与数据边界的事实确认。
2. 输出“当前数据模型设计”章节：
   - 梳理实体与关系
   - 区分存储位置与生命周期
   - 说明角色、成员关系、提交与评分、工作区映射
3. 输出“当前功能设计”章节：
   - 以业务模块为主线整理页面、API、权限、状态流
   - 补充跨模块主流程，如“学生完成作业”和“教师批改作业”
4. 输出“问题与需求切入点”章节：
   - 仅总结从当前实现可直接观察到的结构性问题
   - 为后续 `DEMAND.md` 提供需求来源

## Assumptions & Decisions

- 假设最终交付物是给项目维护/产品设计使用的内部分析文档，而不是对外产品说明。
- 决定以“现状说明 + 问题抽取”为主，不直接进入未来方案设计。
- 决定以业务模块组织功能设计，不以路由列表或数据库表清单替代设计说明。
- 决定把课程内容文件系统与数据库持久层分开描述，避免把文件内容误写成数据库模型。
- 决定把对象存储与 Kubernetes 资源纳入“数据/资源模型”说明，因为它们参与提交与工作区核心业务。

## Verification

- 校验所有涉及的实体、表名、角色、接口分组和页面路径都来自实际仓库文件，不凭空补充。
- 校验每个业务模块都能落回到至少一个前端页面或后端路由入口。
- 校验“问题与需求切入点”只基于现有代码结构和已知行为推导，不提前写解决方案。
- 校验最终文档可直接作为后续 `DEMAND.md` 的输入材料，帮助定义优化目标、边界和优先级。
