# HDU RIDE Ubuntu 22.04 / 24.04 从 0 到公网部署手册

本文档面向第一次接触本项目、第一次在 Ubuntu 云主机上部署 Kubernetes 应用的用户。

目标是让你在一台全新的 Ubuntu 22.04 或 24.04 云主机上，把 `HDU RIDE` 从零部署成功，并最终通过域名访问，例如：

- `http://ride.mindsratch.top`
- `https://ride.mindsratch.top`

本文档不仅覆盖“怎么部署”，还覆盖：

- 这个项目到底由哪些部分组成
- 每条命令的作用是什么
- 内容如何更新
- 代码如何升级
- 常见错误如何排查

如果你只想先知道最终架构，可以先看“部署后的整体结构”。

---

## 1. 项目是什么

`HDU RIDE` 是一个面向教学场景的平台，核心能力有：

- 课程内容展示：讲义、章节、作业说明来自仓库中的 `content/`
- 班级与成员管理：由后端和 PostgreSQL 维护
- 作业提交与批改：提交文件保存在 MinIO
- RStudio 在线工作区：每个学生/作业对应一个独立的 Kubernetes Pod、PVC、Service
- 管理端课程导入与重载：管理员可以导入课程 zip，也可以让后端重新加载磁盘上的课程内容

### 1.1 仓库结构

- `backend/`
  - Go 后端
  - 提供登录、班级、作业、提交、评分、课程导入、RStudio 工作区管理
- `frontend/`
  - Vue 3 + Vite + Element Plus 前端
  - 对外提供网站页面
- `content/`
  - 课程内容目录
  - 讲义、作业说明、starter、公开数据、公开测试都在这里
- `deploy/docker/`
  - 后端镜像、前端镜像、RStudio 镜像、前端容器内 Nginx 配置
- `deploy/k8s/`
  - Kubernetes 清单
  - 包括 PostgreSQL、MinIO、后端、前端、内容卷等
- `scripts/`
  - 部署脚本入口
  - `k8s-prod-up.sh` 用于生产部署

### 1.2 运行时架构

部署完成后，组件关系如下：

1. 外层公网流量先到 Ubuntu 主机上的 Nginx
2. Nginx 将流量反代到 Kubernetes 内部前端入口 `127.0.0.1:30080`
3. 前端容器内的 Nginx 负责：
   - 静态页面
   - `/api` 转发给 Go 后端
   - `/ide` 转发给 Go 后端
4. Go 后端负责：
   - 操作 PostgreSQL
   - 读取课程内容
   - 读写 MinIO 对象存储
   - 创建/销毁学生的 RStudio Pod、PVC、Service、NetworkPolicy
   - 将 `/ide/s/:workspaceID/` 代理到对应的 RStudio Pod
5. 学生作业工作区数据保存在动态 PVC 中
6. 课程内容来自宿主机目录 `/opt/hdu-ride/content`

### 1.3 这个部署方案的特点

本文档采用：

- `kubeadm` 单节点 Kubernetes
- `containerd` 作为 Kubernetes 运行时
- `Flannel` 作为容器网络
- `local-path-provisioner` 作为动态存储
- 宿主机 `hostPath` 挂载课程内容目录
- 宿主机 Nginx 负责域名和 HTTPS

这样做的优点是：

- 架构清晰，和正式 Kubernetes 一致
- 不依赖 K3s
- 内容目录就在宿主机上，管理员容易管理
- 后续升级前后端镜像比较直接
- 域名、HTTPS、反向代理都在宿主机 Nginx 上统一管理

---

## 2. 先决条件

开始之前，请确认你具备以下条件。

### 2.1 云主机建议配置

最低建议：

- 4 核 CPU
- 8 GB 内存
- 80 GB 以上系统盘

更稳妥的建议：

- 8 核 CPU
- 16 GB 内存
- 100 GB 以上磁盘

原因：

- PostgreSQL、MinIO、前端、后端本身会占用资源
- 每个 RStudio 工作区还会创建独立 Pod 和 PVC
- 如果教师批改、学生同时进入 RStudio，资源占用会明显增加

### 2.2 域名准备

假设你有域名 `ride.mindsratch.top`，你需要：

1. 在域名服务商后台添加一条 `A` 记录
2. 将 `ride.mindsratch.top` 指向你的云主机公网 IP
3. 等待 DNS 生效

可验证：

```bash
nslookup ride.mindsratch.top
```

如果解析结果已经是你的云主机公网 IP，就可以继续。

### 2.3 云厂商安全组

请在云平台控制台放行以下端口：

- `22/tcp`：SSH
- `80/tcp`：HTTP
- `443/tcp`：HTTPS

