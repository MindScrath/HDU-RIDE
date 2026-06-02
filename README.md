# HDU RIDE

HDU RIDE 是一个面向计量金融与 R 语言课程的教学平台。它将课程内容、作业发布与提交、批改评分、班级管理，以及“按学生/作业创建的 RStudio Server 工作空间”整合在一起。

## 文档入口

- [README.md](file:///d:/Go/HDU-RIDE/README.md)
  - 项目概览与本地开发快速入口
- [DEPLOY.md](file:///d:/Go/HDU-RIDE/DEPLOY.md)
  - Windows 本地开发细节
- [INSTRUCTION.md](file:///d:/Go/HDU-RIDE/INSTRUCTION.md)
  - Ubuntu 云主机首次部署
- [MAINTAIN.md](file:///d:/Go/HDU-RIDE/MAINTAIN.md)
  - 生产环境启停、检查、升级、诊断与恢复

## 技术栈

- 后端：Go + Gin + PostgreSQL，业务代码在 `backend/app/`
- 前端：Next.js 16 + React 19 + Tailwind CSS + shadcn/ui，代码在 `frontend-react/`，使用 Bun 管理依赖
- 课程内容：基于文件的 Markdown/YAML，位于 `content/courses`
- 存储：兼容 S3 的对象存储（例如 MinIO）
- RStudio：Rocker/RStudio 4.6，每个用户作业工作空间对应一个 Kubernetes Pod/PVC/Service，通过 `/ide/s/:workspaceID/` 访问

## 后端所需环境变量

后端会在启动时从仓库根目录读取 `.env`（如果存在），但真实环境变量优先级更高。必填配置缺失时会直接启动失败。

```text
Copy-Item .env.example .env
cd backend
go run . hash-password root123456
```

如果你用本地方式 `go run .` 启动后端，需要把生成的 bcrypt 值写入 `ROOT_PASSWORD_HASH`。
如果你用 `scripts/k8s-dev-up.sh` 部署到 Kubernetes，可以提供 `ROOT_PASSWORD_HASH`，或只设置 `ROOT_PASSWORD`；脚本会在创建 Kubernetes Secret 之前把 `ROOT_PASSWORD` 计算成 hash。

在集群外运行后端（例如本地 `go run .`）时，需要正确设置 `KUBECONFIG`，这样 `client-go` 才能创建 workspace 相关的 Kubernetes 资源。

## 开发

如果你需要代理下载依赖，可以先设置代理（按你本机实际端口调整）：

```powershell
$env:HTTP_PROXY="http://127.0.0.1:9098"
$env:HTTPS_PROXY="http://127.0.0.1:9098"
```

后端：

```powershell
cd backend
go test ./...
go run .
```

前端：

```powershell
cd frontend
bun install
bun run dev
```

Vite 开发服务器会把 `/api` 和 `/ide` 代理到 `http://127.0.0.1:8080`。

## 真实本地运行（需要真实 Kubernetes）

本项目运行时不提供 fake/mock 后端。workspace 的创建依赖 `client-go`，会创建真实的 Kubernetes 对象。单元测试只在 `backend/app/workspace_test.go` 中使用 fake Kubernetes client。

如果安装了 Podman 但未启动：

```powershell
podman machine init
podman machine start
```

如果 `kubectl config current-context` 为空，需要先配置或启动一个真实的 Kubernetes 集群，否则 workspace 功能无法运行。

使用 Podman 构建镜像：

```sh
TAG=dev \
PREFIX=localhost/hdu-ride \
PODMAN_MACHINE_PROXY=http://172.23.128.1:9098 \
sh scripts/podman-build-images.sh
```

这些脚本现在只是跨平台 Go 运维入口的包装；Windows 可直接运行：

```powershell
scripts\rideops.ps1 build-images
scripts\rideops.ps1 k8s-dev-up
scripts\rideops.ps1 db-reset
```

在 Windows/WSL + Podman 的组合下使用 kind 时，镜像拉取会在 Podman VM 内进行；如果需要代理，代理地址要使用 VM 网关地址。下面的脚本会部署 Postgres、MinIO、后端 RBAC、content PVC、MinIO bucket 初始化、课程内容同步，并预载 workspace 镜像：

```sh
BACKEND_IMAGE=localhost/hdu-ride/backend:dev \
PODMAN_MACHINE_PROXY=http://172.23.128.1:9098 \
sh scripts/k8s-dev-up.sh
```

注意：当前 `scripts/k8s-dev-up.sh` 只负责把后端及依赖部署到 kind 集群，不会自动执行 `kubectl port-forward`。如果你要在本机用 Vite 开发服务器访问后端，需要再手工开一个终端执行：

```powershell
kubectl port-forward -n hdu-ride svc/hdu-ride-backend 8080:8080
```

在另一个终端启动前端：

```powershell
cd frontend
bun run dev
```

打开 `http://127.0.0.1:5173`，使用 `root / root123456` 登录（开发环境默认账号）。

## 课程包（内容结构）

教师侧的课程内容以 zip 维护，结构如下：

```text
course.yml
chapters/
assignments/
```

`course.yml` 会分别列出“讲义章节”与“非章节作业”。隐藏测试放在 `tests/hidden` 下，不会被复制到 RStudio 工作空间中。

## 生产排障

如果你是在 Ubuntu 云主机上部署生产环境，出现了 Pod Pending、RStudio 打不开、教师批阅失败、后端 500 或网关 502 等问题，可以直接运行：

```bash
cd /opt/hdu-ride
bash scripts/k8s-prod-check.sh
```

它会生成一份适合贴给 AI 协助排障的诊断报告。更完整的使用说明见 [INSTRUCTION.md](file:///d:/Go/HDU-RIDE/INSTRUCTION.md)，而上线后的日常维护动作统一见 [MAINTAIN.md](file:///d:/Go/HDU-RIDE/MAINTAIN.md)。

如果你是在国内云主机上按生产方式部署，也请优先参考 [INSTRUCTION.md](file:///d:/Go/HDU-RIDE/INSTRUCTION.md) 中已经改成“国内镜像优先”的安装步骤，包括：

- Ubuntu 软件源
- Go 安装包下载源
- Kubernetes apt 源
- Flannel 安装方式
- Docker 镜像加速器与运行期镜像预拉取
