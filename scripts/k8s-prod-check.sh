#!/usr/bin/env sh
set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUTPUT_DIR="${OUTPUT_DIR:-$ROOT/.diagnostics}"
TIMESTAMP="$(date +%Y%m%d-%H%M%S)"
REPORT_PATH="${REPORT_PATH:-$OUTPUT_DIR/k8s-prod-check-$TIMESTAMP.txt}"
ENV_FILE="$ROOT/.env"
NAMESPACE="${K8S_NAMESPACE:-}"

mkdir -p "$OUTPUT_DIR"

if [ -z "$NAMESPACE" ] && [ -f "$ENV_FILE" ]; then
  NAMESPACE="$(awk -F= '
    /^[[:space:]]*#/ { next }
    $1 ~ /^[[:space:]]*$/ { next }
    {
      key=$1
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", key)
      if (key == "K8S_NAMESPACE") {
        sub(/^[^=]*=/, "", $0)
        gsub(/^[[:space:]]+|[[:space:]]+$/, "", $0)
        gsub(/^["'"'"']|["'"'"']$/, "", $0)
        print $0
        exit
      }
    }
  ' "$ENV_FILE")"
fi

if [ -z "$NAMESPACE" ]; then
  NAMESPACE="hdu-ride"
fi

run_section() {
  title="$1"
  shift
  {
    printf '\n===== %s =====\n' "$title"
    printf '$'
    for arg in "$@"; do
      printf ' %s' "$arg"
    done
    printf '\n'
    if "$@"; then
      :
    else
      status=$?
      printf '[command failed with exit code %s]\n' "$status"
    fi
  } >>"$REPORT_PATH" 2>&1
}

run_shell_section() {
  title="$1"
  command_text="$2"
  {
    printf '\n===== %s =====\n' "$title"
    printf '$ %s\n' "$command_text"
    if sh -c "$command_text"; then
      :
    else
      status=$?
      printf '[command failed with exit code %s]\n' "$status"
    fi
  } >>"$REPORT_PATH" 2>&1
}

write_header() {
  {
    printf 'HDU RIDE Kubernetes Production Diagnostic Report\n'
    printf 'Generated at: %s\n' "$(date '+%Y-%m-%d %H:%M:%S %z')"
    printf 'Repository root: %s\n' "$ROOT"
    printf 'Namespace: %s\n' "$NAMESPACE"
    printf 'Hostname: %s\n' "$(hostname 2>/dev/null || printf unknown)"
    printf 'User: %s\n' "$(id -un 2>/dev/null || printf unknown)"
    printf 'Kernel: %s\n' "$(uname -a 2>/dev/null || printf unknown)"
    printf '\n'
    printf 'Share guidance:\n'
    printf '- Default output masks common secrets from .env and Kubernetes Secret yaml.\n'
    printf '- Review the report once before sharing externally.\n'
    printf '- File path: %s\n' "$REPORT_PATH"
    printf '\n'
  } >"$REPORT_PATH"
}

mask_value() {
  value="$1"
  if [ -z "$value" ]; then
    printf '[empty]'
    return
  fi
  printf '[masked len=%s]' "$(printf '%s' "$value" | wc -c | awk '{print $1}')"
}

write_env_summary() {
  {
    printf '\n===== ENV SUMMARY (.env, masked) =====\n'
    if [ ! -f "$ENV_FILE" ]; then
      printf '.env not found at %s\n' "$ENV_FILE"
      return
    fi
    while IFS='=' read -r key raw_value; do
      key="$(printf '%s' "$key" | sed 's/^[[:space:]]*//; s/[[:space:]]*$//')"
      if [ -z "$key" ]; then
        continue
      fi
      case "$key" in
        \#*) continue ;;
      esac
      value="$(printf '%s' "$raw_value" | sed 's/^[[:space:]]*//; s/[[:space:]]*$//; s/^["'"'"']//; s/["'"'"']$//')"
      case "$key" in
        POSTGRES_PASSWORD|DATABASE_URL|S3_SECRET_ACCESS_KEY|S3_ACCESS_KEY_ID|SESSION_SECRET|ROOT_PASSWORD|ROOT_PASSWORD_HASH)
          printf '%s=%s\n' "$key" "$(mask_value "$value")"
          ;;
        *)
          printf '%s=%s\n' "$key" "$value"
          ;;
      esac
    done <"$ENV_FILE"
  } >>"$REPORT_PATH"
}

