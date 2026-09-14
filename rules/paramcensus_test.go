package rules

// The parameter census (task inbox-engine-ratchet-parameter-census): the
// primitive coverage ratchet in rules/acceptance_test.go only asks the
// registry whether each primitive is REGISTERED. A registered primitive with
// an unconsumed parameter passes -- Reanimate's `GainControl$` counted as
// supported while api:ChangeZone never read the key. This file closes that
// blind spot with a second ratchet over the same repo decks:
//
//   - reads are DERIVED from the code, not hand-listed. A test-time static
//     scan (go/parser over the effects/ and rules/ non-test sources)
//     collects every `.Params["Key"]` access and attributes it to the
//     primitive whose implementation reads it. Attribution roots are derived
//     where the code states them (effects.Register call sites,
//     triggerMatches' Mode$ dispatch switch, activeStatics' literal mode
//     arguments); the few roots the code does not state in a
//     machine-readable position (adjustedCost's apply closure,
//     staticEffects' Continuous filter, the trigger-queue drain,
//     mustAttackRequired's direct scan) are declared in handRoots below --
//     those are ATTRIBUTION (which function to read), never read lists.
//   - an unregistered read FAILS the ratchet (the rot guard): any
//     `.Params[...]` access the scanner cannot resolve -- a new dynamic key,
//     a new base variable, a read in a function no root reaches -- breaks
//     TestParamCensusScanIsComplete until it is classified, so the read sets
//     cannot silently rot.
//   - rules.ParseCost reports every token it does not model in Cost.Unknown
//     (rules/mana.go) instead of degrading it silently; the census turns
//     those into `cost:<Token>` labels alongside `param:<Primitive>.<Key>`
//     ones.
//   - knownUnsupportedParams is the seeded baseline over the current repo
//     decks, checked in both directions exactly like knownUnsupported: a
//     card the engine newly cannot support parameter-wise is a regression; a
//     table entry the build now reads is stale and must be deleted. It only
//     ever shrinks, and only when a real read (or a real parse) is added.
//
// Scope: the census walks exactly what cards.Face.Primitives walks (the
// Abilities/Triggers/Statics/Repls plus the SubAbility chains link.go
// resolved), and censuses only REGISTERED primitives -- an unimplemented
// primitive is the first ratchet's business, and when one registers it
// automatically comes under this census. Keyword heads (kw:...) carry no
// parameter maps; their expansions are censused through the expanded
// abilities' APIs. SVar bodies link.go did not link anywhere are unreachable
// by construction and out of scope.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/internal/testutil"
)

// ---------------------------------------------------------------------------
// 1. The static scan
// ---------------------------------------------------------------------------

// bucket classes a `.Params[...]` access by which card-side structure it
// reads: an SA (the api: primitives' parameter maps, plus the generic
// cast/activation machinery in rules), a trigger, a static, or a
// replacement.
type bucket int

const (
	bSA bucket = iota
	bTrig
	bStat
	bRepl
)

func (b bucket) String() string {
	switch b {
	case bSA:
		return "sa"
	case bTrig:
		return "trig"
	case bStat:
		return "stat"
	default:
		return "repl"
	}
}

// baseBuckets maps the base expression text of a `.Params` selector to its
// bucket for the rules package. In effects every Params map belongs to a
// *cards.SA (the only Params-bearing type any effects function sees), so
// everything there is bSA. Each rules entry below is hand-verified against
// the base's declared type; a name not listed here is UNCLASSIFIED and fails
// the census (the rot guard) rather than being guessed at:
//
//	t       cards.Trigger (trigger-match/queue function parameters)
//	s,st,sv cards.Static / staticView (static and restriction machinery)
//	r, repl cards.Repl; m.repl the replMatch pair (replacement machinery)
//	sa, ab  *cards.SA parameters; sub the SubAbility$ chain successor;
//	cp, copy, targetSA, SA, Ability, With, head, ma *cards.SA locals;
//	pt.SA   the pendingTrigger's effect SA.
var baseBuckets = map[string]bucket{
	"t": bTrig,
	"s": bStat, "st": bStat, "sv": bStat,
	"r": bRepl, "repl": bRepl, "m.repl": bRepl, "c.repl": bRepl,
	"sa": bSA, "ab": bSA, "sub": bSA, "cp": bSA, "copy": bSA,
	"targetSA": bSA, "SA": bSA, "Ability": bSA, "With": bSA,
	"head": bSA, "ma": bSA, "pt.SA": bSA,
	// selector bases: r.With is cards.Repl's resolved With *cards.SA,
	// rp.sa the resume plan's SA, o.Ability the stack object's resolved SA.
	"r.With": bSA, "rp.sa": bSA, "o.Ability": bSA,
}

// callSite records one package-local call with its string-literal arguments
// (for `.Params[paramIdent]` resolution) -- args[i] is "" when argument i is
// not a string literal.
type callSite struct {
	callee string
	args   []string
}

// fnInfo is one function's scanned shape. name is package-qualified
// ("rules:Engine.triggerMatches", "effects:effDealDamage"); methods keep
// their receiver type ("Engine.triggerMatches").
type fnInfo struct {
	pkg       string
	name      string
	reads     map[bucket]map[string]bool
	keyRds    map[bucket]map[string]bool
	calls     map[string]bool
	callSites []callSite
	sig       []string
	strParams map[string]int
}

type scan struct {
	fns     map[string]*fnInfo
	apiImpl map[string]string // api -> effects function name (from Register)
	// statRoots[mode] = functions whose code calls activeStatics("mode").
	statRoots map[string]map[string]bool
	// modeFns[trigMode] = the dispatch function triggerMatches calls for it;
	// dispatchFns is the set of all dispatch callees (excluded from the
	// SHARED trigger read set so each mode only inherits its own branch).
	modeFns      map[string]string
	dispatchFns  map[string]bool
	unclassified []string
	// ambiguousStat records activeStatics calls whose mode is not a literal,
	// for rotGuard to reconcile against handRoots.stat.
	ambiguousStat map[string][]string
}

// scanPackages parses the non-test sources of the rules package (dir ".",
// where a rules test binary runs) and the effects package ("../effects").
func scanPackages(t *testing.T) *scan {
	t.Helper()
	s := &scan{
		fns:         map[string]*fnInfo{},
		apiImpl:     map[string]string{},
		statRoots:   map[string]map[string]bool{},
		modeFns:     map[string]string{},
		dispatchFns: map[string]bool{},
	}
	for _, spec := range []struct{ dir, pkg string }{
		{dir: ".", pkg: "rules"},
		{dir: "../effects", pkg: "effects"},
	} {
		files, err := filepath.Glob(filepath.Join(spec.dir, "*.go"))
		if err != nil || len(files) == 0 {
			t.Fatalf("paramcensus: glob %s: %v", spec.dir, err)
		}
		for _, f := range files {
			if strings.HasSuffix(f, "_test.go") {
				continue
			}
			s.scanFile(t, f, spec.pkg)
		}
	}
	s.propagateKeyReads()
	return s
}

func (s *scan) failf(t *testing.T, pos token.Position, format string, args ...any) {
	s.unclassified = append(s.unclassified, fmt.Sprintf("%s: %s", pos, fmt.Sprintf(format, args...)))
}

func (s *scan) fnOf(pkg, name string) *fnInfo {
	key := pkg + ":" + name
	if fi, ok := s.fns[key]; ok {
		return fi
	}
	fi := &fnInfo{
		pkg: pkg, name: name,
		reads:     map[bucket]map[string]bool{},
		keyRds:    map[bucket]map[string]bool{},
		calls:     map[string]bool{},
		strParams: map[string]int{},
	}
	s.fns[key] = fi
	return fi
}

func (s *scan) addRead(fi *fnInfo, b bucket, key string) {
	if fi.reads[b] == nil {
		fi.reads[b] = map[string]bool{}
	}
	fi.reads[b][key] = true
}

func (s *scan) addKeyRead(fi *fnInfo, b bucket, param string) {
	if fi.keyRds[b] == nil {
		fi.keyRds[b] = map[string]bool{}
	}
	fi.keyRds[b][param] = true
}

// exprText renders the base expression of a .Params selector for bucket
// lookup: plain identifiers and one-level selector chains (m.repl, pt.SA).
func exprText(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		if x, ok := v.X.(*ast.Ident); ok {
			return x.Name + "." + v.Sel.Name
		}
	}
	return ""
}

