package steamcmd

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"k8s.io/klog/v2"
)

// Client handles SteamCMD operations for TF2 updates
type Client struct {
	steamCMDPath  string
	steamApp      string
	steamAppID    string
	gameMountPath string
	updateScript  string
}

// NewClient creates a new SteamCMD client
func NewClient(steamCMDPath, steamApp, steamAppID, gameMountPath, updateScript string) *Client {
	return &Client{
		steamCMDPath:  steamCMDPath,
		steamApp:      steamApp,
		steamAppID:    steamAppID,
		gameMountPath: gameMountPath,
		updateScript:  updateScript,
	}
}

// isGameInstalled checks if the game is already installed
func (c *Client) isGameInstalled() bool {
	// Check if the game directory exists and contains essential files
	gameDir := filepath.Join(c.gameMountPath, c.steamApp)
	if _, err := os.Stat(gameDir); os.IsNotExist(err) {
		return false
	}

	// Check for a critical game file to confirm installation
	srcdsFile := filepath.Join(gameDir, "srcds_run")
	if _, err := os.Stat(srcdsFile); os.IsNotExist(err) {
		return false
	}

	return true
}

// CheckUpdate checks if a TF2 update is available by comparing build IDs
// This method does NOT download any files - it only queries metadata
func (c *Client) CheckUpdate(ctx context.Context) (bool, error) {
	klog.V(2).Info("Checking for updates by comparing build IDs")

	// Check if game is installed at all
	if !c.isGameInstalled() {
		klog.Info("Game not installed, initial installation required")
		return true, nil
	}

	// Get the currently installed build ID
	installedBuildID, err := c.getInstalledBuildID()
	if err != nil {
		return false, fmt.Errorf("failed to get installed build ID: %w", err)
	}

	if installedBuildID == "" {
		klog.Info("No build ID found in manifest, assuming update needed")
		return true, nil
	}

	klog.V(2).Infof("Installed build ID: %s", installedBuildID)

	// Get the latest available build ID from Steam
	latestBuildID, err := c.getLatestBuildID(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get latest build ID: %w", err)
	}

	klog.V(2).Infof("Latest build ID: %s", latestBuildID)

	// Compare build IDs
	if installedBuildID != latestBuildID {
		klog.Infof("Update available: installed=%s, latest=%s", installedBuildID, latestBuildID)
		return true, nil
	}

	klog.Info("Game is up to date")
	return false, nil
}

// ApplyUpdate downloads and applies a TF2 update
func (c *Client) ApplyUpdate(ctx context.Context) error {
	if !c.isGameInstalled() {
		klog.Info("Performing initial game installation via SteamCMD")
	} else {
		klog.Info("Applying TF2 update via SteamCMD")
	}

	scriptPath := filepath.Join(c.gameMountPath, c.updateScript)
	if err := c.createUpdateScript(scriptPath, false); err != nil {
		return fmt.Errorf("failed to create update script: %w", err)
	}

	output, err := c.runSteamCMD(ctx, scriptPath, "update")

	// Check for 0x6 error state and attempt recovery
	if c.hasState0x6Error(output) {
		klog.Warning("Detected 0x6 error state, attempting recovery...")
		if err := c.clearSteamApps(); err != nil {
			return fmt.Errorf("failed to clear steamapps for recovery: %w", err)
		}

		// Retry update after clearing steamapps
		klog.Info("Retrying update after clearing steamapps...")
		output, err = c.runSteamCMD(ctx, scriptPath, "update-retry")

		if err != nil {
			return fmt.Errorf("steamcmd update failed after 0x6 recovery: %w, output: %s", err, string(output))
		}
	} else if err != nil {
		return fmt.Errorf("steamcmd update failed: %w, output: %s", err, string(output))
	}

	if !strings.Contains(string(output), "Success") {
		return fmt.Errorf("update may have failed, check output: %s", string(output))
	}

	return nil
}

// ValidateUpdate validates the installed game files
func (c *Client) ValidateUpdate(ctx context.Context) error {
	klog.Info("Validating TF2 installation")

	scriptPath := filepath.Join(c.gameMountPath, "validate_script.txt")
	if err := c.createValidateScript(scriptPath); err != nil {
		return fmt.Errorf("failed to create validate script: %w", err)
	}

	output, err := c.runSteamCMD(ctx, scriptPath, "validate")

	if err != nil {
		return fmt.Errorf("validation failed: %w, output: %s", err, string(output))
	}

	if !strings.Contains(string(output), "Success") {
		return fmt.Errorf("validation reported issues: %s", string(output))
	}

	return nil
}

