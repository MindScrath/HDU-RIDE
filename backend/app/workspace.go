package app

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
)

type WorkspaceManager struct {
	client  kubernetes.Interface
	restCfg *rest.Config
	cfg     Config
}

type WorkspaceObjects struct {
	PodName     string
	ServiceName string
	PVCName     string
}

func newWorkspaceManager(cfg Config) (*WorkspaceManager, error) {
	restCfg, err := rest.InClusterConfig()
	if err != nil {
		kubeconfig := strings.TrimSpace(os.Getenv("KUBECONFIG"))
		if kubeconfig == "" {
			return nil, fmt.Errorf("KUBECONFIG is required outside a cluster: %w", err)
		}
		restCfg, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, err
		}
	}
	client, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, err
	}
	return &WorkspaceManager{client: client, restCfg: restCfg, cfg: cfg}, nil
}

func NewWorkspaceManager(cfg Config) (*WorkspaceManager, error) {
	return newWorkspaceManager(cfg)
}

func (m *WorkspaceManager) Create(ctx context.Context, id, userID, courseID, assignmentID, image string) (WorkspaceObjects, error) {
	name := "rstudio-" + safeName(id)
	pvcName := "home-" + safeName(id)
	labels := map[string]string{
		"app.kubernetes.io/name":       "hdu-ride-rstudio",
		"app.kubernetes.io/managed-by": "hdu-ride",
		"hdu-ride/workspace-id":        id,
		"hdu-ride/user-id":             safeLabel(userID),
	}

	if _, err := m.client.CoreV1().PersistentVolumeClaims(m.cfg.K8sNamespace).Create(ctx, m.pvc(pvcName, labels), metav1.CreateOptions{}); err != nil {
		return WorkspaceObjects{}, err
	}
	if _, err := m.client.CoreV1().Pods(m.cfg.K8sNamespace).Create(ctx, m.pod(name, pvcName, labels, courseID, assignmentID, image), metav1.CreateOptions{}); err != nil {
		return WorkspaceObjects{}, err
	}
	if _, err := m.client.CoreV1().Services(m.cfg.K8sNamespace).Create(ctx, m.service(name, labels), metav1.CreateOptions{}); err != nil {
		return WorkspaceObjects{}, err
	}
	if _, err := m.client.NetworkingV1().NetworkPolicies(m.cfg.K8sNamespace).Create(ctx, m.networkPolicy(name, labels), metav1.CreateOptions{}); err != nil {
		return WorkspaceObjects{}, err
	}
	return WorkspaceObjects{PodName: name, ServiceName: name, PVCName: pvcName}, nil
}

func (m *WorkspaceManager) Stop(ctx context.Context, objects WorkspaceObjects) {
	policy := metav1.DeletePropagationBackground
	_ = m.client.NetworkingV1().NetworkPolicies(m.cfg.K8sNamespace).Delete(ctx, objects.ServiceName, metav1.DeleteOptions{PropagationPolicy: &policy})
	_ = m.client.CoreV1().Services(m.cfg.K8sNamespace).Delete(ctx, objects.ServiceName, metav1.DeleteOptions{PropagationPolicy: &policy})
	_ = m.client.CoreV1().Pods(m.cfg.K8sNamespace).Delete(ctx, objects.PodName, metav1.DeleteOptions{PropagationPolicy: &policy})
	_ = m.client.CoreV1().PersistentVolumeClaims(m.cfg.K8sNamespace).Delete(ctx, objects.PVCName, metav1.DeleteOptions{PropagationPolicy: &policy})
}

func (m *WorkspaceManager) Exists(ctx context.Context, objects WorkspaceObjects) bool {
	if _, err := m.client.CoreV1().Pods(m.cfg.K8sNamespace).Get(ctx, objects.PodName, metav1.GetOptions{}); err != nil {
		return false
	}
	if _, err := m.client.CoreV1().Services(m.cfg.K8sNamespace).Get(ctx, objects.ServiceName, metav1.GetOptions{}); err != nil {
		return false
	}
	return true
}

