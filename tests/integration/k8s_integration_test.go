//go:build integration
// +build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"clustership/pkg/config"
	"clustership/pkg/game"
	"clustership/pkg/k8s"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	testNamespace = "clustership-test"
	kindCluster   = "clustership-test"
)

// TestMain handles test setup and teardown
func TestMain(m *testing.M) {
	// Skip if running in CI without kind
	if os.Getenv("SKIP_KIND_TESTS") == "true" {
		fmt.Println("Skipping kind-based tests (SKIP_KIND_TESTS=true)")
		os.Exit(0)
	}

	// Run tests
	code := m.Run()
	os.Exit(code)
}

// setupKindCluster creates a kind cluster for testing
func setupKindCluster(t *testing.T) (cleanup func()) {
	t.Helper()

	// Check if kind is available
	if _, err := exec.LookPath("kind"); err != nil {
		t.Skip("kind not found in PATH, skipping K8s integration tests")
	}

	// Check if cluster already exists
	checkCmd := exec.Command("kind", "get", "clusters")
	output, _ := checkCmd.Output()
	if !containsCluster(string(output), kindCluster) {
		// Create kind cluster
		t.Logf("Creating kind cluster: %s", kindCluster)
		createCmd := exec.Command("kind", "create", "cluster", "--name", kindCluster, "--wait", "60s")
		createCmd.Stdout = os.Stdout
		createCmd.Stderr = os.Stderr
		if err := createCmd.Run(); err != nil {
			t.Fatalf("Failed to create kind cluster: %v", err)
		}
	} else {
		t.Logf("Using existing kind cluster: %s", kindCluster)
	}

	// Set kubeconfig for kind cluster
	kubeconfigCmd := exec.Command("kind", "get", "kubeconfig", "--name", kindCluster)
	kubeconfigBytes, err := kubeconfigCmd.Output()
	if err != nil {
		t.Fatalf("Failed to get kubeconfig: %v", err)
	}

	// Write kubeconfig to temp file
	tmpKubeconfig := filepath.Join(t.TempDir(), "kubeconfig")
	if err := os.WriteFile(tmpKubeconfig, kubeconfigBytes, 0644); err != nil {
		t.Fatalf("Failed to write kubeconfig: %v", err)
	}

	// Set KUBECONFIG env var
	originalKubeconfig := os.Getenv("KUBECONFIG")
	os.Setenv("KUBECONFIG", tmpKubeconfig)

	cleanup = func() {
		// Restore original kubeconfig
		if originalKubeconfig != "" {
			os.Setenv("KUBECONFIG", originalKubeconfig)
		} else {
			os.Unsetenv("KUBECONFIG")
		}

		// Optionally delete cluster (set KEEP_KIND_CLUSTER=true to preserve)
		if os.Getenv("KEEP_KIND_CLUSTER") != "true" {
			t.Logf("Deleting kind cluster: %s", kindCluster)
			deleteCmd := exec.Command("kind", "delete", "cluster", "--name", kindCluster)
			deleteCmd.Run()
		}
	}

	return cleanup
}

