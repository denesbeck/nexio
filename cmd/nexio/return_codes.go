package main

var COMMON_RETURN_CODES = map[int]string{
	001: "Nexio not initialized.",
	002: "Path ignored by one of the rules defined in the rules file.",
	003: "Nexio already initialized.",
	004: "Invalid path.",
	005: "Unexpected error.",
}

var STAGE_RETURN_CODES = map[int]string{
	100: "Fallback RC (should not occur).",
	101: "File no longer exists, removed from staging.",            // file was staged (ADD), but it got removed
	102: "Staged file updated.",                                    // file was staged (ADD), but it got modified
	103: "File already staged.",                                    // the user staged the same file again (ADD)
	104: "File no longer exists, removed from staging.",            // file was staged (MOD), but it got removed
	105: "Staged file updated.",                                    // file was staged (MOD), but it got modified
	106: "File already staged.",                                    // the user staged the same file again (MOD)
	107: "Staged file updated.",                                    // file was staged (REM), but it got added back and modified
	108: "File already staged.",                                    // the user staged the same file again (REM)
	109: "File added to staging.",                                  // file committed but not staged -> staged (REM)
	110: "File added to staging.",                                  // file committed but not staged -> staged (MOD)
	111: "File not modified.",                                      // file committed, not staged, not modified
	112: "File added to staging.",                                  // file not committed, not staged -> staged (ADD)
	113: "File restored to committed state, removed from staging.", // file was staged (REM), but it got added back without modifications
}

var BRANCH_RETURN_CODES = map[int]string{
	201: "Invalid branch name.",
	202: "Cannot create branch from both commit and branch.",
	203: "Source branch does not exist.",
	204: "Failed to create branch from commit.",
	205: "Branch already exists.",
	206: "Branch created successfully.",
	207: "Branch does not exist.", // drop
	208: "Cannot delete current branch.",
	209: "Cannot delete default branch.",
	210: "Branch deleted successfully.",
	211: "Already on target branch.",                        // switch
	212: "Branch does not exist.",                           // switch
	213: "Current branch: ",                                 // switch
	214: "Cannot switch branches with uncommitted changes.", // switch
	215: "Target branch already set as default.",            // config
	216: "Branch does not exist.",                           // config
	217: "No branches found. .nexio folder seems to be corrupted!",
}

var WORKDIR_RETURN_CODES = map[int]string{
	301: "Success!",
	302: "No commits found.",
}

var HISTORY_RETURN_CODES = map[int]string{
	401: "Success!",
	402: "No commits found.",
}

var STATUS_RETURN_CODES = map[int]string{
	501: "No files staged for commit.",
	502: "Get status success.",
	503: "No tracked files have been modified or deleted.",
	504: "No untracked files.",
	505: "Nothing to commit, working tree clean.",
}

var CONFIG_RETURN_CODES = map[int]string{
	601: "Get default branch success.",
	602: "Set default branch success.",
	603: "Set config success.",
	604: "Get config success.",
	605: "Name not set.",
	606: "Email not set.",
	607: "Name and/or email not set.",
	608: "Remote not set.",
}

var REMOTE_RETURN_CODES = map[int]string{
	1101: "Push completed successfully.",
	1102: "Pull completed successfully.",
	1103: "Clone completed successfully.",
	1104: "No remote configured.",
	1105: "Remote is locked.",
	1106: "Remote has commits not present locally.",
	1107: "Local history has diverged from remote.",
	1108: "Uncommitted staged changes exist.",
}

var COMMIT_RETURN_CODES = map[int]string{
	701: "Nothing to commit.",
	702: "Commit registered successfully.",
}

var UNSTAGE_RETURN_CODES = map[int]string{
	801: "File removed from staging.",
	802: "File was not staged.",
	803: "Failed to remove file from staging.",
}

var PURGE_RETURN_CODES = map[int]string{
	901: "Nexio purged successfully.",
	902: "Cancelled.",
}

var CLEAN_RETURN_CODES = map[int]string{
	1001: "Nexio cleaned successfully.",
	1002: "Cancelled.",
}
