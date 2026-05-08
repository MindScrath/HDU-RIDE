# HDU RIDE 当前系统设计说明

本文档用于在编写后续 `DEMAND.md` 之前，先梳理当前系统已经存在的数据模型设计、功能设计与主要结构性问题。内容完全基于当前仓库实现，不预设未来方案。

## 1. 系统定位与边界

HDU RIDE 是一个面向课程教学场景的教学平台，核心目标是把以下能力整合到同一个系统内：

- 课程内容发布：讲义与作业以 Markdown/YAML 课程包形式维护
- 班级教学组织：班级创建、班级成员管理、助教分配
- 作业教学闭环：学生进入作业、打开 RStudio、提交、教师查看、评分、发布成绩
- 在线实践环境：按学生和作业维度创建独立 RStudio 工作区

当前系统不是传统“全数据库驱动”的 LMS，而是由四类存储/运行域共同构成：

- PostgreSQL：保存业务主数据和业务关系
- 文件系统 `content/`：保存课程、章节、作业元数据与 Markdown 内容
- 对象存储：保存提交文本和提交时归档出来的工作区压缩包
- Kubernetes：承载每个工作区对应的 Pod、Service、PVC、NetworkPolicy

这意味着后续需求设计不能只盯数据库表，必须把“课程内容域”“对象存储域”“工作区资源域”一起看。

## 2. 当前数据模型设计

## 2.1 模型分层

当前系统的数据模型可以分为四层：

### A. 业务持久化模型

存储在 PostgreSQL 中，描述用户、班级、提交、评分、工作区记录和会话状态。

### B. 课程内容模型

来自 `content/courses/*` 目录下的 `course.yml`、`assignment.yml`、Markdown 文件，描述课程结构和作业元数据。

### C. 提交产物模型

存储在对象存储中，包括学生文本答案和归档后的工作区文件。

### D. 运行时资源模型

由 Kubernetes 中的 Pod、PVC、Service、NetworkPolicy 组成，数据库中的 `workspaces` 只是这些资源的索引记录。

## 2.2 PostgreSQL 数据模型

当前 schema 在后端启动时由内联 SQL 初始化，不使用独立 migration 系统。数据库实际有 8 张表。

### 1. `users`

用途：系统全局用户主表。

关键字段：

- `id`：用户主键
- `username`：登录账号，唯一
- `display_name`：展示姓名
- `password_hash`：密码哈希
- `role`：全局角色，取值为 `root/admin/teacher/assistant/student`
- `status`：账号状态，取值为 `active/disabled`
- `created_at`：创建时间

设计含义：

- `role` 是全局权限入口，不是班级内角色
- 用户可用于后台管理、班级教学、学生学习三类场景
- `disabled` 用户无法继续通过 session 使用系统

### 2. `sessions`

用途：登录态与会话续存。

关键字段：

- `token_hash`：session token 哈希，主键
- `user_id`：关联用户
- `expires_at`：过期时间
- `created_at`：创建时间

设计含义：

- 服务端只存 token 哈希，不直接存原始 token
- 鉴权依赖 cookie + 数据库 session 校验
- 禁用用户或重置密码时会清理相关 session

### 3. `classes`

用途：班级主表。

关键字段：

- `id`：班级主键
- `course_id`：绑定课程包 ID
- `name`：班级名
- `term`：学期
- `note`：备注
- `created_by`：创建者用户 ID
- `created_at`：创建时间

设计含义：

- 班级是“课程内容域”与“教学组织域”的绑定点
- 一个班级只绑定一个课程包
- 教师对班级的管理权限由 `created_by` 决定

### 4. `class_members`

用途：班级成员关系表。

关键字段：

- `class_id`
- `user_id`
- `member_role`：班级内角色，取值为 `student/assistant`
- `joined_at`

设计含义：

- 这是学生和助教进入班级的真实入口
- `member_role` 表示班级内身份，不等于 `users.role`
- 学生是否能看到作业、能否创建工作区，首先取决于是否在这张表里

### 5. `submissions`

用途：作业提交记录。

关键字段：

- `id`
- `class_id`
- `assignment_id`
- `user_id`
- `text_object`：文本答案对象存储路径
- `file_object`：工作区归档对象存储路径
- `attempt`：第几次提交
- `late`：是否晚交
- `created_at`

