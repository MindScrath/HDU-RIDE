#!/usr/bin/env sh
set -eu

K8S_VERSION_MINOR="${K8S_VERSION_MINOR:-v1.29}"
K8S_MIRROR_BASE="${K8S_MIRROR_BASE:-https://mirrors.aliyun.com/kubernetes-new/core/stable/$K8S_VERSION_MINOR/deb/}"
PAUSE_IMAGE="${PAUSE_IMAGE:-registry.aliyuncs.com/google_containers/pause:3.9}"
CONTAINERD_CONFIG="${CONTAINERD_CONFIG:-/etc/containerd/config.toml}"
SYSCTL_FILE="${SYSCTL_FILE:-/etc/sysctl.d/99-kubernetes-cri.conf}"
MODULES_FILE="${MODULES_FILE:-/etc/modules-load.d/br_netfilter.conf}"
K8S_APT_LIST="${K8S_APT_LIST:-/etc/apt/sources.list.d/kubernetes.list}"
K8S_KEYRING="${K8S_KEYRING:-/etc/apt/keyrings/kubernetes-apt-keyring.gpg}"

if command -v sudo >/dev/null 2>&1; then
  SUDO="sudo"
else
  SUDO=""
fi

run_root() {
  if [ -n "$SUDO" ]; then
    $SUDO "$@"
  else
    "$@"
  fi
}

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "错误：缺少命令 $1"
    exit 1
  fi
}

if [ -n "$SUDO" ]; then
  run_root true
elif [ "$(id -u)" != "0" ]; then
  echo "错误：请使用 root 运行，或安装 sudo 后重新执行。"
  exit 1
fi

need_cmd curl
need_cmd gpg
need_cmd sed
need_cmd tee
need_cmd systemctl
need_cmd containerd

configure_kernel() {
  run_root modprobe br_netfilter
  printf 'br_netfilter\n' | run_root tee "$MODULES_FILE" >/dev/null

  cat <<'EOF' | run_root tee "$SYSCTL_FILE" >/dev/null
net.bridge.bridge-nf-call-iptables = 1
net.ipv4.ip_forward = 1
EOF

  run_root sysctl --system >/dev/null
  run_root swapoff -a
  if [ -f /etc/fstab ]; then
    run_root sed -i '/ swap / s/^/#/' /etc/fstab
  fi
}

configure_kubernetes_repo() {
  run_root mkdir -p /etc/apt/keyrings
  curl -fsSL "${K8S_MIRROR_BASE}Release.key" | run_root gpg --dearmor -o "$K8S_KEYRING"
  printf 'deb [signed-by=%s] %s /\n' "$K8S_KEYRING" "$K8S_MIRROR_BASE" | run_root tee "$K8S_APT_LIST" >/dev/null
}

install_packages() {
  run_root apt update
  run_root apt install -y kubelet kubeadm kubectl
  run_root apt-mark hold kubelet kubeadm kubectl
  run_root systemctl enable --now kubelet
}

configure_containerd() {
  run_root mkdir -p "$(dirname "$CONTAINERD_CONFIG")"
  containerd config default | run_root tee "$CONTAINERD_CONFIG" >/dev/null

  run_root sed -i 's/SystemdCgroup = false/SystemdCgroup = true/' "$CONTAINERD_CONFIG"
  run_root sed -i "s|sandbox = .*|sandbox = '$PAUSE_IMAGE'|g" "$CONTAINERD_CONFIG"

  run_root systemctl restart containerd
  run_root systemctl restart kubelet || true
}

configure_kernel
configure_kubernetes_repo
install_packages
configure_containerd

echo
echo "Kubernetes 基础环境初始化完成。"
echo "- Kubernetes apt 源：$K8S_MIRROR_BASE"
echo "- pause 镜像：$PAUSE_IMAGE"
echo "- containerd 配置：$CONTAINERD_CONFIG"
echo
echo "建议下一步执行："
echo "1. sudo kubeadm init --pod-network-cidr=10.244.0.0/16 --image-repository registry.aliyuncs.com/google_containers"
echo "2. 配置当前用户的 kubectl"
echo "3. bash scripts/k8s-install-flannel.sh"
echo "4. bash scripts/k8s-install-local-path.sh"
