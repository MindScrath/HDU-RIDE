package app

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr              string
	DatabaseURL           string
	ContentRoot           string
	ContentPVCName        string
	S3Endpoint            string
	S3Bucket              string
	S3AccessKey           string
	S3SecretKey           string
	S3UseSSL              bool
	SessionSecret         string
	RootUsername          string
	RootPasswordHash      string
	K8sNamespace          string
	WorkspaceImageDefault string
	WorkspaceStorageClass string
	WorkspaceCPURequest   string
	WorkspaceCPULimit     string
	WorkspaceMemRequest   string
	WorkspaceMemLimit     string
	SessionTTL            time.Duration
}

func loadConfig() (Config, error) {
	useSSL, err := strconv.ParseBool(requiredEnv("S3_USE_SSL"))
	if err != nil {
		return Config{}, fmt.Errorf("S3_USE_SSL: %w", err)
	}

	cfg := Config{
		HTTPAddr:              requiredEnv("HTTP_ADDR"),
		DatabaseURL:           requiredEnv("DATABASE_URL"),
		ContentRoot:           requiredEnv("CONTENT_ROOT"),
		ContentPVCName:        requiredEnv("CONTENT_PVC_NAME"),
		S3Endpoint:            trimEndpoint(requiredEnv("S3_ENDPOINT")),
		S3Bucket:              requiredEnv("S3_BUCKET"),
		S3AccessKey:           requiredEnv("S3_ACCESS_KEY_ID"),
		S3SecretKey:           requiredEnv("S3_SECRET_ACCESS_KEY"),
		S3UseSSL:              useSSL,
		SessionSecret:         requiredEnv("SESSION_SECRET"),
		RootUsername:          requiredEnv("ROOT_USERNAME"),
		RootPasswordHash:      requiredEnv("ROOT_PASSWORD_HASH"),
		K8sNamespace:          requiredEnv("K8S_NAMESPACE"),
		WorkspaceImageDefault: requiredEnv("WORKSPACE_IMAGE_DEFAULT"),
		WorkspaceStorageClass: requiredEnv("WORKSPACE_STORAGE_CLASS"),
		WorkspaceCPURequest:   requiredEnv("WORKSPACE_CPU_REQUEST"),
		WorkspaceCPULimit:     requiredEnv("WORKSPACE_CPU_LIMIT"),
		WorkspaceMemRequest:   requiredEnv("WORKSPACE_MEM_REQUEST"),
		WorkspaceMemLimit:     requiredEnv("WORKSPACE_MEM_LIMIT"),
		SessionTTL:            12 * time.Hour,
	}
	return cfg, nil
}

func LoadConfig() (Config, error) {
	if err := loadDotEnv(); err != nil {
		return Config{}, err
	}
	return loadConfig()
}

func loadDotEnv() error {
	path, ok := findDotEnv()
	if !ok {
		return nil
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" || os.Getenv(key) != "" {
			continue
		}
		os.Setenv(key, cleanEnvValue(value))
	}
	return scanner.Err()
}

func findDotEnv() (string, bool) {
	dir, err := os.Getwd()
	if err != nil {
		return "", false
	}
	for {
		path := filepath.Join(dir, ".env")
		if _, err := os.Stat(path); err == nil {
			return path, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func cleanEnvValue(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"'`)
	return value
}

func requiredEnv(key string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		panic("missing required environment variable: " + key)
	}
	return value
}

func trimEndpoint(value string) string {
	value = strings.TrimPrefix(value, "https://")
	value = strings.TrimPrefix(value, "http://")
	return strings.TrimRight(value, "/")
}
