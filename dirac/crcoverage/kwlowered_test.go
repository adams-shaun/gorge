package crcoverage_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TestLoweredKeywordRowsAreNotNA keeps the 702-keyword-abilities rows of the
// keyword heads the dirac kernel lowers honest. The rows are hand-written and
// an `n/a` row needs no code reference, so TestCRCoverageIsCurrent passes one
// forever even after kgen starts lowering the head. This test reads the kgen
// registries that say "kgen lowers this head" -- the string keys of the
// simpleKeywords / expandedKeywords map literals and the string index of every
// kwHandlers["X"] = ... assignment (parsed by AST, never imported: this
// package stays free of kernel imports) -- and fails when a head's only
// 702.N rows are still `n/a`. TestLoweredKeywordRowsMutation pins the check
// itself.

// crKgenRegistries parses the non-test .go files of ../kgen and returns every
// keyword head the registries hold, mapped to the registry names that hold it.
func crKgenRegistries(t *testing.T) map[string][]string {
	t.Helper()
	dir, err := filepath.Abs("../kgen")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "lower.go")); err != nil {
		t.Fatalf("kgen package not at %s: %v", dir, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	regs := map[string][]string{}
	seen := map[string]map[string]bool{}
	add := func(head, reg string) {
		if seen[head] == nil {
			seen[head] = map[string]bool{}
		}
		if seen[head][reg] {
			return
		}
		seen[head][reg] = true
		regs[head] = append(regs[head], reg)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Errorf("parse %s: %v", path, err)
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch d := n.(type) {
			case *ast.ValueSpec:
				if len(d.Values) != 1 || len(d.Names) != 1 {
					return true
				}
				reg := d.Names[0].Name
				if reg != "simpleKeywords" && reg != "expandedKeywords" {
					return true
				}
				lit, ok := d.Values[0].(*ast.CompositeLit)
				if !ok {
					return true
				}
				for _, elt := range lit.Elts {
					kv, ok := elt.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					if head, ok := crLitString(kv.Key); ok {
						add(head, reg)
					}
				}
			case *ast.AssignStmt:
				for _, lhs := range d.Lhs {
					idx, ok := lhs.(*ast.IndexExpr)
					if !ok {
						continue
					}
					if x, ok := idx.X.(*ast.Ident); !ok || x.Name != "kwHandlers" {
						continue
					}
					if head, ok := crLitString(idx.Index); ok {
						add(head, "kwHandlers")
					}
				}
			}
			return true
		})
	}
	return regs
}

// crLitString reads a string literal expression.
func crLitString(e ast.Expr) (string, bool) {
	bl, ok := e.(*ast.BasicLit)
	if !ok || bl.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(bl.Value)
	if err != nil {
		return "", false
	}
	return s, true
}

// loweredKeywordRule maps every lowered head that is a CR 702 keyword ability
// to its CR 702 number. The number must match a row of the 702 chapter table
// (crRuleCovers).
var loweredKeywordRule = map[string]string{
	// the simpleKeywords heads (KeywordMask bits), CR 702.2-702.21
	"Deathtouch": "702.2", "Defender": "702.3", "Double Strike": "702.4",
	"Enchant": "702.5", "Equip": "702.6", "First Strike": "702.7", "Flying": "702.9",
	"Haste": "702.10", "Hexproof": "702.11", "Indestructible": "702.12",
	"Lifelink": "702.15", "Protection from everything": "702.16", "Reach": "702.17",
	"Shroud": "702.18", "Trample": "702.19", "Vigilance": "702.20", "Ward": "702.21",
	// the expandedKeywords and kwHandlers heads
	"Affinity": "702.41", "Afterlife": "702.135", "Annihilator": "702.86",
	"Afflict": "702.130", "Backup": "702.165", "Battle cry": "702.91",
	"Bushido": "702.45", "Blitz": "702.152", "Champion": "702.72", "Cipher": "702.99", "Conspire": "702.78",
	"Cumulative upkeep": "702.24", "Demonstrate": "702.144", "Disturb": "702.146",
	"Echo": "702.30", "Embalm": "702.128", "Enlist": "702.154",
	"Eternalize": "702.129", "Exalted": "702.83", "Fabricate": "702.123",
	"Fading": "702.32", "Fear": "702.36", "Flanking": "702.25", "For Mirrodin": "702.159",
	"Graft": "702.58", "Hideaway": "702.75", "Job select": "702.182",
	"Level up": "702.87", "Living Weapon": "702.92", "Living metal": "702.161",
	"Menace": "702.111", "Mentor": "702.134", "Modular": "702.44",
	"More Than Meets the Eye": "702.162",
	"Outlast":                 "702.107", "Persist": "702.79", "Phasing": "702.26", "Prowess": "702.108",
	"Rampage": "702.23", "Ravenous": "702.156", "Replicate": "702.56", "Riot": "702.136",
	"Scavenge": "702.97", "Shadow": "702.28", "Soulbond": "702.95", "Soulshift": "702.46",
	"Split second": "702.61", "Squad": "702.157", "Sunburst": "702.47", "Transmute": "702.53",
	"Undying": "702.93", "Vanishing": "702.63", "Reinforce": "702.77",
}

