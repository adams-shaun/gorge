package archtest

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var sourceImporter = regexp.MustCompile(`importer\.ForCompiler\([^,()]+,\s*"source"`)

// TestSourceImporterRecoversGOROOT holds every use of the "source" importer to
// the -trimpath fallback. CI builds with GOFLAGS=-trimpath (one Go cache per
// box), and a -trimpath binary reports no GOROOT, so build.Default.GOROOT is ""
// and the source importer cannot find the standard library ("$GOROOT not set").
// A file that type-checks through importer.ForCompiler(fset, "source", nil)
// sets build.Default.GOROOT from `go env GOROOT` when it is empty.
func TestSourceImporterRecoversGOROOT(t *testing.T) {
	root := filepath.Join("..", "..")
	out, err := exec.Command("git", "-C", root, "ls-files", "-z", "--cached", "--", "*.go").Output()
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	self := "internal/archtest/source_importer_goroot_test.go"
	for _, name := range strings.Split(strings.TrimRight(string(out), "\x00"), "\x00") {
		if name == "" || name == self {
			continue
		}
		b, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		if !sourceImporter.Match(b) {
			continue
		}
		if !bytes.Contains(b, []byte(`"env", "GOROOT"`)) {
			t.Errorf(`%s uses importer.ForCompiler(..., "source", ...) without the -trimpath GOROOT fallback: `+
				`if build.Default.GOROOT == "" { set it from exec.Command("go", "env", "GOROOT").Output() }`, name)
		}
	}
}