// scanFile parses one source file: signatures, direct reads, calls, call-site
// literals, Register/activeStatics/dispatch-switch attribution, and the
// range-over-Params whitelist pattern (see scanRangeWhitelist).
func (s *scan) scanFile(t *testing.T, path, pkg string) {
	fset := token.NewFileSet()
	af, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("paramcensus: parse %s: %v", path, err)
	}
	// Assignment LHS index expressions are writes (`copy.Params["Produced"] = c`),
	// never reads.
	writes := map[ast.Node]bool{}
	ast.Inspect(af, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for _, lhs := range as.Lhs {
			if _, ok := lhs.(*ast.IndexExpr); ok {
				writes[lhs] = true
			}
		}
		return true
	})

	for _, decl := range af.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		name := fd.Name.Name
		if fd.Recv != nil && len(fd.Recv.List) > 0 {
			if star, ok := fd.Recv.List[0].Type.(*ast.StarExpr); ok {
				if id, ok := star.X.(*ast.Ident); ok {
					name = id.Name + "." + name
				}
			}
		}
		fi := s.fnOf(pkg, name)
		for _, p := range fd.Type.Params.List {
			for _, pn := range p.Names {
				fi.sig = append(fi.sig, pn.Name)
				if pt, ok := p.Type.(*ast.Ident); ok && pt.Name == "string" {
					fi.strParams[pn.Name] = len(fi.sig) - 1
				}
			}
		}
		s.scanDispatchSwitch(t, fset, fd, pkg)
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.IndexExpr:
				if writes[v] {
					return true
				}
				sel, ok := v.X.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "Params" {
					return true
				}
				base := exprText(sel.X)
				b, ok := s.bucketOf(t, fset, v.Pos(), base, pkg)
				if !ok {
					return true
				}
				switch idx := v.Index.(type) {
				case *ast.BasicLit:
					if idx.Kind != token.STRING {
						s.failf(t, fset.Position(v.Pos()), "%s: non-string literal Params key %s", name, idx.Value)
						return true
					}
					key, err := strconv.Unquote(idx.Value)
					if err != nil {
						s.failf(t, fset.Position(v.Pos()), "%s: unparseable Params key %s", name, idx.Value)
						return true
					}
					s.addRead(fi, b, key)
				case *ast.Ident:
					if _, isParam := fi.strParams[idx.Name]; isParam {
						s.addKeyRead(fi, b, idx.Name)
						return true
					}
					s.failf(t, fset.Position(v.Pos()),
						"%s: dynamic Params key %q that is not a function parameter -- resolve it via a parameter or classify it", name, idx.Name)
				default:
					s.failf(t, fset.Position(v.Pos()), "%s: unclassifiable Params key expression %T", name, v.Index)
				}
				return true
			case *ast.CallExpr:
				s.scanCall(t, fset, fi, name, v, pkg)
				return true
			case *ast.RangeStmt:
				s.scanRangeWhitelist(t, fset, fi, name, v, pkg, writes)
				return true
			}
			return true
		})
	}
}

// scanDispatchSwitch derives the trigger Mode$ dispatch from
// Engine.triggerMatches' switch: case literal -> the callee the case body
// calls. Hand-listing the mode functions here would be exactly the hand list
// this census must not keep.
func (s *scan) scanDispatchSwitch(t *testing.T, fset *token.FileSet, fd *ast.FuncDecl, pkg string) {
	if pkg != "rules" || fd.Recv == nil || fd.Name.Name != "triggerMatches" {
		// Only triggerMatches' switch IS the mode dispatch; a switch on
		// t.Mode elsewhere (e.g. trigger_referents' referent bindings) is not.
		return
	}
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		sw, ok := n.(*ast.SwitchStmt)
		if !ok {
			return true
		}
		tag, ok := sw.Tag.(*ast.SelectorExpr)
		if !ok || tag.Sel.Name != "Mode" {
			return true
		}
		if id, ok := tag.X.(*ast.Ident); !ok || id.Name != "t" {
			return true
		}
		for _, c := range sw.Body.List {
			cc, ok := c.(*ast.CaseClause)
			if !ok {
				continue
			}
			var modes []string
			for _, e := range cc.List {
				if lit, ok := e.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					if v, err := strconv.Unquote(lit.Value); err == nil {
						modes = append(modes, v)
					}
				}
			}
			if len(modes) == 0 {
				continue
			}
			var callee string
			ast.Inspect(cc, func(m ast.Node) bool {
				if callee != "" {
					return false
				}
				if ce, ok := m.(*ast.CallExpr); ok {
					if sel, ok := ce.Fun.(*ast.SelectorExpr); ok {
						if id, ok := sel.X.(*ast.Ident); ok && id.Name == "e" {
							callee = "Engine." + sel.Sel.Name
							return false
						}
					}
					if id, ok := ce.Fun.(*ast.Ident); ok {
						callee = id.Name
						return false
					}
				}
				return true
			})
			if callee == "" {
				if len(modes) == 1 && modes[0] == "Always" {
					// Always' arm sets matched = true inline; its reads are
					// the shared set.
					continue
				}
				s.failf(t, fset.Position(cc.Pos()), "trigger dispatch case %v calls no local function", modes)
				continue
			}
			for _, m := range modes {
				if m != "Always" { // Always' arm reads nothing mode-specific
					s.modeFns[m] = callee
				}
				s.dispatchFns[callee] = true
			}
		}
		return false
	})
}

// scanCall records package-local calls (with literal args for key
// propagation) and the attribution roots the code states: effects.Register
// and rules' activeStatics. FuncLit bodies are attributed to the enclosing
// function by the caller's Inspect, so closures like adjustedCost's apply
// participate here.
func (s *scan) scanCall(t *testing.T, fset *token.FileSet, fi *fnInfo, fname string, ce *ast.CallExpr, pkg string) {
	var callee string
	switch fun := ce.Fun.(type) {
	case *ast.Ident:
		callee = fun.Name
	case *ast.SelectorExpr:
		if id, ok := fun.X.(*ast.Ident); ok {
			if id.Name == "e" && pkg == "rules" {
				callee = "Engine." + fun.Sel.Name
			} else if strings.HasPrefix(fname, id.Name+".") {
				// a method calling another method on the same receiver
				callee = id.Name + "." + fun.Sel.Name
			} else {
				return // cross-package (effects.X) or unrelated selector: not local
			}
		} else {
			return
		}
	default:
		return
	}
	if callee == "Engine.activeStatics" && pkg == "rules" {
		if len(ce.Args) > 0 {
			if lit, ok := ce.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
				if mode, err := strconv.Unquote(lit.Value); err == nil {
					if s.statRoots[mode] == nil {
						s.statRoots[mode] = map[string]bool{}
					}
					s.statRoots[mode][fname] = true
					return
				}
			}
		}
		if s.ambiguousStat == nil {
			s.ambiguousStat = map[string][]string{}
		}
		s.ambiguousStat[fname] = append(s.ambiguousStat[fname], fset.Position(ce.Pos()).String())
		return
	}
	fi.calls[callee] = true
	if callee == "Register" && pkg == "effects" && len(ce.Args) == 2 {
		if lit, ok := ce.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
			if api, err := strconv.Unquote(lit.Value); err == nil {
				if id, ok := ce.Args[1].(*ast.Ident); ok {
					s.apiImpl[api] = id.Name
				}
			}
		}
	}
	if len(ce.Args) == 0 {
		return
	}
	args := make([]string, len(ce.Args))
	for i, a := range ce.Args {
		if lit, ok := a.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			if v, err := strconv.Unquote(lit.Value); err == nil {
				args[i] = v
			}
		}
	}
	fi.callSites = append(fi.callSites, callSite{callee: callee, args: args})
}

// scanRangeWhitelist handles the one `for k := range X.Params` shape in the
// scanned code (Engine.mustAttackRequired): a range over a Params map whose
// body switches on the range variable is a READ of the keys named in the case
// clauses (the whitelist). A range whose body does not match that shape is a
// rot-guard failure. Derived from the code, not hand-set.
func (s *scan) scanRangeWhitelist(t *testing.T, fset *token.FileSet, fi *fnInfo, fname string, rs *ast.RangeStmt, pkg string, writes map[ast.Node]bool) {
	sel, ok := rs.X.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Params" {
		return
	}
	base := exprText(sel.X)
	b, ok := s.bucketOf(t, fset, rs.X.Pos(), base, pkg)
	if !ok {
		return
	}
	keyIdent, ok := rs.Key.(*ast.Ident)
	if !ok {
		s.failf(t, fset.Position(rs.X.Pos()), "%s: range over Params without an index variable", fname)
		return
	}
	var keys []string
	switched := false
	ast.Inspect(rs.Body, func(n ast.Node) bool {
		sw, ok := n.(*ast.SwitchStmt)
		if !ok {
			return true
		}
		tag, ok := sw.Tag.(*ast.Ident)
		if !ok || tag.Name != keyIdent.Name {
			return true
		}
		switched = true
		for _, c := range sw.Body.List {
			cc, ok := c.(*ast.CaseClause)
			if !ok {
				continue
			}
			for _, e := range cc.List {
				if lit, ok := e.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					if v, err := strconv.Unquote(lit.Value); err == nil {
						keys = append(keys, v)
					}
				}
			}
		}
		return false
	})
	if !switched {
		// A copy loop (`for k, v := range src.Params { dst.Params[k] = v }`)
		// is not a read: every use of the key sits in a write-position index.
		if rangeKeyIsWriteOnly(rs, keyIdent.Name, writes) {
			return
		}
		s.failf(t, fset.Position(rs.X.Pos()),
			"%s: range over Params is neither the case-whitelist nor a copy shape -- classify it", fname)
		return
	}
	for _, k := range keys {
		s.addRead(fi, b, k)
	}
}

