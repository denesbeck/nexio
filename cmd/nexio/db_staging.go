package main

import "database/sql"

// DBLogOperation adds an entry to the staging table
func DBLogOperation(id, op, path, blobHash string) {
	Debug("Logging operation: id=%s, op=%s, path=%s", id, op, path)
	_, err := db.Exec(
		"INSERT INTO staging (id, op, path, blob_hash) VALUES (?, ?, ?, ?)",
		id, op, path, blobHash,
	)
	if err != nil {
		Debug("Failed to log operation: %v", err)
		MustSucceed(err, "operation failed")
	}
	Debug("Operation logged successfully")
}

// DBLogEntryLookup finds a staging entry by op and path.
// Op "*" is treated as a wildcard matching any operation.
func DBLogEntryLookup(op, path string) (bool, *LogFileEntry) {
	Debug("Looking up log entry: op=%s, path=%s", op, path)
	var e LogFileEntry
	var blobHash sql.NullString
	var query string
	var args []interface{}

	if op == "*" {
		query = "SELECT id, op, path, blob_hash FROM staging WHERE path = ? LIMIT 1"
		args = []interface{}{path}
	} else {
		query = "SELECT id, op, path, blob_hash FROM staging WHERE op = ? AND path = ? LIMIT 1"
		args = []interface{}{op, path}
	}

	err := db.QueryRow(query, args...).Scan(&e.Id, &e.Op, &e.Path, &blobHash)
	if err != nil {
		Debug("No matching log entry found")
		return false, nil
	}
	if blobHash.Valid {
		e.BlobHash = blobHash.String
	}
	Debug("Found log entry: id=%s, op=%s", e.Id, e.Op)
	return true, &e
}

// DBIsFileStaged checks if a file path exists in the staging table
func DBIsFileStaged(path string) bool {
	Debug("Checking if file is staged: %s", path)
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM staging WHERE path = ?", path).Scan(&count)
	if err != nil {
		Debug("Failed to check staging status: %v", err)
		return false
	}
	staged := count > 0
	Debug("File staged: %v", staged)
	return staged
}

// DBGetStagingLogs returns all staging entries ordered by rowid
func DBGetStagingLogs() []LogFileEntry {
	Debug("Getting staging logs content")
	rows, err := db.Query("SELECT id, op, path, COALESCE(blob_hash, '') FROM staging ORDER BY rowid")
	if err != nil {
		Debug("Failed to query staging logs: %v", err)
		MustSucceed(err, "operation failed")
	}
	defer rows.Close()

	var entries []LogFileEntry
	for rows.Next() {
		var e LogFileEntry
		if err := rows.Scan(&e.Id, &e.Op, &e.Path, &e.BlobHash); err != nil {
			Debug("Failed to scan staging log entry: %v", err)
			MustSucceed(err, "operation failed")
		}
		entries = append(entries, e)
	}
	if entries == nil {
		entries = []LogFileEntry{}
	}
	Debug("Retrieved %d log entries", len(entries))
	return entries
}

// DBRemoveLogEntry removes a staging entry by id
func DBRemoveLogEntry(id string) error {
	Debug("Removing log entry: id=%s", id)
	_, err := db.Exec("DELETE FROM staging WHERE id = ?", id)
	if err != nil {
		Debug("Failed to remove log entry: %v", err)
		return err
	}
	Debug("Log entry removed successfully")
	return nil
}

// DBTruncateLogs clears all staging entries
func DBTruncateLogs() {
	Debug("Truncating staging logs")
	_, err := db.Exec("DELETE FROM staging")
	if err != nil {
		Debug("Failed to truncate staging logs: %v", err)
		MustSucceed(err, "operation failed")
	}
	Debug("Staging logs truncated successfully")
}

func DBTruncateLogsTx(tx *sql.Tx) error {
	Debug("Truncating staging logs")
	if _, err := tx.Exec("DELETE FROM staging"); err != nil {
		Debug("Failed to truncate staging logs: %v", err)
		return err
	}
	Debug("Staging logs truncated successfully")
	return nil
}

// DBUpdateLogEntryBlobHash updates the blob hash of a staging entry
func DBUpdateLogEntryBlobHash(id, blobHash string) {
	Debug("Updating log entry blob hash: id=%s", id)
	_, err := db.Exec("UPDATE staging SET blob_hash = ? WHERE id = ?", blobHash, id)
	if err != nil {
		Debug("Failed to update log entry blob hash: %v", err)
		MustSucceed(err, "operation failed")
	}
	Debug("Log entry blob hash updated successfully")
}
