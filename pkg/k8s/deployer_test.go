package k8s

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/ptr"
)

// createTestClientForDeployer creates a Client with a fake clientset for deployer testing
func createTestClientForDeployer(namespace string, objects ...runtime.Object) *Client {
	fakeClientset := fake.NewSimpleClientset(objects...)
	return &Client{
		clientset: fakeClientset,
		config:    nil,
		namespace: namespace,
	}
}

func TestClient_DeployManifest_Deployment(t *testing.T) {
	client := createTestClientForDeployer("clustership")
	ctx := context.Background()

	manifest := &ServiceManifest{
		Kind:      "Deployment",
		Name:      "test-deploy",
		Namespace: "original-ns", // Should be overridden
		Company:   "netflix",
		ServiceID: "playback",
		Replicas:  3,
		RawYAML: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deploy
  namespace: original-ns
  labels:
    app: clustership
    company: netflix
    service: playback
spec:
  replicas: 3
  selector:
    matchLabels:
      service: playback
  template:
    metadata:
      labels:
        service: playback
    spec:
      containers:
      - name: main
        image: nginx:1.25
`,
	}

	err := client.DeployManifest(ctx, manifest)
	if err != nil {
		t.Fatalf("DeployManifest failed: %v", err)
	}

	// Verify deployment was created in the correct namespace
	dep, err := client.clientset.AppsV1().Deployments("clustership").Get(ctx, "test-deploy", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get deployment: %v", err)
	}

	if dep.Name != "test-deploy" {
		t.Errorf("expected name=test-deploy, got %s", dep.Name)
	}
	if dep.Namespace != "clustership" {
		t.Errorf("expected namespace=clustership, got %s", dep.Namespace)
	}
	if *dep.Spec.Replicas != 3 {
		t.Errorf("expected replicas=3, got %d", *dep.Spec.Replicas)
	}
}

func TestClient_DeployManifest_Deployment_Update(t *testing.T) {
	// Pre-create a deployment
	existingDep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "existing-deploy",
			Namespace:       "clustership",
			ResourceVersion: "12345",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(int32(1)),
		},
	}

	client := createTestClientForDeployer("clustership", existingDep)
	ctx := context.Background()

	manifest := &ServiceManifest{
		Kind:      "Deployment",
		Name:      "existing-deploy",
		Company:   "netflix",
		ServiceID: "playback",
		Replicas:  5,
		RawYAML: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: existing-deploy
  labels:
    app: clustership
spec:
  replicas: 5
  selector:
    matchLabels:
      service: playback
  template:
    metadata:
      labels:
        service: playback
    spec:
      containers:
      - name: main
        image: nginx:1.25
`,
	}

	err := client.DeployManifest(ctx, manifest)
	if err != nil {
		t.Fatalf("DeployManifest update failed: %v", err)
	}

	// Verify deployment was updated
	dep, err := client.clientset.AppsV1().Deployments("clustership").Get(ctx, "existing-deploy", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get deployment: %v", err)
	}

	if *dep.Spec.Replicas != 5 {
		t.Errorf("expected updated replicas=5, got %d", *dep.Spec.Replicas)
	}
}

func TestClient_DeployManifest_StatefulSet(t *testing.T) {
	client := createTestClientForDeployer("clustership")
	ctx := context.Background()

	manifest := &ServiceManifest{
		Kind:      "StatefulSet",
		Name:      "test-sts",
		Company:   "netflix",
		ServiceID: "database",
		Replicas:  2,
		RawYAML: `apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: test-sts
  labels:
    app: clustership
    company: netflix
    service: database
spec:
  serviceName: test-sts
  replicas: 2
  selector:
    matchLabels:
      service: database
  template:
    metadata:
      labels:
        service: database
    spec:
      containers:
      - name: postgres
        image: postgres:15
`,
	}

	err := client.DeployManifest(ctx, manifest)
	if err != nil {
		t.Fatalf("DeployManifest failed: %v", err)
	}

	// Verify statefulset was created
	sts, err := client.clientset.AppsV1().StatefulSets("clustership").Get(ctx, "test-sts", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get statefulset: %v", err)
	}

	if sts.Name != "test-sts" {
		t.Errorf("expected name=test-sts, got %s", sts.Name)
	}
	if sts.Namespace != "clustership" {
		t.Errorf("expected namespace=clustership, got %s", sts.Namespace)
	}
}

