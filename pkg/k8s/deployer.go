package k8s

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

// DeployManifest deploys a service manifest to the cluster
func (c *Client) DeployManifest(manifest *ServiceManifest) error {
	ctx := context.Background()

	switch manifest.Kind {
	case "Deployment":
		return c.deployDeployment(ctx, manifest)
	case "StatefulSet":
		return c.deployStatefulSet(ctx, manifest)
	case "Pod":
		return c.deployPod(ctx, manifest)
	default:
		return fmt.Errorf("unsupported kind: %s", manifest.Kind)
	}
}

func (c *Client) deployDeployment(ctx context.Context, manifest *ServiceManifest) error {
	var dep appsv1.Deployment
	if err := yaml.Unmarshal([]byte(manifest.RawYAML), &dep); err != nil {
		return fmt.Errorf("failed to unmarshal deployment: %w", err)
	}

	// override namespace
	dep.Namespace = c.namespace

	client := c.clientset.AppsV1().Deployments(c.namespace)

	// check if exists
	existing, err := client.Get(ctx, dep.Name, metav1.GetOptions{})
	if err == nil {
		// update existing
		dep.ResourceVersion = existing.ResourceVersion
		_, err = client.Update(ctx, &dep, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update deployment: %w", err)
		}
		return nil
	}

	if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check deployment: %w", err)
	}

	// create new
	_, err = client.Create(ctx, &dep, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create deployment: %w", err)
	}

	return nil
}

func (c *Client) deployStatefulSet(ctx context.Context, manifest *ServiceManifest) error {
	var sts appsv1.StatefulSet
	if err := yaml.Unmarshal([]byte(manifest.RawYAML), &sts); err != nil {
		return fmt.Errorf("failed to unmarshal statefulset: %w", err)
	}

	sts.Namespace = c.namespace

	client := c.clientset.AppsV1().StatefulSets(c.namespace)

	existing, err := client.Get(ctx, sts.Name, metav1.GetOptions{})
	if err == nil {
		sts.ResourceVersion = existing.ResourceVersion
		_, err = client.Update(ctx, &sts, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update statefulset: %w", err)
		}
		return nil
	}

	if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check statefulset: %w", err)
	}

	_, err = client.Create(ctx, &sts, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create statefulset: %w", err)
	}

	return nil
}

func (c *Client) deployPod(ctx context.Context, manifest *ServiceManifest) error {
	var pod corev1.Pod
	if err := yaml.Unmarshal([]byte(manifest.RawYAML), &pod); err != nil {
		return fmt.Errorf("failed to unmarshal pod: %w", err)
	}

	pod.Namespace = c.namespace

	client := c.clientset.CoreV1().Pods(c.namespace)

	// pods cant be updated, delete and recreate
	existing, err := client.Get(ctx, pod.Name, metav1.GetOptions{})
	if err == nil {
		err = client.Delete(ctx, existing.Name, metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("failed to delete existing pod: %w", err)
		}
	}

	_, err = client.Create(ctx, &pod, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create pod: %w", err)
	}

	return nil
}

// DeleteService removes all pods for a service
func (c *Client) DeleteService(serviceID, company string) error {
	ctx := context.Background()
	selector := fmt.Sprintf("app=clustership,service=%s,company=%s", serviceID, company)

	// delete deployment if exists
	deps, err := c.clientset.AppsV1().Deployments(c.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err == nil {
		for _, d := range deps.Items {
			c.clientset.AppsV1().Deployments(c.namespace).Delete(ctx, d.Name, metav1.DeleteOptions{})
		}
	}

	// delete statefulsets
	stss, err := c.clientset.AppsV1().StatefulSets(c.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err == nil {
		for _, s := range stss.Items {
			c.clientset.AppsV1().StatefulSets(c.namespace).Delete(ctx, s.Name, metav1.DeleteOptions{})
		}
	}

	// delete pods directly
	return c.clientset.CoreV1().Pods(c.namespace).DeleteCollection(
		ctx,
		metav1.DeleteOptions{},
		metav1.ListOptions{LabelSelector: selector},
	)
}

// ScaleDeployment changes replica count for a deployment
func (c *Client) ScaleDeployment(name string, replicas int32) error {
	ctx := context.Background()

	dep, err := c.clientset.AppsV1().Deployments(c.namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get deployment: %w", err)
	}

	dep.Spec.Replicas = &replicas
	_, err = c.clientset.AppsV1().Deployments(c.namespace).Update(ctx, dep, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to scale deployment: %w", err)
	}

	return nil
}

// GetDeploymentStatus returns deployment info
func (c *Client) GetDeploymentStatus(name string) (*DeploymentInfo, error) {
	ctx := context.Background()

	dep, err := c.clientset.AppsV1().Deployments(c.namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment: %w", err)
	}

	return &DeploymentInfo{
		Name:      dep.Name,
		Namespace: dep.Namespace,
		Replicas:  *dep.Spec.Replicas,
		Ready:     dep.Status.ReadyReplicas,
		Available: dep.Status.AvailableReplicas,
		ServiceID: dep.Labels["service"],
		Company:   dep.Labels["company"],
	}, nil
}
