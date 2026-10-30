package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These tests swap os.Stdout/os.Stderr, so none of them call t.Parallel.

func runCapturing(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	oldOut, oldErr := os.Stdout, os.Stderr
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout, os.Stderr = outW, errW
	defer func() { os.Stdout, os.Stderr = oldOut, oldErr }()

	code = Run(args)

	os.Stdout, os.Stderr = oldOut, oldErr
	_ = outW.Close()
	_ = errW.Close()
	read := func(f *os.File) string {
		var sb strings.Builder
		buf := make([]byte, 4096)
		for {
			n, err := f.Read(buf)
			sb.Write(buf[:n])
			if err != nil {
				break
			}
		}
		_ = f.Close()
		return sb.String()
	}
	return code, read(outR), read(errR)
}

func TestHelpAndVersionExitZero(t *testing.T) {
	cases := [][]string{
		{"-h"}, {"-help"}, {"--help"}, {"help"}, {"help", "-h"}, {"help", "--help"},
		{"help", "render"}, {"help", "matrix"}, {"help", "optimize"}, {"help", "tui"}, {"help", "interactive"},
		{"render", "-h"}, {"render", "-help"}, {"render", "--help"}, {"render", "help"},
		{"matrix", "-h"}, {"matrix", "-help"}, {"matrix", "--help"}, {"matrix", "help"},
		{"optimize", "-h"}, {"optimize", "-help"}, {"optimize", "--help"}, {"optimize", "help"},
		{"tui", "-h"}, {"tui", "--help"}, {"tui", "help"}, {"interactive", "--help"}, {"interactive", "help"},
		{"version"}, {"-version"}, {"--version"},
		{"render", "version"}, {"render", "-version"}, {"render", "--version"},
		{"matrix", "version"}, {"matrix", "-version"}, {"matrix", "--version"},
		{"optimize", "version"}, {"optimize", "-version"}, {"optimize", "--version"},
		{"tui", "version"}, {"tui", "--version"}, {"interactive", "--version"},
	}
	for _, args := range cases {
		code, stdout, stderr := runCapturing(t, args...)
		if code != 0 {
			t.Errorf("%v: exit code = %d, want 0 (stderr %q)", args, code, stderr)
		}
		if strings.TrimSpace(stdout) == "" {
			t.Errorf("%v: expected output on stdout", args)
		}
		if stderr != "" {
			t.Errorf("%v: unexpected stderr %q", args, stderr)
		}
	}
}

func TestHelpContent(t *testing.T) {
	cases := []struct {
		args []string
		want []string
	}{
		{[]string{"--help"}, []string{"promptdeck render", "promptdeck matrix", "promptdeck optimize", "must not"}},
		{[]string{"help", "render"}, []string{"Usage: promptdeck render", "-template", "-vars", "-out"}},
		{[]string{"matrix", "-h"}, []string{"Usage: promptdeck matrix", "-matrix", "-out-dir", "-ext", "-manifest"}},
		{[]string{"optimize", "--help"}, []string{"Usage: promptdeck optimize", "-scores", "-top", "-json"}},
		{[]string{"tui", "--help"}, []string{"Usage: promptdeck [tui]", "terminal"}},
	}
	for _, tc := range cases {
		_, stdout, _ := runCapturing(t, tc.args...)
		for _, want := range tc.want {
			if !strings.Contains(stdout, want) {
				t.Errorf("%v: help is missing %q:\n%s", tc.args, want, stdout)
			}
		}
	}
}

func TestVersionOutput(t *testing.T) {
	_, stdout, _ := runCapturing(t, "version")
	if !strings.HasPrefix(stdout, "promptdeck ") || strings.Count(stdout, "\n") != 1 {
		t.Errorf("version output = %q", stdout)
	}

	old := Version
	Version = "1.2.3"
	defer func() { Version = old }()
	for _, args := range [][]string{{"--version"}, {"version"}, {"matrix", "--version"}, {"optimize", "version"}} {
		if _, stdout, _ := runCapturing(t, args...); strings.TrimSpace(stdout) != "promptdeck 1.2.3" {
			t.Errorf("%v: version output = %q", args, stdout)
		}
	}
}

