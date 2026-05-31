# HDU RIDE（React 版）—— Ubuntu 22.04 / 24.04 从 0 到公网部署手册

本文档面向**全新 Ubuntu 22.04 / 24.04 云主机**，从零将 HDU RIDE 完整部署到公网。

**本文档覆盖整个项目**：Go 后端、React 前端、PostgreSQL、MinIO、Kubernetes、Nginx、HTTPS。

文档分工如下：

- 首次部署看本文档
- 日常升级/维护看 [REACT-UP.md](REACT-UP.md)
- 后端 / K8s / 数据库深度运维看 [INSTRUCTION.md](INSTRUCTION.md)
- Windows 本地开发看 [DEPLOY.md](DEPLOY.md)

---

## 1. 项目是什么

`HDU RIDE` 是一个教学平台，核心组件如下：

| 组件 | 技术栈 | 说明 |
|------|--------|------|
| 前端 | Next.js + React 19 + Shadcn/ui + Tailwind CSS | 对外提供网站页面 |
| AI 助手 | CopilotKit AG-UI + 阿里云百炼（通义千问） | 在线 AI 聊天，支持流式响应 |
| 后端 | Go | 登录、班级、作业、提交、评分、工作区管理 |
| 数据库 | PostgreSQL | 业务主数据 |
| 对象存储 | MinIO | 提交文件与工作区归档 |
| 工作区 | Kubernetes Pod + PVC + Service | 每个学生/作业独立 RStudio 在线环境 |
| 反向代理 | 宿主机 Nginx | 公网入口，域名 + HTTPS |

### 1.1 仓库结构

```
/opt/hdu-ride/
├── backend/              # Go 后端
├── frontend-react/       # React 前端 (Next.js)
├── content/              # 课程内容（讲义、作业说明、starter、测试数据）
├── deploy/
│   ├── docker/           # Dockerfile（后端、前端、RStudio 自定义镜像、Nginx 配置）
│   └── k8s/              # Kubernetes 清单
└── scripts/              # 部署/运维脚本入口
```

### 1.2 运行时链路

```
公网 → 宿主机 Nginx (80/443)
        → 前端 NodePort (127.0.0.1:30080)
          → 前端容器 (Next.js, 端口 3000)
            → /api/* 和 /ide/* → Go 后端 (hdu-ride-backend:8080)
              → PostgreSQL / MinIO / 课程内容 / K8s 工作区
```

> **与旧 Vue 版的关键区别**：前端不再是 Nginx 容器提供静态文件。Next.js 自带服务器，通过内置 rewrites 代理 `/api` 和 `/ide` 到 Go 后端。

---

## 2. 先决条件

### 2.1 云主机建议配置

最低建议：

- 4 核 CPU
- 8 GB 内存
- 80 GB 以上系统盘

更稳妥：

- 8 核 CPU
- 16 GB 内存
- 100 GB 以上磁盘

### 2.2 域名准备

假设域名为 `ride.mindsratch.top`：

1. 在域名服务商后台添加 A 记录，指向云主机公网 IP
2. 验证：`nslookup ride.mindsratch.top`

### 2.3 云厂商安全组

放行以下端口：

- `22/tcp` —— SSH
- `80/tcp` —— HTTP
- `443/tcp` —— HTTPS

---

## 3. Ubuntu 基础初始化

```bash
sudo apt update

# 国内环境建议先换阿里云镜像源
sudo cp /etc/apt/sources.list /etc/apt/sources.list.bak
sudo sed -i 's|http://archive.ubuntu.com/ubuntu/|https://mirrors.aliyun.com/ubuntu/|g; s|http://security.ubuntu.com/ubuntu/|https://mirrors.aliyun.com/ubuntu/|g' /etc/apt/sources.list
sudo apt update

# 安装基础工具
sudo apt install -y curl wget git vim nano jq unzip ca-certificates gnupg lsb-release apt-transport-https software-properties-common nginx docker.io

# 启动服务
sudo systemctl enable --now docker
sudo systemctl enable --now nginx
sudo systemctl enable --now containerd

# 设置时区
sudo timedatectl set-timezone Asia/Shanghai
```

---

## 4. 安装 Go