// rangeKeyIsWriteOnly reports whether the range body is a pure Params copy
// loop: every statement is `dst.Params[key] = v` (a collected write-position
// index) and the key never appears on any right-hand side.
func rangeKeyIsWriteOnly(rs *ast.RangeStmt, key string, writes map[ast.Node]bool) bool {
	usesKey := func(e ast.Expr) bool {
		found := false
		ast.Inspect(e, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && id.Name == key {
				found = true
			}
			return !found
		})
		return found
	}
	for _, stmt := range rs.Body.List {
		as, ok := stmt.(*ast.AssignStmt)
		if !ok || as.Tok != token.ASSIGN || len(as.Lhs) != 1 {
			return false
		}
		ix, ok := as.Lhs[0].(*ast.IndexExpr)
		if !ok || !writes[ix] {
			return false
		}
		id, ok := ix.Index.(*ast.Ident)
		if !ok || id.Name != key {
			return false
		}
		for _, rhs := range as.Rhs {
			if usesKey(rhs) {
				return false
			}
		}
	}
	return true
}

func cloneKeySet(src map[string]bool) map[string]bool {
	out := make(map[string]bool, len(src))
	for k := range src {
		out[k] = true
	}
	return out
}

// bucketOf classes a Params base. effects: everything is an SA map. rules:
// the verified baseBuckets table; anything else is a rot-guard failure.
func (s *scan) bucketOf(t *testing.T, fset *token.FileSet, pos token.Pos, base, pkg string) (bucket, bool) {
	if pkg == "effects" {
		return bSA, true
	}
	if b, ok := baseBuckets[base]; ok {
		return b, true
	}
	s.failf(t, fset.Position(pos),
		"unclassified Params base %q -- add it to baseBuckets with a type justification", base)
	return 0, false
}

// propagateKeyReads resolves `.Params[paramIdent]` reads: every caller of a
// function that reads one of its own string parameters as a key gains the
// literal keys passed at those positions. This is what makes hasStat /
// statInt / statList / actorMatches / effects/count's Num-body read keys
// derived from call sites rather than guessed.
func (s *scan) propagateKeyReads() {
	for _, fi := range s.fns {
		for _, cs := range fi.callSites {
			target := s.fns[fi.pkg+":"+cs.callee]
			if target == nil {
				continue
			}
			for b, params := range target.keyRds {
				for param := range params {
					idx, ok := target.strParams[param]
					if !ok || idx >= len(cs.args) || cs.args[idx] == "" {
						continue
					}
					s.addRead(fi, b, cs.args[idx])
				}
			}
		}
	}
}

// closureReads unions fn's own reads with everything reachable through its
// package-local calls. exclude skips dispatch callees (the shared trigger set
// must not inherit every mode's reads). visited memoises; out accumulates.
func (s *scan) closureReads(fi *fnInfo, exclude map[string]bool, visited map[string]bool, out map[bucket]map[string]bool) {
	if visited[fi.name] {
		return
	}
	visited[fi.name] = true
	// NOTE: fi.keyRds deliberately does NOT join here -- a `.Params[param]`
	// read only counts once propagateKeyReads resolved it to literal keys at
	// call sites (which land in fi.reads of the callers). Unresolved key
	// parameters are not reads.
	for b, keys := range fi.reads {
		if out[b] == nil {
			out[b] = map[string]bool{}
		}
		for k := range keys {
			out[b][k] = true
		}
	}
	for callee := range fi.calls {
		if exclude[callee] {
			continue
		}
		if target := s.fns[fi.pkg+":"+callee]; target != nil {
			s.closureReads(target, exclude, visited, out)
		}
	}
}

// ---------------------------------------------------------------------------
// 2. Attribution roots the code does not state in a machine-readable position
// ---------------------------------------------------------------------------

// handRoots declares ATTRIBUTION (which function to read for a primitive) for
// the few roots the code does not state via Register / the dispatch switch /
// activeStatics literals. The reads still come from the scanned bodies.
var handRoots = struct {
	stat map[string][]string
	trig []string
	repl []string
}{
	stat: map[string][]string{
		// adjustedCost's local `apply` closure and costModifiers' range over
		// []string{"RaiseCost", "ReduceCost"} both call activeStatics with a
		// variable; the literals sit at their callers. Declared instead of
		// refactored so the scan stays read-only over production code.
		"RaiseCost":  {"Engine.adjustedCost", "Engine.costModifiers"},
		"ReduceCost": {"Engine.adjustedCost", "Engine.costModifiers"},
		// staticEffects filters on st.Mode != "Continuous" before reading.
		"Continuous": {"Engine.staticEffects"},
		// mustAttackRequired scans MustAttack statics directly, with no
		// activeStatics call; its Params reads are the whitelist switch.
		"MustAttack": {"Engine.mustAttackRequired"},
	},
	// The trigger-queue drain and the stack-resolution paths read trigger
	// params (OptionalDecider$, TriggerDescription$, Static$, ValidCard$)
	// outside triggerMatches' dispatch: the drain (pushTrigger and the labels
	// it renders), the resolve-time optional ask (resolveTop), the triggered
	// mana-ability gate (isTriggeredManaAbility), the referent bindings
	// (triggerReferents) and the stack-optional query (StackOptional). All are
	// mode-SHARED machinery; each mode still adds its own dispatch branch on
	// top.
	trig: []string{"Engine.pushTrigger", "Engine.triggerLabel", "Engine.abilityLabel",
		"Engine.resolveTop", "Engine.isTriggeredManaAbility", "Engine.triggerReferents",
		"Engine.StackOptional", "Engine.optionalDecider"},
	// applyReplacements is the replacement pipeline's root beside
	// replacementMatches, whose `r.Event != "Moved"` early return scopes every
	// r.Params read in it to repl:Moved. collectETBChoices reads the
	// ETBReplacement repl's Keyword$ at cast-offer time.
	repl: []string{"Engine.applyReplacements", "Engine.collectETBChoices"},
}

// derivedReads is the per-primitive read set the scan attributes.
type derivedReads struct {
	api  map[string]map[string]bool
	trig map[string]map[string]bool
	stat map[string]map[string]bool
	repl map[string]map[string]bool
}

func (s *scan) derived() *derivedReads {
	d := &derivedReads{
		api:  map[string]map[string]bool{},
		trig: map[string]map[string]bool{},
		stat: map[string]map[string]bool{},
		repl: map[string]map[string]bool{},
	}
	// api: the effects implementation's closure, UNION the generic cast/
	// activation/legality machinery: every rules-side SA read applies to
	// whatever primitive is being cast, activated or targeted, so all of them
	// are reads for every api primitive. (The union can only over-suppress a
	// key whose name the machinery genuinely reads for a different purpose;
	// the seeded baseline is hand-checked against exactly that.)
	rulesAllSA := map[string]bool{}
	for key, fi := range s.fns {
		if !strings.HasPrefix(key, "rules:") {
			continue
		}
		for k := range fi.reads[bSA] {
			rulesAllSA[k] = true
		}
	}
	// effects-generic: effects.Resolve runs the Condition* gate (conditionMet)
	// on EVERY SA chain regardless of api, so its closure joins the union the
	// same way the rules machinery does.
	effectsGeneric := map[bucket]map[string]bool{}
	if fi := s.fns["effects:Resolve"]; fi != nil {
		s.closureReads(fi, nil, map[string]bool{}, effectsGeneric)
	}
	for api, fn := range s.apiImpl {
		out := map[bucket]map[string]bool{}
		if fi := s.fns["effects:"+fn]; fi != nil {
			s.closureReads(fi, nil, map[string]bool{}, out)
		}
		keys := map[string]bool{}
		for k := range out[bSA] {
			keys[k] = true
		}
		for k := range rulesAllSA {
			keys[k] = true
		}
		for k := range effectsGeneric[bSA] {
			keys[k] = true
		}
		d.api[api] = keys
	}
	// trig: shared = triggerMatches minus the dispatch callees, plus the
	// drain root; each mode adds its dispatch function's closure.
	shared := map[bucket]map[string]bool{}
	if fi := s.fns["rules:Engine.triggerMatches"]; fi != nil {
		s.closureReads(fi, s.dispatchFns, map[string]bool{}, shared)
	}
	for _, root := range handRoots.trig {
		if fi := s.fns["rules:"+root]; fi != nil {
			s.closureReads(fi, nil, map[string]bool{}, shared)
		}
	}
	d.trig["Always"] = cloneKeySet(shared[bTrig]) // no dispatch branch of its own
	for mode, fn := range s.modeFns {
		keys := map[string]bool{}
		for k := range shared[bTrig] {
			keys[k] = true
		}
		out := map[bucket]map[string]bool{}
		if fi := s.fns["rules:"+fn]; fi != nil {
			s.closureReads(fi, nil, map[string]bool{}, out)
		}
		for k := range out[bTrig] {
			keys[k] = true
		}
		d.trig[mode] = keys
	}
	// stat: per mode, every activeStatics literal call site plus the
	// hand-declared roots.
	for mode, roots := range s.statRoots {
		keys := map[string]bool{}
		for root := range roots {
			out := map[bucket]map[string]bool{}
			if fi := s.fns["rules:"+root]; fi != nil {
				s.closureReads(fi, nil, map[string]bool{}, out)
			}
			for k := range out[bStat] {
				keys[k] = true
			}
		}
		d.stat[mode] = keys
	}
	for mode, roots := range handRoots.stat {
		keys := d.stat[mode]
		if keys == nil {
			keys = map[string]bool{}
		}
		for _, root := range roots {
			out := map[bucket]map[string]bool{}
			if fi := s.fns["rules:"+root]; fi != nil {
				s.closureReads(fi, nil, map[string]bool{}, out)
			}
			for k := range out[bStat] {
				keys[k] = true
			}
		}
		d.stat[mode] = keys
	}
	// repl: replacementMatches + the pipeline root. Every r.Params read in
	// them is scoped to repl:Moved by replacementMatches' early return.
	keys := map[string]bool{}
	for _, root := range append([]string{"Engine.replacementMatches"}, handRoots.repl...) {
		out := map[bucket]map[string]bool{}
		if fi := s.fns["rules:"+root]; fi != nil {
			s.closureReads(fi, nil, map[string]bool{}, out)
		}
		for k := range out[bRepl] {
			keys[k] = true
		}
	}
	d.repl["Moved"] = keys
	return d
}