设计含义：

- 一个提交对应一个学生在某班级某作业的一次提交尝试
- 系统不把作业文件直接存数据库，而是存对象路径
- 提交行为依赖已有工作区，并在提交时触发工作区归档

### 6. `grades`

用途：提交评分表。

关键字段：

- `id`
- `submission_id`：唯一，表示一条提交最多一条评分
- `score`
- `comment`
- `grader_id`
- `published_at`
- `updated_at`

设计含义：

- 评分是对具体 submission 的附属记录，而不是对 assignment 的直接评分
- 采用“保存评分”和“发布成绩”一体化流程，但仍保留 `published_at` 字段区分是否公开
- 当前模型天然支持历史提交分别评分，但前端主要聚焦每个学生最近一次提交

### 7. `workspaces`

用途：在线工作区记录表。

关键字段：

- `id`
- `user_id`
- `class_id`
- `assignment_id`
- `pod_name`
- `service_name`
- `pvc_name`
- `status`
- `last_seen_at`
- `created_at`

设计含义：

- 这张表保存的是工作区资源索引，不是完整运行时状态机
- 实际运行状态仍需通过 Kubernetes 资源存在性与就绪性判断
- 一个用户在某班级某作业上理论上只有一个当前活跃工作区，旧工作区用 `status='stopped'` 标记

### 8. `events`

用途：行为审计日志。

关键字段：

- `id`
- `user_id`
- `action`
- `target`
- `created_at`

设计含义：

- 记录登录、建班、导入成员、评分、发布成绩、工作区启动等行为
- 当前更像审计流水，不是完整业务事件总线

## 2.3 课程内容模型

课程内容不在数据库中，而是文件系统模型。

### 课程层

`course.yml` 定义：

- `id`
- `title`
- `description`
- `chapters`
- `assignments`

系统会把它加载为 `CourseBundle`。

### 章节层

章节由 `ChapterMeta` 与 `LectureMeta` 表达：

- chapter：`id/title/order/sections`
- lecture：`id/file/title/order`

讲义正文来自 Markdown 文件，运行时渲染为 HTML。

### 作业层

作业由 `assignment.yml` 定义：

- `id`
- `title`
- `open_at`
- `due_at`
- `rstudio_image`
- `starter`
- `submit_path`

系统会把它加载为 `AssignmentMeta`。

设计含义：

- 课程、章节、作业元数据是“内容包驱动”
- 班级通过 `course_id` 绑定课程内容
- 作业发布时间和截止时间直接来自文件，不来自数据库
- 作业使用的 RStudio 镜像也属于内容配置的一部分

## 2.4 对象存储模型

对象存储当前主要承担两类数据：

- 文本答案：`text_object`
- 工作区归档：`file_object`

归档的本质是对学生当前 RStudio 工作区目录进行打包。也就是说：

- 数据库只知道“对象路径”
- 真正的提交内容在对象存储
- 教师复核历史提交时，会以归档对象为输入恢复出可查看的工作区

## 2.5 Kubernetes 工作区资源模型

工作区创建时会生成一组实际资源：

- PVC：保存学生 RStudio home 目录
- Pod：运行 RStudio
- Service：供网关转发访问
- NetworkPolicy：限制网络访问

数据库中的 `workspaces` 与真实资源的映射关系如下：

- `pod_name` -> Pod
- `service_name` -> Service
- `pvc_name` -> PVC

设计含义：

- 业务上叫“工作区”，技术上是“一组 K8s 资源”
- 数据库并不保存工作区内文件内容
- 作业 starter/data/tests 是由 init container 从课程内容卷复制到 `/home/rstudio/workspace/<assignmentID>`

## 2.6 关键实体关系

可以把当前核心关系理解为：

- 一个 `User` 可以创建多个 `Class`
- 一个 `Class` 绑定一个 `course_id`
- 一个 `Class` 通过 `class_members` 拥有多个学生/助教
- 一个 `User` 在某个 `Class` 的某个 `Assignment` 上可以有多次 `Submission`
- 一个 `Submission` 最多对应一个 `Grade`
- 一个 `User` 在某个 `Class + Assignment` 上可以拥有一个当前活跃 `Workspace`
- 一个 `Class` 所属课程的讲义与作业元数据来自文件系统，而不是数据库

## 2.7 当前数据模型的主要特点

