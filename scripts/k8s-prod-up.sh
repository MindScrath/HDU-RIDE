#!/usr/bin/env bash
set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"

load_env() {
  file="$1"
  [ -f "$file" ] || return 0
  while IFS= read -r line || [ -n "$line" ]; do
    case "$line" in
      ""|\#*) continue ;;
    esac
    key="${line%%=*}"
    value="${line#*=}"
    [ "$key" = "$line" ] && continue
    [ -n "${key:-}" ] || continue
    eval "current=\${$key:-}"
    [ -n "$current" ] && continue
    export "$key=$value"
  done < "$file"
}

load_env "$ROOT/.env"

NAMESPACE="${NAMESPACE:-${K8S_NAMESPACE:-hdu-ride}}"
BACKEND_IMAGE="${BACKEND_IMAGE:-hdu-ride-backend:latest}"
FRONTEND_IMAGE="${FRONTEND_IMAGE:-hdu-ride-frontend:latest}"
WORKSPACE_IMAGE="${WORKSPACE_IMAGE:-${WORKSPACE_IMAGE_DEFAULT:-rocker/rstudio:4.6.0}}"
POSTGRES_DB="${POSTGRES_DB:-hdu_ride}"

required() {
  eval "value=\${$1:-}"
  if [ -z "$value" ]; then
    printf 'missing required environment variable: %s\n' "$1" >&2
    exit 1
  fi
}

required POSTGRES_USER
required POSTGRES_PASSWORD
required S3_ACCESS_KEY_ID
required S3_SECRET_ACCESS_KEY
required S3_BUCKET
required SESSION_SECRET
required ROOT_USERNAME
required ROOT_PASSWORD_HASH

kubectl apply -f "$ROOT/deploy/k8s/namespace.yml"

kubectl create secret generic postgres-auth -n "$NAMESPACE" \
  --from-literal=username="$POSTGRES_USER" \
  --from-literal=password="$POSTGRES_PASSWORD" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl create secret generic minio-auth -n "$NAMESPACE" \
  --from-literal=MINIO_ROOT_USER="$S3_ACCESS_KEY_ID" \
  --from-literal=MINIO_ROOT_PASSWORD="$S3_SECRET_ACCESS_KEY" \
  --dry-run=client -o yaml | kubectl apply -f -

CLUSTER_DATABASE_URL="postgres://$POSTGRES_USER:$POSTGRES_PASSWORD@postgres.$NAMESPACE.svc.cluster.local:5432/$POSTGRES_DB?sslmode=disable"

kubectl create secret generic hdu-ride-backend-env -n "$NAMESPACE" \
  --from-literal=HTTP_ADDR=":8080" \
  --from-literal=DATABASE_URL="$CLUSTER_DATABASE_URL" \
  --from-literal=CONTENT_ROOT="/content" \
  --from-literal=CONTENT_PVC_NAME="${CONTENT_PVC_NAME:-hdu-ride-content}" \
  --from-literal=S3_ENDPOINT="minio.$NAMESPACE.svc.cluster.local:9000" \
  --from-literal=S3_BUCKET="$S3_BUCKET" \
  --from-literal=S3_ACCESS_KEY_ID="$S3_ACCESS_KEY_ID" \
  --from-literal=S3_SECRET_ACCESS_KEY="$S3_SECRET_ACCESS_KEY" \
  --from-literal=S3_USE_SSL="${S3_USE_SSL:-false}" \
  --from-literal=SESSION_SECRET="$SESSION_SECRET" \
  --from-literal=ROOT_USERNAME="$ROOT_USERNAME" \
  --from-literal=ROOT_PASSWORD_HASH="$ROOT_PASSWORD_HASH" \
  --from-literal=K8S_NAMESPACE="$NAMESPACE" \
  --from-literal=WORKSPACE_IMAGE_DEFAULT="$WORKSPACE_IMAGE" \
  --from-literal=WORKSPACE_STORAGE_CLASS="${WORKSPACE_STORAGE_CLASS:-standard}" \
  --from-literal=WORKSPACE_CPU_REQUEST="${WORKSPACE_CPU_REQUEST:-500m}" \
  --from-literal=WORKSPACE_CPU_LIMIT="${WORKSPACE_CPU_LIMIT:-1}" \
  --from-literal=WORKSPACE_MEM_REQUEST="${WORKSPACE_MEM_REQUEST:-1Gi}" \
  --from-literal=WORKSPACE_MEM_LIMIT="${WORKSPACE_MEM_LIMIT:-2Gi}" \
  --dry-run=client -o yaml | kubectl apply -f -

# 应用生产环境专属的 PV 和 PVC（挂载宿主机目录）
kubectl apply -f "$ROOT/deploy/k8s/content-pvc-prod.yml"

kubectl apply -f "$ROOT/deploy/k8s/postgres.yml"
kubectl apply -f "$ROOT/deploy/k8s/minio.yml"
kubectl wait -n "$NAMESPACE" --for=condition=Ready pod -l app=postgres --timeout=180s
kubectl wait -n "$NAMESPACE" --for=condition=Ready pod -l app=minio --timeout=180s

kubectl run minio-mc -n "$NAMESPACE" --rm -i --restart=Never --image=minio/mc --image-pull-policy=IfNotPresent \
  --env="MINIO_ROOT_USER=$S3_ACCESS_KEY_ID" \
  --env="MINIO_ROOT_PASSWORD=$S3_SECRET_ACCESS_KEY" \
  --command -- sh -c "mc alias set local http://minio:9000 \$MINIO_ROOT_USER \$MINIO_ROOT_PASSWORD && mc mb -p local/$S3_BUCKET || true"

# 注意：生产环境不再需要执行 sync 脚本，因为 content-pvc-prod.yml 直接挂载了物理目录

kubectl apply -f "$ROOT/deploy/k8s/backend.yml"
kubectl set image deployment/hdu-ride-backend -n "$NAMESPACE" backend="$BACKEND_IMAGE"
kubectl rollout status deployment/hdu-ride-backend -n "$NAMESPACE" --timeout=180s

# 部署前端
kubectl apply -f "$ROOT/deploy/k8s/frontend.yml"
kubectl set image deployment/hdu-ride-frontend -n "$NAMESPACE" frontend="$FRONTEND_IMAGE"
kubectl rollout status deployment/hdu-ride-frontend -n "$NAMESPACE" --timeout=180s

printf 'Production Deployment Complete!\n'
printf 'Backend root login: %s\n' "$ROOT_USERNAME"
printf 'Frontend is running on NodePort 30080\n'
