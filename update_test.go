package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchLatestRelease_Success(t *testing.T) {
	mockRelease := GitHubRelease{
		TagName: "v2.0.0",
		Assets: []GitHubAsset{
			{Name: "envycontrol-linux-amd64", BrowserDownloadURL: "https://example.com/amd64"},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(mockRelease)
	}))
	defer ts.Close()

	release, err := fetchLatestRelease(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if release.TagName != "v2.0.0" {
		t.Errorf("Expected tag v2.0.0, got %s", release.TagName)
	}

	if len(release.Assets) != 1 || release.Assets[0].Name != "envycontrol-linux-amd64" {
		t.Errorf("Assets improperly decoded: %+v", release.Assets)
	}
}

func TestFetchLatestRelease_Forbidden(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer ts.Close()

	_, err := fetchLatestRelease(context.Background(), ts.URL)
	if err == nil {
		t.Fatalf("Expected error for 403 status, got nil")
	}

	expectedErrStr := "GitHub API rate limit exceeded. Please try updating again later."
	if !strings.Contains(err.Error(), expectedErrStr) {
		t.Errorf("Expected error to contain %q, got %q", expectedErrStr, err.Error())
	}
}

func TestGetTargetAssetURL(t *testing.T) {
	release := &GitHubRelease{
		TagName: "v1.5.0",
		Assets: []GitHubAsset{
			{Name: "envycontrol-linux-amd64", BrowserDownloadURL: "https://mock.url/amd64"},
			{Name: "envycontrol-linux-arm64", BrowserDownloadURL: "https://mock.url/arm64"},
		},
	}

	// Test successful resolution for linux/amd64
	url, err := getTargetAssetURL(release, "linux", "amd64")
	if err != nil {
		t.Fatalf("Expected no error for linux/amd64, got %v", err)
	}
	if url != "https://mock.url/amd64" {
		t.Errorf("Expected URL 'https://mock.url/amd64', got %s", url)
	}

	// Test successful resolution for linux/arm64
	url, err = getTargetAssetURL(release, "linux", "arm64")
	if err != nil {
		t.Fatalf("Expected no error for linux/arm64, got %v", err)
	}
	if url != "https://mock.url/arm64" {
		t.Errorf("Expected URL 'https://mock.url/arm64', got %s", url)
	}

	// Test failing resolution for non-existent architectures
	_, err = getTargetAssetURL(release, "freebsd", "amd64")
	if err == nil {
		t.Fatalf("Expected error for missing asset, got nil")
	}
	if !strings.Contains(err.Error(), "no pre-compiled binary found") {
		t.Errorf("Expected 'no pre-compiled binary found' error, got %v", err)
	}
}
