package cardset_test

import (
	"path/filepath"
	"sort"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// fdnData is the fdn table's data, for what the Set interface does not expose
// (Filters, Exprs, Conds, Overrides, Emblems). It skips with no corpus.
func fdnData(t *testing.T) *cardset.PoolData {
	t.Helper()
	cardsettest.Require(t, "fdn")
	dir, err := cardset.TablesDir()
	if err != nil {
		t.Fatal(err)
	}
	p, err := cardset.LoadFile(filepath.Join(dir, "fdn.ktab"))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// refusedCards is the coverage ratchet: exactly the pool cards dirac/kgen
// refuses (coverage.md says why). A card newly refused fails here; so does
// a listed card the generator now supports (delete it from the list).
var refusedCards = []string{}

func TestCoverageRatchet(t *testing.T) {
	p := fdnData(t)
	var got []string
	for i := 0; i < p.NumCards+p.NumTokens; i++ {
		if p.Status[i] == cardset.StatusNotLowered {
			got = append(got, p.Names[i])
		}
	}
	want := append([]string(nil), refusedCards...)
	sort.Strings(want)
	sort.Strings(got)
	gi, wi := 0, 0
	for gi < len(got) || wi < len(want) {
		switch {
		case wi == len(want) || (gi < len(got) && got[gi] < want[wi]):
			t.Errorf("newly refused: %s (coverage.md names the refusal)", got[gi])
			gi++
		case gi == len(got) || want[wi] < got[gi]:
			t.Errorf("now supported, drop it from refusedCards: %s", want[wi])
			wi++
		default:
			gi++
			wi++
		}
	}
	t.Logf("refused %d of %d pool cards", len(got), p.NumCards)
}

// TestProgramsWellFormed checks every supported card's abilities reference
// only ops below OpCount and in-range operands.
func TestProgramsWellFormed(t *testing.T) {
	p := fdnData(t)
	for i := 0; i < p.NumCards+p.NumTokens; i++ {
		if p.Status[i] == cardset.StatusNotLowered {
			continue
		}
		d := &p.Defs[i]
		for ai := range d.Abilities {
			checkAbility(t, p, p.Names[i], d, ai, &d.Abilities[ai])
		}
		checkTarget(t, p, p.Names[i], -1, d.Enchant)
	}
	for ei, e := range p.Emblems {
		for ai := range e.Abilities {
			for _, in := range e.Abilities[ai].Prog {
				if !in.Op.Known() {
					t.Errorf("emblem %d: op %d out of range", ei, in.Op)
				}
			}
		}
	}
	for fi, f := range p.Filters {
		if f.Or != 0 && int(f.Or) >= fi { // an alternative chain points to earlier filters (acyclic)
			t.Errorf("filter %d: Or %d out of range", fi, f.Or)
		}
		if int(f.Sub) > len(p.Subtypes) || int(f.NotSub) > len(p.Subtypes) {
			t.Errorf("filter %d: subtype out of range", fi)
		}
		checkValue(t, p, "filter", fi, f.PredV)
	}
	for ei, e := range p.Exprs {
		if ei == 0 {
			continue
		}
		if e.Kind == kop.ExNone || e.Kind >= kop.ExCount_ {
			t.Errorf("expr %d: kind %d", ei, e.Kind)
		}
		if int(e.F) >= len(p.Filters) {
			t.Errorf("expr %d: filter out of range", ei)
		}
		for _, s := range e.Sub {
			checkValue(t, p, "expr", ei, s)
		}
	}
	for ci, c := range p.Conds {
		if c.Kind >= kop.CondCount_ || int(c.F) >= len(p.Filters) {
			t.Errorf("cond %d: kind %d filter %d", ci, c.Kind, c.F)
		}
		checkValue(t, p, "cond", ci, c.A)
		checkValue(t, p, "cond", ci, c.B)
	}
	if len(p.OverrideSubs) != len(p.Overrides) {
		t.Errorf("p.OverrideSubs %d != p.Overrides %d", len(p.OverrideSubs), len(p.Overrides))
	}
}

func checkValue(t *testing.T, p *cardset.PoolData, what string, i int, v kop.ValueRef) {
	t.Helper()
	switch v.K {
	case kop.VExpr:
		if v.N <= 0 || int(v.N) >= len(p.Exprs) {
			t.Errorf("%s %d: expr %d out of range", what, i, v.N)
		}
	case kop.VCountFilter:
		if v.N <= 0 || int(v.N) >= len(p.Filters) {
			t.Errorf("%s %d: count filter %d out of range", what, i, v.N)
		}
	default:
		if v.K > kop.VExpr {
			t.Errorf("%s %d: value kind %d", what, i, v.K)
		}
	}
}

func checkTarget(t *testing.T, p *cardset.PoolData, card string, ai int, ts kop.TargetSpec) {
	t.Helper()
	if int(ts.Filter) >= len(p.Filters) {
		t.Errorf("%s ab %d: target filter %d out of range", card, ai, ts.Filter)
	}
	if ts.Kind > kop.TgtCard {
		t.Errorf("%s ab %d: target kind %d", card, ai, ts.Kind)
	}
}

// TestOverrideGrants checks override records grant abilities that exist on
// some card (the grant names an ability index of the applying card).
func TestOverrideGrants(t *testing.T) {
	p := fdnData(t)
	for oi, o := range p.Overrides {
		for _, g := range o.Grant {
			if g == 0 {
				continue
			}
			ok := false
			for i := 0; i < p.NumCards+p.NumTokens && !ok; i++ {
				ok = p.Status[i] != cardset.StatusNotLowered && int(g) <= len(p.Defs[i].Abilities)
			}
			if !ok {
				t.Errorf("override %d grants ability %d which no card has", oi, g)
			}
		}
	}
}

func checkAbility(t *testing.T, p *cardset.PoolData, card string, d *kop.CardDef, ai int, ab *kop.Ability) {
	t.Helper()
	if int(ab.NTargets) > kop.MaxTargets {
		t.Errorf("%s ab %d: %d targets", card, ai, ab.NTargets)
	}
	for ti := 0; ti < int(ab.NTargets); ti++ {
		checkTarget(t, p, card, ai, ab.Targets[ti])
		if ab.TgtModes[ti]>>ab.NModes != 0 {
			t.Errorf("%s ab %d: target %d modes %#x beyond %d modes", card, ai, ti, ab.TgtModes[ti], ab.NModes)
		}
	}
	for _, f := range []kop.FilterID{ab.Cost.Filter, ab.Trig.Filter, ab.Trig.Filter2, ab.Static.Filter, ab.Static.Filter2, ab.Repl.Filter, ab.ManaRestrict} {
		if int(f) >= len(p.Filters) {
			t.Errorf("%s ab %d: filter %d out of range", card, ai, f)
		}
	}
	for _, c := range []kop.CondID{ab.Trig.Cond, ab.Static.Cond2, ab.Repl.Cond} {
		if int(c) >= len(p.Conds) {
			t.Errorf("%s ab %d: cond %d out of range", card, ai, c)
		}
	}
	if ab.Static.Grant > 0 && int(ab.Static.Grant) > len(d.Abilities) {
		t.Errorf("%s ab %d: static grant %d out of range", card, ai, ab.Static.Grant)
	}
	if ab.Static.Override > 0 && int(ab.Static.Override) > len(p.Overrides) {
		t.Errorf("%s ab %d: static override %d out of range", card, ai, ab.Static.Override)
	}
	checkValue(t, p, card, ai, ab.Static.VA)
	checkValue(t, p, card, ai, ab.Static.VB)
	checkValue(t, p, card, ai, ab.ManaV)
	for pc, in := range ab.Prog {
		if in.Op == kop.OpNop || !in.Op.Known() {
			t.Errorf("%s ab %d instr %d: op %d", card, ai, pc, in.Op)
			continue
		}
		if in.Obj > kop.RefHost {
			t.Errorf("%s ab %d instr %d %s: ref %d", card, ai, pc, in.Op, in.Obj)
		}
		if in.Obj >= kop.RefTarget0 && in.Obj <= kop.RefTarget2 && int(in.Obj-kop.RefTarget0) >= int(ab.NTargets) {
			t.Errorf("%s ab %d instr %d %s: target ref beyond %d slots", card, ai, pc, in.Op, ab.NTargets)
		}
		if int(in.F) >= len(p.Filters) {
			t.Errorf("%s ab %d instr %d %s: filter %d out of range", card, ai, pc, in.Op, in.F)
		}
		checkValue(t, p, card, ai, in.A)
		checkValue(t, p, card, ai, in.B)
		skip := -1
		switch in.Op {
		case kop.OpCharm, kop.OpBranch, kop.OpSkipUnless, kop.OpRepeatEachPlayer:
			skip = int(in.A.N)
		case kop.OpUnlessPaid, kop.OpIfPaid:
			skip = int(in.C >> 4)
		}
		if skip >= 0 && pc+skip >= len(ab.Prog) {
			t.Errorf("%s ab %d instr %d %s: skip %d past the program end", card, ai, pc, in.Op, skip)
		}
		switch in.Op {
		case kop.OpSkipUnless:
			if in.C <= 0 || int(in.C) >= len(p.Conds) {
				t.Errorf("%s ab %d instr %d: cond %d out of range", card, ai, pc, in.C)
			}
		case kop.OpCharm:
			if in.C < 0 || int(in.C) >= int(ab.NModes) {
				t.Errorf("%s ab %d instr %d: charm mode %d of %d", card, ai, pc, in.C, ab.NModes)
			}
		case kop.OpCreateToken, kop.OpCreateTokenEx:
			if int(in.C) < p.NumCards || int(in.C) >= p.NumCards+p.NumTokens {
				t.Errorf("%s ab %d instr %d: token %d out of range", card, ai, pc, in.C)
			}
		case kop.OpDelayedProgram, kop.OpReflexiveTrigger:
			if in.C < 0 || int(in.C) >= len(d.Abilities) || d.Abilities[in.C].AFlags&kop.AfLatent == 0 {
				t.Errorf("%s ab %d instr %d: latent ability %d", card, ai, pc, in.C)
			}
		case kop.OpOverride:
			if in.C < 0 || int(in.C) >= len(p.Overrides) {
				t.Errorf("%s ab %d instr %d: override %d out of range", card, ai, pc, in.C)
			}
		case kop.OpCreateEmblem:
			if in.C < 0 || int(in.C) >= len(p.Emblems) {
				t.Errorf("%s ab %d instr %d: emblem %d out of range", card, ai, pc, in.C)
			}
		case kop.OpPutCounter, kop.OpRemoveCounter, kop.OpRemoveCounterRem, kop.OpEnterWithCounters:
			if in.C < 0 || kop.CounterKind(in.C) >= kop.NumCounterKinds2 {
				t.Errorf("%s ab %d instr %d: counter kind %d", card, ai, pc, in.C)
			}
		case kop.OpFight, kop.OpBite:
			r := kop.Ref(in.C)
			if r < kop.RefTarget0 || int(r-kop.RefTarget0) >= int(ab.NTargets) {
				t.Errorf("%s ab %d instr %d: fight/bite target ref %d", card, ai, pc, in.C)
			}
		}
	}
}

// Pool is one table over every pool card (CardID = pool index) and its
// tokens, the unsupported cards inert.
func TestPoolTable(t *testing.T) {
	set := cardsettest.Set(t, "fdn")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	if len(tbl.Cards) < set.NumCards() {
		t.Fatalf("%d cards, want at least %d", len(tbl.Cards), set.NumCards())
	}
	inert := 0
	for i := 0; i < set.NumCards(); i++ {
		if tbl.Cards[i].Name != set.Names()[i] {
			t.Fatalf("CardID %d is %q, want %q", i, tbl.Cards[i].Name, set.Names()[i])
		}
		if set.Inert(i) {
			inert++
			if len(tbl.Cards[i].Abilities) != 0 || tbl.Cards[i].Keywords != 0 {
				t.Fatalf("inert %q keeps abilities", set.Names()[i])
			}
		}
	}
	t.Logf("%d cards (%d inert), %d subtypes", len(tbl.Cards), inert, len(tbl.Subtypes))
}

// Games over the pool table with the inert fallbacks in the decks run to
// their end: an inert card plays as a vanilla card.
func TestPoolTableInertGames(t *testing.T) {
	set := cardsettest.Set(t, "fdn")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	var inert []kop.CardID
	for i := 0; i < set.NumCards(); i++ {
		if set.Inert(i) {
			inert = append(inert, kop.CardID(i))
		}
	}
	if len(inert) == 0 {
		t.Skip("every pool card is supported: no inert fallback to play")
	}
	land := func(name string) kop.CardID {
		i, ok := set.Lookup(name)
		if !ok {
			t.Fatalf("no %s", name)
		}
		return kop.CardID(i)
	}
	var deck []kop.CardID
	for len(deck) < 24 {
		deck = append(deck, inert...)
	}
	deck = deck[:24]
	for _, l := range []string{"Plains", "Island", "Swamp", "Mountain", "Forest"} {
		for k := 0; k < 4; k++ {
			deck = append(deck, land(l))
		}
	}
	for g := uint64(1); g <= 30; g++ {
		var s dirac.State
		s.Init(tbl, deck, deck, g, dirac.Config{})
		pol := dirac.NewRandomPolicy(g)
		if _, done := dirac.PlayRandom(&s, &pol, 20000); !done {
			t.Fatalf("game %d did not end", g)
		}
	}
}
