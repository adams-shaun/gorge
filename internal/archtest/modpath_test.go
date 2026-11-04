package archtest

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// moduleFileNameOK reports whether one path element is a file name the Go
// module zip accepts (golang.org/x/mod/module's fileNameOK): ASCII letters and
// digits, the punctuation below, or a non-ASCII letter. An apostrophe, a quote,
// a backquote, a colon or a backslash anywhere in a tracked path makes `go get`
// of this module fail ("malformed file path"), which stops every importer.
func moduleFileNameOK(elem string) bool {
	if elem == "" {
		return false
	}
	for _, r := range elem {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case strings.ContainsRune("!#$%&()+,-.=@[]^_{}~ ", r):
		case r > 0x7f:
		default:
			return false
		}
	}
	return true
}

// TestTrackedPathsAreModuleSafe keeps gorge importable as a Go module: every
// tracked path must survive the module zip's file-path check.
func TestTrackedPathsAreModuleSafe(t *testing.T) {
	cmd := exec.Command("git", "-C", filepath.Join("..", ".."), "ls-files", "-z", "--cached")
	cmd.Env = cards.GitEnv()
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	for _, p := range strings.Split(strings.TrimRight(string(out), "\x00"), "\x00") {
		for _, elem := range strings.Split(p, "/") {
			if !moduleFileNameOK(elem) {
				t.Errorf("%s: %q is not a file name a Go module zip accepts, so `go get` of gorge fails for every importer; rename it", p, elem)
				break
			}
		}
	}
}

func TestModuleFileNameOK(t *testing.T) {
	for elem, want := range map[string]bool{
		"pins.0.json": true, "a-b_c~d+e": true, "victor_valgavoths_seneschal.0.json": true,
		"victor_valgavoth's_seneschal.0.json": false, `a"b`: false, "a`b": false, "a:b": false, `a\b`: false, "": false,
	} {
		if got := moduleFileNameOK(elem); got != want {
			t.Errorf("moduleFileNameOK(%q) = %v, want %v", elem, got, want)
		}
	}
}