// ---------------------------------------------------------------------------
// 3. The rot guard
// ---------------------------------------------------------------------------

// rotGuard fails when a read escaped the census:
//
//   - any unclassified access collected during the scan;
//   - an effects function reading Params but unreachable from every
//     Register'd implementation (a primitive implemented but never
//     registered, or a helper only dead code reaches);
//   - a rules function reading trigger/static/replacement params outside
//     every attribution root for that bucket.
//
// A new read therefore cannot join the codebase without either being
// attributed or failing this test.
func (s *scan) rotGuard(t *testing.T) {
	t.Helper()
	// Every non-literal activeStatics call site must be covered by a
	// handRoots.stat declaration naming its function.
	for fname, sites := range s.ambiguousStat {
		covered := false
		for _, fns := range handRoots.stat {
			for _, f := range fns {
				if f == fname {
					covered = true
				}
			}
		}
		if !covered {
			t.Errorf("paramcensus: activeStatics non-literal mode call in %s (%s) has no handRoots.stat attribution",
				fname, strings.Join(sites, ", "))
		}
	}
	if len(s.unclassified) > 0 {
		sort.Strings(s.unclassified)
		t.Fatalf("paramcensus: %d unclassified Params reads (the census cannot rot):\n%s",
			len(s.unclassified), strings.Join(s.unclassified, "\n"))
	}
	// effects reachability.
	visited := map[string]bool{}
	for _, fn := range s.apiImpl {
		if fi := s.fns["effects:"+fn]; fi != nil {
			s.closureReads(fi, nil, visited, map[bucket]map[string]bool{})
		}
	}
	for key, fi := range s.fns {
		if !strings.HasPrefix(key, "effects:") {
			continue
		}
		total := 0
		for b := range fi.reads {
			total += len(fi.reads[b])
		}
		if total > 0 && !visited[fi.name] {
			t.Errorf("paramcensus: effects function %s reads Params but is unreachable from every Register'd implementation", fi.name)
		}
	}
	// rules bucket reachability.
	for key, fi := range s.fns {
		if !strings.HasPrefix(key, "rules:") {
			continue
		}
		for _, b := range []bucket{bTrig, bStat, bRepl} {
			if len(fi.reads[b]) == 0 {
				continue
			}
			if !s.reachFromRoots(b, fi.name) {
				t.Errorf("paramcensus: rules function %s reads %s params outside every attribution root for that bucket", fi.name, b)
			}
		}
	}
}

// reachFromRoots reports whether fn is reachable from the roots declared for
// bucket b (no exclusions: reachability, not read attribution).
func (s *scan) reachFromRoots(b bucket, fn string) bool {
	var roots []string
	switch b {
	case bTrig:
		roots = append([]string{"Engine.triggerMatches"}, handRoots.trig...)
		for _, f := range s.modeFns {
			roots = append(roots, f)
		}
	case bStat:
		for mode := range s.statRoots {
			for root := range s.statRoots[mode] {
				roots = append(roots, root)
			}
		}
		for _, list := range handRoots.stat {
			roots = append(roots, list...)
		}
	case bRepl:
		roots = append([]string{"Engine.replacementMatches"}, handRoots.repl...)
	default:
		return true
	}
	visited := map[string]bool{}
	var dfs func(fi *fnInfo)
	dfs = func(fi *fnInfo) {
		if visited[fi.name] {
			return
		}
		visited[fi.name] = true
		for c := range fi.calls {
			if target := s.fns[fi.pkg+":"+c]; target != nil {
				dfs(target)
			}
		}
	}
	for _, root := range roots {
		if fi := s.fns["rules:"+root]; fi != nil {
			dfs(fi)
		}
	}
	return visited[fn]
}

// ---------------------------------------------------------------------------
// 4. Structural keys, the ignore list, and the card-side census
// ---------------------------------------------------------------------------

// structuralKeys are the keys the cards IR itself consumes (cards/parse.go,
// cards/link.go) rather than any effects/rules implementation: the line heads
// and the chain pointers. They are never `.Params[...]` read in Go because
// the parser turned them into structure (Trigger.Mode, Repl.With, sa.Sub...)
// before any primitive ran.
var structuralKeys = map[string]map[string]bool{
	"sa":   {"SubAbility": true},
	"trig": {"Mode": true, "Execute": true},
	"stat": {"Mode": true},
	"repl": {"Event": true, "ReplaceWith": true},
}

// ignoredParamKeys are keys the census deliberately does NOT report, each
// with the Forge behaviour that makes it presentation/AI-only. Verified
// against the pinned Forge checkout (~/projects/ref/forge; grep the key to
// see every consumer).
var ignoredParamKeys = map[string]string{
	// forge/ai reading: targeting and payment heuristics, never rules
	// behaviour (forge-gui the AI reads these in its score functions).
	"AILogic":            "forge-ai targeting heuristic (AbilityFactory getAiLogic); no rules effect",
	"AITgts":             "forge-ai target scoring hint",
	"AILifeThreshold":    "forge-ai life-threshold heuristic for AI choices",
	"AINoRecursiveCheck": "forge-ai recursion guard for AI evaluation only",
	"AIPhyrexianPayment": "forge-ai Phyrexian-cost payment preference",
	// Human-readable text: rendered in Forge's UI, never read by rules.
	// (SpellDescription is NOT ignored: this engine reads it for ability
	// option labels -- rules/legal.go's legalActions -- so it is a real read.)
	"StackDescription":      "stack-item caption template",
	"TriggerDescription":    "trigger caption template",
	"ChangeTypeDesc":        "UI text for a search's ChangeType$",
	"ValidDescription":      "UI text naming the valid cards",
	"CostDesc":              "UI text for an alternate cost",
	"ConditionDescription":  "display text for a Condition* gate (effects/conditions.go documents it as ignored display text)",
	"PrecostDesc":           "UI text preceding a cost prompt",
	"TgtPrompt":             "target-prompt UI text",
	"ChoiceTitle":           "choice-dialog title",
	"AdditionalDescription": "extra rules-text fragment for the UI",
	"VoteMessage":           "vote dialog message text",
	// AI curse marker: forge-ai uses it to prefer targeting; no rules effect.
	"IsCurse": "forge-ai marks curse Auras/spells for AI preference",
	// Human-readable description text (Forge renders it in dialogs).
	"Description": "UI/dialog description text",
	// forge-game ChangeZoneEffect.java: the search prompt's message text.
	"SelectPrompt": "search prompt message text (forge-game ChangeZoneEffect.java:1110)",
	// forge-game EffectEffect.java:150-162: the effect token's picture key.
	"Image": "effect token image key (forge-game EffectEffect.java:150)",
}

// censusResult is one census run: per-card labels plus aggregate sets.
type censusResult struct {
	labels      map[string][]string // card (first-face name) -> sorted labels
	paramLabels []string            // distinct param: labels across the decks
	costLabels  []string            // distinct cost: labels across the decks
}

// measureParamCensus walks the repo decks' compiled cards and reports, per
// card, every parameter key a REGISTERED primitive's implementation does not
// read and every cost token ParseCost does not model. drop simulates a
// deleted consumer (primitive label, e.g. "api:Sacrifice" -> set of keys to
// treat as unread) so the plumbing is testable without editing production
// code. The zero-drop result is memoised: every ratchet run in one test
// binary shares it.
var (
	censusOnce  sync.Once
	censusBase  censusResult
	censusReads *derivedReads
)

func measureParamCensus(t *testing.T, drop map[string]map[string]bool) (censusResult, *derivedReads) {
	t.Helper()
	if drop == nil {
		censusOnce.Do(func() {
			s := scanPackages(t)
			s.rotGuard(t)
			censusReads = s.derived()
			censusBase = walkRepoDeckCensus(t, censusReads, nil)
		})
		return censusBase, censusReads
	}
	s := scanPackages(t)
	s.rotGuard(t)
	d := s.derived()
	return walkRepoDeckCensus(t, d, drop), d
}

