package controller

import (
	"context"
	"fmt"

	"k8s.io/klog/v2"
)

// restartPods restarts all pods matching the configured selector
func (uc *UpdateController) restartPods(ctx context.Context) error {
	klog.Infof("Finding pods with selector: %s", uc.config.PodSelector)

	// Get pods matching selector
	pods, err := uc.k8sClient.ListPodsBySelector(ctx, uc.config.PodSelector)
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	if len(pods) == 0 {
		klog.Warning("No pods found matching selector")
		return nil
	}

	klog.Infof("Found %d pods to restart", len(pods))

	// Track workloads to restart (to avoid duplicate restarts)
	workloadsRestarted := make(map[string]bool)

	for _, pod := range pods {
		// Determine the owner (Deployment, StatefulSet, etc.)
		ownerKind, ownerName, err := uc.k8sClient.GetPodOwner(pod)
		if err != nil {
			klog.Warningf("Failed to get owner for pod %s: %v", pod.Name, err)
			continue
		}

		workloadKey := fmt.Sprintf("%s/%s", ownerKind, ownerName)
		if workloadsRestarted[workloadKey] {
			klog.V(2).Infof("Workload %s already restarted, skipping", workloadKey)
			continue
		}

		klog.Infof("Restarting %s: %s", ownerKind, ownerName)
		if err := uc.restartWorkload(ctx, ownerKind, ownerName); err != nil {
			klog.Errorf("Failed to restart %s/%s: %v", ownerKind, ownerName, err)
			continue
		}

		workloadsRestarted[workloadKey] = true
		klog.Infof("Successfully initiated restart for %s/%s", ownerKind, ownerName)
	}

	if len(workloadsRestarted) == 0 {
		return fmt.Errorf("failed to restart any workloads")
	}

	klog.Infof("Successfully restarted %d workloads", len(workloadsRestarted))
	return nil
}

// restartWorkload restarts a specific workload by kind and name
func (uc *UpdateController) restartWorkload(ctx context.Context, kind, name string) error {
	switch kind {
	case "Deployment":
		return uc.k8sClient.RestartDeployment(ctx, name)
	case "StatefulSet":
		return uc.k8sClient.RestartStatefulSet(ctx, name)
	case "DaemonSet":
		return uc.k8sClient.RestartDaemonSet(ctx, name)
	case "ReplicaSet":
		return uc.k8sClient.RestartReplicaSet(ctx, name)
	default:
		return fmt.Errorf("unsupported workload kind: %s", kind)
	}
}