### 优点

- 结构直接，便于快速定位业务数据
- 课程内容与业务数据解耦，便于教师以课程包方式维护教学内容
- 工作区与提交记录分开，支持“在线做题”和“历史复核”

### 局限

- 模型定义分散在 SQL、Go struct、YAML、TS 类型四处，维护成本高
- `users.role` 与 `class_members.member_role` 存在耦合，容易引入权限歧义
- `workspaces.status` 表达能力有限，无法完整覆盖创建中、运行中、失效、恢复失败等状态
- 课程内容不进数据库后，许多业务规则依赖文件系统加载结果，导致业务域边界不够直观

## 3. 当前功能设计

当前功能可以按九个业务模块理解。

## 3.1 认证与会话

目标用户：

- 所有系统用户

核心能力：

- 用户名密码登录
- 基于 cookie 的 session 保持
- 获取当前登录用户
- 退出登录
- 修改本人密码

主要后端接口：

- `POST /api/login`
- `GET /api/session`
- `POST /api/logout`
- `PATCH /api/me/password`

主要前端入口：

- 登录页
- 顶部用户菜单

设计特点：

- 认证完全依赖后端 session 表，不使用 JWT
- 修改密码后会清除同一用户的其他 session
- `disabled` 用户无法继续通过鉴权

## 3.2 用户管理

目标用户：

- `root`
- `admin`

核心能力：

- 新建用户
- 修改显示名、角色、状态
- 重置用户密码
- 批量禁用/启用/改角色

主要后端接口：

- `GET /api/admin/users`
- `POST /api/admin/users`
- `POST /api/admin/users/bulk`
- `PATCH /api/admin/users/:id`
- `DELETE /api/admin/users/:id`
- `POST /api/admin/users/:id/password`

主要前端入口：

- 管理后台 `用户管理`

设计特点：

- `root` 与 `admin` 有明显层级差异
- `admin` 不能管理 `root`
- 删除用户当前并非物理删除，而是把状态改为 `disabled`

## 3.3 班级管理

目标用户：

- `root/admin/teacher`

核心能力：

- 查看可访问班级
- 创建班级
- 删除班级

主要后端接口：

- `GET /api/classes`
- `POST /api/classes`
- `POST /api/classes/bulk`
- `GET /api/classes/:classID`
- `DELETE /api/classes/:classID`

主要前端入口：

- 班级首页

设计特点：

- `teacher` 只能看和管理自己创建的班级
- `assistant/student` 只能看到自己加入的班级
- 班级删除会级联删除成员关系、提交、成绩，并停止相关工作区

## 3.4 班级成员管理

目标用户：

- `root/admin/teacher`

核心能力：

- 查看班级成员
- 导入学生
- 设为助教/学生
- 移除成员
- 重置学生密码

主要后端接口：

- `GET /api/classes/:classID/members`
- `POST /api/classes/:classID/members/import`
- `POST /api/classes/:classID/members/bulk`
- `DELETE /api/classes/:classID/members/:userID`
- `POST /api/classes/:classID/members/:userID/password`
- `POST /api/classes/:classID/assistants`

主要前端入口：

- 班级成员页

设计特点：

- 学生导入会自动 upsert 用户账号
- 学生能否真正进入教学流程，取决于是否在 `class_members` 中
- 班级助教身份会反向影响全局 `users.role`

## 3.5 课程内容与讲义

目标用户：

- 所有已登录用户

核心能力：

- 读取默认课程讲义
- 按班级读取绑定课程讲义
- 渲染 Markdown 讲义

主要后端接口：

- `GET /api/lectures`
- `GET /api/lectures/:lectureID`
- `GET /api/classes/:classID/lectures`
- `GET /api/classes/:classID/lectures/:lectureID`

主要前端入口：

- 讲义页

设计特点：

- 全局讲义模式会落到默认课程
- 班级讲义模式按班级绑定的 `course_id` 取内容
- 讲义 HTML 在后端渲染和净化后返回前端

## 3.6 作业浏览与学生提交

目标用户：

- 学生为主
- 教师/助教可查看同一页面的另一种形态

核心能力：

- 查看作业列表和题面
- 打开作业对应的 RStudio 工作区
- 提交当前工作区
- 查看个人提交次数、最近提交时间和成绩状态

主要后端接口：

