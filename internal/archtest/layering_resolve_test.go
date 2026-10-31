package archtest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// layering_resolve_test.go pins engine/rules/resolve, the L6 resolution kernel of
// the rules-engine lasagna (spec 2026-10-03 §3 and §7, W3): checkpoint at the
// intent boundary that begins a resolution, the intent tape, and the
// ask/answer protocol, read and driven through the resolve.Board interface
// package rules implements. Its rows live in their own file (not
// TestDependencyOrderHolds' slice) so the W5 extraction steps running in
// parallel do not edit the same lines; the checks are the same direct and
// transitive ones.

// resolveImports is the closed set of module packages engine/rules/resolve may
// import directly: the L1 decision and event vocabulary its tape is made of
// (intents, posed decisions, the event log's verify window) and the L0/L1
// object vocabulary. The kernel never sees an effects.Ctx, a SpecContext or a
// card's text -- the ask-free predicate's text half is in cards and its
// object half in rules, behind Board.MayAsk -- so effects and the subsystem
// packages are deliberately absent.
//
// To widen it, add the package here with a sentence arguing it sits below
// L6 and that the kernel needs it rather than a Board method; rules itself
// never belongs in this list.
var resolveImports = map[string]bool{
	module + "/engine/cards":    true,
	module + "/engine/state":    true,
	module + "/engine/decision": true,
	module + "/engine/events":   true,
}

// TestResolveImportsStayBelowRules pins engine/rules/resolve's direct module imports
// to resolveImports and forbids the transitive edges the layering rules out:
// rules itself (the kernel reaches the engine only through Board), the
// effects layer (no Ctx or SpecContext crosses the interface), the subsystem
// packages below it that it has no business reaching around Board, and the
// bot layer. It skips until the package exists.
func TestResolveImportsStayBelowRules(t *testing.T) {
	const sub = module + "/engine/rules/resolve"
	pkgs := packages(t)
	p, ok := pkgs[sub]
	if !ok {
		t.Skip("engine/rules/resolve is not built yet")
	}
	var bad []string
	for imp := range p.imports {
		if !strings.HasPrefix(imp, module+"/") {
			continue // the standard library (go.mod has no requires)
		}
		if !resolveImports[imp] {
			bad = append(bad, imp)
		}
	}
	sort.Strings(bad)
	for _, imp := range bad {
		t.Errorf("%s imports %s; the resolution kernel may import only %v "+
			"(internal/archtest/layering_resolve_test.go resolveImports)", sub, imp, sortedKeys(resolveImports))
	}
	for _, to := range []string{
		module + "/engine/rules",
		module + "/engine/effects",
		module + "/engine/rules/chars",
		module + "/engine/rules/combat",
		module + "/engine/rules/pay",
		module + "/engine/rules/trigmatch",
		module + "/bots/botpolicy",
	} {
		if p.deps[to] {
			t.Errorf("%s depends on %s (transitively); the dependency order forbids it", sub, to)
		}
	}
}

// TestNothingBelowRulesReachesTheKernel forbids the upward edges into the L6
// kernel: the effect primitives reach a tape answer through their Host seam
// (rules implements it), never by importing the kernel, and the L2-L5
// packages sit below it. Rows whose from package does not exist yet are
// skipped, so they bind as each package lands.
func TestNothingBelowRulesReachesTheKernel(t *testing.T) {
	const kernel = module + "/engine/rules/resolve"
	pkgs := packages(t)
	for _, from := range []string{
		module + "/engine/cards",
		module + "/engine/state",
		module + "/engine/decision",
		module + "/engine/events",
		module + "/engine/effects",
		module + "/bots/botpolicy",
		module + "/engine/rules/cost",
		module + "/engine/rules/chars",
		module + "/engine/rules/combat",
		module + "/engine/rules/pay",
		module + "/engine/rules/trigmatch",
	} {
		p, ok := pkgs[from]
		if !ok {
			continue
		}
		if p.deps[kernel] {
			t.Errorf("%s depends on %s (transitively); the resolution kernel sits above it", from, kernel)
		}
	}
}

// resolveBoardMethods is the method count of resolve.Board, the interface
// engine/rules/resolve drives the engine through (engine/rules/resolve/board.go). It is a
// SHRINK-ONLY ratchet on the pattern of combatBoardMethods: measured 15 when
// the kernel landed (W3 steps 0-1). A change that removes a method lowers the
// constant in the same commit; NEVER raise it -- fold a new need into an
// existing method (Record carries every per-kind answer record, Restore the
// whole checkpoint restore) instead of widening the interface.
const resolveBoardMethods = 15

// TestResolveBoardOnlyShrinks counts resolve.Board's methods (embedded
// interfaces are counted by name and fail: Board declares every method
// itself, so its size is visible here).
func TestResolveBoardOnlyShrinks(t *testing.T) {
	dir := filepath.Join("..", "..", "engine", "rules", "resolve")
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil || len(files) == 0 {
		t.Skip("engine/rules/resolve is not built yet")
	}
	n, found := 0, false
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, file, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts := spec.(*ast.TypeSpec)
				it, ok := ts.Type.(*ast.InterfaceType)
				if !ok || ts.Name.Name != "Board" {
					continue
				}
				found = true
				for _, m := range it.Methods.List {
					if len(m.Names) == 0 {
						t.Errorf("resolve.Board embeds %v; declare its methods directly so this ratchet sees them", m.Type)
						continue
					}
					n += len(m.Names)
				}
			}
		}
	}
	if !found {
		t.Fatal("engine/rules/resolve declares no Board interface; the ratchet would run vacuously")
	}
	switch {
	case n > resolveBoardMethods:
		t.Errorf("resolve.Board has %d methods, above the frozen ceiling of %d. This is a SHRINK-ONLY "+
			"ratchet: fold the need into an existing Board method instead of adding one. A raised "+
			"constant needs a review.", n, resolveBoardMethods)
	case n < resolveBoardMethods:
		t.Logf("resolve.Board has %d methods, below the ceiling of %d: lower resolveBoardMethods in "+
			"internal/archtest/layering_resolve_test.go to %d in the same commit.", n, resolveBoardMethods, n)
	default:
		t.Logf("resolve.Board: %d methods (at ceiling)", n)
	}
}
