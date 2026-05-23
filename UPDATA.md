# HDU RIDE 上线更新手册

本文档只讲一件事：当你已经有一台正在运行的服务器后，改了前端、后端或 `content/` 之后，怎样把新版本正确上线。

## 1. 先记住三件事

1. 改了 `backend/` 或 `frontend/`，必须重新构建镜像，并重新让 Pod 启动。
2. Kubernetes 里真正需要重启的是 `Deployment` 对应的 Pod，不是 `Service`。
3. 生产环境的课程内容来源是宿主机目录 `/opt/hdu-ride/content`，不是容器内手改的文件。

## 2. 为什么你改了镜像 Pod 还是没变

当前仓库里：

- 后端镜像是 `hdu-ride-backend:latest`
- 前端镜像是 `hdu-ride-frontend:latest`
- `deploy/k8s/backend.yml` 和 `deploy/k8s/frontend.yml` 都是 `imagePullPolicy: IfNotPresent`

这意味着：

- 你就算重新 `docker build` 了同名 `latest` 镜像，正在运行的旧 Pod 也不会自动变成新代码
- `kubectl apply -f ...` 只是在集群里更新清单，不等于强制重建现有 Pod
- `Service` 只是转发流量，不负责重建 Pod，所以重启 `Service` 没用

因此，代码上线时一定要做两件事：

1. 让新镜像进入 Kubernetes 实际使用的 `containerd`
2. 让 `Deployment` 重新 rollout，生成新 Pod

## 3. 代码上线标准流程

以下流程适用于：

- 修改了 Go 后端代码
- 修改了 Vue 前端代码
- 修改了 `deploy/docker/backend.Dockerfile`
- 修改了 `deploy/docker/frontend.Dockerfile`
- 修改了前端容器内 Nginx 配置

服务器上统一在 `/opt/hdu-ride` 操作。

### 3.1 拉取最新代码

```bash
cd /opt/hdu-ride
git pull
```

如果你不是直接 `git pull`，而是通过上传文件覆盖代码，也要保证最终服务器上的目录内容已经是最新的。

### 3.2 重新构建前后端镜像

```bash
cd /opt/hdu-ride
sudo docker build -t hdu-ride-backend:latest -f deploy/docker/backend.Dockerfile .
sudo docker build -t hdu-ride-frontend:latest -f deploy/docker/frontend.Dockerfile .
```

### 3.3 把 Docker 镜像导入 containerd

这一步不能省。

原因是：

- `docker images` 里的镜像，Kubernetes 不会直接拿来跑
- 当前服务器上的 Kubernetes 运行时是 `containerd`
- 所以要把镜像从 Docker 再导入 `containerd`

执行：

```bash
cd /tmp
sudo docker save hdu-ride-backend:latest -o hdu-ride-backend.tar
sudo docker save hdu-ride-frontend:latest -o hdu-ride-frontend.tar

sudo ctr -n k8s.io images import hdu-ride-backend.tar
sudo ctr -n k8s.io images import hdu-ride-frontend.tar
```

可以验证：

```bash
sudo ctr -n k8s.io images list | grep hdu-ride
```

### 3.4 重新应用 Kubernetes 资源

```bash
cd /opt/hdu-ride
bash scripts/k8s-prod-up.sh
```

这一步会做这些事：

- 校验 `.env`
- 校验并应用命名空间、Secret、内容卷、Postgres、MinIO
- 应用 `backend.yml` 和 `frontend.yml`
- 执行 `kubectl set image`

但是请注意：

- 如果镜像标签还是同一个 `latest`
- 或者当前 Deployment 中记录的镜像名没有变化

那么仅执行这一步，有可能不会把已经在运行的旧 Pod 换掉。

所以后面还要显式重启 Deployment。

### 3.5 关键步骤：重启 Deployment，而不是重启 Service

执行：

```bash
kubectl rollout restart deployment/hdu-ride-backend -n hdu-ride
kubectl rollout restart deployment/hdu-ride-frontend -n hdu-ride
```

然后等待新 Pod 完成替换：

```bash
kubectl rollout status deployment/hdu-ride-backend -n hdu-ride --timeout=180s
kubectl rollout status deployment/hdu-ride-frontend -n hdu-ride --timeout=180s
```

这是代码上线里最关键的一步。

如果你不做这一步，常见结果就是：

- 镜像已经重新 build 了
- 镜像也已经导入 `containerd` 了
- 但是线上跑的还是旧 Pod
- 页面看起来像“没更新”

### 3.6 验证 Pod 是否真的换新了