// walkRepoDeckCensus does the card-side walk. Params are labelled only for
// REGISTERED primitives (an unimplemented primitive's unread params are
// noise -- the first ratchet owns the card); cost tokens are labelled
// regardless, because an unmodelled token in a cost string is a real silent
// substitution even when the primitive around it is already unsupported.
func walkRepoDeckCensus(t *testing.T, d *derivedReads, drop map[string]map[string]bool) censusResult {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	res := censusResult{labels: map[string][]string{}}
	paramSeen := map[string]bool{}
	costSeen := map[string]bool{}
	cardSeen := map[string]bool{}
	labelFor := func(prim, key string) string {
		return "param:" + prim + "." + key
	}
	for _, name := range testutil.RepoDeckNames() {
		for _, c := range testutil.RepoDeck(t, reg, name) {
			cardName := c.Faces[0].Name
			if cardSeen[cardName] {
				continue
			}
			cardSeen[cardName] = true
			labels := map[string]bool{}
			addLabel := func(l string) {
				labels[l] = true
				if strings.HasPrefix(l, "param:") {
					paramSeen[l] = true
				} else {
					costSeen[l] = true
				}
			}
			for _, f := range c.Faces {
				var walk func(sa *cards.SA)
				walk = func(sa *cards.SA) {
					if sa == nil {
						return
					}
					prim := "api:" + sa.API
					readSet := d.api[sa.API]
					for k, v := range sa.Params {
						if k == "Cost" || k == "UnlessCost" {
							for _, tok := range ParseCost(v).Unknown {
								addLabel("cost:" + tok)
							}
						}
						if readSet == nil {
							continue // unregistered primitive: ratchet 1 owns it
						}
						if structuralKeys["sa"][k] || ignoredParamKeys[k] != "" {
							continue
						}
						if !readSet[k] || drop != nil && drop[prim][k] {
							addLabel(labelFor(prim, k))
						}
					}
					walk(sa.Sub)
				}
				for _, a := range f.Abilities {
					walk(a)
				}
				for _, tr := range f.Triggers {
					prim := "trig:" + tr.Mode
					readSet := d.trig[tr.Mode]
					if readSet != nil {
						for k := range tr.Params {
							if structuralKeys["trig"][k] || ignoredParamKeys[k] != "" {
								continue
							}
							if !readSet[k] || drop != nil && drop[prim][k] {
								addLabel(labelFor(prim, k))
							}
						}
					}
					walk(tr.Effect)
				}
				for _, st := range f.Statics {
					prim := "stat:" + st.Mode
					readSet := d.stat[st.Mode]
					for k, v := range st.Params {
						if k == "Cost" || k == "UnlessCost" {
							for _, tok := range ParseCost(v).Unknown {
								addLabel("cost:" + tok)
							}
						}
						if readSet == nil {
							continue // unregistered static mode: ratchet 1 owns it
						}
						if structuralKeys["stat"][k] || ignoredParamKeys[k] != "" {
							continue
						}
						if !readSet[k] || drop != nil && drop[prim][k] {
							addLabel(labelFor(prim, k))
						}
					}
				}
				for _, r := range f.Repls {
					prim := "repl:" + r.Event
					readSet := d.repl[r.Event]
					if readSet != nil {
						for k := range r.Params {
							if structuralKeys["repl"][k] || ignoredParamKeys[k] != "" {
								continue
							}
							if !readSet[k] || drop != nil && drop[prim][k] {
								addLabel(labelFor(prim, k))
							}
						}
					}
					walk(r.With)
				}
			}
			if len(labels) > 0 {
				out := make([]string, 0, len(labels))
				for l := range labels {
					out = append(out, l)
				}
				sort.Strings(out)
				res.labels[cardName] = out
			}
		}
	}
	for l := range paramSeen {
		res.paramLabels = append(res.paramLabels, l)
	}
	for l := range costSeen {
		res.costLabels = append(res.costLabels, l)
	}
	sort.Strings(res.paramLabels)
	sort.Strings(res.costLabels)
	return res
}

// ---------------------------------------------------------------------------
// 5. The ratchet and its probes
// ---------------------------------------------------------------------------