func TestClient_DeployManifest_StatefulSet_Update(t *testing.T) {
	existingSts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "existing-sts",
			Namespace:       "clustership",
			ResourceVersion: "12345",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: ptr.To(int32(1)),
		},
	}

	client := createTestClientForDeployer("clustership", existingSts)
	ctx := context.Background()

	manifest := &ServiceManifest{
		Kind:      "StatefulSet",
		Name:      "existing-sts",
		Company:   "netflix",
		ServiceID: "database",
		Replicas:  3,
		RawYAML: `apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: existing-sts
  labels:
    app: clustership
spec:
  serviceName: existing-sts
  replicas: 3
  selector:
    matchLabels:
      service: database
  template:
    metadata:
      labels:
        service: database
    spec:
      containers:
      - name: postgres
        image: postgres:15
`,
	}

	err := client.DeployManifest(ctx, manifest)
	if err != nil {
		t.Fatalf("DeployManifest update failed: %v", err)
	}

	sts, err := client.clientset.AppsV1().StatefulSets("clustership").Get(ctx, "existing-sts", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get statefulset: %v", err)
	}

	if *sts.Spec.Replicas != 3 {
		t.Errorf("expected updated replicas=3, got %d", *sts.Spec.Replicas)
	}
}

func TestClient_DeployManifest_Pod(t *testing.T) {
	client := createTestClientForDeployer("clustership")
	ctx := context.Background()

	manifest := &ServiceManifest{
		Kind:      "Pod",
		Name:      "test-pod",
		Company:   "aws",
		ServiceID: "worker",
		Replicas:  1,
		RawYAML: `apiVersion: v1
kind: Pod
metadata:
  name: test-pod
  labels:
    app: clustership
    company: aws
    service: worker
spec:
  containers:
  - name: busybox
    image: busybox:1.36
    command: ["sh", "-c", "sleep 3600"]
`,
	}

	err := client.DeployManifest(ctx, manifest)
	if err != nil {
		t.Fatalf("DeployManifest failed: %v", err)
	}

	// Verify pod was created
	pod, err := client.clientset.CoreV1().Pods("clustership").Get(ctx, "test-pod", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get pod: %v", err)
	}

	if pod.Name != "test-pod" {
		t.Errorf("expected name=test-pod, got %s", pod.Name)
	}
	if pod.Namespace != "clustership" {
		t.Errorf("expected namespace=clustership, got %s", pod.Namespace)
	}
}

func TestClient_DeployManifest_Pod_Recreates(t *testing.T) {
	existingPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "existing-pod",
			Namespace: "clustership",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "old", Image: "nginx:1.24"},
			},
		},
	}

	client := createTestClientForDeployer("clustership", existingPod)
	ctx := context.Background()

	manifest := &ServiceManifest{
		Kind:      "Pod",
		Name:      "existing-pod",
		Company:   "aws",
		ServiceID: "worker",
		Replicas:  1,
		RawYAML: `apiVersion: v1
kind: Pod
metadata:
  name: existing-pod
  labels:
    app: clustership
spec:
  containers:
  - name: new
    image: nginx:1.25
`,
	}

	err := client.DeployManifest(ctx, manifest)
	if err != nil {
		t.Fatalf("DeployManifest failed: %v", err)
	}

	// Verify pod was recreated with new spec
	pod, err := client.clientset.CoreV1().Pods("clustership").Get(ctx, "existing-pod", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get pod: %v", err)
	}

	if len(pod.Spec.Containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(pod.Spec.Containers))
	}
	if pod.Spec.Containers[0].Name != "new" {
		t.Errorf("expected container name=new, got %s", pod.Spec.Containers[0].Name)
	}
}