如果你只通过本机执行 `kubectl`，则不需要对公网开放 `6443`。

`30080` 是 NodePort，但本文档使用宿主机 Nginx 回环代理，不建议直接对公网开放 `30080`。

---

## 3. 部署后的整体结构

部署完成后，主机上的关键目录和职责如下：

- `/opt/hdu-ride`
  - 项目仓库
- `/opt/hdu-ride/content`
  - 管理员直接维护的课程内容目录
  - 网站讲义、作业、starter、公开数据都从这里读取
- `/etc/nginx/sites-available/hdu-ride`
  - 域名反代配置
- `/etc/kubernetes/`
  - kubeadm 初始化出的 Kubernetes 配置
- `/var/lib/containerd/`
  - containerd 镜像与运行数据
- `/var/lib/rancher/local-path-provisioner/`
  - local-path 动态存储实际落盘位置

---

## 4. Ubuntu 基础初始化

以下步骤在全新 Ubuntu 22.04 / 24.04 上执行。

### 4.1 更新系统并安装基础工具

```bash
sudo apt update
sudo apt install -y \
  curl \
  wget \
  git \
  vim \
  nano \
  jq \
  unzip \
  ca-certificates \
  gnupg \
  lsb-release \
  apt-transport-https \
  software-properties-common \
  nginx \
  docker.io
```

这条命令的作用：

- `curl/wget/git`：下载与拉取代码
- `vim/nano`：编辑配置文件
- `jq`：调试接口时很有用
- `nginx`：公网反向代理
- `docker.io`：本地构建镜像

### 4.2 启动并设置服务开机自启

```bash
sudo systemctl enable --now docker
sudo systemctl enable --now nginx
sudo systemctl enable --now containerd
```

说明：

- `docker` 用来构建镜像
- `containerd` 是 Kubernetes 实际使用的运行时
- `nginx` 是公网入口

### 4.3 可选：设置时区

```bash
timedatectl
sudo timedatectl set-timezone Asia/Shanghai
```

---

## 5. 安装 Go

本仓库的生产部署脚本 `scripts/k8s-prod-up.sh` 最终会调用：

```bash
cd backend
go run . ops k8s-prod-up
```

所以服务器上必须安装 Go。

当前仓库的 Go 版本要求以 `backend/go.mod` 为准。当前仓库写的是 `go 1.26`。

以下命令以 `amd64` 服务器为例安装 Go 1.26：

```bash
cd /tmp
curl -LO https://go.dev/dl/go1.26.0.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.26.0.linux-amd64.tar.gz
echo 'export PATH=/usr/local/go/bin:$PATH' | sudo tee /etc/profile.d/go.sh
source /etc/profile.d/go.sh
go version
```

如果你的机器不是 `amd64`，请改成对应架构的安装包。

---

## 6. 安装 Kubernetes

这里使用的是标准 `kubeadm`，不是 K3s。

### 6.1 安装 kubeadm / kubelet / kubectl

```bash
sudo mkdir -p /etc/apt/keyrings
curl -fsSL https://pkgs.k8s.io/core:/stable:/v1.29/deb/Release.key | sudo gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg
echo 'deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v1.29/deb/ /' | sudo tee /etc/apt/sources.list.d/kubernetes.list
sudo apt update
sudo apt install -y kubelet kubeadm kubectl
sudo apt-mark hold kubelet kubeadm kubectl
sudo systemctl enable --now kubelet
```

作用：

- `kubeadm`：初始化集群
- `kubelet`：节点代理
- `kubectl`：命令行管理工具

### 6.2 打开内核参数并关闭 swap

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

作用：

- 允许桥接网络通过 iptables 处理
- 打开 IPv4 转发
- 关闭 swap，满足 kubeadm 要求

### 6.3 配置 containerd

很多国内服务器初始化失败，根因是：

- `SystemdCgroup` 没有打开
- sandbox `pause` 镜像默认走 `registry.k8s.io`

先生成配置：

```bash
sudo mkdir -p /etc/containerd
containerd config default | sudo tee /etc/containerd/config.toml >/dev/null
```

再修改 `SystemdCgroup`：

```bash
sudo sed -i 's/SystemdCgroup = false/SystemdCgroup = true/' /etc/containerd/config.toml
```

再把 sandbox 镜像改成国内可拉取地址：

```bash
sudo sed -i "s|sandbox = .*|sandbox = 'registry.aliyuncs.com/google_containers/pause:3.9'|g" /etc/containerd/config.toml
```

最后重启：

```bash
sudo systemctl restart containerd
sudo systemctl restart kubelet
```

### 6.4 初始化单节点集群

