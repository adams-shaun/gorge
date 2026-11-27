package scripts

// TestTestTargetsBuildsDiracTablesWhenTheChangeFeedsTheKey: test-targets.sh builds dirac's tables when a test outside
// ./dirac needs them and the change set feeds the tables' key (scripts/table-key-files.sh's list = kgen's compile
// inputs, engine/cards' Go files, Makefile's FORGE_REF), but not when nothing that feeds the key moved. 11-26 #5551: a
// dirac/kgen change made TestCreatePodDiracSeatsFour in ./server/gorged die on the gate box with "no card tables for
// this binary: run make dirac-tables", because test-one only built the tables when PKG itself was under ./dirac.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The key files the stub scripts/table-key-files.sh lists: one directory directly keyed (dirac, engine/cards,
// engine/effects), a nested one collapsing to dirac, and a root-level file.
const stubTableKeyFiles = "dirac/state.go\ndirac/kgen/gen.go\nengine/cards/cards.go\nengine/effects/effects.go\ntablesrc.go\n"

func commitChange(t *testing.T, repo string) {
	t.Helper()
	sh(t, repo, "git", "add", "-A")
	sh(t, repo, "git", "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "-m", "change")
}

func tablesRig(t *testing.T) (*rig, string) {
	t.Helper()
	r := newRig(t)
	// The rig commits a case's change on its branch, and BASE=land would move with it (an empty diff); pin a base
	// branch at the initial commit and diff against that.
	sh(t, r.repo, "git", "branch", "base")
	kept := make([]string, 0, len(r.env))
	for _, kv := range r.env {
		if !strings.HasPrefix(kv, "BASE=") {
			kept = append(kept, kv)
		}
	}
	r.env = append(kept, "BASE=base")
	keyfiles := filepath.Join(r.repo, "keyfiles.txt")
	write(t, keyfiles, stubTableKeyFiles, 0o644)
	write(t, filepath.Join(r.repo, "scripts", "table-key-files.sh"),
		"#!/bin/sh\ncat \"$TABLE_KEY_FILES\"\n", 0o755)
	makelog := filepath.Join(filepath.Dir(r.repo), "make.log")
	write(t, filepath.Join(r.bin, "make"), "#!/bin/sh\nprintf '%s\\n' \"$*\" >>\"$MAKE_LOG\"\n", 0o755)
	// Local (the box is the caller's concern); one test-one of a non-dirac package.
	r.set("GORGE_REMOTE_OFF=1", "TABLE_KEY_FILES="+keyfiles, "MAKE_LOG="+makelog)
	return r, makelog
}

func ranDiracTables(t *testing.T, log string) bool {
	t.Helper()
	b, err := os.ReadFile(log)
	if err != nil {
		return false
	}
	return strings.Contains(string(b), "dirac-tables")
}

func TestTestTargetsBuildsDiracTablesWhenTheChangeFeedsTheKey(t *testing.T) {
	cases := []struct {
		name  string
		apply func(t *testing.T, r *rig)
		want  bool
	}{
		{"nothing changed", func(*testing.T, *rig) {}, false},
		{"dirac/kgen source", func(t *testing.T, r *rig) {
			write(t, filepath.Join(r.repo, "dirac", "kgen", "gen.go"), "package kgen\n", 0o644)
			commitChange(t, r.repo)
		}, true},
		{"dirac source", func(t *testing.T, r *rig) {
			write(t, filepath.Join(r.repo, "dirac", "state.go"), "package dirac\n", 0o644)
			commitChange(t, r.repo)
		}, true},
		{"engine/cards source", func(t *testing.T, r *rig) {
			write(t, filepath.Join(r.repo, "engine", "cards", "card.go"), "package cards\n", 0o644)
			commitChange(t, r.repo)
		}, true},
		{"engine/effects source", func(t *testing.T, r *rig) {
			write(t, filepath.Join(r.repo, "engine", "effects", "params", "params.go"), "package params\n", 0o644)
			commitChange(t, r.repo)
		}, true},
		{"root-level key file", func(t *testing.T, r *rig) {
			write(t, filepath.Join(r.repo, "tablesrc.go"), "package gorge\n", 0o644)
			commitChange(t, r.repo)
		}, true},
		{"uncommitted dirac source", func(t *testing.T, r *rig) {
			write(t, filepath.Join(r.repo, "dirac", "state.go"), "package dirac\n", 0o644)
		}, true},
		{"Makefile FORGE_REF", func(t *testing.T, r *rig) {
			write(t, filepath.Join(r.repo, "Makefile"), "FORGE_REF ?= 0123456789abcdef\n", 0o644)
			commitChange(t, r.repo)
		}, true},
		// Nothing that feeds the key moved: no tables build, and the gate stays as cheap as before.
		{"engine/rules source is not a kgen input", func(t *testing.T, r *rig) {
			write(t, filepath.Join(r.repo, "engine", "rules", "engine.go"), "package rules\n", 0o644)
			commitChange(t, r.repo)
		}, false},
		{"Makefile change that is not FORGE_REF", func(t *testing.T, r *rig) {
			write(t, filepath.Join(r.repo, "Makefile"), "FOO := 1\n", 0o644)
			commitChange(t, r.repo)
		}, false},
		{"server source", func(t *testing.T, r *rig) {
			write(t, filepath.Join(r.repo, "server", "gorged", "x.go"), "package gorged\n", 0o644)
			commitChange(t, r.repo)
		}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, log := tablesRig(t)
			c.apply(t, r)
			out, errs, code := r.run("one")
			if code != 0 || !strings.HasPrefix(verdictLines(t, out)[0], "test-one: PASS ") {
				t.Fatalf("exit %d\nstdout:\n%s\nstderr:\n%s", code, out, errs)
			}
			if got := ranDiracTables(t, log); got != c.want {
				// point at the exact decision: the make log records what dirac_tables() ran
				t.Errorf("built dirac tables = %v, want %v\nstdout:\n%s\nstderr:\n%s", got, c.want, out, errs)
			}
		})
	}
}
