#!/usr/bin/env sh
set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TAG="${TAG:-dev}"
PREFIX="${PREFIX:-localhost/hdu-ride}"
PODMAN_MACHINE_PROXY="${PODMAN_MACHINE_PROXY:-}"

BACKEND_IMAGE="$PREFIX/backend:$TAG"
FRONTEND_IMAGE="$PREFIX/frontend:$TAG"
RSTUDIO_IMAGE="$PREFIX/rstudio:$TAG"

podman info >/dev/null

pull_image() {
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

for image in \
  docker.io/library/golang:1.26-alpine \
  docker.io/library/alpine:3.22 \
  docker.io/oven/bun:1.3 \
  docker.io/library/nginx:1.29-alpine \
  docker.io/rocker/rstudio:4.6.0
do
  pull_image "$image"
done

BUILD_ARGS=""
if [ -n "$PODMAN_MACHINE_PROXY" ]; then
  BUILD_ARGS="--build-arg HTTP_PROXY=$PODMAN_MACHINE_PROXY --build-arg HTTPS_PROXY=$PODMAN_MACHINE_PROXY --build-arg http_proxy=$PODMAN_MACHINE_PROXY --build-arg https_proxy=$PODMAN_MACHINE_PROXY"
fi

# shellcheck disable=SC2086
printf 'Building %s\n' "$BACKEND_IMAGE"
podman build $BUILD_ARGS -f "$ROOT/deploy/docker/backend.Dockerfile" -t "$BACKEND_IMAGE" "$ROOT"
# shellcheck disable=SC2086
printf 'Building %s\n' "$FRONTEND_IMAGE"
podman build $BUILD_ARGS -f "$ROOT/deploy/docker/frontend.Dockerfile" -t "$FRONTEND_IMAGE" "$ROOT"
# shellcheck disable=SC2086
printf 'Building %s\n' "$RSTUDIO_IMAGE"
podman build $BUILD_ARGS -f "$ROOT/deploy/docker/rstudio.Dockerfile" -t "$RSTUDIO_IMAGE" "$ROOT"

printf 'Built images:\n  %s\n  %s\n  %s\n' "$BACKEND_IMAGE" "$FRONTEND_IMAGE" "$RSTUDIO_IMAGE"
