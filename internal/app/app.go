package app

import (
	"fmt"
	"os"
	"runtime/debug"
)

// Version is the release version. main sets it (GoReleaser injects
// main.version); "dev" falls back to the module version from build info.
var Version = "dev"

func Run(args []string) int {
	if len(args) == 0 {
		return runTUI()
	}

	// "help" and "version" directly after a subcommand behave like the flags.
	// Only args[1] is checked, so a flag value named help/version is untouched.
	if len(args) > 1 && isSubcommand(args[0]) {
		switch {
		case args[1] == "help":
			return runHelp([]string{args[0]})
		case isVersionArg(args[1]):
			return runVersion()
		}
	}

	switch args[0] {
	case "render":
		return runRender(args[1:])
	case "matrix":
		return runMatrix(args[1:])
	case "optimize":
		return runOptimize(args[1:])
	case "tui", "interactive":
		if len(args) > 1 && isHelpArg(args[1]) {
			return runHelp([]string{"tui"})
		}
		return runTUI()
	case "help", "-h", "-help", "--help":
		return runHelp(args[1:])
	case "version", "-version", "--version":
		return runVersion()
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q\n\n", args[0])
		fmt.Fprint(os.Stderr, usageText)
		return 2
	}
}

func isSubcommand(arg string) bool {
	switch arg {
	case "render", "matrix", "optimize", "tui", "interactive":
		return true
	}
	return false
}

// runHelp prints top-level usage, or the usage of the named subcommand.
func runHelp(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stdout, usageText)
		return 0
	}
	switch args[0] {
	case "render":
		return runRender([]string{"-h"})
	case "matrix":
		return runMatrix([]string{"-h"})
	case "optimize":
		return runOptimize([]string{"-h"})
	case "tui", "interactive":
		fmt.Fprint(os.Stdout, tuiUsageText)
		return 0
	case "help", "version", "-h", "-help", "--help":
		fmt.Fprint(os.Stdout, usageText)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q\n\n", args[0])
		fmt.Fprint(os.Stderr, usageText)
		return 2
	}
}

func runVersion() int {
	fmt.Fprintln(os.Stdout, "promptdeck "+versionString())
	return 0
}

func isVersionArg(arg string) bool {
	switch arg {
	case "version", "-version", "--version":
		return true
	}
	return false
}

func isHelpArg(arg string) bool {
	switch arg {
	case "-h", "-help", "--help":
		return true
	}
	return false
}

func versionString() string {
	if Version != "" && Version != "dev" {
		return Version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}

const usageText = `promptdeck renders reusable prompt templates and optimizes prompt experiments.

Usage:
  promptdeck render   -template FILE -vars FILE [-out FILE]
  promptdeck matrix   -template FILE -matrix FILE [-out-dir DIR] [-ext EXT] [-manifest FILE]
  promptdeck optimize -template FILE -matrix FILE -scores FILE [-top N] [-out FILE] [-json]
  promptdeck help [render|matrix|optimize|tui]
  promptdeck version

Examples:
  promptdeck render -template examples/review.tmpl -vars examples/vars.json
  promptdeck matrix -template examples/review.tmpl -matrix examples/matrix.json
  promptdeck optimize -template examples/review.tmpl -matrix examples/matrix.json -scores examples/scores.json

Notes:
  Running promptdeck with no arguments (or "tui") launches an interactive
  terminal UI. Scripts and coding agents must not do that: use render,
  matrix and optimize.

Exit codes: 0 success, 1 failure, 2 usage error.
`

const tuiUsageText = `Usage: promptdeck [tui]

Launches the interactive terminal UI. It needs a real terminal; scripts and
coding agents should use "promptdeck render", "matrix" and "optimize" instead.
`
