# HDU RIDE Ubuntu 22.04 / 24.04 从 0 到公网部署手册

本文档面向第一次在 Ubuntu 云主机上部署本项目的用户。

目标是把 `HDU RIDE` 从零部署到公网，并通过域名访问，例如：

- `http://ride.mindsratch.top`
- `https://ride.mindsratch.top`

文档同时覆盖：

- 部署步骤
- 内容更新
- 代码升级
- 常见排障

---

## 1. 项目是什么

`HDU RIDE` 是一个教学平台，核心组件如下：

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

部署后链路如下：

1. 公网流量先进入宿主机 Nginx
2. Nginx 反代到前端 NodePort `127.0.0.1:30080`
3. 前端容器内 Nginx 提供静态页面，并将 `/api`、`/ide` 转发给 Go 后端
4. Go 后端负责 PostgreSQL、MinIO、课程内容和 RStudio 工作区管理
5. 学生工作区数据保存在动态 PVC 中，课程内容来自 `/opt/hdu-ride/content`

### 1.3 本文档采用的方案

- 单节点 `kubeadm` Kubernetes
- `containerd` 作为运行时
- `Flannel` 作为容器网络
- `local-path-provisioner` 作为动态存储
- 宿主机 `hostPath` 挂载课程内容目录
- 宿主机 Nginx 负责域名和 HTTPS

优点：

- 架构接近标准 Kubernetes
- 不依赖 K3s
- 内容目录在宿主机上，便于维护
- 镜像升级路径清晰

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

部署完成后的关键目录：

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

## 4. 获取项目代码

生产部署统一使用 `/opt/hdu-ride`，但不要直接在 `/opt` 下面 `git clone`。

如果当前还是一台几乎没初始化过的全新 Ubuntu，而系统里还没有 `git`，先执行：

```bash
sudo apt update
sudo apt install -y git
```

推荐做法：

1. 先在当前用户的 `home` 目录下克隆仓库
2. 再整体拷贝到 `/opt/hdu-ride`
3. 后续所有命令都在 `/opt/hdu-ride` 中执行

推荐执行：

```bash
cd ~
git clone https://github.com/MindScrath/HDU-RIDE.git hdu-ride
sudo rm -rf /opt/hdu-ride
sudo mkdir -p /opt
sudo cp -a ~/hdu-ride /opt/hdu-ride
sudo chown -R $USER:$USER /opt/hdu-ride
cd /opt/hdu-ride
```

如果代码已经存在，后续升级直接在 `/opt/hdu-ride` 里执行即可：

```bash
cd /opt/hdu-ride
git pull
```

说明：

- 部署脚本、内容目录、构建上下文都以 `/opt/hdu-ride` 为基准
- 只是“获取代码”这一步不建议直接在 `/opt` 里执行 `git clone`

---

## 5. Ubuntu 基础初始化

以下步骤在全新 Ubuntu 22.04 / 24.04 上执行。

### 5.1 更新系统并安装基础工具

国内云主机建议先把 Ubuntu 软件源切到国内镜像，再执行 `apt update`。以阿里云镜像站为例：

```bash
sudo cp /etc/apt/sources.list /etc/apt/sources.list.bak
sudo sed -i 's|http://archive.ubuntu.com/ubuntu/|https://mirrors.aliyun.com/ubuntu/|g; s|http://security.ubuntu.com/ubuntu/|https://mirrors.aliyun.com/ubuntu/|g' /etc/apt/sources.list
sudo apt update
```

如果你的机器位于腾讯云、华为云，也可以换成对应云厂商的 Ubuntu 镜像源；原则上都比默认国外源稳定。

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

### 5.2 启动并设置服务开机自启

```bash
sudo systemctl enable --now docker
sudo systemctl enable --now nginx
sudo systemctl enable --now containerd
```

说明：

- `docker` 用来构建镜像
- `containerd` 是 Kubernetes 实际使用的运行时
- `nginx` 是公网入口

### 5.3 可选：设置时区

```bash
timedatectl
sudo timedatectl set-timezone Asia/Shanghai
```

---

## 6. 安装 Go

