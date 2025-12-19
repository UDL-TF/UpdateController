package k8s

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// Client wraps Kubernetes client operations
type Client struct {
	clientset *kubernetes.Clientset
	namespace string
}

// NewClient creates a new Kubernetes client wrapper
func NewClient(clientset *kubernetes.Clientset, namespace string) *Client {
	return &Client{
		clientset: clientset,
		namespace: namespace,
	}
}

// ListPodsBySelector lists pods matching a label selector
func (c *Client) ListPodsBySelector(ctx context.Context, selector string) ([]*corev1.Pod, error) {
	listOptions := metav1.ListOptions{
		LabelSelector: selector,
	}

	podList, err := c.clientset.CoreV1().Pods(c.namespace).List(ctx, listOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	result := make([]*corev1.Pod, len(podList.Items))
	for i := range podList.Items {
		result[i] = &podList.Items[i]
	}

	return result, nil
}

// GetPodOwner returns the owner kind and name for a pod
func (c *Client) GetPodOwner(pod *corev1.Pod) (kind, name string, err error) {
	if len(pod.OwnerReferences) == 0 {
		return "", "", fmt.Errorf("pod %s has no owner references", pod.Name)
	}

	// Get the first owner reference
	owner := pod.OwnerReferences[0]

	// If the owner is a ReplicaSet, we need to get its owner (likely a Deployment)
	if owner.Kind == "ReplicaSet" {
		rs, getErr := c.clientset.AppsV1().ReplicaSets(c.namespace).Get(context.Background(), owner.Name, metav1.GetOptions{})
		if getErr != nil {
			return owner.Kind, owner.Name, nil // Return ReplicaSet if we can't get its owner
		}

		if len(rs.OwnerReferences) > 0 {
			rsOwner := rs.OwnerReferences[0]
			return rsOwner.Kind, rsOwner.Name, nil
		}

		return owner.Kind, owner.Name, nil
	}

	return owner.Kind, owner.Name, nil
}

// RestartDeployment restarts a Deployment by adding/updating a restart annotation
func (c *Client) RestartDeployment(ctx context.Context, name string) error {
	deployment, err := c.clientset.AppsV1().Deployments(c.namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get deployment: %w", err)
	}

	if deployment.Spec.Template.ObjectMeta.Annotations == nil {
		deployment.Spec.Template.ObjectMeta.Annotations = make(map[string]string)
	}

	deployment.Spec.Template.ObjectMeta.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)

	_, err = c.clientset.AppsV1().Deployments(c.namespace).Update(ctx, deployment, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update deployment: %w", err)
	}

	klog.V(2).Infof("Added restart annotation to deployment %s", name)
	return nil
}

// RestartStatefulSet restarts a StatefulSet by adding/updating a restart annotation
func (c *Client) RestartStatefulSet(ctx context.Context, name string) error {
	statefulSet, err := c.clientset.AppsV1().StatefulSets(c.namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get statefulset: %w", err)
	}

	if statefulSet.Spec.Template.ObjectMeta.Annotations == nil {
		statefulSet.Spec.Template.ObjectMeta.Annotations = make(map[string]string)
	}

	statefulSet.Spec.Template.ObjectMeta.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)

	_, err = c.clientset.AppsV1().StatefulSets(c.namespace).Update(ctx, statefulSet, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update statefulset: %w", err)
	}

	klog.V(2).Infof("Added restart annotation to statefulset %s", name)
	return nil
}

// RestartDaemonSet restarts a DaemonSet by adding/updating a restart annotation
func (c *Client) RestartDaemonSet(ctx context.Context, name string) error {
	daemonSet, err := c.clientset.AppsV1().DaemonSets(c.namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get daemonset: %w", err)
	}

	if daemonSet.Spec.Template.ObjectMeta.Annotations == nil {
		daemonSet.Spec.Template.ObjectMeta.Annotations = make(map[string]string)
	}

	daemonSet.Spec.Template.ObjectMeta.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)

	_, err = c.clientset.AppsV1().DaemonSets(c.namespace).Update(ctx, daemonSet, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update daemonset: %w", err)
	}

	klog.V(2).Infof("Added restart annotation to daemonset %s", name)
	return nil
}

// RestartReplicaSet restarts a ReplicaSet by scaling to 0 and back
func (c *Client) RestartReplicaSet(ctx context.Context, name string) error {
	replicaSet, err := c.clientset.AppsV1().ReplicaSets(c.namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get replicaset: %w", err)
	}

	// Store original replica count
	originalReplicas := *replicaSet.Spec.Replicas

	// Scale to 0
	zero := int32(0)
	replicaSet.Spec.Replicas = &zero
	_, err = c.clientset.AppsV1().ReplicaSets(c.namespace).Update(ctx, replicaSet, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to scale down replicaset: %w", err)
	}

	klog.V(2).Infof("Scaled replicaset %s to 0", name)

	// Wait a moment
	time.Sleep(2 * time.Second)

	// Scale back to original
	replicaSet, err = c.clientset.AppsV1().ReplicaSets(c.namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get replicaset after scale down: %w", err)
	}

	replicaSet.Spec.Replicas = &originalReplicas
	_, err = c.clientset.AppsV1().ReplicaSets(c.namespace).Update(ctx, replicaSet, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to scale up replicaset: %w", err)
	}

	klog.V(2).Infof("Scaled replicaset %s back to %d", name, originalReplicas)
	return nil
}