```bash
cd /tmp
curl -LO https://mirrors.aliyun.com/golang/go1.26.0.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.26.0.linux-amd64.tar.gz
echo 'export PATH=/usr/local/go/bin:$PATH' | sudo tee /etc/profile.d/go.sh
source /etc/profile.d/go.sh
go version

# 配置国内代理
go env -w GOPROXY=https://goproxy.cn,direct
go env -w GOSUMDB=sum.golang.google.cn
```

---

## 5. 安装 Bun（React 前端运行时 + 包管理器）

```bash
curl -fsSL https://bun.sh/install | bash
source ~/.bashrc
bun --version
```

> **Docker 构建前提**：项目使用 npm 创建（有 `package-lock.json`），但 Dockerfile 基于 Bun。构建镜像前必须先运行：
>
> ```bash
> cd /opt/hdu-ride/frontend-react && bun install
> ```
>
> 这会生成 `bun.lock`，Docker 构建时才能正确 `COPY` 并安装依赖。

---

## 6. 安装 Kubernetes

### 6.1 安装 kubeadm / kubelet / kubectl

```bash
sudo mkdir -p /etc/apt/keyrings
curl -fsSL https://mirrors.aliyun.com/kubernetes-new/core/stable/v1.29/deb/Release.key | sudo gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg
echo 'deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://mirrors.aliyun.com/kubernetes-new/core/stable/v1.29/deb/ /' | sudo tee /etc/apt/sources.list.d/kubernetes.list
sudo apt update
sudo apt install -y kubelet kubeadm kubectl
sudo apt-mark hold kubelet kubeadm kubectl
sudo systemctl enable --now kubelet
```

### 6.2 内核参数 + 关闭 swap

```bash
sudo modprobe br_netfilter
echo "br_netfilter" | sudo tee /etc/modules-load.d/br_netfilter.conf

cat <<'EOF' | sudo tee /etc/sysctl.d/99-kubernetes-cri.conf
net.bridge.bridge-nf-call-iptables = 1
net.ipv4.ip_forward = 1
EOF

sudo sysctl --system
sudo swapoff -a
sudo sed -i '/ swap / s/^/#/' /etc/fstab
```

### 6.3 配置 containerd

```bash
sudo mkdir -p /etc/containerd
containerd config default | sudo tee /etc/containerd/config.toml >/dev/null
sudo sed -i 's/SystemdCgroup = false/SystemdCgroup = true/' /etc/containerd/config.toml
sudo sed -i "s|sandbox = .*|sandbox = 'registry.aliyuncs.com/google_containers/pause:3.9'|g" /etc/containerd/config.toml
sudo systemctl restart containerd
sudo systemctl restart kubelet
```

### 6.4 初始化单节点集群

```bash
sudo kubeadm init \
  --pod-network-cidr=10.244.0.0/16 \
  --image-repository registry.aliyuncs.com/google_containers
```

### 6.5 配置 kubectl

```bash
mkdir -p $HOME/.kube
sudo cp /etc/kubernetes/admin.conf $HOME/.kube/config
sudo chown $(id -u):$(id -g) $HOME/.kube/config
kubectl get nodes
```

> 此时节点显示 `NotReady` 是正常的——还没安装网络插件。

### 6.6 允许调度到控制平面（单节点必须）

```bash
kubectl taint nodes --all node-role.kubernetes.io/control-plane- || true
kubectl taint nodes --all node-role.kubernetes.io/master- || true
```

> `|| true` 是为了忽略 "not found" 错误——新版本 K8s 用 `control-plane`，旧版本用 `master`，你的集群可能只有其中一个 taint。报 `taint not found` 是正常的，说明没有需要移除的 taint。

### 6.7 安装 Flannel 网络

```bash
cd /opt/hdu-ride
bash scripts/k8s-install-flannel.sh
```

等待 Pod 就绪：

```bash
kubectl get pods -n kube-flannel
kubectl get pods -n kube-system
kubectl get nodes   # 应显示 Ready
```

---

## 7. 安装动态存储

```bash
cd /opt/hdu-ride
bash scripts/k8s-install-local-path.sh
```

验证：

```bash
kubectl get storageclass
# 应显示 local-path (default)
```

---

## 8. 获取项目代码

