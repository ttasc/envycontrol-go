// sysutil_test.go
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseOSRelease(t *testing.T) {
	tmpDir := t.TempDir()

	// Assuming refactored variable `OsReleasePath = "/etc/os-release"`
	OsReleasePath = filepath.Join(tmpDir, "os-release")

	fedoraContent := `NAME="Fedora Linux"
ID=fedora
ID_LIKE="rhel centos suse"
VERSION_ID="38"`

	os.WriteFile(OsReleasePath, []byte(fedoraContent), 0644)

	id, idLike := parseOSRelease()
	if id != "fedora" {
		t.Errorf("Expected ID 'fedora', got '%s'", id)
	}
	if idLike != "rhel centos suse" {
		t.Errorf("Expected ID_LIKE 'rhel centos suse', got '%s'", idLike)
	}
}

// Assumes the command selection switch statement was extracted from RebuildInitramfs
// for testability.
func TestBuildInitramfsCommand(t *testing.T) {
	tests := []struct {
		name      string
		ostree    bool
		id        string
		idLike    string
		hasDracut bool
		expected  string // checking just the binary name
	}{
		{"Immutable OS", true, "fedora", "", false, "rpm-ostree"},
		{"Debian/Ubuntu", false, "ubuntu", "debian", false, "update-initramfs"},
		{"Fedora/RHEL", false, "fedora", "rhel", true, "dracut"},
		{"Arch Linux", false, "arch", "", false, "mkinitcpio"},
		{"EndeavourOS (Dracut)", false, "endeavouros", "arch", true, "dracut-rebuild"},
		{"Unsupported", false, "unknownos", "", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock the fileExists function closure logic
			mockFileExists := func(path string) bool {
				if tt.ostree && path == "/ostree" { return true }
				if tt.hasDracut && path == "/usr/bin/dracut" { return true }
				return false
			}

			// In sysutil.go, you would extract:
			// func getInitramfsCommand(id, idLike string, fileExists func(string) bool) []string
			cmd := getInitramfsCommand(tt.id, tt.idLike, mockFileExists)

			if tt.expected == "" && len(cmd) != 0 {
				t.Errorf("Expected unsupported to return nil command, got %v", cmd)
			} else if tt.expected != "" && (len(cmd) == 0 || cmd[0] != tt.expected) {
				t.Errorf("Expected binary '%s', got %v", tt.expected, cmd)
			}
		})
	}
}
