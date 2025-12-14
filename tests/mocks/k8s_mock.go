// Package mocks provides mock implementations for Kubernetes client testing.
package mocks

import (
	"context"
	"sync"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	"clustership/pkg/k8s"
)

// MockClient wraps a fake kubernetes clientset for testing
type MockClient struct {
	Clientset *fake.Clientset
	Namespace string

	// Track calls for verification
	mu                sync.Mutex
	DeployCalls       []DeployCall
	DeleteCalls       []DeleteCall
	NamespaceCreated  []string
	NamespaceCleaned  []string
}

// DeployCall records a deploy manifest call
type DeployCall struct {
	Manifest *k8s.ServiceManifest
	Err      error
}

// DeleteCall records a delete service call
type DeleteCall struct {
	ServiceID string
	Company   string
	Err       error
}

// NewMockClient creates a new mock client with a fake clientset
func NewMockClient(namespace string) *MockClient {
	return &MockClient{
		Clientset:        fake.NewSimpleClientset(),
		Namespace:        namespace,
		DeployCalls:      make([]DeployCall, 0),
		DeleteCalls:      make([]DeleteCall, 0),
		NamespaceCreated: make([]string, 0),
		NamespaceCleaned: make([]string, 0),
	}
}

// NewMockClientWithObjects creates a mock client with pre-existing objects
func NewMockClientWithObjects(namespace string, objects ...interface{}) *MockClient {
	// Convert to runtime.Object slice
	runtimeObjects := make([]interface{}, 0, len(objects))
	for _, obj := range objects {
		runtimeObjects = append(runtimeObjects, obj)
	}

	mc := &MockClient{
		Clientset:        fake.NewSimpleClientset(),
		Namespace:        namespace,
		DeployCalls:      make([]DeployCall, 0),
		DeleteCalls:      make([]DeleteCall, 0),
		NamespaceCreated: make([]string, 0),
		NamespaceCleaned: make([]string, 0),
	}

	return mc
}

// GetClientset returns the underlying fake clientset
func (m *MockClient) GetClientset() kubernetes.Interface {
	return m.Clientset
}

// GetNamespace returns the configured namespace
func (m *MockClient) GetNamespace() string {
	return m.Namespace
}

// RecordDeployCall records a deploy call for later verification
func (m *MockClient) RecordDeployCall(manifest *k8s.ServiceManifest, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.DeployCalls = append(m.DeployCalls, DeployCall{Manifest: manifest, Err: err})
}

// RecordDeleteCall records a delete call for later verification
func (m *MockClient) RecordDeleteCall(serviceID, company string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.DeleteCalls = append(m.DeleteCalls, DeleteCall{ServiceID: serviceID, Company: company, Err: err})
}

// RecordNamespaceCreated records that a namespace was created
func (m *MockClient) RecordNamespaceCreated(ns string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.NamespaceCreated = append(m.NamespaceCreated, ns)
}

// RecordNamespaceCleaned records that a namespace was cleaned
func (m *MockClient) RecordNamespaceCleaned(ns string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.NamespaceCleaned = append(m.NamespaceCleaned, ns)
}

// GetDeployCalls returns all recorded deploy calls
func (m *MockClient) GetDeployCalls() []DeployCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]DeployCall{}, m.DeployCalls...)
}

// GetDeleteCalls returns all recorded delete calls
func (m *MockClient) GetDeleteCalls() []DeleteCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]DeleteCall{}, m.DeleteCalls...)
}

// Reset clears all recorded calls
func (m *MockClient) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.DeployCalls = make([]DeployCall, 0)
	m.DeleteCalls = make([]DeleteCall, 0)
	m.NamespaceCreated = make([]string, 0)
	m.NamespaceCleaned = make([]string, 0)
}

