package scripts

// TestCorpusKeysDoNotDependOnLocale pins the invariant that the corpus/tree keys (ck, dk, tree) are a pure
// function of committed content, not of the box's locale. 2026-11-27: the owned box (en_US.UTF-8) and spotci (C) logged
// different keys for the same merge-queue commit (ck ...c9935cefe9a1 vs ...3e44c20a36e0) because ci-corpus-keys.sh's
// `sort -z` and table-key-files.sh's `sort -u` collated the same bytes two ways; kgen's published table key
// (byte-order over the embeds) was identical, proving the content equal and only the ordering different. Running
// the key script under a byte-order (C) and a reordering (en_US.UTF-8) locale must print byte-identical keys.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCorpusKeysDoNotDependOnLocale(t *testing.T) {
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	run := func(locale string) string {
		cmd := exec.Command("bash", "scripts/ci-corpus-keys.sh")
		cmd.Dir = root
		env := make([]string, 0, len(os.Environ())+2)
		for _, e := range os.Environ() {
			if strings.HasPrefix(e, "LC_") || strings.HasPrefix(e, "LANG=") {
				continue // set below, so the ambient locale cannot mask the test
			}
			env = append(env, e)
		}
		cmd.Env = append(env, "LC_ALL="+locale, "LANG="+locale)
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("ci-corpus-keys.sh under LC_ALL=%s: %v", locale, err)
		}
		return string(out)
	}
	line := func(s, k string) string {
		for _, l := range strings.Split(s, "\n") {
			if strings.HasPrefix(l, k+"=") {
				return l
			}
		}
		return k + "=<missing>"
	}
	c, en := run("C"), run("en_US.UTF-8")
	for _, k := range []string{"ck", "dk", "tree"} {
		if line(c, k) != line(en, k) {
			t.Errorf("%s depends on the locale: %q (C) != %q (en_US.UTF-8); the key must be a function of content", k, line(c, k), line(en, k))
		}
	}
}