command_exists() {
  command -v "$1" >/dev/null 2>&1
}

write_missing_command() {
  {
    printf '\n===== %s =====\n' "$1"
    printf '[skipped: command not found]\n'
  } >>"$REPORT_PATH"
}

write_header
write_env_summary

if command_exists kubectl; then
  run_section "KUBECTL VERSION" kubectl version --client --output=yaml
  run_section "KUBECTL CONTEXT" kubectl config current-context
  run_section "CLUSTER INFO" kubectl cluster-info
  run_section "NODES" kubectl get nodes -o wide
  run_section "NODE DESCRIBE" kubectl describe nodes
  run_section "ALL STORAGECLASS" kubectl get storageclass
  run_section "ALL PV" kubectl get pv
  run_section "NAMESPACE LIST" kubectl get ns
  run_section "HDU-RIDE PVC" kubectl get pvc -n "$NAMESPACE"
  run_section "HDU-RIDE SVC" kubectl get svc -n "$NAMESPACE"
  run_section "HDU-RIDE DEPLOY" kubectl get deploy -n "$NAMESPACE" -o wide
  run_section "HDU-RIDE STS" kubectl get sts -n "$NAMESPACE" -o wide
  run_section "HDU-RIDE PODS" kubectl get pods -n "$NAMESPACE" -o wide
  run_section "LOCAL-PATH PODS" kubectl get pods -n local-path-storage -o wide
  run_section "KUBE-SYSTEM PODS" kubectl get pods -n kube-system -o wide
  run_section "HDU-RIDE EVENTS" kubectl get events -n "$NAMESPACE" --sort-by=.lastTimestamp
  run_section "KUBE-SYSTEM EVENTS" kubectl get events -n kube-system --sort-by=.lastTimestamp
  run_section "LOCAL-PATH EVENTS" kubectl get events -n local-path-storage --sort-by=.lastTimestamp
  run_section "CONTENT PV YAML" kubectl get pv hdu-ride-content-pv -o yaml
  run_section "CONTENT PVC YAML" kubectl get pvc hdu-ride-content -n "$NAMESPACE" -o yaml
  run_section "BACKEND DEPLOY YAML" kubectl get deploy hdu-ride-backend -n "$NAMESPACE" -o yaml
  run_section "FRONTEND DEPLOY YAML" kubectl get deploy hdu-ride-frontend -n "$NAMESPACE" -o yaml
  run_section "POSTGRES STS YAML" kubectl get sts postgres -n "$NAMESPACE" -o yaml
  run_section "MINIO STS YAML" kubectl get sts minio -n "$NAMESPACE" -o yaml
  run_section "BACKEND LOGS" kubectl logs -n "$NAMESPACE" -l app.kubernetes.io/name=hdu-ride-backend --tail=400
  run_section "FRONTEND LOGS" kubectl logs -n "$NAMESPACE" -l app.kubernetes.io/name=hdu-ride-frontend --tail=200
  run_section "POSTGRES LOGS" kubectl logs -n "$NAMESPACE" postgres-0 --tail=200
  run_section "MINIO LOGS" kubectl logs -n "$NAMESPACE" minio-0 --tail=200

  run_shell_section "RSTUDIO POD LIST" "kubectl get pods -n '$NAMESPACE' | grep '^rstudio-' || true"
  run_shell_section "RSTUDIO PVC LIST" "kubectl get pvc -n '$NAMESPACE' | grep '^home-' || true"
  run_shell_section "RSTUDIO SERVICE LIST" "kubectl get svc -n '$NAMESPACE' | grep 'rstudio-' || true"
  run_shell_section "PENDING POD DESCRIBE" "for p in \$(kubectl get pods -n '$NAMESPACE' --no-headers 2>/dev/null | awk '\$3 != \"Running\" && \$3 != \"Completed\" {print \$1}'); do echo; echo '--- POD:' \$p '---'; kubectl describe pod -n '$NAMESPACE' \$p; done"
  run_shell_section "PENDING PVC DESCRIBE" "for p in \$(kubectl get pvc -n '$NAMESPACE' --no-headers 2>/dev/null | awk '\$2 != \"Bound\" {print \$1}'); do echo; echo '--- PVC:' \$p '---'; kubectl describe pvc -n '$NAMESPACE' \$p; done"
  run_shell_section "RSTUDIO POD DESCRIBE" "for p in \$(kubectl get pods -n '$NAMESPACE' --no-headers 2>/dev/null | awk '/^rstudio-/ {print \$1}'); do echo; echo '--- RSTUDIO POD:' \$p '---'; kubectl describe pod -n '$NAMESPACE' \$p; done"
  run_shell_section "RSTUDIO POD LOGS" "for p in \$(kubectl get pods -n '$NAMESPACE' --no-headers 2>/dev/null | awk '/^rstudio-/ {print \$1}'); do echo; echo '--- RSTUDIO LOGS:' \$p '---'; kubectl logs -n '$NAMESPACE' \$p -c rstudio --tail=200 || true; done"
  run_shell_section "INIT CONTAINER LOGS" "for p in \$(kubectl get pods -n '$NAMESPACE' --no-headers 2>/dev/null | awk '/^rstudio-/ {print \$1}'); do echo; echo '--- INIT LOGS:' \$p '---'; kubectl logs -n '$NAMESPACE' \$p -c seed-assignment --tail=200 || true; done"
  run_shell_section "SECRET SUMMARY" "for s in postgres-auth minio-auth hdu-ride-backend-env; do echo; echo '--- SECRET:' \$s '---'; kubectl get secret -n '$NAMESPACE' \$s -o jsonpath='{.data}' 2>/dev/null || true; echo; done"