// MockPodWatcher provides controlled pod event emission for testing
type MockPodWatcher struct {
	mu        sync.Mutex
	events    chan k8s.PodEvent
	callbacks []func(k8s.PodEvent)
	stopped   bool
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewMockPodWatcher creates a new mock pod watcher
func NewMockPodWatcher() *MockPodWatcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &MockPodWatcher{
		events:    make(chan k8s.PodEvent, 100),
		callbacks: make([]func(k8s.PodEvent), 0),
		stopped:   false,
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Subscribe adds a callback for pod events
func (w *MockPodWatcher) Subscribe(callback func(k8s.PodEvent)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.callbacks = append(w.callbacks, callback)
}

// Start begins watching (no-op for mock, just marks as started)
func (w *MockPodWatcher) Start() error {
	return nil
}

// Stop stops the watcher
func (w *MockPodWatcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.stopped {
		w.stopped = true
		w.cancel()
		close(w.events)
	}
}

// Events returns the event channel
func (w *MockPodWatcher) Events() <-chan k8s.PodEvent {
	return w.events
}

// EmitEvent sends a controlled event to all subscribers and the channel
func (w *MockPodWatcher) EmitEvent(event k8s.PodEvent) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.stopped {
		return
	}

	// Send to channel
	select {
	case w.events <- event:
	default:
		// Channel full, skip
	}

	// Call all callbacks
	for _, cb := range w.callbacks {
		cb(event)
	}
}

// EmitPodAdded emits a pod added event
func (w *MockPodWatcher) EmitPodAdded(name, namespace, serviceID, company string) {
	w.EmitEvent(k8s.PodEvent{
		Type: string(watch.Added),
		Pod: k8s.PodInfo{
			Name:      name,
			Namespace: namespace,
			Status:    k8s.PodPending,
			ServiceID: serviceID,
			Company:   company,
			Ready:     false,
			Restarts:  0,
		},
		Message: "Pod " + name + " created",
	})
}

// EmitPodRunning emits a pod running event
func (w *MockPodWatcher) EmitPodRunning(name, namespace, serviceID, company string) {
	w.EmitEvent(k8s.PodEvent{
		Type: string(watch.Modified),
		Pod: k8s.PodInfo{
			Name:      name,
			Namespace: namespace,
			Status:    k8s.PodRunning,
			ServiceID: serviceID,
			Company:   company,
			Ready:     true,
			Restarts:  0,
		},
		Message: "Pod " + name + " is running",
	})
}

// EmitPodDeleted emits a pod deleted event
func (w *MockPodWatcher) EmitPodDeleted(name, namespace, serviceID, company string) {
	w.EmitEvent(k8s.PodEvent{
		Type: string(watch.Deleted),
		Pod: k8s.PodInfo{
			Name:      name,
			Namespace: namespace,
			Status:    k8s.PodTerminating,
			ServiceID: serviceID,
			Company:   company,
			Ready:     false,
			Restarts:  0,
		},
		Message: "Pod " + name + " deleted",
	})
}

// EmitPodFailed emits a pod failed event
func (w *MockPodWatcher) EmitPodFailed(name, namespace, serviceID, company string, restarts int) {
	w.EmitEvent(k8s.PodEvent{
		Type: string(watch.Modified),
		Pod: k8s.PodInfo{
			Name:      name,
			Namespace: namespace,
			Status:    k8s.PodFailed,
			ServiceID: serviceID,
			Company:   company,
			Ready:     false,
			Restarts:  restarts,
		},
		Message: "Pod " + name + " failed",
	})
}

// IsStopped returns whether the watcher is stopped
func (w *MockPodWatcher) IsStopped() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.stopped
}

// MockDeployer tracks deployment operations without cluster access
type MockDeployer struct {
	mu              sync.Mutex
	DeployedManifests []*k8s.ServiceManifest
	DeletedServices   []ServiceRef
	ScaledDeployments []ScaleOp
	ShouldFail        bool
	FailError         error
}

// ServiceRef identifies a service by ID and company
type ServiceRef struct {
	ServiceID string
	Company   string
}

// ScaleOp records a scale operation
type ScaleOp struct {
	Name     string
	Replicas int32
}

