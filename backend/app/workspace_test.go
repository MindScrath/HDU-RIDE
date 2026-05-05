package app

import (
	"context"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestWorkspaceManagerCreateObjects(t *testing.T) {
	cfg := Config{
		K8sNamespace:          "hdu-ride",
		ContentPVCName:        "hdu-ride-content",
		WorkspaceStorageClass: "standard",
		WorkspaceCPURequest:   "500m",
		WorkspaceCPULimit:     "1",
		WorkspaceMemRequest:   "1Gi",
		WorkspaceMemLimit:     "2Gi",
	}
	manager := &WorkspaceManager{client: fake.NewSimpleClientset(), cfg: cfg}

	objects, err := manager.Create(context.Background(), "ws-abc", "user-1", "intro-r", "hw01", "rocker/rstudio:4.6.0")
	if err != nil {
		t.Fatal(err)
	}
	if objects.PodName != "rstudio-ws-abc" || objects.ServiceName != "rstudio-ws-abc" || objects.PVCName != "home-ws-abc" {
		t.Fatalf("unexpected object names: %+v", objects)
	}

	pod, err := manager.client.CoreV1().Pods(cfg.K8sNamespace).Get(context.Background(), objects.PodName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(pod.Spec.InitContainers) != 1 || !strings.Contains(strings.Join(pod.Spec.InitContainers[0].Command, " "), "tests/public") {
		t.Fatalf("assignment seed init container missing public files: %+v", pod.Spec.InitContainers)
	}
	if strings.Contains(strings.Join(pod.Spec.InitContainers[0].Command, " "), "tests/hidden") {
		t.Fatal("hidden tests must not be copied into workspace")
	}
	if got := pod.Spec.Containers[0].Env[0].Name + "=" + pod.Spec.Containers[0].Env[0].Value; got != "DISABLE_AUTH=true" {
		t.Fatalf("unexpected auth env: %s", got)
	}

	if _, err := manager.client.CoreV1().Services(cfg.K8sNamespace).Get(context.Background(), objects.ServiceName, metav1.GetOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.client.CoreV1().PersistentVolumeClaims(cfg.K8sNamespace).Get(context.Background(), objects.PVCName, metav1.GetOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.client.NetworkingV1().NetworkPolicies(cfg.K8sNamespace).Get(context.Background(), objects.ServiceName, metav1.GetOptions{}); err != nil {
		t.Fatal(err)
	}
}