func (m *WorkspaceManager) Archive(ctx context.Context, objects WorkspaceObjects, assignmentID string, out io.Writer) error {
	if m.restCfg == nil {
		return fmt.Errorf("kubernetes rest config is required")
	}
	req := m.client.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(m.cfg.K8sNamespace).
		Name(objects.PodName).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: "rstudio",
			Command:   []string{"sh", "-c", "tar -C /home/rstudio -czf - --exclude='./.cache' --exclude='./.local' --exclude='./.config' --exclude='./.rstudio' ."},
			Stdout:    true,
			Stderr:    true,
		}, scheme.ParameterCodec)
	exec, err := remotecommand.NewSPDYExecutor(m.restCfg, "POST", req.URL())
	if err != nil {
		return err
	}
	var stderr bytes.Buffer
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{Stdout: out, Stderr: &stderr})
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func (m *WorkspaceManager) Restore(ctx context.Context, objects WorkspaceObjects, assignmentID string, in io.Reader) error {
	if m.restCfg == nil {
		return fmt.Errorf("kubernetes rest config is required")
	}
	target := shellQuote("/home/rstudio")
	command := fmt.Sprintf("set -eu; target=%s; find \"$target\" -mindepth 1 -maxdepth 1 ! -name '.rstudio' ! -name '.local' ! -name '.cache' ! -name '.config' -exec rm -rf {} +; tar -xzf - -C \"$target\"; chown -R 1000:1000 \"$target\"", target)
	req := m.client.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(m.cfg.K8sNamespace).
		Name(objects.PodName).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: "rstudio",
			Command:   []string{"sh", "-c", command},
			Stdin:     true,
			Stderr:    true,
		}, scheme.ParameterCodec)
	exec, err := remotecommand.NewSPDYExecutor(m.restCfg, "POST", req.URL())
	if err != nil {
		return err
	}
	var stderr bytes.Buffer
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{Stdin: in, Stderr: &stderr})
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func (m *WorkspaceManager) WaitReady(ctx context.Context, objects WorkspaceObjects, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		pod, err := m.client.CoreV1().Pods(m.cfg.K8sNamespace).Get(ctx, objects.PodName, metav1.GetOptions{})
		if err == nil && podReady(pod) {
			return nil
		}
		if time.Now().After(deadline) {
			if err != nil {
				return err
			}
			return fmt.Errorf("workspace pod not ready: %s", objects.PodName)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func (m *WorkspaceManager) pvc(name string, labels map[string]string) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("5Gi")},
			},
			StorageClassName: &m.cfg.WorkspaceStorageClass,
		},
	}
}

func (m *WorkspaceManager) pod(name, pvcName string, labels map[string]string, courseID, assignmentID, image string) *corev1.Pod {
	falseValue := false
	copyScript := fmt.Sprintf(`
set -eu
target="/home/rstudio/workspace/%s"
mkdir -p "$target"
cp "/content/courses/%s/assignments/%s/README.md" "$target/README.md"
for d in starter data/public tests/public; do
  if [ -d "/content/courses/%s/assignments/%s/$d" ]; then
    mkdir -p "$target/$d"
    cp -R "/content/courses/%s/assignments/%s/$d/." "$target/$d/"
  fi
done
chown -R 1000:1000 /home/rstudio
`, assignmentID, courseID, assignmentID, courseID, assignmentID, courseID, assignmentID)

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels},
		Spec: corev1.PodSpec{
			AutomountServiceAccountToken: &falseValue,
			RestartPolicy:                corev1.RestartPolicyAlways,
			Volumes: []corev1.Volume{
				{Name: "home", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvcName}}},
				{Name: "course-content", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: m.cfg.ContentPVCName, ReadOnly: true}}},
			},
			InitContainers: []corev1.Container{{
				Name:    "seed-assignment",
				Image:   "busybox:1.36",
				Command: []string{"sh", "-c", copyScript},
				VolumeMounts: []corev1.VolumeMount{
					{Name: "home", MountPath: "/home/rstudio"},
					{Name: "course-content", MountPath: "/content", ReadOnly: true},
				},
			}},
			Containers: []corev1.Container{{
				Name:  "rstudio",
				Image: image,
				Env: []corev1.EnvVar{
					{Name: "DISABLE_AUTH", Value: "true"},
					{Name: "USERID", Value: "1000"},
					{Name: "GROUPID", Value: "1000"},
					{Name: "RUNROOTLESS", Value: "false"},
				},
				Ports: []corev1.ContainerPort{{Name: "http", ContainerPort: 8787}},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse(m.cfg.WorkspaceCPURequest),
						corev1.ResourceMemory: resource.MustParse(m.cfg.WorkspaceMemRequest),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse(m.cfg.WorkspaceCPULimit),
						corev1.ResourceMemory: resource.MustParse(m.cfg.WorkspaceMemLimit),
					},
				},
				VolumeMounts: []corev1.VolumeMount{
					{Name: "home", MountPath: "/home/rstudio"},
				},
			}},
		},
	}
}

func (m *WorkspaceManager) service(name string, labels map[string]string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: map[string]string{"hdu-ride/workspace-id": labels["hdu-ride/workspace-id"]},
			Ports: []corev1.ServicePort{{
				Name:       "http",
				Port:       8787,
				TargetPort: intstr.FromString("http"),
			}},
		},
	}
}

func (m *WorkspaceManager) networkPolicy(name string, labels map[string]string) *networkingv1.NetworkPolicy {
	port := intstr.FromInt32(8787)
	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{"hdu-ride/workspace-id": labels["hdu-ride/workspace-id"]}},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{{
				From: []networkingv1.NetworkPolicyPeer{{
					PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app.kubernetes.io/name": "hdu-ride-backend"}},
				}},
				Ports: []networkingv1.NetworkPolicyPort{{Port: &port}},
			}},
		},
	}
}

func safeName(value string) string {
	value = strings.ToLower(value)
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

func safeLabel(value string) string {
	out := safeName(value)
	if len(out) > 63 {
		return out[:63]
	}
	return out
}

func podReady(pod *corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}
