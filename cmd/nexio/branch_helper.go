package main

import (
	"encoding/json"
	"errors"
	"os"
	"slices"
)

type BranchMetadata struct {
	Default string `json:"default"`
	Current string `json:"current"`
}

const (
	DefaultBranch = "default"
	CurrentBranch = "current"
	InitBranch    = "main"
)

func GetCurrentBranchName() string {
	Debug("Getting current branch name")
	metadata := GetBranchesMetadata()
	Debug("Current branch: %s", metadata.Current)
	return metadata.Current
}

func GetDefaultBranchName() string {
	Debug("Getting default branch name")
	metadata := GetBranchesMetadata()
	Debug("Default branch: %s", metadata.Default)
	return metadata.Default
}

func CreateBranchesMetadata() {
	Debug("Creating branches metadata")
	payload := BranchMetadata{
		Default: InitBranch,
		Current: InitBranch,
	}
	WriteJson(GetDir("branches_metadata"), payload)
	Debug("Branches metadata created with initial branch: %s", InitBranch)
}

func GetBranchesMetadata() (m *BranchMetadata) {
	Debug("Reading branches metadata")
	content, err := os.ReadFile(GetDir("branches_metadata"))
	if err != nil {
		Debug("Failed to read branches metadata")
		MustSucceed(err, "operation failed")
	}
	var metadata BranchMetadata
	if err = json.Unmarshal(content, &metadata); err != nil {
		Debug("Failed to unmarshal branches metadata")
		MustSucceed(err, "operation failed")
	}
	Debug("Branches metadata retrieved successfully")
	return &metadata
}

func SetBranch(branch string, configParam string) error {
	Debug("Setting branch: branch=%s, config=%s", branch, configParam)
	err := WithLock(GetDir("branches_metadata"), DefaultLockTimeout, func() error {
		metadata := GetBranchesMetadata()

		if (configParam == DefaultBranch && metadata.Default == branch) || (configParam == CurrentBranch && metadata.Current == branch) {
			Debug("%s", BRANCH_RETURN_CODES[215])
			return errors.New(BRANCH_RETURN_CODES[215])
		}

		branches := ListBranches()
		if slices.Contains(branches, branch) {
			if configParam == DefaultBranch {
				metadata.Default = branch
				Debug("Setting default branch to: %s", branch)
			} else {
				metadata.Current = branch
				Debug("Setting current branch to: %s", branch)
			}
		} else {
			Debug("Branch does not exist: %s", branch)
			return errors.New(BRANCH_RETURN_CODES[216])
		}

		jsonData, err := json.Marshal(metadata)
		if err != nil {
			Debug("Failed to marshal branch metadata")
			MustSucceed(err, "operation failed")
		}
		if err = os.WriteFile(GetDir("branches_metadata"), jsonData, 0644); err != nil {
			Debug("Failed to write branch metadata")
			MustSucceed(err, "operation failed")
		}
		Debug("Branch metadata updated successfully")
		return nil
	})

	if err != nil {
		if err.Error() == BRANCH_RETURN_CODES[215] || err.Error() == BRANCH_RETURN_CODES[216] {
			return err
		} else {
			MustSucceed(err, "operation failed")
		}
	}
	return nil
}

func ListBranches() []string {
	Debug("Listing all branches")
	entries, err := os.ReadDir(GetDir("branches"))
	if err != nil {
		Debug("Failed to read branches directory")
		MustSucceed(err, "operation failed")
	}
	branches := []string{}
	for _, e := range entries {
		if e.IsDir() {
			branches = append(branches, e.Name())
		}
	}
	Debug("Found %d branches: %v", len(branches), branches)
	return branches
}
