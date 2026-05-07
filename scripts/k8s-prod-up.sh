#!/usr/bin/env sh
set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
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

EXPECTED_CONTENT_SC="${CONTENT_STORAGE_CLASS:-$(read_env_value WORKSPACE_STORAGE_CLASS standard)}"
EXPECTED_CONTENT_SIZE="${CONTENT_STORAGE_SIZE:-20Gi}"
CONTENT_NAMESPACE="${CONTENT_NAMESPACE:-$(read_env_value K8S_NAMESPACE hdu-ride)}"
CONTENT_PV_NAME="${CONTENT_PV_NAME:-hdu-ride-content-pv}"
CONTENT_PVC_NAME="${CONTENT_PVC_NAME:-hdu-ride-content}"
CONTENT_PVC_MANIFEST="$ROOT/deploy/k8s/content-pvc-prod.yml"

if [ ! -f "$CONTENT_PVC_MANIFEST" ]; then
  echo "错误：找不到生产内容卷清单 $CONTENT_PVC_MANIFEST"
  exit 1
fi

SC_DECLARED_COUNT="$(grep -c "storageClassName: ${EXPECTED_CONTENT_SC}" "$CONTENT_PVC_MANIFEST" || true)"
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

if command -v kubectl >/dev/null 2>&1 && kubectl get pv "$CONTENT_PV_NAME" >/dev/null 2>&1; then
  SC_CHECK="$(kubectl get pv "$CONTENT_PV_NAME" -o jsonpath='{.spec.storageClassName}')"
  SIZE_CHECK="$(kubectl get pv "$CONTENT_PV_NAME" -o jsonpath='{.spec.capacity.storage}')"
  RECREATE_CONTENT_VOLUME="0"
  if [ "$SC_CHECK" != "$EXPECTED_CONTENT_SC" ] || [ "$SIZE_CHECK" != "$EXPECTED_CONTENT_SIZE" ]; then
    RECREATE_CONTENT_VOLUME="1"
  fi
  if kubectl get pvc "$CONTENT_PVC_NAME" -n "$CONTENT_NAMESPACE" >/dev/null 2>&1; then
    PVC_SC_CHECK="$(kubectl get pvc "$CONTENT_PVC_NAME" -n "$CONTENT_NAMESPACE" -o jsonpath='{.spec.storageClassName}')"
    PVC_SIZE_CHECK="$(kubectl get pvc "$CONTENT_PVC_NAME" -n "$CONTENT_NAMESPACE" -o jsonpath='{.spec.resources.requests.storage}')"
    if [ "$PVC_SC_CHECK" != "$EXPECTED_CONTENT_SC" ] || [ "$PVC_SIZE_CHECK" != "$EXPECTED_CONTENT_SIZE" ]; then
      RECREATE_CONTENT_VOLUME="1"
    fi
  fi
  if [ "$RECREATE_CONTENT_VOLUME" = "1" ]; then
    echo "检测到静态内容卷属性与期望不符，正在删除旧的 PV/PVC 后重建..."
    kubectl delete pvc "$CONTENT_PVC_NAME" -n "$CONTENT_NAMESPACE" --ignore-not-found
    kubectl delete pv "$CONTENT_PV_NAME" --ignore-not-found
  fi
fi

cd "$ROOT/backend"
go run . ops k8s-prod-up
