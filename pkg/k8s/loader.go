package k8s

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

// LoadManifest reads and parses a Kubernetes YAML file
func LoadManifest(path string) (*ServiceManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	// Parse metadata to determine resource kind
	var meta struct {
		Kind     string `json:"kind"`
		Metadata struct {
			Name      string            `json:"name"`
			Namespace string            `json:"namespace"`
			Labels    map[string]string `json:"labels"`
		} `json:"metadata"`
	}

	if err := yaml.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("failed to parse yaml: %w", err)
	}

	manifest := &ServiceManifest{
		Kind:      meta.Kind,
		Name:      meta.Metadata.Name,
		Namespace: meta.Metadata.Namespace,
		Labels:    meta.Metadata.Labels,
		Company:   meta.Metadata.Labels["company"],
		ServiceID: meta.Metadata.Labels["service"],
		RawYAML:   string(data),
		YAMLPath:  path,
	}

	// Extract details based on resource kind
	switch meta.Kind {
	case "Deployment":
		var dep appsv1.Deployment
		if err := yaml.Unmarshal(data, &dep); err != nil {
			return nil, fmt.Errorf("failed to parse deployment: %w", err)
		}
		if dep.Spec.Replicas != nil {
			manifest.Replicas = *dep.Spec.Replicas
		}
		manifest.Containers = extractContainers(dep.Spec.Template.Spec.Containers)

	case "StatefulSet":
		var sts appsv1.StatefulSet
		if err := yaml.Unmarshal(data, &sts); err != nil {
			return nil, fmt.Errorf("failed to parse statefulset: %w", err)
		}
		if sts.Spec.Replicas != nil {
			manifest.Replicas = *sts.Spec.Replicas
		}
		manifest.Containers = extractContainers(sts.Spec.Template.Spec.Containers)

	case "Pod":
		var pod corev1.Pod
		if err := yaml.Unmarshal(data, &pod); err != nil {
			return nil, fmt.Errorf("failed to parse pod: %w", err)
		}
		manifest.Replicas = 1
		manifest.Containers = extractContainers(pod.Spec.Containers)
	}

	return manifest, nil
}

func extractContainers(containers []corev1.Container) []ContainerInfo {
	var result []ContainerInfo
	for _, c := range containers {
		info := ContainerInfo{
			Name:    c.Name,
			Image:   c.Image,
			Command: c.Command,
		}

		for _, p := range c.Ports {
			info.Ports = append(info.Ports, p.ContainerPort)
		}

		if c.Resources.Limits != nil {
			if cpu := c.Resources.Limits.Cpu(); cpu != nil {
				info.CPU = cpu.String()
			}
			if mem := c.Resources.Limits.Memory(); mem != nil {
				info.Memory = mem.String()
			}
		}

		result = append(result, info)
	}
	return result
}

// LoadCompanyManifests loads all YAML manifests for a company
func LoadCompanyManifests(templatesDir, company string) ([]*ServiceManifest, error) {
	dir := filepath.Join(templatesDir, "k8s", strings.ToLower(company))

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read company dir %s: %w", dir, err)
	}

	var manifests []*ServiceManifest
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}

		path := filepath.Join(dir, e.Name())
		m, err := LoadManifest(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping %s: %v\n", path, err)
			continue
		}
		manifests = append(manifests, m)
	}

	return manifests, nil
}

// GetTemplatesDir finds the templates directory from common locations
func GetTemplatesDir() string {
	candidates := []string{
		"templates",
		"./templates",
		"../templates",
	}

	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}

	return "templates" // default
}
