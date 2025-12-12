package main

import (
	"bytes"
	"compress/zlib"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"

	"lukechampine.com/blake3"
)

const zlibCompressionLevel = 6

func HashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := blake3.New(32, nil) // 32 bytes
	// Stream file data to the hasher for processing
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// Get BLAKE3 hash of a byte slice
func HashBytes(data []byte) string {
	sum := blake3.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Get the path to a blob considering the sharded directory
func BlobPath(hash string) string {
	return filepath.Join(dirs.Objects, hash[:2], hash[2:])
}

func BlobExists(hash string) bool { return FileExists(BlobPath(hash)) }

func WriteBlob(path string) (string, error) {
	Debug("Writing blob for file: %s", path)

	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		Debug("Failed to read file: %s", path)
		return "", err
	}

	// Compute hash
	hash := HashBytes(data)
	Debug("Computed hash: %s", hash)

	// Check if blob already exists (deduplication)
	if BlobExists(hash) {
		Debug("Blob already exists, skipping write")
		return hash, nil
	}

	// Compress data
	var compressed bytes.Buffer
	writer, err := zlib.NewWriterLevel(&compressed, zlibCompressionLevel)
	if err != nil {
		Debug("Failed to create zlib writer")
		return "", err
	}
	defer writer.Close()

	if _, err := writer.Write(data); err != nil {
		Debug("Failed to compress data")
		return "", err
	}
	if err := writer.Close(); err != nil {
		Debug("Failed to close zlib writer")
		return "", err
	}

	// Create shard directory
	blobPath := BlobPath(hash)
	shardDir := filepath.Dir(blobPath)
	if err := os.MkdirAll(shardDir, 0755); err != nil {
		Debug("Failed to create shard directory: %s", shardDir)
		return "", err
	}

	// Write compressed blob
	if err := os.WriteFile(blobPath, compressed.Bytes(), 0644); err != nil {
		Debug("Failed to write blob: %s", blobPath)
		return "", err
	}

	Debug("Blob writeen successfully: %s", blobPath)
	return hash, nil
}

// Read and decompress a blob from the object store.
// Return the original file content.
func ReadBlob(hash string) ([]byte, error) {
	Debug("Reading blob: %s", hash)

	blobPath := BlobPath(hash)
	compressed, err := os.ReadFile(blobPath)
	if err != nil {
		Debug("Failed to read blob: %s", blobPath)
		return nil, err
	}

	reader, err := zlib.NewReader(bytes.NewReader(compressed))
	if err != nil {
		Debug("Failed to create zlib reader")
		return nil, err
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		Debug("Failed to decompress blob")
		return nil, err
	}

	Debug("Blob read successfully: %d bytes", len(data))
	return data, nil
}

// Decompress a blob and write it to the destination path.
func RestoreBlob(hash string, dest string, mode os.FileMode) error {
	Debug("Restoring blob %s to %s", hash, dest)

	data, err := ReadBlob(hash)
	if err != nil {
		return err
	}

	destDir := filepath.Dir(dest)
	if destDir != "." {
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return err
		}
	}

	if err := os.WriteFile(dest, data, mode); err != nil {
		Debug("Failed to write file: %s", dest)
		return err
	}

	Debug("Blob restored successfully: %s", dest)
	return nil
}