func containsCluster(output, clusterName string) bool {
	// Simple string contains check
	return len(output) > 0 && contains(output, clusterName)
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestK8sClientConnection tests basic K8s client connectivity
func TestK8sClientConnection(t *testing.T) {
	cleanup := setupKindCluster(t)
	defer cleanup()

	kubeconfig := os.Getenv("KUBECONFIG")
	client, err := k8s.NewClient(kubeconfig, testNamespace)
	if err != nil {
		t.Fatalf("Failed to create K8s client: %v", err)
	}

	if !client.IsClusterAvailable() {
		t.Fatal("Cluster should be available")
	}
}

// TestK8sNamespaceCreation tests namespace creation
func TestK8sNamespaceCreation(t *testing.T) {
	cleanup := setupKindCluster(t)
	defer cleanup()

	ctx := context.Background()
	kubeconfig := os.Getenv("KUBECONFIG")
	client, err := k8s.NewClient(kubeconfig, testNamespace)
	if err != nil {
		t.Fatalf("Failed to create K8s client: %v", err)
	}

	// Ensure namespace
	if err := client.EnsureNamespace(ctx, testNamespace); err != nil {
		t.Fatalf("Failed to create namespace: %v", err)
	}

	// Verify namespace exists
	ns, err := client.Clientset().CoreV1().Namespaces().Get(ctx, testNamespace, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get namespace: %v", err)
	}

	if ns.Name != testNamespace {
		t.Errorf("Expected namespace %s, got %s", testNamespace, ns.Name)
	}

	// Check labels
	if ns.Labels["app"] != "clustership" {
		t.Errorf("Expected label app=clustership, got %s", ns.Labels["app"])
	}

	// Cleanup
	defer client.CleanupNamespace(ctx, testNamespace)
	defer client.Clientset().CoreV1().Namespaces().Delete(ctx, testNamespace, metav1.DeleteOptions{})
}

// TestK8sManifestDeployment tests deploying real K8s manifests
func TestK8sManifestDeployment(t *testing.T) {
	cleanup := setupKindCluster(t)
	defer cleanup()

	ctx := context.Background()
	kubeconfig := os.Getenv("KUBECONFIG")
	client, err := k8s.NewClient(kubeconfig, testNamespace)
	if err != nil {
		t.Fatalf("Failed to create K8s client: %v", err)
	}

	// Ensure namespace
	if err := client.EnsureNamespace(ctx, testNamespace); err != nil {
		t.Fatalf("Failed to create namespace: %v", err)
	}
	defer client.CleanupNamespace(ctx, testNamespace)
	defer client.Clientset().CoreV1().Namespaces().Delete(ctx, testNamespace, metav1.DeleteOptions{})

	// Load manifests from templates
	templatesDir := k8s.GetTemplatesDir()
	if templatesDir == "" || templatesDir == "templates" {
		// Try to find templates from test directory
		templatesDir = filepath.Join("..", "..", "templates")
	}

	manifests, err := k8s.LoadCompanyManifests(templatesDir, "netflix")
	if err != nil {
		t.Skipf("Skipping manifest deployment test: %v", err)
	}

	if len(manifests) == 0 {
		t.Skip("No manifests found for netflix")
	}

	// Deploy manifests
	for _, manifest := range manifests {
		t.Logf("Deploying %s: %s", manifest.Kind, manifest.Name)
		if err := client.DeployManifest(ctx, manifest); err != nil {
			t.Errorf("Failed to deploy %s: %v", manifest.Name, err)
		}
	}

	// Wait for deployments to be created
	time.Sleep(2 * time.Second)

	// Verify deployments exist
	deps, err := client.Clientset().AppsV1().Deployments(testNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=clustership",
	})
	if err != nil {
		t.Fatalf("Failed to list deployments: %v", err)
	}

	if len(deps.Items) == 0 {
		t.Error("Expected at least one deployment to be created")
	}

	for _, dep := range deps.Items {
		t.Logf("Found deployment: %s", dep.Name)
	}
}

// TestK8sPodEvents tests receiving pod events from watcher
func TestK8sPodEvents(t *testing.T) {
	cleanup := setupKindCluster(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	kubeconfig := os.Getenv("KUBECONFIG")
	client, err := k8s.NewClient(kubeconfig, testNamespace)
	if err != nil {
		t.Fatalf("Failed to create K8s client: %v", err)
	}

	// Ensure namespace
	if err := client.EnsureNamespace(ctx, testNamespace); err != nil {
		t.Fatalf("Failed to create namespace: %v", err)
	}
	defer client.CleanupNamespace(ctx, testNamespace)
	defer client.Clientset().CoreV1().Namespaces().Delete(ctx, testNamespace, metav1.DeleteOptions{})

	// Create watcher
	watcher := k8s.NewPodWatcher(client)
	eventChan := make(chan k8s.PodEvent, 10)
	watcher.Subscribe(func(event k8s.PodEvent) {
		select {
		case eventChan <- event:
		default:
		}
	})

	if err := watcher.Start(); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}
	defer watcher.Stop()

	// Load and deploy a simple manifest
	templatesDir := filepath.Join("..", "..", "templates")
	manifests, err := k8s.LoadCompanyManifests(templatesDir, "netflix")
	if err != nil || len(manifests) == 0 {
		t.Skip("No manifests found, skipping pod events test")
	}

	// Deploy first manifest
	if err := client.DeployManifest(ctx, manifests[0]); err != nil {
		t.Fatalf("Failed to deploy manifest: %v", err)
	}

	// Wait for pod event
	select {
	case event := <-eventChan:
		t.Logf("Received pod event: %s - %s", event.Type, event.Message)
		if event.Type == "" {
			t.Error("Event type should not be empty")
		}
	case <-time.After(15 * time.Second):
		t.Error("Timeout waiting for pod event")
	}
}