else
  write_missing_command "KUBERNETES SECTIONS"
fi

if command_exists systemctl; then
  run_section "KUBELET STATUS" systemctl status kubelet --no-pager
  run_section "CONTAINERD STATUS" systemctl status containerd --no-pager
  run_section "NGINX STATUS" systemctl status nginx --no-pager
else
  write_missing_command "SYSTEMCTL STATUS"
fi

if command_exists journalctl; then
  run_section "KUBELET JOURNAL" journalctl -u kubelet -n 300 --no-pager
  run_section "CONTAINERD JOURNAL" journalctl -u containerd -n 300 --no-pager
  run_section "NGINX JOURNAL" journalctl -u nginx -n 200 --no-pager
else
  write_missing_command "JOURNALCTL"
fi

if command_exists crictl; then
  run_section "CRICTL PODS" crictl pods
  run_section "CRICTL PS" crictl ps -a
else
  write_missing_command "CRICTL"
fi

if command_exists ctr; then
  run_section "CTR IMAGES" ctr -n k8s.io images list
else
  write_missing_command "CTR"
fi

if command_exists docker; then
  run_section "DOCKER IMAGES" docker images
else
  write_missing_command "DOCKER"
fi

run_section "DISK USAGE" df -h
run_section "MEMORY" free -h
run_section "UPTIME" uptime
run_section "IP ADDR" ip addr
run_section "IP ROUTE" ip route
run_shell_section "FORWARD POLICY" "iptables -S FORWARD 2>/dev/null || true"
run_shell_section "UFW STATUS" "ufw status 2>/dev/null || true"
run_shell_section "SWAP STATUS" "swapon --show 2>/dev/null || true"
run_shell_section "SYSCTL K8S KEYS" "sysctl net.bridge.bridge-nf-call-iptables net.ipv4.ip_forward 2>/dev/null || true"
run_shell_section "CONTAINERD SANDBOX IMAGE" "grep -n 'sandbox' /etc/containerd/config.toml 2>/dev/null || true"
run_shell_section "CONTAINERD SYSTEMD CGROUP" "grep -n 'SystemdCgroup' /etc/containerd/config.toml 2>/dev/null || true"

{
  printf '\n===== END =====\n'
  printf 'Diagnostic report saved to: %s\n' "$REPORT_PATH"
} >>"$REPORT_PATH"

printf '诊断报告已生成：%s\n' "$REPORT_PATH"