// knownUnsupportedParams is the PARAMETER ratchet (task
// inbox-engine-ratchet-parameter-census), measured by
// TestEveryRepoDeckParamsAreRead on its first run against the current repo
// decks and seeded by hand, exactly like knownUnsupported. Label grammar:
//
//	param:<Primitive>.<Key>  a key the card's script carries on a registered
//	                         primitive whose implementation never reads it
//	                         (e.g. param:api:ChangeZone.GainControl);
//	cost:<Token>             a cost token ParseCost silently substitutes
//	                         generic mana for (e.g. cost:PayEnergy).
//
// Presentation/AI keys are never measured (ignoredParamKeys above, each with
// its Forge citation). The table is checked in both directions -- a newly
// unread key is a regression; a stale entry means the key is now read and
// must be deleted -- so it only ever shrinks, and only when a real read or a
// real ParseCost model is added.
var knownUnsupportedParams = map[string][]string{
	"Abbot of Keral Keep":         {"param:api:Cleanup.ClearRemembered", "param:api:Dig.RememberChanged", "param:api:Effect.ExileOnMoved", "param:trig:SpellCast.Keyword", "param:trig:SpellCast.KeywordLine"},
	"Ad Nauseam":                  {"param:api:Repeat.RepeatOptional"},
	"Adaptive Automaton":          {"param:api:ChooseType.Type", "param:repl:Moved.KeywordLine"},
	"Aether Vial":                 {"param:api:ChangeZone.Optional"},
	"Aftermath Analyst":           {"param:api:ChangeZoneAll.Tapped"},
	"Ancient Stirrings":           {"param:api:Dig.ForceRevealToController"},
	"Angelic Accord":              {"param:trig:Phase.CheckSVar", "param:trig:Phase.SVarCompare"},
	"Angelic Overseer":            {"param:stat:Continuous.IsPresent"},
	"Angelic Skirmisher":          {"param:api:Pump.KWChoice"},
	"Army of the Damned":          {"param:api:Token.TokenTapped"},
	"Assassin's Trophy":           {"param:api:ChangeZone.Optional", "param:api:ChangeZone.ShuffleNonMandatory"},
	"Auriok Steelshaper":          {"param:stat:Continuous.IsPresent", "param:stat:ReduceCost.ValidSpell"},
	"Azusa, Lost but Seeking":     {"param:stat:Continuous.AdjustLandPlays"},
	"Baloth Prime":                {"param:api:PutCounter.ETB", "param:api:Token.TokenTapped"},
	"Banisher Priest":             {"param:api:ChangeZone.Duration"},
	"Banishing Light":             {"param:api:ChangeZone.Duration"},
	"Batterskull":                 {"param:api:Attach.Keyword", "param:api:Attach.KeywordLine", "param:trig:ChangesZone.Keyword", "param:trig:ChangesZone.KeywordLine"},
	"Battlegrace Angel":           {"param:trig:Attacks.Keyword", "param:trig:Attacks.KeywordLine"},
	"Bile Blight":                 {"param:api:Cleanup.ClearRemembered", "param:api:Pump.RememberTargets"},
	"Blazemire Verge":             {"param:api:Mana.IsPresent"},
	"Blood Crypt":                 {"param:api:Tap.UnlessPayer"},
	"Bloodchief Ascension":        {"param:trig:Phase.CheckSVar", "param:trig:Phase.SVarCompare"},
	"Bloodsoaked Champion":        {"param:api:ChangeZone.CheckSVar"},
	"Bonesplitter":                {"param:api:Attach.Keyword", "param:api:Attach.KeywordLine"},
	"Borderland Ranger":           {"param:api:ChangeZone.ShuffleNonMandatory"},
	"Braids, Arisen Nightmare":    {"param:api:Cleanup.ClearRemembered", "param:api:Sacrifice.Optional"},
	"Brainstorm":                  {"param:api:ChangeZone.Mandatory", "param:api:ChangeZone.Reorder"},
	"Burning Wish":                {"param:api:ChangeZone.Hidden", "param:api:ChangeZone.Reveal"},
	"Butcher Ghoul":               {"param:trig:ChangesZone.Keyword", "param:trig:ChangesZone.KeywordLine"},
	"Cavern of Souls":             {"param:api:ChooseType.Type", "param:api:Mana.AddsNoCounter", "param:api:Mana.RestrictValid", "param:repl:Moved.KeywordLine"},
	"Celestial Colonnade":         {"param:api:Animate.Colors", "param:api:Animate.Keywords", "param:api:Animate.OverwriteColors"},
	"Chain Lightning":             {"param:api:CopySpellAbility.Controller"},
	"Chalice of the Void":         {"param:api:PutCounter.ETB", "param:repl:Moved.KeywordLine"},
	"Chandra, Awakened Inferno":   {"cost:SubCounter", "param:api:Cleanup.ClearRemembered", "param:api:DealDamage.ReplaceDyingDefined", "param:api:DealDamage.Ultimate", "param:api:Effect.EffectOwner", "param:api:Effect.Name"},
	"Chaos Warp":                  {"param:api:Dig.DestinationZone2", "param:api:Dig.LibraryPosition2", "param:api:Dig.Reveal"},
	"Conduit of Worlds":           {"param:api:Cleanup.ClearRemembered", "param:stat:Continuous.AffectedZone", "param:stat:Continuous.MayPlay"},
	"Council's Judgment":          {"param:api:Vote.VoteCard", "param:api:Vote.VoteSubAbility"},
	"Crucible of Worlds":          {"param:stat:Continuous.AffectedZone", "param:stat:Continuous.MayPlay"},
	"Cultivate":                   {"param:api:ChangeZone.Mandatory", "param:api:ChangeZone.NoLooking", "param:api:ChangeZone.Reveal", "param:api:Cleanup.ClearRemembered"},
	"Dark Fortress":               {"param:api:Mana.IsPresent"},
	"Dauthi Voidwalker":           {"param:api:ChangeZone.Hidden", "param:api:Effect.ForgetOnMoved"},
	"Daze":                        {"cost:Return", "param:stat:AlternativeCost.EffectZone", "param:stat:AlternativeCost.ValidSA"},
	"Deadly Rollick":              {"param:stat:AlternativeCost.EffectZone", "param:stat:AlternativeCost.IsPresent", "param:stat:AlternativeCost.ValidPlayer", "param:stat:AlternativeCost.ValidSA"},
	"Defense of the Heart":        {"param:trig:Phase.CheckSVar", "param:trig:Phase.SVarCompare"},
	"Deflecting Swat":             {"param:stat:AlternativeCost.EffectZone", "param:stat:AlternativeCost.IsPresent", "param:stat:AlternativeCost.ValidPlayer", "param:stat:AlternativeCost.ValidSA"},
	"Delver of Secrets":           {"param:api:Cleanup.ClearRemembered", "param:api:PeekAndReveal.PeekAmount"},
	"Demonic Tutor":               {"param:api:ChangeZone.Mandatory"},
	"Eldrazi Temple":              {"param:api:Mana.RestrictValid"},
	"Electrostatic Bolt":          {"param:api:DealDamage.ConditionCheckSVar", "param:api:DealDamage.ConditionSVarCompare"},
	"Empty the Warrens":           {"param:trig:SpellCast.Keyword", "param:trig:SpellCast.KeywordLine"},
	"Endless One":                 {"param:api:PutCounter.ETB", "param:repl:Moved.KeywordLine"},
	"Escape Tunnel":               {"param:api:Effect.ExileOnMoved"},
	"Evendo Brushrazer":           {"param:stat:Continuous.AffectedZone", "param:stat:Continuous.CheckSVar", "param:stat:Continuous.Condition", "param:stat:Continuous.MayPlay"},
	"Experiment One":              {"param:trig:ChangesZone.Keyword", "param:trig:ChangesZone.KeywordLine"},
	"Exploration Broodship":       {"param:stat:Continuous.AddStaticAbility"},
	"Fabled Passage":              {"param:api:Cleanup.ClearRemembered"},
	"Flayer Husk":                 {"param:api:Attach.Keyword", "param:api:Attach.KeywordLine", "param:trig:ChangesZone.Keyword", "param:trig:ChangesZone.KeywordLine"},
	"Flickerwisp":                 {"param:api:ChangeZone.Mandatory", "param:api:Cleanup.ClearRemembered", "param:api:DelayedTrigger.RememberObjects"},
	"Forbidding Watchtower":       {"param:api:Animate.Colors", "param:api:Animate.OverwriteColors"},
	"Force of Will":               {"cost:ExileFromHand", "param:api:Counter.Destination", "param:stat:AlternativeCost.EffectZone", "param:stat:AlternativeCost.ValidSA"},
	"Foreboding Ruins":            {"cost:Reveal", "param:api:Tap.UnlessPayer"},
	"Forked Bolt":                 {"param:api:DealDamage.DividedAsYouChoose"},
	"Gamble":                      {"param:api:ChangeZone.Mandatory"},
	"Geralf's Messenger":          {"param:trig:ChangesZone.Keyword", "param:trig:ChangesZone.KeywordLine"},
	"Ghalta, Primal Hunger":       {"param:stat:ReduceCost.EffectZone"},
	"Ghost Quarter":               {"param:api:ChangeZone.Optional", "param:api:ChangeZone.ShuffleNonMandatory"},
	"Giada, Font of Hope":         {"param:api:Mana.RestrictValid", "param:api:PutCounter.ETB", "param:repl:Moved.KeywordLine"},
	"Gitaxian Probe":              {"param:api:RevealHand.Look"},
	"Gleeful Arsonist":            {"param:trig:ChangesZone.Keyword", "param:trig:ChangesZone.KeywordLine"},
	"Goblin Guide":                {"param:api:Dig.LibraryPosition2", "param:api:Dig.Reveal"},
	"Grand Abolisher":             {"param:stat:CantBeActivated.AffectedZone", "param:stat:CantBeActivated.Condition", "param:stat:CantBeCast.Condition"},
	"Grave Titan":                 {"param:trig:Attacks.Secondary"},
	"Gravecrawler":                {"param:stat:Continuous.AffectedZone", "param:stat:Continuous.EffectZone", "param:stat:Continuous.IsPresent", "param:stat:Continuous.MayPlay"},
	"Grim Tutor":                  {"param:api:ChangeZone.Mandatory"},
	"Hallowed Fountain":           {"param:api:Tap.UnlessPayer"},
	"Hangarback Walker":           {"param:api:PutCounter.ETB", "param:repl:Moved.KeywordLine"},
	"Hearthhull, the Worldseed":   {"param:stat:Continuous.AddAbility", "param:stat:Continuous.AddTrigger"},
	"Icetill Explorer":            {"param:stat:Continuous.AdjustLandPlays", "param:stat:Continuous.AffectedZone", "param:stat:Continuous.MayPlay"},
	"Imperial Seal":               {"param:api:ChangeZone.Mandatory"},
	"Impulse":                     {"param:api:Dig.NoReveal"},
	"Incinerate":                  {"param:api:Cleanup.ClearRemembered", "param:api:Effect.ForgetOnMoved"},
	"Infernal Tutor":              {"param:api:ChangeZone.ConditionCheckSVar", "param:api:ChangeZone.ConditionSVarCompare", "param:api:ChangeZone.Mandatory", "param:api:Cleanup.ClearRemembered", "param:api:Reveal.ConditionCheckSVar"},
	"Into the Roil":               {"param:api:Draw.Condition"},
	"Jace, the Mind Sculptor":     {"param:api:ChangeZone.Mandatory", "param:api:ChangeZoneAll.Shuffle", "param:api:ChangeZoneAll.Ultimate", "param:api:Dig.LibraryPosition2"},
	"Jeska's Will":                {"param:api:Charm.MinCharmNum"},
	"Journey to Nowhere":          {"param:api:ChangeZone.ForgetOtherTargets", "param:api:ChangeZone.RememberTargets"},
	"Karn Liberated":              {"param:api:ChangeZone.Hidden", "param:api:ChangeZone.Mandatory", "param:api:ChangeZoneAll.GainControl", "param:api:RestartGame.RestrictFromValid", "param:api:RestartGame.RestrictFromZone", "param:api:RestartGame.Ultimate"},
	"Karn, the Great Creator":     {"param:api:Animate.Duration", "param:api:ChangeZone.Hidden", "param:api:ChangeZone.Reveal", "param:stat:CantBeActivated.AffectedZone"},
	"Knight of Infamy":            {"param:trig:Attacks.Keyword", "param:trig:Attacks.KeywordLine"},
	"Knight of the White Orchid":  {"param:api:ChangeZone.ShuffleNonMandatory", "param:trig:ChangesZone.CheckSVar", "param:trig:ChangesZone.SVarCompare"},
	"Kodama's Reach":              {"param:api:ChangeZone.Mandatory", "param:api:ChangeZone.NoLooking", "param:api:ChangeZone.Reveal", "param:api:Cleanup.ClearRemembered"},
	"Kor Skyfisher":               {"param:api:ChangeZone.Hidden", "param:api:ChangeZone.Mandatory"},
	"Land Tax":                    {"param:api:ChangeZone.ShuffleNonMandatory", "param:trig:Phase.CheckSVar", "param:trig:Phase.SVarCompare"},
	"Leonin Relic-Warder":         {"param:api:ChangeZone.ForgetOtherTargets", "param:api:ChangeZone.RememberTargets"},
	"Leonin Scimitar":             {"param:api:Attach.Keyword", "param:api:Attach.KeywordLine"},
	"Lightning Greaves":           {"param:api:Attach.Keyword", "param:api:Attach.KeywordLine"},
	"Linvala, Keeper of Silence":  {"param:stat:CantBeActivated.AffectedZone"},
	"Lion's Eye Diamond":          {"param:api:Mana.InstantSpeed"},
	"Lord Windgrace":              {"param:api:Cleanup.ClearRemembered", "param:api:Destroy.Ultimate", "param:api:Discard.RememberDiscarded"},
	"Loxodon Warhammer":           {"param:api:Attach.Keyword", "param:api:Attach.KeywordLine"},
	"Master of Etherium":          {"param:stat:Continuous.CharacteristicDefining"},
	"Matter Reshaper":             {"param:api:Dig.DestinationZone2", "param:api:Dig.Reveal"},
	"Meathook Massacre II":        {"param:api:ChangeZone.GainControl", "param:api:ChangeZone.UnlessPayer"},
	"Mishra's Factory":            {"param:api:Animate.RemoveCreatureTypes"},
	"Mistveil Plains":             {"param:api:ChangeZone.IsPresent", "param:api:ChangeZone.PresentCompare"},
	"Mogis, God of Slaughter":     {"param:api:DealDamage.UnlessPayer", "param:stat:Continuous.CheckSVar", "param:stat:Continuous.RemoveType", "param:stat:Continuous.SVarCompare"},
	"Monastery Swiftspear":        {"param:trig:SpellCast.Keyword", "param:trig:SpellCast.KeywordLine"},
	"Myriad Landscape":            {"param:api:ChangeZone.ShareLandType"},
	"Necrodominance":              {"cost:PayLife", "param:api:ChangeZone.Hidden", "param:stat:Continuous.SetMaxHandSize"},
	"Necropotence":                {"param:api:ChangeZone.ExileFaceDown", "param:api:Cleanup.ClearRemembered", "param:api:DelayedTrigger.RememberObjects", "param:api:DelayedTrigger.ValidPlayer"},
	"Nissa's Pilgrimage":          {"param:api:ChangeZone.ForgetChanged", "param:api:ChangeZone.Mandatory", "param:api:ChangeZone.NoLooking", "param:api:Cleanup.ClearRemembered"},
	"Ob Nixilis, Captive Kingpin": {"param:api:Cleanup.ClearRemembered", "param:api:Dig.RememberChanged", "param:api:Effect.ForgetOnMoved"},
	"Ojer Axonil, Deepest Might":  {"param:api:ChangeZone.Transformed", "param:api:SetState.CheckSVar", "param:api:SetState.SVarCompare"},
	"Oracle of Mul Daya":          {"param:stat:Continuous.AdjustLandPlays", "param:stat:Continuous.AffectedZone", "param:stat:Continuous.MayLookAt", "param:stat:Continuous.MayPlay"},
	"Overseer of the Damned":      {"param:api:Token.TokenTapped"},
	"Palace Jailer":               {"param:api:Effect.EffectOwner", "param:api:Effect.ForgetOnMoved"},
	"Path to Exile":               {"param:api:ChangeZone.Optional", "param:api:ChangeZone.ShuffleNonMandatory"},
	"Phyrexian Revoker":           {"param:repl:Moved.KeywordLine"},
	"Pithing Needle":              {"param:repl:Moved.KeywordLine"},
	"Ponder":                      {"param:api:RearrangeTopOfLibrary.MayShuffle"},
	"Profane Tutor":               {"param:api:ChangeZone.Mandatory"},
	"Purphoros, God of the Forge": {"param:stat:Continuous.CheckSVar", "param:stat:Continuous.RemoveType", "param:stat:Continuous.SVarCompare"},
	"Ragavan, Nimble Pilferer":    {"param:api:Cleanup.ClearRemembered", "param:api:Dig.RememberChanged", "param:api:Effect.ForgetOnMoved"},
	"Rakdos, Lord of Riots":       {"param:stat:CantBeCast.CheckSVar", "param:stat:CantBeCast.EffectZone", "param:stat:CantBeCast.SVarCompare"},
	"Ramunap Excavator":           {"param:stat:Continuous.AffectedZone", "param:stat:Continuous.MayPlay"},
	"Rancor":                      {"param:api:Attach.Keyword", "param:api:Attach.KeywordLine"},
	"Razorkin Needlehead":         {"param:stat:Continuous.Condition"},
	"Reality Smasher":             {"param:api:Counter.UnlessPayer", "param:trig:BecomesTarget.ValidSource"},
	"Realms Uncharted":            {"param:api:ChangeZone.DifferentNames", "param:api:ChangeZone.Mandatory", "param:api:ChangeZone.NoLooking", "param:api:ChangeZone.Reveal", "param:api:Cleanup.ClearRemembered"},
	"Relic of Progenitus":         {"cost:Exile", "param:api:ChangeZone.Hidden", "param:api:ChangeZone.Mandatory"},
	"Remand":                      {"param:api:Counter.Destination"},
	"Resplendent Angel":           {"param:trig:Phase.CheckSVar", "param:trig:Phase.SVarCompare"},
	"Restless Cottage":            {"param:api:Animate.Colors", "param:api:Animate.OverwriteColors"},
	"Riddlesmith":                 {"cost:Draw"},
	"Righteous Valkyrie":          {"param:stat:Continuous.CheckSVar", "param:stat:Continuous.SVarCompare"},
	"Roiling Vortex":              {"param:trig:SpellCast.ValidSA"},
	"Sanctum Prelate":             {"param:repl:Moved.KeywordLine"},
	"Scapeshift":                  {"param:api:Cleanup.ClearRemembered", "param:api:Sacrifice.Optional"},
	"Scourge of Valkas":           {"param:api:DealDamage.DamageSource"},
	"Screaming Nemesis":           {"param:api:Cleanup.ClearRemembered"},
	"Sea Gate Wreckage":           {"param:api:Draw.Activation"},
	"Serra Avenger":               {"param:stat:CantBeCast.CheckSVar", "param:stat:CantBeCast.EffectZone", "param:stat:CantBeCast.SVarCompare"},
	"Silkwrap":                    {"param:api:ChangeZone.Duration"},
	"Skullclamp":                  {"param:api:Attach.Keyword", "param:api:Attach.KeywordLine"},
	"Skyclave Apparition":         {"param:api:Cleanup.ClearRemembered", "param:api:Token.TokenPower", "param:api:Token.TokenToughness"},
	"Snapcaster Mage":             {"param:api:Pump.PumpZone"},
	"Solemn Simulacrum":           {"param:api:ChangeZone.ShuffleNonMandatory"},
	"Sower of Discord":            {"param:api:Cleanup.ClearRemembered", "param:repl:Moved.KeywordLine", "param:trig:DamageDoneOnce.ActiveZones", "param:trig:DamageDoneOnce.Secondary"},
	"Splendid Reclamation":        {"param:api:ChangeZoneAll.Tapped"},
	"Springbloom Druid":           {"param:api:ChangeZone.ShuffleNonMandatory"},
	"Squadron Hawk":               {"param:api:ChangeZone.ShuffleNonMandatory"},
	"Stasis Snare":                {"param:api:ChangeZone.Duration"},
	"Static Orb":                  {"param:stat:Continuous.IsPresent"},
	"Steel Leaf Champion":         {"param:stat:CantBlockBy.ValidAttacker"},
	"Stoneforge Mystic":           {"param:api:ChangeZone.ShuffleNonMandatory"},
	"Strangleroot Geist":          {"param:trig:ChangesZone.Keyword", "param:trig:ChangesZone.KeywordLine"},
	"Sublime Archangel":           {"param:trig:Attacks.Keyword", "param:trig:Attacks.KeywordLine"},
	"Sun Titan":                   {"param:trig:Attacks.Secondary"},
	"Sword of Fire and Ice":       {"param:api:Attach.Keyword", "param:api:Attach.KeywordLine", "param:stat:Continuous.AddSVar"},
	"Szarel, Genesis Shepherd":    {"param:stat:Continuous.AffectedZone", "param:stat:Continuous.MayPlay"},
	"Tainted Peak":                {"param:api:Mana.IsPresent"},
	"Temple of the False God":     {"param:api:Mana.IsPresent", "param:api:Mana.PresentCompare"},
	"Temur Sabertooth":            {"param:api:ChangeZone.Hidden", "param:api:Cleanup.ClearRemembered"},
	"Tendrils of Agony":           {"param:trig:SpellCast.Keyword", "param:trig:SpellCast.KeywordLine"},
	"Terminus":                    {"param:api:ChangeZoneAll.LibraryPosition"},
	"The Lord of Pain":            {"param:trig:SpellCast.ActivatorThisTurnCast"},
	"Thirst for Knowledge":        {"param:api:Discard.UnlessType"},
	"Thornspire Verge":            {"param:api:Mana.IsPresent"},
	"Through the Forest Gate":     {"param:api:Dig.SkipReorder", "param:api:Dig.Tapped"},
	"Thunderbreak Regent":         {"param:trig:BecomesTarget.ValidSource"},
	"Tome of Legends":             {"param:api:PutCounter.ETB", "param:repl:Moved.KeywordLine", "param:trig:Attacks.Secondary"},
	"Toxic Deluge":                {"cost:PayLife"},
	"Trinket Mage":                {"param:api:ChangeZone.ShuffleNonMandatory"},
	"Troop of Ponies":             {"param:api:ChangeZone.ForgetChanged", "param:api:ChangeZone.Mandatory", "param:api:ChangeZone.NoLooking", "param:api:ChangeZone.Reveal", "param:api:Cleanup.ClearRemembered"},
	"Umezawa's Jitte":             {"param:api:Attach.Keyword", "param:api:Attach.KeywordLine"},
	"Valakut Exploration":         {"param:api:ChangeZoneAll.RememberChanged", "param:api:Cleanup.ClearRemembered", "param:api:DamageAll.ValidPlayers", "param:api:Dig.RememberChanged", "param:api:Effect.ForgetOnMoved", "param:trig:Phase.CheckSVar", "param:trig:Phase.SVarCompare"},
	"Valkyrie Harbinger":          {"param:trig:Phase.CheckSVar", "param:trig:Phase.SVarCompare"},
	"Vampire Lacerator":           {"param:api:LoseLife.ConditionCheckSVar", "param:api:LoseLife.ConditionSVarCompare"},
	"Vampiric Tutor":              {"param:api:ChangeZone.Mandatory"},
	"Vastwood Hydra":              {"param:api:PutCounter.ChoiceAmount", "param:api:PutCounter.DividedAsYouChoose", "param:api:PutCounter.ETB", "param:api:PutCounter.MinChoiceAmount", "param:repl:Moved.KeywordLine"},
	"Vexing Devil":                {"cost:DamageYou", "param:api:Sacrifice.UnlessPayer", "param:api:Sacrifice.UnlessSwitched"},
	"Vial Smasher the Fierce":     {"param:api:Cleanup.ClearChosenPlayer", "param:trig:SpellCast.ActivatorThisTurnCast"},
	"Victimize":                   {"param:api:ChangeZone.ConditionCheckSVar", "param:api:ChangeZone.ConditionSVarCompare", "param:api:Cleanup.ClearRemembered"},
	"Vines of Vastwood":           {"param:api:Effect.ExileOnMoved"},
	"Virtue of Persistence":       {"param:api:ChangeZone.GainControl", "param:api:ChangeZone.Mandatory"},
	"Voracious Hydra":             {"param:api:PutCounter.ETB", "param:repl:Moved.KeywordLine"},
	"Walk-In Closet":              {"param:api:Effect.ReplacementEffects", "param:stat:Continuous.AffectedZone", "param:stat:Continuous.MayPlay"},
	"Walking Ballista":            {"param:api:PutCounter.ETB", "param:repl:Moved.KeywordLine"},
	"Wastewood Verge":             {"param:api:Mana.IsPresent"},
	"Whirler Rogue":               {"cost:tapXType", "param:api:Effect.ExileOnMoved"},
	"Whisperer of the Wilds":      {"param:api:Mana.IsPresent"},
	"Windgrace's Judgment":        {"param:api:Destroy.TargetsForEachPlayer"},
	"Winged Boots":                {"param:api:Attach.Keyword", "param:api:Attach.KeywordLine"},
	"World Shaper":                {"param:api:ChangeZoneAll.Tapped", "param:api:Mill.Optional"},
	"Wrenn and Six":               {"param:api:Effect.Name", "param:api:Effect.Stackable", "param:api:Effect.Ultimate"},
	"Yavimaya Elder":              {"param:api:ChangeZone.ShuffleNonMandatory"},
	"Zombie Apocalypse":           {"param:api:ChangeZoneAll.Tapped"},
}

