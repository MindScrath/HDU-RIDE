#!/usr/bin/env sh
set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
GO_MOD_FILE="${GO_MOD_FILE:-$ROOT/backend/go.mod}"
GO_VERSION="${GO_VERSION:-}"
GO_ARCH="${GO_ARCH:-$(uname -m)}"
GO_OS="${GO_OS:-linux}"
GO_INSTALL_DIR="${GO_INSTALL_DIR:-/usr/local}"
GO_PROFILE_FILE="${GO_PROFILE_FILE:-/etc/profile.d/go.sh}"
TMP_DIR="${TMP_DIR:-/tmp}"

if [ -z "$GO_VERSION" ] && [ -f "$GO_MOD_FILE" ]; then
  GO_VERSION="$(awk '/^go[[:space:]]+[0-9]+\.[0-9]+(\.[0-9]+)?$/ { print $2; exit }' "$GO_MOD_FILE")"
fi

if [ -z "$GO_VERSION" ]; then
  GO_VERSION="1.26.0"
fi

case "$GO_ARCH" in
  x86_64|amd64) GO_ARCH="amd64" ;;
  aarch64|arm64) GO_ARCH="arm64" ;;
  *)
    echo "错误：暂不支持的架构 $GO_ARCH，请手动设置 GO_ARCH=amd64 或 GO_ARCH=arm64"
    exit 1
    ;;
esac

GO_TARBALL="go${GO_VERSION}.${GO_OS}-${GO_ARCH}.tar.gz"
GO_MIRRORS="${GO_MIRRORS:-https://mirrors.aliyun.com/golang
https://golang.google.cn/dl
https://go.dev/dl}"

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

download_file() {
  target_path="$1"
  for base in $GO_MIRRORS; do
    url="${base%/}/$GO_TARBALL"
    echo "尝试下载：$url"
    if command -v curl >/dev/null 2>&1; then
      if curl -fL --connect-timeout 15 --retry 2 --retry-delay 2 -o "$target_path" "$url"; then
        echo "下载成功：$url"
        return 0
      fi
    elif command -v wget >/dev/null 2>&1; then
      if wget -O "$target_path" "$url"; then
        echo "下载成功：$url"
        return 0
      fi
    else
      echo "错误：缺少 curl 或 wget"
      exit 1
    fi
    echo "下载失败，切换下一个镜像源。"
  done
  return 1
}

need_cmd tar
need_cmd awk
need_cmd uname

if [ -n "$SUDO" ]; then
  run_root true
elif [ "$(id -u)" != "0" ]; then
  echo "错误：请使用 root 运行，或安装 sudo 后重新执行。"
  exit 1
fi

archive_path="$TMP_DIR/$GO_TARBALL"
download_file "$archive_path" || {
  echo "错误：所有 Go 下载源都失败了，请检查网络或手动设置 GO_MIRRORS。"
  exit 1
}

run_root rm -rf "$GO_INSTALL_DIR/go"
run_root tar -C "$GO_INSTALL_DIR" -xzf "$archive_path"

profile_tmp="$(mktemp)"
cat >"$profile_tmp" <<EOF
export PATH=$GO_INSTALL_DIR/go/bin:\$PATH
EOF
run_root mkdir -p "$(dirname "$GO_PROFILE_FILE")"
run_root cp "$profile_tmp" "$GO_PROFILE_FILE"
rm -f "$profile_tmp"

echo
echo "Go 安装完成。"
echo "- 版本：$GO_VERSION"
echo "- 架构：$GO_ARCH"
echo "- 安装目录：$GO_INSTALL_DIR/go"
echo "- 环境文件：$GO_PROFILE_FILE"
echo
echo "请执行："
echo "source $GO_PROFILE_FILE"
echo "go version"
