# HDU RIDE 生产环境维护手册

本文档面向已经完成部署、正在维护 Ubuntu 云主机生产环境的管理员。

- 首次部署请看 [INSTRUCTION.md](file:///d:/Go/HDU-RIDE/INSTRUCTION.md)
- Windows 本地开发请看 [DEPLOY.md](file:///d:/Go/HDU-RIDE/DEPLOY.md)

---

## 1. 文档范围

本文档只讲上线后的维护动作，重点包括：

- 查看当前服务状态
- 启动、停止、重启服务
- 更新课程内容
- 升级代码与镜像
- 使用诊断脚本和常用运维工具
- 执行危险恢复操作前的判断与注意事项

不包括：

- 从零安装 Ubuntu、kubeadm、Flannel、local-path、Nginx、Certbot
- Windows 本地开发环境启动

---

## 2. 对象总览

### 2.1 宿主机服务

- `nginx`
  - 宿主机反向代理
  - 对公网暴露 `ride.mindsratch.top`
- `containerd`
  - Kubernetes 运行时
- `kubelet`
  - Kubernetes 节点代理

### 2.2 Kubernetes 命名空间与核心对象

- 命名空间：`hdu-ride`
- 后端 Deployment：`hdu-ride-backend`
- 后端 Service：`hdu-ride-backend`
- 前端 Deployment：`hdu-ride-frontend`
- 前端 Service：`hdu-ride-frontend`
- PostgreSQL StatefulSet：`postgres`
- PostgreSQL Service：`postgres`
- MinIO StatefulSet：`minio`
- MinIO Service：`minio`
- 课程内容 PVC：`hdu-ride-content`
- 课程内容 PV：`hdu-ride-content-pv`

### 2.3 基础设施对象

- Pod 网络：`kube-flannel` 命名空间中的 `kube-flannel-ds-*`
- 动态存储命名空间：`local-path-storage`
- 动态存储 Deployment：`local-path-provisioner`
- 默认 `StorageClass`：`local-path`

### 2.4 按需创建的对象

- 学生或教师打开 RStudio 时，会按需创建：
  - `rstudio-*` Pod
  - `home-*` PVC
  - `rstudio-*` Service
- 这些对象不是常驻服务，不需要日常手工启动。

---

## 3. 日常速查

### 3.1 查看整体状态

```bash
kubectl get pods -n hdu-ride
kubectl get svc -n hdu-ride
kubectl get pvc -n hdu-ride
kubectl get storageclass
kubectl get pv
```

正常时应至少看到：

- `hdu-ride-backend` Pod 为 `Running`
- `hdu-ride-frontend` Pod 为 `Running`
- `postgres-0` 为 `Running`
- `minio-0` 为 `Running`
- `local-path` 为默认 `StorageClass`

### 3.2 查看核心日志

```bash
kubectl logs -n hdu-ride -l app.kubernetes.io/name=hdu-ride-backend --tail=200
kubectl logs -n hdu-ride -l app.kubernetes.io/name=hdu-ride-frontend --tail=200
kubectl logs -n hdu-ride postgres-0 --tail=200
kubectl logs -n hdu-ride minio-0 --tail=200
```

### 3.3 查看宿主机服务状态

```bash
sudo systemctl status nginx
sudo systemctl status containerd
sudo systemctl status kubelet
```

### 3.4 一键诊断

```bash
cd /opt/hdu-ride
bash scripts/k8s-prod-check.sh
```

诊断报告默认输出到：

```text
/opt/hdu-ride/.diagnostics/k8s-prod-check-时间戳.txt
```

---

## 4. 服务启停与重启

### 4.1 宿主机服务

### 查看状态

```bash
sudo systemctl status nginx
sudo systemctl status containerd
sudo systemctl status kubelet
```

### 启动

```bash
sudo systemctl start nginx
sudo systemctl start containerd
sudo systemctl start kubelet
```

### 停止

```bash
sudo systemctl stop nginx
sudo systemctl stop containerd
sudo systemctl stop kubelet
```

说明：

- `stop nginx` 只会让公网入口不可访问，Kubernetes 内部服务仍可能继续运行。
- `stop containerd` 或 `stop kubelet` 会影响整个集群，不要在正常业务时段随意执行。

### 重启

```bash
sudo systemctl restart nginx
sudo systemctl restart containerd
sudo systemctl restart kubelet
```

### 仅重载 Nginx 配置

```bash
sudo nginx -t
sudo systemctl reload nginx
```

适用场景：

- 修改了站点配置
- 续签证书后想无中断加载新配置

### 4.2 Kubernetes 业务组件

### 重启后端和前端

```bash
kubectl rollout restart deployment/hdu-ride-backend -n hdu-ride
kubectl rollout restart deployment/hdu-ride-frontend -n hdu-ride
kubectl rollout status deployment/hdu-ride-backend -n hdu-ride
kubectl rollout status deployment/hdu-ride-frontend -n hdu-ride
```

适用场景：

- 已导入同名新镜像
- 更新了 Secret、配置或代码镜像
- 前端或后端状态异常，需要平滑重建 Pod

### 停止和恢复前端/后端

```bash
kubectl scale deployment/hdu-ride-backend -n hdu-ride --replicas=0
kubectl scale deployment/hdu-ride-frontend -n hdu-ride --replicas=0
```

恢复：

```bash
kubectl scale deployment/hdu-ride-backend -n hdu-ride --replicas=2
kubectl scale deployment/hdu-ride-frontend -n hdu-ride --replicas=1
```

说明：

- 这是业务层“停站”方式，不会删除数据卷。
- 停止后用户将无法访问网站或 API。

### 停止和恢复 PostgreSQL / MinIO

```bash
kubectl scale statefulset/postgres -n hdu-ride --replicas=0
kubectl scale statefulset/minio -n hdu-ride --replicas=0
```

恢复：

```bash
kubectl scale statefulset/postgres -n hdu-ride --replicas=1
kubectl scale statefulset/minio -n hdu-ride --replicas=1
```

说明：

- 这是有状态服务，不要在有用户正在使用时随意停止。
- 停止后，登录、课程数据、作业归档、批阅恢复都会受影响。

### 删除异常 Pod 让其自动重建

```bash
kubectl delete pod -n hdu-ride -l app.kubernetes.io/name=hdu-ride-backend
kubectl delete pod -n hdu-ride -l app.kubernetes.io/name=hdu-ride-frontend
```

适用场景：

- 某个 Pod 状态异常
- 需要让 Deployment 按当前模板重新拉起 Pod

### 4.3 RStudio 工作区对象

### 查看工作区对象

```bash
kubectl get pods -n hdu-ride | grep '^rstudio-' || true
kubectl get pvc -n hdu-ride | grep '^home-' || true
kubectl get svc -n hdu-ride | grep 'rstudio-' || true
```

### 删除异常工作区

```bash
kubectl delete pod -n hdu-ride -l app.kubernetes.io/name=hdu-ride-rstudio --ignore-not-found
kubectl delete svc -n hdu-ride -l hdu-ride/workspace-id --ignore-not-found
```

说明：

- 这会清掉当前在线工作区，用户需要重新点击“打开 RStudio”。
- 如果只删 Pod/Service，不删 `home-*` PVC，原工作区数据仍会保留。

---

## 5. 内容维护

### 5.1 维护目录

生产环境课程内容目录在：

```text
/opt/hdu-ride/content
```

这里保存：

- 讲义章节
- 作业说明
- starter
- 数据文件
- 公共测试

### 5.2 改内容后的正确动作

课程内容目录虽然直接挂载到后端容器，但后端启动时会把内容读入内存。

因此修改 `/opt/hdu-ride/content` 后，需要执行以下二选一动作：

- 在管理后台点击“重新加载”
- 调用后端接口 `POST /api/admin/courses/reload`

不要误以为“改完文件网站会立刻变化”。

### 5.3 什么时候不需要重建镜像

下面这些情况通常不需要重新构建镜像：

- 只改了 `content/` 下的讲义、作业或 starter
- 只补充课程数据文件

这类变更通常只需要：

1. 修改 `/opt/hdu-ride/content`
2. 管理端重新加载课程

### 5.4 什么时候必须重新部署

下面这些情况需要重新构建并重新部署：

- 改了 `backend/`
- 改了 `frontend/`
- 改了 `deploy/docker/*`
- 改了 `deploy/k8s/*`
- 改了 `.env`

---

## 6. 代码升级

### 6.1 推荐顺序

```bash
cd /opt/hdu-ride
git pull
sudo docker build -t hdu-ride-backend:latest -f deploy/docker/backend.Dockerfile .
sudo docker build -t hdu-ride-frontend:latest -f deploy/docker/frontend.Dockerfile .
sudo docker save hdu-ride-backend:latest | sudo ctr -n k8s.io images import -
sudo docker save hdu-ride-frontend:latest | sudo ctr -n k8s.io images import -
bash scripts/k8s-prod-up.sh
```

### 6.2 升级后检查

```bash
kubectl get pods -n hdu-ride
kubectl rollout status deployment/hdu-ride-backend -n hdu-ride
kubectl rollout status deployment/hdu-ride-frontend -n hdu-ride
```

正常时应看到：

- `backend` 与 `frontend` rollout 完成
- `postgres-0` 与 `minio-0` 仍然正常
- 网站主页和登录页可正常访问

### 6.3 如果只想重启，不想重跑整套部署

当你已经导入了同名新镜像，只想让 Pod 重新拉起时，可用：

```bash
kubectl rollout restart deployment/hdu-ride-backend -n hdu-ride
kubectl rollout restart deployment/hdu-ride-frontend -n hdu-ride
```

---

## 7. 诊断与排障工具

### 7.1 优先使用一键诊断脚本

```bash
cd /opt/hdu-ride
bash scripts/k8s-prod-check.sh
```

它会收集：

- 节点、Pod、PVC、PV、StorageClass
- 事件、核心日志、RStudio 相关对象
- `kubelet`、`containerd`、`nginx` 状态和日志
- 宿主机网络、磁盘、内存、swap、iptables
- `containerd` 关键配置

### 7.2 常用排障命令

```bash
kubectl get pods -n hdu-ride -o wide
kubectl describe pod <pod-name> -n hdu-ride
kubectl logs <pod-name> -n hdu-ride --tail=200
kubectl describe pvc <pvc-name> -n hdu-ride
kubectl get events -n hdu-ride --sort-by=.lastTimestamp
```

### 7.3 宿主机侧排障命令

```bash
sudo systemctl status kubelet
sudo systemctl status containerd
sudo systemctl status nginx
sudo journalctl -u kubelet -n 200 --no-pager
sudo journalctl -u containerd -n 200 --no-pager
sudo journalctl -u nginx -n 200 --no-pager
```

### 7.4 存储问题优先检查

```bash
kubectl get storageclass
kubectl get pv
kubectl get pvc -n hdu-ride
kubectl describe pvc -n hdu-ride <pvc-name>
kubectl get pods -n local-path-storage
```

如果工作区创建失败，最常见先看：

- `home-*` PVC 是否 `Pending`
- `local-path-provisioner` 是否 `Running`
- `local-path` 是否仍是默认 `StorageClass`

---

## 8. 危险恢复操作

以下操作都可能影响线上用户、课程内容、提交数据或工作区状态。请务必确认适用场景后再执行。

### 8.1 危险：重置数据库

命令入口：

```bash
cd /opt/hdu-ride/backend
go run . ops db-reset
```

后果：

- 删除并重建核心业务表
- 清空用户、班级、提交、评分、工作区等数据库数据

仅适用于：

- 全新测试环境
- 明确要重置整套业务数据的恢复场景

不要用于：

- 正常生产环境
- 只想修一个用户、一个班级、一个作业的情况

### 8.2 危险：批量删除 RStudio 工作区

```bash
kubectl delete pod -n hdu-ride -l app.kubernetes.io/name=hdu-ride-rstudio --ignore-not-found
kubectl delete svc -n hdu-ride -l hdu-ride/workspace-id --ignore-not-found
kubectl delete pvc -n hdu-ride -l app.kubernetes.io/name=hdu-ride-rstudio --ignore-not-found
```

后果：

- 所有在线工作区被强制清理
- 如果连 `home-*` PVC 一起删，工作区数据也会丢失

仅适用于：

- 工作区资源污染严重
- 批量 Pending / 卡死，需要整体清场重建

### 8.3 危险：删除静态内容卷对象

```bash
kubectl delete pvc hdu-ride-content -n hdu-ride --ignore-not-found
kubectl delete pv hdu-ride-content-pv --ignore-not-found
```

后果：

- 会解除后端对课程内容卷的绑定
- 后续需要重新应用 `content-pvc-prod.yml`

仅适用于：

- 内容卷 `storageClassName`、容量或绑定状态错误
- 明确要按当前清单重建静态内容卷对象

说明：

- 这两个对象删掉后，并不等于宿主机目录 `/opt/hdu-ride/content` 被删掉
- 但后端会在对象重建前失去这块卷

### 8.4 危险：停止 PostgreSQL / MinIO

```bash
kubectl scale statefulset/postgres -n hdu-ride --replicas=0
kubectl scale statefulset/minio -n hdu-ride --replicas=0
```

后果：

- PostgreSQL 停止后，站点登录和业务读写都会失败
- MinIO 停止后，提交归档、教师批阅恢复都会失败

仅适用于：

- 明确的维护窗口
- 需要对数据层单独做维护或排障

---

## 9. 推荐维护习惯

- 先跑 `bash scripts/k8s-prod-check.sh`，再做进一步排障
- 改内容和改代码分开处理，不要混用恢复步骤
- 先看 `kubectl get pods/pvc/events`，再决定是否删对象
- 改 Nginx 配置后先 `sudo nginx -t`，通过后再 `reload`
- 危险操作前先记录当前状态，至少保存 `kubectl get pods,pvc,pv -A`

---

## 10. 文档分工

- [README.md](file:///d:/Go/HDU-RIDE/README.md)
  - 项目概览
  - 本地开发快速入口
- [DEPLOY.md](file:///d:/Go/HDU-RIDE/DEPLOY.md)
  - Windows 本地开发细节
- [INSTRUCTION.md](file:///d:/Go/HDU-RIDE/INSTRUCTION.md)
  - Ubuntu 云主机首次部署
- `MAINTAIN.md`
  - 生产环境启停、检查、升级、诊断、恢复