本仓库的生产部署脚本 `scripts/k8s-prod-up.sh` 最终会调用：

```bash
cd backend
go run . ops k8s-prod-up
```

所以服务器上必须安装 Go。

当前仓库的 Go 版本要求以 `backend/go.mod` 为准。当前仓库写的是 `go 1.26`。

以下命令以 `amd64` 服务器为例安装 Go 1.26。

国内环境不要优先使用 `go.dev`，建议直接使用国内镜像站。这里优先使用阿里云镜像：

```bash
cd /tmp
curl -LO https://mirrors.aliyun.com/golang/go1.26.0.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.26.0.linux-amd64.tar.gz
echo 'export PATH=/usr/local/go/bin:$PATH' | sudo tee /etc/profile.d/go.sh
source /etc/profile.d/go.sh
go version
```

如果你的机器不是 `amd64`，请改成对应架构的安装包文件名。

如果阿里云镜像临时不可用，再退回这些备选：

- `https://golang.google.cn/dl/`
- `https://studygolang.com/dl`

---

## 7. 安装 Kubernetes

这里使用的是标准 `kubeadm`，不是 K3s。

### 7.1 安装 kubeadm / kubelet / kubectl

```bash
sudo mkdir -p /etc/apt/keyrings
curl -fsSL https://mirrors.aliyun.com/kubernetes-new/core/stable/v1.29/deb/Release.key | sudo gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg
echo 'deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://mirrors.aliyun.com/kubernetes-new/core/stable/v1.29/deb/ /' | sudo tee /etc/apt/sources.list.d/kubernetes.list
sudo apt update
sudo apt install -y kubelet kubeadm kubectl
sudo apt-mark hold kubelet kubeadm kubectl
sudo systemctl enable --now kubelet
```

作用：

- `kubeadm`：初始化集群
- `kubelet`：节点代理
- `kubectl`：命令行管理工具

### 7.2 打开内核参数并关闭 swap

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

### 7.3 配置 containerd

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

### 7.4 初始化单节点集群

```bash
sudo kubeadm init \
  --pod-network-cidr=10.244.0.0/16 \
  --image-repository registry.aliyuncs.com/google_containers
```

成功后，会打印一段提示，包含：

- `Your Kubernetes control-plane has initialized successfully!`

### 7.5 配置当前用户的 kubectl

```bash
mkdir -p $HOME/.kube
sudo cp /etc/kubernetes/admin.conf $HOME/.kube/config
sudo chown $(id -u):$(id -g) $HOME/.kube/config
kubectl get nodes
```

### 7.6 允许工作负载调度到控制平面节点

单机部署必须执行：

```bash
kubectl taint nodes --all node-role.kubernetes.io/control-plane-
kubectl taint nodes --all node-role.kubernetes.io/master-
```

说明：

- 不同发行版或不同安装方式下，污点键可能是 `control-plane`，也可能是 `master`
- 在单节点 Kubernetes 中，必须把这两类常见污点都移除，避免所有业务 Pod 因 `NoSchedule` 卡死

### 7.7 安装 Flannel 网络插件

```bash
cd /opt/hdu-ride
bash scripts/k8s-install-flannel.sh
```

说明：