// loweredNotKeywordRule holds lowered heads that are not a CR 702 keyword
// ability, with the one-line reason no 702.N row is owed.
var loweredNotKeywordRule = map[string]string{
	"etbCounter":     "a gorge expansion tag, not a CR keyword",
	"ETBReplacement": "a gorge expansion tag, not a CR keyword",
	"Class":          "CR 716 Class level statics, not a 702 keyword",
	"Strive":         "an ability word (CR 207.2c), not a keyword ability",
}

func TestLoweredKeywordRowsAreNotNA(t *testing.T) {
	regs := crKgenRegistries(t)
	if len(regs) == 0 {
		t.Fatal("no kgen registry heads parsed")
	}
	rows := crReadChapter(t, filepath.Join(crCoverageDir, "702-keyword-abilities.md"))
	for _, msg := range crLoweredKeywordRowErrs(regs, rows) {
		t.Error(msg)
	}
}

func TestLoweredKeywordRowsMutation(t *testing.T) {
	regs := map[string][]string{"Persist": {"expandedKeywords"}}

	t.Run("n/a-only row fails", func(t *testing.T) {
		errs := crLoweredKeywordRowErrs(regs, []crRow{
			{file: "702-keyword-abilities.md", line: 1, rule: "702.79a", status: "n/a"},
		})
		if len(errs) != 1 {
			t.Fatalf("got %d errors, want 1: %v", len(errs), errs)
		}
		if !strings.Contains(errs[0], "Persist") || !strings.Contains(errs[0], "CR 702.79") || !strings.Contains(errs[0], "702-keyword-abilities.md") {
			t.Errorf("error %q does not name the head, the rule and the chapter file", errs[0])
		}
	})

	t.Run("one non-n/a row passes", func(t *testing.T) {
		errs := crLoweredKeywordRowErrs(regs, []crRow{
			{file: "702-keyword-abilities.md", line: 1, rule: "702.79a", status: "partial"},
		})
		if len(errs) != 0 {
			t.Errorf("got errors, want none: %v", errs)
		}
	})
}

// crLoweredKeywordRowErrs is the check of TestLoweredKeywordRowsAreNotNA as a
// value, so the mutation test can run it on an in-memory row set: it fails for
// (a) a registry head in neither table, (b) a head whose matched rows are all
// n/a, and (c) a head whose CR number matches no row at all.
func crLoweredKeywordRowErrs(regs map[string][]string, rows []crRow) []string {
	var errs []string
	for _, head := range crSortedKeys(regs) {
		at := strings.Join(regs[head], "+")
		num, ok := loweredKeywordRule[head]
		if !ok {
			if _, ok := loweredNotKeywordRule[head]; !ok {
				errs = append(errs, fmt.Sprintf("kgen lowers keyword head %q (%s) but it is in neither loweredKeywordRule nor loweredNotKeywordRule of kwlowered_test.go", head, at))
			}
			continue
		}
		matched, allNA := 0, true
		var where []string
		for _, r := range rows {
			if !crRuleCovers(r.rule, num) {
				continue
			}
			matched++
			where = append(where, fmt.Sprintf("%s:%d", r.file, r.line))
			if r.status != "n/a" {
				allNA = false
			}
		}
		switch {
		case matched == 0:
			errs = append(errs, fmt.Sprintf("kgen lowers keyword head %q (%s): no row matches CR %s", head, at, num))
		case allNA:
			errs = append(errs, fmt.Sprintf("kgen lowers keyword head %q (%s) but every matching row (CR %s, %s) is still n/a", head, at, num, strings.Join(where, " ")))
		}
	}
	return errs
}

// crRuleCovers reports whether a row's rule cell covers a CR 702 number: the
// number matches an id of the cell only when it is followed by a non-digit, so
// 702.5 does not match 702.50, and a range cell 702.158-702.160 covers every
// number in it.
var crRuleRange = regexp.MustCompile(`^(\d{3}\.\d+)-(\d{3}\.\d+)$`)

func crRuleCovers(rule, num string) bool {
	if m := crRuleRange.FindStringSubmatch(rule); m != nil {
		lo, hi := crRuleSubNum(m[1]), crRuleSubNum(m[2])
		n := crRuleSubNum(num)
		return lo <= n && n <= hi
	}
	for _, id := range strings.FieldsFunc(rule, func(r rune) bool { return r == ',' || r == '-' || r == '/' }) {
		if id == num {
			return true
		}
		if strings.HasPrefix(id, num) {
			rest := id[len(num):]
			if rest == "" || rest[0] < '0' || rest[0] > '9' {
				return true
			}
		}
	}
	return false
}

func crRuleSubNum(s string) int {
	i := strings.IndexByte(s, '.')
	n := 0
	for _, c := range s[i+1:] {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n
}

func crSortedKeys(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
