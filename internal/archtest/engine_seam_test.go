package archtest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// hostGorgePaths are the server/host files that name engine/rules or
// engine/replay. Everything else in the host runs its game through engine/game,
// so a table can run on another engine.
var hostGorgePaths = map[string]bool{
	// Search, review and feedback capture, which only run on gorge's engine.
	"botenv.go":          true,
	"closedlist.go":      true,
	"feedback.go":        true,
	"review.go":          true,
	"reviewdecisions.go": true,
	"shadowscan.go":      true,
	// Support for the contract tests of gorge tables.
	"contract.go":         true,
	"contract_envleak.go": true,
	"contract_privacy.go": true,
	"contract_shadow.go":  true,
	// MatchTossWinner is gorge's toss.
	"series.go": true,
	// Narrowed by matchGorgeNames.
	"match.go": true,
}

// matchGorgeNames is all match.go may name: the engine handle, nil on another
// engine, and the panic the play loop recovers as a stall.
var matchGorgeNames = map[string]bool{
	"rules.Engine":         true,
	"rules.IsVolumeReason": true,
	"rules.LivelockError":  true,
}

// engineNames returns what file names from engine/rules and engine/replay, each
// as "rules.Config", or nil when it imports neither.
func engineNames(t *testing.T, file string) map[string]bool {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", file, err)
	}
	local := map[string]string{}
	for _, im := range f.Imports {
		path, err := strconv.Unquote(im.Path.Value)
		if err != nil {
			t.Fatalf("%s: import %s: %v", file, im.Path.Value, err)
		}
		if path != module+"/engine/rules" && path != module+"/engine/replay" {
			continue
		}
		base := filepath.Base(path)
		if im.Name != nil {
			local[im.Name.Name] = base
		} else {
			local[base] = base
		}
	}
	if len(local) == 0 {
		return nil
	}
	names := map[string]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if x, ok := sel.X.(*ast.Ident); ok && local[x.Name] != "" {
				names[local[x.Name]+"."+sel.Sel.Name] = true
			}
		}
		return true
	})
	return names
}

// TestHostNamesRulesOnlyInGorgePaths holds the host's reach past engine/game to
// hostGorgePaths, in both directions: a file that stops naming gorge's engine
// leaves the list.
func TestHostNamesRulesOnlyInGorgePaths(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", "server", "host", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, file := range files {
		base := filepath.Base(file)
		if strings.HasSuffix(base, "_test.go") {
			continue
		}
		names := engineNames(t, file)
		if names == nil {
			continue
		}
		seen[base] = true
		sorted := make([]string, 0, len(names))
		for name := range names {
			sorted = append(sorted, name)
		}
		sort.Strings(sorted)
		if !hostGorgePaths[base] {
			t.Errorf("server/host/%s names %v: go through engine/game, or say here why this path only runs on gorge's engine", base, sorted)
		}
		if base == "match.go" {
			for _, name := range sorted {
				if !matchGorgeNames[name] {
					t.Errorf("server/host/match.go names %s: the play loop runs any engine", name)
				}
			}
		}
	}
	for base := range hostGorgePaths {
		if !seen[base] {
			t.Errorf("server/host/%s is in hostGorgePaths but does not name engine/rules or engine/replay", base)
		}
	}
}
