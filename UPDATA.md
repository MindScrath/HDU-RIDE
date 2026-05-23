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

**如果你只改了 content/，不要用下面这组命令。请直接跳到第 8 节，或执行：**

```bash
bash scripts/update-content.sh
```

**以下仅适用于修改了前后端代码的情况：**

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

## 7. content 加载机制（理解这个才能排错）

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

后端在**启动时**调用 `app.LoadCourses()` 扫描 `/content/courses/*/course.yml`，把所有章节课时和作业信息加载到内存。之后前端请求讲义列表、作业列表、渲染内容，全部走内存，不会再读磁盘。

### 7.2 所以”改了文件但页面没变”是正常的

因为后端读的是**内存里的旧数据**。你改了磁盘上的 Markdown，后端不知道。必须触发**重载（reload）**，后端才会重新扫描目录、覆盖内存数据。

### 7.3 课程加载的容错行为

如果一个课程的 `course.yml` 有 YAML 语法错误，**整个课程**会加载失败（后端启动时会打印 WARNING 日志，但仍然会继续运行其他课程）。

如果一个课程中某个作业目录缺失或 `assignment.yml` 有误，**该作业会被跳过**，其他作业和章节仍然正常加载。重载 API 会返回具体的错误信息，告知哪个作业出了问题。

### 7.4 最常见的加载失败原因

| 现象 | 可能原因 |
|------|----------|
| 所有讲义都看不到 | course.yml 不在正确位置，或 YAML 缩进/语法错误 |
| 某个章节点击后空白 | 对应的 `.md` 文件不存在或路径在 course.yml 中写错 |
| 某个作业不显示 | `assignments/<id>/assignment.yml` 缺失、日期格式错误、或目录名与 course.yml 中 `id` 不一致 |
| 全部课程消失 | `/content/courses/` 目录不存在，或没有任何 course.yml |

---

## 8. content 更新的正确步骤

### 8.1 快速更新（推荐：使用脚本）

服务器上一键完成：

```bash
cd /opt/hdu-ride
bash scripts/update-content.sh
```

这个脚本会：
1. 执行 `git pull` 拉取最新 content
2. 显示 content/ 的变更摘要
3. 自动触发课程重载（优先尝试 API，失败时回退到重启 backend）

如果不想 git pull（比如已经手动更新了文件）：

```bash
bash scripts/update-content.sh --no-pull
```

如果 API 重载不可用，直接用重启方式：

```bash
bash scripts/update-content.sh --restart
```

### 8.2 手动更新（分步操作）

如果脚本不可用，手动执行以下步骤：

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
3. 点击”重新加载”

方式 B — 重启 backend（备用，有几秒中断）：
```bash
kubectl rollout restart deployment/hdu-ride-backend -n hdu-ride
kubectl rollout status deployment/hdu-ride-backend -n hdu-ride --timeout=180s
```

**第三步：验证**

刷新浏览器页面，检查：
- 讲义列表中是否出现新增的章节
- 作业列表中是否出现新增的作业
- 点击章节和作业，内容是否正确渲染

---

## 9. 内容更新常见问题排查

### 9.1 更新后全部讲义消失

最可能的两个原因：

1. **course.yml 有 YAML 语法错误** — 检查缩进是否用空格（不能用 Tab）、是否有中文冒号等特殊字符导致解析失败。
2. **重载成功但报了 warning** — 打开浏览器 F12 Network 面板，查看 `/api/admin/courses/reload` 的返回值，如果有 `”warning”` 字段，里面会列出具体的加载问题。

排查步骤：

```bash
# 在服务器上检查 course.yml 是否能被 Go YAML 解析
cd /opt/hdu-ride
grep -n “^[[:space:]]” content/courses/intro-r/course.yml | head -20

# 查看后端日志
kubectl logs -l app.kubernetes.io/name=hdu-ride-backend -n hdu-ride --tail=50 | grep -i “warn\|error\|fail”
```

### 9.2 部分作业不显示

在 course.yml 中列出的每个作业，必须对应一个目录：

```text
courses/<课程id>/assignments/<作业id>/
    ├── assignment.yml    ← 必须有
    └── README.md         ← 必须有
```

常见错误：
- course.yml 里写 `id: hw02`，但目录叫 `homework02` — 名字必须完全一致
- `assignment.yml` 里的日期格式不是 `2026-05-25 09:00` — 必须精确到分钟
- 目录权限问题导致后端读不了文件

### 9.3 讲义内容渲染异常

如果页面显示了 YAML front matter 原始内容（比如看到 `title: xxx` 而不是正常标题），说明 front matter 解析出了问题。这可能是因为 Markdown 文件的换行符混合（Windows `\r\n` 和 Linux `\n` 混用）。确保文件中的 `---` 分隔符前后换行一致。

---

## 10. 各环境 content 同步方式总结

| 环境 | content 在哪里 | 更新方式 |
|------|---------------|----------|
| **生产 (Ubuntu)** | 宿主机 `/opt/hdu-ride/content` → hostPath PV → Pod `/content` | `git pull` + 重载（或用 `scripts/update-content.sh`） |
| **本地开发 (kind)** | Windows `content/` 目录 | `scripts/rideops.ps1 sync-content` → 复制到 PVC → 重载 |
| **本地开发 (Podman 直接跑)** | 本地文件系统 | 后端直接读本地目录，修改后重载即可 |

生产环境**不要**使用 `scripts/k8s-sync-content.sh`，因为生产用的是 `hostPath` PV——宿主机目录直接挂载进 Pod，不需要额外的 tar + kubectl exec 同步步骤。

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

而 `content` 的原则也只有三句话：

- **生产以 `/opt/hdu-ride/content` 为准**，不要去容器里手改文件
- **改完必须重载**，否则后端内存里还是旧数据——执行 `bash scripts/update-content.sh`，或通过管理后台点击"重新加载"
- **重载报错时看 warning**，它会告诉你哪个作业或文件出了问题

