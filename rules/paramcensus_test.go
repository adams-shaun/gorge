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
//   - the scan is data-flow shaped, not shape-matched: an access counts
//     wherever the Params map is reached -- `x.Params["k"]`, a LOCAL ALIAS
//     (`p := x.Params; p["k"]`), a HELPER-PASSED map (a function whose
//     parameter is a map[string]string, attributed through its call sites'
//     arguments), or a range. An aliased or helper-passed read the scanner
//     cannot attribute fails the rot guard, so a consumer cannot hide by
//     renaming the map.
//   - an unregistered read FAILS the ratchet (the rot guard): any
//     `.Params[...]` access the scanner cannot resolve -- a new dynamic key,
//     a new base variable, a read in a function no root reaches -- breaks
//     TestParamCensusScanIsComplete until it is classified, so the read sets
//     cannot silently rot.
//   - the rules-side generic machinery (cast/activation/legality/targeting)
//     applies to whatever primitive runs, so its SA reads join every api's
//     read set -- but the API-SPECIALISED rules paths (the mana-ability
//     offer/payment/resolution chain, the Charm mode ask/resume, the
//     unless-pay resume for Counter/CopySpellAbility) run only for their own
//     APIs, and apiSpecificRulesSA attributes their reads accordingly: the
//     mana path's `Amount$` read no longer masks api:Sacrifice's unread
//     `Amount$`.
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
// Scope: the census walks what cards.Face.Primitives walks (the
// Abilities/Triggers/Statics/Repls plus the SubAbility chains link.go
// resolved) AND every SVar body in each face's own SVar table, and censuses
// only REGISTERED primitives -- an unimplemented primitive is the first
// ratchet's business, and when one registers it automatically comes under
// this census. Keyword heads (kw:...) carry no parameter maps; their
// expansions are censused through the expanded abilities' APIs. The SVar
// walk is load-bearing, not decoration: ResolveSVar's production callers
// execute bodies the Link pass never attached -- Charm's `Choices$` entries
// (effects/misc.go effCharm), Repeat/RepeatEach's `RepeatSubAbility$`
// (effects/choose_control.go), Branch's True/FalseSubAbility$
// (rules/resolution.go) and the delayed trigger's Execute$
// (rules/trigger_match.go) -- so a body an execution-bearing parameter
// names must be censused like a linked sub-ability. The walk is the
// conservative superset: EVERY face SVar is parsed, not only the ones a
// parameter reference can be traced through, so a new execution-bearing
// parameter cannot silently open a blind spot. Ability-less bodies
// (`SVar:X:Count$...` expressions behind ConditionCheckSVar$/SVarCompare$)
// fail parseSA and drop out; cycle protection is ResolveSVar's depth cap
// plus one visit per SVar name.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
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
	// selector bases: r.With and m.repl.With are cards.Repl's resolved
	// With *cards.SA (the ReplaceWith$ body: a real SA parameter map, read
	// as generic machinery), rp.sa the resume plan's SA, o.Ability the
	// stack object's resolved SA.
	"r.With": bSA, "m.repl.With": bSA, "rp.sa": bSA, "o.Ability": bSA,
	// index bases: candidates/rc.cands are []replMatch (the phase-
	// replacement pipeline and its parked-choice resume), so element .repl
	// is the same cards.Repl pair the named bases read.
	"candidates[i].repl": bRepl, "rc.cands[i].repl": bRepl,
}

// callSite records one package-local call with its string-literal arguments
// (for `.Params[paramIdent]` resolution) -- args[i] is "" when argument i is
// not a string literal.
type callSite struct {
	callee string
	args   []string
	// exprs renders each argument expression's text ("c.SVars",
	// "sa.Params", a plain identifier) for map[string]string-parameter
	// attribution: args[i] is the rendered text, "" when not renderable.
	exprs []string
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
	// mapParams records parameters whose declared type is map[string]string
	// (the Params maps' type): an index read on one is a potential Params
	// read attributed through the call sites' arguments, never silently
	// dropped.
	mapParams map[string]int
	// mapIdxRds records literal keys indexed on a mapParams parameter
	// ("" marks a dynamic key, which cannot be attributed and fails).
	mapIdxRds map[string]map[string]bool
	// stringMapResults holds this function's result POSITIONS typed
	// map[string]string, so a local assigned from a call can be tracked.
	stringMapResults []int
	// aliases maps a local name to the base text of the Params map it was
	// assigned (`params := sa.Params` -> "sa", chains resolved).
	aliases map[string]string
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
	// guardErrs carries rotGuard's findings; failGuard turns them into test
	// failures, and the probes inspect the list directly.
	guardErrs []string
	// mapAttributed records which map[string]string parameters had at least
	// one attributed call site ("pkg:fn:param"), set by propagateKeyReads.
	mapAttributed map[string]bool
	// mapArgFailures collects map[string]string-parameter call sites whose
	// argument could not be attributed to a Params map, resolved during
	// propagateKeyReads and surfaced by rotGuard.
	mapArgFailures []string
	// ambiguousStat records activeStatics calls whose mode is not a literal,
	// for rotGuard to reconcile against handRoots.stat.
	ambiguousStat map[string][]string
	// complete marks a scan over the REAL packages (scanPackages): the
	// apiSpecificRulesSA staleness check only makes sense there -- a probe's
	// synthetic scan deliberately contains none of the real functions.
	complete bool
}

// newScan builds an empty scan; scanPackages and the synthetic-source probes
// both start here.
func newScan() *scan {
	return &scan{
		fns:         map[string]*fnInfo{},
		apiImpl:     map[string]string{},
		statRoots:   map[string]map[string]bool{},
		modeFns:     map[string]string{},
		dispatchFns: map[string]bool{},
	}
}

// scanPackages parses the non-test sources of the rules package (dir ".",
// where a rules test binary runs) and the effects package ("../effects").
func scanPackages(t *testing.T) *scan {
	t.Helper()
	s := newScan()
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
			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatalf("paramcensus: read %s: %v", f, err)
			}
			s.scanSource(t, f, spec.pkg, string(src))
		}
	}
	s.propagateKeyReads()
	s.complete = true
	return s
}

// scanSource scans one compilation unit from its bytes -- the real packages'
// files and the probes' synthetic sources all come through here. Pass one
// declares every function's signature shape (string params, map[string]string
// params, map[string]string result positions) so pass two can classify
// aliases, helper-passed maps and call-returned maps without declaration-order
// dependence; pass two collects reads, calls and attribution roots.
func (s *scan) scanSource(t *testing.T, path, pkg, src string) {
	fset := token.NewFileSet()
	af, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		t.Fatalf("paramcensus: parse %s: %v", path, err)
	}
	for _, decl := range af.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		fi := s.fnOf(pkg, s.funcName(fd))
		position := 0
		for _, p := range fd.Type.Params.List {
			for _, pn := range p.Names {
				fi.sig = append(fi.sig, pn.Name)
				if pt, ok := p.Type.(*ast.Ident); ok && pt.Name == "string" {
					fi.strParams[pn.Name] = position
				}
				if isStringMap(p.Type) {
					fi.mapParams[pn.Name] = position
				}
				position++
			}
			if len(p.Names) == 0 {
				position++
			}
		}
		if fd.Type.Results != nil {
			rpos := 0
			for _, r := range fd.Type.Results.List {
				if isStringMap(r.Type) {
					for range max(1, len(r.Names)) {
						fi.stringMapResults = append(fi.stringMapResults, rpos)
						rpos++
					}
				}
				rpos += max(1, len(r.Names))
			}
		}
	}
	for _, decl := range af.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		s.scanFuncBody(t, fset, fd, pkg)
	}
}

// funcName renders a declaration's name the way fnInfo keys it: methods keep
// their receiver type ("Engine.triggerMatches").
func (s *scan) funcName(fd *ast.FuncDecl) string {
	name := fd.Name.Name
	if fd.Recv != nil && len(fd.Recv.List) > 0 {
		if star, ok := fd.Recv.List[0].Type.(*ast.StarExpr); ok {
			if id, ok := star.X.(*ast.Ident); ok {
				name = id.Name + "." + name
			}
		}
	}
	return name
}

// isStringMap reports whether the type expression is exactly
// map[string]string -- the type of every cards Params map.
func isStringMap(e ast.Expr) bool {
	mt, ok := e.(*ast.MapType)
	if !ok {
		return false
	}
	k, kok := mt.Key.(*ast.Ident)
	v, vok := mt.Value.(*ast.Ident)
	return kok && vok && k.Name == "string" && v.Name == "string"
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
		mapParams: map[string]int{},
		aliases:   map[string]string{},
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
// lookup: plain identifiers and selector/index chains of any depth
// (m.repl, pt.SA, m.repl.With, rc.cands[i].repl). An index expression
// renders with a fixed "[i]" placeholder: the bucket depends on the ELEMENT
// type (which slice it indexes), never on which element, so one table entry
// covers every index. A chain whose head is not renderable (a call result,
// a composite) renders "" and fails the rot guard.
func exprText(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		if x := exprText(v.X); x != "" {
			return x + "." + v.Sel.Name
		}
	case *ast.IndexExpr:
		if x := exprText(v.X); x != "" {
			return x + "[i]"
		}
	}
	return ""
}

