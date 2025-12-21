//go:build ignore

// Nexio Storage Benchmark: JSON vs SQLite
//
// This script compares the performance of JSON file-based storage vs SQLite
// for Nexio's metadata operations (staging, file lists, garbage collection).
//
// Usage:
//   cd scripts
//   go run benchmark_storage.go
//
// Requirements:
//   go get modernc.org/sqlite

package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// ============================================================================
// Data structures (matching nexio)
// ============================================================================

type LogFileEntry struct {
	Id       string `json:"id"`
	Op       string `json:"op"`
	Path     string `json:"path"`
	BlobHash string `json:"blobHashField"`
}

type FileListEntry struct {
	Id       string `json:"id"`
	CommitId string `json:"commitId"`
	Path     string `json:"path"`
	BlobHash string `json:"blobHash"`
	Mode     uint32 `json:"mode"`
}

// ============================================================================
// Helpers
// ============================================================================

func randomHash() string {
	const chars = "0123456789abcdef"
	b := make([]byte, 64)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}

func randomPath(i int) string {
	dirs := []string{"src", "lib", "pkg", "internal", "cmd", "test", "docs"}
	return fmt.Sprintf("%s/subdir%d/file%d.go", dirs[rand.Intn(len(dirs))], i%100, i)
}

// ============================================================================
// JSON-based implementation (current nexio approach)
// ============================================================================

type JSONStore struct {
	stagingFile string
	fileListDir string
}

func NewJSONStore(dir string) *JSONStore {
	stagingFile := filepath.Join(dir, "staging.json")
	fileListDir := filepath.Join(dir, "commits")
	os.MkdirAll(fileListDir, 0755)
	os.WriteFile(stagingFile, []byte("[]"), 0644)
	return &JSONStore{stagingFile: stagingFile, fileListDir: fileListDir}
}

func (s *JSONStore) AddToStaging(entry LogFileEntry) error {
	data, _ := os.ReadFile(s.stagingFile)
	var entries []LogFileEntry
	json.Unmarshal(data, &entries)
	entries = append(entries, entry)
	newData, _ := json.Marshal(entries)
	return os.WriteFile(s.stagingFile, newData, 0644)
}

func (s *JSONStore) LookupStaging(path string) (*LogFileEntry, bool) {
	data, _ := os.ReadFile(s.stagingFile)
	var entries []LogFileEntry
	json.Unmarshal(data, &entries)
	for _, e := range entries {
		if e.Path == path {
			return &e, true
		}
	}
	return nil, false
}

func (s *JSONStore) WriteCommitFileList(commitId string, files []FileListEntry) error {
	dir := filepath.Join(s.fileListDir, commitId)
	os.MkdirAll(dir, 0755)
	data, _ := json.Marshal(files)
	return os.WriteFile(filepath.Join(dir, "fileList.json"), data, 0644)
}

func (s *JSONStore) LookupFileInCommit(commitId, path string) (*FileListEntry, bool) {
	data, _ := os.ReadFile(filepath.Join(s.fileListDir, commitId, "fileList.json"))
	var entries []FileListEntry
	json.Unmarshal(data, &entries)
	for _, e := range entries {
		if e.Path == path {
			return &e, true
		}
	}
	return nil, false
}

func (s *JSONStore) CollectAllBlobHashes() map[string]struct{} {
	hashes := make(map[string]struct{})
	commits, _ := os.ReadDir(s.fileListDir)
	for _, c := range commits {
		data, _ := os.ReadFile(filepath.Join(s.fileListDir, c.Name(), "fileList.json"))
		var entries []FileListEntry
		json.Unmarshal(data, &entries)
		for _, e := range entries {
			hashes[e.BlobHash] = struct{}{}
		}
	}
	return hashes
}

// ============================================================================
// SQLite-based implementation
// ============================================================================

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(dir string) *SQLiteStore {
	dbPath := filepath.Join(dir, "index.db")
	db, _ := sql.Open("sqlite", dbPath)

	db.Exec(`CREATE TABLE IF NOT EXISTS staging (
		id TEXT PRIMARY KEY,
		op TEXT,
		path TEXT,
		blob_hash TEXT
	)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_staging_path ON staging(path)`)

	db.Exec(`CREATE TABLE IF NOT EXISTS files (
		id TEXT PRIMARY KEY,
		commit_id TEXT,
		path TEXT,
		blob_hash TEXT,
		mode INTEGER
	)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_files_commit_path ON files(commit_id, path)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_files_blob ON files(blob_hash)`)

	return &SQLiteStore{db: db}
}

func (s *SQLiteStore) Close() {
	s.db.Close()
}

func (s *SQLiteStore) AddToStaging(entry LogFileEntry) error {
	_, err := s.db.Exec("INSERT INTO staging (id, op, path, blob_hash) VALUES (?, ?, ?, ?)",
		entry.Id, entry.Op, entry.Path, entry.BlobHash)
	return err
}

