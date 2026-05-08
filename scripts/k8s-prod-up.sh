#!/usr/bin/env sh
set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "错误：缺少命令 $1"
    exit 1
  fi
}

read_env_value() {
  key="$1"
  fallback="${2:-}"
  env_file="$ROOT/.env"
  if [ ! -f "$env_file" ]; then
    printf '%s\n' "$fallback"
    return
  fi
  value="$(awk -F= -v key="$key" '
    /^[[:space:]]*#/ { next }
    $1 ~ /^[[:space:]]*$/ { next }
    {
      k=$1
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", k)
      if (k == key) {
        sub(/^[^=]*=/, "", $0)
        gsub(/^[[:space:]]+|[[:space:]]+$/, "", $0)
        gsub(/^["'"'"']|["'"'"']$/, "", $0)
        print $0
        exit
      }
    }
  ' "$env_file")"
  if [ -n "$value" ]; then
    printf '%s\n' "$value"
  else
    printf '%s\n' "$fallback"
  fi
}

untaint_single_node() {
  kubectl taint nodes --all node-role.kubernetes.io/control-plane- >/dev/null 2>&1 || true
  kubectl taint nodes --all node-role.kubernetes.io/master- >/dev/null 2>&1 || true
}

ensure_local_path_storage() {
  need_install="0"

  if ! kubectl get storageclass local-path >/dev/null 2>&1; then
    need_install="1"
  fi
  if ! kubectl get deployment local-path-provisioner -n local-path-storage >/dev/null 2>&1; then
    need_install="1"
  fi

  if [ "$need_install" = "1" ]; then
    echo "检测到 local-path 动态存储尚未就绪，正在自动执行 scripts/k8s-install-local-path.sh ..."
    sh "$ROOT/scripts/k8s-install-local-path.sh"
  fi
}

EXPECTED_WORKSPACE_SC="${WORKSPACE_STORAGE_CLASS:-$(read_env_value WORKSPACE_STORAGE_CLASS local-path)}"
EXPECTED_CONTENT_SC="${CONTENT_STORAGE_CLASS:-}"
EXPECTED_CONTENT_SIZE="${CONTENT_STORAGE_SIZE:-20Gi}"
CONTENT_NAMESPACE="${CONTENT_NAMESPACE:-$(read_env_value K8S_NAMESPACE hdu-ride)}"
CONTENT_PV_NAME="${CONTENT_PV_NAME:-hdu-ride-content-pv}"
CONTENT_PVC_NAME="${CONTENT_PVC_NAME:-hdu-ride-content}"
CONTENT_PVC_MANIFEST="$ROOT/deploy/k8s/content-pvc-prod.yml"

require_command kubectl
require_command go

export GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"
export GOSUMDB="${GOSUMDB:-sum.golang.google.cn}"

if [ ! -f "$CONTENT_PVC_MANIFEST" ]; then
  echo "错误：找不到生产内容卷清单 $CONTENT_PVC_MANIFEST"
  exit 1
fi

ensure_local_path_storage

if [ "$EXPECTED_CONTENT_SC" = "" ]; then
  SC_DECLARED_COUNT="$(grep -c 'storageClassName: ""' "$CONTENT_PVC_MANIFEST" || true)"
else
  SC_DECLARED_COUNT="$(grep -c "storageClassName: ${EXPECTED_CONTENT_SC}" "$CONTENT_PVC_MANIFEST" || true)"
fi
if [ "$SC_DECLARED_COUNT" -lt 2 ]; then
  echo "错误：$CONTENT_PVC_MANIFEST 中的 PV/PVC 都必须声明 storageClassName: ${EXPECTED_CONTENT_SC}"
  exit 1
fi

SIZE_DECLARED_COUNT="$(grep -c "storage: ${EXPECTED_CONTENT_SIZE}" "$CONTENT_PVC_MANIFEST" || true)"
if [ "$SIZE_DECLARED_COUNT" -lt 2 ]; then
  echo "错误：$CONTENT_PVC_MANIFEST 中的 PV/PVC 容量都必须声明为 ${EXPECTED_CONTENT_SIZE}"
  exit 1
fi

if command -v kubectl >/dev/null 2>&1; then
  untaint_single_node
fi

RECREATE_CONTENT_VOLUME="0"
if command -v kubectl >/dev/null 2>&1; then
  if kubectl get pv "$CONTENT_PV_NAME" >/dev/null 2>&1; then
    SC_CHECK="$(kubectl get pv "$CONTENT_PV_NAME" -o jsonpath='{.spec.storageClassName}')"
    SIZE_CHECK="$(kubectl get pv "$CONTENT_PV_NAME" -o jsonpath='{.spec.capacity.storage}')"
    if [ "$SC_CHECK" != "$EXPECTED_CONTENT_SC" ] || [ "$SIZE_CHECK" != "$EXPECTED_CONTENT_SIZE" ]; then
      RECREATE_CONTENT_VOLUME="1"
    fi
  fi
  if kubectl get pvc "$CONTENT_PVC_NAME" -n "$CONTENT_NAMESPACE" >/dev/null 2>&1; then
    PVC_SC_CHECK="$(kubectl get pvc "$CONTENT_PVC_NAME" -n "$CONTENT_NAMESPACE" -o jsonpath='{.spec.storageClassName}')"
    PVC_SIZE_CHECK="$(kubectl get pvc "$CONTENT_PVC_NAME" -n "$CONTENT_NAMESPACE" -o jsonpath='{.spec.resources.requests.storage}')"
    if [ "$PVC_SC_CHECK" != "$EXPECTED_CONTENT_SC" ] || [ "$PVC_SIZE_CHECK" != "$EXPECTED_CONTENT_SIZE" ]; then
      RECREATE_CONTENT_VOLUME="1"
    fi
  fi
fi

if [ "$RECREATE_CONTENT_VOLUME" = "1" ]; then
  echo "检测到静态内容卷属性与期望不符，正在删除旧的 PV/PVC 后重建..."
  kubectl delete pvc "$CONTENT_PVC_NAME" -n "$CONTENT_NAMESPACE" --ignore-not-found
  kubectl delete pv "$CONTENT_PV_NAME" --ignore-not-found
fi

if [ "$EXPECTED_WORKSPACE_SC" = "" ]; then
  echo "错误：WORKSPACE_STORAGE_CLASS 不能为空，单节点动态工作区建议使用 local-path"
  exit 1
fi

if ! kubectl get storageclass "$EXPECTED_WORKSPACE_SC" >/dev/null 2>&1; then
  echo "错误：找不到动态存储类 $EXPECTED_WORKSPACE_SC，请检查 .env 中的 WORKSPACE_STORAGE_CLASS 或先完成 local-path 安装"
  exit 1
fi

cd "$ROOT/backend"
go run . ops k8s-prod-up