// TestEveryRepoDeckParamsAreRead is the parameter ratchet: every card across
// the repo decks carries only the unread parameters and unmodelled cost
// tokens knownUnsupportedParams lists, and every entry in the table is still
// measured. Same failure style as the primitive ratchet.
func TestEveryRepoDeckParamsAreRead(t *testing.T) {
	res, _ := measureParamCensus(t, nil)
	t.Logf("param census: %d of %d distinct repo-deck cards carry at least one unread param or unmodelled cost token; %d distinct param labels, %d distinct cost labels",
		len(res.labels), distinctRepoDeckCards(t), len(res.paramLabels), len(res.costLabels))
	for card, got := range res.labels {
		want, ok := knownUnsupportedParams[card]
		if !ok {
			t.Errorf("%s carries unread params %v, which is not in knownUnsupportedParams -- new gap, add it to the table", card, got)
			continue
		}
		if !sameSet(want, got) {
			t.Errorf("%s: knownUnsupportedParams says %v, measured %v -- update the table to match", card, want, got)
		}
	}
	for card, want := range knownUnsupportedParams {
		if _, still := res.labels[card]; !still {
			t.Errorf("%s now reads everything knownUnsupportedParams listed (%v) -- delete the stale entry", card, want)
		}
	}
}

// TestParamCensusScanIsComplete is the rot guard on its own: the scan must
// classify every Params read in effects and rules, and every read must sit
// inside an attribution root. Called from measureParamCensus too; this
// standalone form keeps the failure visible without a corpus present.
func TestParamCensusScanIsComplete(t *testing.T) {
	s := scanPackages(t)
	s.rotGuard(t)
}

