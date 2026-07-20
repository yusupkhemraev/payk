// Command payk is a keyboard-first TUI HTTP client.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/debug"

	tea "charm.land/bubbletea/v2"

	"github.com/yusupkhemraev/payk/internal/tui"
)

// version is stamped by goreleaser via -ldflags; go install builds fall
// back to the module version from build info.
var version = "dev"

func resolveVersion() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok &&
		info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return version
}

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("payk " + resolveVersion())
		return
	}

	program := tea.NewProgram(tui.New(tui.Config{}))
	if _, err := program.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "payk:", err)
		os.Exit(1)
	}
}
