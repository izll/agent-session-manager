package session

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// Nothing may name the multiplexer binary directly.
//
// There were 117 calls to exec.Command("tmux", …) spread over eight files. A
// Windows build compiled cleanly from that and then failed at every one of
// them, because tmux has no Windows build — the app needs psmux there, and
// there was no single place to say so.
//
// The rule is easy to break by accident: exec.Command("tmux", …) is the obvious
// thing to write, and nothing about it looks wrong.
func TestNothingCallsTmuxDirectly(t *testing.T) {
	direct := regexp.MustCompile(`exec\.Command(Context)?\([^)]*"tmux"`)

	root := ".."
	var offenders []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// Nothing to check in the build output or the module cache.
			if name := info.Name(); name == ".git" || name == "dist" || name == "test-builds" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		source, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		for i, line := range strings.Split(string(source), "\n") {
			// Comments may still describe the old form; only code counts.
			if strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}
			if direct.MatchString(line) {
				offenders = append(offenders, filepath.Base(path)+":"+strconv.Itoa(i+1))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the tree: %v", err)
	}

	if len(offenders) > 0 {
		t.Errorf("these call the multiplexer by name instead of through TmuxCommand, "+
			"so a Windows build cannot use psmux: %s", strings.Join(offenders, ", "))
	}
}

// The binary is swappable at runtime, for anyone running tmux from an unusual
// place — through WSL or Cygwin, or simply not on PATH.
func TestBinaryCanBeOverridden(t *testing.T) {
	original := TmuxBinary()
	defer SetTmuxBinary(original)

	SetTmuxBinary("/opt/custom/tmux")
	if got := TmuxBinary(); got != "/opt/custom/tmux" {
		t.Errorf("TmuxBinary() = %q after override; want /opt/custom/tmux", got)
	}
	if cmd := TmuxCommand("has-session"); cmd.Args[0] != "/opt/custom/tmux" {
		t.Errorf("TmuxCommand used %q; want the overridden binary", cmd.Args[0])
	}

	// An empty name restores the default rather than leaving the app unable to
	// find any binary at all — a cleared setting must not be a broken one.
	SetTmuxBinary("")
	if got := TmuxBinary(); got != defaultTmuxBinary {
		t.Errorf("TmuxBinary() = %q after clearing; want the default %q", got, defaultTmuxBinary)
	}
}

// The default differs per platform, and that is the entire point of the
// indirection: everywhere else it is tmux, on Windows it is psmux.
func TestDefaultIsThePlatformsMultiplexer(t *testing.T) {
	if defaultTmuxBinary == "" {
		t.Fatal("no default multiplexer binary for this platform")
	}
	t.Logf("default multiplexer on this platform: %q", defaultTmuxBinary)
}

// An empty override must not be treated as a request for a binary named "".
//
// main passes os.Getenv("ASMGR_TMUX") straight in, which is empty for everyone
// who has not set it — by far the common case. Left unguarded that would clear
// the platform default and leave the app looking for a program with no name.
func TestEmptyOverrideKeepsTheDefault(t *testing.T) {
	original := TmuxBinary()
	defer SetTmuxBinary(original)

	SetTmuxBinary("psmux")
	SetTmuxBinary("") // as an unset ASMGR_TMUX arrives

	if got := TmuxBinary(); got != defaultTmuxBinary {
		t.Errorf("TmuxBinary() = %q after an empty override; want %q", got, defaultTmuxBinary)
	}
}
