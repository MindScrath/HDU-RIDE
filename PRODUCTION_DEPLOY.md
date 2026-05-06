# HDU RIDE Ubuntu 生产环境部署与上线指南

本文档专为在 Ubuntu 云主机上进行生产环境正式部署而编写，目标域名以 `ride.mindsratch.top` 为例。

部署方案采用了 **标准版 Kubernetes (kubeadm) + 宿主机直接挂载内容目录 + Nginx 反向代理** 的架构。我们采用标准的生产级别 K8s，以满足高可用和严肃生产的需求。

这种架构的**最大优势**在于：
> **内容热更新**：通过 Kubernetes 的 HostPath 挂载，管理员只需要在 Ubuntu 宿主机的 `/opt/hdu-ride/content` 目录中修改/添加作业和讲义（甚至可以通过 Git 定时 Pull），网站内容就会**立即自动生效**，无需执行任何复杂的同步脚本或重启服务！

---

## 1. 基础环境准备

在您的 Ubuntu 云主机上，首先安装必要的软件：Docker、标准的 Kubernetes (kubeadm, kubelet, kubectl) 以及 Nginx。

```bash
# 1. 更新系统包并安装 Docker 和 Nginx
sudo apt update
sudo apt install -y curl git docker.io nginx apt-transport-https ca-certificates curl gpg

# 2. 安装生产级别标准 Kubernetes 组件
# 添加 Kubernetes 官方 GPG 密钥
curl -fsSL https://pkgs.k8s.io/core:/stable:/v1.29/deb/Release.key | sudo gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg
# 添加 Kubernetes apt 仓库
echo 'deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v1.29/deb/ /' | sudo tee /etc/apt/sources.list.d/kubernetes.list
# 安装 kubelet, kubeadm 和 kubectl，并锁定版本
sudo apt update
sudo apt install -y kubelet kubeadm kubectl
sudo apt-mark hold kubelet kubeadm kubectl

# 配置 Containerd 兼容 kubelet 的 Cgroup 驱动以及配置沙箱镜像
sudo mkdir -p /etc/containerd
containerd config default | sudo tee /etc/containerd/config.toml
sudo sed -i 's/SystemdCgroup = false/SystemdCgroup = true/' /etc/containerd/config.toml
sudo sed -i "s|sandbox = .*|sandbox = \"registry.aliyuncs.com/google_containers/pause:3.9\"|g" /etc/containerd/config.toml
sudo systemctl restart containerd

# 3. 初始化单节点 Kubernetes 集群
# 初始化前，需要开启内核模块以支持 iptables 桥接（这是 K8s 网络的要求）
sudo modprobe br_netfilter
echo "br_netfilter" | sudo tee /etc/modules-load.d/br_netfilter.conf
echo "net.bridge.bridge-nf-call-iptables=1" | sudo tee /etc/sysctl.d/k8s.conf
echo "net.ipv4.ip_forward=1" | sudo tee -a /etc/sysctl.d/k8s.conf
sudo sysctl --system

# 此外，kubeadm 要求禁用 swap
sudo swapoff -a
sudo sed -i '/ swap / s/^/#/' /etc/fstab

# 执行集群初始化 (如果在国内服务器，可以添加 --image-repository registry.aliyuncs.com/google_containers 加速拉取镜像)
sudo kubeadm init --pod-network-cidr=10.244.0.0/16 --image-repository registry.aliyuncs.com/google_containers

# 4. 配置 kubectl 权限 (允许当前非 root 用户直接使用 kubectl)
mkdir -p $HOME/.kube
sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config
sudo chown $(id -u):$(id -g) $HOME/.kube/config

# 5. 去除 Master 节点污点（由于是单机部署，必须允许 Pod 调度到主节点）
kubectl taint nodes --all node-role.kubernetes.io/control-plane-

# 6. 安装网络插件 (Flannel)
kubectl apply -f https://github.com/flannel-io/flannel/releases/latest/download/kube-flannel.yml
```

## 2. 获取代码与极简内容目录配置

将项目代码克隆到服务器，并建立专属的物理内容目录。

```bash
# 1. 克隆代码
cd /opt
sudo git clone <您的项目Git地址> hdu-ride
sudo chown -R $USER:$USER /opt/hdu-ride
cd /opt/hdu-ride

# 2. 创建内容物理目录
# 以后管理员发布作业和讲义，只需要修改这个目录里的文件即可！
mkdir -p /opt/hdu-ride/content
cp -r content/* /opt/hdu-ride/content/
```

> **原理解析**：我们在 `deploy/k8s/content-pvc-prod.yml` 中声明了一个指向 `/opt/hdu-ride/content` 的 PersistentVolume，并将其绑定到了 `hdu-ride-content` 这个 PVC 上。这使得后端的 `/content` 目录与宿主机实现了双向直通。

## 3. 构建生产环境镜像并导入集群

