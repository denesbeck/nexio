package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pterm/pterm"
	"github.com/pterm/pterm/putils"
)

func Success(content string) {
	contentStyle := pterm.NewStyle(pterm.Bold)
	iconStyle := pterm.NewStyle(pterm.FgLightGreen, pterm.Bold)
	contentStyle.Println(iconStyle.Sprint("> ") + content + "  ")
}

func Info(content string) {
	contentStyle := pterm.NewStyle(pterm.Bold)
	iconStyle := pterm.NewStyle(pterm.FgLightBlue, pterm.Bold)
	contentStyle.Println(iconStyle.Sprint("> ") + content + "  ")
}

func Warning(content string) {
	contentStyle := pterm.NewStyle(pterm.Bold)
	iconStyle := pterm.NewStyle(pterm.FgLightYellow, pterm.Bold)
	contentStyle.Println(iconStyle.Sprint("> ") + content + "  ")
}

func Fail(content string) {
	contentStyle := pterm.NewStyle(pterm.Bold)
	iconStyle := pterm.NewStyle(pterm.FgLightRed, pterm.Bold)
	contentStyle.Println(iconStyle.Sprint("> ") + content + "  ")
}

func Spinner(labels []string, showTimer bool) func() {
	multi, _ := pterm.DefaultMultiPrinter.Start()

	successPrinter := pterm.PrefixPrinter{
		Prefix: pterm.Prefix{
			Text:  "✓",
			Style: pterm.NewStyle(pterm.FgGreen),
		},
	}

	spinners := make([]*pterm.SpinnerPrinter, 0, len(labels))

	for _, label := range labels {
		spinner, _ := pterm.DefaultSpinner.
			WithSequence(" ⣾ ", " ⣽ ", " ⣻ ", " ⢿ ", " ⡿ ", " ⣟ ", " ⣯ ", " ⣷ ").
			WithStyle(pterm.NewStyle(pterm.FgCyan)).
			WithShowTimer(showTimer).
			WithWriter(multi.NewWriter()).
			Start(label)

		spinner.SuccessPrinter = &successPrinter

		spinners = append(spinners, spinner)
	}

	return func() {
		for i, s := range spinners {
			s.Success(labels[i])
		}
		multi.Stop()
	}
}

func Text(content string, icon string) {
	if icon == "" {
		pterm.Println(content)
		return
	}
	iconStyle := pterm.NewStyle(pterm.FgLightBlue)
	pterm.Println(iconStyle.Sprint(icon) + "  " + content)
}

func BreakLine() {
	pterm.Println()
}

func List(rootNode string, list []string, ordered bool) {
	style := pterm.NewStyle(pterm.Bold)
	style.Println(rootNode)
	for i, item := range list {
		var prefix string
		if ordered {
			prefix = fmt.Sprintf("%d. ", i+1)
		} else {
			prefix = "  "
		}
		fmt.Println("  " + prefix + item)
	}
}

func TreeList(files []string, sorted bool) {
	if len(files) == 0 {
		return
	}

	if len(files) == 1 {
		pterm.Println("  └── " + files[0])
		return
	}

	sortedFiles := make([]string, len(files))
	copy(sortedFiles, files)

	if sorted {
		sort.Strings(sortedFiles)
	}

	for i, file := range sortedFiles {
		if i == len(sortedFiles)-1 {
			pterm.Println("  └── " + file)
		} else {
			pterm.Println("  ├── " + file)
		}
	}
}

func Tree(list pterm.LeveledList, rootText string) {
	root := putils.TreeFromLeveledList(list)
	root.Text = rootText

	pterm.DefaultTree.WithRoot(root).Render()
}

func StyledBranch(branch string) string {
	style := pterm.NewStyle(pterm.FgLightYellow)
	return style.Sprint(branch)
}

func StyledCommit(commit string) string {
	style := pterm.NewStyle(pterm.FgLightRed)
	return style.Sprint(commit)
}

func Code(code string) string {
	style := pterm.NewStyle(pterm.Reset, pterm.FgCyan)
	return style.Sprint(code)
}

func ErrorMsg(msg string) string {
	style := pterm.NewStyle(pterm.Reset, pterm.FgRed)
	return style.Sprint(msg)
}

func Bold(content string) string {
	style := pterm.NewStyle(pterm.Bold)
	return style.Sprint(content)
}

func Box(title string, content string) {
	box := pterm.DefaultBox.
		WithBoxStyle(pterm.NewStyle(pterm.FgLightWhite)).
		WithHorizontalString("─").
		WithVerticalString("│").
		WithTopPadding(0).
		WithBottomPadding(0).
		WithLeftPadding(2).
		WithRightPadding(2)

	if title == "" {
		box.Print(content)
	} else {
		box.WithTitle(title).Print(content)
	}
}

func GenerateLeveledList(files []string) pterm.LeveledList {
	list := pterm.LeveledList{}
	seen := make(map[string]bool)

	for _, file := range files {
		parts := strings.Split(filepath.ToSlash(file), "/")
		currentPath := ""

		for index, part := range parts {
			if index > 0 {
				currentPath += "/"
			}
			currentPath += part

			if seen[currentPath] {
				continue
			}
			seen[currentPath] = true

			list = append(list, pterm.LeveledListItem{
				Level: index,
				Text:  part,
			})
		}
	}

	return list
}
