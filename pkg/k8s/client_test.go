package k8s

import (
	"context"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

// createTestClient creates a Client with a fake clientset for testing
func createTestClient(namespace string, objects ...runtime.Object) *Client {
	fakeClientset := fake.NewSimpleClientset(objects...)
	return &Client{
		clientset: fakeClientset,
		config:    nil,
		namespace: namespace,
	}
}

func TestClient_GetNamespace(t *testing.T) {
	client := createTestClient("test-namespace")

	ns := client.GetNamespace()
	if ns != "test-namespace" {
		t.Errorf("expected namespace=test-namespace, got %s", ns)
	}
}

func TestClient_Clientset(t *testing.T) {
	client := createTestClient("test-namespace")

	cs := client.Clientset()
	if cs == nil {
		t.Error("expected non-nil clientset")
	}
}

func TestClient_IsClusterAvailable(t *testing.T) {
	client := createTestClient("test-namespace")

	// Fake clientset should always be available
	if !client.IsClusterAvailable() {
		t.Error("expected cluster to be available with fake clientset")
	}
}

func TestClient_EnsureNamespace_Creates(t *testing.T) {
	client := createTestClient("clustership")
	ctx := context.Background()

	err := client.EnsureNamespace(ctx, "clustership")
	if err != nil {
		t.Fatalf("EnsureNamespace failed: %v", err)
	}

	// Verify namespace was created
	ns, err := client.clientset.CoreV1().Namespaces().Get(ctx, "clustership", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get namespace: %v", err)
	}

	if ns.Name != "clustership" {
		t.Errorf("expected namespace name=clustership, got %s", ns.Name)
	}
	if ns.Labels["app"] != "clustership" {
		t.Errorf("expected label app=clustership, got %s", ns.Labels["app"])
	}
	if ns.Labels["managed-by"] != "clustership" {
		t.Errorf("expected label managed-by=clustership, got %s", ns.Labels["managed-by"])
	}
}

func TestClient_EnsureNamespace_AlreadyExists(t *testing.T) {
	// Pre-create the namespace
	existingNS := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "existing-ns",
			Labels: map[string]string{
				"app": "clustership",
			},
		},
	}
	client := createTestClient("existing-ns", existingNS)
	ctx := context.Background()

	// Should not error when namespace exists
	err := client.EnsureNamespace(ctx, "existing-ns")
	if err != nil {
		t.Errorf("EnsureNamespace should not fail for existing namespace: %v", err)
	}
}

func TestClient_CleanupNamespace(t *testing.T) {
	// Create client with some resources
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deploy",
			Namespace: "clustership",
			Labels: map[string]string{
				"app": "clustership",
			},
		},
	}
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sts",
			Namespace: "clustership",
			Labels: map[string]string{
				"app": "clustership",
			},
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "clustership",
			Labels: map[string]string{
				"app": "clustership",
			},
		},
	}

	client := createTestClient("clustership", dep, sts, pod)
	ctx := context.Background()

	err := client.CleanupNamespace(ctx, "clustership")
	if err != nil {
		t.Fatalf("CleanupNamespace failed: %v", err)
	}

	// Note: fake clientset DeleteCollection doesn't actually delete,
	// so we just verify no error occurred
}

func TestClient_CleanupNamespace_EmptyNamespace(t *testing.T) {
	client := createTestClient("empty-ns")
	ctx := context.Background()

	// Should not error on empty namespace
	err := client.CleanupNamespace(ctx, "empty-ns")
	if err != nil {
		t.Errorf("CleanupNamespace should not fail for empty namespace: %v", err)
	}
}