```bash
cd ~
git clone https://github.com/MindScrath/HDU-RIDE.git hdu-ride
sudo rm -rf /opt/hdu-ride
sudo mkdir -p /opt
sudo cp -a ~/hdu-ride /opt/hdu-ride
sudo chown -R $USER:$USER /opt/hdu-ride
cd /opt/hdu-ride
```

---

## 9. 配置环境变量

### 9.1 从模板复制

```bash
cd /opt/hdu-ride
cp .env.example .env
nano .env
```

> **注意**：`.env.example` 可能尚未包含 AI 助手和镜像名变量（`BAILIAN_API_KEY`、`BAILIAN_APP_ID`、`BACKEND_IMAGE`、`FRONTEND_IMAGE`）。如果复制后缺少这些字段，请参照下方 9.2 节的完整模板手动添加到 `.env` 末尾。

### 9.2 必须修改的字段

```dotenv
# ── 数据库 ──
POSTGRES_DB=hdu_ride
POSTGRES_USER=hdu
POSTGRES_PASSWORD=请换成强密码

# ── 对象存储 ──
S3_BUCKET=hdu-ride
S3_ACCESS_KEY_ID=请换成你自己的MinIO账号
S3_SECRET_ACCESS_KEY=请换成你自己的MinIO密码

# ── 会话 ──
SESSION_SECRET=请换成长随机字符串（例如 openssl rand -hex 32 的输出）

# ── 管理员 ──
ROOT_USERNAME=root
ROOT_PASSWORD=请先自定义管理员密码

# ── 工作区 ──
WORKSPACE_STORAGE_CLASS=local-path
WORKSPACE_IMAGE_DEFAULT=rocker/rstudio:4.6.0

# ── AI 助手（React 版新增）──
BAILIAN_API_KEY=sk-xxxxxxxxxxxxxxxx     # 阿里云百炼 API Key
BAILIAN_APP_ID=xxxxxxxxxxxxxxxx         # 阿里云百炼 App ID

# ── 镜像名 ──
BACKEND_IMAGE=hdu-ride-backend:latest
FRONTEND_IMAGE=hdu-ride-frontend:latest
```

### 9.3 生成 ROOT_PASSWORD_HASH

```bash
cd /opt/hdu-ride/backend
go run . hash-password '你的管理员密码'
```

把输出填入 `.env` 的 `ROOT_PASSWORD_HASH`。

### 9.4 准备内容目录

```bash
mkdir -p /opt/hdu-ride/content
```

课程内容结构：

```text
/opt/hdu-ride/content/
  courses/
    intro-r/
      course.yml
      chapters/
      assignments/
        hw01/
          assignment.yml
          README.md
          starter/
          data/public/
          tests/public/
```

---

## 10. 构建并导入镜像

### 10.1 配置 Docker 镜像加速器（国内环境）

```bash
sudo mkdir -p /etc/docker
cat <<'EOF' | sudo tee /etc/docker/daemon.json
{
  "registry-mirrors": [
    "https://docker.m.daocloud.io"
  ]
}
EOF
sudo systemctl daemon-reload
sudo systemctl restart docker
```

### 10.2 构建项目镜像

```bash
cd /opt/hdu-ride

# ① 构建前端前，先生成 bun.lock（项目由 npm 创建，Dockerfile 基于 Bun）
cd frontend-react && bun install && cd ..

# ② 构建镜像
sudo docker build -t hdu-ride-backend:latest -f deploy/docker/backend.Dockerfile .
sudo docker build -t hdu-ride-frontend:latest -f deploy/docker/frontend.Dockerfile \
  --build-arg NEXT_PUBLIC_GO_API_URL=http://hdu-ride-backend:8080 \
  .
```

说明：

- `backend.Dockerfile`：Go 1.26 编译 → Alpine 运行，暴露 8080
- `frontend.Dockerfile`：Bun 构建 Next.js standalone → Bun slim 运行，暴露 3000

### 10.3 拉取运行期镜像

```bash
sudo docker pull postgres:18-alpine
sudo docker pull minio/minio:latest
sudo docker pull minio/mc:latest
sudo docker pull busybox:1.36
sudo docker pull rocker/rstudio:4.6.0
```

如果 Docker Hub 拉取困难，通过代理中转：

```bash
sudo docker pull docker.m.daocloud.io/postgres:18-alpine
sudo docker tag docker.m.daocloud.io/postgres:18-alpine postgres:18-alpine
# 其他镜像同理
```

