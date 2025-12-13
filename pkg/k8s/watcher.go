package k8s

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// WatchPods watches for pod changes and sends events to callback
func (c *Client) WatchPods(ctx context.Context, callback func(PodEvent)) error {
	watcher, err := c.clientset.CoreV1().Pods(c.namespace).Watch(ctx, metav1.ListOptions{
		LabelSelector: "app=clustership",
	})
	if err != nil {
		return fmt.Errorf("failed to start watch: %w", err)
	}

	go func() {
		defer watcher.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-watcher.ResultChan():
				if !ok {
					return
				}

				pod, ok := event.Object.(*corev1.Pod)
				if !ok {
					continue
				}

				podEvent := PodEvent{
					Type: string(event.Type),
					Pod: PodInfo{
						Name:      pod.Name,
						Namespace: pod.Namespace,
						Status:    toPodStatus(pod.Status.Phase),
						ServiceID: pod.Labels["service"],
						Company:   pod.Labels["company"],
						Ready:     isPodReady(pod),
						Restarts:  getRestarts(pod),
					},
					Message: getEventMessage(event.Type, pod),
				}

				// check terminating
				if pod.DeletionTimestamp != nil {
					podEvent.Pod.Status = PodTerminating
				}

				callback(podEvent)
			}
		}
	}()

	return nil
}

func isPodReady(pod *corev1.Pod) bool {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func getEventMessage(eventType watch.EventType, pod *corev1.Pod) string {
	svc := pod.Labels["service"]
	if svc == "" {
		svc = "unknown"
	}

	switch eventType {
	case watch.Added:
		return fmt.Sprintf("Pod %s created for %s", pod.Name, svc)
	case watch.Modified:
		if pod.DeletionTimestamp != nil {
			return fmt.Sprintf("Pod %s terminating", pod.Name)
		}
		return fmt.Sprintf("Pod %s status: %s", pod.Name, pod.Status.Phase)
	case watch.Deleted:
		return fmt.Sprintf("Pod %s deleted", pod.Name)
	default:
		return fmt.Sprintf("Pod %s event: %s", pod.Name, eventType)
	}
}

// PodWatcher manages watching pods with reconnect
type PodWatcher struct {
	client    *Client
	events    chan PodEvent
	ctx       context.Context
	cancel    context.CancelFunc
	callbacks []func(PodEvent)
}

// NewPodWatcher creates a watcher with auto-reconnect
func NewPodWatcher(client *Client) *PodWatcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &PodWatcher{
		client: client,
		events: make(chan PodEvent, 100),
		ctx:    ctx,
		cancel: cancel,
	}
}

// Subscribe adds a callback for pod events
func (w *PodWatcher) Subscribe(callback func(PodEvent)) {
	w.callbacks = append(w.callbacks, callback)
}

// Start begins watching pods
func (w *PodWatcher) Start() error {
	return w.client.WatchPods(w.ctx, func(event PodEvent) {
		// send to channel
		select {
		case w.events <- event:
		default:
			// channel full, skip
		}

		// call callbacks
		for _, cb := range w.callbacks {
			cb(event)
		}
	})
}

// Stop stops the watcher
func (w *PodWatcher) Stop() {
	w.cancel()
	close(w.events)
}

// Events returns the event channel
func (w *PodWatcher) Events() <-chan PodEvent {
	return w.events
}
