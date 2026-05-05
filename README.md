# HDU RIDE

HDU RIDE is a teaching platform for quantitative finance and R coursework. It combines course content, assignments, grading, class management, and per-student RStudio Server workspaces.

## Stack

- Backend: Go + Gin + PostgreSQL, with business code under `backend/app/`
- Frontend: Vue 3 + Vite 8 + Element Plus in `frontend/`, managed with Bun
- Course content: file-based Markdown/YAML under `content/courses`
- Storage: S3-compatible object storage, such as MinIO
- RStudio: Rocker/RStudio 4.6, one Kubernetes Pod/PVC/Service per user assignment workspace, accessed through `/ide/s/:workspaceID/`

## Required Backend Environment

The backend reads `.env` from the repository root when present, while real environment variables still take precedence. It fails at startup when required configuration is missing.

```text
Copy-Item .env.example .env
cd backend
go run . hash-password root123456
```

For local `go run .`, write the generated bcrypt value to `ROOT_PASSWORD_HASH`. For `scripts/k8s-dev-up.sh`, either provide `ROOT_PASSWORD_HASH` or set `ROOT_PASSWORD`; the script hashes `ROOT_PASSWORD` before creating the Kubernetes secret.

Outside the cluster, set `KUBECONFIG` so `client-go` can create workspace resources.

## Development

Use the proxy ports you provided when downloading dependencies:

```powershell
$env:HTTP_PROXY="http://127.0.0.1:9098"
$env:HTTPS_PROXY="http://127.0.0.1:9098"
```

Backend:

```powershell
cd backend
go test ./...
go run .
```

Frontend:

```powershell
cd frontend
bun install
bun run dev
```

The Vite dev server proxies `/api` and `/ide` to `http://127.0.0.1:8080`.

## Real Local Runtime

There is no runtime fake/mock backend. Workspace creation uses `client-go` and real Kubernetes objects. Unit tests use a fake Kubernetes client only in `backend/app/workspace_test.go`.

If Podman is installed but stopped:

```powershell
podman machine init
podman machine start
```

If `kubectl config current-context` is empty, configure or start a real Kubernetes cluster before workspace features can run.

Build images with Podman:

```sh
TAG=dev \
PREFIX=localhost/hdu-ride \
PODMAN_MACHINE_PROXY=http://172.23.128.1:9098 \
sh scripts/podman-build-images.sh
```

For kind with Podman on this Windows/WSL setup, registry pulls must run inside the Podman VM and use the VM gateway address for the local proxy. Deploy real Postgres, MinIO, backend RBAC, content PVC, bucket setup, content sync, and preload workspace images:

```sh
BACKEND_IMAGE=localhost/hdu-ride/backend:dev \
PODMAN_MACHINE_PROXY=http://172.23.128.1:9098 \
PORT_FORWARD=1 \
sh scripts/k8s-dev-up.sh
```

Start the frontend in another terminal:

```powershell
cd frontend
bun run dev
```

Open `http://127.0.0.1:5173` and log in with `root / root123456` for the dev stack.

## Course Package

Teachers maintain content as a zip with this structure:

```text
course.yml
chapters/
assignments/
```

`course.yml` lists lecture chapters and unchaptered assignments separately. Hidden tests stay under `tests/hidden` and are not copied into RStudio workspaces.
