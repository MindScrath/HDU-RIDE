#!/usr/bin/env sh
set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
NAMESPACE="${NAMESPACE:-hdu-ride}"
CONTENT_DIR="${CONTENT_DIR:-$ROOT/content}"

kubectl config current-context >/dev/null
kubectl apply -f "$ROOT/deploy/k8s/namespace.yml"
kubectl apply -f "$ROOT/deploy/k8s/content-pvc.yml"

cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: hdu-ride-content-sync
  namespace: $NAMESPACE
spec:
  restartPolicy: Never
  containers:
    - name: sync
      image: alpine:3.22
      command: ["sh", "-c", "trap : TERM INT; sleep infinity & wait"]
      volumeMounts:
        - name: content
          mountPath: /content
  volumes:
    - name: content
      persistentVolumeClaim:
        claimName: hdu-ride-content
EOF

kubectl wait -n "$NAMESPACE" --for=condition=Ready pod/hdu-ride-content-sync --timeout=120s
MSYS_NO_PATHCONV=1 kubectl exec -n "$NAMESPACE" hdu-ride-content-sync -- sh -c "rm -rf /content/*"
tar -C "$CONTENT_DIR" -cf - . | MSYS_NO_PATHCONV=1 kubectl exec -i -n "$NAMESPACE" hdu-ride-content-sync -- tar -C /content -xf -
printf 'Synced %s to PVC hdu-ride-content in namespace %s\n' "$CONTENT_DIR" "$NAMESPACE"