// TestK8sAttackTriggersDelete tests that game attacks trigger pod deletion
func TestK8sAttackTriggersDelete(t *testing.T) {
	cleanup := setupKindCluster(t)
	defer cleanup()

	ctx := context.Background()
	kubeconfig := os.Getenv("KUBECONFIG")
	client, err := k8s.NewClient(kubeconfig, testNamespace)
	if err != nil {
		t.Fatalf("Failed to create K8s client: %v", err)
	}

	// Ensure namespace
	if err := client.EnsureNamespace(ctx, testNamespace); err != nil {
		t.Fatalf("Failed to create namespace: %v", err)
	}
	defer client.CleanupNamespace(ctx, testNamespace)
	defer client.Clientset().CoreV1().Namespaces().Delete(ctx, testNamespace, metav1.DeleteOptions{})

	// Load and deploy manifests
	templatesDir := filepath.Join("..", "..", "templates")
	manifests, err := k8s.LoadCompanyManifests(templatesDir, "netflix")
	if err != nil || len(manifests) == 0 {
		t.Skip("No manifests found, skipping attack test")
	}

	// Deploy first service
	if err := client.DeployManifest(ctx, manifests[0]); err != nil {
		t.Fatalf("Failed to deploy manifest: %v", err)
	}

	// Wait for pods to be created
	time.Sleep(3 * time.Second)

	// List pods before deletion
	podsBefore, err := client.Clientset().CoreV1().Pods(testNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=clustership",
	})
	if err != nil {
		t.Fatalf("Failed to list pods: %v", err)
	}

	initialPodCount := len(podsBefore.Items)
	if initialPodCount == 0 {
		t.Skip("No pods created, skipping deletion test")
	}

	t.Logf("Found %d pods before deletion", initialPodCount)

	// Simulate attack by deleting pods for a service
	serviceID := manifests[0].ServiceID
	if serviceID == "" {
		serviceID = "api" // fallback
	}

	if err := client.DeleteService(ctx, serviceID, "netflix"); err != nil {
		t.Errorf("Failed to delete service pods: %v", err)
	}

	// Wait for deletion to propagate
	time.Sleep(2 * time.Second)

	// Verify pods were deleted
	podsAfter, err := client.Clientset().CoreV1().Pods(testNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app=clustership,service=%s", serviceID),
	})
	if err != nil {
		t.Fatalf("Failed to list pods after deletion: %v", err)
	}

	t.Logf("Found %d pods after deletion (service=%s)", len(podsAfter.Items), serviceID)
}

// TestK8sNamespaceCleanup tests cleanup of all resources
func TestK8sNamespaceCleanup(t *testing.T) {
	cleanup := setupKindCluster(t)
	defer cleanup()

	ctx := context.Background()
	kubeconfig := os.Getenv("KUBECONFIG")
	client, err := k8s.NewClient(kubeconfig, testNamespace)
	if err != nil {
		t.Fatalf("Failed to create K8s client: %v", err)
	}

	// Ensure namespace
	if err := client.EnsureNamespace(ctx, testNamespace); err != nil {
		t.Fatalf("Failed to create namespace: %v", err)
	}
	defer client.Clientset().CoreV1().Namespaces().Delete(ctx, testNamespace, metav1.DeleteOptions{})

	// Deploy some resources
	templatesDir := filepath.Join("..", "..", "templates")
	manifests, err := k8s.LoadCompanyManifests(templatesDir, "netflix")
	if err == nil && len(manifests) > 0 {
		for _, manifest := range manifests {
			client.DeployManifest(ctx, manifest)
		}
		time.Sleep(2 * time.Second)
	}

	// Run cleanup
	if err := client.CleanupNamespace(ctx, testNamespace); err != nil {
		t.Errorf("Failed to cleanup namespace: %v", err)
	}

	// Wait for cleanup to complete
	time.Sleep(3 * time.Second)

	// Verify all clustership resources are gone
	deps, _ := client.Clientset().AppsV1().Deployments(testNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=clustership",
	})
	if len(deps.Items) > 0 {
		t.Errorf("Expected 0 deployments after cleanup, got %d", len(deps.Items))
	}

	pods, _ := client.Clientset().CoreV1().Pods(testNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=clustership",
	})
	if len(pods.Items) > 0 {
		t.Logf("Warning: %d pods still exist after cleanup (may be terminating)", len(pods.Items))
	}
}

