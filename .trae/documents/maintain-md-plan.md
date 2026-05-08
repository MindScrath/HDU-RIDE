# `MAINTAIN.md` 编写计划

## Summary

目标是在仓库根目录新增一份以生产环境为主的运维手册 `MAINTAIN.md`，按“操作场景”组织服务启停、检查、重启、诊断、内容维护、升级、恢复等内容，并在现有入口文档中增加跳转，形成统一的运维入口。

文档的预期读者是 Ubuntu 云主机上的实际维护者，而不是第一次部署项目的新手。因此重点是“上线后怎么维护”，不是“从 0 到部署”。

## Current State Analysis

### 现有运维入口

- 生产部署入口：
  - `scripts/k8s-prod-up.sh`
  - 内部最终调用 `backend/ops.go` 的 `go run . ops k8s-prod-up`
- 生产诊断入口：
  - `scripts/k8s-prod-check.sh`
- 基础设施安装入口：
  - `scripts/k8s-install-flannel.sh`
  - `scripts/k8s-install-local-path.sh`
- 本地开发入口：
  - `scripts/rideops.ps1`
  - `scripts/rideops.cmd`
  - `scripts/k8s-dev-up.sh`
  - `scripts/k8s-sync-content.sh`
  - `scripts/podman-build-images.sh`

### 当前真实资源与服务名

根据 `deploy/k8s/*.yml`，生产环境的核心对象名已经明确：

- Kubernetes 命名空间：`hdu-ride`
- 后端 Deployment / Service：`hdu-ride-backend`
- 前端 Deployment / Service：`hdu-ride-frontend`
- PostgreSQL StatefulSet / Service：`postgres`
- MinIO StatefulSet / Service：`minio`
- 动态存储 Deployment：`local-path-provisioner`
- 动态存储命名空间：`local-path-storage`
- 前端公网入口依赖宿主机 Nginx 反代到 `127.0.0.1:30080`

### 当前文档分散情况

- `INSTRUCTION.md`
  - 已包含部署、升级、部分重启、内容更新、诊断脚本、常见问题，但偏“部署手册”
- `README.md`
  - 以项目概览和本地开发为主，只轻度提到生产诊断
- `DEPLOY.md`
  - 以 Windows 本地开发为主

### 已确认可纳入运维手册的命令与接口

- 宿主机服务：
  - `systemctl status|restart|reload nginx`
  - `systemctl status kubelet`
  - `systemctl status containerd`
- Kubernetes 运维：
  - `kubectl get pods|svc|pvc|storageclass`
  - `kubectl logs`
  - `kubectl rollout restart|status`
  - `kubectl delete pod`
- 内容维护：
  - 管理端 `POST /api/admin/courses/reload`
- 恢复/危险动作：
  - `ops db-reset`
  - 删除工作区 Pod / PVC
  - 删除静态内容卷 PV / PVC

## Proposed Changes

### 1. 新增 `MAINTAIN.md`

文件：

- `d:\Go\HDU-RIDE\MAINTAIN.md`

内容结构：

- **文档范围与原则**
  - 明确这是“生产环境维护手册”
  - 明确本地开发请看 `DEPLOY.md`
  - 明确首次部署请看 `INSTRUCTION.md`
- **服务与对象总览**
  - 列出宿主机服务、Kubernetes 命名空间、Deployment、StatefulSet、Service、StorageClass
  - 给出对象名与用途，便于后续所有命令直接复制使用
- **日常操作速查**
  - 查看整体状态
  - 查看 Pod / PVC / StorageClass
  - 查看后端、前端、Postgres、MinIO 日志
  - 运行一键诊断脚本
- **服务启停与重启**
  - 宿主机侧：`nginx`、`kubelet`、`containerd`
  - Kubernetes 侧：重启后端、前端
  - 明确哪些组件不建议直接“停掉”
- **内容维护**
  - 修改 `/opt/hdu-ride/content`
  - 管理端点击“重新加载”
  - 区分“改内容”和“改代码”
- **代码升级**
  - `git pull`
  - 构建镜像
  - 导入 `containerd`
  - 重新执行 `k8s-prod-up.sh`
- **诊断与排障**
  - 优先运行 `scripts/k8s-prod-check.sh`
  - 常用 `kubectl` / `systemctl` / `journalctl` 命令
- **危险/恢复操作**
  - 数据库重置
  - 删除工作区 Pod/PVC
  - 删除静态内容卷 PV/PVC
  - 每类操作都加明显“危险”说明、适用场景、后果

编写原则：

- 按操作场景，而不是按组件碎片化组织
- 先给“做什么”，再给“为什么”
- 命令尽量直接可复制
- 明确“正常现象 / 继续条件 / 危险后果”

### 2. 在 `INSTRUCTION.md` 增加维护入口

文件：

- `d:\Go\HDU-RIDE\INSTRUCTION.md`

修改方向：

- 在适合的收尾或运维章节加入 `MAINTAIN.md` 的跳转
- 明确职责分工：
  - `INSTRUCTION.md` 负责首次部署
  - `MAINTAIN.md` 负责上线后维护

原因：

- 现有 `INSTRUCTION.md` 已很长，不适合继续承载所有维护细节
- 用户已明确希望统一服务启停和运维工具说明

### 3. 在 `README.md` 增加轻量入口

文件：

- `d:\Go\HDU-RIDE\README.md`

修改方向：

- 新增一小段维护文档入口
- 指向：
  - `DEPLOY.md`：本地开发
  - `INSTRUCTION.md`：首次生产部署
  - `MAINTAIN.md`：生产维护

原因：

- 顶层入口文档需要让用户知道“该去哪里看”
- 避免维护说明继续散落

## Assumptions & Decisions

- 已确认 `MAINTAIN.md` 以**生产环境为主**，不是本地开发手册。
- 已确认组织方式采用**按操作场景**。
- 已确认危险/恢复类操作要**纳入文档并显著标红**。
- 已确认后续执行时要**联动现有文档**，至少补维护文档入口。
- 本次计划默认不重构现有脚本逻辑，只整理与统一文档入口；除非执行阶段发现某个文档必须引用的脚本行为与实际不一致，再做最小必要修正。

## Verification Steps

执行阶段完成后需要验证：

1. `MAINTAIN.md` 中列出的脚本、资源名、服务名全部来自仓库当前实现，路径和对象名可对上。
2. `MAINTAIN.md` 能覆盖以下核心维护场景：
   - 查看整体状态
   - 重启后端/前端
   - 查看日志
   - 运行诊断脚本
   - 更新内容并重载课程
   - 升级代码并重新部署
   - 执行危险恢复操作前的风险提示
3. `README.md` 与 `INSTRUCTION.md` 已新增对 `MAINTAIN.md` 的清晰入口。
4. 新增或修改后的 Markdown 文件无诊断错误。
