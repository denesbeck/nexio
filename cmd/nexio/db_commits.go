package main

import (
	"database/sql"
)

type Parent struct {
	CommitId    string
	ParentId    string
	ParentOrder int
}

// DBGetLastCommit returns the last commit on the current branch
func DBGetLastCommit() Commit {
	Debug("Getting last commit")
	branch := DBGetCurrentBranchName()
	return DBGetLastCommitByBranch(branch)
}

// DBGetLastCommitByBranch returns the last commit on a specific branch
func DBGetLastCommitByBranch(branch string) Commit {
	Debug("Getting last commit for branch: %s", branch)

	// Get head_commit from branch record
	var headCommit sql.NullString
	err := db.QueryRow("SELECT head_commit FROM branches WHERE name = ?", branch).Scan(&headCommit)
	if err != nil {
		Debug("Failed to get branch head: %v", err)
		MustSucceed(err, "operation failed")
	}

	if !headCommit.Valid || headCommit.String == "" {
		Debug("No commits found for branch")
		return Commit{}
	}

	var c Commit
	var parentId sql.NullString
	err = db.QueryRow(
		"SELECT id, timestamp, COALESCE(parent_id, '') FROM commits c LEFT JOIN commit_parents p ON c.id = p.commit_id AND parent_order = 0 WHERE id = ?",
		headCommit.String,
	).Scan(&c.Id, &c.Timestamp, &parentId)
	if err != nil {
		Debug("Failed to get last commit: %v", err)
		MustSucceed(err, "operation failed")
	}

	Debug("Last commit for branch: %s", c.Id)
	return c
}

// DBCountCommits returns the total number of commits on the current branch
// by walking the parent chain from head_commit
func DBCountCommits() int {
	Debug("Counting all commits")
	branch := DBGetCurrentBranchName()
	headCommitId := DBGetHeadCommitForBranch(branch)
	if headCommitId == "" {
		return 0
	}

	count := 0
	currentId := headCommitId
	for currentId != "" {
		count++
		var parentId sql.NullString
		err := db.QueryRow("SELECT parent_id FROM commit_parents WHERE commit_id = ? AND parent_order = 0", currentId).Scan(&parentId)
		if err != nil {
			Debug("Failed to walk parent chain: %v", err)
			break
		}
		if parentId.Valid {
			currentId = parentId.String
		} else {
			currentId = ""
		}
	}
	Debug("Counted %d commits", count)
	return count
}

// DBGetCommits returns all commits on the current branch, sorted chronologically
func DBGetCommits() []Commit {
	Debug("Getting all commits")
	branch := DBGetCurrentBranchName()
	return DBGetCommitsByBranch(branch)
}

// DBGetCommitsByBranch returns all commits on a specific branch by walking
// the parent chain from head_commit. Returns commits in chronological order
// (oldest first).
func DBGetCommitsByBranch(branch string) []Commit {
	Debug("Getting commits for branch: %s", branch)

	headCommitId := DBGetHeadCommitForBranch(branch)
	if headCommitId == "" {
		Debug("No head commit for branch %s", branch)
		return []Commit{}
	}

	// Walk the parent chain from head to root, collecting commits
	var commits []Commit
	currentId := headCommitId
	for currentId != "" {
		var c Commit
		var parentId sql.NullString
		err := db.QueryRow(
			"SELECT id, timestamp, COALESCE(parent_id, '') FROM commits c LEFT JOIN commit_parents p ON c.id = p.commit_id AND parent_order = 0 WHERE id = ?",
			currentId,
		).Scan(&c.Id, &c.Timestamp, &parentId)
		if err != nil {
			Debug("Failed to get commit %s: %v", currentId, err)
			break
		}
		commits = append(commits, c)

		if parentId.Valid && parentId.String != "" {
			currentId = parentId.String
		} else {
			currentId = ""
		}
	}

	// Reverse to get chronological order (oldest first)
	for i, j := 0, len(commits)-1; i < j; i, j = i+1, j-1 {
		commits[i], commits[j] = commits[j], commits[i]
	}

	Debug("Retrieved %d commits", len(commits))
	return commits
}

// DBRegisterCommitTx creates a new commit and updates the branch head
func DBRegisterCommitTx(tx *sql.Tx, commitId, message, branch string) error {
	Debug("Registering commit: id=%s, branch=%s", commitId, branch)

	config := GetConfig()
	timestamp := GetTimestamp()

	// Get current head; it becomes the first parent (parent_order 0) of the new commit
	var head sql.NullString
	err := tx.QueryRow("SELECT head_commit FROM branches WHERE name = ?", branch).Scan(&head)
	if err != nil {
		Debug("Failed to get branch head: %v", err)
		return err
	}

	// Insert the commit and its first-parent link atomically
	if _, err := tx.Exec(
		`INSERT INTO commits (id, timestamp, message, author_name, author_email)
			 VALUES (?, ?, ?, ?, ?)`,
		commitId, timestamp, message, config.Name, config.Email,
	); err != nil {
		return err
	}

	// Root commit has no parent; every other commit links to the previous head
	if head.Valid && head.String != "" {
		if _, err := tx.Exec(
			`INSERT INTO commit_parents (commit_id, parent_id, parent_order)
				 VALUES (?, ?, 0)`,
			commitId, head.String,
		); err != nil {
			return err
		}
	}

	// Update branch head
	if err = DBUpdateBranchHeadTx(tx, branch, commitId); err != nil {
		Debug("Failed to update branch head: %v", err)
		return err
	}
	Debug("Commit registered successfully")
	return nil
}