// scanFile parses one source file: signatures, direct reads, calls, call-site
// literals, Register/activeStatics/dispatch-switch attribution, and the
// range-over-Params whitelist pattern (see scanRangeWhitelist).
// scanFuncBody is pass two for one function: writes, the Params-alias
// pre-pass, then reads/calls/roots. Aliases and helper-passed maps are
// classified here so a Params read cannot hide behind a renamed map.
func (s *scan) scanFuncBody(t *testing.T, fset *token.FileSet, fd *ast.FuncDecl, pkg string) {
	name := s.funcName(fd)
	fi := s.fnOf(pkg, name)
	// Assignment LHS index expressions are writes (`copy.Params["Produced"] = c`),
	// never reads.
	writes := map[ast.Node]bool{}
	ast.Inspect(fd.Body, func(n ast.Node) bool {
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
	// Alias pre-pass: every local assigned a `.Params` selector (directly or
	// through another alias) becomes an alias; locals assigned another
	// selector's map, a map[string]string composite/make, or a call whose
	// result at that position is a map[string]string are tracked so an index
	// READ on them is classified rather than silently dropped.
	otherAliases := map[string]bool{}
	localStringMaps := map[string]bool{}
	s.collectAliases(fd.Body, fi, otherAliases, localStringMaps)
	s.scanDispatchSwitch(t, fset, fd, pkg)
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.IndexExpr:
			if writes[v] {
				return true
			}
			switch xb := v.X.(type) {
			case *ast.SelectorExpr:
				if xb.Sel.Name != "Params" {
					return true // .SVars and every other map field: not a param read
				}
				s.recordParamsRead(t, fset, fi, name, v, exprText(xb.X), pkg)
			case *ast.Ident:
				if base, aliased := fi.aliases[xb.Name]; aliased {
					s.recordParamsRead(t, fset, fi, name, v, base, pkg)
					return true
				}
				if _, isMapParam := fi.mapParams[xb.Name]; isMapParam {
					s.recordMapParamRead(t, fset, fi, name, v, xb.Name)
					return true
				}
				if localStringMaps[xb.Name] {
					s.failf(t, fset.Position(v.Pos()),
						"%s: index read on local map[string]string %q that is not a tracked Params alias -- classify it", name, xb.Name)
					return true
				}
				if otherAliases[xb.Name] {
					return true // an alias of a non-Params map field: not a param read
				}
				// Anything else indexed under a plain identifier is a slice,
				// array or non-string map -- never a Params map (whose only
				// shapes are the selector, alias, helper-param and local
				// forms classified above).
				return true
			default:
				// A call result (or composite) indexed directly is a slice or a
				// non-Params map -- never a Params read (a call returning an SA
				// reaches the map only through a `.Params` selector, which the
				// case above classifies). One exception fails loudly: a call
				// whose position-0 result is itself a map[string]string must
				// not be indexed without classification.
				if ce, ok := v.X.(*ast.CallExpr); ok {
					if id, ok := ce.Fun.(*ast.Ident); ok {
						if target := s.fns[fi.pkg+":"+id.Name]; target != nil && slices.Contains(target.stringMapResults, 0) {
							s.failf(t, fset.Position(v.Pos()),
								"%s: indexes the map[string]string result of %s directly -- assign it to a tracked local or classify it", name, id.Name)
						}
					}
				}
				return true
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

// recordParamsRead records one `.Params[...]` (or alias) access on the given
// base text -- the shared read logic for selector and aliased forms.
func (s *scan) recordParamsRead(t *testing.T, fset *token.FileSet, fi *fnInfo, name string, v *ast.IndexExpr, base, pkg string) {
	b, ok := s.bucketOf(t, fset, v.Pos(), base, pkg)
	if !ok {
		return
	}
	switch idx := v.Index.(type) {
	case *ast.BasicLit:
		if idx.Kind != token.STRING {
			s.failf(t, fset.Position(v.Pos()), "%s: non-string literal Params key %s", name, idx.Value)
			return
		}
		key, err := strconv.Unquote(idx.Value)
		if err != nil {
			s.failf(t, fset.Position(v.Pos()), "%s: unparseable Params key %s", name, idx.Value)
			return
		}
		s.addRead(fi, b, key)
	case *ast.Ident:
		if _, isParam := fi.strParams[idx.Name]; isParam {
			s.addKeyRead(fi, b, idx.Name)
			return
		}
		s.failf(t, fset.Position(v.Pos()),
			"%s: dynamic Params key %q that is not a function parameter -- resolve it via a parameter or classify it", name, idx.Name)
	default:
		s.failf(t, fset.Position(v.Pos()), "%s: unclassifiable Params key expression %T", name, v.Index)
	}
}

// recordMapParamRead records an index on a map[string]string parameter (the
// helper-passed-map form). Literal keys are attributed through the call
// sites' arguments in propagateKeyReads; a dynamic key cannot be attributed
// and fails. Whitelisted non-card maps (parseStaticLine's SVar table, ...) skip
// attribution entirely.
func (s *scan) recordMapParamRead(t *testing.T, fset *token.FileSet, fi *fnInfo, name string, v *ast.IndexExpr, param string) {
	if _, skipped := stringMapParams[fi.pkg+":"+fi.name+":"+param]; skipped {
		return
	}
	var key string
	switch idx := v.Index.(type) {
	case *ast.BasicLit:
		if idx.Kind == token.STRING {
			if k, err := strconv.Unquote(idx.Value); err == nil {
				key = k
			}
		}
		if key == "" {
			s.failf(t, fset.Position(v.Pos()), "%s: non-string literal key on map[string]string parameter %q", name, param)
			return
		}
	case *ast.Ident:
		if _, isStr := fi.strParams[idx.Name]; isStr {
			// A dynamic key taken from a string parameter: the read is real
			// but its key is only known at the call sites, where BOTH the
			// key argument and the map argument must be attributed.
			key = ""
		} else {
			s.failf(t, fset.Position(v.Pos()),
				"%s: dynamic key %q on map[string]string parameter %q is not a function parameter -- classify it", name, idx.Name, param)
			return
		}
	default:
		s.failf(t, fset.Position(v.Pos()), "%s: unclassifiable key expression on map[string]string parameter %q", name, param)
		return
	}
	if fi.mapIdxRds == nil {
		fi.mapIdxRds = map[string]map[string]bool{}
	}
	if fi.mapIdxRds[param] == nil {
		fi.mapIdxRds[param] = map[string]bool{}
	}
	fi.mapIdxRds[param][key] = true
}

// collectAliases finds the Params-alias and local-string-map locals of one
// function body: single assignments (`x := y.Params`, `x := y.SVars`,
// `x := otherAlias`, `x := make(map[string]string...)`,
// `x := map[string]string{...}`, `x := f(...)` with a map[string]string
// result at that position) and var declarations of the same shapes.
func (s *scan) collectAliases(body *ast.BlockStmt, fi *fnInfo, otherAliases, localStringMaps map[string]bool) {
	track := func(lhs *ast.Ident, rhs ast.Expr) {
		rhs = ast.Unparen(rhs)
		switch r := rhs.(type) {
		case *ast.SelectorExpr:
			if r.Sel.Name == "Params" {
				fi.aliases[lhs.Name] = exprText(r.X)
			} else {
				otherAliases[lhs.Name] = true
			}
		case *ast.Ident:
			if base, ok := fi.aliases[r.Name]; ok {
				fi.aliases[lhs.Name] = base
			} else if otherAliases[r.Name] {
				otherAliases[lhs.Name] = true
			}
		case *ast.CompositeLit:
			if isStringMap(r.Type) {
				localStringMaps[lhs.Name] = true
			}
		case *ast.CallExpr:
			if callee, ok := r.Fun.(*ast.Ident); ok {
				if target := s.fns[fi.pkg+":"+callee.Name]; target != nil {
					for _, pos := range target.stringMapResults {
						if pos == 0 {
							localStringMaps[lhs.Name] = true
						}
					}
				}
			}
		}
	}
	ast.Inspect(body, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.AssignStmt:
			if len(v.Rhs) == 1 && len(v.Lhs) == 1 {
				if id, ok := v.Lhs[0].(*ast.Ident); ok && (v.Tok == token.DEFINE || v.Tok == token.ASSIGN) {
					track(id, v.Rhs[0])
				}
			} else if len(v.Rhs) == 1 && len(v.Lhs) > 1 && v.Tok == token.DEFINE {
				// Multi-assign from one call: a map[string]string result at
				// position i makes LHS i a tracked local string map.
				if ce, ok := ast.Unparen(v.Rhs[0]).(*ast.CallExpr); ok {
					if callee, ok := ce.Fun.(*ast.Ident); ok {
						if target := s.fns[fi.pkg+":"+callee.Name]; target != nil {
							for _, pos := range target.stringMapResults {
								if pos < len(v.Lhs) {
									if id, ok := v.Lhs[pos].(*ast.Ident); ok {
										localStringMaps[id.Name] = true
									}
								}
							}
						}
					}
				}
			}
		case *ast.GenDecl:
			for _, spec := range v.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok || len(vs.Values) != len(vs.Names) {
					continue
				}
				for i, val := range vs.Values {
					if id := vs.Names[i]; id != nil {
						track(id, val)
					}
				}
			}
		}
		return true
	})
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
	exprs := make([]string, len(ce.Args))
	for i, a := range ce.Args {
		exprs[i] = argText(a)
		if lit, ok := a.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			if v, err := strconv.Unquote(lit.Value); err == nil {
				args[i] = v
			}
		}
	}
	fi.callSites = append(fi.callSites, callSite{callee: callee, args: args, exprs: exprs})
}

