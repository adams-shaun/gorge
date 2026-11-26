package codeshape

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
)

// The no-strings rule. Strings belong at the external boundaries (card
// script parsing at load, deck/pool files, wire and replay formats, CLI
// flags, stats JSON) and in reporting (logs, errors, diagnostics, labels a
// person reads). Everything else -- walks, legality, payment, memo keys,
// type/keyword/subtype checks, param lookups, card identity, net feature
// encoding -- uses integer IDs, enums or bitsets resolved once at the
// boundary. A string compare, string-keyed map or substring scan on a
// per-object, per-walk, per-event or per-search-node path is a bug.
//
// This file measures the string machinery in the engine's and the search
// bots' non-test runtime files: strings.* calls, fmt.Sprint* calls,
// strconv.* calls and map[string] types. cmd/countgrowth holds each count
// to the base-relative count growth rule (countgrowth.go) against the merge
// base; there are no stored ceilings. For a genuine new boundary or
// reporting FILE, add it to StringAllowlist with its reason.

// StringScopes are the repo-relative trees whose string machinery is
// counted.
var StringScopes = []string{"engine/rules", "engine/cards", "bots/policynet", "bots/azmcts"}

// StringAllowlist names the boundary and reporting files the counts skip,
// each with its reason. A stale entry (no scope holds the file any more) is
// refused by cmd/countgrowth.
var StringAllowlist = map[string]string{
	"engine/rules/oracle_run.go":   "the oracle-scenario runner: reads scenario JSON and reports mismatches",
	"engine/rules/clone_gen.go":    "generated clone code: its map[string] copies mirror fields counted at their declaration",
	"engine/rules/cost/parse.go":   "the cost-text parser: the boundary every compiled cost is built at",
	"engine/rules/cost/format.go":  "FormatCost: the cost rendering labels and the wire carry",
	"engine/cards/parse.go":        "the card-script parser (load-time boundary)",
	"engine/cards/digreveal.go":    "reads a card's Oracle text and Dig params (card-text boundary); once per Dig resolution or kgen lowering, never per walk",
	"engine/cards/cache_codec.go":  "the IR cache's byte format",
	"engine/cards/image_build.go":  "the compiled catalog image's string table",
	"engine/cards/registry.go":     "corpus loading: card files, names and the cache",
	"bots/policynet/checkpoint.go": "the .dzgw checkpoint format",
	"bots/azmcts/record.go":        "visit records written for training",
}

// StringsCounts is one scope's measurement.
type StringsCounts struct{ Calls, Sprint, Strconv, Maps int }

// StringCountMetric is one string count: its name, how to read it from a
// scope's measurement, and its advice.
type StringCountMetric struct {
	Name   string
	Get    func(StringsCounts) int
	Advice string
}

// StringCountMetrics returns the four string counts of a scope.
func StringCountMetrics() []StringCountMetric {
	return []StringCountMetric{
		{"strings.* calls", func(c StringsCounts) int { return c.Calls },
			"convert the new sites to IDs at the boundary (the doc comment of stringscount.go)"},
		{"fmt.Sprint* calls", func(c StringsCounts) int { return c.Sprint },
			"convert the new sites to IDs at the boundary (the doc comment of stringscount.go)"},
		{"strconv.* calls", func(c StringsCounts) int { return c.Strconv },
			"convert the new sites to IDs at the boundary (the doc comment of stringscount.go)"},
		{"map[string] types", func(c StringsCounts) int { return c.Maps },
			"convert the new sites to IDs at the boundary (the doc comment of stringscount.go)"},
	}
}

// MeasureStrings counts scope's non-test Go files under root, recursively,
// skipping (and recording in seen) the allowlisted ones.
func MeasureStrings(root, scope string, seen map[string]bool) (StringsCounts, error) {
	var c StringsCounts
	err := filepath.WalkDir(filepath.Join(root, scope), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if _, ok := StringAllowlist[rel]; ok {
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
					c.Calls++
				case "strconv":
					c.Strconv++
				case "fmt":
					if strings.HasPrefix(se.Sel.Name, "Sprint") {
						c.Sprint++
					}
				}
			case *ast.MapType:
				if id, ok := x.Key.(*ast.Ident); ok && id.Name == "string" {
					c.Maps++
				}
			}
			return true
		})
		return nil
	})
	return c, err
}
