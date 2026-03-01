package main

func DisplayUnstageResults(results []UnstageResult) {
	if len(results) == 0 {
		Debug("Results length is 0.")
		Info("No files to unstage.")
		return
	}

	var unstaged, notStaged, failed []string

	for _, r := range results {
		switch r.ReturnCode {
		case 801:
			unstaged = append(unstaged, r.FilePath)
		case 802:
			notStaged = append(notStaged, r.FilePath)
		case 803:
			failed = append(failed, r.FilePath)
		}
	}

	if len(unstaged)+len(notStaged)+len(failed) == 0 {
		Info("Nothing to unstage.")
		Debug("Nothing to unstage.")
		return
	}

	if len(unstaged) > 0 {
		BreakLine()
		Success("󰍷 Removed from staging %s", FormatFileCount(len(unstaged)))
		list := GenerateLeveledList(unstaged)
		Tree(list, ".", true)
	}

	if len(notStaged) > 0 {
		BreakLine()
		Info(" Not staged %s", FormatFileCount(len(notStaged)))
		list := GenerateLeveledList(notStaged)
		Tree(list, ".", true)
	}

	if len(failed) > 0 {
		BreakLine()
		Fail(" Failed %s", FormatFileCount(len(failed)))
		list := GenerateLeveledList(failed)
		Tree(list, ".", true)
	}
}
