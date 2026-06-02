# HDU RIDE（React 版）—— 日常更新维护手册

本文档只讲一件事：已经部署运行后，改了代码或内容，怎样正确上线。

文档分工如下：

- 首次部署看 [REACT-ON.md](REACT-ON.md)
- 日常更新看本文档
- 后端 / K8s / 数据库深度运维看 [INSTRUCTION.md](INSTRUCTION.md)

---

## 1. 先记住三件事

1. 改了 `backend/` 或 `frontend-react/`，必须**重新构建镜像 → 导入 containerd → 重启 Deployment**。三步缺一不可。
2. Kubernetes 里真正需要重启的是 `Deployment` 对应的 Pod，不是 `Service`。
3. 生产环境课程内容来源是宿主机目录 `/opt/hdu-ride/content`，不是容器内手改的文件。

---

## 2. 为什么改了镜像 Pod 还是没变

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

---

## 3. 前后端都改了（标准全量更新）

适用场景：

- 修改了 Go 后端代码
- 修改了 React 前端代码
- 修改了 `deploy/docker/backend.Dockerfile`
- 修改了 `deploy/docker/frontend.Dockerfile`
- 修改了 `frontend-react/next.config.ts`

服务器上统一在 `/opt/hdu-ride` 操作。

```bash
# 1. 拉取最新代码
cd /opt/hdu-ride
git pull

# 2. 生成 bun.lock（如果前端依赖有变化）
cd frontend-react && bun install && cd ..

# 3. 重新构建两个镜像
sudo docker build -t hdu-ride-backend:latest -f deploy/docker/backend.Dockerfile .

sudo docker build -t hdu-ride-frontend:latest \
  -f deploy/docker/frontend.Dockerfile \
  --build-arg NEXT_PUBLIC_GO_API_URL=http://hdu-ride-backend:8080 \
  .

# 4. 导入 containerd
cd /tmp
sudo docker save hdu-ride-backend:latest -o hdu-ride-backend.tar
sudo docker save hdu-ride-frontend:latest -o hdu-ride-frontend.tar
sudo ctr -n k8s.io images import hdu-ride-backend.tar
sudo ctr -n k8s.io images import hdu-ride-frontend.tar

# 5. 重新应用 K8s 资源
cd /opt/hdu-ride
bash scripts/k8s-prod-up.sh

# 6. 重启 Deployment（关键步骤！）
kubectl rollout restart deployment/hdu-ride-backend -n hdu-ride
kubectl rollout restart deployment/hdu-ride-frontend -n hdu-ride

# 7. 等待新 Pod 就绪
kubectl rollout status deployment/hdu-ride-backend -n hdu-ride --timeout=180s
kubectl rollout status deployment/hdu-ride-frontend -n hdu-ride --timeout=180s

# 8. 最终检查
kubectl get pods -n hdu-ride
```

---

## 4. 只改了后端

适用场景：

- 修改了 `backend/app/*.go`
- 修改了 `deploy/docker/backend.Dockerfile`

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

---

## 5. 只改了前端

适用场景：

- 修改了 `frontend-react/app/*.tsx`
- 修改了 `frontend-react/components/*.tsx`
- 修改了 `frontend-react/lib/*.ts`
- 修改了 `frontend-react/package.json`
- 修改了 `deploy/docker/frontend.Dockerfile`

```bash
cd /opt/hdu-ride
git pull

# 如果改了 package.json 或新增了依赖，重新生成 bun.lock
cd frontend-react && bun install && cd ..

sudo docker build -t hdu-ride-frontend:latest \
  -f deploy/docker/frontend.Dockerfile \
  --build-arg NEXT_PUBLIC_GO_API_URL=http://hdu-ride-backend:8080 \
  .

cd /tmp
sudo docker save hdu-ride-frontend:latest -o hdu-ride-frontend.tar
sudo ctr -n k8s.io images import hdu-ride-frontend.tar

cd /opt/hdu-ride
bash scripts/k8s-prod-up.sh

kubectl rollout restart deployment/hdu-ride-frontend -n hdu-ride
kubectl rollout status deployment/hdu-ride-frontend -n hdu-ride --timeout=180s
```

> `--build-arg NEXT_PUBLIC_GO_API_URL=http://hdu-ride-backend:8080` 是**生产环境必须的**。它告诉 Next.js 在 K8s 集群内部用 Service 名 `hdu-ride-backend` 来代理 `/api` 和 `/ide` 请求。如果不传，默认值是 `http://localhost:8080`，在 K8s 环境下会导致所有 API 请求失败。

---

## 6. 验证新 Pod 是否真的换新了

```bash
kubectl get pods -n hdu-ride -o wide
```

重点检查：

- 新 Pod 的 `AGE` 是不是刚刚（几秒或几分钟）
- 老 Pod 是否已经终止（`Terminating` 或已消失）
- `READY` 列是否都是 `1/1`

再看 Deployment 状态：

```bash
kubectl get deploy -n hdu-ride
kubectl describe deploy hdu-ride-backend -n hdu-ride | tail -20
kubectl describe deploy hdu-ride-frontend -n hdu-ride | tail -20
```

如果上线后有异常，看日志：

```bash
kubectl logs -l app.kubernetes.io/name=hdu-ride-backend -n hdu-ride --tail=200
kubectl logs -l app.kubernetes.io/name=hdu-ride-frontend -n hdu-ride --tail=200
```

