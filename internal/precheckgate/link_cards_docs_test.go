package precheckgate

// Nothing a contributor or an agent reads may tell them to link the main checkout's .cards into a worktree
// (`ln -s <main>/.cards .cards`): that corpus sat on an older FORGE_REF than main (2026-11-25), so tests, redcheck and a
// bug hunt ran on a stale corpus. scripts/link-cards.sh links the store's corpus of THIS tree's own FORGE_REF.

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var linkMainCards = regexp.MustCompile("ln -s[a-z]* [^\n`]*\\.cards")

func TestDocsLinkCorpusByRef(t *testing.T) {
	root := filepath.Join("..", "..")
	files := []string{"AGENTS.md", filepath.Join("scripts", "test-targets.sh")}
	for _, dir := range []string{filepath.Join("docs", "agents"), filepath.Join("docs", "site")} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(p string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() && strings.HasSuffix(p, ".md") {
				rel, _ := filepath.Rel(root, p)
				files = append(files, rel)
			}
			return err
		})
		if err != nil {
			t.Fatalf("walk %s: %v", dir, err)
		}
	}
	for _, f := range files {
		b, err := os.ReadFile(filepath.Join(root, f))
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		if m := linkMainCards.Find(b); m != nil {
			t.Errorf("%s says %q: tell the reader to run scripts/link-cards.sh (the corpus of this tree's own FORGE_REF), not to link a shared .cards", f, m)
		}
	}
}
