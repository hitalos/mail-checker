package main

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

var progressFormat = ternary(term.IsTerminal(int(os.Stdout.Fd())), "[\x1b[31m%s%s\033[m]", "[%s%s]")

func progress(current, total int) {
	columns, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || columns < 40 {
		columns = 80
	}
	barSize := columns - 32
	done := current * barSize / total
	rest := barSize - done
	percent := float64(current) / float64(total) * 100
	bar := fmt.Sprintf(progressFormat, strings.Repeat("#", done), strings.Repeat(" ", rest))
	fmt.Printf("\rProgress: %s %.2f%% (%d/%d)", bar, percent, current, total)
}

func ternary(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}