### 10.4 导入到 containerd

Kubernetes 运行时是 `containerd`，不是 Docker。必须把镜像导入：

```bash
cd /tmp

sudo docker save hdu-ride-backend:latest -o hdu-ride-backend.tar
sudo docker save hdu-ride-frontend:latest -o hdu-ride-frontend.tar
sudo docker save postgres:18-alpine -o postgres.tar
sudo docker save minio/minio:latest -o minio.tar
sudo docker save minio/mc:latest -o minio-mc.tar
sudo docker save busybox:1.36 -o busybox.tar
sudo docker save rocker/rstudio:4.6.0 -o rstudio.tar

sudo ctr -n k8s.io images import hdu-ride-backend.tar
sudo ctr -n k8s.io images import hdu-ride-frontend.tar
sudo ctr -n k8s.io images import postgres.tar
sudo ctr -n k8s.io images import minio.tar
sudo ctr -n k8s.io images import minio-mc.tar
sudo ctr -n k8s.io images import busybox.tar
sudo ctr -n k8s.io images import rstudio.tar
```

验证：

```bash
sudo ctr -n k8s.io images list | grep -E 'hdu-ride|postgres|minio|busybox|rstudio'
```

---

## 11. 执行生产部署

```bash
cd /opt/hdu-ride
bash scripts/k8s-prod-up.sh
```

这个脚本会：

1. 检查环境（kubectl、Go、存储类）
2. 创建 namespace、Secret、内容卷 PV/PVC
3. 部署 PostgreSQL、MinIO
4. 初始化 MinIO bucket
5. 部署 Go 后端、React 前端
6. 等待所有 Pod Ready

### 验证

```bash
kubectl get pods -n hdu-ride
kubectl get svc -n hdu-ride
```

应看到所有 Pod 为 `Running`：

- `postgres-0`
- `minio-0`
- `hdu-ride-backend-...`
- `hdu-ride-frontend-...`

前端 NodePort 默认为 `127.0.0.1:30080`。

---

## 12. 配置域名反代

### 12.1 创建 Nginx 站点

```bash
sudo nano /etc/nginx/sites-available/hdu-ride
```

```nginx
server {
    listen 80;
    server_name ride.mindsratch.top;

    location / {
        proxy_pass http://127.0.0.1:30080;
        proxy_http_version 1.1;

        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # WebSocket 支持（RStudio 需要）
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";

        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;
    }
}
```

> **注意**：这里的 `proxy_pass` 指向 `http://127.0.0.1:30080`（K8s NodePort），不是直接指向 Next.js 端口。所有 `/api` 和 `/ide` 代理由 Next.js 内部的 rewrites 处理，宿主机 Nginx 不需要单独配置 `/api` 和 `/ide` 的 location 块。

### 12.2 启用站点

```bash
sudo ln -sf /etc/nginx/sites-available/hdu-ride /etc/nginx/sites-enabled/
sudo rm -f /etc/nginx/sites-enabled/default
sudo nginx -t
sudo systemctl reload nginx
```

现在访问 `http://ride.mindsratch.top` 应能看到登录页。

---

## 13. 配置 HTTPS

```bash
sudo apt install -y certbot python3-certbot-nginx
sudo certbot --nginx -d ride.mindsratch.top
```

验证自动续期：

```bash
sudo certbot renew --dry-run
```

---

## 14. 首次上线后的业务初始化

站点能打开只代表服务启动成功，还需要业务初始化：

1. **管理员登录**：用 `.env` 中 `ROOT_USERNAME` / `ROOT_PASSWORD` 登录
2. **导入课程**：管理 → 课程内容 → 上传课程 zip 或配置宿主机 `/opt/hdu-ride/content` 后点击"重新加载"
3. **创建班级**：在班级页面新建班级并绑定课程
4. **创建学生账号**：管理 → 用户管理 → 新建用户
5. **将学生加入班级**：进入班级 → 成员 → 导入学生
6. **测试完整流程**：

   - 学生登录 → 查看讲义 → 打开作业 → 进入 RStudio → 提交
   - 教师登录 → 查看提交 → 打开复核工作区 → 评分 → 发布成绩
   - 测试 AI 助手：进入 AI 助手页面，发送消息验证响应