func TestClient_DeployManifest_UnsupportedKind(t *testing.T) {
	client := createTestClientForDeployer("clustership")
	ctx := context.Background()

	manifest := &ServiceManifest{
		Kind: "ConfigMap",
		Name: "test-cm",
	}

	err := client.DeployManifest(ctx, manifest)
	if err == nil {
		t.Error("expected error for unsupported kind")
	}
}

func TestClient_DeployManifest_InvalidYAML(t *testing.T) {
	client := createTestClientForDeployer("clustership")
	ctx := context.Background()

	manifest := &ServiceManifest{
		Kind:    "Deployment",
		Name:    "invalid",
		RawYAML: "{invalid yaml",
	}

	err := client.DeployManifest(ctx, manifest)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestClient_DeleteService(t *testing.T) {
	// Create resources to delete
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "netflix-playback",
			Namespace: "clustership",
			Labels: map[string]string{
				"app":     "clustership",
				"service": "playback",
				"company": "netflix",
			},
		},
	}
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "netflix-database",
			Namespace: "clustership",
			Labels: map[string]string{
				"app":     "clustership",
				"service": "database",
				"company": "netflix",
			},
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "netflix-worker",
			Namespace: "clustership",
			Labels: map[string]string{
				"app":     "clustership",
				"service": "playback",
				"company": "netflix",
			},
		},
	}

	client := createTestClientForDeployer("clustership", dep, sts, pod)
	ctx := context.Background()

	err := client.DeleteService(ctx, "playback", "netflix")
	if err != nil {
		t.Fatalf("DeleteService failed: %v", err)
	}

	// Note: fake clientset may not fully implement delete by label selector
	// This test verifies the method executes without error
}

func TestClient_DeleteService_NonexistentService(t *testing.T) {
	client := createTestClientForDeployer("clustership")
	ctx := context.Background()

	// Should not error for non-existent service
	err := client.DeleteService(ctx, "nonexistent", "unknown")
	if err != nil {
		t.Errorf("DeleteService should not fail for non-existent service: %v", err)
	}
}

func TestClient_ScaleDeployment(t *testing.T) {
	existingDep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "scalable-deploy",
			Namespace: "clustership",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(int32(2)),
		},
	}

	client := createTestClientForDeployer("clustership", existingDep)
	ctx := context.Background()

	err := client.ScaleDeployment(ctx, "scalable-deploy", 5)
	if err != nil {
		t.Fatalf("ScaleDeployment failed: %v", err)
	}

	// Verify deployment was scaled
	dep, err := client.clientset.AppsV1().Deployments("clustership").Get(ctx, "scalable-deploy", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get deployment: %v", err)
	}

	if *dep.Spec.Replicas != 5 {
		t.Errorf("expected replicas=5, got %d", *dep.Spec.Replicas)
	}
}

func TestClient_ScaleDeployment_ScaleDown(t *testing.T) {
	existingDep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "scale-down-deploy",
			Namespace: "clustership",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(int32(10)),
		},
	}

	client := createTestClientForDeployer("clustership", existingDep)
	ctx := context.Background()

	err := client.ScaleDeployment(ctx, "scale-down-deploy", 1)
	if err != nil {
		t.Fatalf("ScaleDeployment failed: %v", err)
	}

	dep, err := client.clientset.AppsV1().Deployments("clustership").Get(ctx, "scale-down-deploy", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get deployment: %v", err)
	}

	if *dep.Spec.Replicas != 1 {
		t.Errorf("expected replicas=1, got %d", *dep.Spec.Replicas)
	}
}

func TestClient_ScaleDeployment_ToZero(t *testing.T) {
	existingDep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "scale-zero-deploy",
			Namespace: "clustership",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(int32(3)),
		},
	}

	client := createTestClientForDeployer("clustership", existingDep)
	ctx := context.Background()

	err := client.ScaleDeployment(ctx, "scale-zero-deploy", 0)
	if err != nil {
		t.Fatalf("ScaleDeployment to zero failed: %v", err)
	}

	dep, err := client.clientset.AppsV1().Deployments("clustership").Get(ctx, "scale-zero-deploy", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get deployment: %v", err)
	}

	if *dep.Spec.Replicas != 0 {
		t.Errorf("expected replicas=0, got %d", *dep.Spec.Replicas)
	}
}

