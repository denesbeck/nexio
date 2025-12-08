package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
)

func CopyFile(src, dst string) error {
	Debug("Copying file from %s to %s", src, dst)
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		Debug("Source file does not exist: %s", src)
		return err
	}

	if !sourceFileStat.Mode().IsRegular() {
		Debug("Source is not a regular file: %s", src)
		return os.ErrInvalid
	}

	source, err := os.Open(src)
	if err != nil {
		Debug("Failed to open source file: %s", src)
		return err
	}
	defer source.Close()

	if FileExists(dst) {
		Debug("Destination file exists, removing: %s", dst)
		if err := os.Remove(dst); err != nil {
			return err
		}
	}

	path, _ := ParsePath(dst)
	if path != "" {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			Debug("Creating destination directory: %s", path)
			if err := os.MkdirAll(path, 0700); err != nil {
				return err
			}
		}
	}

	destination, err := os.Create(dst)
	if err != nil {
		Debug("Failed to create destination file: %s", dst)
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	if err != nil {
		Debug("Failed to copy file contents")
		return err
	}

	if err := destination.Sync(); err != nil {
		Debug("Failed to sync destination file: %s", dst)
		return err
	}

	if err := os.Chmod(dst, sourceFileStat.Mode()); err != nil {
		Debug("Failed to set permissions on destination file: %s", dst)
		return err
	}

	Debug("File copied successfully with permissions %v", sourceFileStat.Mode())
	return nil
}

func RemoveFile(path string) {
	Debug("Removing file/directory: %s", path)
	err := os.RemoveAll(path)
	if err != nil {
		Debug("Failed to remove file/directory: %s", path)
		MustSucceed(err, "operation failed")
	}
	Debug("File/directory removed successfully")
}

func EmptyDir(path string) error {
	Debug("Emptying directory: %s", path)

	// Read directory contents
	entries, err := os.ReadDir(path)
	if err != nil {
		// If directory doesn't exist, create it
		if os.IsNotExist(err) {
			Debug("Directory doesn't exist, creating: %s", path)
			if err := os.MkdirAll(path, os.ModePerm); err != nil {
				Debug("Failed to create directory: %s", path)
				return err
			}
			return nil
		}
		Debug("Failed to read directory: %s", path)
		return err
	}

	// Remove each entry
	for _, entry := range entries {
		entryPath := filepath.Join(path, entry.Name())
		if err := os.RemoveAll(entryPath); err != nil {
			Debug("Failed to remove entry: %s", entryPath)
			return err
		}
	}

	Debug("Directory emptied successfully")
	return nil
}

func FileExists(path string) bool {
	exists := false
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		exists = true
	}
	Debug("Checking if file exists: %s = %v", path, exists)
	return exists
}

func IsModified(file1, file2 string) (bool, error) {
	Debug("Checking if files are modified: %s vs %s", file1, file2)
	stat1, err := os.Stat(file1)
	if err != nil {
		Debug("Failed to stat first file: %s", file1)
		return false, err
	}
	stat2, err := os.Stat(file2)
	if err != nil {
		Debug("Failed to stat second file: %s", file2)
		return false, err
	}
	size1 := stat1.Size()
	size2 := stat2.Size()

	if size1 != size2 {
		Debug("Files have different sizes")
		return true, nil
	}

	f1, err := os.Open(file1)
	if err != nil {
		Debug("Failed to open first file: %s", file1)
		return false, err
	}
	defer f1.Close()

	f2, err := os.Open(file2)
	if err != nil {
		Debug("Failed to open first file: %s", file2)
		return false, err
	}
	defer f2.Close()

	const bufferSize = 8192 // 8KB
	buffer1 := make([]byte, bufferSize)
	buffer2 := make([]byte, bufferSize)

	for {
		n1, err1 := f1.Read(buffer1)
		n2, err2 := f2.Read(buffer2)

		if n1 != n2 {
			Debug("Files are different (read different amounts)")
			return true, nil
		}

		if !bytes.Equal(buffer1[:n1], buffer2[:n2]) {
			Debug("Files are different")
			return true, nil
		}

		if err1 == io.EOF && err2 == io.EOF {
			Debug("Files are identical")
			return false, nil
		}
		if err1 != nil {
			Debug("Failed to read first file: %s", file1)
			return false, err1
		}
		if err2 != nil {
			Debug("Failed to read second file: %s", file2)
			return false, err2
		}
	}
}