先看 Pod：

```bash
kubectl get pods -n hdu-ride -o wide
```

再看 Deployment 状态：

```bash
kubectl get deploy -n hdu-ride
kubectl describe deploy hdu-ride-backend -n hdu-ride
kubectl describe deploy hdu-ride-frontend -n hdu-ride
```

重点检查：

- 新 Pod 的创建时间是不是刚刚
- 老 Pod 是否已经终止
- `AVAILABLE` 是否恢复正常

如果上线后有异常，再看日志：

```bash
kubectl logs -l app.kubernetes.io/name=hdu-ride-backend -n hdu-ride --tail=200
kubectl logs -l app.kubernetes.io/name=hdu-ride-frontend -n hdu-ride --tail=200
```

## 4. 最短上线命令

如果你只是改了前后端代码，通常直接执行下面这组命令即可：

```bash
cd /opt/hdu-ride
git pull

sudo docker build -t hdu-ride-backend:latest -f deploy/docker/backend.Dockerfile .
sudo docker build -t hdu-ride-frontend:latest -f deploy/docker/frontend.Dockerfile .

cd /tmp
sudo docker save hdu-ride-backend:latest -o hdu-ride-backend.tar
sudo docker save hdu-ride-frontend:latest -o hdu-ride-frontend.tar
sudo ctr -n k8s.io images import hdu-ride-backend.tar
sudo ctr -n k8s.io images import hdu-ride-frontend.tar

cd /opt/hdu-ride
bash scripts/k8s-prod-up.sh

kubectl rollout restart deployment/hdu-ride-backend -n hdu-ride
kubectl rollout restart deployment/hdu-ride-frontend -n hdu-ride

kubectl rollout status deployment/hdu-ride-backend -n hdu-ride --timeout=180s
kubectl rollout status deployment/hdu-ride-frontend -n hdu-ride --timeout=180s
kubectl get pods -n hdu-ride
```

## 5. 如果只改了后端

只需要重建后端镜像，并只重启后端 Deployment：

```bash
cd /opt/hdu-ride
git pull

sudo docker build -t hdu-ride-backend:latest -f deploy/docker/backend.Dockerfile .

cd /tmp
sudo docker save hdu-ride-backend:latest -o hdu-ride-backend.tar
sudo ctr -n k8s.io images import hdu-ride-backend.tar

cd /opt/hdu-ride
bash scripts/k8s-prod-up.sh

kubectl rollout restart deployment/hdu-ride-backend -n hdu-ride
kubectl rollout status deployment/hdu-ride-backend -n hdu-ride --timeout=180s
```

## 6. 如果只改了前端

只需要重建前端镜像，并只重启前端 Deployment：

```bash
cd /opt/hdu-ride
git pull

sudo docker build -t hdu-ride-frontend:latest -f deploy/docker/frontend.Dockerfile .

cd /tmp
sudo docker save hdu-ride-frontend:latest -o hdu-ride-frontend.tar
sudo ctr -n k8s.io images import hdu-ride-frontend.tar

cd /opt/hdu-ride
bash scripts/k8s-prod-up.sh

kubectl rollout restart deployment/hdu-ride-frontend -n hdu-ride
kubectl rollout status deployment/hdu-ride-frontend -n hdu-ride --timeout=180s
```

## 7. content 目录到底怎么用

生产环境里，课程内容卷来自：

- Kubernetes PV: `deploy/k8s/content-pvc-prod.yml`
- 宿主机实际目录：`/opt/hdu-ride/content`
- 后端容器内挂载目录：`/content`

也就是说，线上真正应该维护的是服务器上的：

```text
/opt/hdu-ride/content
```

不要做这些事：

- 不要手工进后端 Pod 里改 `/content`
- 不要把课程内容只改在你本地电脑上却不上传到服务器
- 不要误以为重建前后端镜像会自动更新 `content`

因为：

- 生产环境 `content` 是独立的宿主机目录
- 它和前后端镜像不是一回事
- 改镜像不会覆盖它，删 Pod 也不会自动清空它

## 8. content 上线或更新的正确步骤

适用于：

- 修改讲义
- 修改章节 Markdown
- 修改课程配置
- 修改作业说明
- 修改 starter
- 修改公开数据
- 修改测试文件

### 8.1 把新内容放到服务器的 `/opt/hdu-ride/content`

你可以用任一方式更新：

- `git pull`
- `scp` / `sftp`
- VS Code Remote SSH
- 直接在服务器编辑

但最终目标都一样：让服务器上的这个目录变成最新内容。

例如：