---

## 15. AI 助手配置说明

React 版新增的 AI 助手使用阿里云百炼平台（通义千问）。

### 15.1 获取 API 密钥

1. 登录 [阿里云百炼平台](https://bailian.console.aliyun.com/)
2. 创建应用，获取 `API Key` 和 `App ID`
3. 确保已开通模型服务（推荐 `qwen-plus`）

### 15.2 配置方式

**生产环境（K8s）**：密钥通过 `k8s-prod-up.sh` 自动写入 K8s Secret：

```bash
# 确认 .env 中有这两行
BAILIAN_API_KEY=sk-xxxxxxxxxxxxxxxx
BAILIAN_APP_ID=xxxxxxxxxxxxxxxx
```

**开发环境**：写在 `frontend-react/.env.local`：

```env
BAILIAN_API_KEY=sk-xxxxxxxxxxxxxxxx
BAILIAN_APP_ID=xxxxxxxxxxxxxxxx
NEXT_PUBLIC_GO_API_URL=http://localhost:8080
```

### 15.3 切换模型

编辑 `frontend-react/app/api/copilotkit/route.ts`：

```typescript
const serviceAdapter = new OpenAIAdapter({
  openai,
  model: 'qwen-plus',  // 可选: qwen-turbo（快）, qwen-plus（均衡）, qwen-max（最强）
})
```

### 15.4 AI 助手无响应时的排查

```bash
# 直接测试百炼 API
curl -X POST https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions \
  -H "Authorization: Bearer $BAILIAN_API_KEY" \
  -H "X-DashScope-AppId: $BAILIAN_APP_ID" \
  -H "Content-Type: application/json" \
  -d '{"model":"qwen-plus","messages":[{"role":"user","content":"你好"}]}'
```

---

## 16. 前端 Markdown / LaTeX 公式说明

React 版使用 `react-markdown + remark-math + rehype-katex` 渲染公式，替代旧版的 `markdown-it`。

支持的公式格式：

- 行内公式：`$E = mc^2$`
- 块级公式：`$$\hat{\beta} = (X^TX)^{-1}X^Ty$$`
- 对齐环境需包裹：`\begin{aligned} ... \end{aligned}`

如果公式不渲染，检查：

1. `frontend-react/app/globals.css` 中有 `@import 'katex/dist/katex.min.css';`
2. `components/markdown/MarkdownRenderer.tsx` 中插件已正确配置

---

## 17. 验收清单

### 基础设施

```bash
kubectl get nodes                        # Ready
kubectl get storageclass                 # local-path (default)
kubectl get pods -n hdu-ride            # 全部 Running
kubectl get svc -n hdu-ride             # 前端 NodePort 30080
```

### 网站入口

- `http://ride.mindsratch.top` → 登录页
- `https://ride.mindsratch.top` → 登录页

### 业务闭环

- [ ] root 能登录
- [ ] 能看到班级列表
- [ ] 能创建班级
- [ ] 能创建学生账号并加入班级
- [ ] 学生能查看讲义（Markdown + 公式正确渲染）
- [ ] 学生能打开 RStudio 工作区
- [ ] 学生能提交作业
- [ ] 教师能查看提交列表
- [ ] 教师能批改并发布成绩
- [ ] AI 助手能正常对话（流式响应）
- [ ] 修改课程内容后点击"重新加载"能生效

---

## 18. 常见问题

### 18.1 `kubeadm init` 失败

- 检查 swap 是否关闭：`free -m`
- 检查 containerd SystemdCgroup：`grep SystemdCgroup /etc/containerd/config.toml`
- 查看日志：`journalctl -xeu kubelet`

### 18.2 Pod 一直 Pending

```bash
kubectl describe pod <pod名> -n hdu-ride
```

常见原因：

- 存储类缺失 → 执行第 7 节
- 资源不足 → 调低 `.env` 中的 `WORKSPACE_CPU_REQUEST` 和 `WORKSPACE_MEM_REQUEST`
- 镜像未导入 → 执行第 10.4 节

### 18.3 RStudio 打不开

1. 确认学生已加入班级
2. 确认 `local-path` 存储类正常
3. 确认 `rocker/rstudio:4.6.0` 已导入 containerd
4. 确认 Nginx 配置了 WebSocket 头（`Upgrade` / `Connection`）

### 18.4 页面空白 / 登录后跳回登录页

- Go 后端是否在运行
- 浏览器 DevTools → Application → Cookies 检查 `session_token`

### 18.5 AI 助手报错

- `.env` 中的 `BAILIAN_API_KEY` 和 `BAILIAN_APP_ID` 是否正确
- 百炼控制台是否开通了 `qwen-plus` 模型
- 查看前端容器日志：`kubectl logs deploy/hdu-ride-frontend -n hdu-ride`

### 18.6 修改课程内容后页面未生效

课程内容被后端加载到内存中。修改 `/opt/hdu-ride/content/` 后：

1. 登录管理员后台
2. 进入课程管理页
3. 点击"重新加载"

---

## 19. 快速部署总结（完整命令序列）

```bash
# === 第 1 步：基础环境 ===
sudo apt update && sudo apt install -y git curl nginx docker.io vim jq
sudo systemctl enable --now docker nginx containerd

# === 第 2 步：安装 Go ===
cd /tmp && curl -LO https://mirrors.aliyun.com/golang/go1.26.0.linux-amd64.tar.gz
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.26.0.linux-amd64.tar.gz
echo 'export PATH=/usr/local/go/bin:$PATH' | sudo tee /etc/profile.d/go.sh
source /etc/profile.d/go.sh
go env -w GOPROXY=https://goproxy.cn,direct

# === 第 3 步：安装 Bun ===
curl -fsSL https://bun.sh/install | bash && source ~/.bashrc

# === 第 4 步：安装 K8s + 初始化集群 ===
# （执行第 6 节完整步骤）

# === 第 5 步：安装 Flannel + local-path ===
# （执行第 6.7 和 第 7 节）

# === 第 6 步：获取代码 ===
cd ~ && git clone <repo-url> hdu-ride
sudo cp -a ~/hdu-ride /opt/hdu-ride && sudo chown -R $USER:$USER /opt/hdu-ride
cd /opt/hdu-ride

# === 第 7 步：配置环境 ===
cp .env.example .env && nano .env
# 修改：POSTGRES_PASSWORD, S3_ACCESS_KEY_ID, S3_SECRET_ACCESS_KEY,
#       SESSION_SECRET, ROOT_PASSWORD, BAILIAN_API_KEY, BAILIAN_APP_ID
cd backend && go run . hash-password '你的密码' && cd ..
# 把输出的哈希填入 .env 的 ROOT_PASSWORD_HASH

# === 第 8 步：构建 + 导入镜像 ===
cd frontend-react && bun install && cd ..   # 生成 bun.lock（必须）
sudo docker build -t hdu-ride-backend:latest -f deploy/docker/backend.Dockerfile .
sudo docker build -t hdu-ride-frontend:latest -f deploy/docker/frontend.Dockerfile \
  --build-arg NEXT_PUBLIC_GO_API_URL=http://hdu-ride-backend:8080 \
  .
sudo docker pull postgres:18-alpine && sudo docker pull minio/minio:latest
sudo docker pull minio/mc:latest && sudo docker pull busybox:1.36
sudo docker pull rocker/rstudio:4.6.0

cd /tmp
sudo docker save hdu-ride-backend:latest -o backend.tar
sudo docker save hdu-ride-frontend:latest -o frontend.tar
sudo docker save postgres:18-alpine -o postgres.tar
sudo docker save minio/minio:latest -o minio.tar
sudo docker save minio/mc:latest -o minio-mc.tar
sudo docker save busybox:1.36 -o busybox.tar
sudo docker save rocker/rstudio:4.6.0 -o rstudio.tar

for f in backend frontend postgres minio minio-mc busybox rstudio; do
  sudo ctr -n k8s.io images import ${f}.tar
done

# === 第 9 步：部署 ===
cd /opt/hdu-ride && bash scripts/k8s-prod-up.sh

# === 第 10 步：Nginx + HTTPS ===
sudo nano /etc/nginx/sites-available/hdu-ride   # 参考第 12 节
sudo ln -sf /etc/nginx/sites-available/hdu-ride /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx
sudo certbot --nginx -d ride.mindsratch.top

# === 完成 ===
# 访问 https://ride.mindsratch.top，用 root 账号登录
```
