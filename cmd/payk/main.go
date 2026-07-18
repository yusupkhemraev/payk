// Command payk is a keyboard-first TUI HTTP client.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/yusupkhemraev/payk/internal/tui"
)

// version is stamped by goreleaser via -ldflags.
var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("payk " + version)
		return
	}

	program := tea.NewProgram(tui.New())
	if _, err := program.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "payk:", err)
		os.Exit(1)
	}
}