// NewMockDeployer creates a new mock deployer
func NewMockDeployer() *MockDeployer {
	return &MockDeployer{
		DeployedManifests: make([]*k8s.ServiceManifest, 0),
		DeletedServices:   make([]ServiceRef, 0),
		ScaledDeployments: make([]ScaleOp, 0),
		ShouldFail:        false,
	}
}

// DeployManifest records the manifest deployment
func (d *MockDeployer) DeployManifest(ctx context.Context, manifest *k8s.ServiceManifest) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.ShouldFail {
		return d.FailError
	}

	d.DeployedManifests = append(d.DeployedManifests, manifest)
	return nil
}

// DeleteService records the service deletion
func (d *MockDeployer) DeleteService(ctx context.Context, serviceID, company string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.ShouldFail {
		return d.FailError
	}

	d.DeletedServices = append(d.DeletedServices, ServiceRef{
		ServiceID: serviceID,
		Company:   company,
	})
	return nil
}

// ScaleDeployment records the scale operation
func (d *MockDeployer) ScaleDeployment(ctx context.Context, name string, replicas int32) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.ShouldFail {
		return d.FailError
	}

	d.ScaledDeployments = append(d.ScaledDeployments, ScaleOp{
		Name:     name,
		Replicas: replicas,
	})
	return nil
}

// GetDeployedManifests returns all deployed manifests
func (d *MockDeployer) GetDeployedManifests() []*k8s.ServiceManifest {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]*k8s.ServiceManifest{}, d.DeployedManifests...)
}

// GetDeletedServices returns all deleted services
func (d *MockDeployer) GetDeletedServices() []ServiceRef {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]ServiceRef{}, d.DeletedServices...)
}

// GetScaledDeployments returns all scale operations
func (d *MockDeployer) GetScaledDeployments() []ScaleOp {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]ScaleOp{}, d.ScaledDeployments...)
}

// SetShouldFail configures the deployer to fail operations
func (d *MockDeployer) SetShouldFail(fail bool, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.ShouldFail = fail
	d.FailError = err
}

// Reset clears all recorded operations
func (d *MockDeployer) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.DeployedManifests = make([]*k8s.ServiceManifest, 0)
	d.DeletedServices = make([]ServiceRef, 0)
	d.ScaledDeployments = make([]ScaleOp, 0)
	d.ShouldFail = false
	d.FailError = nil
}

// WasDeployed checks if a manifest with the given name was deployed
func (d *MockDeployer) WasDeployed(name string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, m := range d.DeployedManifests {
		if m.Name == name {
			return true
		}
	}
	return false
}

// WasDeleted checks if a service was deleted
func (d *MockDeployer) WasDeleted(serviceID, company string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, ref := range d.DeletedServices {
		if ref.ServiceID == serviceID && ref.Company == company {
			return true
		}
	}
	return false
}

// WasScaled checks if a deployment was scaled
func (d *MockDeployer) WasScaled(name string, replicas int32) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, op := range d.ScaledDeployments {
		if op.Name == name && op.Replicas == replicas {
			return true
		}
	}
	return false
}

// Helper functions for creating test pods

// CreateTestPod creates a corev1.Pod for testing
func CreateTestPod(name, namespace, serviceID, company string, phase corev1.PodPhase) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app":     "clustership",
				"service": serviceID,
				"company": company,
			},
		},
		Status: corev1.PodStatus{
			Phase: phase,
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
}

// CreateTestPodWithContainerStatus creates a pod with container status for restart tests
func CreateTestPodWithContainerStatus(name, namespace, serviceID, company string, restarts int32) *corev1.Pod {
	pod := CreateTestPod(name, namespace, serviceID, company, corev1.PodRunning)
	pod.Status.ContainerStatuses = []corev1.ContainerStatus{
		{
			Name:         "main",
			Ready:        true,
			RestartCount: restarts,
		},
	}
	return pod
}

// CreateTestNamespace creates a corev1.Namespace for testing
func CreateTestNamespace(name string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"app":        "clustership",
				"managed-by": "clustership",
			},
		},
	}
}