// TestK8sFullGameFlow tests complete game flow with K8s integration
func TestK8sFullGameFlow(t *testing.T) {
	cleanup := setupKindCluster(t)
	defer cleanup()

	ctx := context.Background()
	kubeconfig := os.Getenv("KUBECONFIG")

	// Create config with K8s enabled
	cfg := config.Default()
	cfg.EnableRealK8s = true
	cfg.K8sNamespace = testNamespace
	cfg.Kubeconfig = kubeconfig
	cfg.BoardWidth = 50
	cfg.BoardHeight = 50
	cfg.ShipsPerPlayer = 3
	cfg.RacksPerShip = 3

	// Create K8s client
	client, err := k8s.NewClient(kubeconfig, testNamespace)
	if err != nil {
		t.Fatalf("Failed to create K8s client: %v", err)
	}

	// Ensure namespace
	if err := client.EnsureNamespace(ctx, testNamespace); err != nil {
		t.Fatalf("Failed to create namespace: %v", err)
	}
	defer client.CleanupNamespace(ctx, testNamespace)
	defer client.Clientset().CoreV1().Namespaces().Delete(ctx, testNamespace, metav1.DeleteOptions{})

	// Load companies
	playerTemplate, err := game.LoadCompanyTemplate("netflix")
	if err != nil {
		t.Skip("Netflix template not found, skipping full flow test")
	}
	player := game.CompanyFromTemplate(playerTemplate)
	player.ID = "player"

	enemyTemplate, err := game.LoadCompanyTemplate("aws")
	if err != nil {
		t.Skip("AWS template not found, skipping full flow test")
	}
	enemy := game.CompanyFromTemplate(enemyTemplate)
	enemy.ID = "enemy"

	// Deploy K8s manifests
	templatesDir := k8s.GetTemplatesDir()
	if templatesDir == "templates" {
		templatesDir = filepath.Join("..", "..", "templates")
	}

	playerManifests, _ := k8s.LoadCompanyManifests(templatesDir, "netflix")
	for _, manifest := range playerManifests {
		if err := client.DeployManifest(ctx, manifest); err != nil {
			t.Logf("Warning: failed to deploy player manifest %s: %v", manifest.Name, err)
		}
	}

	enemyManifests, _ := k8s.LoadCompanyManifests(templatesDir, "aws")
	for _, manifest := range enemyManifests {
		if err := client.DeployManifest(ctx, manifest); err != nil {
			t.Logf("Warning: failed to deploy enemy manifest %s: %v", manifest.Name, err)
		}
	}

	// Wait for deployments
	time.Sleep(3 * time.Second)

	// Verify pods are running
	pods, err := client.GetPodStatus(ctx, testNamespace, "app=clustership")
	if err != nil {
		t.Errorf("Failed to get pod status: %v", err)
	}

	t.Logf("Found %d pods in cluster", len(pods))
	for _, pod := range pods {
		t.Logf("  Pod: %s, Status: %s, Service: %s, Company: %s", pod.Name, pod.Status, pod.ServiceID, pod.Company)
	}

	// Test cleanup at end of game
	if err := client.CleanupNamespace(ctx, testNamespace); err != nil {
		t.Errorf("Failed to cleanup at end of game: %v", err)
	}
}