func TestClient_GetPodStatus(t *testing.T) {
	// Create pods with different statuses
	runningPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "running-pod",
			Namespace: "clustership",
			Labels: map[string]string{
				"app":     "clustership",
				"service": "playback",
				"company": "netflix",
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				},
			},
			ContainerStatuses: []corev1.ContainerStatus{
				{
					RestartCount: 2,
				},
			},
		},
	}

	pendingPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pending-pod",
			Namespace: "clustership",
			Labels: map[string]string{
				"app":     "clustership",
				"service": "database",
				"company": "netflix",
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
		},
	}

	client := createTestClient("clustership", runningPod, pendingPod)
	ctx := context.Background()

	pods, err := client.GetPodStatus(ctx, "clustership", "app=clustership")
	if err != nil {
		t.Fatalf("GetPodStatus failed: %v", err)
	}

	if len(pods) != 2 {
		t.Fatalf("expected 2 pods, got %d", len(pods))
	}

	// Find running pod
	var foundRunning, foundPending bool
	for _, p := range pods {
		if p.Name == "running-pod" {
			foundRunning = true
			if p.Status != PodRunning {
				t.Errorf("expected running-pod status=Running, got %s", p.Status)
			}
			if !p.Ready {
				t.Error("expected running-pod to be ready")
			}
			if p.ServiceID != "playback" {
				t.Errorf("expected ServiceID=playback, got %s", p.ServiceID)
			}
			if p.Company != "netflix" {
				t.Errorf("expected Company=netflix, got %s", p.Company)
			}
			if p.Restarts != 2 {
				t.Errorf("expected Restarts=2, got %d", p.Restarts)
			}
		}
		if p.Name == "pending-pod" {
			foundPending = true
			if p.Status != PodPending {
				t.Errorf("expected pending-pod status=Pending, got %s", p.Status)
			}
		}
	}

	if !foundRunning {
		t.Error("did not find running-pod")
	}
	if !foundPending {
		t.Error("did not find pending-pod")
	}
}

func TestClient_GetPodStatus_WithLabelSelector(t *testing.T) {
	// Create pods with different labels
	netflixPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "netflix-pod",
			Namespace: "clustership",
			Labels: map[string]string{
				"app":     "clustership",
				"company": "netflix",
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}

	awsPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "aws-pod",
			Namespace: "clustership",
			Labels: map[string]string{
				"app":     "clustership",
				"company": "aws",
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}

	client := createTestClient("clustership", netflixPod, awsPod)
	ctx := context.Background()

	// Filter by company
	pods, err := client.GetPodStatus(ctx, "clustership", "app=clustership,company=netflix")
	if err != nil {
		t.Fatalf("GetPodStatus failed: %v", err)
	}

	if len(pods) != 1 {
		t.Fatalf("expected 1 pod with netflix label, got %d", len(pods))
	}
	if pods[0].Name != "netflix-pod" {
		t.Errorf("expected netflix-pod, got %s", pods[0].Name)
	}
}

func TestClient_GetPodStatus_EmptyResult(t *testing.T) {
	client := createTestClient("clustership")
	ctx := context.Background()

	pods, err := client.GetPodStatus(ctx, "clustership", "app=clustership")
	if err != nil {
		t.Fatalf("GetPodStatus failed: %v", err)
	}

	if len(pods) != 0 {
		t.Errorf("expected 0 pods, got %d", len(pods))
	}
}

func TestClient_GetPodStatus_TerminatingPod(t *testing.T) {
	now := metav1.Now()
	terminatingPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "terminating-pod",
			Namespace:         "clustership",
			DeletionTimestamp: &now,
			Labels: map[string]string{
				"app": "clustership",
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning, // Still running but terminating
		},
	}

	client := createTestClient("clustership", terminatingPod)
	ctx := context.Background()

	pods, err := client.GetPodStatus(ctx, "clustership", "app=clustership")
	if err != nil {
		t.Fatalf("GetPodStatus failed: %v", err)
	}

	if len(pods) != 1 {
		t.Fatalf("expected 1 pod, got %d", len(pods))
	}
	if pods[0].Status != PodTerminating {
		t.Errorf("expected status=Terminating, got %s", pods[0].Status)
	}
}

func TestToPodStatus(t *testing.T) {
	tests := []struct {
		phase    corev1.PodPhase
		expected PodStatus
	}{
		{corev1.PodPending, PodPending},
		{corev1.PodRunning, PodRunning},
		{corev1.PodSucceeded, PodSucceeded},
		{corev1.PodFailed, PodFailed},
		{corev1.PodUnknown, PodUnknown},
		{"InvalidPhase", PodUnknown},
	}

	for _, tt := range tests {
		t.Run(string(tt.phase), func(t *testing.T) {
			result := toPodStatus(tt.phase)
			if result != tt.expected {
				t.Errorf("toPodStatus(%s) = %s, want %s", tt.phase, result, tt.expected)
			}
		})
	}
}

