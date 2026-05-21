package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ExecuteTransaction thực thi "Bản kế hoạch" một cách an toàn
func ExecuteTransaction(plan TransactionPlan) error {
	LogDebug("Preparing to execute filesystem transaction...")

	// 1. Tạo Backup trước khi làm bất kỳ điều gì
	err := createBackup(plan)
	if err != nil {
		return fmt.Errorf("failed to create backup, aborting transaction: %v", err)
	}
	LogInfo("Created safety backup at %s", BackupFilePath)

	// 2. Thực hiện Xóa file rác
	for _, path := range plan.ToRemove {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			LogWarning("Failed to remove legacy file %s: %v", path, err)
		} else if err == nil {
			LogDebug("Removed %s", path)
		}
	}

	// 3. Thực hiện Ghi file mới
	for _, fileConf := range plan.ToCreate {
		err := atomicWrite(fileConf)
		if err != nil {
			// [FAIL-FAST]: Lỗi ghi đĩa -> Lập tức tự cứu lấy hệ thống
			LogError("Filesystem error during transaction: %v", err)
			LogError("Triggering Emergency Rollback...")
			if rbErr := RollbackTransaction(); rbErr != nil {
				return fmt.Errorf("CRITICAL: transaction failed AND rollback failed: %v", rbErr)
			}
			return fmt.Errorf("transaction failed but system was safely rolled back")
		}
	}

	return nil
}

// Hàm ghi file con (bao gồm tự tạo thư mục và chmod)
func atomicWrite(conf FileConfig) error {
	dir := filepath.Dir(conf.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	mode := os.FileMode(0644)
	if conf.Executable {
		mode = 0755
	}

	if err := os.WriteFile(conf.Path, []byte(conf.Content), mode); err != nil {
		return err
	}

	if conf.Executable {
		if err := os.Chmod(conf.Path, 0755); err != nil {
			return fmt.Errorf("chmod +x failed for %s", conf.Path)
		}
	}

	LogInfo("Created file %s", conf.Path)
	return nil
}

// createBackup duyệt qua danh sách các file mà tool chuẩn bị can thiệp,
// nếu file nào đang tồn tại trên đĩa cứng, nhét nó vào file backup.tar.gz
func createBackup(plan TransactionPlan) error {
	// Gộp tất cả đường dẫn (cần xóa + cần ghi đè) thành một tập hợp duy nhất
	pathsToBackup := make(map[string]bool)
	for _, p := range plan.ToRemove {
		pathsToBackup[p] = true
	}
	for _, p := range plan.ToCreate {
		pathsToBackup[p.Path] = true
	}

	// Tạo thư mục /var/lib/envycontrol nếu chưa có
	os.MkdirAll(filepath.Dir(BackupFilePath), 0755)

	backupFile, err := os.Create(BackupFilePath)
	if err != nil {
		return err
	}
	defer backupFile.Close()

	gw := gzip.NewWriter(backupFile)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	for path := range pathsToBackup {
		info, err := os.Stat(path)
		if err == nil && !info.IsDir() { // File có tồn tại
			header, err := tar.FileInfoHeader(info, info.Name())
			if err != nil {
				return err
			}
			// Header tar lưu đường dẫn tuyệt đối bỏ đi dấu '/' ở đầu
			header.Name = path

			if err := tw.WriteHeader(header); err != nil {
				return err
			}

			file, err := os.Open(path)
			if err != nil {
				return err
			}
			_, err = io.Copy(tw, file)
			file.Close()
			if err != nil {
				return err
			}
			LogDebug("Backed up %s", path)
		}
	}

	return nil
}

// RollbackTransaction giải nén backup.tar.gz đè ngược lại vào hệ thống (Cứu hộ)
func RollbackTransaction() error {
	if _, err := os.Stat(BackupFilePath); os.IsNotExist(err) {
		return fmt.Errorf("no backup found at %s", BackupFilePath)
	}

	backupFile, err := os.Open(BackupFilePath)
	if err != nil {
		return err
	}
	defer backupFile.Close()

	gr, err := gzip.NewReader(backupFile)
	if err != nil {
		return err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break // Hết file nén
		}
		if err != nil {
			return err
		}

		// Đường dẫn gốc lúc nén là absolute path
		targetPath := header.Name
		if targetPath == "" {
			continue
		}

		// Tạo lại thư mục cha nếu lỡ tay bị xóa
		os.MkdirAll(filepath.Dir(targetPath), 0755)

		file, err := os.OpenFile(targetPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, os.FileMode(header.Mode))
		if err != nil {
			return err
		}

		if _, err := io.Copy(file, tr); err != nil {
			file.Close()
			return err
		}
		file.Close()
		LogWarning("Rolled back file: %s", targetPath)
	}

	LogInfo("Rollback completed successfully. System state restored.")
	return nil
}
