package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

// parseFlags parses args into fs. Help requests (-h, -help, --help) print the
// flag usage to stdout and report help=true; parse errors are written to
// stderr. Callers return 0 for help and 2 for any other error.
func parseFlags(fs *flag.FlagSet, usageLine string, args []string) (help bool, err error) {
	var buf bytes.Buffer
	fs.SetOutput(&buf)
	fs.Usage = func() {
		fmt.Fprintf(&buf, "Usage: %s\n\nFlags:\n", usageLine)
		fs.PrintDefaults()
	}
	err = fs.Parse(args)
	switch {
	case err == nil:
		return false, nil
	case errors.Is(err, flag.ErrHelp):
		_, _ = buf.WriteTo(os.Stdout)
		return true, nil
	default:
		_, _ = buf.WriteTo(os.Stderr)
		return false, err
	}
}

func runRender(args []string) int {
	fs := flag.NewFlagSet("render", flag.ContinueOnError)
	templatePath := fs.String("template", "", "path to a Go text template")
	varsPath := fs.String("vars", "", "path to a JSON object or array of objects")
	outPath := fs.String("out", "", "optional output file")

	if help, err := parseFlags(fs, "promptdeck render -template FILE -vars FILE [-out FILE]", args); err != nil {
		return 2
	} else if help {
		return 0
	}
	if *templatePath == "" || *varsPath == "" {
		fmt.Fprintln(os.Stderr, "both -template and -vars are required")
		return 2
	}

	body, err := renderWithVars(*templatePath, *varsPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if *outPath != "" {
		if err := os.WriteFile(*outPath, []byte(body), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}

	fmt.Print(body)
	return 0
}

func runMatrix(args []string) int {
	fs := flag.NewFlagSet("matrix", flag.ContinueOnError)
	templatePath := fs.String("template", "", "path to a Go text template")
	matrixPath := fs.String("matrix", "", "path to a JSON object of string arrays")
	outDir := fs.String("out-dir", "", "optional output directory for rendered prompts")
	ext := fs.String("ext", ".txt", "output extension when -out-dir is provided")
	manifestPath := fs.String("manifest", "", "optional output manifest path")

	if help, err := parseFlags(fs, "promptdeck matrix -template FILE -matrix FILE [-out-dir DIR] [-ext EXT] [-manifest FILE]", args); err != nil {
		return 2
	} else if help {
		return 0
	}
	if *templatePath == "" || *matrixPath == "" {
		fmt.Fprintln(os.Stderr, "both -template and -matrix are required")
		return 2
	}

	candidates, err := buildCandidates(*templatePath, *matrixPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	if *manifestPath != "" {
		body, err := json.MarshalIndent(candidates, "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if err := os.WriteFile(*manifestPath, append(body, '\n'), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}

	if *outDir == "" {
		for i, prompt := range promptsFromCandidates(candidates) {
			if i > 0 {
				fmt.Println("\n---")
			}
			fmt.Print(prompt)
		}
		return 0
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	for _, candidate := range candidates {
		name := fmt.Sprintf("%s%s", candidate.Index, *ext)
		if err := os.WriteFile(filepath.Join(*outDir, name), []byte(candidate.Prompt), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	fmt.Printf("wrote %d prompts to %s\n", len(candidates), *outDir)
	return 0
}

func runOptimize(args []string) int {
	fs := flag.NewFlagSet("optimize", flag.ContinueOnError)
	templatePath := fs.String("template", "", "path to a Go text template")
	matrixPath := fs.String("matrix", "", "path to a JSON object of string arrays")
	scoresPath := fs.String("scores", "", "path to a JSON array of prompt scores")
	topN := fs.Int("top", 3, "number of ranked prompts to return")
	outPath := fs.String("out", "", "optional path to write the best prompt")
	jsonOutput := fs.Bool("json", false, "emit machine-readable JSON")

	if help, err := parseFlags(fs, "promptdeck optimize -template FILE -matrix FILE -scores FILE [-top N] [-out FILE] [-json]", args); err != nil {
		return 2
	} else if help {
		return 0
	}
	if *templatePath == "" || *matrixPath == "" || *scoresPath == "" {
		fmt.Fprintln(os.Stderr, "template, matrix, and scores are required")
		return 2
	}

	candidates, err := buildCandidates(*templatePath, *matrixPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	scores, err := loadScores(*scoresPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	report := optimize(candidates, scores, *topN)
	if report.Best != nil && *outPath != "" {
		if err := os.WriteFile(*outPath, []byte(report.Best.Prompt), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}

	if *jsonOutput {
		body, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Println(string(body))
		return 0
	}

	printOptimizationReport(report)
	return 0
}
