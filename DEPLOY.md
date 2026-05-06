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

## 6. 系统业务初始化与班级配置（重要！）

由于系统业务逻辑的限制，学生必须被显式地加入到某一个“班级”中，才能查看到该班级的作业列表，并且**必须在作业列表中才能成功启动对应作业的 RStudio 工作区**。如果不进行班级配置，前端会遇到“打开 RStudio 无反应/报错”或作业列表为空的问题（因为前端会发送一个缺少 `classID` 的非法 403 请求）。

**请在首次登录后，按以下步骤完成业务初始化：**

1. 使用管理员/教师账号（例如 `root`）登录系统。
2. 点击左侧导航栏的 **班级**，创建一个新班级（如：“计量金融 2026 春”），并绑定导入好的课程（如：`intro-r`）。
3. 如果需要学生测试账号，可前往 **管理 -> 用户管理** 创建一个学生账号。
4. 返回 **班级**，点击刚创建的班级，进入 **成员管理** 选项卡。
5. 将创建好的学生账号导入/添加到该班级中。
6. 使用学生账号重新登录，即可在 **作业** 菜单中正常查看作业列表，并成功创建和打开对应作业的 RStudio 工作区。

## 7. 启动前端开发服务器

新开一个 PowerShell 终端窗口，进入 `frontend` 目录并启动 Vite 开发服务器：

```powershell
cd frontend
bun install
bun run dev
```

启动成功后，Vite 会监听在 `http://localhost:5173`。

## 8. 后端代码修改与镜像更新
如果对后端 Go 代码进行了二次开发，由于服务运行在本地 Kind 集群中，我们需要通过以下步骤将新代码打包为镜像并应用到集群中，否则修改不会生效：

1. **构建后端镜像**：
   在项目根目录运行，将后端编译并打包为 Docker 镜像：
   ```powershell
   podman build --no-cache -t localhost/hdu-ride/backend:dev -f deploy/docker/backend.Dockerfile .
   ```
2. **导出镜像为 tar 包**：
   将镜像保存为本地文件，供 Kind 集群读取：
   ```powershell
   podman save -o backend-dev.tar localhost/hdu-ride/backend:dev
   ```
3. **将镜像加载到 Kind 集群**：
   把导出的 tar 包导入到正在运行的 Kind 内部：
   ```powershell
   kind load image-archive backend-dev.tar --name hdu-ride
   ```
4. **更新并重启 Deployment**：
   确保集群中的 Deployment 指向 `dev` 标签的镜像，并强制重启 Pod 以应用更新：
   ```powershell
   kubectl set image deployment/hdu-ride-backend -n hdu-ride backend=localhost/hdu-ride/backend:dev
   kubectl rollout restart deployment/hdu-ride-backend -n hdu-ride
   ```

## 9. 常见问题排查与注意事项

**1. 教师看不到学生在 RStudio 中提交的代码**
- **原因**：Kubernetes 启动 `rocker/rstudio` 容器时（以 `uid=0` 和 `USERID=1000` 运行），镜像内置的 `cont-init.d/02_userconf` 脚本可能错误判断为 Rootless 模式，导致登录用户强制变为 `root`，工作目录变为 `/root`，而系统默认打包的是 `/home/rstudio`。
- **解决**：在后端生成 Kubernetes Pod 的环境变量配置中（`backend/app/workspace.go`），必须显式注入环境变量 `RUNROOTLESS=false`。此修复目前已在代码库中生效。

## 10. 访问系统

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