func TestSubcommandHelpSpellingsAgree(t *testing.T) {
	for _, name := range []string{"render", "matrix", "optimize"} {
		_, viaFlag, _ := runCapturing(t, name, "-h")
		for _, args := range [][]string{{name, "--help"}, {name, "help"}, {"help", name}} {
			_, got, _ := runCapturing(t, args...)
			if viaFlag == "" || viaFlag != got {
				t.Errorf("%v differs from %s -h:\n%q\n%q", args, name, got, viaFlag)
			}
		}
	}
	_, top, _ := runCapturing(t, "help")
	for _, args := range [][]string{{"-h"}, {"--help"}, {"help", "--help"}, {"help", "-h"}} {
		if code, got, _ := runCapturing(t, args...); code != 0 || got != top {
			t.Errorf("%v: code %d, differs from top-level help", args, code)
		}
	}
}

func TestHelpWordAsFlagValueIsNotHijacked(t *testing.T) {
	dir := t.TempDir()
	// A template file literally named "help" / "version" is a flag value, not a request.
	for _, name := range []string{"help", "version"} {
		tmpl := filepath.Join(dir, name)
		vars := filepath.Join(dir, "vars.json")
		if err := os.WriteFile(tmpl, []byte("Hello {{.name}}"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(vars, []byte(`{"name":"Deck"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		// Relative path so the flag value is exactly "help" / "version".
		wd, _ := os.Getwd()
		if err := os.Chdir(dir); err != nil {
			t.Fatal(err)
		}
		code, stdout, stderr := runCapturing(t, "render", "-template", name, "-vars", "vars.json")
		_ = os.Chdir(wd)
		if code != 0 || stdout != "Hello Deck" {
			t.Errorf("render -template %s: code %d stdout %q stderr %q", name, code, stdout, stderr)
		}
	}

	// A missing file named "help" is an ordinary failure (exit 1), not help output.
	code, stdout, stderr := runCapturing(t, "render", "-template", "help", "-vars", filepath.Join(dir, "nope.json"))
	if code != 1 || stdout != "" || stderr == "" {
		t.Errorf("render -template help: code %d stdout %q stderr %q, want 1, empty stdout, error on stderr", code, stdout, stderr)
	}
	// The word is only special directly after the subcommand.
	code, stdout, _ = runCapturing(t, "matrix", "-ext", "version", "-h")
	if code != 0 || !strings.Contains(stdout, "Usage: promptdeck matrix") {
		t.Errorf("matrix -ext version -h: code %d stdout %q", code, stdout)
	}
}

func TestRealErrorsKeepExitCodes(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	cases := []struct {
		args []string
		want int
	}{
		{[]string{"bogus"}, 2},
		{[]string{"-bogus"}, 2},
		{[]string{"help", "bogus"}, 2},
		{[]string{"render"}, 2},
		{[]string{"render", "-template", "x"}, 2},
		{[]string{"render", "-bogus"}, 2},
		{[]string{"matrix"}, 2},
		{[]string{"matrix", "-bogus"}, 2},
		{[]string{"optimize"}, 2},
		{[]string{"optimize", "-template", "x", "-matrix", "y"}, 2},
		{[]string{"optimize", "-top", "notanumber"}, 2},
		{[]string{"render", "-template", missing, "-vars", missing}, 1},
		{[]string{"matrix", "-template", missing, "-matrix", missing}, 1},
		{[]string{"optimize", "-template", missing, "-matrix", missing, "-scores", missing}, 1},
	}
	for _, tc := range cases {
		code, stdout, stderr := runCapturing(t, tc.args...)
		if code != tc.want {
			t.Errorf("%v: exit code = %d, want %d", tc.args, code, tc.want)
		}
		if stdout != "" {
			t.Errorf("%v: errors must not print to stdout, got %q", tc.args, stdout)
		}
		if stderr == "" {
			t.Errorf("%v: expected an error message on stderr", tc.args)
		}
	}
}

func TestUnknownSubcommandPrintsUsageToStderr(t *testing.T) {
	code, stdout, stderr := runCapturing(t, "bogus")
	if code != 2 || stdout != "" {
		t.Errorf("code %d stdout %q, want 2 and empty stdout", code, stdout)
	}
	if !strings.Contains(stderr, `unknown subcommand "bogus"`) || !strings.Contains(stderr, "Usage:") {
		t.Errorf("stderr = %q, want the error and usage", stderr)
	}
}
