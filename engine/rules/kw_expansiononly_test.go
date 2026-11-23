package rules

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// TestKeywordRulesReadsComplete ratchets cards.keywordRulesReads (through its
// only consumer, cards.KeywordExpansionOnly) against this tree's sources. It
// parses every non-test .go file under engine/rules and engine/effects and
// collects every string literal whose keyword head names a registered
// expander head, case-insensitively, as cards.KeywordHead does (an
// "Equip:2"-shaped literal compares by its head). It fails in both
// directions:
//
//   - a head found named in code yet answered expansion-only -- a new rules
//     read of an expansion-only head would slip past dirac's generator, which
//     models an expansion-only head as exactly its expansion lines;
//   - a registered head answered NOT expansion-only though no code names it --
//     a stale keywordRulesReads entry blocking a head from passing through.
//
// The with<Keyword> filter atoms (engine/effects/filter.go, the with<K> loop)
// build their names by concatenation, so this scanner never sees them as
// literal heads. They are filter words, not keyword-head reads, and dirac
// handles them on its side (T13b); a head reaching that loop only through a
// filter word stays expansion-only.
func TestKeywordRulesReadsComplete(t *testing.T) {
	heads := registeredExpanderHeads(t)
	if len(heads) == 0 {
		t.Fatal("no registered expander heads recovered from engine/cards; scanner premise gone")
	}
	lowerToHead := map[string]string{}
	for _, h := range heads {
		lowerToHead[strings.ToLower(h)] = h
	}
	found := map[string][]string{} // lower(head) -> file:line sites
	for _, dir := range []string{".", "../effects"} {
		for head, sites := range scanKeywordHeadLiterals(t, dir, lowerToHead) {
			found[head] = append(found[head], sites...)
		}
	}

	var namedButExpansionOnly, blockedButNeverNamed []string
	for head, sites := range found {
		sort.Strings(sites)
		if cards.KeywordExpansionOnly(lowerToHead[head]) {
			namedButExpansionOnly = append(namedButExpansionOnly, head+" ("+strings.Join(sites, ", ")+")")
		}
	}
	for _, head := range heads {
		if len(found[strings.ToLower(head)]) == 0 && !cards.KeywordExpansionOnly(head) {
			blockedButNeverNamed = append(blockedButNeverNamed, head)
		}
	}
	sort.Strings(namedButExpansionOnly)
	sort.Strings(blockedButNeverNamed)
	if len(namedButExpansionOnly) > 0 {
		t.Errorf("heads named in engine/rules or engine/effects but answered expansion-only:\n\t%s",
			strings.Join(namedButExpansionOnly, "\n\t"))
	}
	if len(blockedButNeverNamed) > 0 {
		t.Errorf("registered heads answered NOT expansion-only though no code names them:\n\t%s",
			strings.Join(blockedButNeverNamed, "\n\t"))
	}
}

// registeredExpanderHeads recovers the registered keyword heads from the
// engine/cards sources: it parses every non-test .go file there and collects
// the string literals handed to registerKeyword, so the head set tracks the
// registry without a second hand-maintained list.
func registeredExpanderHeads(t *testing.T) []string {
	t.Helper()
	var heads []string
	forEachParsedFile(t, "../cards", func(fset *token.FileSet, f *ast.File) {
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			id, ok := call.Fun.(*ast.Ident)
			if !ok || id.Name != "registerKeyword" {
				return true
			}
			for _, arg := range call.Args {
				if s, ok := unquoteLit(arg); ok {
					heads = append(heads, s)
				}
			}
			return true
		})
	})
	return heads
}

// scanKeywordHeadLiterals parses every non-test .go file under dir (relative
// to the package dir this test runs in) and maps each literal that names a
// registered expander head, case-insensitively, to its file:line sites.
func scanKeywordHeadLiterals(t *testing.T, dir string, lowerToHead map[string]string) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	forEachParsedFile(t, dir, func(fset *token.FileSet, f *ast.File) {
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			s, err := strconv.Unquote(lit.Value)
			if err != nil {
				return true
			}
			head := lowerToHead[strings.ToLower(cards.KeywordHead(s))]
			if head == "" {
				return true
			}
			key := strings.ToLower(head)
			pos := fset.Position(lit.Pos())
			out[key] = append(out[key], pos.Filename+":"+strconv.Itoa(pos.Line))
			return true
		})
	})
	return out
}

// forEachParsedFile walks dir (relative to the package dir a test runs in)
// and parses each non-test .go file, skipping testdata trees.
func forEachParsedFile(t *testing.T, dir string, visit func(*token.FileSet, *ast.File)) {
	t.Helper()
	fset := token.NewFileSet()
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return perr
		}
		visit(fset, f)
		return nil
	})
	if err != nil {
		t.Fatalf("parsing %s: %v", dir, err)
	}
}

// unquoteLit reports whether e is a string literal, and its unquoted text.
func unquoteLit(e ast.Expr) (string, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return s, true
}
