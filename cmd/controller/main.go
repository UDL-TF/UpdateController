package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/UDL-TF/RestartController/pkg/k8s"
	"github.com/UDL-TF/UpdateController/internal/controller"
	"github.com/UDL-TF/UpdateController/internal/steamcmd"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

func main() {
	klog.InitFlags(nil)

	var kubeconfig string
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file (optional, uses in-cluster config if not provided)")
	flag.Parse()

	// Load configuration from environment
	config := controller.LoadConfig()

	klog.Infof("Starting UpdateController for %s (AppID: %s)", config.SteamApp, config.SteamAppID)
	klog.Infof("Check interval: %s", config.CheckInterval)
	klog.Infof("Namespace: %s", config.Namespace)
	klog.Infof("Pod selector: %s", config.PodSelector)

	// Initialize Kubernetes client
	k8sConfig, err := buildKubeConfig(kubeconfig)
	if err != nil {
		klog.Fatalf("Failed to build Kubernetes config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		klog.Fatalf("Failed to create Kubernetes client: %v", err)
	}

	k8sClient := k8s.NewClient(clientset, config.Namespace)

	// Initialize SteamCMD client
	steamClient := steamcmd.NewClient(
		config.SteamCMDPath,
		config.SteamApp,
		config.SteamAppID,
		config.GameMountPath,
		config.UpdateScript,
	)

	// Create controller
	ctrl := controller.NewUpdateController(config, k8sClient, steamClient)

	// Setup signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start controller
	go func() {
		if err := ctrl.Run(ctx); err != nil {
			klog.Errorf("Controller error: %v", err)
			cancel()
		}
	}()

	// Wait for shutdown signal
	sig := <-sigChan
	klog.Infof("Received signal %v, shutting down gracefully...", sig)
	cancel()

	// Give the controller time to clean up
	time.Sleep(2 * time.Second)
	klog.Info("Shutdown complete")
}

// buildKubeConfig builds Kubernetes configuration from kubeconfig file or in-cluster config
func buildKubeConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig != "" {
		klog.Infof("Using kubeconfig: %s", kubeconfig)
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}

	klog.Info("Using in-cluster configuration")
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
	}
	return config, nil
}
