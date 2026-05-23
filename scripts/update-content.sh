#!/usr/bin/env sh
# ============================================================
# HDU RIDE 内容更新脚本 (Content Update Script)
# ============================================================
# 用途：更新服务器上的课程内容（讲义、作业）并触发后端重载
# 适用：只改了 content/ 目录，不需要重建前后端镜像
#
# 用法：
#   bash scripts/update-content.sh           # git pull + reload
#   bash scripts/update-content.sh --no-pull  # 只 reload，不 pull
#   bash scripts/update-content.sh --restart  # 通过重启 backend 来 reload
# ============================================================
set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
NAMESPACE="${K8S_NAMESPACE:-hdu-ride}"
DO_PULL=true
USE_RESTART=false

for arg in "$@"; do
  case "$arg" in
    --no-pull) DO_PULL=false ;;
    --restart) USE_RESTART=true ;;
  esac
done

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "错误：缺少命令 $1，请先安装"
    exit 1
  fi
}

# ---- Step 1: 更新 content 目录 ----
if [ "$DO_PULL" = true ]; then
  echo "==> Step 1/3: 拉取最新代码..."
  cd "$ROOT"
  git pull --ff-only
  echo ""
  echo "content/ 变更摘要："
  git diff --stat HEAD~1 -- content/ 2>/dev/null || echo "  (无法比较，可能是首次部署)"
  echo ""
else
  echo "==> Step 1/3: 跳过 git pull (使用了 --no-pull)"
fi

# ---- Step 2: 确认 content 目录存在 ----
echo "==> Step 2/3: 检查 content 目录..."
if [ ! -d "$ROOT/content/courses" ]; then
  echo "错误：$ROOT/content/courses 不存在，请检查 content 目录"
  exit 1
fi
echo "  课程目录:"
find "$ROOT/content/courses" -name "course.yml" -exec dirname {} \; | while read dir; do
  echo "    - $(basename "$(dirname "$dir")")/$(basename "$dir")"
done

# ---- Step 3: 触发课程重载 ----
echo ""
echo "==> Step 3/3: 触发课程重载..."

if [ "$USE_RESTART" = true ]; then
  # ---- 方式 A: 重启 backend Pod ----
  echo "  方式：重启 backend Deployment"
  require_command kubectl

  kubectl rollout restart deployment/hdu-ride-backend -n "$NAMESPACE"
  echo "  等待 rollout 完成..."
  kubectl rollout status deployment/hdu-ride-backend -n "$NAMESPACE" --timeout=180s

  echo ""
  echo "  Backend Pod 状态:"
  kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/name=hdu-ride-backend
else
  # ---- 方式 B: 通过 API 触发重载 (推荐) ----
  require_command kubectl

  BACKEND_POD=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/name=hdu-ride-backend -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
  if [ -z "$BACKEND_POD" ]; then
    echo "  未找到 backend Pod，回退到重启方式..."
    kubectl rollout restart deployment/hdu-ride-backend -n "$NAMESPACE"
    kubectl rollout status deployment/hdu-ride-backend -n "$NAMESPACE" --timeout=180s
  else
    echo "  方式：调用后端 API (Pod: $BACKEND_POD)"

    # 通过 kubectl exec 在 Pod 内调用 reload API
    # /api/admin/courses/reload 需要 admin 权限
    # 后端 Pod 内部可以直接访问 localhost
    RELOAD_RESULT=$(kubectl exec -n "$NAMESPACE" "$BACKEND_POD" -- \
      wget -q -O- --header='Content-Type: application/json' \
      --post-data='{}' http://localhost:8080/api/admin/courses/reload 2>&1) || true

    if echo "$RELOAD_RESULT" | grep -q '"ok":true'; then
      echo "  重载成功: $RELOAD_RESULT"
    else
      echo "  API 重载失败（可能需要管理员 session），回退到重启方式..."
      echo "  提示：管理员可登录网页后台，进入课程管理页点击'重新加载'"
      echo ""
      echo "  正在重启 backend..."
      kubectl rollout restart deployment/hdu-ride-backend -n "$NAMESPACE"
      kubectl rollout status deployment/hdu-ride-backend -n "$NAMESPACE" --timeout=180s
      echo "  重启完成。"
    fi
  fi
fi

echo ""
echo "============================================"
echo "  内容更新完成！"
echo "  请刷新浏览器页面检查讲义和作业是否正确显示"
echo "============================================"
