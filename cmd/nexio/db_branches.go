package main

import (
	"database/sql"
	"errors"
)

// DBGetCurrentBranchName returns the name of the current branch
func DBGetCurrentBranchName() string {
	Debug("Getting current branch name")
	var name string
	err := db.QueryRow("SELECT name FROM branches WHERE is_current = 1").Scan(&name)
	if err != nil {
		Debug("Failed to get current branch: %v", err)
		MustSucceed(err, "operation failed")
	}
	Debug("Current branch: %s", name)
	return name
}

// DBGetDefaultBranchName returns the name of the default branch
func DBGetDefaultBranchName() string {
	Debug("Getting default branch name")
	var name string
	err := db.QueryRow("SELECT name FROM branches WHERE is_default = 1").Scan(&name)
	if err != nil {
		Debug("Failed to get default branch: %v", err)
		MustSucceed(err, "operation failed")
	}
	Debug("Default branch: %s", name)
	return name
}

// DBCreateBranch creates a new branch record
func DBCreateBranch(name string, isDefault, isCurrent bool) error {
	Debug("Creating branch: %s (default=%v, current=%v)", name, isDefault, isCurrent)
	defVal := 0
	if isDefault {
		defVal = 1
	}
	curVal := 0
	if isCurrent {
		curVal = 1
	}
	_, err := db.Exec(
		"INSERT INTO branches (name, is_default, is_current) VALUES (?, ?, ?)",
		name, defVal, curVal,
	)
	if err != nil {
		Debug("Failed to create branch: %v", err)
		return err
	}
	Debug("Branch created successfully")
	return nil
}

// DBSetBranch updates the current or default branch setting
func DBSetBranch(branch string, configParam string) error {
	Debug("Setting branch: branch=%s, config=%s", branch, configParam)

	// Check if already set
	if configParam == DefaultBranch {
		current := DBGetDefaultBranchName()
		if current == branch {
			Debug("%s", BRANCH_RETURN_CODES[215])
			return errors.New(BRANCH_RETURN_CODES[215])
		}
	} else if configParam == CurrentBranch {
		current := DBGetCurrentBranchName()
		if current == branch {
			Debug("%s", BRANCH_RETURN_CODES[215])
			return errors.New(BRANCH_RETURN_CODES[215])
		}
	}

	// Check branch exists
	branches := DBListBranches()
	found := false
	for _, b := range branches {
		if b == branch {
			found = true
			break
		}
	}
	if !found {
		Debug("Branch does not exist: %s", branch)
		return errors.New(BRANCH_RETURN_CODES[216])
	}

	if configParam == DefaultBranch {
		// Unset old default, set new default
		_, err := db.Exec("UPDATE branches SET is_default = 0 WHERE is_default = 1")
		if err != nil {
			MustSucceed(err, "operation failed")
		}
		_, err = db.Exec("UPDATE branches SET is_default = 1 WHERE name = ?", branch)
		if err != nil {
			MustSucceed(err, "operation failed")
		}
		Debug("Default branch set to: %s", branch)
	} else {
		// Unset old current, set new current
		_, err := db.Exec("UPDATE branches SET is_current = 0 WHERE is_current = 1")
		if err != nil {
			MustSucceed(err, "operation failed")
		}
		_, err = db.Exec("UPDATE branches SET is_current = 1 WHERE name = ?", branch)
		if err != nil {
			MustSucceed(err, "operation failed")
		}
		Debug("Current branch set to: %s", branch)
	}

	Debug("Branch metadata updated successfully")
	return nil
}

// DBListBranches returns all branch names
func DBListBranches() []string {
	Debug("Listing all branches")
	rows, err := db.Query("SELECT name FROM branches ORDER BY name")
	if err != nil {
		Debug("Failed to list branches: %v", err)
		MustSucceed(err, "operation failed")
	}
	defer rows.Close()

	var branches []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			Debug("Failed to scan branch name: %v", err)
			MustSucceed(err, "operation failed")
		}
		branches = append(branches, name)
	}
	if branches == nil {
		branches = []string{}
	}
	Debug("Found %d branches: %v", len(branches), branches)
	return branches
}

// DBBranchExists checks if a branch exists
func DBBranchExists(name string) bool {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM branches WHERE name = ?", name).Scan(&count)
	if err != nil {
		return false
	}
	return count > 0
}

// DBDropBranch removes a branch record from the database.
// Since commits are shared across branches, we only delete commits
// that are not reachable from any other branch's head.
func DBDropBranch(name string) error {
	Debug("Dropping branch: %s", name)

	// Collect all commit IDs reachable from this branch
	branchCommitIds := collectCommitIds(name)

	// Collect all commit IDs reachable from OTHER branches
	otherBranches := DBListBranches()
	reachableFromOthers := make(map[string]struct{})
	for _, other := range otherBranches {
		if other == name {
			continue
		}
		for _, id := range collectCommitIds(other) {
			reachableFromOthers[id] = struct{}{}
		}
	}

	// Delete orphaned commits (only reachable from the branch being dropped)
	for _, commitId := range branchCommitIds {
		if _, reachable := reachableFromOthers[commitId]; !reachable {
			db.Exec("DELETE FROM files WHERE commit_id = ?", commitId)
			db.Exec("DELETE FROM commit_logs WHERE commit_id = ?", commitId)
			db.Exec("DELETE FROM commits WHERE id = ?", commitId)
		}
	}

	// Delete the branch itself
	_, err := db.Exec("DELETE FROM branches WHERE name = ?", name)
	if err != nil {
		return err
	}

	Debug("Branch dropped successfully")
	return nil
}

// collectCommitIds walks the parent chain from a branch's head_commit
// and returns all commit IDs in the chain.
func collectCommitIds(branch string) []string {
	headId := DBGetHeadCommitForBranch(branch)
	if headId == "" {
		return nil
	}

	var ids []string
	currentId := headId
	for currentId != "" {
		ids = append(ids, currentId)
		var parentId sql.NullString
		err := db.QueryRow("SELECT parent_id FROM commit_parents WHERE commit_id = ? AND parent_order = 0", currentId).Scan(&parentId)
		if err != nil {
			break
		}
		if parentId.Valid && parentId.String != "" {
			currentId = parentId.String
		} else {
			currentId = ""
		}
	}
	return ids
}

// DBUpdateBranchHead updates the head_commit for a branch
func DBUpdateBranchHead(branch, commitId string) {
	Debug("Updating branch head: branch=%s, commit=%s", branch, commitId)
	_, err := db.Exec("UPDATE branches SET head_commit = ? WHERE name = ?", commitId, branch)
	if err != nil {
		Debug("Failed to update branch head: %v", err)
		MustSucceed(err, "operation failed")
	}
	Debug("Branch head updated successfully")
}

func DBUpdateBranchHeadTx(tx *sql.Tx, branch, commitId string) error {
	Debug("Updating branch head: branch=%s, commit=%s", branch, commitId)
	_, err := tx.Exec("UPDATE branches SET head_commit = ? WHERE name = ?", commitId, branch)
	if err != nil {
		Debug("Failed to update branch head: %v", err)
		return err
	}
	Debug("Branch head updated successfully")
	return nil
}