- 仓库已内置固定版 [kube-flannel.yml](file:///d:/Go/HDU-RIDE/deploy/k8s/kube-flannel.yml)
- 脚本会优先通过国内代理地址预拉 Flannel 镜像，再应用本地清单
- 这样可以避免在云主机上直接访问 GitHub Raw 或 GitHub Release

等待网络组件启动：

```bash
kubectl get pods -n kube-flannel
kubectl get pods -n kube-system
```

看到 `kube-flannel`、`coredns` 都变成 `Running` 再继续。

---

## 8. 安装动态存储

没有动态存储类时，PostgreSQL、MinIO 和学生工作区都会因为 PVC 无法绑定而卡在 `Pending`。

### 8.1 固定版本说明

仓库内置了固定版本的存储清单：

- [local-path-storage-v0.0.28.yaml](file:///d:/Go/HDU-RIDE/deploy/k8s/local-path-storage-v0.0.28.yaml)

基础镜像版本固定为：

- `rancher/local-path-provisioner:v0.0.28`
- `busybox:1.36`

仓库还内置了单节点专用的存储类定义：

- [storageclasses-single-node.yml](file:///d:/Go/HDU-RIDE/deploy/k8s/storageclasses-single-node.yml)

当前存储策略：

1. 单节点环境只保留一个动态 `StorageClass`：`local-path`
2. `local-path` 被设为默认类
3. `volumeBindingMode` 保持为 `WaitForFirstConsumer`

- 保留 `WaitForFirstConsumer`，避免 `no node was specified`
- 只保留一个动态类，减少单节点环境下的绑定歧义

### 8.2 一键安装 local-path-provisioner

直接执行：

```bash
cd /opt/hdu-ride
bash scripts/k8s-install-local-path.sh
```

脚本会自动完成：

1. 预拉取 `rancher/local-path-provisioner:v0.0.28`
2. 预拉取 `busybox:1.36`
3. 导入 `containerd`
4. 强制移除单节点常见的 `control-plane/master` 调度污点
5. 应用仓库中的固定版本 provisioner YAML
6. 删除并重建 `local-path` 这个动态 `StorageClass`
7. 将 `local-path` 设为默认 `StorageClass`
8. 保持 `volumeBindingMode=WaitForFirstConsumer`
9. 等待 `local-path-provisioner` 就绪

`scripts/k8s-prod-up.sh` 在检测到 `local-path` 缺失时也会自动调用这个脚本，但首次部署仍建议先单独执行一次，便于先排除基础设施问题。

如果你所在环境只能从镜像代理拉取，也不需要手工改 YAML，只要这样执行：

```bash
cd /opt/hdu-ride
LOCAL_PATH_PROVISIONER_PULL_IMAGE=<你的镜像代理地址>/rancher/local-path-provisioner:v0.0.28 \
LOCAL_PATH_HELPER_PULL_IMAGE=<你的镜像代理地址>/busybox:1.36 \
bash scripts/k8s-install-local-path.sh
```

### 8.3 验证

```bash
kubectl get storageclass
```

你应该能看到：

- `local-path`
- 它带有 `(default)` 标记
- `VOLUMEBINDINGMODE` 应为 `WaitForFirstConsumer`

---

## 9. 准备课程内容目录

生产环境直接使用宿主机目录作为课程内容源：

- 宿主机目录：`/opt/hdu-ride/content`
- 容器挂载路径：`/content`

对应配置见 `deploy/k8s/content-pvc-prod.yml` 的 `hostPath`。

### 9.1 静态 PV 的命名空间一致性说明

内容卷是静态精确绑定，手工调整 PV/PVC 时只需要记住两点：

1. `PersistentVolumeClaim` 的命名空间正确，是 `hdu-ride`
2. 静态 PV 与静态 PVC 自身的属性严格一致，例如 `storageClassName`、`capacity`、`accessModes`

不一致时通常会出现：

- `hdu-ride-content` 一直无法绑定
- 后端 Pod 因挂载内容卷失败而长期处于 `Pending`

当前仓库已在 [content-pvc-prod.yml](file:///d:/Go/HDU-RIDE/deploy/k8s/content-pvc-prod.yml) 中配置为：

- PV 使用 `hostPath`
- PVC 通过 `volumeName: hdu-ride-content-pv` 精确指向该 PV
- PV/PVC 的 `storageClassName` 都固定为 `""`

因此：

- 内容卷不会走动态供给
- 内容卷不会再依赖 `WORKSPACE_STORAGE_CLASS`
- 你修改 `.env` 中的 `WORKSPACE_STORAGE_CLASS` 时，不需要同步修改内容卷 YAML

生产脚本 [k8s-prod-up.sh](file:///d:/Go/HDU-RIDE/scripts/k8s-prod-up.sh) 会在部署前检查内容卷属性；若发现旧 PV/PVC 不一致，会先删除再重建。

### 9.2 初始化内容目录

```bash
mkdir -p /opt/hdu-ride/content
test -d /opt/hdu-ride/content/courses && echo "content 目录已就绪"
```

说明：

- `/opt/hdu-ride/content` 就是生产内容目录
- 管理员后续直接维护这个目录即可，不需要额外同步
- 建议把该目录纳入 Git 并定期备份

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
- `README.md` 是作业说明；缺失时系统会生成占位说明文件

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
WORKSPACE_STORAGE_CLASS=local-path
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
  - 这是学生/教师 RStudio 工作区的动态存储类
  - 本文档默认使用 `local-path`
  - 它只影响动态工作区卷，不影响静态内容卷
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

国内环境建议优先配置 Docker 镜像加速器，再执行 `docker pull`。

如果你已经在阿里云容器镜像服务中拿到了专属加速地址，可以先写入：

```bash
sudo mkdir -p /etc/docker
cat <<'EOF' | sudo tee /etc/docker/daemon.json
{
  "registry-mirrors": [
    "https://<your_code>.mirror.aliyuncs.com",
    "https://docker.m.daocloud.io"
  ]
}
EOF
sudo systemctl daemon-reload
sudo systemctl restart docker
```

如果你暂时没有阿里云专属加速地址，至少保留 `https://docker.m.daocloud.io` 作为公开代理入口。

然后再拉取运行期镜像：

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

如果你所在网络里 `docker.io` 仍然不稳定，也可以显式通过代理前缀拉取后再打回原标签，例如：

```bash
sudo docker pull docker.m.daocloud.io/postgres:18-alpine
sudo docker tag docker.m.daocloud.io/postgres:18-alpine postgres:18-alpine
```

其他镜像同理：

- `docker.m.daocloud.io/minio/minio:latest`
- `docker.m.daocloud.io/minio/mc:latest`
- `docker.m.daocloud.io/library/busybox:1.36`
- `docker.m.daocloud.io/rocker/rstudio:4.6.0`

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

- 检查 `kubectl`、`go` 是否存在
- 如果集群里还没有 `local-path` 或 `local-path-provisioner`，自动执行 `scripts/k8s-install-local-path.sh`
- 强制移除单节点常见的 `control-plane/master` 调度污点
- 生产内容卷清单 [content-pvc-prod.yml](file:///d:/Go/HDU-RIDE/deploy/k8s/content-pvc-prod.yml) 中的 PV/PVC 是否都声明了静态绑定用的 `storageClassName: ""`
- 生产内容卷清单中的 PV/PVC 容量是否保持一致
- 如果集群里已经存在静态 PV/PVC，但其 `storageClassName` 或容量与当前期望不一致，先删除旧对象再继续部署
- `WORKSPACE_STORAGE_CLASS` 是否非空；单节点默认应为 `local-path`
- `.env` 中指定的 `WORKSPACE_STORAGE_CLASS` 是否真实存在于当前集群中

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

### 16.1 先理解一个关键事实

课程内容目录虽然是宿主机直挂载，但后端会在启动时把课程内容加载到内存里。

修改 `/opt/hdu-ride/content/...` 后，文件已经在容器可见，但前端不会立刻刷新；你需要再执行一次“课程重载”。

通常不需要重建镜像，也不需要重启整个站点。

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

### 19.8 中途装乱了如何恢复并继续部署

这是单节点 Kubernetes 上最常见的真实场景：

- 存储类装了一半
- `content-pvc-prod.yml` 改过几次
- `WORKSPACE_STORAGE_CLASS` 前后不一致
- `postgres/minio/backend/frontend` 有的已经启动，有的还在报错

这时**不要直接推倒整个 Kubernetes 集群重装**，优先按下面的方法恢复。

#### 情况 A：轻度混乱，只是存储类或内容卷不一致

典型现象：

- `hdu-ride-content` 是 `Pending`
- `kubectl describe pvc` 里出现 `VolumeMismatch`
- `k8s-prod-up.sh` 提示 `storageClassName` 不匹配

恢复步骤：

```bash
cd /opt/hdu-ride
git pull

# 让 .env 与当前单节点默认策略一致
sed -i 's/^WORKSPACE_STORAGE_CLASS=.*/WORKSPACE_STORAGE_CLASS=local-path/' .env

# 重新安装单节点存储类
bash scripts/k8s-install-local-path.sh

# 删除旧的静态内容卷对象并按新清单重建
kubectl delete pvc hdu-ride-content -n hdu-ride --ignore-not-found
kubectl delete pv hdu-ride-content-pv --ignore-not-found
kubectl apply -f deploy/k8s/content-pvc-prod.yml

# 重新部署
bash scripts/k8s-prod-up.sh
```

注意：

- 删除 `hdu-ride-content-pv/pvc` 不会删除宿主机上的 `/opt/hdu-ride/content`
- 因为它是 `hostPath` 静态内容目录，真正的数据仍在磁盘上

#### 情况 B：中度混乱，数据库/对象存储服务也被错误配置影响

典型现象：

- `postgres-0`、`minio-0` 启动失败
- `backend` 一直起不来
- 之前删过一部分对象，当前状态不完整

恢复步骤：

```bash
cd /opt/hdu-ride
git pull

sed -i 's/^WORKSPACE_STORAGE_CLASS=.*/WORKSPACE_STORAGE_CLASS=local-path/' .env

# 修复单节点调度与存储类
bash scripts/k8s-install-local-path.sh

# 删除内容卷对象，重新创建
kubectl delete pvc hdu-ride-content -n hdu-ride --ignore-not-found
kubectl delete pv hdu-ride-content-pv --ignore-not-found

# 删除业务工作负载，让它们用新配置重建
kubectl delete deploy hdu-ride-backend -n hdu-ride --ignore-not-found
kubectl delete deploy hdu-ride-frontend -n hdu-ride --ignore-not-found
kubectl delete sts postgres -n hdu-ride --ignore-not-found
kubectl delete sts minio -n hdu-ride --ignore-not-found

# 重新部署
bash scripts/k8s-prod-up.sh
```

这一步通常已经足够恢复大多数“半成功半失败”的服务器。

#### 情况 C：RStudio 工作区资源污染了环境

典型现象：

- 网站能打开，但学生或教师的 RStudio 老是失败
- 命名空间里残留很多 `rstudio-*` Pod、PVC、Service

恢复步骤：

```bash
kubectl delete pod -n hdu-ride -l app.kubernetes.io/name=hdu-ride-rstudio --ignore-not-found
kubectl delete svc -n hdu-ride -l hdu-ride/workspace-id --ignore-not-found
kubectl delete pvc -n hdu-ride -l app.kubernetes.io/name=hdu-ride-rstudio --ignore-not-found 2>/dev/null || true
```

删除这些动态工作区对象后，用户下一次打开 RStudio 时，后端会重新创建干净的工作区。

#### 恢复完成后的验收命令

```bash
kubectl get storageclass
kubectl get pv
kubectl get pvc -n hdu-ride
kubectl get pods -n hdu-ride
kubectl get svc -n hdu-ride
```

你应当看到：

- `local-path` 是默认 `StorageClass`
- `local-path` 的 `VOLUMEBINDINGMODE` 是 `WaitForFirstConsumer`
- `hdu-ride-content` 为 `Bound`
- `postgres-0`、`minio-0`、`hdu-ride-backend`、`hdu-ride-frontend` 都是 `Running`

#### 什么情况下才需要重装整个 Kubernetes

只有当下面这些更底层的问题出现时，才建议重装整个单节点集群：

- `kube-system` 核心组件异常，例如 `apiserver`、`etcd`、`coredns` 本身不正常
- `kubectl get nodes` 都已经不稳定
- `containerd` 或 `kubelet` 长期异常，且修复无效

如果只是 `hdu-ride` 命名空间里的资源乱了，通常不需要重装整个集群。

---

## 20. 一键诊断脚本

当你在云主机上遇到“网页能打开但 RStudio 不行”“PVC 一直 Pending”“教师打不开学生工作区”“后端 500 / 网关 502”这类问题时，最麻烦的不是修，而是先把现场信息收集完整。

仓库里现在提供了一个专门给生产环境排障用的脚本：

- `scripts/k8s-prod-check.sh`

直接执行：

```bash
cd /opt/hdu-ride
bash scripts/k8s-prod-check.sh
```

执行完成后，它会在仓库根目录生成一份报告，默认位置类似：

```text
/opt/hdu-ride/.diagnostics/k8s-prod-check-20260507-153000.txt
```

### 20.1 它会收集什么

它会尽量自动收集以下信息：

- `.env` 的关键信息摘要
- `kubectl` 当前上下文、节点、命名空间、StorageClass、PV、PVC
- `hdu-ride` 命名空间中的 Pod、Service、Deployment、StatefulSet、事件
- `local-path-storage`、`kube-system` 命名空间中的 Pod 与事件
- `hdu-ride-backend`、`hdu-ride-frontend`、`postgres-0`、`minio-0` 日志
- 所有 `rstudio-*` Pod、`home-*` PVC 的状态、`describe` 与日志
- `kubelet`、`containerd`、`nginx` 的 `systemctl status` 与近期 `journalctl`
- `ctr`、`crictl`、`docker` 镜像信息
- 磁盘、内存、网络、iptables、ufw、swap、sysctl、containerd 关键配置

### 20.2 脱敏说明

脚本默认会对 `.env` 中的常见敏感字段做简单脱敏，例如：

- `POSTGRES_PASSWORD`
- `DATABASE_URL`
- `S3_SECRET_ACCESS_KEY`
- `SESSION_SECRET`
- `ROOT_PASSWORD`
- `ROOT_PASSWORD_HASH`

但诊断报告里仍然可能包含环境路径、主机名、域名、对象名等运维信息。

所以建议你：

1. 生成报告后先自己看一眼
2. 确认没有不想外发的内容
3. 再把整份报告贴给 AI 或发给协作者

### 20.3 可选参数

如果你想自定义输出目录或命名空间，可以这样运行：

```bash
cd /opt/hdu-ride
K8S_NAMESPACE=hdu-ride OUTPUT_DIR=/tmp bash scripts/k8s-prod-check.sh
```

如果你想固定输出文件名：

```bash
cd /opt/hdu-ride
REPORT_PATH=/tmp/hdu-ride-check.txt bash scripts/k8s-prod-check.sh
```

这在你准备多次对比排障结果时会很有用。

---

## 21. 生产环境运维建议

必须修改：

- PostgreSQL 密码
- MinIO 账号密码
- root 管理员密码
- `SESSION_SECRET`

### 21.2 定期备份

至少要备份：

- PostgreSQL 数据
- MinIO 数据
- `/opt/hdu-ride/content`
- `.env`

### 21.3 把内容目录纳入 Git 管理

推荐做法：

- 仓库代码和 `/opt/hdu-ride/content` 都放在 Git 下管理
- 内容更新走 commit
- 管理员更新内容后只需重载课程

### 21.4 修改内容优先，不要轻易改生产数据库

讲义、作业说明、starter、测试数据都应该通过内容目录维护，而不是手工改容器里的文件。

---

## 22. 最终推荐的日常操作清单

### 22.1 首次部署

按顺序执行：

1. 获取项目代码并放到 `/opt/hdu-ride`
2. 安装基础工具
3. 安装 Go
4. 安装 Kubernetes
5. 安装 Flannel
6. 安装 local-path-provisioner
7. 准备 `/opt/hdu-ride/content`
8. 配 `.env`
9. 构建并导入镜像
10. 执行 `bash scripts/k8s-prod-up.sh`
11. 配宿主机 Nginx
12. 配 HTTPS
13. 用 root 登录验收

### 22.2 更新课程内容

1. 修改 `/opt/hdu-ride/content`
2. 管理员后台点击“重新加载”

### 22.3 更新后端前端代码

1. `git pull`
2. `docker build`
3. `ctr import`
4. 重新执行 `bash scripts/k8s-prod-up.sh`

---

## 23. 你应该记住的三句话

1. 内容更新不等于代码更新，改 `content/` 通常不需要重建镜像。
2. 改了内容后要“重新加载课程”，因为后端会把课程读进内存。
3. 裸机 `kubeadm` 单节点部署时，动态存储和 Pod 网络是最容易漏掉的两件事。