// createUpdateScript creates a SteamCMD script for updating
func (c *Client) createUpdateScript(scriptPath string, validateOnly bool) error {
	validateFlag := ""
	if validateOnly {
		validateFlag = "validate"
	}

	script := fmt.Sprintf(`@ShutdownOnFailedCommand 1
@NoPromptForPassword 1
force_install_dir %s
login anonymous
app_update %s %s
quit
`, c.gameMountPath, c.steamAppID, validateFlag)

	if err := os.WriteFile(scriptPath, []byte(script), 0644); err != nil {
		return fmt.Errorf("failed to write script file: %w", err)
	}

	klog.V(3).Infof("Created SteamCMD script at %s", scriptPath)
	return nil
}

// createValidateScript creates a SteamCMD script for validation
func (c *Client) createValidateScript(scriptPath string) error {
	script := fmt.Sprintf(`@ShutdownOnFailedCommand 1
@NoPromptForPassword 1
force_install_dir %s
login anonymous
app_update %s validate
quit
`, c.gameMountPath, c.steamAppID)

	if err := os.WriteFile(scriptPath, []byte(script), 0644); err != nil {
		return fmt.Errorf("failed to write script file: %w", err)
	}

	return nil
}

// hasState0x6Error checks if the output contains the 0x6 error state
func (c *Client) hasState0x6Error(output []byte) bool {
	outputStr := string(output)
	return strings.Contains(outputStr, "state is 0x6") ||
		strings.Contains(outputStr, "state is 0x606") ||
		strings.Contains(outputStr, "Error! App") && strings.Contains(outputStr, "0x6")
}

// clearSteamApps removes the steamapps directory to recover from 0x6 errors
func (c *Client) clearSteamApps() error {
	steamAppsPath := filepath.Join(c.gameMountPath, "steamapps")
	klog.Warningf("Clearing steamapps directory at %s to recover from 0x6 error", steamAppsPath)

	if err := os.RemoveAll(steamAppsPath); err != nil {
		return fmt.Errorf("failed to remove steamapps directory: %w", err)
	}

	klog.Info("Successfully cleared steamapps directory")
	return nil
}

// getInstalledBuildID reads the installed build ID from the local manifest file
func (c *Client) getInstalledBuildID() (string, error) {
	manifestPath := filepath.Join(c.gameMountPath, "steamapps", fmt.Sprintf("appmanifest_%s.acf", c.steamAppID))

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil // Not installed
		}
		return "", fmt.Errorf("failed to read manifest: %w", err)
	}

	// Parse the ACF file for buildid
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.Contains(line, "\"buildid\"") {
			// Format: "buildid"		"12345678"
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				buildID := strings.Trim(parts[1], "\"")
				return buildID, nil
			}
		}
	}

	return "", fmt.Errorf("buildid not found in manifest")
}

// getLatestBuildID queries SteamCMD for the latest available build ID without downloading
func (c *Client) getLatestBuildID(ctx context.Context) (string, error) {
	// Create app_info_print script
	scriptPath := filepath.Join(c.gameMountPath, "app_info_check.txt")
	script := fmt.Sprintf(`@ShutdownOnFailedCommand 1
@NoPromptForPassword 1
login anonymous
app_info_print %s
quit
`, c.steamAppID)

	if err := os.WriteFile(scriptPath, []byte(script), 0644); err != nil {
		return "", fmt.Errorf("failed to write app info script: %w", err)
	}
	defer os.Remove(scriptPath)

	cmd := exec.CommandContext(ctx, c.steamCMDPath+"/steamcmd.sh", "+runscript", scriptPath)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return "", fmt.Errorf("failed to query app info: %w, output: %s", err, string(output))
	}

	// Parse the output for the buildid in the public branch
	// Look for: "buildid"		"12345678" in the "branches" -> "public" section
	outputStr := string(output)
	lines := strings.Split(outputStr, "\n")
	inPublicBranch := false

	for i, line := range lines {
		if strings.Contains(line, "\"public\"") {
			inPublicBranch = true
			continue
		}

		if inPublicBranch {
			// Check if we've exited the public branch section
			if strings.Contains(line, "\"}") && !strings.Contains(line, "\"buildid\"") {
				inPublicBranch = false
				continue
			}

			if strings.Contains(line, "\"buildid\"") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					buildID := strings.Trim(parts[1], "\"")
					return buildID, nil
				}
			}
		}

		// Alternative: look for buildid directly after app ID section
		if strings.Contains(line, fmt.Sprintf("\"%s\"", c.steamAppID)) {
			// Scan next ~50 lines for buildid in common section
			for j := i; j < i+50 && j < len(lines); j++ {
				if strings.Contains(lines[j], "\"buildid\"") && !strings.Contains(lines[j], "branches") {
					parts := strings.Fields(lines[j])
					if len(parts) >= 2 {
						buildID := strings.Trim(parts[1], "\"")
						if buildID != "" && buildID != "0" {
							return buildID, nil
						}
					}
				}
			}
		}
	}

	return "", fmt.Errorf("buildid not found in app_info output")
}

