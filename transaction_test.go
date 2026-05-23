// transaction_test.go
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExecuteAndRollbackTransaction(t *testing.T) {
	tmpDir := t.TempDir()

	// Redirect Backup system to temp dir
	BackupFilePath = filepath.Join(tmpDir, "backup", "backup.tar.gz")

	targetFile1 := filepath.Join(tmpDir, "etc", "test-remove.conf")
	targetFile2 := filepath.Join(tmpDir, "etc", "test-create.conf")

	// Pre-create the file that should be backed up and removed
	os.MkdirAll(filepath.Dir(targetFile1), 0755)
	os.WriteFile(targetFile1, []byte("legacy system config"), 0644)

	plan := TransactionPlan{
		ToRemove: []string{targetFile1},
		ToCreate: []FileConfig{
			{Path: targetFile2, Content: "new shiny config", Executable: false},
		},
	}

	// 1. Execute Transaction
	created, err := ExecuteTransaction(plan)
	if err != nil {
		t.Fatalf("Transaction failed: %v", err)
	}

	// Verify State After Transaction
	if _, err := os.Stat(targetFile1); !os.IsNotExist(err) {
		t.Errorf("targetFile1 should have been removed")
	}
	data, _ := os.ReadFile(targetFile2)
	if string(data) != "new shiny config" {
		t.Errorf("targetFile2 was not created correctly")
	}
	if _, err := os.Stat(BackupFilePath); os.IsNotExist(err) {
		t.Errorf("Backup archive was not created at %s", BackupFilePath)
	}

	// 2. Trigger Manual Rollback (Simulating a system failure later in orchestration)
	err = RollbackTransaction(created)
	if err != nil {
		t.Fatalf("Rollback failed: %v", err)
	}

	// Verify State After Rollback
	if _, err := os.Stat(targetFile2); !os.IsNotExist(err) {
		t.Errorf("Rollback failed to clean up newly created orphaned file")
	}
	restoredData, err := os.ReadFile(targetFile1)
	if err != nil {
		t.Fatalf("Rollback failed to restore legacy file: %v", err)
	}
	if string(restoredData) != "legacy system config" {
		t.Errorf("Rollback corrupted restored file contents")
	}
}

func TestExecuteTransaction_AtomicFailureTriggersRollback(t *testing.T) {
	tmpDir := t.TempDir()
	BackupFilePath = filepath.Join(tmpDir, "backup.tar.gz")

	goodFile := filepath.Join(tmpDir, "success.conf")
	badFileDir := filepath.Join(tmpDir, "readonly_dir")
	badFile := filepath.Join(badFileDir, "fail.conf")

	os.MkdirAll(badFileDir, 0500) // Read-only directory to force failure

	plan := TransactionPlan{
		ToRemove: []string{},
		ToCreate: []FileConfig{
			{Path: goodFile, Content: "success", Executable: false},
			{Path: badFile, Content: "fail", Executable: false}, // Will fail here
		},
	}

	createdFiles, err := ExecuteTransaction(plan)

	// Expect failure
	if err == nil {
		t.Fatalf("Expected transaction to fail on read-only directory")
	}

	// Make sure ExecuteTransaction() remembers that it successfully created goodFile.
	if len(createdFiles) != 1 || createdFiles[0] != goodFile {
		t.Errorf("Expected createdFiles to contain exactly [%s], got %v", goodFile, createdFiles)
	}

	// Because of Fail-Fast mechanism, `goodFile` should be cleaned up
	// by the internal automatic rollback.
	if _, statErr := os.Stat(goodFile); !os.IsNotExist(statErr) {
		t.Errorf("Internal transaction rollback failed to clean up %s", goodFile)
	}

	// Fix permission to allow temp dir cleanup by Go
	if err := os.Chmod(badFileDir, 0755); err != nil {
		t.Fatal(err)
	}
}