由于我们使用了原生的 Kubernetes 和 Containerd（或 Docker 作为底层运行时），我们需要将构建的镜像喂给节点。

```bash
cd /opt/hdu-ride

# 1. 编译后端镜像
sudo docker build -t hdu-ride-backend:latest -f deploy/docker/backend.Dockerfile .

# 2. 编译前端镜像
# (前端 Dockerfile 已配置自动下载 bun 并编译打包为 Nginx 静态服务)
sudo docker build -t hdu-ride-frontend:latest -f deploy/docker/frontend.Dockerfile .

# 3. 将 Docker 镜像加载到 Kubernetes 运行时 (如果使用的是 kubeadm + containerd)
sudo docker save hdu-ride-backend:latest -o backend.tar
sudo docker save hdu-ride-frontend:latest -o frontend.tar
sudo ctr -n=k8s.io images import backend.tar
sudo ctr -n=k8s.io images import frontend.tar
```

## 4. 部署服务到 Kubernetes 集群

首先准备环境秘钥：
```bash
cp .env.example .env
nano .env
```
> **`.env` 文件编辑指南**：
> 打开 `.env` 文件后，由于这是生产环境，您**必须**修改以下几项关键配置，以保证系统安全：
> 1. `POSTGRES_PASSWORD`: 修改为一个复杂的数据库密码。
> 2. `S3_ACCESS_KEY_ID` / `S3_SECRET_ACCESS_KEY`: 修改为 MinIO (对象存储) 的复杂账号密码。
> 3. `SESSION_SECRET`: 随意输入一段长且复杂的随机英文字符串（用于加密 Cookie）。
> 4. `ROOT_PASSWORD`: 设定您超级管理员 `root` 的登录密码（比如：`Admin@2026`）。
> 5. `ROOT_PASSWORD_HASH`: **这是最重要的一步！** 后端不能直接存明文密码，您需要在**本地电脑**（装有 Go 环境的地方）进入项目的 `backend` 目录，执行 `go run . hash-password 您的密码`，它会输出一串类似 `$2a$10$xxxx` 的字符。将这串字符复制，填写到服务器 `.env` 的 `ROOT_PASSWORD_HASH=` 后面。
> 6. `WORKSPACE_STORAGE_CLASS`: 确保其值为 `standard` (或其他您在原生 K8s 中安装并配置的 StorageClass 名称)，以支持动态分配。

为了方便一键启动生产服务，我们在 `scripts/` 下提供了一个生产部署脚本 `k8s-prod-up.sh`。可以直接执行，它会读取 `.env` 并创建所有 K8s 资源：

```bash
bash scripts/k8s-prod-up.sh
```

> **注意**：前端服务 `hdu-ride-frontend` 部署成功后，会通过 NodePort 暴露在宿主机的 `30080` 端口上。前端容器内自带的 Nginx 已配置好对后端的反代。

## 5. 配置 Nginx 反向代理与域名绑定

服务已在内部 `30080` 端口就绪，现在配置云主机外层真实的 Nginx 来接收 `ride.mindsratch.top` 的公网请求。

1. 新建 Nginx 配置文件：
```bash
sudo nano /etc/nginx/sites-available/hdu-ride
```

2. 写入以下内容（注意替换域名）：
```nginx
server {
    listen 80;
    server_name ride.mindsratch.top;

    # 我们强烈推荐后续使用 certbot 为域名配置 SSL/HTTPS
    # sudo apt install certbot python3-certbot-nginx
    # sudo certbot --nginx -d ride.mindsratch.top

    location / {
        # 代理到内部的 NodePort (前端容器)
        proxy_pass http://127.0.0.1:30080;
        proxy_http_version 1.1;
        
        # 必须：支持 WebSocket (RStudio IDE 需要)
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        
        # 传递真实客户端信息
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # 防止 RStudio 长连接断开
        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;
    }
}
```

3. 启用配置并重启 Nginx：
```bash
sudo ln -s /etc/nginx/sites-available/hdu-ride /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl restart nginx
```

## 6. 验收上线与日常维护

1. **解析域名**：前往您的域名服务商控制台，将 `ride.mindsratch.top` 的 A 记录指向该 Ubuntu 云主机的公网 IP。
2. **访问网站**：浏览器打开 `http://ride.mindsratch.top` 即可访问系统。
3. **初始化**：首次登录请使用 `root` 账号，创建班级并导入学生成员。学生必须在班级内才能正常启动 RStudio 工作区。
4. **日常发作业/更新讲义**：
   管理员只需将电脑上的 Markdown 或 YAML 配置文件，通过 SFTP/SCP 上传到云主机的 `/opt/hdu-ride/content` 目录中。由于做了物理挂载，您上传完毕后，**不需要重启任何服务**，前端直接刷新页面，新作业和讲义即刻生效！极大地提升了内容发布效率。