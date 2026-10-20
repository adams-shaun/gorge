package codeshape

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The no-strings ratchet. Strings belong at the external boundaries (card
// script parsing at load, deck/pool files, wire and replay formats, CLI
// flags, stats JSON) and in reporting (logs, errors, diagnostics, labels a
// person reads). Everything else -- walks, legality, payment, memo keys,
// type/keyword/subtype checks, param lookups, card identity, net feature
// encoding -- uses integer IDs, enums or bitsets resolved once at the
// boundary. A string compare, string-keyed map or substring scan on a
// per-object, per-walk, per-event or per-search-node path is a bug.
//
// This test counts the string machinery in the engine's and the search
// bots' non-test runtime files: strings.* calls, fmt.Sprint* calls,
// strconv.* calls and map[string] types. Each count FAILS when it grows past
// its constant and LOGS when it falls below; a change that converts sites
// lowers the constant to the new measurement in the same commit. NEVER
// raise one -- convert at the boundary instead, or, for a genuine new
// boundary or reporting FILE, add it to stringsRatchetAllowlist with its
// reason.
var stringsRatchetScopes = []string{"engine/rules", "engine/cards", "bots/policynet", "bots/azmcts"}

// stringsRatchetMax is the measured count per scope and metric.
var stringsRatchetMax = map[string]stringsCounts{
	"engine/rules":   {calls: 1693, sprint: 236, strconv: 159, maps: 240},
	"engine/cards":   {calls: 308, sprint: 5, strconv: 35, maps: 106},
	"bots/policynet": {calls: 49, sprint: 7, strconv: 4, maps: 18},
	"bots/azmcts":    {calls: 7, sprint: 1, strconv: 0, maps: 0},
}

// stringsRatchetAllowlist names the boundary and reporting files the
// ratchet does not count, each with its reason.
var stringsRatchetAllowlist = map[string]string{
	"engine/rules/oracle_run.go":   "the oracle-scenario runner: reads scenario JSON and reports mismatches",
	"engine/rules/clone_gen.go":    "generated clone code: its map[string] copies mirror fields counted at their declaration",
	"engine/rules/cost/parse.go":   "the cost-text parser: the boundary every compiled cost is built at",
	"engine/rules/cost/format.go":  "FormatCost: the cost rendering labels and the wire carry",
	"engine/cards/parse.go":        "the card-script parser (load-time boundary)",
	"engine/cards/cache_codec.go":  "the IR cache's byte format",
	"engine/cards/image_build.go":  "the compiled catalog image's string table",
	"engine/cards/registry.go":     "corpus loading: card files, names and the cache",
	"bots/policynet/checkpoint.go": "the .dzgw checkpoint format",
	"bots/azmcts/record.go":        "visit records written for training",
}

type stringsCounts struct{ calls, sprint, strconv, maps int }

func TestStringsRatchet(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	seen := map[string]bool{}
	for _, scope := range stringsRatchetScopes {
		got, err := measureStrings(root, scope, seen)
		if err != nil {
			t.Fatalf("measure %s: %v", scope, err)
		}
		want := stringsRatchetMax[scope]
		for _, m := range []struct {
			name      string
			got, want int
		}{
			{"strings.* calls", got.calls, want.calls},
			{"fmt.Sprint* calls", got.sprint, want.sprint},
			{"strconv.* calls", got.strconv, want.strconv},
			{"map[string] types", got.maps, want.maps},
		} {
			switch {
			case m.got > m.want:
				t.Errorf("%s: %d %s, above the ratchet's %d: convert the new sites to IDs at the boundary (see this file's doc)", scope, m.got, m.name, m.want)
			case m.got < m.want:
				t.Logf("%s: %d %s, below the ratchet's %d: lower stringsRatchetMax", scope, m.got, m.name, m.want)
			}
		}
	}
	for f := range stringsRatchetAllowlist {
		if !seen[f] {
			t.Errorf("stringsRatchetAllowlist names %s, which no scope holds: delete the stale entry", f)
		}
	}
}

// measureStrings counts scope's non-test Go files, recursively, skipping
// (and recording in seen) the allowlisted ones.
func measureStrings(root, scope string, seen map[string]bool) (stringsCounts, error) {
	var c stringsCounts
	err := filepath.WalkDir(filepath.Join(root, scope), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if _, ok := stringsRatchetAllowlist[rel]; ok {
			seen[rel] = true
			return nil
		}
		f, err := parser.ParseFile(token.NewFileSet(), p, nil, 0)
		if err != nil {
			return err
		}
		imports := map[string]string{} // local name -> import path
		for _, im := range f.Imports {
			path, _ := strconv.Unquote(im.Path.Value)
			name := path[strings.LastIndex(path, "/")+1:]
			if im.Name != nil {
				name = im.Name.Name
			}
			imports[name] = path
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.CallExpr:
				se, ok := x.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				id, ok := se.X.(*ast.Ident)
				if !ok {
					return true
				}
				switch imports[id.Name] {
				case "strings":
					c.calls++
				case "strconv":
					c.strconv++
				case "fmt":
					if strings.HasPrefix(se.Sel.Name, "Sprint") {
						c.sprint++
					}
				}
			case *ast.MapType:
				if id, ok := x.Key.(*ast.Ident); ok && id.Name == "string" {
					c.maps++
				}
			}
			return true
		})
		return nil
	})
	return c, err
}
