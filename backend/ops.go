package main

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"hdu-ride/backend/app"
)

func runOps(args []string) error {
	if len(args) == 0 || args[0] == "help" {
		fmt.Println("usage: hdu-ride-backend ops <db-init|db-reset|build-images|sync-content|k8s-dev-up|k8s-prod-up>")
		return nil
	}
	root, err := repoRoot()
	if err != nil {
		return err
	}
	_ = loadEnvFile(filepath.Join(root, ".env"))

	switch args[0] {
	case "db-init":
		return opsDBInit()
	case "db-reset":
		return opsDBReset()
	case "build-images":
		return opsBuildImages(root)
	case "sync-content":
		return opsSyncContent(root, envDefault("NAMESPACE", envDefault("K8S_NAMESPACE", "hdu-ride")), envDefault("CONTENT_DIR", filepath.Join(root, "content")), true)
	case "k8s-dev-up":
		return opsK8sDevUp(root)
	case "k8s-prod-up":
		return opsK8sProdUp(root)
	default:
		return fmt.Errorf("unknown ops command: %s", args[0])
	}
}

func opsDBInit() error {
	ctx := context.Background()
	cfg, err := app.LoadConfig()
	if err != nil {
		return err
	}
	db, err := app.OpenDB(ctx, cfg)
	if err != nil {
		return err
	}
	defer db.Close()
	return app.InitSchema(ctx, db, cfg)
}

func opsDBReset() error {
	ctx := context.Background()
	cfg, err := app.LoadConfig()
	if err != nil {
		return err
	}
	db, err := app.OpenDB(ctx, cfg)
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err := db.Exec(ctx, `
drop table if exists events cascade;
drop table if exists workspaces cascade;
drop table if exists grades cascade;
drop table if exists submissions cascade;
drop table if exists class_members cascade;
drop table if exists classes cascade;
drop table if exists sessions cascade;
drop table if exists users cascade;
`); err != nil {
		return err
	}
	return app.InitSchema(ctx, db, cfg)
}

func opsBuildImages(root string) error {
	tag := envDefault("TAG", "dev")
	prefix := envDefault("PREFIX", "localhost/hdu-ride")
	proxy := os.Getenv("PODMAN_MACHINE_PROXY")
	backendImage := envDefault("BACKEND_IMAGE", prefix+"/backend:"+tag)
	frontendImage := envDefault("FRONTEND_IMAGE", prefix+"/frontend:"+tag)
	rstudioImage := envDefault("RSTUDIO_IMAGE", prefix+"/rstudio:"+tag)

	if err := runCmd("", "podman", "info"); err != nil {
		return err
	}
	for _, image := range []string{
		"docker.io/library/golang:1.26-alpine",
		"docker.io/library/alpine:3.22",
		"docker.io/oven/bun:1.3",
		"docker.io/library/nginx:1.29-alpine",
		"docker.io/rocker/rstudio:4.6.0",
	} {
		if err := ensurePodmanImage(image, proxy); err != nil {
			return err
		}
	}

	buildArgs := []string{"build"}
	if proxy != "" {
		buildArgs = append(buildArgs,
			"--build-arg", "HTTP_PROXY="+proxy,
			"--build-arg", "HTTPS_PROXY="+proxy,
			"--build-arg", "http_proxy="+proxy,
			"--build-arg", "https_proxy="+proxy,
		)
	}
	if registry := os.Getenv("BUN_CONFIG_REGISTRY"); registry != "" {
		buildArgs = append(buildArgs, "--build-arg", "BUN_CONFIG_REGISTRY="+registry)
	}
	if err := runCmd("", "podman", append(append([]string{}, buildArgs...), "-f", filepath.Join(root, "deploy", "docker", "backend.Dockerfile"), "-t", backendImage, root)...); err != nil {
		return err
	}
	if err := runCmd("", "podman", append(append([]string{}, buildArgs...), "-f", filepath.Join(root, "deploy", "docker", "frontend.Dockerfile"), "-t", frontendImage, root)...); err != nil {
		return err
	}
	if err := runCmd("", "podman", append(append([]string{}, buildArgs...), "-f", filepath.Join(root, "deploy", "docker", "rstudio.Dockerfile"), "-t", rstudioImage, root)...); err != nil {
		return err
	}
	fmt.Printf("Built images:\n  %s\n  %s\n  %s\n", backendImage, frontendImage, rstudioImage)
	return nil
}

