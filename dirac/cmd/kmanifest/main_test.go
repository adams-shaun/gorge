package main

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cli"
	"github.com/adams-shaun/gorge/dirac/manifest"
)

// fixtureRoot copies dirac/manifest's fixture tree into a temporary module
// root.
func fixtureRoot(t *testing.T) string {
	t.Helper()
	dst := t.TempDir()
	src := os.DirFS(filepath.Join("..", "..", "manifest", "testdata", "tree"))
	err := fs.WalkDir(src, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		out := filepath.Join(dst, filepath.FromSlash(p))
		if d.IsDir() {
			return os.MkdirAll(out, 0o755)
		}
		b, err := fs.ReadFile(src, p)
		if err != nil {
			return err
		}
		return os.WriteFile(out, b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

func run(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := cli.Execute(newRoot(), args, nil, &out, &errb)
	return code, out.String(), errb.String()
}

func TestGen(t *testing.T) {
	root := fixtureRoot(t)
	code, out, errs := run(t, "gen", "--root", root)
	if code != 0 {
		t.Fatalf("gen: exit %d: %s", code, errs)
	}
	want := "wrote " + strings.Join(manifest.GenFiles, "\nwrote ") + "\n"
	if out != want {
		t.Errorf("gen printed %q, want %q", out, want)
	}
	if code, out, _ = run(t, "gen", "--root", root); code != 0 || out != "" {
		t.Errorf("second gen: exit %d, printed %q; want a silent no-op", code, out)
	}
	if code, out, _ = run(t, "gen", "--check", "--root", root); code != 0 || out != "" {
		t.Errorf("gen --check on a fresh tree: exit %d, printed %q", code, out)
	}
	if err := os.WriteFile(filepath.Join(root, "dirac", "manifest", "ExtOp", "ExtBar"),
		[]byte("family: ExtOp\nname: ExtBar\nvalue: 9\nstatus: active\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, out, _ = run(t, "gen", "--check", "--root", root); code != 1 || out != "stale "+manifest.KopGen+"\n" {
		t.Errorf("gen --check on a stale tree: exit %d, printed %q", code, out)
	}
}

func TestGenRefusesAnInvalidTree(t *testing.T) {
	root := fixtureRoot(t)
	if err := os.WriteFile(filepath.Join(root, "dirac", "manifest", "Op", "OpHot"),
		[]byte("family: Op\nname: OpHot\nvalue: next\nstatus: active\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, errs := run(t, "gen", "--root", root)
	if code != 1 || !strings.Contains(errs, "add an approval line") {
		t.Errorf("gen on an unapproved hot op: exit %d, stderr %q", code, errs)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(manifest.KopGen))); err == nil {
		t.Errorf("gen wrote %s from an invalid tree", manifest.KopGen)
	}
}