// DBGetCommitMetadata returns the metadata for a specific commit
func DBGetCommitMetadata(commitId string) CommitMetadata {
	Debug("Getting commit metadata: %s", commitId)
	var m CommitMetadata
	err := db.QueryRow(
		"SELECT message, author_name, author_email FROM commits WHERE id = ?",
		commitId,
	).Scan(&m.Message, &m.Author.Name, &m.Author.Email)
	if err != nil {
		Debug("Failed to get commit metadata: %v", err)
		MustSucceed(err, "operation failed")
	}
	Debug("Commit metadata retrieved successfully")
	return m
}

// DBGetCommitLogs returns the log entries for a specific commit
func DBGetCommitLogs(commitId string) []LogFileEntry {
	Debug("Getting commit logs: %s", commitId)
	rows, err := db.Query(
		"SELECT id, op, path, COALESCE(blob_hash, '') FROM commit_logs WHERE commit_id = ?",
		commitId,
	)
	if err != nil {
		Debug("Failed to get commit logs: %v", err)
		MustSucceed(err, "operation failed")
	}
	defer rows.Close()

	var logs []LogFileEntry
	for rows.Next() {
		var e LogFileEntry
		if err := rows.Scan(&e.Id, &e.Op, &e.Path, &e.BlobHash); err != nil {
			Debug("Failed to scan commit log: %v", err)
			MustSucceed(err, "operation failed")
		}
		logs = append(logs, e)
	}
	if err := rows.Err(); err != nil {
		MustSucceed(err, "operation failed")
	}

	if logs == nil {
		logs = []LogFileEntry{}
	}
	Debug("Retrieved %d commit log entries", len(logs))
	return logs
}

// DBSaveCommitLogs saves staging log entries as commit logs
func DBSaveCommitLogs(tx *sql.Tx, commitId string, logs []LogFileEntry) error {
	Debug("Saving commit logs: commit=%s, entries=%d", commitId, len(logs))
	for _, entry := range logs {
		_, err := tx.Exec(
			"INSERT INTO commit_logs (id, commit_id, op, path, blob_hash) VALUES (?, ?, ?, ?, ?)",
			entry.Id, commitId, entry.Op, entry.Path, entry.BlobHash,
		)
		if err != nil {
			Debug("Failed to save commit log entry: %v", err)
			return err
		}
	}
	Debug("Commit logs saved successfully")
	return nil
}

func DBGetParents(commitId string) []Parent {
	Debug("Reading parents of commit: %s", commitId)

	rows, err := db.Query(
		"SELECT commit_id, parent_id, parent_order FROM commit_parents WHERE commit_id = ? ORDER BY parent_order",
		commitId,
	)

	if err != nil {
		Debug("Failed to fetch parents of commit: %s", commitId)
		MustSucceed(err, "operation failed")
	}
	defer rows.Close()

	var parents []Parent
	for rows.Next() {
		var p Parent
		if err := rows.Scan(&p.CommitId, &p.ParentId, &p.ParentOrder); err != nil {
			MustSucceed(err, "operation failed")
		}
		parents = append(parents, p)
	}
	if err := rows.Err(); err != nil {
		MustSucceed(err, "operation failed")
	}

	Debug("Parents loaded successfully for commit: %s", commitId)
	return parents
}

func DBGetFirstParent(commitId string) string {
	Debug("Reading first parent for commit: %s", commitId)

	var firstParent string
	err := db.QueryRow("SELECT parent_id FROM commit_parents WHERE commit_id = ? AND parent_order = 0", commitId).Scan(&firstParent)

	if err == sql.ErrNoRows {
		Debug("Root commit (%s) without any parent commits", commitId)
		return ""
	}

	if err != nil {
		MustSucceed(err, "operation failed")
	}
	Debug("First parent loaded successfully for commit: %s", commitId)
	return firstParent
}

func DBAddParent(commitId, parentId string, order int) {
	Debug("Registering parent %s for commit %s with order %d", parentId, commitId, order)
	_, err := db.Exec("INSERT INTO commit_parents (commit_id, parent_id, parent_order) VALUES (?, ?, ?)", commitId, parentId, order)

	if err != nil {
		Debug("Registering parent %s for commit %s with order %d failed!", parentId, commitId, order)
		MustSucceed(err, "operation failed")
	}
	Debug("Registering parent %s for commit %s with order %d succeeded!", parentId, commitId, order)
}