- `GET /api/classes/:classID/assignments`
- `GET /api/classes/:classID/assignments/:assignmentID`
- `POST /api/classes/:classID/assignments/:assignmentID/workspace`
- `POST /api/classes/:classID/assignments/:assignmentID/submit`

主要前端入口：

- 作业页学生模式

核心流程：

1. 学生进入班级作业
2. 页面读取作业题面和当前个人状态
3. 学生点击“打开 RStudio”，系统创建或复用工作区
4. 学生在工作区完成作业
5. 学生点击提交，系统归档当前工作区并写入 `submissions`
6. 页面刷新后展示新的提交状态

设计特点：

- 提交不上传本地文件，而是提交服务器侧工作区快照
- 是否晚交由提交时刻和 `due_at` 比较得到
- 作业状态是运行时聚合结果，不是独立数据库表

## 3.7 教师/助教批改与成绩发布

目标用户：

- `root/admin/teacher/assistant`

核心能力：

- 查看某作业下各学生提交
- 查看历史提交版本
- 打开某次提交对应的复核工作区
- 录入评分与评语
- 发布成绩
- 导出成绩 CSV

主要后端接口：

- `GET /api/classes/:classID/assignments/:assignmentID/submissions`
- `GET /api/submissions/:id`
- `GET /api/submissions/:id/preview`
- `GET /api/submissions/:id/archive`
- `POST /api/submissions/:id/workspace`
- `POST /api/submissions/:id/grade`
- `POST /api/grades/:id/publish`
- `GET /api/classes/:classID/assignments/:assignmentID/grades/export`

主要前端入口：

- 作业页教师/助教模式

核心流程：

1. 教师/助教进入作业页
2. 系统列出该作业所有提交，并按学生聚合出最新记录
3. 教师选择某次提交
4. 系统把该提交归档恢复到一个可查看的工作区
5. 教师录入分数和评语
6. 系统保存评分并标记发布时间

设计特点：

- 当前前端在“保存评分”后会立即调用“发布成绩”，形成一键发布体验
- 批改逻辑依赖工作区恢复，而不是单独的文件 diff 或只读预览
- 导出成绩会为每位学生取最近一次提交和其成绩

## 3.8 工作区管理与 IDE 网关

目标用户：

- 学生
- 教师/助教复核提交时

核心能力：

- 创建工作区
- 等待工作区 ready
- 停止工作区
- 心跳续活
- 通过网关访问 RStudio

主要后端接口：

- `POST /api/classes/:classID/assignments/:assignmentID/workspace`
- `POST /api/submissions/:id/workspace`
- `DELETE /api/workspaces/:id`
- `POST /api/workspaces/:id/heartbeat`
- `ANY /ide/s/:workspaceID/*path`

核心实现机制：

- 创建 PVC、Pod、Service、NetworkPolicy
- Init container 把当前作业 starter/data/tests/public 复制进学生 home
- 反向代理把 `/ide/s/:workspaceID/` 转发到对应 Service

设计特点：

- 工作区访问权限以 `workspace.user_id` 为边界，管理员可越权查看
- 工作区 readiness 需要查询 Kubernetes，而不是只看数据库
- 教师查看提交时，系统实际上是恢复一份历史归档到工作区中

## 3.9 课程导入与重载

目标用户：

- `root/admin`

核心能力：

- 上传课程 zip
- 解压覆盖到内容目录
- 重新加载课程内容

主要后端接口：

- `POST /api/admin/courses/import`
- `POST /api/admin/courses/reload`

主要前端入口：

- 管理后台 `课程内容`

设计特点：

- 课程内容更新后需要显式重载，后端不会自动热更新内存中的课程模型
- 课程导入本质是文件系统操作，不会写入数据库

## 4. 关键业务流

## 4.1 学生完成一次作业

1. 学生登录系统并进入自己所属班级
2. 进入作业页，系统根据班级绑定课程读取题面与作业状态
3. 学生启动 RStudio 工作区
4. 系统创建 K8s 资源并把 starter/data/tests/public 拷贝到工作目录
5. 学生完成作业后点击提交
6. 系统将当前工作区打包上传到对象存储，并写入 `submissions`
7. 若已超过截止时间，则该提交标记为 `late=true`

## 4.2 教师批改一次提交