---

## 7. 只改了课程内容

课程内容更新**不需要重建镜像**。后端在启动时把课程内容加载到内存，修改磁盘上的文件后必须触发重载。

### 7.1 数据流

```
宿主机 /opt/hdu-ride/content/
    │
    │  hostPath PV
    ▼
Kubernetes PVC hdu-ride-content
    │
    │  volumeMount /content
    ▼
后端容器 /content/
    │
    │  app.LoadCourses() 启动时加载到内存
    ▼
后端内存 (content.CourseStore) ◄── 前端 API 从这里读取
```

### 7.2 快速更新（推荐：用脚本）

```bash
cd /opt/hdu-ride
bash scripts/update-content.sh
```

这个脚本会：
1. 执行 `git pull` 拉取最新 content
2. 显示 content/ 的变更摘要
3. 自动触发课程重载（优先尝试 API，失败时回退到重启 backend）

如果不想 git pull（已经手动更新了文件）：

```bash
bash scripts/update-content.sh --no-pull
```

如果 API 重载不可用，直接用重启方式：

```bash
bash scripts/update-content.sh --restart
```

### 7.3 手动更新（分步操作）

**第一步：更新文件**

```bash
cd /opt/hdu-ride
git pull
```

确认文件已更新：

```bash
ls /opt/hdu-ride/content/courses/
cat /opt/hdu-ride/content/courses/intro-r/course.yml | head -10
```

**第二步：触发重载（二选一）**

方式 A — 通过管理后台（推荐，无中断）：
1. 用管理员账号登录
2. 进入课程管理页
3. 点击"重新加载"

方式 B — 重启 backend（有几秒中断）：

```bash
kubectl rollout restart deployment/hdu-ride-backend -n hdu-ride
kubectl rollout status deployment/hdu-ride-backend -n hdu-ride --timeout=180s
```

**第三步：验证**

刷新浏览器页面，检查：
- 讲义列表中是否出现新增的章节
- 作业列表中是否出现新增的作业
- 点击章节和作业，内容是否正确渲染

### 7.4 课程加载的容错行为

- 如果某个课程的 `course.yml` 有 YAML 语法错误，**整个课程**会加载失败（后端启动时打印 WARNING 日志，但继续运行其他课程）
- 如果某个作业目录缺失或 `assignment.yml` 有误，**该作业被跳过**，其他作业和章节正常加载

### 7.5 更新后全部讲义消失

最可能的原因：

1. **course.yml 有 YAML 语法错误** — 检查缩进（必须用空格，不能用 Tab）
2. **重载成功但报了 warning** — 打开浏览器 F12 Network 面板，查看 reload API 返回值

排查：

```bash
kubectl logs -l app.kubernetes.io/name=hdu-ride-backend -n hdu-ride --tail=50 | grep -i "warn\|error\|fail"
```

---

## 8. 出问题时怎么强制换 Pod

如果你已经确认新镜像导入成功，但 Pod 还是异常，按下面顺序处理。

### 8.1 先正常 rollout restart

```bash
kubectl rollout restart deployment/hdu-ride-backend -n hdu-ride
kubectl rollout restart deployment/hdu-ride-frontend -n hdu-ride
kubectl rollout status deployment/hdu-ride-backend -n hdu-ride --timeout=180s
kubectl rollout status deployment/hdu-ride-frontend -n hdu-ride --timeout=180s
```

### 8.2 如果 rollout 卡住，先看事件

```bash
kubectl get pods -n hdu-ride
kubectl get events -n hdu-ride --sort-by=.lastTimestamp | tail -30
```

### 8.3 必要时删除旧 Pod，让 Deployment 立即补新 Pod

```bash
kubectl delete pod -l app.kubernetes.io/name=hdu-ride-backend -n hdu-ride
kubectl delete pod -l app.kubernetes.io/name=hdu-ride-frontend -n hdu-ride
```

删除后 Deployment 会自动创建新 Pod。

注意：

- 删除的是 Pod，不是 Deployment
- 不要删 `Service`
- 不要删 `pv/pvc`，除非你明确在修存储问题

---

## 9. 上线后检查清单

代码上线后至少检查：

```bash
kubectl get pods -n hdu-ride
kubectl get svc -n hdu-ride
```

并确认：

- `postgres-0` 是 `Running`
- `minio-0` 是 `Running`
- `hdu-ride-backend` 的 Pod 是**新创建的**并处于 `Running`
- `hdu-ride-frontend` 的 Pod 是**新创建的**并处于 `Running`
- 网页访问正常
- 管理员登录正常

如果更新了内容，还要额外确认：

- 目标课程页面已经显示新内容
- 作业说明、讲义等变化已经可见

---

## 10. 总结

上线时最容易漏掉的点只有两个：

1. 改了前后端代码后，没有把镜像导入 `containerd`
2. 导入新镜像后，没有执行 `kubectl rollout restart deployment/...`

而 `content` 的原则也只有三句话：

- **生产以 `/opt/hdu-ride/content` 为准**，不要去容器里手改文件
- **改完必须重载**，否则后端内存里还是旧数据 — 执行 `bash scripts/update-content.sh`，或通过管理后台点击"重新加载"
- **重载报错时看 warning**，它会告诉你哪个作业或文件出了问题
