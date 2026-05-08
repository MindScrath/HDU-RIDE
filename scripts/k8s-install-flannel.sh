#!/usr/bin/env sh
set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
MANIFEST="$ROOT/deploy/k8s/kube-flannel.yml"

FLANNEL_IMAGE="${FLANNEL_IMAGE:-ghcr.io/flannel-io/flannel:v0.28.4}"
FLANNEL_CNI_IMAGE="${FLANNEL_CNI_IMAGE:-ghcr.io/flannel-io/flannel-cni-plugin:v1.9.1-flannel1}"
FLANNEL_PULL_IMAGE="${FLANNEL_PULL_IMAGE:-docker.m.daocloud.io/$FLANNEL_IMAGE}"
FLANNEL_CNI_PULL_IMAGE="${FLANNEL_CNI_PULL_IMAGE:-docker.m.daocloud.io/$FLANNEL_CNI_IMAGE}"

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "错误：缺少命令 $1"
    exit 1
  fi
}

pull_and_tag() {
  pull_image="$1"
  target_image="$2"

  sudo ctr -n k8s.io images pull "$pull_image"
  if [ "$pull_image" != "$target_image" ]; then
    sudo ctr -n k8s.io images rm "$target_image" >/dev/null 2>&1 || true
    sudo ctr -n k8s.io images tag "$pull_image" "$target_image"
  fi
}

require_command kubectl
require_command ctr

if [ ! -f "$MANIFEST" ]; then
  echo "错误：找不到 Flannel 清单 $MANIFEST"
  exit 1
fi

pull_and_tag "$FLANNEL_PULL_IMAGE" "$FLANNEL_IMAGE"
pull_and_tag "$FLANNEL_CNI_PULL_IMAGE" "$FLANNEL_CNI_IMAGE"

kubectl apply -f "$MANIFEST"
kubectl rollout status daemonset/kube-flannel-ds -n kube-flannel --timeout=180s
kubectl get pods -n kube-flannel -o wide