```bash
sudo kubeadm init \
  --pod-network-cidr=10.244.0.0/16 \
  --image-repository registry.aliyuncs.com/google_containers
```

成功后，会打印一段提示，包含：

- `Your Kubernetes control-plane has initialized successfully!`

### 6.5 配置当前用户的 kubectl

```bash
mkdir -p $HOME/.kube
sudo cp /etc/kubernetes/admin.conf $HOME/.kube/config
sudo chown $(id -u):$(id -g) $HOME/.kube/config
kubectl get nodes
```

### 6.6 允许工作负载调度到控制平面节点

单机部署必须执行：

```bash
kubectl taint nodes --all node-role.kubernetes.io/control-plane-
kubectl taint nodes --all node-role.kubernetes.io/master-
```

说明：

- 不同发行版或不同安装方式下，污点键可能是 `control-plane`，也可能是 `master`
- 在单节点 Kubernetes 中，必须把这两类常见污点都移除，避免所有业务 Pod 因 `NoSchedule` 卡死

### 6.7 安装 Flannel 网络插件

```bash
kubectl apply -f https://github.com/flannel-io/flannel/releases/latest/download/kube-flannel.yml
```

等待网络组件启动：

```bash
kubectl get pods -n kube-system
```

看到 `coredns`、`kube-flannel` 变成 `Running` 再继续。

---

## 7. 安装动态存储

这是生产部署中非常容易漏掉的一步。

本项目有三类需要存储的东西：

1. PostgreSQL 数据卷
2. MinIO 数据卷
3. 学生 RStudio 工作区 PVC

其中：

- PostgreSQL 和 MinIO 的 StatefulSet 会申请 PVC
- 学生工作区会动态创建 PVC

如果你没有动态存储类，Pod 会一直卡在 `Pending`。

### 7.1 固定版本说明

为了方便用户安装，仓库已经直接内置了固定版本的存储类清单：