func TestGetRestarts(t *testing.T) {
	tests := []struct {
		name           string
		containerStats []corev1.ContainerStatus
		expected       int
	}{
		{
			name:           "no containers",
			containerStats: nil,
			expected:       0,
		},
		{
			name: "single container no restarts",
			containerStats: []corev1.ContainerStatus{
				{RestartCount: 0},
			},
			expected: 0,
		},
		{
			name: "single container with restarts",
			containerStats: []corev1.ContainerStatus{
				{RestartCount: 5},
			},
			expected: 5,
		},
		{
			name: "multiple containers sum restarts",
			containerStats: []corev1.ContainerStatus{
				{RestartCount: 3},
				{RestartCount: 2},
				{RestartCount: 1},
			},
			expected: 6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := &corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: tt.containerStats,
				},
			}
			result := getRestarts(pod)
			if result != tt.expected {
				t.Errorf("getRestarts() = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestClient_GetPodStatus_NotReady(t *testing.T) {
	notReadyPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "not-ready-pod",
			Namespace: "clustership",
			Labels: map[string]string{
				"app": "clustership",
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionFalse,
				},
			},
		},
	}

	client := createTestClient("clustership", notReadyPod)
	ctx := context.Background()

	pods, err := client.GetPodStatus(ctx, "clustership", "app=clustership")
	if err != nil {
		t.Fatalf("GetPodStatus failed: %v", err)
	}

	if len(pods) != 1 {
		t.Fatalf("expected 1 pod, got %d", len(pods))
	}
	if pods[0].Ready {
		t.Error("expected pod to not be ready")
	}
}

func TestClient_GetPodStatus_AllPhases(t *testing.T) {
	phases := []corev1.PodPhase{
		corev1.PodPending,
		corev1.PodRunning,
		corev1.PodSucceeded,
		corev1.PodFailed,
		corev1.PodUnknown,
	}

	var objects []runtime.Object
	for i, phase := range phases {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      string(phase) + "-pod",
				Namespace: "clustership",
				Labels: map[string]string{
					"app":   "clustership",
					"phase": string(phase),
				},
			},
			Status: corev1.PodStatus{
				Phase: phases[i],
			},
		}
		objects = append(objects, pod)
	}

	client := createTestClient("clustership", objects...)
	ctx := context.Background()

	pods, err := client.GetPodStatus(ctx, "clustership", "app=clustership")
	if err != nil {
		t.Fatalf("GetPodStatus failed: %v", err)
	}

	if len(pods) != len(phases) {
		t.Fatalf("expected %d pods, got %d", len(phases), len(pods))
	}
}

func TestNewClient_WithInvalidKubeconfig(t *testing.T) {
	// Test with non-existent kubeconfig
	_, err := NewClient("/nonexistent/path/kubeconfig", "test")
	if err == nil {
		t.Error("expected error for invalid kubeconfig path")
	}
}

// Benchmark tests
func BenchmarkGetPodStatus(b *testing.B) {
	// Create 100 pods
	var objects []runtime.Object
	for i := 0; i < 100; i++ {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod-" + string(rune('0'+i%10)) + string(rune('0'+i/10)),
				Namespace: "clustership",
				Labels: map[string]string{
					"app": "clustership",
				},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
			},
		}
		objects = append(objects, pod)
	}

	client := createTestClient("clustership", objects...)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.GetPodStatus(ctx, "clustership", "app=clustership")
	}
}

// Test context cancellation
func TestClient_GetPodStatus_ContextCancellation(t *testing.T) {
	client := createTestClient("clustership")
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Context is cancelled but fake client may or may not respect it
	// The important thing is it doesn't hang
	done := make(chan struct{})
	go func() {
		_, _ = client.GetPodStatus(ctx, "clustership", "app=clustership")
		close(done)
	}()

	select {
	case <-done:
		// Success - returned quickly
	case <-time.After(5 * time.Second):
		t.Error("GetPodStatus did not return in time with cancelled context")
	}
}
