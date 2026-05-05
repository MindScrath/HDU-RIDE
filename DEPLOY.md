# HDU RIDE 本地部署与启动指南

本文档记录了在 Windows 系统下，通过 Podman + kind (Kubernetes) 运行本项目的完整且正确的步骤。

## 1. 环境准备

确保您已安装并配置好以下工具：
- **Podman** 及 Podman Desktop (用于替代 Docker)
- **kind** (Kubernetes in Docker)
- **kubectl** (Kubernetes 命令行工具)
- **Bun** (前端包管理与运行工具)
- **Git Bash** 或其他兼容的 sh 环境 (Windows 环境下执行 `.sh` 脚本必需)

在开始前，请启动 Podman 虚拟机：
```powershell
podman machine start
```

如果还没有创建 kind 集群，请创建一个（例如命名为 `hdu-ride`）：
```powershell
kind create cluster --name hdu-ride
```

确保 `kubectl config current-context` 指向正确的集群。

## 2. 后端环境变量配置

项目后端会在集群中运行，首先需要准备本地环境变量配置，并生成默认 root 账号的密码 Hash：

1. 从模板复制 `.env`：
   ```powershell
   Copy-Item .env.example .env
   ```

2. 生成 `root` 用户的密码 Hash：
   ```powershell
   cd backend
   go run . hash-password root123456
   cd ..
   ```

3. 修改根目录下的 `.env` 文件，将生成的 Hash 填入 `ROOT_PASSWORD_HASH` 中，并将 `ROOT_PASSWORD` 改为 `root123456`：
   ```env
   ROOT_PASSWORD=root123456
   ROOT_PASSWORD_HASH=$2a$10$... (替换为您生成的 Hash)
   ```

## 3. Windows 下路径转换问题修复

由于 Windows 上的 Git Bash (MSYS) 会将 `/content` 这样的路径自动转换为 Windows 路径（如 `C:/Program Files/Git/content`），从而导致 `k8s-sync-content.sh` 脚本在使用 `kubectl exec` 挂载目录时出现 `tar: can't change directory` 错误。

我们在 `scripts/k8s-sync-content.sh` 中为相关 `kubectl` 命令添加了 `MSYS_NO_PATHCONV=1` 前缀以禁止自动路径转换：
```sh
MSYS_NO_PATHCONV=1 kubectl exec -n "$NAMESPACE" hdu-ride-content-sync -- sh -c "rm -rf /content/*"
tar -C "$CONTENT_DIR" -cf - . | MSYS_NO_PATHCONV=1 kubectl exec -i -n "$NAMESPACE" hdu-ride-content-sync -- tar -C /content -xf -
```
此修复已包含在当前代码库中。

## 4. 部署后端及依赖服务到 Kubernetes

通过 Git Bash 的 `sh` 执行官方提供的部署脚本。该脚本会自动部署 Postgres、MinIO、初始化存储桶、同步课程内容，并将后端部署到集群中，最后开启本地端口转发。

在 PowerShell 中执行以下命令（注意需要指定 Git Bash 的 sh 路径，否则 Windows 默认无法执行 `.sh`）：

```powershell
$env:PORT_FORWARD="1"
& "C:\Program Files\Git\bin\sh.exe" scripts/k8s-dev-up.sh
```

此命令将阻塞终端，并保持 `8080` 端口的转发（`kubectl port-forward -n hdu-ride svc/hdu-ride-backend 8080:8080`）。请**保持此终端窗口打开**。

## 5. 课程内容的更新与同步

如果您修改了 `content/` 目录下的 Markdown、YAML 课程配置文件，或添加了新的作业，需要将它们同步到 Kubernetes 集群中（因为后端读取的是集群 PVC 中的文件）。

您可以单独运行专门的同步脚本：

```powershell
& "C:\Program Files\Git\bin\sh.exe" scripts/k8s-sync-content.sh
```

此脚本会将本地的 `content/` 目录打包并通过 `kubectl` 传输到名为 `hdu-ride-content-sync` 的 Pod 中，解压并覆盖到挂载了 PVC 的 `/content` 目录下，从而完成更新。

## 6. 启动前端开发服务器

新开一个 PowerShell 终端窗口，进入 `frontend` 目录并启动 Vite 开发服务器：

```powershell
cd frontend
bun install
bun run dev
```

启动成功后，Vite 会监听在 `http://localhost:5173`。

## 7. 访问系统

在浏览器中打开：[http://localhost:5173/](http://localhost:5173/)

**默认测试账号**：
- 用户名：`root`
- 密码：`root123456` (或您在 `.env` 中设置的其他密码)

---
**附注：关于数据库重置**
如果遇到更改了 `.env` 中的密码但仍无法登录的情况，通常是因为 Postgres 数据库是之前遗留的，而后端初始化时使用了 `ON CONFLICT DO NOTHING`。此时您可以通过以下命令进入 Postgres 手动更新密码：
```powershell
kubectl exec -it postgres-0 -n hdu-ride -- psql -U hdu -d hdu_ride -c "UPDATE users SET password_hash = '\$2a\$10\$...' WHERE username = 'root';"
```
