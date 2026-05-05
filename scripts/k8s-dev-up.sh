#!/usr/bin/env sh
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
CLUSTER_NAME="${CLUSTER_NAME:-hdu-ride}"
BACKEND_IMAGE="${BACKEND_IMAGE:-localhost/hdu-ride/backend:dev}"
WORKSPACE_IMAGE="${WORKSPACE_IMAGE:-${WORKSPACE_IMAGE_DEFAULT:-rocker/rstudio:4.6.0}}"
PODMAN_MACHINE_PROXY="${PODMAN_MACHINE_PROXY:-}"
PORT_FORWARD="${PORT_FORWARD:-}"
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

if [ -z "${ROOT_PASSWORD_HASH:-}" ]; then
  required ROOT_PASSWORD
  ROOT_PASSWORD_HASH="$(cd "$ROOT/backend" && go run . hash-password "$ROOT_PASSWORD")"
fi

import_kind_image() {
  image="$1"
  safe="$(printf '%s' "$image" | tr '/:' '--')"
  archive="${TMPDIR:-/tmp}/$safe.tar"
  podman save -o "$archive" "$image"
  kind load image-archive --name "$CLUSTER_NAME" "$archive"
  rm -f "$archive"
}

ensure_podman_image() {
  image="$1"
  if podman image exists "$image" >/dev/null 2>&1; then
    return
  fi
  if [ -n "$PODMAN_MACHINE_PROXY" ]; then
    podman machine ssh "export HTTP_PROXY=$PODMAN_MACHINE_PROXY HTTPS_PROXY=$PODMAN_MACHINE_PROXY http_proxy=$PODMAN_MACHINE_PROXY https_proxy=$PODMAN_MACHINE_PROXY; podman pull $image"
  else
    podman pull "$image"
  fi
}

prepare_kind_image() {
  ensure_podman_image "$1"
  import_kind_image "$1"
}

kubectl config current-context >/dev/null

for image in \
  docker.io/library/postgres:18-alpine \
  docker.io/minio/minio:latest \
  docker.io/minio/mc:latest \
  docker.io/library/alpine:3.22 \
  docker.io/library/busybox:1.36 \
  "$WORKSPACE_IMAGE"
do
  prepare_kind_image "$image"
done
import_kind_image "$BACKEND_IMAGE"

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

kubectl apply -f "$ROOT/deploy/k8s/content-pvc.yml"
kubectl apply -f "$ROOT/deploy/k8s/postgres.yml"
kubectl apply -f "$ROOT/deploy/k8s/minio.yml"
kubectl wait -n "$NAMESPACE" --for=condition=Ready pod -l app=postgres --timeout=180s
kubectl wait -n "$NAMESPACE" --for=condition=Ready pod -l app=minio --timeout=180s

kubectl run minio-mc -n "$NAMESPACE" --rm -i --restart=Never --image=minio/mc --image-pull-policy=IfNotPresent \
  --env="MINIO_ROOT_USER=$S3_ACCESS_KEY_ID" \
  --env="MINIO_ROOT_PASSWORD=$S3_SECRET_ACCESS_KEY" \
  --command -- sh -c "mc alias set local http://minio:9000 \$MINIO_ROOT_USER \$MINIO_ROOT_PASSWORD && mc mb -p local/$S3_BUCKET || true"

NAMESPACE="$NAMESPACE" CONTENT_DIR="$ROOT/content" sh "$ROOT/scripts/k8s-sync-content.sh"

kubectl apply -f "$ROOT/deploy/k8s/backend.yml"
kubectl set image deployment/hdu-ride-backend -n "$NAMESPACE" backend="$BACKEND_IMAGE"
kubectl rollout status deployment/hdu-ride-backend -n "$NAMESPACE" --timeout=180s

printf 'Backend root login: %s\n' "$ROOT_USERNAME"
printf 'Backend service: svc/hdu-ride-backend.%s:8080\n' "$NAMESPACE"

if [ -n "$PORT_FORWARD" ]; then
  printf 'Starting kubectl port-forward on http://127.0.0.1:8080\n'
  kubectl port-forward -n "$NAMESPACE" svc/hdu-ride-backend 8080:8080
fi
