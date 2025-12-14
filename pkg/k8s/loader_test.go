package k8s

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadManifest_Deployment(t *testing.T) {
	// Create temp file with deployment YAML
	content := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment
  namespace: test-ns
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
        ports:
        - containerPort: 8080
        resources:
          limits:
            cpu: "500m"
            memory: "256Mi"
`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "deployment.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	manifest, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}

	// Verify parsed fields
	if manifest.Kind != "Deployment" {
		t.Errorf("expected Kind=Deployment, got %s", manifest.Kind)
	}
	if manifest.Name != "test-deployment" {
		t.Errorf("expected Name=test-deployment, got %s", manifest.Name)
	}
	if manifest.Namespace != "test-ns" {
		t.Errorf("expected Namespace=test-ns, got %s", manifest.Namespace)
	}
	if manifest.Company != "netflix" {
		t.Errorf("expected Company=netflix, got %s", manifest.Company)
	}
	if manifest.ServiceID != "playback" {
		t.Errorf("expected ServiceID=playback, got %s", manifest.ServiceID)
	}
	if manifest.Replicas != 3 {
		t.Errorf("expected Replicas=3, got %d", manifest.Replicas)
	}
	if len(manifest.Containers) != 1 {
		t.Errorf("expected 1 container, got %d", len(manifest.Containers))
	}
	if manifest.Containers[0].Name != "main" {
		t.Errorf("expected container name=main, got %s", manifest.Containers[0].Name)
	}
	if manifest.Containers[0].Image != "nginx:1.25" {
		t.Errorf("expected image=nginx:1.25, got %s", manifest.Containers[0].Image)
	}
	if manifest.Containers[0].CPU != "500m" {
		t.Errorf("expected CPU=500m, got %s", manifest.Containers[0].CPU)
	}
	if manifest.Containers[0].Memory != "256Mi" {
		t.Errorf("expected Memory=256Mi, got %s", manifest.Containers[0].Memory)
	}
	if len(manifest.Containers[0].Ports) != 1 || manifest.Containers[0].Ports[0] != 8080 {
		t.Errorf("expected Ports=[8080], got %v", manifest.Containers[0].Ports)
	}
}

func TestLoadManifest_StatefulSet(t *testing.T) {
	content := `apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: test-statefulset
  labels:
    app: clustership
    company: netflix
    service: database
spec:
  serviceName: test-statefulset
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
        image: postgres:15-alpine
        ports:
        - containerPort: 5432
        resources:
          limits:
            cpu: "1"
            memory: "512Mi"
`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "statefulset.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	manifest, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}

	if manifest.Kind != "StatefulSet" {
		t.Errorf("expected Kind=StatefulSet, got %s", manifest.Kind)
	}
	if manifest.Replicas != 2 {
		t.Errorf("expected Replicas=2, got %d", manifest.Replicas)
	}
	if manifest.ServiceID != "database" {
		t.Errorf("expected ServiceID=database, got %s", manifest.ServiceID)
	}
	if len(manifest.Containers) != 1 {
		t.Errorf("expected 1 container, got %d", len(manifest.Containers))
	}
	if manifest.Containers[0].Name != "postgres" {
		t.Errorf("expected container name=postgres, got %s", manifest.Containers[0].Name)
	}
}

func TestLoadManifest_Pod(t *testing.T) {
	content := `apiVersion: v1
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
    resources:
      limits:
        cpu: "100m"
        memory: "64Mi"
`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "pod.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	manifest, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}

	if manifest.Kind != "Pod" {
		t.Errorf("expected Kind=Pod, got %s", manifest.Kind)
	}
	// Pods always have replicas=1
	if manifest.Replicas != 1 {
		t.Errorf("expected Replicas=1, got %d", manifest.Replicas)
	}
	if manifest.Company != "aws" {
		t.Errorf("expected Company=aws, got %s", manifest.Company)
	}
	if len(manifest.Containers[0].Command) != 3 {
		t.Errorf("expected 3 command args, got %d", len(manifest.Containers[0].Command))
	}
}

func TestLoadManifest_InvalidYAML(t *testing.T) {
	content := `{invalid yaml content`

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "invalid.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err := LoadManifest(path)
	if err == nil {
		t.Error("expected error for invalid YAML, got nil")
	}
}

func TestLoadManifest_MissingFile(t *testing.T) {
	_, err := LoadManifest("/nonexistent/path/file.yaml")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestLoadManifest_MissingLabels(t *testing.T) {
	content := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: no-labels-deploy
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
`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "no-labels.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	manifest, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}

	// Should parse but have empty company/service
	if manifest.Company != "" {
		t.Errorf("expected empty Company, got %s", manifest.Company)
	}
	if manifest.ServiceID != "" {
		t.Errorf("expected empty ServiceID, got %s", manifest.ServiceID)
	}
}

func TestLoadManifest_MultiContainer(t *testing.T) {
	content := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: multi-container
  labels:
    app: clustership
    company: test
    service: multi
spec:
  replicas: 1
  selector:
    matchLabels:
      service: multi
  template:
    metadata:
      labels:
        service: multi
    spec:
      containers:
      - name: main
        image: nginx:1.25
        ports:
        - containerPort: 80
        resources:
          limits:
            cpu: "200m"
            memory: "128Mi"
      - name: sidecar
        image: fluent/fluent-bit:2.0
        ports:
        - containerPort: 2020
        resources:
          limits:
            cpu: "50m"
            memory: "32Mi"
`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "multi.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	manifest, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}

	if len(manifest.Containers) != 2 {
		t.Fatalf("expected 2 containers, got %d", len(manifest.Containers))
	}

	// Verify first container
	if manifest.Containers[0].Name != "main" {
		t.Errorf("expected first container name=main, got %s", manifest.Containers[0].Name)
	}
	if len(manifest.Containers[0].Ports) != 1 || manifest.Containers[0].Ports[0] != 80 {
		t.Errorf("expected first container ports=[80], got %v", manifest.Containers[0].Ports)
	}

	// Verify second container
	if manifest.Containers[1].Name != "sidecar" {
		t.Errorf("expected second container name=sidecar, got %s", manifest.Containers[1].Name)
	}
	if manifest.Containers[1].Image != "fluent/fluent-bit:2.0" {
		t.Errorf("expected second container image=fluent/fluent-bit:2.0, got %s", manifest.Containers[1].Image)
	}
}

func TestLoadManifest_NoReplicas(t *testing.T) {
	// Deployment without explicit replicas (defaults to nil/0 in parsed struct)
	content := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: no-replicas
  labels:
    app: clustership
    company: test
    service: test
spec:
  selector:
    matchLabels:
      service: test
  template:
    metadata:
      labels:
        service: test
    spec:
      containers:
      - name: main
        image: nginx
`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "no-replicas.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	manifest, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}

	// When replicas is not specified, it should be 0
	if manifest.Replicas != 0 {
		t.Errorf("expected Replicas=0 when not specified, got %d", manifest.Replicas)
	}
}

func TestLoadCompanyManifests(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()
	companyDir := filepath.Join(tmpDir, "k8s", "testcompany")
	if err := os.MkdirAll(companyDir, 0755); err != nil {
		t.Fatalf("failed to create test dir: %v", err)
	}

	// Create deployment manifest
	deployContent := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: service-a
  labels:
    app: clustership
    company: testcompany
    service: service-a
spec:
  replicas: 2
  selector:
    matchLabels:
      service: service-a
  template:
    metadata:
      labels:
        service: service-a
    spec:
      containers:
      - name: main
        image: nginx
`
	if err := os.WriteFile(filepath.Join(companyDir, "service-a.yaml"), []byte(deployContent), 0644); err != nil {
		t.Fatalf("failed to write deploy file: %v", err)
	}

	// Create statefulset manifest
	stsContent := `apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: service-b
  labels:
    app: clustership
    company: testcompany
    service: service-b
spec:
  serviceName: service-b
  replicas: 1
  selector:
    matchLabels:
      service: service-b
  template:
    metadata:
      labels:
        service: service-b
    spec:
      containers:
      - name: db
        image: postgres:15
`
	if err := os.WriteFile(filepath.Join(companyDir, "service-b.yaml"), []byte(stsContent), 0644); err != nil {
		t.Fatalf("failed to write sts file: %v", err)
	}

	manifests, err := LoadCompanyManifests(tmpDir, "testcompany")
	if err != nil {
		t.Fatalf("LoadCompanyManifests failed: %v", err)
	}

	if len(manifests) != 2 {
		t.Errorf("expected 2 manifests, got %d", len(manifests))
	}

	// Verify we got both kinds
	kinds := make(map[string]bool)
	for _, m := range manifests {
		kinds[m.Kind] = true
	}
	if !kinds["Deployment"] {
		t.Error("expected Deployment manifest")
	}
	if !kinds["StatefulSet"] {
		t.Error("expected StatefulSet manifest")
	}
}

func TestLoadCompanyManifests_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	companyDir := filepath.Join(tmpDir, "k8s", "emptycompany")
	if err := os.MkdirAll(companyDir, 0755); err != nil {
		t.Fatalf("failed to create test dir: %v", err)
	}

	manifests, err := LoadCompanyManifests(tmpDir, "emptycompany")
	if err != nil {
		t.Fatalf("LoadCompanyManifests failed: %v", err)
	}

	if len(manifests) != 0 {
		t.Errorf("expected 0 manifests for empty directory, got %d", len(manifests))
	}
}

func TestLoadCompanyManifests_NonexistentDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := LoadCompanyManifests(tmpDir, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent directory, got nil")
	}
}

func TestLoadCompanyManifests_SkipsNonYAML(t *testing.T) {
	tmpDir := t.TempDir()
	companyDir := filepath.Join(tmpDir, "k8s", "mixed")
	if err := os.MkdirAll(companyDir, 0755); err != nil {
		t.Fatalf("failed to create test dir: %v", err)
	}

	// Create valid yaml
	yamlContent := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: valid-deploy
  labels:
    app: clustership
    company: mixed
    service: valid
spec:
  replicas: 1
  selector:
    matchLabels:
      service: valid
  template:
    metadata:
      labels:
        service: valid
    spec:
      containers:
      - name: main
        image: nginx
`
	if err := os.WriteFile(filepath.Join(companyDir, "valid.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write yaml file: %v", err)
	}

	// Create non-yaml file
	if err := os.WriteFile(filepath.Join(companyDir, "readme.txt"), []byte("readme content"), 0644); err != nil {
		t.Fatalf("failed to write txt file: %v", err)
	}

	// Create json file (should be skipped)
	if err := os.WriteFile(filepath.Join(companyDir, "config.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("failed to write json file: %v", err)
	}

	manifests, err := LoadCompanyManifests(tmpDir, "mixed")
	if err != nil {
		t.Fatalf("LoadCompanyManifests failed: %v", err)
	}

	if len(manifests) != 1 {
		t.Errorf("expected 1 manifest (only yaml), got %d", len(manifests))
	}
}

func TestLoadCompanyManifests_SkipsSubdirectories(t *testing.T) {
	tmpDir := t.TempDir()
	companyDir := filepath.Join(tmpDir, "k8s", "withsubdir")
	subDir := filepath.Join(companyDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create test dir: %v", err)
	}

	// Create yaml in main dir
	mainContent := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: main-deploy
  labels:
    app: clustership
    company: withsubdir
    service: main
spec:
  replicas: 1
  selector:
    matchLabels:
      service: main
  template:
    metadata:
      labels:
        service: main
    spec:
      containers:
      - name: main
        image: nginx
`
	if err := os.WriteFile(filepath.Join(companyDir, "main.yaml"), []byte(mainContent), 0644); err != nil {
		t.Fatalf("failed to write main yaml: %v", err)
	}

	// Create yaml in subdir (should be ignored)
	subContent := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: sub-deploy
  labels:
    app: clustership
    company: withsubdir
    service: sub
spec:
  replicas: 1
  selector:
    matchLabels:
      service: sub
  template:
    metadata:
      labels:
        service: sub
    spec:
      containers:
      - name: main
        image: nginx
`
	if err := os.WriteFile(filepath.Join(subDir, "sub.yaml"), []byte(subContent), 0644); err != nil {
		t.Fatalf("failed to write sub yaml: %v", err)
	}

	manifests, err := LoadCompanyManifests(tmpDir, "withsubdir")
	if err != nil {
		t.Fatalf("LoadCompanyManifests failed: %v", err)
	}

	if len(manifests) != 1 {
		t.Errorf("expected 1 manifest (subdirectories should be skipped), got %d", len(manifests))
	}
}

func TestLoadCompanyManifests_SkipsInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	companyDir := filepath.Join(tmpDir, "k8s", "invalid")
	if err := os.MkdirAll(companyDir, 0755); err != nil {
		t.Fatalf("failed to create test dir: %v", err)
	}

	// Create valid yaml
	validContent := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: valid-deploy
  labels:
    app: clustership
    company: invalid
    service: valid
spec:
  replicas: 1
  selector:
    matchLabels:
      service: valid
  template:
    metadata:
      labels:
        service: valid
    spec:
      containers:
      - name: main
        image: nginx
`
	if err := os.WriteFile(filepath.Join(companyDir, "valid.yaml"), []byte(validContent), 0644); err != nil {
		t.Fatalf("failed to write valid yaml: %v", err)
	}

	// Create invalid yaml
	invalidContent := `{invalid: yaml: content`
	if err := os.WriteFile(filepath.Join(companyDir, "invalid.yaml"), []byte(invalidContent), 0644); err != nil {
		t.Fatalf("failed to write invalid yaml: %v", err)
	}

	manifests, err := LoadCompanyManifests(tmpDir, "invalid")
	if err != nil {
		t.Fatalf("LoadCompanyManifests failed: %v", err)
	}

	// Should only have the valid manifest
	if len(manifests) != 1 {
		t.Errorf("expected 1 manifest (invalid should be skipped), got %d", len(manifests))
	}
}

func TestExtractContainers_Empty(t *testing.T) {
	// Test with empty slice (edge case)
	content := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: no-containers
  labels:
    app: clustership
    company: test
    service: test
spec:
  replicas: 1
  selector:
    matchLabels:
      service: test
  template:
    metadata:
      labels:
        service: test
    spec:
      containers: []
`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "no-containers.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	manifest, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}

	if manifest.Containers == nil {
		t.Error("expected empty slice, got nil")
	}
	if len(manifest.Containers) != 0 {
		t.Errorf("expected 0 containers, got %d", len(manifest.Containers))
	}
}

func TestExtractContainers_NoResources(t *testing.T) {
	content := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: no-resources
  labels:
    app: clustership
    company: test
    service: test
spec:
  replicas: 1
  selector:
    matchLabels:
      service: test
  template:
    metadata:
      labels:
        service: test
    spec:
      containers:
      - name: main
        image: nginx
`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "no-resources.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	manifest, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}

	if len(manifest.Containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(manifest.Containers))
	}

	// CPU and Memory should be empty strings when not specified
	if manifest.Containers[0].CPU != "" {
		t.Errorf("expected empty CPU, got %s", manifest.Containers[0].CPU)
	}
	if manifest.Containers[0].Memory != "" {
		t.Errorf("expected empty Memory, got %s", manifest.Containers[0].Memory)
	}
}

func TestGetTemplatesDir(t *testing.T) {
	// This test verifies GetTemplatesDir returns a string
	// Actual directory existence depends on runtime environment
	dir := GetTemplatesDir()
	if dir == "" {
		t.Error("GetTemplatesDir returned empty string")
	}
}

// Table-driven test for various manifest kinds
func TestLoadManifest_KindVariations(t *testing.T) {
	tests := []struct {
		name         string
		kind         string
		wantReplicas int32
	}{
		{
			name:         "Deployment",
			kind:         "Deployment",
			wantReplicas: 3,
		},
		{
			name:         "StatefulSet",
			kind:         "StatefulSet",
			wantReplicas: 2,
		},
		{
			name:         "Pod",
			kind:         "Pod",
			wantReplicas: 1, // Pods always have replicas=1
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var content string
			switch tt.kind {
			case "Deployment":
				content = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
  labels:
    app: clustership
    company: test
    service: test
spec:
  replicas: 3
  selector:
    matchLabels:
      service: test
  template:
    metadata:
      labels:
        service: test
    spec:
      containers:
      - name: main
        image: nginx
`
			case "StatefulSet":
				content = `apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: test
  labels:
    app: clustership
    company: test
    service: test
spec:
  serviceName: test
  replicas: 2
  selector:
    matchLabels:
      service: test
  template:
    metadata:
      labels:
        service: test
    spec:
      containers:
      - name: main
        image: nginx
`
			case "Pod":
				content = `apiVersion: v1
kind: Pod
metadata:
  name: test
  labels:
    app: clustership
    company: test
    service: test
spec:
  containers:
  - name: main
    image: nginx
`
			}

			tmpDir := t.TempDir()
			path := filepath.Join(tmpDir, "manifest.yaml")
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				t.Fatalf("failed to write test file: %v", err)
			}

			manifest, err := LoadManifest(path)
			if err != nil {
				t.Fatalf("LoadManifest failed: %v", err)
			}

			if manifest.Kind != tt.kind {
				t.Errorf("expected Kind=%s, got %s", tt.kind, manifest.Kind)
			}
			if manifest.Replicas != tt.wantReplicas {
				t.Errorf("expected Replicas=%d, got %d", tt.wantReplicas, manifest.Replicas)
			}
		})
	}
}

// Test case sensitivity of company names in path
func TestLoadCompanyManifests_CaseInsensitive(t *testing.T) {
	tmpDir := t.TempDir()
	// Create with lowercase (as the function lowercases)
	companyDir := filepath.Join(tmpDir, "k8s", "netflix")
	if err := os.MkdirAll(companyDir, 0755); err != nil {
		t.Fatalf("failed to create test dir: %v", err)
	}

	content := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
  labels:
    app: clustership
    company: netflix
    service: test
spec:
  replicas: 1
  selector:
    matchLabels:
      service: test
  template:
    metadata:
      labels:
        service: test
    spec:
      containers:
      - name: main
        image: nginx
`
	if err := os.WriteFile(filepath.Join(companyDir, "test.yaml"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write yaml: %v", err)
	}

	// Call with mixed case - should lowercase to match directory
	manifests, err := LoadCompanyManifests(tmpDir, "Netflix")
	if err != nil {
		t.Fatalf("LoadCompanyManifests failed: %v", err)
	}

	if len(manifests) != 1 {
		t.Errorf("expected 1 manifest, got %d", len(manifests))
	}
}
