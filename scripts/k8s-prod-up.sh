#!/usr/bin/env sh
set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
EXPECTED_CONTENT_SC="${CONTENT_STORAGE_CLASS:-local-path}"
CONTENT_PV_NAME="${CONTENT_PV_NAME:-hdu-ride-content-pv}"
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

if command -v kubectl >/dev/null 2>&1 && kubectl get pv "$CONTENT_PV_NAME" >/dev/null 2>&1; then
  SC_CHECK="$(kubectl get pv "$CONTENT_PV_NAME" -o jsonpath='{.spec.storageClassName}')"
  if [ "$SC_CHECK" != "$EXPECTED_CONTENT_SC" ]; then
    echo "错误：静态 PV ${CONTENT_PV_NAME} 的 storageClassName 必须设为 ${EXPECTED_CONTENT_SC}！"
    exit 1
  fi
fi

cd "$ROOT/backend"
go run . ops k8s-prod-up
