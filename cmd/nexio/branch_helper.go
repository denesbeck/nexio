package main

const (
	DefaultBranch = "default"
	CurrentBranch = "current"
	InitBranch    = "main"
)

func GetCurrentBranchName() string {
	return DBGetCurrentBranchName()
}

func GetDefaultBranchName() string {
	return DBGetDefaultBranchName()
}

func CreateBranchesMetadata() {
	// In SQLite mode, the initial branch is created by DBCreateBranch during init
	Debug("CreateBranchesMetadata called (handled by InitDB)")
}

func GetBranchesMetadata() *BranchMetadata {
	Debug("Reading branches metadata from DB")
	return &BranchMetadata{
		Default: DBGetDefaultBranchName(),
		Current: DBGetCurrentBranchName(),
	}
}

func SetBranch(branch string, configParam string) error {
	return DBSetBranch(branch, configParam)
}

func ListBranches() []string {
	return DBListBranches()
}

// BranchMetadata is kept for compatibility with config.go
type BranchMetadata struct {
	Default string `json:"default"`
	Current string `json:"current"`
}