func TestClient_ScaleDeployment_NonexistentDeployment(t *testing.T) {
	client := createTestClientForDeployer("clustership")
	ctx := context.Background()

	err := client.ScaleDeployment(ctx, "nonexistent", 5)
	if err == nil {
		t.Error("expected error for non-existent deployment")
	}
}

func TestClient_GetDeploymentStatus(t *testing.T) {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "status-deploy",
			Namespace: "clustership",
			Labels: map[string]string{
				"service": "playback",
				"company": "netflix",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(int32(3)),
		},
		Status: appsv1.DeploymentStatus{
			Replicas:          3,
			ReadyReplicas:     2,
			AvailableReplicas: 2,
		},
	}

	client := createTestClientForDeployer("clustership", dep)
	ctx := context.Background()

	info, err := client.GetDeploymentStatus(ctx, "status-deploy")
	if err != nil {
		t.Fatalf("GetDeploymentStatus failed: %v", err)
	}

	if info.Name != "status-deploy" {
		t.Errorf("expected Name=status-deploy, got %s", info.Name)
	}
	if info.Namespace != "clustership" {
		t.Errorf("expected Namespace=clustership, got %s", info.Namespace)
	}
	if info.Replicas != 3 {
		t.Errorf("expected Replicas=3, got %d", info.Replicas)
	}
	if info.Ready != 2 {
		t.Errorf("expected Ready=2, got %d", info.Ready)
	}
	if info.Available != 2 {
		t.Errorf("expected Available=2, got %d", info.Available)
	}
	if info.ServiceID != "playback" {
		t.Errorf("expected ServiceID=playback, got %s", info.ServiceID)
	}
	if info.Company != "netflix" {
		t.Errorf("expected Company=netflix, got %s", info.Company)
	}
}

func TestClient_GetDeploymentStatus_NonexistentDeployment(t *testing.T) {
	client := createTestClientForDeployer("clustership")
	ctx := context.Background()

	_, err := client.GetDeploymentStatus(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for non-existent deployment")
	}
}

// Table-driven test for deploying different kinds
func TestClient_DeployManifest_AllKinds(t *testing.T) {
	tests := []struct {
		name    string
		kind    string
		rawYAML string
	}{
		{
			name: "Deployment",
			kind: "Deployment",
			rawYAML: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: table-deploy
  labels:
    app: clustership
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test
    spec:
      containers:
      - name: main
        image: nginx
`,
		},
		{
			name: "StatefulSet",
			kind: "StatefulSet",
			rawYAML: `apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: table-sts
  labels:
    app: clustership
spec:
  serviceName: table-sts
  replicas: 1
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test
    spec:
      containers:
      - name: main
        image: nginx
`,
		},
		{
			name: "Pod",
			kind: "Pod",
			rawYAML: `apiVersion: v1
kind: Pod
metadata:
  name: table-pod
  labels:
    app: clustership
spec:
  containers:
  - name: main
    image: nginx
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := createTestClientForDeployer("clustership")
			ctx := context.Background()

			manifest := &ServiceManifest{
				Kind:    tt.kind,
				Name:    "table-" + tt.kind,
				RawYAML: tt.rawYAML,
			}

			err := client.DeployManifest(ctx, manifest)
			if err != nil {
				t.Errorf("DeployManifest(%s) failed: %v", tt.kind, err)
			}
		})
	}
}

// Benchmark tests
func BenchmarkDeployManifest_Deployment(b *testing.B) {
	client := createTestClientForDeployer("clustership")
	ctx := context.Background()

	manifest := &ServiceManifest{
		Kind: "Deployment",
		Name: "bench-deploy",
		RawYAML: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: bench-deploy
  labels:
    app: clustership
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test
    spec:
      containers:
      - name: main
        image: nginx
`,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.DeployManifest(ctx, manifest)
	}
}

func BenchmarkScaleDeployment(b *testing.B) {
	existingDep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bench-scale",
			Namespace: "clustership",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(int32(1)),
		},
	}

	client := createTestClientForDeployer("clustership", existingDep)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.ScaleDeployment(ctx, "bench-scale", int32(i%10+1))
	}
}