// argText renders an argument expression for map[string]string-parameter
// attribution: plain identifiers, one-level selector chains and parenthesised
// forms of those; anything else renders "" (unattributable, and the rot
// guard says so when it is a map argument).
func argText(e ast.Expr) string {
	switch v := ast.Unparen(e).(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		if x, ok := v.X.(*ast.Ident); ok {
			return x.Name + "." + v.Sel.Name
		}
	}
	return ""
}

// scanRangeWhitelist handles `for k := range X.Params` shapes (X a selector
// base or a tracked alias): a range over a Params map whose body switches on
// the range variable is a READ of the keys named in the case clauses (the
// whitelist). A range whose body does not match that shape is a rot-guard
// failure. Derived from the code, not hand-set.
func (s *scan) scanRangeWhitelist(t *testing.T, fset *token.FileSet, fi *fnInfo, fname string, rs *ast.RangeStmt, pkg string, writes map[ast.Node]bool) {
	var base string
	switch x := rs.X.(type) {
	case *ast.SelectorExpr:
		if x.Sel.Name != "Params" {
			return
		}
		base = exprText(x.X)
	case *ast.Ident:
		if b, ok := fi.aliases[x.Name]; ok {
			base = b
		} else {
			return // range over some other map: not a Params read
		}
	default:
		return
	}
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

// stringMapParams whitelists the map[string]string-typed parameters whose
// maps are NOT card Params maps, so their index reads are not attributed:
// each entry is "pkg:func:param" with its justification. Anything not listed
// here AND not called with a `.Params`/alias argument fails the rot guard.
var stringMapParams = map[string]string{
	// effects/misc.go parseStaticLine: svars is the face's SVars table (a
	// cards.SA's SVar: bodies), read by NAME to fetch a static line -- not a
	// card Params map.
	"effects:parseStaticLine:svars": "SVars table lookup by static-line name, not a card Params map",
	// effects/misc.go compoundRememberedSpec: params is the map parseStaticLine
	// built from one SVar static line -- its ValidCard$/ValidTarget$ keys are
	// consumed here, but the map originates in an SVar body, not a card's
	// Params map.
	"effects:compoundRememberedSpec:params": "keys of a parseStaticLine-built static line (an SVar body), not a card Params map",
}

// propagateKeyReads resolves two indirect read shapes:
//
//   - `.Params[paramIdent]` reads: every caller of a function that reads one
//     of its own string parameters as a key gains the literal keys passed at
//     those positions (hasStat / statInt / statList / actorMatches /
//     effects/count's Num-body).
//   - map[string]string-parameter reads (the helper-passed-map form): a
//     literal key indexed on such a parameter is attributed to the caller
//     with the BUCKET of the argument expression -- which must be a `.Params`
//     selector or a tracked Params alias; anything else is a
//     mapArgFailure the rot guard surfaces. parseStaticLine and
//     compoundRememberedSpec are whitelisted (stringMapParams): their maps
//     are SVar tables and parsed static lines, not card Params.
func (s *scan) propagateKeyReads() {
	s.mapAttributed = map[string]bool{}
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
			for param, keys := range target.mapIdxRds {
				idx, ok := target.mapParams[param]
				if !ok || idx >= len(cs.exprs) {
					continue
				}
				s.mapAttributed[target.pkg+":"+target.name+":"+param] = true
				arg := cs.exprs[idx]
				var base string
				switch {
				case strings.HasSuffix(arg, ".Params"):
					base = strings.TrimSuffix(arg, ".Params")
				default:
					if b, isAlias := fi.aliases[arg]; isAlias {
						base = b
						break
					}
					s.mapArgFailures = append(s.mapArgFailures, fmt.Sprintf(
						"%s calls %s passing %q for map[string]string parameter %q -- not a Params map; classify it in stringMapParams or fix the argument",
						fi.name, target.name, arg, param))
					continue
				}
				b, ok := s.bucketOf(nil, token.NewFileSet(), token.NoPos, base, fi.pkg)
				if !ok {
					continue // already recorded as unclassified by bucketOf
				}
				for key := range keys {
					if key == "" {
						s.mapArgFailures = append(s.mapArgFailures, fmt.Sprintf(
							"%s: map[string]string parameter %q of %s is indexed with a dynamic key -- classify it",
							target.name, param, fi.name))
						continue
					}
					s.addRead(fi, b, key)
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

// apiSpecificRulesSA names the rules functions whose SA (bSA) reads execute
// only inside the listed APIs' code paths -- NOT for every cast/activation/
// targeting of any primitive. Their direct SA reads are REMOVED from the
// generic rules union and attributed to exactly the named APIs, so the mana
// path's `Amount$`/`Produced$` reads no longer mask e.g. api:Sacrifice's
// genuinely unread `Amount$`, and the unless-pay resume's `UnlessCost$` read
// no longer masks api:Sacrifice's unread `UnlessCost$`. Each entry was
// verified by reading its callers: every call site sits on a path only that
// API reaches (the mana-ability offer/payment/resolution chain, the Charm
// mode ask/resume pair, the modal-trigger placement ask, the unless-pay
// resume arms effCounter/effCopySpellAbility suspend with). The rot guard
// fails on a stale entry (renamed function, or one that no longer reads SA
// params); a NEW api-specialised path must be added here or its reads
// over-suppress every other API's real gaps.
var apiSpecificRulesSA = map[string][]string{
	// The mana-ability chain: activateMana through resolveManaEffectColor,
	// the AvailableMana projection, and activatedMatchesValidSA's
	// Produced$-based mana-ability recognition -- all run on mana abilities
	// (api:Mana) only.
	"Engine.activateMana":           {"Mana"},
	"Engine.manaAbilityPayable":     {"Mana"},
	"Engine.emitManaTap":            {"Mana"},
	"Engine.isTriggeredManaAbility": {"Mana"},
	"triggeredManaColourChoice":     {"Mana"},
	"Engine.resolveManaAbility":     {"Mana"},
	"Engine.resolveManaEffect":      {"Mana"},
	"manaColourPrompt":              {"Mana"},
	"Engine.AvailableMana":          {"Mana"},
	"addAvailable":                  {"Mana"},
	"availableAmount":               {"Mana"},
	"activatedMatchesValidSA":       {"Mana"},
	// The Charm mode paths: the CR 601.2b cast-time modes ask (castModeAsk),
	// the per-mode target declaration (modalTargetSA), the resume-side mode
	// decisions/labels, and the modal-trigger placement ask (CharmNum$).
	"Engine.castModeAsk":     {"Charm"},
	"modalTargetSA":          {"Charm"},
	"modeDecision":           {"Charm"},
	"modeDecisionForChoices": {"Charm"},
	"modeLabels":             {"Charm"},
	"modeChoiceNames":        {"Charm"},
	"Engine.askTriggerModes": {"Charm"},
	// The unless-pay resume arm: only effCounter and effCopySpellAbility
	// suspend with an UnlessCost$ ask, so resumeResolution's UnlessCost$
	// read belongs to those two APIs alone.
	"Engine.resumeResolution": {"Counter", "CopySpellAbility"},
	// The ward payment path: only the Ward keyword's expanded trigger
	// reaches these (resumeResolution dispatches on rp.sa.API == "Ward"),
	// so their UnlessCost$ reads belong to api:Ward alone -- left in the
	// generic union they would mask every other API's unread UnlessCost$
	// (measured: api:Tap on Blood Crypt/Hallowed Fountain, api:Sacrifice on
	// Vexing Devil, api:LoseLife on Torment of Hailfire's shape).
	"Engine.beginWardPayment":  {"Ward"},
	"Engine.settleWardPayment": {"Ward"},
	// The opening-hand pregame actions: applyOpeningEffect, its delayed-
	// trigger registration and the answer handler run ONLY on the expanded
	// opening-action SVar of a MayEffectFromOpeningHand keyword (Chancellor
	// of the Tangle, Gemstone Caverns, Impatient Iguana), so their
	// Origin$/Destination$/BecomeStartingPlayer$/Triggers$/SubAbility$
	// reads belong to the APIs such an action carries (ChangeZone for the
	// put-onto-battlefield shape, PutCounter for the counter rider, Effect
	// for the delayed-trigger shape) -- left in the generic union they
	// would mask every other API's unread Destination$ (measured:
	// api:Counter on Force of Will and Remand).
	"Engine.applyOpeningEffect":            {"ChangeZone", "PutCounter", "Effect"},
	"Engine.registerOpeningEffectTriggers": {"ChangeZone", "PutCounter", "Effect"},
	"Engine.handleOpening":                 {"ChangeZone", "PutCounter", "Effect"},
}

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
		"Continuous": {"Engine.staticEffects", "warpGraveyardAllowed"},
		// warpGraveyardAllowed scans Continuous MayPlay statics directly
		// over the face's Statics slice (Timeline Culler's explicit
		// graveyard-Warp permission), with no activeStatics call -- the
		// same direct-scan shape mustAttackRequired has.
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
	// ETBReplacement repl's Keyword$ at cast-offer time. handleReplacement is
	// the parked-repl-choice decision handler (it resumes the parked phase
	// machinery and reads the parked repl's Optional$ directly), reached
	// through the decision resume path rather than the pipeline.
	repl: []string{"Engine.applyReplacements", "Engine.collectETBChoices",
		"Engine.handleReplacement"},
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
	// whatever primitive is being cast, activated or targeted. The
	// API-SPECIALISED rules paths (apiSpecificRulesSA) are excluded from the
	// union and attributed to their own APIs only -- the mana path's
	// Amount$/Produced$ reads must not mark Amount$ read for api:Sacrifice.
	rulesAllSA := map[string]bool{}
	specialised := map[string]map[string]bool{} // api -> direct SA reads
	excludedSA := map[string]bool{}
	for fn, apis := range apiSpecificRulesSA {
		excludedSA[fn] = true
		fi := s.fns["rules:"+fn]
		if fi == nil {
			continue // rotGuard already failed on the stale entry
		}
		for k := range fi.reads[bSA] {
			for _, api := range apis {
				if specialised[api] == nil {
					specialised[api] = map[string]bool{}
				}
				specialised[api][k] = true
			}
		}
	}
	for key, fi := range s.fns {
		if !strings.HasPrefix(key, "rules:") || excludedSA[fi.name] {
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
		for k := range specialised[api] {
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

// rotGuard collects every way a read could have escaped the census into
// s.guardErrs:
//
//   - any unclassified access collected during the scan;
//   - an effects function reading Params but unreachable from every
//     Register'd implementation (a primitive implemented but never
//     registered, or a helper only dead code reaches);
//   - a rules function reading trigger/static/replacement params outside
//     every attribution root for that bucket;
//   - a map[string]string parameter indexed but never attributed through a
//     `.Params`/alias call-site argument, or an unattributable argument;
//   - a stale apiSpecificRulesSA entry (the function is gone or no longer
//     reads SA params).
//
// A new read therefore cannot join the codebase without either being
// attributed or failing the guard; failGuard turns the findings into test
// failures and the probes inspect the list directly.
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
			s.guardErrs = append(s.guardErrs, fmt.Sprintf(
				"paramcensus: activeStatics non-literal mode call in %s (%s) has no handRoots.stat attribution",
				fname, strings.Join(sites, ", ")))
		}
	}
	if len(s.unclassified) > 0 {
		sort.Strings(s.unclassified)
		s.guardErrs = append(s.guardErrs, fmt.Sprintf("paramcensus: %d unclassified Params reads (the census cannot rot):\n%s",
			len(s.unclassified), strings.Join(s.unclassified, "\n")))
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
			s.guardErrs = append(s.guardErrs, fmt.Sprintf(
				"paramcensus: effects function %s reads Params but is unreachable from every Register'd implementation", fi.name))
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
				s.guardErrs = append(s.guardErrs, fmt.Sprintf(
					"paramcensus: rules function %s reads %s params outside every attribution root for that bucket", fi.name, b))
			}
		}
	}
	// Helper-passed maps: every indexed map[string]string parameter must have
	// been attributed through a call site (whitelisted non-card maps never
	// enter mapIdxRds at all).
	for key, fi := range s.fns {
		for param := range fi.mapIdxRds {
			if !s.mapAttributed[key+":"+param] {
				s.guardErrs = append(s.guardErrs, fmt.Sprintf(
					"paramcensus: %s indexes map[string]string parameter %q but no call site passes a Params map -- the helper is unreachable or mis-fed", fi.name, param))
			}
		}
	}
	sort.Strings(s.mapArgFailures)
	s.guardErrs = append(s.guardErrs, s.mapArgFailures...)
	if !s.complete {
		return
	}
	// apiSpecificRulesSA must name real rules functions that still read SA
	// params -- a renamed or refactored function must not leave a silent
	// stale entry behind.
	for fn := range apiSpecificRulesSA {
		fi, ok := s.fns["rules:"+fn]
		if !ok {
			s.guardErrs = append(s.guardErrs, fmt.Sprintf(
				"paramcensus: apiSpecificRulesSA names rules function %q, which no longer exists -- rename the entry", fn))
			continue
		}
		if len(fi.reads[bSA]) == 0 {
			s.guardErrs = append(s.guardErrs, fmt.Sprintf(
				"paramcensus: apiSpecificRulesSA entry %q no longer reads SA params -- delete the stale entry", fn))
		}
	}
}

// failGuard fails the test if rotGuard collected any findings.
func (s *scan) failGuard(t *testing.T) {
	t.Helper()
	if len(s.guardErrs) > 0 {
		sort.Strings(s.guardErrs)
		t.Fatalf("paramcensus rot guard: %d findings:\n%s", len(s.guardErrs), strings.Join(s.guardErrs, "\n"))
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
// cards/link.go, cards/keywords.go) rather than any effects/rules
// implementation: the line heads, the chain pointers, and the keyword-
// expansion tags. They are never `.Params[...]` read in Go because the parser
// turned them into structure (Trigger.Mode, Repl.With, sa.Sub...) before any
// primitive ran.
//
// Keyword and KeywordLine are generated by cards/keywords.go's
// expandKeywords, not read from any Forge script (measured at the corpus
// pin: ZERO raw cardsfolder lines carry `| Keyword$ ` or `KeywordLine$` --
// re-measure that claim before trusting this classification for a new
// corpus): every expanded ability/trigger/replacement is tagged
// Params["Keyword"] (head) and Params["KeywordLine"] (the full line) so
// nothing downstream needs to re-derive the difference. Both tags are
// consumed structurally: the KeywordLine tag IS the idempotence check in
// cards/keywords.go's `has` closure (the T:/R:/A: lookups at the top of
// expandKeywords), and rules/cast.go's collectETBChoices reads the Keyword
// tag ("ETBReplacement") to find an ETB replacement's target options. They
// are therefore never a per-primitive script parameter and are never
// measured.
var structuralKeys = func() map[string]map[string]bool {
	m := map[string]map[string]bool{
		"sa":   {"SubAbility": true},
		"trig": {"Mode": true, "Execute": true},
		"stat": {"Mode": true},
		"repl": {"Event": true, "ReplaceWith": true},
	}
	for _, keys := range m {
		keys["Keyword"] = true
		keys["KeywordLine"] = true
	}
	return m
}()

// ignoredParamKeys are keys the census deliberately does NOT report, each
// with the Forge behaviour that makes it presentation/AI-only. Verified
// against the pinned Forge checkout (~/projects/ref/forge; grep the key to
// see every consumer).
var ignoredParamKeys = map[string]string{
	// Forge citations below are relative to ~/projects/ref/forge. These keys
	// affect Forge AI or rendered text only, never its game rules.
	"AILogic":            "AI heuristic; forge-game/src/main/java/forge/game/card/CardFactoryUtil.java",
	"AITgts":             "AI target hint; forge-game/src/main/java/forge/game/card/CardState.java",
	"AILifeThreshold":    "AI life threshold; forge-ai/src/main/java/forge/ai/AiController.java",
	"AINoRecursiveCheck": "AI recursion guard; forge-ai/src/main/java/forge/ai/ability/DelayedTriggerAi.java",
	"AIPhyrexianPayment": "AI payment preference; forge-ai/src/main/java/forge/ai/ComputerUtilMana.java",
	// SpellDescription is deliberately NOT ignored: legalActions renders it.
	"StackDescription":      "stack caption; forge-game/src/main/java/forge/game/card/CardFactory.java",
	"TriggerDescription":    "trigger caption; forge-game/src/main/java/forge/game/card/CardFactoryUtil.java",
	"ChangeTypeDesc":        "search UI text; forge-game/src/main/java/forge/game/ability/effects/ChangeZoneEffect.java",
	"ValidDescription":      "valid-card UI text; forge-game/src/main/java/forge/game/card/Card.java",
	"CostDesc":              "alternate-cost UI text; forge-game/src/main/java/forge/game/card/CardFactoryUtil.java",
	"ConditionDescription":  "condition display text; forge-game/src/main/java/forge/game/ability/SpellAbilityEffect.java",
	"PrecostDesc":           "cost-prompt prefix; forge-game/src/main/java/forge/game/card/CardFactoryUtil.java",
	"TgtPrompt":             "target-prompt UI text; forge-game/src/main/java/forge/game/card/CardFactoryUtil.java",
	"ChoiceTitle":           "choice-dialog title; forge-game/src/main/java/forge/game/card/CardFactoryUtil.java",
	"AdditionalDescription": "extra UI rules-text fragment; forge-game/src/main/java/forge/game/ability/effects/CharmEffect.java",
	"VoteMessage":           "vote-dialog message; forge-game/src/main/java/forge/game/ability/effects/VoteEffect.java",
	"IsCurse":               "AI curse marker; forge-game/src/main/java/forge/game/spellability/SpellAbility.java",
	"Description":           "UI/dialog description; forge-game/src/main/java/forge/game/card/CardFactory.java",
	"SelectPrompt":          "search prompt message; forge-game/src/main/java/forge/game/ability/effects/ChangeZoneEffect.java",
	"Image":                 "effect-token image key; forge-game/src/main/java/forge/game/card/CardFactory.java",
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
			s.failGuard(t)
			censusReads = s.derived()
			censusBase = walkRepoDeckCensus(t, censusReads, nil)
		})
		return censusBase, censusReads
	}
	s := scanPackages(t)
	s.rotGuard(t)
	s.failGuard(t)
	d := s.derived()
	return walkRepoDeckCensus(t, d, drop), d
}

// faceUnknownCostLabels returns every unmodelled cost token from every
// face-owned value production passes to ParseCost: the printed ManaCost and
// the four parameterised casting keywords. SA/static Cost$ and UnlessCost$
// values are walked beside their owning primitive below. Keeping the
// face-owned sources together is deliberate: a new printed or keyword cost
// cannot silently evade the ratchet merely because it is not a Params entry.
func faceUnknownCostLabels(f *cards.Face) []string {
	inputs := []string{f.ManaCost}
	for _, keyword := range [...]string{"Kicker", "Surge", "Flashback", "Miracle"} {
		if cost, ok := f.KeywordParam(keyword); ok {
			inputs = append(inputs, cost)
		}
	}
	seen := map[string]bool{}
	for _, input := range inputs {
		for _, token := range ParseCost(input).Unknown {
			seen["cost:"+token] = true
		}
	}
	out := make([]string, 0, len(seen))
	for label := range seen {
		out = append(out, label)
	}
	sort.Strings(out)
	return out
}

// cardCensusLabels is the card-side census walk shared by the repo-deck
// ratchet and focused fixtures. Params are labelled only for REGISTERED
// primitives (an unimplemented primitive's unread params are noise -- the
// first ratchet owns the card); cost tokens are labelled regardless, because
// an unmodelled token in a cost string is a real silent substitution even
// when the primitive around it is already unsupported.
//
// The walk also parses EVERY SVar in each face's own SVar table through
// cards.ResolveSVar and censuses the resulting bodies like linked
// sub-abilities. This is the conservative superset of walking only the
// execution-bearing SVar-reference parameters -- `Choices$` (effCharm),
// `RepeatSubAbility$`/`RepeatEach` (effRepeat/effRepeatEach), Branch's
// True/FalseSubAbility$ (resumeResolution) and the delayed trigger's
// `Execute$` (checkDelayedTriggers) -- so the next execution-bearing
// parameter is covered without touching this walk (see the file-head scope
// note). One visit per name; ResolveSVar's depth cap bounds any body's own
// SubAbility$ chain.
func cardCensusLabels(c *cards.Card, d *derivedReads, drop map[string]map[string]bool) []string {
	labels := map[string]bool{}
	addLabel := func(l string) { labels[l] = true }
	labelFor := func(prim, key string) string { return "param:" + prim + "." + key }
	for _, f := range c.Faces {
		for _, label := range faceUnknownCostLabels(f) {
			addLabel(label)
		}
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
		// SVar bodies the Link pass did not attach (see the comment above):
		// one ResolveSVar per name; a body that is not an ability (Count$
		// expressions behind ConditionCheckSVar$/SVarCompare$) fails parseSA
		// and yields nil.
		for name := range f.SVars {
			walk(cards.ResolveSVar(f.SVars, name))
		}
	}
	out := make([]string, 0, len(labels))
	for label := range labels {
		out = append(out, label)
	}
	sort.Strings(out)
	return out
}

// walkRepoDeckCensus applies the card-side walk to each distinct repo-deck
// card and records its per-card plus aggregate labels.
func walkRepoDeckCensus(t *testing.T, d *derivedReads, drop map[string]map[string]bool) censusResult {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	res := censusResult{labels: map[string][]string{}}
	paramSeen := map[string]bool{}
	costSeen := map[string]bool{}
	cardSeen := map[string]bool{}
	for _, name := range testutil.RepoDeckNames() {
		for _, c := range testutil.RepoDeck(t, reg, name) {
			cardName := c.Faces[0].Name
			if cardSeen[cardName] {
				continue
			}
			cardSeen[cardName] = true
			labels := cardCensusLabels(c, d, drop)
			if len(labels) == 0 {
				continue
			}
			res.labels[cardName] = labels
			for _, label := range labels {
				if strings.HasPrefix(label, "param:") {
					paramSeen[label] = true
				} else {
					costSeen[label] = true
				}
			}
		}
	}
	for label := range paramSeen {
		res.paramLabels = append(res.paramLabels, label)
	}
	for label := range costSeen {
		res.costLabels = append(res.costLabels, label)
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
	"Abbot of Keral Keep":         {"param:api:Cleanup.ClearRemembered", "param:api:Dig.RememberChanged", "param:api:Effect.ExileOnMoved"},
	"Ad Nauseam":                  {"param:api:Cleanup.ClearRemembered", "param:api:Dig.RememberChanged", "param:api:Dig.Reveal", "param:api:Repeat.RepeatOptional"},
	"Adaptive Automaton":          {"param:api:ChooseType.Type"},
	"Aftermath Analyst":           {"param:api:ChangeZoneAll.Tapped"},
	"Ancient Stirrings":           {"param:api:Dig.ForceRevealToController"},
	"Angelic Accord":              {"param:trig:Phase.CheckSVar", "param:trig:Phase.SVarCompare"},
	"Angelic Overseer":            {"param:stat:Continuous.IsPresent"},
	"Angelic Skirmisher":          {"param:api:Pump.KWChoice"},
	"Army of the Damned":          {"param:api:Token.TokenTapped"},
	"Assassin's Trophy":           {"param:api:ChangeZone.ShuffleNonMandatory"},
	"Auriok Steelshaper":          {"param:stat:Continuous.IsPresent", "param:stat:ReduceCost.ValidSpell"},
	"Azusa, Lost but Seeking":     {"param:stat:Continuous.AdjustLandPlays"},
	"Baloth Prime":                {"param:api:PutCounter.ETB", "param:api:Token.TokenTapped"},
	"Banisher Priest":             {"param:api:ChangeZone.Duration"},
	"Banishing Light":             {"param:api:ChangeZone.Duration"},
	"Bile Blight":                 {"param:api:Cleanup.ClearRemembered", "param:api:Pump.RememberTargets"},
	"Blazemire Verge":             {"param:api:Mana.IsPresent"},
	"Blood Crypt":                 {"param:api:Tap.UnlessCost", "param:api:Tap.UnlessPayer"},
	"Bloodchief Ascension":        {"param:trig:Phase.CheckSVar", "param:trig:Phase.SVarCompare"},
	"Bloodsoaked Champion":        {"param:api:ChangeZone.CheckSVar"},
	"Borderland Ranger":           {"param:api:ChangeZone.ShuffleNonMandatory"},
	"Braids, Arisen Nightmare":    {"param:api:Cleanup.ClearRemembered", "param:api:Draw.ConditionCheckSVar", "param:api:Draw.ConditionSVarCompare", "param:api:LoseLife.ConditionCheckSVar", "param:api:LoseLife.ConditionSVarCompare", "param:api:Sacrifice.Amount", "param:api:Sacrifice.Optional"},
	"Brainstorm":                  {"param:api:ChangeZone.Mandatory", "param:api:ChangeZone.Reorder"},
	"Burning Wish":                {"param:api:ChangeZone.Hidden", "param:api:ChangeZone.Reveal"},
	"Cavern of Souls":             {"param:api:ChooseType.Type", "param:api:Mana.AddsNoCounter", "param:api:Mana.RestrictValid"},
	"Celestial Colonnade":         {"param:api:Animate.Colors", "param:api:Animate.Keywords", "param:api:Animate.OverwriteColors"},
	"Chain Lightning":             {"param:api:CopySpellAbility.Controller"},
	"Chalice of the Void":         {"param:api:PutCounter.ETB"},
	"Chandra, Awakened Inferno":   {"cost:SubCounter", "param:api:Cleanup.ClearRemembered", "param:api:DealDamage.ReplaceDyingDefined", "param:api:DealDamage.Ultimate", "param:api:Effect.EffectOwner", "param:api:Effect.Name"},
	"Chaos Warp":                  {"param:api:Dig.DestinationZone2", "param:api:Dig.LibraryPosition2", "param:api:Dig.Reveal"},
	"Conduit of Worlds":           {"param:api:Cleanup.ClearRemembered"},
	"Council's Judgment":          {"param:api:Vote.VoteCard", "param:api:Vote.VoteSubAbility"},
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
	"Endless One":                 {"param:api:PutCounter.ETB"},
	"Escape Tunnel":               {"param:api:Effect.ExileOnMoved"},
	"Evendo Brushrazer":           {"param:stat:Continuous.CheckSVar", "param:stat:Continuous.Condition"},
	"Exploration Broodship":       {"param:stat:Continuous.AddStaticAbility"},
	"Fabled Passage":              {"param:api:Cleanup.ClearRemembered"},
	"Flickerwisp":                 {"param:api:ChangeZone.Mandatory", "param:api:Cleanup.ClearRemembered", "param:api:DelayedTrigger.RememberObjects"},
	"Forbidding Watchtower":       {"param:api:Animate.Colors", "param:api:Animate.OverwriteColors"},
	"Force of Will":               {"param:api:Counter.Destination", "param:stat:AlternativeCost.EffectZone", "param:stat:AlternativeCost.ValidSA"},
	"Foreboding Ruins":            {"param:api:Tap.UnlessCost", "param:api:Tap.UnlessPayer"},
	"Forked Bolt":                 {"param:api:DealDamage.DividedAsYouChoose"},
	"Gamble":                      {"param:api:ChangeZone.Mandatory"},
	"Ghalta, Primal Hunger":       {"param:stat:ReduceCost.EffectZone"},
	"Ghost Quarter":               {"param:api:ChangeZone.ShuffleNonMandatory"},
	"Ghoulcaller's Chant":         {"param:api:ChangeZone.Mandatory"},
	"Giada, Font of Hope":         {"param:api:Mana.RestrictValid", "param:api:PutCounter.ETB"},
	"Goblin Guide":                {"param:api:Dig.LibraryPosition2", "param:api:Dig.Reveal"},
	"Grand Abolisher":             {"param:stat:CantBeActivated.AffectedZone", "param:stat:CantBeActivated.Condition", "param:stat:CantBeCast.Condition"},
	"Grave Titan":                 {"param:trig:Attacks.Secondary"},
	"Gravecrawler":                {"param:stat:Continuous.IsPresent"},
	"Grim Tutor":                  {"param:api:ChangeZone.Mandatory"},
	"Hallowed Fountain":           {"param:api:Tap.UnlessCost", "param:api:Tap.UnlessPayer"},
	"Hangarback Walker":           {"param:api:PutCounter.ETB"},
	"Hearthhull, the Worldseed":   {"param:stat:Continuous.AddAbility", "param:stat:Continuous.AddTrigger"},
	"Icetill Explorer":            {"param:stat:Continuous.AdjustLandPlays"},
	"Imperial Seal":               {"param:api:ChangeZone.Mandatory"},
	"Impulse":                     {"param:api:Dig.NoReveal"},
	"Incinerate":                  {"param:api:Cleanup.ClearRemembered", "param:api:Effect.ForgetOnMoved"},
	"Infernal Tutor":              {"param:api:ChangeZone.ConditionCheckSVar", "param:api:ChangeZone.ConditionSVarCompare", "param:api:ChangeZone.Mandatory", "param:api:Cleanup.ClearRemembered", "param:api:Reveal.ConditionCheckSVar"},
	"Into the Roil":               {"param:api:Draw.Condition"},
	"Jace, the Mind Sculptor":     {"param:api:ChangeZone.Mandatory", "param:api:ChangeZoneAll.Shuffle", "param:api:ChangeZoneAll.Ultimate", "param:api:Dig.LibraryPosition2"},
	"Jeska's Will":                {"param:api:Cleanup.ClearRemembered", "param:api:Dig.RememberChanged", "param:api:Effect.ForgetOnMoved"},
	"Journey to Nowhere":          {"param:api:ChangeZone.ForgetOtherTargets", "param:api:ChangeZone.RememberTargets"},
	"Karn Liberated":              {"param:api:ChangeZone.Hidden", "param:api:ChangeZone.Mandatory", "param:api:ChangeZoneAll.GainControl", "param:api:RestartGame.RestrictFromValid", "param:api:RestartGame.RestrictFromZone", "param:api:RestartGame.Ultimate"},
	"Karn, the Great Creator":     {"param:api:Animate.Duration", "param:api:ChangeZone.Hidden", "param:api:ChangeZone.Reveal", "param:stat:CantBeActivated.AffectedZone"},
	"Knight of the White Orchid":  {"param:api:ChangeZone.ShuffleNonMandatory", "param:trig:ChangesZone.CheckSVar", "param:trig:ChangesZone.SVarCompare"},
	"Kodama's Reach":              {"param:api:ChangeZone.Mandatory", "param:api:ChangeZone.NoLooking", "param:api:ChangeZone.Reveal", "param:api:Cleanup.ClearRemembered"},
	"Kor Skyfisher":               {"param:api:ChangeZone.Hidden", "param:api:ChangeZone.Mandatory"},
	"Land Tax":                    {"param:api:ChangeZone.ShuffleNonMandatory", "param:trig:Phase.CheckSVar", "param:trig:Phase.SVarCompare"},
	"Leonin Relic-Warder":         {"param:api:ChangeZone.ForgetOtherTargets", "param:api:ChangeZone.RememberTargets"},
	"Linvala, Keeper of Silence":  {"param:stat:CantBeActivated.AffectedZone"},
	"Lion's Eye Diamond":          {"param:api:Mana.InstantSpeed"},
	"Lord Windgrace":              {"param:api:Cleanup.ClearRemembered", "param:api:Destroy.Ultimate", "param:api:Discard.RememberDiscarded"},
	"Master of Etherium":          {"param:stat:Continuous.CharacteristicDefining"},
	"Matter Reshaper":             {"param:api:Dig.DestinationZone2", "param:api:Dig.Reveal"},
	"Meathook Massacre II":        {"param:api:ChangeZone.GainControl", "param:api:ChangeZone.UnlessCost", "param:api:ChangeZone.UnlessPayer", "param:api:Sacrifice.Amount"},
	"Mishra's Factory":            {"param:api:Animate.RemoveCreatureTypes"},
	"Mistveil Plains":             {"param:api:ChangeZone.IsPresent", "param:api:ChangeZone.PresentCompare"},
	"Mogis, God of Slaughter":     {"param:api:DealDamage.UnlessCost", "param:api:DealDamage.UnlessPayer", "param:stat:Continuous.CheckSVar", "param:stat:Continuous.RemoveType", "param:stat:Continuous.SVarCompare"},
	"Myriad Landscape":            {"param:api:ChangeZone.ShareLandType"},
	"Necrodominance":              {"cost:PayLife", "param:api:ChangeZone.Hidden", "param:stat:Continuous.SetMaxHandSize"},
	"Necropotence":                {"param:api:ChangeZone.ExileFaceDown", "param:api:Cleanup.ClearRemembered", "param:api:DelayedTrigger.RememberObjects", "param:api:DelayedTrigger.ValidPlayer"},
	"Nissa's Pilgrimage":          {"param:api:ChangeZone.ForgetChanged", "param:api:ChangeZone.Mandatory", "param:api:ChangeZone.NoLooking", "param:api:Cleanup.ClearRemembered"},
	"Ob Nixilis, Captive Kingpin": {"param:api:Cleanup.ClearRemembered", "param:api:Dig.RememberChanged", "param:api:Effect.ForgetOnMoved"},
	"Ojer Axonil, Deepest Might":  {"param:api:ChangeZone.Transformed", "param:api:SetState.CheckSVar", "param:api:SetState.SVarCompare"},
	"Oracle of Mul Daya":          {"param:stat:Continuous.AdjustLandPlays", "param:stat:Continuous.MayLookAt"},
	"Overseer of the Damned":      {"param:api:Token.TokenTapped"},
	"Palace Jailer":               {"param:api:Effect.EffectOwner", "param:api:Effect.ForgetOnMoved"},
	"Path to Exile":               {"param:api:ChangeZone.ShuffleNonMandatory"},
	"Phyrexian Obliterator":       {"param:api:Sacrifice.Amount"},
	"Planar Engineering":          {"param:api:Sacrifice.Amount"},
	"Planetary Annihilation":      {"param:api:ChooseCard.Reveal"},
	"Ponder":                      {"param:api:RearrangeTopOfLibrary.MayShuffle"},
	"Price of Progress":           {"param:api:RepeatEach.DamageMap"},
	"Profane Tutor":               {"param:api:ChangeZone.Mandatory"},
	"Purphoros, God of the Forge": {"param:stat:Continuous.CheckSVar", "param:stat:Continuous.RemoveType", "param:stat:Continuous.SVarCompare"},
	"Ragavan, Nimble Pilferer":    {"param:api:Cleanup.ClearRemembered", "param:api:Dig.RememberChanged", "param:api:Effect.ForgetOnMoved"},
	"Rakdos, Lord of Riots":       {"param:stat:CantBeCast.CheckSVar", "param:stat:CantBeCast.SVarCompare"},
	"Razorkin Needlehead":         {"param:stat:Continuous.Condition"},
	"Reality Smasher":             {"param:trig:BecomesTarget.ValidSource"},
	"Realms Uncharted":            {"param:api:ChangeZone.DifferentNames", "param:api:ChangeZone.Mandatory", "param:api:ChangeZone.NoLooking", "param:api:ChangeZone.Reveal", "param:api:Cleanup.ClearRemembered"},
	"Relic of Progenitus":         {"cost:Exile", "param:api:ChangeZone.Hidden", "param:api:ChangeZone.Mandatory"},
	"Remand":                      {"param:api:Counter.Destination"},
	"Resplendent Angel":           {"param:trig:Phase.CheckSVar", "param:trig:Phase.SVarCompare"},
	"Restless Cottage":            {"param:api:Animate.Colors", "param:api:Animate.OverwriteColors"},
	"Riddlesmith":                 {"cost:Draw"},
	"Righteous Valkyrie":          {"param:stat:Continuous.CheckSVar", "param:stat:Continuous.SVarCompare"},
	"Roiling Vortex":              {"param:trig:SpellCast.ValidSA"},
	"Scapeshift":                  {"param:api:Cleanup.ClearRemembered", "param:api:Sacrifice.Amount", "param:api:Sacrifice.Optional"},
	"Screaming Nemesis":           {"param:api:Cleanup.ClearRemembered"},
	"Sea Gate Wreckage":           {"param:api:Draw.Activation"},
	"Serra Avenger":               {"param:stat:CantBeCast.CheckSVar", "param:stat:CantBeCast.SVarCompare"},
	"Silkwrap":                    {"param:api:ChangeZone.Duration"},
	"Skyclave Apparition":         {"param:api:Cleanup.ClearRemembered", "param:api:Token.TokenPower", "param:api:Token.TokenToughness"},
	"Snapcaster Mage":             {"param:api:Pump.PumpZone"},
	"Solemn Simulacrum":           {"param:api:ChangeZone.ShuffleNonMandatory"},
	"Sower of Discord":            {"param:api:Cleanup.ClearRemembered", "param:trig:DamageDoneOnce.ActiveZones", "param:trig:DamageDoneOnce.Secondary"},
	"Splendid Reclamation":        {"param:api:ChangeZoneAll.Tapped"},
	"Springbloom Druid":           {"param:api:ChangeZone.ShuffleNonMandatory"},
	"Squadron Hawk":               {"param:api:ChangeZone.ShuffleNonMandatory"},
	"Stasis Snare":                {"param:api:ChangeZone.Duration"},
	"Static Orb":                  {"param:stat:Continuous.IsPresent"},
	"Steel Leaf Champion":         {"param:stat:CantBlockBy.ValidAttacker"},
	"Stoneforge Mystic":           {"param:api:ChangeZone.ShuffleNonMandatory"},
	"Sun Titan":                   {"param:trig:Attacks.Secondary"},
	"Sword of Fire and Ice":       {"param:stat:Continuous.AddSVar"},
	"Tainted Peak":                {"param:api:Mana.IsPresent"},
	"Temple of the False God":     {"param:api:Mana.IsPresent", "param:api:Mana.PresentCompare"},
	"Temur Sabertooth":            {"param:api:ChangeZone.Hidden", "param:api:Cleanup.ClearRemembered"},
	"Terminus":                    {"param:api:ChangeZoneAll.LibraryPosition"},
	"The Lord of Pain":            {"param:trig:SpellCast.ActivatorThisTurnCast"},
	"Thirst for Knowledge":        {"param:api:Discard.UnlessType"},
	"Thornspire Verge":            {"param:api:Mana.IsPresent"},
	"Through the Forest Gate":     {"param:api:Dig.SkipReorder", "param:api:Dig.Tapped"},
	"Thunderbreak Regent":         {"param:trig:BecomesTarget.ValidSource"},
	"Tome of Legends":             {"param:api:PutCounter.ETB", "param:trig:Attacks.Secondary"},
	"Torment of Hailfire":         {"param:api:LoseLife.UnlessCost", "param:api:LoseLife.UnlessPayer"},
	"Toxic Deluge":                {"cost:PayLife"},
	"Trinket Mage":                {"param:api:ChangeZone.ShuffleNonMandatory"},
	"Troop of Ponies":             {"param:api:ChangeZone.ForgetChanged", "param:api:ChangeZone.Mandatory", "param:api:ChangeZone.NoLooking", "param:api:ChangeZone.Reveal", "param:api:Cleanup.ClearRemembered"},
	"Valakut Exploration":         {"param:api:Cleanup.ClearRemembered", "param:api:Dig.RememberChanged", "param:api:Effect.ForgetOnMoved", "param:trig:Phase.CheckSVar", "param:trig:Phase.SVarCompare"},
	"Valkyrie Harbinger":          {"param:trig:Phase.CheckSVar", "param:trig:Phase.SVarCompare"},
	"Vampire Lacerator":           {"param:api:LoseLife.ConditionCheckSVar", "param:api:LoseLife.ConditionSVarCompare"},
	"Vampiric Tutor":              {"param:api:ChangeZone.Mandatory"},
	"Vastwood Hydra":              {"param:api:PutCounter.ChoiceAmount", "param:api:PutCounter.DividedAsYouChoose", "param:api:PutCounter.ETB", "param:api:PutCounter.MinChoiceAmount"},
	"Vexing Devil":                {"cost:DamageYou", "param:api:Sacrifice.UnlessCost", "param:api:Sacrifice.UnlessPayer", "param:api:Sacrifice.UnlessSwitched"},
	"Vial Smasher the Fierce":     {"param:api:Cleanup.ClearChosenPlayer", "param:trig:SpellCast.ActivatorThisTurnCast"},
	"Victimize":                   {"param:api:ChangeZone.ConditionCheckSVar", "param:api:ChangeZone.ConditionSVarCompare", "param:api:Cleanup.ClearRemembered"},
	"Vines of Vastwood":           {"param:api:Effect.ExileOnMoved"},
	"Virtue of Persistence":       {"param:api:ChangeZone.GainControl", "param:api:ChangeZone.Mandatory"},
	"Voracious Hydra":             {"param:api:PutCounter.ETB"},
	"Walk-In Closet":              {"param:api:ChangeZone.Hidden", "param:api:Effect.ReplacementEffects"},
	"Walking Ballista":            {"param:api:PutCounter.ETB"},
	"Wastewood Verge":             {"param:api:Mana.IsPresent"},
	"Whirler Rogue":               {"param:api:Effect.ExileOnMoved"},
	"Whisperer of the Wilds":      {"param:api:Mana.IsPresent"},
	"Windgrace's Judgment":        {"param:api:Destroy.TargetsForEachPlayer"},
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
	s.failGuard(t)
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
// brief was written from: Daze's Return<1/Island> alternative cost (ParseCost
// silently substituting generic mana), Chandra's SubCounter<X/LOYALTY>, plus
// Relic of Progenitus's bare Exile<1/Card.YouOwn> and Vexing Devil's
// DamageYou<1> -- still unmodelled heads. Force of Will's
// ExileFromHand<1/Card.Blue+Other> and Whirler Rogue's tapXType<2/Artifact>
// were in the original pin but the alternative-cost work on main
// (ExileFromHand/ExileFromGrave/Reveal/Behold/tapXType/Blight/Forage heads in
// ParseCost) now models them, so the pin also asserts they are GONE --
// cost: labels only ever shrink when a real ParseCost model lands.
// (The brief's other example, Reanimate's GainControl$ on api:ChangeZone, is
// not in any repo deck; the same label appears in the baseline on Meathook
// Massacre II and retires the moment the read is implemented.)
func TestParamCensusPinsTheImportReviewExamples(t *testing.T) {
	res, _ := measureParamCensus(t, nil)
	want := map[string]string{
		"Relic of Progenitus":       "cost:Exile",
		"Daze":                      "cost:Return",
		"Chandra, Awakened Inferno": "cost:SubCounter",
		"Vexing Devil":              "cost:DamageYou",
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
	// The two original cost examples retired with main's alternative-cost
	// ParseCost heads; assert the shrink so a regression that reintroduces
	// the silent substitution fails here.
	for card, label := range map[string]string{
		"Force of Will": "cost:ExileFromHand",
		"Whirler Rogue": "cost:tapXType",
	} {
		for _, l := range res.labels[card] {
			if l == label {
				t.Errorf("%s: %s is back in the census -- ParseCost stopped modelling the token", card, label)
			}
		}
	}
}

// TestParseCostReportsUnmodelledCostTokens pins the reporting half: tokens
// ParseCost does not model land in Cost.Unknown; modelled ones never do.
// TestParamCensusCatchesAliasedParamsReads pins the alias finding: a
// consumer that reads the Params map through a local alias (`p := sa.Params;
// p["SacValid"]`) must be DERIVED by the scan -- the old selector-shape-only
// scan silently skipped it, and an unreachable aliased consumer passed
// TestParamCensusScanIsComplete. Both halves are pinned: the read is
// attributed, and the unreachable consumer fails the rot guard by name.
func TestParamCensusCatchesAliasedParamsReads(t *testing.T) {
	s := newScan()
	s.scanSource(t, "probe_alias.go", "effects", `package effects

import "github.com/adams-shaun/gorge/cards"

func censusProbeAlias(sa *cards.SA) string {
	params := sa.Params
	chained := params
	return chained["SacValid"]
}
`)
	if fi := s.fns["effects:censusProbeAlias"]; fi == nil || !fi.reads[bSA]["SacValid"] {
		t.Fatalf("aliased (and chained-alias) Params read not derived -- the scan is still shape-matched: %+v", fi)
	}
	s.rotGuard(t)
	flagged := false
	for _, err := range s.guardErrs {
		if strings.Contains(err, "censusProbeAlias") && strings.Contains(err, "unreachable") {
			flagged = true
		}
	}
	if !flagged {
		t.Fatalf("rot guard did not flag the unreachable aliased consumer; findings: %v", s.guardErrs)
	}
}

// TestParamCensusCatchesHelperPassedMaps pins the helper-passed-map finding:
// a function whose parameter is a map[string]string gets its index reads
// attributed through the call sites' arguments (so the CALLER carries the
// read with the argument's bucket), and a call passing a non-Params map --
// or no call at all -- fails the rot guard.
func TestParamCensusCatchesHelperPassedMaps(t *testing.T) {
	s := newScan()
	s.scanSource(t, "probe_helper.go", "effects", `package effects

import "github.com/adams-shaun/gorge/cards"

func censusProbeHelper(m map[string]string) string {
	return m["SacValid"]
}

func censusProbeCaller(sa *cards.SA) string {
	return censusProbeHelper(sa.Params)
}
`)
	s.propagateKeyReads()
	if fi := s.fns["effects:censusProbeCaller"]; fi == nil || !fi.reads[bSA]["SacValid"] {
		t.Fatalf("helper-passed Params read not attributed to the caller: %+v", fi)
	}
	s.rotGuard(t)
	// (The caller's own unreachability finding is inherent to synthetic code;
	// what must hold is the helper attribution: no non-Params / unattributed
	// finding for the helper or its call site.)
	for _, err := range s.guardErrs {
		if strings.Contains(err, "not a Params map") || strings.Contains(err, "no call site passes") {
			t.Fatalf("helper attribution failed: %s", err)
		}
	}

	// The same helper fed a non-Params map[string]string (here an alias of
	// the face's SVars table) must fail the guard instead of passing.
	s2 := newScan()
	s2.scanSource(t, "probe_helper2.go", "effects", `package effects

import "github.com/adams-shaun/gorge/cards"

func censusProbeHelper2(m map[string]string) string {
	return m["SacValid"]
}

func censusProbeCaller2(sa *cards.SA) string {
	svars := sa.SVars
	return censusProbeHelper2(svars)
}
`)
	s2.propagateKeyReads()
	s2.rotGuard(t)
	flagged := false
	for _, err := range s2.guardErrs {
		if strings.Contains(err, "censusProbeCaller2") && strings.Contains(err, "not a Params map") {
			flagged = true
		}
	}
	if !flagged {
		t.Fatalf("rot guard did not flag the non-Params map argument; findings: %v", s2.guardErrs)
	}
}

// TestParamCensusAttributesSpecialisedRulesPaths pins the api-specific
// attribution: the mana path's Amount$/Produced$ reads belong to api:Mana
// alone, the unless-pay resume's UnlessCost$ to Counter/CopySpellAbility, the
// Charm mode paths' Choices$/CharmNum$ to api:Charm -- so the genuinely
// unread parameters on other APIs surface in the census (Sacrifice.Amount on
// the five repo-deck carriers; Vexing Devil's Sacrifice.UnlessCost).
func TestParamCensusAttributesSpecialisedRulesPaths(t *testing.T) {
	res, d := measureParamCensus(t, nil)
	want := map[string]map[string]bool{
		"Mana":             {"Amount": true, "Produced": true},
		"Counter":          {"UnlessCost": true},
		"CopySpellAbility": {"UnlessCost": true},
		"Charm":            {"CharmNum": true, "Choices": true},
	}
	for api, keys := range want {
		for key := range keys {
			if !d.api[api][key] {
				t.Errorf("d.api[%q][%q] = false -- the specialised attribution lost a real read", api, key)
			}
		}
	}
	for _, wrong := range []struct{ api, key string }{
		{"Sacrifice", "Amount"}, {"Sacrifice", "UnlessCost"}, {"Sacrifice", "Produced"},
		{"DealDamage", "CharmNum"}, {"DealDamage", "Produced"}, {"ChangeZone", "Amount"},
	} {
		if d.api[wrong.api][wrong.key] {
			t.Errorf("d.api[%q][%q] = true -- a specialised rules path still over-suppresses this API's gap", wrong.api, wrong.key)
		}
	}
	// The census-level effect on the real repo decks: every Sacrifice ability
	// carrying an Amount$ or UnlessCost$ its implementation never reads is now
	// labelled (previously masked by the Mana/Counter reads).
	for card, label := range map[string]string{
		"Braids, Arisen Nightmare": "param:api:Sacrifice.Amount",
		"Phyrexian Obliterator":    "param:api:Sacrifice.Amount",
		"Planar Engineering":       "param:api:Sacrifice.Amount",
		"Scapeshift":               "param:api:Sacrifice.Amount",
		"Meathook Massacre II":     "param:api:Sacrifice.Amount",
		"Vexing Devil":             "param:api:Sacrifice.UnlessCost",
	} {
		found := false
		for _, l := range res.labels[card] {
			if l == label {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s: expected %s in the census (labels %v)", card, label, res.labels[card])
		}
	}
}

// TestParamCensusCatchesFaceOwnedCosts pins the cost sources that are not
// Params maps: the normal printed cost plus Kicker, Surge, Flashback and
// Miracle. It drives a two-face card through cardCensusLabels, the same
// card-side walk walkRepoDeckCensus uses, so removing faceUnknownCostLabels'
// production call makes this test fail even though no current repo-deck card
// carries one of these unknown face-owned cost tokens.
func TestParamCensusCatchesFaceOwnedCosts(t *testing.T) {
	c := &cards.Card{Faces: []*cards.Face{
		{
			ManaCost: "PayEnergy<X>",
			Keywords: []string{
				"Kicker:Return<1/CARDNAME>",
				"Surge:PaySurge<1>",
			},
		},
		{Keywords: []string{
			"Flashback:ExileFromGrave<1/Card>",
			"Miracle:PayMiracle<1>",
		}},
	}}
	want := []string{
		"cost:PayEnergy", "cost:Return", "cost:PaySurge",
		"cost:PayMiracle",
	}
	d := &derivedReads{api: map[string]map[string]bool{}, trig: map[string]map[string]bool{}, stat: map[string]map[string]bool{}, repl: map[string]map[string]bool{}}
	if got := cardCensusLabels(c, d, nil); !sameSet(got, want) {
		t.Errorf("card-side face-owned costs = %v, want %v", got, want)
	}
}

// TestParamCensusCatchesSVarBodyGaps pins the SVar-body walk: a body named
// by an execution-bearing SVar-reference parameter -- Charm's `Choices$`
// entries (effCharm resolves and runs each) or Repeat's `RepeatSubAbility$`
// (effRepeat resolves and runs it) -- must be censused like a linked
// sub-ability. Before that walk existed the outer primitives' own params
// (Choices$, RepeatSubAbility$) were all read, the bodies were never
// attached by Link, and a repo-deck card could carry an unread parameter or
// an unmodelled cost token inside a selected/repeated body that no ratchet
// saw. The fixture runs the real derived read sets (measureParamCensus),
// and each expected label is reachable ONLY through the SVar body: remove
// the face-SVar walk from cardCensusLabels and both labels disappear.
func TestParamCensusCatchesSVarBodyGaps(t *testing.T) {
	_, d := measureParamCensus(t, nil)
	// Preconditions: the outer primitives' parameters the fixture carries are
	// genuinely read (so the labels can only come from the bodies), and the
	// body keys are genuinely unread (so the fixture measures a real gap).
	if !d.api["Charm"]["Choices"] || !d.api["Repeat"]["RepeatSubAbility"] {
		t.Fatalf("outer Choices$/RepeatSubAbility$ reads lost -- fixture premise broken")
	}
	if d.api["LoseLife"]["UnlessCost"] {
		t.Fatalf("api:LoseLife now reads UnlessCost$ -- re-point the fixture at a genuinely unread key")
	}
	c := &cards.Card{Faces: []*cards.Face{{
		// A modal spell whose one mode loses life unless a cost is paid
		// (Torment of Hailfire's shape), and a repeat whose body carries an
		// energy cost ParseCost does not model (the Chthonian Nightmare
		// shape, reached through RepeatSubAbility$).
		Abilities: []*cards.SA{
			{Kind: "SP", API: "Charm", Params: map[string]string{"Choices": "DBMode,DBMoney", "CharmNum": "1"}},
			{Kind: "SP", API: "Repeat", Params: map[string]string{"RepeatNum": "2", "RepeatSubAbility": "DBMoney"}},
		},
		SVars: map[string]string{
			"DBMode":  "DB$ LoseLife | UnlessCost$ 3 | Defined$ Remembered",
			"DBMoney": "DB$ LoseLife | Cost$ PayEnergy<X>",
		},
	}}}
	want := []string{"param:api:LoseLife.UnlessCost", "cost:PayEnergy"}
	if got := cardCensusLabels(c, d, nil); !sameSet(got, want) {
		t.Errorf("SVar-body census = %v, want %v -- an unread key or unmodelled token inside a Choices$/RepeatSubAbility$ body is not being reported", got, want)
	}
	// The drop plumbing reaches the bodies too: pretending the LoseLife
	// UnlessCost$ read existed (it does not) must not un-report the body's
	// gap through some other path.
	if got := cardCensusLabels(c, d, map[string]map[string]bool{"api:LoseLife": {"UnlessCost": true}}); !sameSet(got, want) {
		t.Errorf("drop-simulated census = %v, want %v", got, want)
	}
}

func TestParseCostReportsUnmodelledCostTokens(t *testing.T) {
	cases := []struct {
		cost string
		want []string
	}{
		{"PayEnergy<X> Sac<1/Creature> Return<1/CARDNAME>", []string{"PayEnergy", "Return"}},
		{"PayLife<5>", nil},
		{"PayLife<X>", []string{"PayLife"}},
		// Recognised heads whose INSTANCE is malformed or out of range: the
		// head is known, the instance is not modelled -- reported too.
		{"PayLife<99999999999999999999>", []string{"PayLife"}},
		{"Sac<99999999999999999999/Creature>", []string{"Sac"}},
		{"AddCounter<99999999999999999999/LOYALTY>", []string{"AddCounter"}},
		{"PayLife<abc>", []string{"PayLife"}},
		{"Sac</Creature>", []string{"Sac"}},
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