// TestParamCensusDetectsADeletedConsumer is the done-means probe: pretend the
// SacValid$ read in effects' Sacrifice implementation was deleted (the scan
// drops that key from that primitive's read set) and assert the census then
// reports param:api:Sacrifice.SacValid for exactly the repo-deck cards whose
// Sacrifice abilities carry SacValid$ -- while the real baseline reports none.
func TestParamCensusDetectsADeletedConsumer(t *testing.T) {
	base, d := measureParamCensus(t, nil)
	if !d.api["Sacrifice"]["SacValid"] {
		t.Fatalf("scan no longer derives the SacValid$ read for api:Sacrifice -- the consumer was deleted for real; fix the census or re-seed")
	}
	for _, labels := range base.labels {
		for _, l := range labels {
			if l == "param:api:Sacrifice.SacValid" {
				t.Fatalf("baseline already reports param:api:Sacrifice.SacValid -- SacValid$ is unread on main; this probe is stale")
			}
		}
	}
	// The cards the probe must move: every deck card carrying SacValid$ on an
	// api:Sacrifice ability, computed by the same walk the census uses.
	carriers := map[string]bool{}
	reg := testutil.CorpusRegistry(t)
	for _, name := range testutil.RepoDeckNames() {
		for _, c := range testutil.RepoDeck(t, reg, name) {
			cn := c.Faces[0].Name
			for _, f := range c.Faces {
				var w func(sa *cards.SA)
				w = func(sa *cards.SA) {
					if sa == nil {
						return
					}
					if sa.API == "Sacrifice" {
						if _, ok := sa.Params["SacValid"]; ok {
							carriers[cn] = true
						}
					}
					w(sa.Sub)
				}
				for _, a := range f.Abilities {
					w(a)
				}
				for _, tr := range f.Triggers {
					w(tr.Effect)
				}
				for _, r := range f.Repls {
					w(r.With)
				}
			}
		}
	}
	if len(carriers) == 0 {
		t.Fatalf("no repo-deck card carries SacValid$ on an api:Sacrifice ability -- probe measures nothing")
	}
	res, _ := measureParamCensus(t, map[string]map[string]bool{"api:Sacrifice": {"SacValid": true}})
	for card := range carriers {
		labels := res.labels[card]
		hit := false
		for _, l := range labels {
			if l == "param:api:Sacrifice.SacValid" {
				hit = true
				break
			}
		}
		if !hit {
			t.Errorf("%s: dropped SacValid$ read but census did not report it (labels %v)", card, labels)
			continue
		}
		t.Logf("deleted-consumer probe: %s reports param:api:Sacrifice.SacValid", card)
	}
	if t.Failed() {
		return
	}
	affected := 0
	for card, labels := range res.labels {
		hit := false
		for _, l := range labels {
			if l == "param:api:Sacrifice.SacValid" {
				hit = true
				break
			}
		}
		if hit {
			affected++
			if !carriers[card] {
				t.Errorf("%s reported param:api:Sacrifice.SacValid but carries no SacValid$ on a Sacrifice ability", card)
			}
		}
	}
	if affected != len(carriers) {
		t.Errorf("probe reported %d affected cards, %d carriers measured", affected, len(carriers))
	}
}

// TestParamCensusPinsTheImportReviewExamples pins the examples the task
// brief was written from: Force of Will's ExileFromHand<1/Card.Blue+Other>
// and Daze's Return<1/Island> alternative costs (ParseCost silently
// substituting generic mana), Chandra's SubCounter<X/LOYALTY> and Whirler
// Rogue's tapXType<2/Artifact> activation costs. (The brief's other
// example, Reanimate's GainControl$ on api:ChangeZone, is not in any repo
// deck; the same label appears in the baseline on Meathook Massacre II and
// retires the moment the read is implemented.)
func TestParamCensusPinsTheImportReviewExamples(t *testing.T) {
	res, _ := measureParamCensus(t, nil)
	want := map[string]string{
		"Force of Will":             "cost:ExileFromHand",
		"Daze":                      "cost:Return",
		"Chandra, Awakened Inferno": "cost:SubCounter",
		"Whirler Rogue":             "cost:tapXType",
	}
	for card, label := range want {
		found := false
		for _, l := range res.labels[card] {
			if l == label {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s: expected %s in the census (labels %v) -- the gap was fixed or the census went blind", card, label, res.labels[card])
		}
	}
}

// TestParseCostReportsUnmodelledCostTokens pins the reporting half: tokens
// ParseCost does not model land in Cost.Unknown; modelled ones never do.
func TestParseCostReportsUnmodelledCostTokens(t *testing.T) {
	cases := []struct {
		cost string
		want []string
	}{
		{"PayEnergy<X> Sac<1/Creature> Return<1/CARDNAME>", []string{"PayEnergy", "Return"}},
		{"PayLife<5>", nil},
		{"PayLife<X>", []string{"PayLife"}},
		{"2 U U Sac<1/Creature>", nil},
		{"AddCounter<1/M1M1>", []string{"AddCounter"}},
		{"", nil},
	}
	for _, tc := range cases {
		got := ParseCost(tc.cost).Unknown
		if !sameSet(orEmpty(got), orEmpty(tc.want)) {
			t.Errorf("ParseCost(%q).Unknown = %v, want %v", tc.cost, got, tc.want)
		}
	}
}

func orEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// distinctRepoDeckCards counts the distinct cards the census walked (for the
// report line).
func distinctRepoDeckCards(t *testing.T) int {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	seen := map[string]bool{}
	for _, name := range testutil.RepoDeckNames() {
		for _, c := range testutil.RepoDeck(t, reg, name) {
			seen[c.Faces[0].Name] = true
		}
	}
	return len(seen)
}