// runSteamCMD executes steamcmd scripts while streaming progress output.
func (c *Client) runSteamCMD(ctx context.Context, scriptPath, stage string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, c.steamCMDPath+"/steamcmd.sh", "+runscript", scriptPath)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to attach steamcmd stdout: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to attach steamcmd stderr: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start steamcmd: %w", err)
	}

	var combined bytes.Buffer
	var combinedMu sync.Mutex
	var wg sync.WaitGroup
	logStream := func(r io.Reader, stream string) {
		defer wg.Done()
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			line := scanner.Text()
			trimmed := strings.TrimSpace(line)
			combinedMu.Lock()
			combined.WriteString(line)
			combined.WriteByte('\n')
			combinedMu.Unlock()

			if trimmed == "" {
				continue
			}

			if c.shouldLogProgress(trimmed) {
				klog.Infof("[%s:%s] %s", stage, stream, trimmed)
			} else {
				klog.V(4).Infof("[%s:%s] %s", stage, stream, trimmed)
			}
		}

		if err := scanner.Err(); err != nil {
			klog.Warningf("[%s:%s] error while reading steamcmd output: %v", stage, stream, err)
		}
	}

	wg.Add(2)
	go logStream(stdout, "stdout")
	go logStream(stderr, "stderr")

	cmdErr := cmd.Wait()
	wg.Wait()

	return combined.Bytes(), cmdErr
}

// shouldLogProgress decides whether a steamcmd line should be surfaced at info level.
func (c *Client) shouldLogProgress(line string) bool {
	lower := strings.ToLower(line)
	return strings.Contains(lower, "update state") ||
		strings.Contains(lower, "progress") ||
		strings.Contains(lower, "downloading") ||
		strings.Contains(lower, "install") ||
		strings.Contains(lower, "validat") ||
		strings.Contains(lower, "success") ||
		strings.Contains(lower, "error") ||
		strings.Contains(lower, "app_update")
}

// parseUpdateStatus parses SteamCMD output to determine if an update is needed
// DEPRECATED: This method triggers actual downloads. Use getInstalledBuildID/getLatestBuildID instead.
func (c *Client) parseUpdateStatus(output []byte) bool {
	outputStr := string(output)

	// Check for common indicators that an update is available or was applied
	if strings.Contains(outputStr, "Update state") &&
		strings.Contains(outputStr, "downloading") {
		return true
	}

	// Check if files need to be downloaded (missing files/initial install)
	if strings.Contains(outputStr, "0x") && strings.Contains(outputStr, "downloading") {
		return true
	}

	// If validation shows files need updating
	if strings.Contains(outputStr, "validating") &&
		(strings.Contains(outputStr, "downloading") || strings.Contains(outputStr, "Update required")) {
		return true
	}

	// If it says "fully installed" or "Up-to-Date", no update needed
	if strings.Contains(outputStr, "fully installed") ||
		strings.Contains(outputStr, "Up-to-Date") ||
		strings.Contains(outputStr, "up to date") {
		return false
	}

	// Look for update-related messages
	if strings.Contains(outputStr, "downloading,") ||
		strings.Contains(outputStr, "0x") { // Build ID changes indicate updates
		return true
	}

	// Default to false if we can't determine
	return false
}