- [local-path-storage-v0.0.28.yaml](file:///d:/Go/HDU-RIDE/deploy/k8s/local-path-storage-v0.0.28.yaml)

其中基础镜像版本已经锁死为：

- `rancher/local-path-provisioner:v0.0.28`
- `busybox:1.36`

另外，仓库还内置了单节点专用的存储类定义：

- [storageclasses-single-node.yml](file:///d:/Go/HDU-RIDE/deploy/k8s/storageclasses-single-node.yml)

这里做了两件关键事情：

1. 把 `local-path` 的 `volumeBindingMode` 强制改成 `Immediate`
2. 创建 `standard` 别名，同样指向 `rancher.io/local-path`，并且同样使用 `Immediate`

这意味着以下几处天然保持一致：

1. 仓库中的 YAML 清单
2. 安装脚本预拉取与导入的镜像
3. 集群运行时引用的镜像
4. 单节点环境里实际使用的 `StorageClass` 绑定策略

### 7.2 一键安装 local-path-provisioner

直接执行：

```bash
cd /opt/hdu-ride
bash scripts/k8s-install-local-path.sh
```

这个脚本会自动完成：

1. 预拉取 `rancher/local-path-provisioner:v0.0.28`
2. 预拉取 `busybox:1.36`
3. 导入 `containerd`
4. 强制移除单节点常见的 `control-plane/master` 调度污点
5. 应用仓库中的固定版本 provisioner YAML
6. 删除并重建 `local-path` 与 `standard` 两个 `StorageClass`
7. 将两者的 `volumeBindingMode` 固定为 `Immediate`
8. 把 `standard` 设为默认 `StorageClass`
9. 等待 `local-path-provisioner` 就绪

如果你所在环境只能从镜像代理拉取，也不需要手工改 YAML，只要这样执行：

```bash
cd /opt/hdu-ride
LOCAL_PATH_PROVISIONER_PULL_IMAGE=<你的镜像代理地址>/rancher/local-path-provisioner:v0.0.28 \
LOCAL_PATH_HELPER_PULL_IMAGE=<你的镜像代理地址>/busybox:1.36 \
bash scripts/k8s-install-local-path.sh
```

### 7.3 验证

```bash
kubectl get storageclass
```

你应该能看到：

- `local-path`
- `standard`
- 其中 `standard` 带有 `(default)` 标记
- 两者的 `VOLUMEBINDINGMODE` 都应为 `Immediate`

---

## 8. 获取项目代码

统一把项目放到 `/opt/hdu-ride`。

```bash
cd /opt
sudo git clone <你的仓库地址> hdu-ride
sudo chown -R $USER:$USER /opt/hdu-ride
cd /opt/hdu-ride
```

如果代码已经存在，后续升级用：

```bash
cd /opt/hdu-ride
git pull
```

---

## 9. 准备课程内容目录

生产环境的课程内容不是通过开发模式的同步脚本写入 PVC，而是直接使用宿主机目录：

- 宿主机目录：`/opt/hdu-ride/content`
- 容器挂载路径：`/content`

这来自 `deploy/k8s/content-pvc-prod.yml` 里的 `hostPath`。

### 9.1 静态 PV 的命名空间一致性说明

这里有一个非常容易忽略的点。

虽然 `PersistentVolume` 自身是集群级资源，不属于某个命名空间，但它要绑定的 `PersistentVolumeClaim` 是创建在 `hdu-ride` 命名空间中的。因此在生产环境里，手动创建静态 PV 作为内容卷时，必须同时保证下面两件事：

1. `PersistentVolumeClaim` 的命名空间正确，是 `hdu-ride`
2. 静态 PV 与静态 PVC 的 `storageClassName` 和集群默认动态存储类保持一致，例如本文档默认的 `standard`

如果这里不一致，Kubernetes 会因为 `VolumeMismatch` 拒绝绑定，最终表现为：

- `hdu-ride-content` 一直无法绑定
- 后端 Pod 因挂载内容卷失败而长期处于 `Pending`

当前仓库已经在 [content-pvc-prod.yml](file:///d:/Go/HDU-RIDE/deploy/k8s/content-pvc-prod.yml) 中把内容卷的 `storageClassName` 默认固定为 `standard`。如果你修改了 `.env` 中的 `WORKSPACE_STORAGE_CLASS`，请同步更新这个文件中的 `storageClassName`，否则会因为 `VolumeMismatch` 无法绑定。生产脚本 [k8s-prod-up.sh](file:///d:/Go/HDU-RIDE/scripts/k8s-prod-up.sh) 会在正式部署前做这类一致性检查；如果发现集群里已经存在属性不一致的静态 PV/PVC，还会先删除旧对象再继续 `apply`。

### 9.2 初始化内容目录

```bash
mkdir -p /opt/hdu-ride/content
test -d /opt/hdu-ride/content/courses && echo "content 目录已就绪"
```

说明：

- 本仓库克隆到 `/opt/hdu-ride` 后，仓库内的 `content/` 本身就是生产内容目录
- `deploy/k8s/content-pvc-prod.yml` 会把 `/opt/hdu-ride/content` 直接挂载给后端容器
- 所以后续管理员直接维护 `/opt/hdu-ride/content` 即可，不需要再做额外同步
- 如果担心误操作，建议把 `/opt/hdu-ride` 纳入 Git 管理并定期备份

### 9.3 内容目录结构

课程目录大致如下：

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
          tests/hidden/
```

注意：

- `tests/hidden/` 不会复制到学生工作区
- `starter/` 会进入学生初始工作区
- `README.md` 是作业说明

---

## 10. 准备生产环境变量

### 10.1 从模板复制

```bash
cd /opt/hdu-ride
cp .env.example .env
nano .env
```

### 10.2 生产环境下哪些变量最重要

请至少确认以下字段：

```dotenv
POSTGRES_DB=hdu_ride
POSTGRES_USER=hdu
POSTGRES_PASSWORD=请换成强密码

S3_BUCKET=hdu-ride
S3_ACCESS_KEY_ID=请换成你自己的MinIO账号
S3_SECRET_ACCESS_KEY=请换成你自己的MinIO密码
S3_USE_SSL=false

SESSION_SECRET=请换成长随机字符串

ROOT_USERNAME=root
ROOT_PASSWORD=请先自定义一个管理员密码
ROOT_PASSWORD_HASH=这里稍后填写bcrypt结果

K8S_NAMESPACE=hdu-ride
WORKSPACE_IMAGE_DEFAULT=rocker/rstudio:4.6.0
WORKSPACE_STORAGE_CLASS=standard
WORKSPACE_CPU_REQUEST=500m
WORKSPACE_CPU_LIMIT=1
WORKSPACE_MEM_REQUEST=1Gi
WORKSPACE_MEM_LIMIT=2Gi

BACKEND_IMAGE=hdu-ride-backend:latest
FRONTEND_IMAGE=hdu-ride-frontend:latest
```

重点解释：

- `POSTGRES_PASSWORD`
  - PostgreSQL 密码
- `S3_ACCESS_KEY_ID` / `S3_SECRET_ACCESS_KEY`
  - MinIO 管理账号密码
- `SESSION_SECRET`
  - 用于会话签名，必须足够随机
- `ROOT_USERNAME`
  - 管理员用户名，通常保留 `root`
- `ROOT_PASSWORD_HASH`
  - root 密码的 bcrypt 哈希
- `WORKSPACE_STORAGE_CLASS`
  - 一定要与你安装的动态存储一致
  - 本文档默认使用 `standard`
  - `standard` 是单节点环境下指向 `local-path` 的兼容别名，两者都使用 `Immediate`
- `WORKSPACE_IMAGE_DEFAULT`
  - 默认的 RStudio 工作区镜像
  - 当前仓库默认用 `rocker/rstudio:4.6.0`

### 10.3 生成 ROOT_PASSWORD_HASH

例如你希望 root 密码是 `Admin@2026`，执行：

```bash
cd /opt/hdu-ride/backend
go run . hash-password 'Admin@2026'
```

输出会类似：

```text
$2a$10$xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

把这段复制到 `.env`：

```dotenv
ROOT_PASSWORD_HASH=$2a$10$xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

注意：

- `.env` 只是普通文件，不会像 shell 那样展开 `$`
- 所以直接粘贴 bcrypt 值即可
- 不要在生产中继续保留默认 `change-me`

### 10.4 关于 `.env.example` 里看起来“像开发环境”的项

模板里有这些字段：

- `DATABASE_URL`
- `CONTENT_ROOT`
- `S3_ENDPOINT`
- `HTTP_ADDR`

在生产部署脚本里，真正传给后端容器的值会由 `backend/ops.go` 自动生成并写入 Kubernetes Secret。

所以：

- 生产部署最关键的是上面列出的密码、命名空间、存储类、镜像、资源限制
- `DATABASE_URL`、`CONTENT_ROOT`、`S3_ENDPOINT` 可以保留模板值，不影响 `k8s-prod-up.sh`

---

## 11. 构建并预加载镜像

这一步很重要。

因为生产脚本只会把 Kubernetes 清单 apply 到集群里，如果相关镜像没有提前导入到 `containerd`，Kubernetes 就会在运行时去公网拉镜像，国内环境经常会慢、会超时。

建议把用到的镜像全部提前准备好。

### 11.1 构建前后端镜像

```bash
cd /opt/hdu-ride
sudo docker build -t hdu-ride-backend:latest -f deploy/docker/backend.Dockerfile .
sudo docker build -t hdu-ride-frontend:latest -f deploy/docker/frontend.Dockerfile .
```

作用：

- 生成生产可用的后端镜像
- 生成生产可用的前端镜像

### 11.2 预拉取运行期镜像

```bash
sudo docker pull postgres:18-alpine
sudo docker pull minio/minio:latest
sudo docker pull minio/mc:latest
sudo docker pull busybox:1.36
sudo docker pull rocker/rstudio:4.6.0
```

作用：

- `postgres:18-alpine`
  - PostgreSQL 数据库镜像
- `minio/minio:latest`
  - MinIO 服务端镜像
- `minio/mc:latest`
  - 初始化 bucket 用的客户端镜像
- `busybox:1.36`
  - 给 RStudio 工作区预填 starter 内容的 init container
  - local-path 存储类的 helperPod 也使用这个固定版本
- `rocker/rstudio:4.6.0`
  - 学生与教师使用的 RStudio 工作区镜像

### 11.3 导入到 containerd

Kubernetes 运行时看的是 `containerd`，不是 Docker 守护进程。

所以必须把镜像从 Docker 再导入 containerd。

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

可验证：

```bash
sudo ctr -n k8s.io images list | grep -E 'hdu-ride|postgres|minio|local-path-provisioner|busybox|rstudio'
```

### 11.4 可选：使用自定义 RStudio 镜像

仓库里还有 `deploy/docker/rstudio.Dockerfile`，它会在 `rocker/rstudio:4.6.0` 基础上安装：

- `tidyverse`
- `rmarkdown`
- `renv`

如果你希望学生开箱即用这些包，可以自己构建：

```bash
cd /opt/hdu-ride
sudo docker build -t hdu-ride-rstudio:latest -f deploy/docker/rstudio.Dockerfile .
sudo docker save hdu-ride-rstudio:latest -o hdu-ride-rstudio.tar
sudo ctr -n k8s.io images import hdu-ride-rstudio.tar
```

然后把 `.env` 中改成：

```dotenv
WORKSPACE_IMAGE_DEFAULT=hdu-ride-rstudio:latest
```

如果你不确定，就先继续使用默认的 `rocker/rstudio:4.6.0`。

---

## 12. 执行生产部署

### 12.1 运行一键部署脚本

```bash
cd /opt/hdu-ride
bash scripts/k8s-prod-up.sh
```

这个脚本在真正开始部署前，会先做一轮环境检查，至少包括：

- 强制移除单节点常见的 `control-plane/master` 调度污点
- 生产内容卷清单 [content-pvc-prod.yml](file:///d:/Go/HDU-RIDE/deploy/k8s/content-pvc-prod.yml) 中的 PV/PVC 是否都声明了与 `.env` 中 `WORKSPACE_STORAGE_CLASS` 一致的 `storageClassName`
- 生产内容卷清单中的 PV/PVC 容量是否保持一致
- 如果集群里已经存在静态 PV/PVC，但其 `storageClassName` 或容量与当前期望不一致，先删除旧对象再继续部署

如果检查不通过，脚本会直接退出并给出错误提示；如果只是发现集群里已有旧的错误静态 PV/PVC，则会先删除旧对象，避免因为不可变字段导致修复失败。

这条命令实际会：

1. 进入 `backend/`
2. 用 `go run . ops k8s-prod-up` 调用运维入口
3. 读取仓库根目录 `.env`
4. 创建或更新以下资源：
   - `namespace`
   - `Secret`
   - `content-pvc-prod`
   - `postgres`
   - `minio`
   - `hdu-ride-backend`
   - `hdu-ride-frontend`
5. 等待 PostgreSQL 和 MinIO Ready
6. 用 `minio/mc` 初始化 bucket
7. 设置前后端 Deployment 的镜像并等待 rollout 成功

### 12.2 验证部署是否完成

```bash
kubectl get pods -n hdu-ride
kubectl get svc -n hdu-ride
kubectl get pvc -n hdu-ride
```

理想状态下你会看到：

- `postgres-0` 为 `Running`
- `minio-0` 为 `Running`
- `hdu-ride-backend-...` 为 `Running`
- `hdu-ride-frontend-...` 为 `Running`
- 前端 Service 类型为 `NodePort`

前端会暴露为：

- `127.0.0.1:30080`

可在服务器本机执行：

```bash
curl -I http://127.0.0.1:30080
```

如果返回 `200 OK` 或 `304` 等正常 HTTP 响应，说明站点入口已经在机内可用。

---

## 13. 配置域名反代

现在 Kubernetes 内部服务已经运行，但公网还访问不到，需要宿主机 Nginx 代理域名。

### 13.1 创建 Nginx 站点文件

```bash
sudo nano /etc/nginx/sites-available/hdu-ride
```

写入以下内容：

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

        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";

        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;
    }
}
```

解释：

- `proxy_pass http://127.0.0.1:30080`
  - 代理到前端 NodePort
- `Upgrade` / `Connection`
  - RStudio 依赖 WebSocket，必须保留
- `proxy_read_timeout 3600s`
  - 避免 RStudio 长连接中断

### 13.2 启用站点

```bash
sudo ln -sf /etc/nginx/sites-available/hdu-ride /etc/nginx/sites-enabled/hdu-ride
sudo nginx -t
sudo systemctl reload nginx
```

### 13.3 先验证 HTTP

现在打开：

- `http://ride.mindsratch.top`

如果域名解析和 Nginx 正常，你应该能看到登录页。

---

## 14. 配置 HTTPS

建议正式环境一定上 HTTPS。

### 14.1 安装 certbot

```bash
sudo apt install -y certbot python3-certbot-nginx
```

### 14.2 申请证书

```bash
sudo certbot --nginx -d ride.mindsratch.top
```

证书申请成功后，Certbot 会自动改写 Nginx 配置并启用 HTTPS。

### 14.3 验证自动续期

```bash
sudo systemctl status certbot.timer
sudo certbot renew --dry-run
```

最终你应该可以通过：

- `https://ride.mindsratch.top`

访问系统。

---

## 15. 首次上线后的业务初始化

站点能打开，只代表“服务起来了”，不代表“业务数据已经初始化好”。

你还需要做这些事情。

### 15.1 管理员登录

使用 `.env` 里设置的：

- `ROOT_USERNAME`
- `ROOT_PASSWORD`

登录。

### 15.2 创建班级

进入管理界面，创建班级，并为班级指定课程。

### 15.3 创建学生账号

在用户管理中创建学生账号。

### 15.4 把学生加入班级

这一步不能漏。

如果学生没有加入任何班级，会出现这些现象：

- 看不到作业
- 打不开作业工作区
- 打开 RStudio 时 403 或无反应

---

## 16. 管理员如何更新网站内容

这是生产运维里最重要的日常动作。

### 16.1 先理解一个关键事实

课程内容目录虽然是宿主机直挂载，但后端会在启动时把课程内容加载到内存里。

所以：

- 修改 `/opt/hdu-ride/content/...` 后
- 文件已经进入容器
- 但前端页面不会自动立刻刷新出新内容

你需要再执行一次“课程重载”。

好消息是：

- 不需要重建镜像
- 不需要重启数据库
- 不需要重启整个站点

### 16.2 推荐更新方式 A：直接改宿主机目录，再重载

适合管理员通过 Git、SFTP、VS Code Remote SSH 维护内容。

步骤：

1. 修改宿主机内容目录：

```bash
cd /opt/hdu-ride/content
```

2. 按课程结构编辑讲义、作业说明、starter、测试数据

3. 用管理员账号登录网站

4. 进入“课程导入/管理”页，点击“重新加载”

这会调用后端的：

- `POST /api/admin/courses/reload`

从而让后端重新扫描 `/content/courses`

### 16.3 推荐更新方式 B：通过管理端导入课程 zip

前端已有管理员课程导入页，可以导入课程压缩包。

课程 zip 结构应类似：

```text
course.yml
chapters/
assignments/
```

导入成功后，后端会自动触发课程重载。

### 16.4 什么时候需要重建镜像，什么时候不需要

不需要重建镜像的情况：

- 修改 `content/` 中的讲义
- 修改作业说明 `README.md`
- 修改 `starter/`
- 修改公开数据 `data/public`
- 修改公开测试 `tests/public`
- 修改隐藏测试 `tests/hidden`

需要重建镜像的情况：

- 修改 Go 后端代码
- 修改 Vue 前端代码
- 修改前端容器内 Nginx 配置
- 修改 RStudio 自定义镜像内容

---

## 17. 如何升级代码

### 17.1 升级后端或前端代码

```bash
cd /opt/hdu-ride
git pull
```

如果改了后端或前端代码，重新构建并导入镜像：

```bash
sudo docker build -t hdu-ride-backend:latest -f deploy/docker/backend.Dockerfile .
sudo docker build -t hdu-ride-frontend:latest -f deploy/docker/frontend.Dockerfile .

cd /tmp
sudo docker save hdu-ride-backend:latest -o hdu-ride-backend.tar
sudo docker save hdu-ride-frontend:latest -o hdu-ride-frontend.tar
sudo ctr -n k8s.io images import hdu-ride-backend.tar
sudo ctr -n k8s.io images import hdu-ride-frontend.tar
```

然后重新执行生产脚本：

```bash
cd /opt/hdu-ride
bash scripts/k8s-prod-up.sh
```

### 17.2 仅重启某个 Deployment

如果你已经导入了同名新镜像，也可以手动重启：

```bash
kubectl rollout restart deployment/hdu-ride-backend -n hdu-ride
kubectl rollout restart deployment/hdu-ride-frontend -n hdu-ride
kubectl rollout status deployment/hdu-ride-backend -n hdu-ride
kubectl rollout status deployment/hdu-ride-frontend -n hdu-ride
```

---

## 18. 验收清单

上线后请按下面顺序验收。

### 18.1 基础服务

```bash
kubectl get pods -n hdu-ride
kubectl get svc -n hdu-ride
kubectl get pvc -n hdu-ride
kubectl get storageclass
```

检查点：

- `postgres-0` 运行
- `minio-0` 运行
- `hdu-ride-backend` 运行
- `hdu-ride-frontend` 运行
- `local-path` 为默认 StorageClass

### 18.2 网站入口

检查：

- `http://ride.mindsratch.top`
- `https://ride.mindsratch.top`

### 18.3 管理员业务

检查：

- root 能登录
- 能看到班级管理页面
- 能创建班级
- 能创建学生账号
- 能把学生加入班级

### 18.4 学生业务

检查：

- 学生能看到作业
- 学生能打开 RStudio
- 学生能提交作业

### 18.5 教师批改

检查：

- 教师能看到提交列表
- 教师点击批阅能打开学生工作区

### 18.6 内容发布

检查：

- 修改 `/opt/hdu-ride/content/...`
- 管理员点击“重新加载”
- 前端能看到新内容

---

## 19. 常见问题排查

### 19.1 `kubeadm init` 报桥接网络或 swap 错误

现象：

- `/proc/sys/net/bridge/bridge-nf-call-iptables does not exist`
- swap 相关报错

处理：

- 重新执行“打开内核参数并关闭 swap”那一节

### 19.2 `kubeadm init` 卡在 waiting for control plane

优先检查：

```bash
systemctl status kubelet
journalctl -xeu kubelet
```

常见原因：

- `SystemdCgroup` 没开
- sandbox `pause` 镜像走国外源
- CNI 未安装

### 19.3 `scripts/k8s-prod-up.sh` 卡在 PostgreSQL / MinIO Ready 之后

通常是 `minio/mc` 镜像没有提前导入，脚本在等它。

先检查：

```bash
kubectl get pods -n hdu-ride
kubectl get events -n hdu-ride --sort-by=.lastTimestamp
```

如果 `minio-mc` 拉镜像慢，按本文档的镜像预加载步骤先导入 `minio/mc:latest`。

### 19.4 网站能打开，但打不开 RStudio

优先排查这几类问题：

1. 学生没有加入班级
2. `local-path` 没装，PVC 一直 `Pending`
3. `rocker/rstudio:4.6.0` 没有提前导入，首次拉镜像超时
4. Nginx 没有配置 WebSocket 头

检查命令：

```bash
kubectl get pods -n hdu-ride
kubectl get pvc -n hdu-ride
kubectl describe pod <rstudio-pod名字> -n hdu-ride
```

### 19.5 教师批阅时 500 或 502

先看后端日志：

```bash
kubectl logs -l app.kubernetes.io/name=hdu-ride-backend -n hdu-ride --tail=200
```

再看相关 RStudio Pod：

```bash
kubectl get pods -n hdu-ride
kubectl describe pod <rstudio-pod名字> -n hdu-ride
kubectl logs <rstudio-pod名字> -n hdu-ride -c rstudio
```

如果是 `workspace gateway error`，重点检查：

- `coredns` 是否正常
- `Flannel` 是否正常
- 宿主机防火墙是否拦截了 Pod 网络

如果启用了 `ufw`，建议至少确认：

```bash
sudo ufw status
```

在单机环境里，如遇到 Pod 间流量异常，可先排除防火墙影响：

```bash
sudo ufw disable
sudo iptables -P FORWARD ACCEPT
```

### 19.6 修改了 `content/`，但页面没变化

原因不是挂载失败，而是后端还没重载内容。

做法：

- 登录管理员后台
- 打开课程管理页
- 点击“重新加载”

### 19.7 Pod 一直 `Pending`

通常是两种原因：

1. 资源不足
2. 存储类缺失

看事件：

```bash
kubectl describe pod <pod名字> -n hdu-ride
```

如果是资源不足，可调低 `.env` 中：

- `WORKSPACE_CPU_REQUEST`
- `WORKSPACE_MEM_REQUEST`

例如：

```dotenv
WORKSPACE_CPU_REQUEST=250m
WORKSPACE_MEM_REQUEST=512Mi
```

---

## 20. 生产环境运维建议

### 20.1 不要直接把默认密码上线

必须修改：

- PostgreSQL 密码
- MinIO 账号密码
- root 管理员密码
- `SESSION_SECRET`

### 20.2 定期备份

至少要备份：

- PostgreSQL 数据
- MinIO 数据
- `/opt/hdu-ride/content`
- `.env`

### 20.3 把内容目录纳入 Git 管理

推荐做法：

- 仓库代码和 `/opt/hdu-ride/content` 都放在 Git 下管理
- 内容更新走 commit
- 管理员更新内容后只需重载课程

### 20.4 修改内容优先，不要轻易改生产数据库

讲义、作业说明、starter、测试数据都应该通过内容目录维护，而不是手工改容器里的文件。

---

## 21. 最终推荐的日常操作清单

### 21.1 首次部署

按顺序执行：

1. 安装基础工具
2. 安装 Go
3. 安装 Kubernetes
4. 安装 Flannel
5. 安装 local-path-provisioner
6. 克隆仓库
7. 准备 `/opt/hdu-ride/content`
8. 配 `.env`
9. 构建并导入镜像
10. 执行 `bash scripts/k8s-prod-up.sh`
11. 配宿主机 Nginx
12. 配 HTTPS
13. 用 root 登录验收

### 21.2 更新课程内容

1. 修改 `/opt/hdu-ride/content`
2. 管理员后台点击“重新加载”

### 21.3 更新后端前端代码

1. `git pull`
2. `docker build`
3. `ctr import`
4. 重新执行 `bash scripts/k8s-prod-up.sh`

---

## 22. 你应该记住的三句话

1. 内容更新不等于代码更新，改 `content/` 通常不需要重建镜像。
2. 改了内容后要“重新加载课程”，因为后端会把课程读进内存。
3. 裸机 `kubeadm` 单节点部署时，动态存储和 Pod 网络是最容易漏掉的两件事。

如果你严格照着这份文档操作，最终应该能够通过 `ride.mindsratch.top` 正常访问网站，并完成：

- 管理员登录
- 班级与用户管理
- 学生打开 RStudio
- 学生提交作业
- 教师在线批阅