func (s *SQLiteStore) AddToStagingBatch(entries []LogFileEntry) error {
	tx, _ := s.db.Begin()
	stmt, _ := tx.Prepare("INSERT INTO staging (id, op, path, blob_hash) VALUES (?, ?, ?, ?)")
	for _, e := range entries {
		stmt.Exec(e.Id, e.Op, e.Path, e.BlobHash)
	}
	stmt.Close()
	return tx.Commit()
}

func (s *SQLiteStore) LookupStaging(path string) (*LogFileEntry, bool) {
	var e LogFileEntry
	err := s.db.QueryRow("SELECT id, op, path, blob_hash FROM staging WHERE path = ?", path).
		Scan(&e.Id, &e.Op, &e.Path, &e.BlobHash)
	if err != nil {
		return nil, false
	}
	return &e, true
}

func (s *SQLiteStore) WriteCommitFileList(commitId string, files []FileListEntry) error {
	tx, _ := s.db.Begin()
	stmt, _ := tx.Prepare("INSERT INTO files (id, commit_id, path, blob_hash, mode) VALUES (?, ?, ?, ?, ?)")
	for _, f := range files {
		stmt.Exec(f.Id, commitId, f.Path, f.BlobHash, f.Mode)
	}
	stmt.Close()
	return tx.Commit()
}

func (s *SQLiteStore) LookupFileInCommit(commitId, path string) (*FileListEntry, bool) {
	var e FileListEntry
	err := s.db.QueryRow("SELECT id, commit_id, path, blob_hash, mode FROM files WHERE commit_id = ? AND path = ?",
		commitId, path).Scan(&e.Id, &e.CommitId, &e.Path, &e.BlobHash, &e.Mode)
	if err != nil {
		return nil, false
	}
	return &e, true
}

func (s *SQLiteStore) CollectAllBlobHashes() map[string]struct{} {
	hashes := make(map[string]struct{})
	rows, _ := s.db.Query("SELECT DISTINCT blob_hash FROM files")
	defer rows.Close()
	for rows.Next() {
		var h string
		rows.Scan(&h)
		hashes[h] = struct{}{}
	}
	return hashes
}

// ============================================================================
// Benchmarks
// ============================================================================