1. 教师进入某班级某作业
2. 系统列出所有提交并展示每名学生最近一次提交
3. 教师选择要查看的提交版本
4. 系统从对象存储下载归档并恢复到工作区
5. 教师在界面中录入分数和评语
6. 系统写入或更新 `grades`
7. 系统发布成绩，学生在自己的作业状态中可看到最新结果

## 4.3 教师组织一个新班级

1. 教师创建班级并指定 `course_id`
2. 导入学生账号，系统写入 `users` 和 `class_members`
3. 视需要把部分成员设为助教
4. 学生登录后通过 `class_members` 获得班级访问资格
5. 后续讲义、作业和提交均围绕该班级展开

## 5. 当前权限设计

当前权限系统由“全局角色 + 班级成员关系”共同决定。

### 全局角色

- `root`：最高权限，能管理所有用户和所有业务对象
- `admin`：系统管理员，但不能管理 `root`
- `teacher`：可创建和管理自己创建的班级
- `assistant`：默认不能全局管理，但可在自己担任助教的班级参与评分
- `student`：普通学习用户

### 班级内角色

- `student`
- `assistant`

权限判断原则：

- 看系统级后台能力，主要依赖 `users.role`
- 看是否可访问班级，依赖 `users.role + class_members`
- 看是否可批改班级，依赖 `teacher` 创建关系或 `assistant` 的班级成员关系

当前最值得注意的一点是：

- 助教既有班级内身份，又会改写全局角色，这使权限模型并不纯粹

## 6. 当前设计的结构性问题与需求切入点

下面这些不是解决方案，而是后续 `DEMAND.md` 最值得展开的需求来源。

### 1. 数据模型定义分散

同一业务概念同时存在于：

- 数据库表
- Go struct
- YAML 内容模型
- TypeScript 类型

后果：

- 新增字段或改语义时容易漏改
- 文档口径难统一
- 前后端和课程内容之间缺少明确契约

### 2. 权限模型存在双轨耦合

当前同时存在：

- `users.role`
- `class_members.member_role`

而且班级助教会同步改写全局角色。

后果：

- 用户身份的真实来源不够单一
- 一个用户在多个班级中的身份表达不够自然
- 后续若扩展更细权限，复杂度会迅速增加

### 3. 课程内容域与业务域边界不够显式

课程、章节、作业来自文件系统，而班级、提交、评分来自数据库。

后果：

- 业务规则实际依赖文件加载成功与否
- 课程内容更新和业务状态变更是两套机制
- 对外写需求时很容易把“课程配置”误写成“数据库模型”

### 4. 工作区状态表达偏弱

`workspaces.status` 当前主要是 `creating/running/stopped` 这类简单状态，但真实过程更复杂：

- 资源可能存在但未 ready
- 数据库状态和 Kubernetes 实际状态可能不一致
- 恢复历史提交可能失败
- 网关可达与工作区可用也不是同一个概念

后果：

- 故障排查复杂
- 前端用户反馈粒度有限
- 很多错误只能以“创建失败/未就绪”粗粒度暴露

### 5. 提交与评分模型偏“实现导向”

当前系统实际围绕“工作区归档”在运作，而不是围绕“提交内容结构”。

后果：

- 作业提交内容缺少统一抽象
- 难以对文本、文件、测试结果、自动评测结果做进一步结构化扩展
- 后续如果引入自动批改、查重、差异对比，会受到模型限制

### 6. 课程导入与重载流程是运维感知型流程

课程导入后，后端需要显式 reload 才会在运行中生效。

后果：

- 内容维护流程对管理员心智要求较高
- 容易出现“文件已更新但页面未生效”的认知错位
- 后续需求中应明确“内容变更反馈”“生效机制”“失败回滚/提示”

## 7. 给后续 `DEMAND.md` 的直接建议

如果下一步要写“进一步优化项目修复 bug”的需求书，建议以本文档为现状基线，需求章节可以直接围绕以下主线展开：

- 数据模型统一与边界清晰化
- 权限模型去耦与角色语义收敛
- 工作区生命周期与错误反馈细化
- 提交/评分模型结构化扩展
- 课程内容更新链路可观测化
- 前后端页面、接口、状态反馈的一致性修复

这样写出来的 `DEMAND.md` 会从“当前系统真实结构”出发，而不是只列零散 bug。