func opsK8sDevUp(root string) error {
	namespace := envDefault("NAMESPACE", envDefault("K8S_NAMESPACE", "hdu-ride"))
	clusterName := envDefault("CLUSTER_NAME", "hdu-ride")
	backendImage := envDefault("BACKEND_IMAGE", "localhost/hdu-ride/backend:dev")
	workspaceImage := envDefault("WORKSPACE_IMAGE", envDefault("WORKSPACE_IMAGE_DEFAULT", "rocker/rstudio:4.6.0"))
	proxy := os.Getenv("PODMAN_MACHINE_PROXY")
	for _, key := range []string{"POSTGRES_USER", "POSTGRES_PASSWORD", "S3_ACCESS_KEY_ID", "S3_SECRET_ACCESS_KEY", "S3_BUCKET", "SESSION_SECRET", "ROOT_USERNAME"} {
		if err := requiredEnv(key); err != nil {
			return err
		}
	}
	rootHash := os.Getenv("ROOT_PASSWORD_HASH")
	if rootHash == "" {
		if err := requiredEnv("ROOT_PASSWORD"); err != nil {
			return err
		}
		hash, err := app.HashPassword(os.Getenv("ROOT_PASSWORD"))
		if err != nil {
			return err
		}
		rootHash = hash
	}

	for _, image := range []string{
		"docker.io/library/postgres:18-alpine",
		"docker.io/minio/minio:latest",
		"docker.io/minio/mc:latest",
		"docker.io/library/alpine:3.22",
		"docker.io/library/busybox:1.36",
		workspaceImage,
		backendImage,
	} {
		if err := prepareKindImage(image, clusterName, proxy); err != nil {
			return err
		}
	}
	if err := applyFile(root, "namespace.yml"); err != nil {
		return err
	}
	if err := applySecrets(namespace, rootHash); err != nil {
		return err
	}
	if err := applyFile(root, "content-pvc.yml", "postgres.yml", "minio.yml"); err != nil {
		return err
	}
	if err := waitCorePods(namespace); err != nil {
		return err
	}
	if err := ensureMinioBucket(namespace); err != nil {
		return err
	}
	if err := opsSyncContent(root, namespace, filepath.Join(root, "content"), true); err != nil {
		return err
	}
	if err := applyFile(root, "backend.yml"); err != nil {
		return err
	}
	if err := runCmd("", "kubectl", "set", "image", "deployment/hdu-ride-backend", "-n", namespace, "backend="+backendImage); err != nil {
		return err
	}
	if err := runCmd("", "kubectl", "rollout", "status", "deployment/hdu-ride-backend", "-n", namespace, "--timeout=180s"); err != nil {
		return err
	}
	fmt.Printf("Backend root login: %s\n", os.Getenv("ROOT_USERNAME"))
	fmt.Printf("Backend service: svc/hdu-ride-backend.%s:8080\n", namespace)
	return nil
}

func opsK8sProdUp(root string) error {
	namespace := envDefault("NAMESPACE", envDefault("K8S_NAMESPACE", "hdu-ride"))
	backendImage := envDefault("BACKEND_IMAGE", "hdu-ride-backend:latest")
	frontendImage := envDefault("FRONTEND_IMAGE", "hdu-ride-frontend:latest")
	for _, key := range []string{"POSTGRES_USER", "POSTGRES_PASSWORD", "S3_ACCESS_KEY_ID", "S3_SECRET_ACCESS_KEY", "S3_BUCKET", "SESSION_SECRET", "ROOT_USERNAME", "ROOT_PASSWORD_HASH"} {
		if err := requiredEnv(key); err != nil {
			return err
		}
	}
	if err := applyFile(root, "namespace.yml"); err != nil {
		return err
	}
	if err := applySecrets(namespace, os.Getenv("ROOT_PASSWORD_HASH")); err != nil {
		return err
	}
	if err := applyFile(root, "content-pvc-prod.yml", "postgres.yml", "minio.yml"); err != nil {
		return err
	}
	if err := waitCorePods(namespace); err != nil {
		return err
	}
	if err := ensureMinioBucket(namespace); err != nil {
		return err
	}
	if err := applyFile(root, "backend.yml", "frontend.yml"); err != nil {
		return err
	}
	if err := runCmd("", "kubectl", "set", "image", "deployment/hdu-ride-backend", "-n", namespace, "backend="+backendImage); err != nil {
		return err
	}
	if err := runCmd("", "kubectl", "rollout", "status", "deployment/hdu-ride-backend", "-n", namespace, "--timeout=180s"); err != nil {
		return err
	}
	if err := runCmd("", "kubectl", "set", "image", "deployment/hdu-ride-frontend", "-n", namespace, "frontend="+frontendImage); err != nil {
		return err
	}
	if err := runCmd("", "kubectl", "rollout", "status", "deployment/hdu-ride-frontend", "-n", namespace, "--timeout=180s"); err != nil {
		return err
	}
	fmt.Println("Production deployment complete.")
	return nil
}