func runBenchmarks() {
	sizes := []int{100, 1000, 5000, 10000}

	fmt.Println("# Nexio Storage Benchmark Results")
	fmt.Println()
	fmt.Println("Generated:", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Println()

	for _, n := range sizes {
		fmt.Printf("## Dataset Size: %d entries\n\n", n)
		benchmarkStaging(n)
		benchmarkFileList(n)
		benchmarkGC(n)
	}
}

func benchmarkStaging(n int) {
	fmt.Printf("### Staging Operations\n\n")

	// Prepare test data
	entries := make([]LogFileEntry, n)
	for i := 0; i < n; i++ {
		entries[i] = LogFileEntry{
			Id:       randomHash()[:20],
			Op:       []string{"ADD", "MOD", "REM"}[rand.Intn(3)],
			Path:     randomPath(i),
			BlobHash: randomHash(),
		}
	}

	// JSON: Sequential adds (simulating nexio add)
	dir1, _ := os.MkdirTemp("", "json-bench")
	defer os.RemoveAll(dir1)
	jsonStore := NewJSONStore(dir1)

	start := time.Now()
	for _, e := range entries {
		jsonStore.AddToStaging(e)
	}
	jsonAddTime := time.Since(start)

	// SQLite: Sequential adds
	dir2, _ := os.MkdirTemp("", "sqlite-bench")
	defer os.RemoveAll(dir2)
	sqlStore := NewSQLiteStore(dir2)
	defer sqlStore.Close()

	start = time.Now()
	for _, e := range entries {
		sqlStore.AddToStaging(e)
	}
	sqliteAddTime := time.Since(start)

	// SQLite: Batched adds (optimal)
	dir3, _ := os.MkdirTemp("", "sqlite-batch-bench")
	defer os.RemoveAll(dir3)
	sqlBatchStore := NewSQLiteStore(dir3)
	defer sqlBatchStore.Close()

	start = time.Now()
	sqlBatchStore.AddToStagingBatch(entries)
	sqliteBatchTime := time.Since(start)

	// Lookups (worst case: last entry)
	lookupPath := entries[n-1].Path

	start = time.Now()
	for i := 0; i < 100; i++ {
		jsonStore.LookupStaging(lookupPath)
	}
	jsonLookupTime := time.Since(start) / 100

	start = time.Now()
	for i := 0; i < 100; i++ {
		sqlStore.LookupStaging(lookupPath)
	}
	sqliteLookupTime := time.Since(start) / 100

	fmt.Printf("| Operation | JSON | SQLite | SQLite (batch) | Improvement |\n")
	fmt.Printf("|-----------|------|--------|----------------|-------------|\n")
	fmt.Printf("| Add %d files | %v | %v | %v | %.0fx faster (batch) |\n",
		n, jsonAddTime.Round(time.Millisecond), sqliteAddTime.Round(time.Millisecond),
		sqliteBatchTime.Round(time.Microsecond), float64(jsonAddTime)/float64(sqliteBatchTime))
	fmt.Printf("| Single lookup | %v | %v | - | %.1fx faster |\n",
		jsonLookupTime.Round(time.Microsecond), sqliteLookupTime.Round(time.Microsecond),
		float64(jsonLookupTime)/float64(sqliteLookupTime))
	fmt.Println()
}

func benchmarkFileList(n int) {
	fmt.Printf("### Commit File List Operations\n\n")

	// Prepare test data
	files := make([]FileListEntry, n)
	for i := 0; i < n; i++ {
		files[i] = FileListEntry{
			Id:       randomHash()[:20],
			CommitId: "abc123",
			Path:     randomPath(i),
			BlobHash: randomHash(),
			Mode:     0644,
		}
	}

	// JSON
	dir1, _ := os.MkdirTemp("", "json-bench")
	defer os.RemoveAll(dir1)
	jsonStore := NewJSONStore(dir1)
	jsonStore.WriteCommitFileList("abc123", files)

	// SQLite
	dir2, _ := os.MkdirTemp("", "sqlite-bench")
	defer os.RemoveAll(dir2)
	sqlStore := NewSQLiteStore(dir2)
	defer sqlStore.Close()
	sqlStore.WriteCommitFileList("abc123", files)

	// Lookup (worst case: last file)
	lookupPath := files[n-1].Path

	start := time.Now()
	for i := 0; i < 100; i++ {
		jsonStore.LookupFileInCommit("abc123", lookupPath)
	}
	jsonLookupTime := time.Since(start) / 100

	start = time.Now()
	for i := 0; i < 100; i++ {
		sqlStore.LookupFileInCommit("abc123", lookupPath)
	}
	sqliteLookupTime := time.Since(start) / 100

	fmt.Printf("| Operation | JSON | SQLite | Improvement |\n")
	fmt.Printf("|-----------|------|--------|-------------|\n")
	fmt.Printf("| File lookup in commit | %v | %v | %.1fx faster |\n",
		jsonLookupTime.Round(time.Microsecond), sqliteLookupTime.Round(time.Microsecond),
		float64(jsonLookupTime)/float64(sqliteLookupTime))
	fmt.Println()
}

func benchmarkGC(n int) {
	fmt.Printf("### Garbage Collection (Collect Referenced Hashes)\n\n")

	numCommits := 50 // Simulate 50 commits
	filesPerCommit := n / 10
	if filesPerCommit < 10 {
		filesPerCommit = 10
	}

	// JSON
	dir1, _ := os.MkdirTemp("", "json-bench")
	defer os.RemoveAll(dir1)
	jsonStore := NewJSONStore(dir1)

	for c := 0; c < numCommits; c++ {
		files := make([]FileListEntry, filesPerCommit)
		for i := 0; i < filesPerCommit; i++ {
			files[i] = FileListEntry{
				Id:       randomHash()[:20],
				CommitId: fmt.Sprintf("commit%d", c),
				Path:     randomPath(i),
				BlobHash: randomHash(),
				Mode:     0644,
			}
		}
		jsonStore.WriteCommitFileList(fmt.Sprintf("commit%d", c), files)
	}

	// SQLite
	dir2, _ := os.MkdirTemp("", "sqlite-bench")
	defer os.RemoveAll(dir2)
	sqlStore := NewSQLiteStore(dir2)
	defer sqlStore.Close()

	for c := 0; c < numCommits; c++ {
		files := make([]FileListEntry, filesPerCommit)
		for i := 0; i < filesPerCommit; i++ {
			files[i] = FileListEntry{
				Id:       randomHash()[:20],
				CommitId: fmt.Sprintf("commit%d", c),
				Path:     randomPath(i),
				BlobHash: randomHash(),
				Mode:     0644,
			}
		}
		sqlStore.WriteCommitFileList(fmt.Sprintf("commit%d", c), files)
	}

	start := time.Now()
	jsonStore.CollectAllBlobHashes()
	jsonGCTime := time.Since(start)

	start = time.Now()
	sqlStore.CollectAllBlobHashes()
	sqliteGCTime := time.Since(start)

	fmt.Printf("| Operation | JSON | SQLite | Improvement |\n")
	fmt.Printf("|-----------|------|--------|-------------|\n")
	fmt.Printf("| Collect hashes (%d commits x %d files) | %v | %v | %.1fx faster |\n",
		numCommits, filesPerCommit,
		jsonGCTime.Round(time.Microsecond), sqliteGCTime.Round(time.Microsecond),
		float64(jsonGCTime)/float64(sqliteGCTime))
	fmt.Println()
}

func main() {
	rand.Seed(time.Now().UnixNano())
	runBenchmarks()
}
