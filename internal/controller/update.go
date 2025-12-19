package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/UDL-TF/UpdateController/internal/k8s"
	"github.com/UDL-TF/UpdateController/internal/steamcmd"
	"k8s.io/klog/v2"
)

// UpdateController manages TF2 server updates and pod restarts
type UpdateController struct {
	config      *Config
	k8sClient   *k8s.Client
	steamClient *steamcmd.Client
	retryCount  int
}

// NewUpdateController creates a new UpdateController instance
func NewUpdateController(config *Config, k8sClient *k8s.Client, steamClient *steamcmd.Client) *UpdateController {
	return &UpdateController{
		config:      config,
		k8sClient:   k8sClient,
		steamClient: steamClient,
		retryCount:  0,
	}
}

// Run starts the controller's main loop
func (uc *UpdateController) Run(ctx context.Context) error {
	klog.Info("UpdateController started")

	ticker := time.NewTicker(uc.config.CheckInterval)
	defer ticker.Stop()

	// Perform initial check
	if err := uc.performUpdateCheck(ctx); err != nil {
		klog.Errorf("Initial update check failed: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			klog.Info("UpdateController stopping")
			return ctx.Err()
		case <-ticker.C:
			if err := uc.performUpdateCheck(ctx); err != nil {
				klog.Errorf("Update check failed: %v", err)
			}
		}
	}
}

// performUpdateCheck checks for updates and applies them if available
func (uc *UpdateController) performUpdateCheck(ctx context.Context) error {
	klog.Info("Checking for TF2 updates...")

	// Check if update is available
	updateAvailable, err := uc.steamClient.CheckUpdate(ctx)
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	if !updateAvailable {
		klog.Info("No updates available, continuing monitoring")
		return nil
	}

	klog.Info("Update available! Starting update process...")
	return uc.applyUpdate(ctx)
}

// applyUpdate downloads and applies the update, then restarts pods
func (uc *UpdateController) applyUpdate(ctx context.Context) error {
	// Download and install update
	klog.Info("Downloading and installing update...")
	if err := uc.steamClient.ApplyUpdate(ctx); err != nil {
		return uc.handleUpdateFailure(err)
	}

	// Validate update
	klog.Info("Validating update...")
	if err := uc.steamClient.ValidateUpdate(ctx); err != nil {
		return uc.handleUpdateFailure(fmt.Errorf("update validation failed: %w", err))
	}

	// Restart affected pods
	klog.Info("Update successful! Restarting affected pods...")
	if err := uc.restartPods(ctx); err != nil {
		return uc.handleUpdateFailure(fmt.Errorf("failed to restart pods: %w", err))
	}

	klog.Info("Update process completed successfully")
	uc.retryCount = 0
	return nil
}

// handleUpdateFailure handles update failures with retry logic
func (uc *UpdateController) handleUpdateFailure(err error) error {
	uc.retryCount++
	klog.Errorf("Update failed (attempt %d/%d): %v", uc.retryCount, uc.config.MaxRetries, err)

	if uc.retryCount >= uc.config.MaxRetries {
		klog.Errorf("Max retries exceeded, giving up on this update")
		uc.retryCount = 0
		return fmt.Errorf("update failed after %d attempts: %w", uc.config.MaxRetries, err)
	}

	klog.Infof("Will retry in %s", uc.config.RetryDelay)
	time.Sleep(uc.config.RetryDelay)

	return err
}
