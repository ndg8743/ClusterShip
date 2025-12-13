package k8s

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Client wraps the k8s clientset
type Client struct {
	clientset *kubernetes.Clientset
	config    *rest.Config
	namespace string
}

// NewClient creates a new k8s client from kubeconfig
func NewClient(kubeconfig, namespace string) (*Client, error) {
	// expand home dir
	if kubeconfig == "" {
		home, _ := os.UserHomeDir()
		kubeconfig = filepath.Join(home, ".kube", "config")
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	return &Client{
		clientset: clientset,
		config:    config,
		namespace: namespace,
	}, nil
}

// IsClusterAvailable checks if we can talk to the cluster
func (c *Client) IsClusterAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := c.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{Limit: 1})
	return err == nil
}

// EnsureNamespace creates the namespace if it doesnt exist
func (c *Client) EnsureNamespace(ns string) error {
	ctx := context.Background()

	_, err := c.clientset.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
	if err == nil {
		return nil // already exists
	}

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
			Labels: map[string]string{
				"app":        "clustership",
				"managed-by": "clustership",
			},
		},
	}

	_, err = c.clientset.CoreV1().Namespaces().Create(ctx, namespace, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create namespace %s: %w", ns, err)
	}
	return nil
}

// GetNamespace returns the configured namespace
func (c *Client) GetNamespace() string {
	return c.namespace
}

// Clientset returns the underlying k8s clientset for advanced ops
func (c *Client) Clientset() *kubernetes.Clientset {
	return c.clientset
}

// CleanupNamespace deletes all clustership resources in the namespace
func (c *Client) CleanupNamespace(ns string) error {
	ctx := context.Background()

	// delete deployments with our label
	err := c.clientset.AppsV1().Deployments(ns).DeleteCollection(
		ctx,
		metav1.DeleteOptions{},
		metav1.ListOptions{LabelSelector: "app=clustership"},
	)
	if err != nil {
		return fmt.Errorf("failed to delete deployments: %w", err)
	}

	// delete statefulsets
	err = c.clientset.AppsV1().StatefulSets(ns).DeleteCollection(
		ctx,
		metav1.DeleteOptions{},
		metav1.ListOptions{LabelSelector: "app=clustership"},
	)
	if err != nil {
		return fmt.Errorf("failed to delete statefulsets: %w", err)
	}

	// delete standalone pods
	err = c.clientset.CoreV1().Pods(ns).DeleteCollection(
		ctx,
		metav1.DeleteOptions{},
		metav1.ListOptions{LabelSelector: "app=clustership"},
	)
	if err != nil {
		return fmt.Errorf("failed to delete pods: %w", err)
	}

	return nil
}

// GetPodStatus returns status of all pods matching label selector
func (c *Client) GetPodStatus(ns, labelSelector string) ([]PodInfo, error) {
	ctx := context.Background()

	pods, err := c.clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	var result []PodInfo
	for _, pod := range pods.Items {
		info := PodInfo{
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Status:    toPodStatus(pod.Status.Phase),
			ServiceID: pod.Labels["service"],
			Company:   pod.Labels["company"],
			Restarts:  getRestarts(&pod),
		}

		// check if all containers ready
		info.Ready = true
		for _, cond := range pod.Status.Conditions {
			if cond.Type == corev1.PodReady && cond.Status != corev1.ConditionTrue {
				info.Ready = false
				break
			}
		}

		// check for terminating
		if pod.DeletionTimestamp != nil {
			info.Status = PodTerminating
		}

		result = append(result, info)
	}

	return result, nil
}

func toPodStatus(phase corev1.PodPhase) PodStatus {
	switch phase {
	case corev1.PodPending:
		return PodPending
	case corev1.PodRunning:
		return PodRunning
	case corev1.PodSucceeded:
		return PodSucceeded
	case corev1.PodFailed:
		return PodFailed
	default:
		return PodUnknown
	}
}

func getRestarts(pod *corev1.Pod) int {
	total := 0
	for _, cs := range pod.Status.ContainerStatuses {
		total += int(cs.RestartCount)
	}
	return total
}