func opsSyncContent(root, namespace, contentDir string, applyPVC bool) error {
	if err := runCmd("", "kubectl", "config", "current-context"); err != nil {
		return err
	}
	if err := applyFile(root, "namespace.yml"); err != nil {
		return err
	}
	if applyPVC {
		if err := applyFile(root, "content-pvc.yml"); err != nil {
			return err
		}
	}
	podYAML := fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  name: hdu-ride-content-sync
  namespace: %s
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
`, namespace)
	if err := runCmdWithInput("", strings.NewReader(podYAML), "kubectl", "apply", "-f", "-"); err != nil {
		return err
	}
	if err := runCmd("", "kubectl", "wait", "-n", namespace, "--for=condition=Ready", "pod/hdu-ride-content-sync", "--timeout=120s"); err != nil {
		return err
	}
	if err := runCmd("", "kubectl", "exec", "-n", namespace, "hdu-ride-content-sync", "--", "sh", "-c", "rm -rf /content/*"); err != nil {
		return err
	}
	reader, writer := io.Pipe()
	go func() {
		err := writeTar(contentDir, writer)
		_ = writer.CloseWithError(err)
	}()
	if err := runCmdWithInput("", reader, "kubectl", "exec", "-i", "-n", namespace, "hdu-ride-content-sync", "--", "tar", "-C", "/content", "-xf", "-"); err != nil {
		return err
	}
	fmt.Printf("Synced %s to PVC hdu-ride-content in namespace %s\n", contentDir, namespace)
	return nil
}

func applySecrets(namespace, rootPasswordHash string) error {
	if err := createSecret(namespace, "postgres-auth", map[string]string{
		"username": os.Getenv("POSTGRES_USER"),
		"password": os.Getenv("POSTGRES_PASSWORD"),
	}); err != nil {
		return err
	}
	if err := createSecret(namespace, "minio-auth", map[string]string{
		"MINIO_ROOT_USER":     os.Getenv("S3_ACCESS_KEY_ID"),
		"MINIO_ROOT_PASSWORD": os.Getenv("S3_SECRET_ACCESS_KEY"),
	}); err != nil {
		return err
	}
	dbName := envDefault("POSTGRES_DB", "hdu_ride")
	databaseURL := fmt.Sprintf("postgres://%s:%s@postgres.%s.svc.cluster.local:5432/%s?sslmode=disable", os.Getenv("POSTGRES_USER"), os.Getenv("POSTGRES_PASSWORD"), namespace, dbName)
	return createSecret(namespace, "hdu-ride-backend-env", map[string]string{
		"HTTP_ADDR":               ":8080",
		"DATABASE_URL":            databaseURL,
		"CONTENT_ROOT":            "/content",
		"CONTENT_PVC_NAME":        envDefault("CONTENT_PVC_NAME", "hdu-ride-content"),
		"S3_ENDPOINT":             "minio." + namespace + ".svc.cluster.local:9000",
		"S3_BUCKET":               os.Getenv("S3_BUCKET"),
		"S3_ACCESS_KEY_ID":        os.Getenv("S3_ACCESS_KEY_ID"),
		"S3_SECRET_ACCESS_KEY":    os.Getenv("S3_SECRET_ACCESS_KEY"),
		"S3_USE_SSL":              envDefault("S3_USE_SSL", "false"),
		"SESSION_SECRET":          os.Getenv("SESSION_SECRET"),
		"ROOT_USERNAME":           os.Getenv("ROOT_USERNAME"),
		"ROOT_PASSWORD_HASH":      rootPasswordHash,
		"K8S_NAMESPACE":           namespace,
		"WORKSPACE_IMAGE_DEFAULT": envDefault("WORKSPACE_IMAGE", envDefault("WORKSPACE_IMAGE_DEFAULT", "rocker/rstudio:4.6.0")),
		"WORKSPACE_STORAGE_CLASS": envDefault("WORKSPACE_STORAGE_CLASS", "standard"),
		"WORKSPACE_CPU_REQUEST":   envDefault("WORKSPACE_CPU_REQUEST", "500m"),
		"WORKSPACE_CPU_LIMIT":     envDefault("WORKSPACE_CPU_LIMIT", "1"),
		"WORKSPACE_MEM_REQUEST":   envDefault("WORKSPACE_MEM_REQUEST", "1Gi"),
		"WORKSPACE_MEM_LIMIT":     envDefault("WORKSPACE_MEM_LIMIT", "2Gi"),
	})
}

func createSecret(namespace, name string, values map[string]string) error {
	args := []string{"create", "secret", "generic", name, "-n", namespace}
	for key, value := range values {
		args = append(args, "--from-literal="+key+"="+value)
	}
	args = append(args, "--dry-run=client", "-o", "yaml")
	var out strings.Builder
	if err := runCmdCapture("", &out, "kubectl", args...); err != nil {
		return err
	}
	return runCmdWithInput("", strings.NewReader(out.String()), "kubectl", "apply", "-f", "-")
}

func applyFile(root string, names ...string) error {
	for _, name := range names {
		if err := runCmd("", "kubectl", "apply", "-f", filepath.Join(root, "deploy", "k8s", name)); err != nil {
			return err
		}
	}
	return nil
}

func waitCorePods(namespace string) error {
	if err := runCmd("", "kubectl", "wait", "-n", namespace, "--for=condition=Ready", "pod", "-l", "app=postgres", "--timeout=180s"); err != nil {
		return err
	}
	return runCmd("", "kubectl", "wait", "-n", namespace, "--for=condition=Ready", "pod", "-l", "app=minio", "--timeout=180s")
}

func ensureMinioBucket(namespace string) error {
	return runCmd("", "kubectl", "run", "minio-mc", "-n", namespace, "--rm", "-i", "--restart=Never", "--image=minio/mc", "--image-pull-policy=IfNotPresent",
		"--env=MINIO_ROOT_USER="+os.Getenv("S3_ACCESS_KEY_ID"),
		"--env=MINIO_ROOT_PASSWORD="+os.Getenv("S3_SECRET_ACCESS_KEY"),
		"--command", "--", "sh", "-c", "mc alias set local http://minio:9000 $MINIO_ROOT_USER $MINIO_ROOT_PASSWORD && mc mb -p local/"+os.Getenv("S3_BUCKET")+" || true")
}

func prepareKindImage(image, clusterName, proxy string) error {
	if err := ensurePodmanImage(image, proxy); err != nil {
		return err
	}
	safe := strings.NewReplacer("/", "-", ":", "-").Replace(image)
	archive := filepath.Join(os.TempDir(), safe+".tar")
	if err := runCmd("", "podman", "save", "-o", archive, image); err != nil {
		return err
	}
	defer os.Remove(archive)
	return runCmd("", "kind", "load", "image-archive", "--name", clusterName, archive)
}

func ensurePodmanImage(image, proxy string) error {
	if err := runCmd("", "podman", "image", "exists", image); err == nil {
		return nil
	}
	if proxy != "" {
		return runCmd("", "podman", "machine", "ssh", "export HTTP_PROXY="+proxy+" HTTPS_PROXY="+proxy+" http_proxy="+proxy+" https_proxy="+proxy+"; podman pull "+shellQuote(image))
	}
	return runCmd("", "podman", "pull", image)
}

func writeTar(root string, out io.Writer) error {
	writer := tar.NewWriter(out)
	defer writer.Close()
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		name, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		name = filepath.ToSlash(name)
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = name
		if err := writer.WriteHeader(header); err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(writer, file)
		return err
	})
}

func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "NewDemand.md")); err == nil {
			return dir, nil
		}
		if filepath.Base(dir) == "backend" {
			parent := filepath.Dir(dir)
			if _, err := os.Stat(filepath.Join(parent, "NewDemand.md")); err == nil {
				return parent, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("repository root not found")
		}
		dir = parent
	}
}

func loadEnvFile(path string) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) == "" || os.Getenv(strings.TrimSpace(key)) != "" {
			continue
		}
		os.Setenv(strings.TrimSpace(key), strings.Trim(strings.TrimSpace(value), `"'`))
	}
	return nil
}

func requiredEnv(key string) error {
	if strings.TrimSpace(os.Getenv(key)) == "" {
		return fmt.Errorf("missing required environment variable: %s", key)
	}
	return nil
}

func envDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func runCmd(dir, name string, args ...string) error {
	return runCmdWithInput(dir, nil, name, args...)
}

func runCmdWithInput(dir string, input io.Reader, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdin = input
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	return cmd.Run()
}

func runCmdCapture(dir string, out io.Writer, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = out
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	return cmd.Run()
}

func shellQuote(value string) string {
	if runtime.GOOS == "windows" {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}
