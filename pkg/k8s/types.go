package k8s

// PodStatus represents the real status of a k8s pod
type PodStatus string

const (
	PodPending     PodStatus = "Pending"
	PodRunning     PodStatus = "Running"
	PodSucceeded   PodStatus = "Succeeded"
	PodFailed      PodStatus = "Failed"
	PodUnknown     PodStatus = "Unknown"
	PodTerminating PodStatus = "Terminating"
)

// PodInfo holds info about a deployed pod
type PodInfo struct {
	Name      string
	Namespace string
	Status    PodStatus
	ServiceID string // links back to game service
	Company   string
	Ready     bool
	Restarts  int
}

// PodEvent fires when pod state changes
type PodEvent struct {
	Type    string // Added, Modified, Deleted
	Pod     PodInfo
	Message string
}

// DeploymentInfo holds info about a k8s deployment
type DeploymentInfo struct {
	Name      string
	Namespace string
	Replicas  int32
	Ready     int32
	Available int32
	ServiceID string
	Company   string
}

// ServiceManifest represents a parsed yaml manifest
type ServiceManifest struct {
	Kind       string            // Deployment, StatefulSet, Pod
	Name       string            // metadata.name
	Namespace  string            // metadata.namespace
	Company    string            // label: company
	ServiceID  string            // label: service
	Replicas   int32             // spec.replicas
	Labels     map[string]string // all labels
	RawYAML    string            // original yaml content
	YAMLPath   string            // file path
	Containers []ContainerInfo
}

// ContainerInfo holds container spec details
type ContainerInfo struct {
	Name    string
	Image   string
	Command []string
	Ports   []int32
	CPU     string // resource limit
	Memory  string // resource limit
}
