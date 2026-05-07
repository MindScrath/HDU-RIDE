# HDU RIDE Windows 本地开发指南

本文档只面向 Windows 本地开发环境，目标是通过 `Podman + kind + Vite` 跑起项目并进行日常调试。

如果您要在 Ubuntu 云主机上做正式部署，请直接看 [INSTRUCTION.md](file:///d:/Go/HDU-RIDE/INSTRUCTION.md)。

## 1. 适用范围

本文档适用于以下场景：

- 在 Windows 上本地开发后端、前端
- 使用 kind 提供本地 Kubernetes 集群
- 使用 Podman 构建并预载镜像
- 使用 Vite 启动前端开发服务器

本文档不再记录生产环境发布、Nginx、域名、HTTPS 或单节点 Kubernetes 运维，这些内容已统一迁移到 [INSTRUCTION.md](file:///d:/Go/HDU-RIDE/INSTRUCTION.md)。

## 2. 环境准备

请先安装：

- Podman
- kind
- kubectl
- Go
- Bun

Windows 下推荐优先使用 PowerShell 配合仓库内的 `scripts\rideops.ps1`。如果您更习惯 `sh`，也可以继续使用 `scripts/*.sh` 包装脚本，但当前仓库的核心逻辑已经统一走 `go run . ops ...`。

启动 Podman 虚拟机：

```powershell
podman machine start
```

如果还没有 kind 集群，请创建一个：

```powershell
kind create cluster --name hdu-ride
```

确认当前上下文正确：

```powershell
kubectl config current-context
```

## 3. 准备 `.env`

从模板复制配置：

```powershell
Copy-Item .env.example .env
```

生成 root 默认密码哈希：

```powershell
cd backend
go run . hash-password root123456
cd ..
```

把输出填入根目录 `.env` 的 `ROOT_PASSWORD_HASH`，并确认：

```env
ROOT_USERNAME=root
ROOT_PASSWORD=root123456
ROOT_PASSWORD_HASH=$2a$10$... 
```

## 4. 构建本地开发镜像

推荐直接使用 PowerShell 包装入口：

```powershell
$env:TAG="dev"
$env:PREFIX="localhost/hdu-ride"
$env:PODMAN_MACHINE_PROXY="http://172.23.128.1:9098"
scripts\rideops.ps1 build-images
```

如果您不用代理，可以省略 `PODMAN_MACHINE_PROXY`。

这一步会：

- 预拉取后端、前端、RStudio 所需基础镜像
- 构建本地开发镜像
- 生成：
  - `localhost/hdu-ride/backend:dev`
  - `localhost/hdu-ride/frontend:dev`
  - `localhost/hdu-ride/rstudio:dev`

## 5. 部署开发环境到 kind

执行：

```powershell
scripts\rideops.ps1 k8s-dev-up
```

或：

```sh
sh scripts/k8s-dev-up.sh
```

当前 `k8s-dev-up` 的实际行为是：

- 部署 PostgreSQL
- 部署 MinIO
- 初始化 bucket
- 创建/更新内容 PVC
- 将本地 `content/` 同步到集群
- 部署后端
- 设置后端镜像并等待 rollout 完成

注意：它**不会自动执行 `kubectl port-forward`**。

如果您要让本机 Vite 开发服务器访问集群里的后端，请另开一个终端手工执行：

```powershell
kubectl port-forward -n hdu-ride svc/hdu-ride-backend 8080:8080
```

这个终端需要保持打开。

## 6. 启动前端开发服务器

再开一个终端：

```powershell
cd frontend
bun install
bun run dev
```

Vite 默认监听：

- `http://127.0.0.1:5173`

当前前端开发代理会把 `/api` 和 `/ide` 转发到：

- `http://127.0.0.1:8080`

所以如果没有上一节的 `kubectl port-forward`，前端登录和 RStudio 代理都不会通。

## 7. 更新课程内容

本地开发模式下，后端读取的是集群里的内容 PVC，而不是直接读取 Windows 文件系统。

如果您修改了 `content/` 下的课程 YAML、讲义或作业内容，请重新执行：

```powershell
scripts\rideops.ps1 sync-content
```

或：

```sh
sh scripts/k8s-sync-content.sh
```

说明：

- 现在 `k8s-sync-content.sh` 只是 Go 运维入口包装
- 实际同步逻辑在 `backend/ops.go` 的 `ops sync-content`
- 旧版文档里关于在 shell 中手工拼接 `MSYS_NO_PATHCONV=1 kubectl exec ...` 的说明已经不再适用

## 8. 业务初始化

首次登录后，请先完成业务初始化，否则学生端会出现“作业列表为空”或“打开 RStudio 无反应”的现象。

必要步骤：

1. 使用管理员账号 `root` 登录
2. 创建班级并绑定课程
3. 创建学生账号
4. 将学生加入班级

如果学生没有被加入班级：

- 前端拿不到有效 `classID`
- 学生看不到作业
- 打开作业工作区时可能出现 403 或无反应

## 9. 修改代码后的更新方式

如果您修改了后端、前端或 RStudio 镜像构建内容，推荐直接重新走统一入口，而不是手工 `podman save` + `kind load`：

```powershell
scripts\rideops.ps1 build-images
scripts\rideops.ps1 k8s-dev-up
```

这样更符合当前仓库结构，也比旧文档中的手工流程更不容易漏步骤。

## 10. 常见问题

### 10.1 教师看不到学生保存在 home 根目录的文件

当前代码已经把提交归档与批阅恢复范围扩展到 `/home/rstudio`，不再只限于 `workspace/<assignmentID>` 子目录。

### 10.2 RStudio 打开后落在 `/root`

当前代码已经在工作区 Pod 环境变量中显式设置 `RUNROOTLESS=false`，用于避免 `rocker/rstudio` 在 Kubernetes 中误判为 rootless 模式。

### 10.3 修改 `content/` 后页面没变化

这通常不是同步失败，而是后端仍持有旧的内存课程数据。开发环境中建议：

1. 先执行 `scripts\rideops.ps1 sync-content`
2. 再重启后端或重新触发课程重载

## 11. 访问方式

浏览器访问：

- `http://127.0.0.1:5173`

默认开发账号：

- 用户名：`root`
- 密码：`root123456`
