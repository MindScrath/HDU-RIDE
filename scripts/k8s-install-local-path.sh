#!/usr/bin/env sh
set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
MANIFEST="$ROOT/deploy/k8s/local-path-storage-v0.0.28.yaml"
SC_MANIFEST="$ROOT/deploy/k8s/storageclasses-single-node.yml"

LOCAL_PATH_PROVISIONER_VERSION="${LOCAL_PATH_PROVISIONER_VERSION:-v0.0.28}"
LOCAL_PATH_PROVISIONER_IMAGE="${LOCAL_PATH_PROVISIONER_IMAGE:-rancher/local-path-provisioner:${LOCAL_PATH_PROVISIONER_VERSION}}"
LOCAL_PATH_PROVISIONER_PULL_IMAGE="${LOCAL_PATH_PROVISIONER_PULL_IMAGE:-}"
LOCAL_PATH_HELPER_IMAGE="${LOCAL_PATH_HELPER_IMAGE:-busybox:1.36}"
LOCAL_PATH_HELPER_PULL_IMAGE="${LOCAL_PATH_HELPER_PULL_IMAGE:-}"

pull_and_import() {
  pull_image="$1"
  target_image="$2"
  archive_name="$3"
  archive_path="/tmp/${archive_name}"

  sudo docker pull "$pull_image" || return 1
  if [ "$pull_image" != "$target_image" ]; then
    sudo docker tag "$pull_image" "$target_image" || return 1
  fi
  sudo docker save "$target_image" -o "$archive_path" || return 1
  sudo ctr -n k8s.io images import "$archive_path" || return 1
  sudo rm -f "$archive_path"
}

pull_with_fallbacks() {
  target_image="$1"
  explicit_pull_image="$2"
  archive_name="$3"
  shift 3

  if [ -n "$explicit_pull_image" ]; then
    echo "尝试拉取：$explicit_pull_image"
    pull_and_import "$explicit_pull_image" "$target_image" "$archive_name"
    return 0
  fi

  for candidate in "$@"; do
    echo "尝试拉取：$candidate"
    if pull_and_import "$candidate" "$target_image" "$archive_name"; then
      return 0
    fi
    echo "拉取失败，继续尝试下一个镜像源。"
  done

  echo "错误：以下镜像源都无法拉取 $target_image"
  for candidate in "$@"; do
    echo "  - $candidate"
  done
  echo "你可以手工指定 LOCAL_PATH_PROVISIONER_PULL_IMAGE / LOCAL_PATH_HELPER_PULL_IMAGE 后重试。"
  exit 1
}

untaint_single_node() {
  kubectl taint nodes --all node-role.kubernetes.io/control-plane- >/dev/null 2>&1 || true
  kubectl taint nodes --all node-role.kubernetes.io/master- >/dev/null 2>&1 || true
}

pull_with_fallbacks \
  "$LOCAL_PATH_PROVISIONER_IMAGE" \
  "$LOCAL_PATH_PROVISIONER_PULL_IMAGE" \
  "local-path-provisioner.tar" \
  "dockerproxy.com/$LOCAL_PATH_PROVISIONER_IMAGE" \
  "docker.m.daocloud.io/$LOCAL_PATH_PROVISIONER_IMAGE" \
  "$LOCAL_PATH_PROVISIONER_IMAGE"

pull_with_fallbacks \
  "$LOCAL_PATH_HELPER_IMAGE" \
  "$LOCAL_PATH_HELPER_PULL_IMAGE" \
  "local-path-helper.tar" \
  "dockerproxy.com/library/busybox:1.36" \
  "docker.m.daocloud.io/library/busybox:1.36" \
  "$LOCAL_PATH_HELPER_IMAGE"

kubectl apply -f "$MANIFEST"
untaint_single_node
kubectl delete storageclass local-path --ignore-not-found
kubectl delete storageclass standard --ignore-not-found
kubectl apply -f "$SC_MANIFEST"
kubectl rollout status deployment/local-path-provisioner -n local-path-storage --timeout=180s
kubectl get storageclass
