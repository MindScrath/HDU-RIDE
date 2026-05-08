#!/usr/bin/env sh
set -eu

UBUNTU_MIRROR_BASE="${UBUNTU_MIRROR_BASE:-https://mirrors.aliyun.com/ubuntu/}"
K8S_MIRROR_BASE="${K8S_MIRROR_BASE:-https://mirrors.aliyun.com/kubernetes-new/core/stable/v1.29/deb/}"
DOCKER_MIRRORS="${DOCKER_MIRRORS:-https://docker.m.daocloud.io}"
GO_PROXY="${GO_PROXY:-https://goproxy.cn,direct}"
GO_SUMDB="${GO_SUMDB:-sum.golang.google.cn}"
BACKUP_SUFFIX="${BACKUP_SUFFIX:-.bak.hdu-ride}"

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

need_cmd awk
need_cmd sed
need_cmd tee

if [ -n "$SUDO" ]; then
  run_root true
elif [ "$(id -u)" != "0" ]; then
  echo "错误：请使用 root 运行，或安装 sudo 后重新执行。"
  exit 1
fi

configure_ubuntu_apt() {
  if [ -f /etc/apt/sources.list ]; then
    backup="/etc/apt/sources.list$BACKUP_SUFFIX"
    if [ ! -f "$backup" ]; then
      run_root cp /etc/apt/sources.list "$backup"
    fi
    run_root sed -i \
      -e "s|http://archive.ubuntu.com/ubuntu/|$UBUNTU_MIRROR_BASE|g" \
      -e "s|https://archive.ubuntu.com/ubuntu/|$UBUNTU_MIRROR_BASE|g" \
      -e "s|http://security.ubuntu.com/ubuntu/|$UBUNTU_MIRROR_BASE|g" \
      -e "s|https://security.ubuntu.com/ubuntu/|$UBUNTU_MIRROR_BASE|g" \
      /etc/apt/sources.list
    echo "已更新 /etc/apt/sources.list"
  fi

  if [ -f /etc/apt/sources.list.d/ubuntu.sources ]; then
    backup="/etc/apt/sources.list.d/ubuntu.sources$BACKUP_SUFFIX"
    if [ ! -f "$backup" ]; then
      run_root cp /etc/apt/sources.list.d/ubuntu.sources "$backup"
    fi
    run_root sed -i \
      -e "s|http://archive.ubuntu.com/ubuntu/|$UBUNTU_MIRROR_BASE|g" \
      -e "s|https://archive.ubuntu.com/ubuntu/|$UBUNTU_MIRROR_BASE|g" \
      -e "s|http://security.ubuntu.com/ubuntu/|$UBUNTU_MIRROR_BASE|g" \
      -e "s|https://security.ubuntu.com/ubuntu/|$UBUNTU_MIRROR_BASE|g" \
      /etc/apt/sources.list.d/ubuntu.sources
    echo "已更新 /etc/apt/sources.list.d/ubuntu.sources"
  fi
}

configure_kubernetes_apt() {
  need_cmd curl
  need_cmd gpg

  run_root mkdir -p /etc/apt/keyrings
  curl -fsSL "${K8S_MIRROR_BASE}Release.key" | run_root gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg
  printf 'deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] %s /\n' "$K8S_MIRROR_BASE" | run_root tee /etc/apt/sources.list.d/kubernetes.list >/dev/null
  echo "已写入 Kubernetes apt 源"
}

configure_docker_mirrors() {
  run_root mkdir -p /etc/docker

  mirrors_json="$(printf '%s\n' "$DOCKER_MIRRORS" | awk '
    BEGIN { first = 1 }
    NF {
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", $0)
      if ($0 == "") next
      if (!first) printf(",\n")
      printf("    \"%s\"", $0)
      first = 0
    }
  ')"

  if [ -z "$mirrors_json" ]; then
    mirrors_json='    "https://docker.m.daocloud.io"'
  fi

  tmp_file="$(mktemp)"
  cat >"$tmp_file" <<EOF
{
  "registry-mirrors": [
$mirrors_json
  ]
}
EOF
  run_root cp "$tmp_file" /etc/docker/daemon.json
  rm -f "$tmp_file"

  if command -v systemctl >/dev/null 2>&1; then
    run_root systemctl daemon-reload || true
    if run_root systemctl is-enabled docker >/dev/null 2>&1 || run_root systemctl is-active docker >/dev/null 2>&1; then
      run_root systemctl restart docker
    fi
    if run_root systemctl is-enabled containerd >/dev/null 2>&1 || run_root systemctl is-active containerd >/dev/null 2>&1; then
      run_root systemctl restart containerd
    fi
  fi

  echo "已写入 /etc/docker/daemon.json"
}

configure_go_env() {
  run_root mkdir -p /etc/profile.d
  tmp_file="$(mktemp)"
  cat >"$tmp_file" <<EOF
export GOPROXY=$GO_PROXY
export GOSUMDB=$GO_SUMDB
EOF
  run_root cp "$tmp_file" /etc/profile.d/hdu-ride-go-proxy.sh
  rm -f "$tmp_file"
  echo "已写入 /etc/profile.d/hdu-ride-go-proxy.sh"
}

configure_ubuntu_apt
run_root apt update
run_root apt install -y ca-certificates curl gnupg apt-transport-https software-properties-common
configure_kubernetes_apt
configure_docker_mirrors
configure_go_env

run_root apt update

echo
echo "国内镜像初始化完成。"
echo "已配置："
echo "- Ubuntu apt 镜像：$UBUNTU_MIRROR_BASE"
echo "- Kubernetes apt 镜像：$K8S_MIRROR_BASE"
echo "- Docker 镜像加速：$DOCKER_MIRRORS"
echo "- Go 代理：$GO_PROXY"
echo
echo "建议下一步执行："
echo "1. source /etc/profile.d/hdu-ride-go-proxy.sh"
echo "2. sudo systemctl restart docker containerd || true"
echo "3. 按 INSTRUCTION.md 继续安装 Go、kubeadm、Flannel 与业务组件"