```bash
cd /opt/hdu-ride/content
find .
```

你应该能在这里看到最新课程目录，例如 `courses/...`。

### 8.2 内容更新后，不需要重建前后端镜像

如果你改的只是 `content/`，通常不需要：

- `docker build`
- `ctr images import`
- `kubectl rollout restart deployment/hdu-ride-frontend`

因为页面代码和后端程序本身没有变。

### 8.3 内容更新后，要让后端重新加载课程

虽然生产内容目录是挂载的，但后端会把课程内容加载到内存里。

所以你改完 `/opt/hdu-ride/content` 后，还要执行下面二选一。

方式 A，推荐：

- 用管理员登录系统
- 进入课程管理页
- 点击“重新加载”

它会调用：

```text
POST /api/admin/courses/reload
```

方式 B，备用：

```bash
kubectl rollout restart deployment/hdu-ride-backend -n hdu-ride
kubectl rollout status deployment/hdu-ride-backend -n hdu-ride --timeout=180s
```

如果后台“重新加载”可用，优先用方式 A，因为它只重载课程内容，不会多做无关重启。

## 9. content 更新时的建议流程

### 9.1 只更新内容

```bash
cd /opt/hdu-ride
git pull

# 确认服务器上的 content 已更新
ls /opt/hdu-ride/content
```

然后：

1. 登录管理员后台
2. 执行课程重新加载
3. 打开网页确认内容已变化

### 9.2 既改代码，也改内容

按下面顺序做最稳妥：

1. 更新 `/opt/hdu-ride` 代码
2. 重建并导入前后端镜像
3. 执行 `bash scripts/k8s-prod-up.sh`
4. 执行 `kubectl rollout restart deployment/...`
5. 确认新 Pod 正常
6. 再执行一次课程重载

这样可以避免你看到“代码已经是新的，但课程还是旧的”这种混合状态。

## 10. 不建议在生产上使用 `scripts/k8s-sync-content.sh`

当前生产环境的内容卷已经是 `hostPath` 直接挂载 `/opt/hdu-ride/content`。

因此在生产机上，更新内容最直接的方法就是：

1. 直接更新 `/opt/hdu-ride/content`
2. 然后执行课程重载

不需要额外再跑一次 `scripts/k8s-sync-content.sh`。

## 11. 出问题时怎么强制换 Pod

如果你已经确认新镜像导入成功，但 Pod 还是异常，可以按下面顺序处理。

### 11.1 先正常 rollout restart

```bash
kubectl rollout restart deployment/hdu-ride-backend -n hdu-ride
kubectl rollout restart deployment/hdu-ride-frontend -n hdu-ride
kubectl rollout status deployment/hdu-ride-backend -n hdu-ride --timeout=180s
kubectl rollout status deployment/hdu-ride-frontend -n hdu-ride --timeout=180s
```

### 11.2 如果 rollout 卡住，先看事件

```bash
kubectl get pods -n hdu-ride
kubectl get events -n hdu-ride --sort-by=.lastTimestamp
```

### 11.3 必要时删除旧 Pod，让 Deployment 立即补新 Pod

```bash
kubectl delete pod -l app.kubernetes.io/name=hdu-ride-backend -n hdu-ride
kubectl delete pod -l app.kubernetes.io/name=hdu-ride-frontend -n hdu-ride
```

删除后 Deployment 会自动补新的 Pod。

注意：

- 删除的是 Pod，不是 Deployment
- 不要删 `Service`
- 不要删 `pv/pvc`，除非你明确在修存储问题

## 12. 上线后检查清单

代码上线后至少检查：

```bash
kubectl get pods -n hdu-ride
kubectl get svc -n hdu-ride
kubectl get pvc -n hdu-ride
```

并确认：

- `postgres-0` 是 `Running`
- `minio-0` 是 `Running`
- `hdu-ride-backend` 的 Pod 是新创建的并处于 `Running`
- `hdu-ride-frontend` 的 Pod 是新创建的并处于 `Running`
- 网页访问正常
- 管理员登录正常

如果更新了内容，还要额外确认：

- 目标课程页面已经显示新内容
- 作业说明、讲义、starter 等变化已经可见

## 13. 最终结论

上线时最容易漏掉的点只有两个：

1. 改了前后端代码后，没有把镜像导入 `containerd`
2. 导入新镜像后，没有执行 `kubectl rollout restart deployment/...`

而 `content` 的原则也只有一句话：

- 生产以 `/opt/hdu-ride/content` 为准，改完后执行课程重载，不要去容器里手改文件
