#!/usr/bin/env sh
set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
MANIFEST="$ROOT/deploy/k8s/local-path-storage-v0.0.28.yaml"
SC_MANIFEST="$ROOT/deploy/k8s/storageclasses-single-node.yml"

LOCAL_PATH_PROVISIONER_VERSION="${LOCAL_PATH_PROVISIONER_VERSION:-v0.0.28}"
LOCAL_PATH_PROVISIONER_IMAGE="${LOCAL_PATH_PROVISIONER_IMAGE:-rancher/local-path-provisioner:${LOCAL_PATH_PROVISIONER_VERSION}}"
LOCAL_PATH_PROVISIONER_PULL_IMAGE="${LOCAL_PATH_PROVISIONER_PULL_IMAGE:-$LOCAL_PATH_PROVISIONER_IMAGE}"
LOCAL_PATH_HELPER_IMAGE="${LOCAL_PATH_HELPER_IMAGE:-busybox:1.36}"
LOCAL_PATH_HELPER_PULL_IMAGE="${LOCAL_PATH_HELPER_PULL_IMAGE:-$LOCAL_PATH_HELPER_IMAGE}"

pull_and_import() {
  pull_image="$1"
  target_image="$2"
  archive_name="$3"
  archive_path="/tmp/${archive_name}"

  sudo docker pull "$pull_image"
  if [ "$pull_image" != "$target_image" ]; then
    sudo docker tag "$pull_image" "$target_image"
  fi
  sudo docker save "$target_image" -o "$archive_path"
  sudo ctr -n k8s.io images import "$archive_path"
  rm -f "$archive_path"
}

untaint_single_node() {
  kubectl taint nodes --all node-role.kubernetes.io/control-plane- >/dev/null 2>&1 || true
  kubectl taint nodes --all node-role.kubernetes.io/master- >/dev/null 2>&1 || true
}

pull_and_import "$LOCAL_PATH_PROVISIONER_PULL_IMAGE" "$LOCAL_PATH_PROVISIONER_IMAGE" "local-path-provisioner.tar"
pull_and_import "$LOCAL_PATH_HELPER_PULL_IMAGE" "$LOCAL_PATH_HELPER_IMAGE" "local-path-helper.tar"

kubectl apply -f "$MANIFEST"
untaint_single_node
kubectl delete storageclass local-path --ignore-not-found
kubectl delete storageclass standard --ignore-not-found
kubectl apply -f "$SC_MANIFEST"
kubectl rollout status deployment/local-path-provisioner -n local-path-storage --timeout=180s
kubectl get storageclass
