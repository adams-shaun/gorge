package kgen

import (
	"bytes"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cardc/forge"
	"github.com/adams-shaun/gorge/engine/cardc/ir"
	"github.com/adams-shaun/gorge/engine/cards"
)

// typedPoolTable loads pool name, sets reader r, and returns its encoded
// table, its supported card names and the pool's filter stats.
func typedPoolTable(t *testing.T, name, cardsDir string, r filterReader) ([]byte, []string, filterStats) {
	t.Helper()
	p, _, prev, err := readPool(gorge.TableSources(), name, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	p.filterReader = r
	d, cv, _, err := tableOf(p, name, prev)
	if err != nil {
		t.Fatal(err)
	}
	refused := map[string]bool{}
	for _, rf := range cv.refused {
		refused[rf.Card] = true
	}
	var sup []string
	for _, n := range p.names {
		if !refused[n] {
			sup = append(sup, n)
		}
	}
	return cardset.EncodeTable(d, "cmp"), sup, p.filterStats
}

// TestTypedFilterTablesByteEqual builds fdn and blb with the string reader
// and with the typed reader: the tables and the supported lists must be the
// same, and at least 85% of the typed run's filter reads take the typed path.
func TestTypedFilterTablesByteEqual(t *testing.T) {
	cardsDir := tableCorpus(t)
	for _, d := range typedSpellingDistinctions {
		t.Logf("spelling distinction (read by the string reader): %s", d)
	}
	for _, d := range typedCollapsedSpellings {
		t.Logf("collapsed spelling (typed, most common form): %s", d)
	}
	for _, name := range []string{"fdn", "blb"} {
		t.Run(name, func(t *testing.T) {
			strTab, strSup, strSt := typedPoolTable(t, name, cardsDir, filterReadString)
			typTab, typSup, st := typedPoolTable(t, name, cardsDir, filterReadTyped)
			if strSt != (filterStats{}) {
				t.Fatalf("the string reader counted typed reads: %+v", strSt)
			}
			if !bytes.Equal(strTab, typTab) {
				t.Errorf("table differs: %d bytes string, %d typed, first diff at byte %d", len(strTab), len(typTab), firstDiff(strTab, typTab))
			}
			if !slices.Equal(strSup, typSup) {
				t.Errorf("supported lists differ: %d string, %d typed", len(strSup), len(typSup))
			}
			total := st.typed + st.fallbackFrontEnd + st.fallbackReader
			if total == 0 {
				t.Fatal("no filter read reached parseFilterText")
			}
			share := float64(st.typed) / float64(total)
			t.Logf("%s: typed %d, fallbackFrontEnd %d, fallbackReader %d, typed share %.3f", name, st.typed, st.fallbackFrontEnd, st.fallbackReader, share)
			for k, n := range st.refusedBy {
				if n != 0 {
					t.Logf("%s: front end refused %d as %s", name, n, forge.RefusalKind(k))
				}
			}
			if share < 0.85 {
				t.Errorf("typed share %.3f < 0.85", share)
			}
		})
	}
}

// readingRefusalMarks are the string reader's refusal texts that name a word
// it cannot read (the others are kernel-capacity refusals).
var readingRefusalMarks = []string{"unknown filter head", "unknown property", "unknown negation", `compare "`, "counter predicate", "totalPT compare"}

// TestTypedFilterRefusalSplit splits edh's filter refusals (string reader)
// into reading and capacity refusals and logs the reading share.
func TestTypedFilterRefusalSplit(t *testing.T) {
	cardsDir := tableCorpus(t)
	p, _, prev, err := readPool(gorge.TableSources(), "edh", cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	_, cv, _, err := tableOf(p, "edh", prev)
	if err != nil {
		t.Fatal(err)
	}
	total, reading, capacity := 0, 0, 0
	for _, rf := range cv.refused {
		if !strings.HasPrefix(rf.Reason, "filter ") {
			continue
		}
		total++
		if slices.ContainsFunc(readingRefusalMarks, func(m string) bool { return strings.Contains(rf.Reason, m) }) {
			reading++
		} else {
			capacity++
		}
	}
	share := 0.0
	if total > 0 {
		share = float64(reading) / float64(total)
	}
	t.Logf("edh filter refusals: %d reading, %d capacity, of %d (reading share %.3f)", reading, capacity, total, share)
	if reading+capacity != total {
		t.Fatalf("%d reading + %d capacity != %d", reading, capacity, total)
	}
}

// TestTypedFilterReaderReadsNoText holds the typed reader to the interface:
// it never reads a node's Src and calls strings only on its type, keyword,
// counter and name words (at most 6 calls).
func TestTypedFilterReaderReadsNoText(t *testing.T) {
	files, err := filepath.Glob("filter_ir*.go")
	if err != nil {
		t.Fatal(err)
	}
	files = slices.DeleteFunc(files, func(f string) bool { return strings.HasSuffix(f, "_test.go") })
	if !slices.Contains(files, "filter_ir.go") || len(files) < 2 {
		t.Fatalf("typed reader files %q: want filter_ir.go and its filter_ir_*.go", files)
	}
	calls := 0
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(src, []byte(".Src")) {
			t.Errorf("%s reads .Src", f)
		}
		af, err := parser.ParseFile(token.NewFileSet(), f, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(af, func(n ast.Node) bool {
			if ce, ok := n.(*ast.CallExpr); ok {
				if se, ok := ce.Fun.(*ast.SelectorExpr); ok {
					if id, ok := se.X.(*ast.Ident); ok && id.Name == "strings" {
						calls++
					}
				}
			}
			return true
		})
	}
	t.Logf("%d strings calls in %q", calls, files)
	if calls > 6 {
		t.Errorf("%d strings calls in the typed reader, want at most 6", calls)
	}
}

// typedFilterKeys are the 13 object-filter keys FilterOf answers for.
var typedFilterKeys = []cards.ParamKey{
	cards.PKAffected, cards.PKChangeType, cards.PKConditionPresent, cards.PKConditionPresent2, cards.PKIsPresent,
	cards.PKIsPresent2, cards.PKValid, cards.PKValidCard, cards.PKValidCards, cards.PKValidCause,
	cards.PKValidSource, cards.PKValidTarget, cards.PKValidTgts,
}

// TestTypedFilterSlotsAgree: kgen's registered binder answers FilterOf for
// every filter key of fdn's and blb's cards exactly as forge.LowerFilter
// reads the same text.
func TestTypedFilterSlotsAgree(t *testing.T) {
	cardsDir := tableCorpus(t)
	if len(typedFilterKeys) != 13 || len(forge.FilterKeys) != 13 {
		t.Fatalf("%d keys here, %d in forge.FilterKeys, want 13", len(typedFilterKeys), len(forge.FilterKeys))
	}
	for _, k := range typedFilterKeys {
		if !cards.IsFilterKey(k) || !slices.Contains(forge.FilterKeys, k.String()) {
			t.Fatalf("%s is not a filter key on both sides", k)
		}
	}
	checked, bound := 0, 0
	check := func(card, what string, params map[string]string, of func(cards.ParamKey) (*ir.Filter, bool)) {
		for _, k := range typedFilterKeys {
			text, ok := params[k.String()]
			if !ok {
				continue
			}
			checked++
			got, gok := of(k)
			want, ref := forge.LowerFilter(text)
			switch {
			case gok != (ref == nil):
				t.Errorf("%s %s %s %q: FilterOf answered %v, LowerFilter refusal %v", card, what, k, text, gok, ref)
			case gok && !ir.Equal(got, want):
				t.Errorf("%s %s %s %q: FilterOf and LowerFilter differ", card, what, k, text)
			case gok:
				bound++
			}
		}
	}
	var walkSA func(card, what string, sa *cards.SA)
	walkSA = func(card, what string, sa *cards.SA) {
		for ; sa != nil; sa = sa.Sub {
			check(card, what, sa.Params, sa.FilterOf)
		}
	}
	for _, name := range []string{"fdn", "blb"} {
		p, _, _, err := readPool(gorge.TableSources(), name, cardsDir)
		if err != nil {
			t.Fatal(err)
		}
		for _, n := range p.names {
			for _, f := range p.byName[n].Faces {
				if f == nil {
					continue
				}
				for _, sa := range f.Abilities {
					walkSA(n, "ability", sa)
				}
				for _, tr := range f.Triggers {
					check(n, "trigger", tr.Params, tr.FilterOf)
					walkSA(n, "trigger effect", tr.Effect)
				}
				for _, s := range f.Statics {
					check(n, "static", s.Params, s.FilterOf)
				}
				for _, r := range f.Repls {
					check(n, "replacement", r.Params, r.FilterOf)
					walkSA(n, "replacement effect", r.With)
				}
			}
		}
	}
	t.Logf("%d filter slots checked, %d bound", checked, bound)
	if checked == 0 || bound == 0 {
		t.Fatalf("%d slots checked, %d bound: the check ran on nothing", checked, bound)
	}
}

// irNode builds an FPred node for the clause cases.
func irNode(subj ir.Subject, p ir.Pred, word string) *ir.Filter {
	return &ir.Filter{Subject: subj, Op: ir.FPred, Pred: p, Word: word}
}

func irNot(n *ir.Filter) *ir.Filter {
	return &ir.Filter{Subject: n.Subject, Op: ir.FNot, Args: []*ir.Filter{n}}
}

func irAll(base *ir.Filter, conj ...*ir.Filter) *ir.Filter {
	return &ir.Filter{Subject: base.Subject, Zones: base.Zones, Op: ir.FAll, Args: append([]*ir.Filter{base}, conj...)}
}

func irRef(n *ir.Filter, r *ir.Ref) *ir.Filter { n.Ref = r; return n }

func irCmp(n *ir.Filter, op ir.CmpOp, v int32) *ir.Filter {
	n.Cmp, n.Value = op, &ir.Value{Kind: ir.VConst, N: v}
	return n
}

// TestFilterIRClauses maps hand-built ir.Filters and compares each with the
// string reader's lFilter for an equivalent text; each built filter is also
// the front end's reading of that text.
func TestFilterIRClauses(t *testing.T) {
	known := func(w string) bool { return w == "Elf" || w == "Goblin" || w == "Wizard" || w == "Equipment" }
	isType := func(w string) bool { _, ok := typeBits[w]; return ok || known(w) }
	obj := ir.SubjObject
	you := func() *ir.Ref { return &ir.Ref{Kind: ir.RefYou} }
	self := func() *ir.Ref { return &ir.Ref{Kind: ir.RefSelf} }
	opps := func() *ir.Ref {
		return &ir.Ref{Kind: ir.RefAll, Filter: irRef(irNode(ir.SubjPlayer, ir.POpponentOf, ""), you())}
	}
	cases := []struct {
		text string
		f    *ir.Filter
	}{
		{"Creature.YouCtrl", irAll(irNode(obj, ir.PCardType, "Creature"), irRef(irNode(obj, ir.PControlledBy, ""), you()))},
		{"Creature.OppCtrl+tapped", irAll(irNode(obj, ir.PCardType, "Creature"), irRef(irNode(obj, ir.PControlledBy, ""), opps()), irNode(obj, ir.PTapped, ""))},
		{"Card.nonLand+cmcLE3", irAll(irNode(obj, ir.PAny, ""), irNot(irNode(obj, ir.PCardType, "Land")), irCmp(irNode(obj, ir.PManaValue, ""), ir.CmpLE, 3))},
		{"Permanent.Legendary", irAll(irNode(obj, ir.PPermanentCard, ""), irNode(obj, ir.PSupertype, "Legendary"))},
		{"Land.Artifact", irAll(irNode(obj, ir.PCardType, "Land"), irNode(obj, ir.PCardType, "Artifact"))},
		{"Creature.Other+attacking", irAll(irNode(obj, ir.PCardType, "Creature"), irNot(irRef(irNode(obj, ir.PIs, ""), self())), irNode(obj, ir.PAttacking, ""))},
		{"Creature.withFlying+White", irAll(irNode(obj, ir.PCardType, "Creature"), irNode(obj, ir.PHasKeyword, "Flying"), &ir.Filter{Subject: obj, Op: ir.FPred, Pred: ir.PColor, Colors: ir.ColorW})},
		{"Creature.counters_GE1_P1P1", irAll(irNode(obj, ir.PCardType, "Creature"), irCmp(irNode(obj, ir.PCounters, "+1/+1"), ir.CmpGE, 1))},
		{"Elf.nonGoblin+YouOwn", irAll(irNode(obj, ir.PSubtype, "Elf"), irNot(irNode(obj, ir.PSubtype, "Goblin")), irRef(irNode(obj, ir.POwnedBy, ""), you()))},
		{"Creature,Player", &ir.Filter{Subject: obj, Op: ir.FAny, Args: []*ir.Filter{irNode(obj, ir.PCardType, "Creature"), irNode(ir.SubjPlayer, ir.PAny, "")}}},
		{"Creature.Elf+Goblin+Wizard", irAll(irNode(obj, ir.PCardType, "Creature"), irNode(obj, ir.PSubtype, "Elf"), irNode(obj, ir.PSubtype, "Goblin"), irNode(obj, ir.PSubtype, "Wizard"))},
		{"Creature.YouCtrl+OppCtrl", irAll(irNode(obj, ir.PCardType, "Creature"), irRef(irNode(obj, ir.PControlledBy, ""), you()), irRef(irNode(obj, ir.PControlledBy, ""), opps()))},
	}
	for _, tc := range cases {
		fe, ref := forge.LowerFilter(tc.text)
		if ref != nil || !ir.Equal(fe, tc.f) {
			t.Errorf("%q: the built filter is not the front end's reading (refusal %v)", tc.text, ref)
			continue
		}
		want, werr := parseFilter(tc.text, known, isType)
		got, gerr := lFilterFromIR(tc.f, known)
		switch {
		case errors.Is(gerr, errNoTypedReading):
			t.Errorf("%q: no typed reading", tc.text)
		case (gerr != nil) != (werr != nil):
			t.Errorf("%q: typed error %v, string error %v", tc.text, gerr, werr)
		case gerr != nil:
			var st filterStats
			if _, rerr := readFilterTyped(tc.text, known, isType, &st); rerr == nil || rerr.Error() != werr.Error() {
				t.Errorf("%q: typed refusal %v is not the string reader's %q", tc.text, rerr, werr)
			}
		case gerr == nil && !reflect.DeepEqual(got, want):
			t.Errorf("%q:\n typed  %v\n string %v", tc.text, got, want)
		}
	}
	// a node outside the table falls back
	if _, err := lFilterFromIR(irAll(irNode(obj, ir.PCardType, "Creature"), irNode(obj, ir.PSuspended, "")), known); !errors.Is(err, errNoTypedReading) {
		t.Errorf("suspended: want errNoTypedReading, got %v", err)
	}
}
