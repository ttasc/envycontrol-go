package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

var githubAPIURL = "https://api.github.com/repos/ttasc/envycontrol-go/releases/latest"

// GitHubRelease represents the JSON structure of a GitHub API release.
type GitHubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []GitHubAsset `json:"assets"`
}

// GitHubAsset represents a downloadable file attached to a GitHub release.
type GitHubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// UpdateEnvyControl handles the top-level orchestration of the update process.
// It establishes a signal shield to prevent dangling files on Ctrl+C, checks the
// latest API release, and safely invokes the atomic binary replacement.
func UpdateEnvyControl() {
	LogInfo("Checking for updates from GitHub...")

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	release, err := fetchLatestRelease(ctx, githubAPIURL)
	if err != nil {
		LogError("%v", err)
		os.Exit(1)
	}

	// Sanitize GitHub tag (e.g., "v1.0.0" -> "1.0.0") to match internal Version constant
	latestVersion := strings.TrimPrefix(release.TagName, "v")
	if latestVersion == Version {
		LogInfo("EnvyControl is already up to date (Version %s)", Version)
		return
	}

	LogInfo("Found new version: %s (Current: %s)", latestVersion, Version)

	downloadURL, err := getTargetAssetURL(release, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		LogError("%v", err)
		os.Exit(1)
	}

	LogInfo("Downloading new executable...")
	if err := downloadAndReplaceBinary(ctx, downloadURL); err != nil {
		LogError("Failed to update: %v", err)
		os.Exit(1)
	}

	LogInfo("Successfully updated to version %s!", latestVersion)
}

// fetchLatestRelease queries the GitHub API for the most recent version tag and assets.
// It respects the context for cancellation and cleanly handles rate limiting.
func fetchLatestRelease(ctx context.Context, apiURL string) (*GitHubRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create API request: %w", err)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to reach GitHub API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("GitHub API rate limit exceeded. Please try updating again later.")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned unexpected status: %s", resp.Status)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to parse GitHub API response: %w", err)
	}

	return &release, nil
}

// getTargetAssetURL matches the runtime OS and architecture against the available release assets.
func getTargetAssetURL(release *GitHubRelease, goos, goarch string) (string, error) {
	targetAssetName := fmt.Sprintf("envycontrol-%s-%s", goos, goarch)
	for _, asset := range release.Assets {
		if asset.Name == targetAssetName {
			return asset.BrowserDownloadURL, nil
		}
	}
	return "", fmt.Errorf("no pre-compiled binary found for %s/%s in release %s", goos, goarch, release.TagName)
}

// downloadAndReplaceBinary fetches the executable safely into a temporary file,
// explicitly grants execution rights (bypassing umask), and swaps it atomically
// with the current running binary using the POSIX rename syscall.
func downloadAndReplaceBinary(ctx context.Context, url string) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to determine active executable path: %w", err)
	}

	// Resolve symlinks to avoid replacing a shortcut instead of the real binary
	if realExecPath, err := filepath.EvalSymlinks(execPath); err == nil {
		execPath = realExecPath
	}

	tmpPath := filepath.Join(filepath.Dir(execPath), ".envycontrol-update.tmp")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create download request: %w", err)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with HTTP %s", resp.Status)
	}

	outFile, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create temporary update file at %s: %w", tmpPath, err)
	}

	// Fail-safe cleanup: ensures temp file is removed if the process panics,
	// is interrupted, or returns early. If os.Rename succeeds, os.Remove safely ignores the missing file.
	defer os.Remove(tmpPath)

	if _, err := io.Copy(outFile, resp.Body); err != nil {
		outFile.Close()
		return fmt.Errorf("failed to write payload to disk: %w", err)
	}

	// The file descriptor must be explicitly closed before chmod or rename can occur.
	if err := outFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file: %w", err)
	}

	// Explicitly set executable bits (0755) to ensure it can be run, regardless of the system's umask.
	if err := os.Chmod(tmpPath, 0755); err != nil {
		return fmt.Errorf("failed to make downloaded binary executable: %w", err)
	}

	// Atomically replace the active binary via POSIX rename. This inherently bypasses
	// "Text file busy" OS errors by relinking the inode.
	if err := os.Rename(tmpPath, execPath); err != nil {
		return fmt.Errorf("failed atomic binary swap: %w", err)
	}

	return nil
}
