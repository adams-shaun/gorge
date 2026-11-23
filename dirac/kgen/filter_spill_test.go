package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

func spillClause(t *testing.T, expr string) (lClause, error) {
	t.Helper()
	f, err := parseFilter(expr, func(w string) bool { return w == "Spider" || w == "Hero" }, func(string) bool { return false })
	if err != nil {
		return lClause{}, err
	}
	if len(f.Clauses) != 1 {
		t.Fatalf("%q: %d clauses", expr, len(f.Clauses))
	}
	return f.Clauses[0], nil
}

func spillBuilder() *cardBuilder {
	return &cardBuilder{pt: &poolTables{subIdx: map[string]int{"Spider": 3, "Hero": 5}, missing: map[string]bool{},
		filterKey: map[string]kop.FilterID{}, exprKey: map[string]int16{}}}
}

// progOf builds the clause and returns its program's literals.
func progOf(t *testing.T, cb *cardBuilder, c lClause, zone kop.Zone) ([]kop.FLit, kop.Filter) {
	t.Helper()
	kf, err := cb.clause(c, zone)
	if err != nil {
		t.Fatal(err)
	}
	if kf.Pred != kop.PredProg || kf.PredN <= 0 || int(kf.PredN) >= len(cb.pt.progs) {
		t.Fatalf("Pred %d PredN %d: not a program of %d", kf.Pred, kf.PredN, len(cb.pt.progs))
	}
	return cb.pt.progs[kf.PredN].Lits, kf
}

// TestFilterSpillPowerCMC: a power and a mana value compare in one clause
// (the struct has one Pred slot) become a two-literal program.
func TestFilterSpillPowerCMC(t *testing.T) {
	c, err := spillClause(t, "Creature.powerGE2+cmcLE3")
	if err != nil {
		t.Fatal(err)
	}
	if c.CmpStat != cmpSpill || c.NSpill != 2 {
		t.Fatalf("CmpStat %d NSpill %d, want a two-literal spill", c.CmpStat, c.NSpill)
	}
	for _, zone := range []kop.Zone{kop.ZoneBattlefield, kop.ZoneGraveyard, kop.ZoneStack, kop.ZoneNone} {
		lits, kf := progOf(t, spillBuilder(), c, zone)
		want := []kop.FLit{{W: kop.FwPowerCmp, Op: kop.CmpGE, V: kop.Const(2)}, {W: kop.FwCMCCmp, Op: kop.CmpLE, V: kop.Const(3)}}
		if len(lits) != 2 || lits[0] != want[0] || lits[1] != want[1] {
			t.Errorf("zone %v: program %+v, want %+v", zone, lits, want)
		}
		if kf.Types != kop.TCreature {
			t.Errorf("zone %v: Types %#x, the struct keeps what fits", zone, kf.Types)
		}
	}
}

// TestFilterSpillLacksKeywords: four "without" words are four negated
// FwKeyword literals; a program shared by two clauses is interned once.
func TestFilterSpillLacksKeywords(t *testing.T) {
	c, err := spillClause(t, "Creature.withoutFirst Strike+withoutDouble Strike+withoutVigilance+withoutHaste")
	if err != nil {
		t.Fatal(err)
	}
	cb := spillBuilder()
	lits, _ := progOf(t, cb, c, kop.ZoneBattlefield)
	if len(lits) != 4 {
		t.Fatalf("program %+v, want four literals", lits)
	}
	for _, l := range lits {
		if l.W != kop.FwKeyword || !l.Neg {
			t.Errorf("literal %+v, want a negated FwKeyword", l)
		}
	}
	_, kf2 := progOf(t, cb, c, kop.ZoneBattlefield)
	if len(cb.pt.progs) != 2 || kf2.PredN != 1 {
		t.Errorf("program interned %d times (PredN %d), want once", len(cb.pt.progs)-1, kf2.PredN)
	}
}

// TestFilterSpillCountersBattlefieldOnly: FwCountersCmp declares only the perm
// matcher, so the pair builds on the battlefield and refuses elsewhere and in
// a static (read in the layer walk too), with the word-in-context class.
func TestFilterSpillCountersBattlefieldOnly(t *testing.T) {
	c, err := spillClause(t, "Card.Self+counters_GE1_LEVEL+counters_LE3_LEVEL")
	if err != nil {
		t.Fatal(err)
	}
	if c.NSpill != 2 {
		t.Fatalf("NSpill %d", c.NSpill)
	}
	lits, _ := progOf(t, spillBuilder(), c, kop.ZoneBattlefield)
	if len(lits) != 2 || lits[0].W != kop.FwCountersCmp || lits[0].Op != kop.CmpGE || lits[1].Op != kop.CmpLE || lits[1].V != kop.Const(3) {
		t.Errorf("program %+v", lits)
	}
	for zone, ctx := range map[kop.Zone]string{kop.ZoneGraveyard: "card", kop.ZoneStack: "spell", kop.ZoneNone: "card"} {
		_, err := spillBuilder().clause(c, zone)
		if err == nil || !strings.Contains(err.Error(), "filter word FwCountersCmp in ") {
			t.Fatalf("zone %v: got %v, want the word-in-context refusal", zone, err)
		}
		if got := refusalClasses(err.Error()); len(got) != 1 || got[0] != "other:filter word FwCountersCmp in "+ctx {
			t.Errorf("zone %v: classes %v, want [other:filter word FwCountersCmp in %s]", zone, got, ctx)
		}
	}
	cb := spillBuilder()
	cb.inStatic = true
	if _, err := cb.clause(c, kop.ZoneBattlefield); err == nil || !strings.Contains(err.Error(), "filter word FwCountersCmp in cur") {
		t.Errorf("static: got %v, want a cur refusal", err)
	}
}

// TestFilterSpillStaticPowerRefuses: a power word is never read in the layer
// walk (no re-entry), so a spilled static filter refuses.
func TestFilterSpillStaticPowerRefuses(t *testing.T) {
	c, err := spillClause(t, "Creature.powerGE2+cmcLE3")
	if err != nil {
		t.Fatal(err)
	}
	cb := spillBuilder()
	cb.inStatic = true
	if _, err := cb.clause(c, kop.ZoneBattlefield); err == nil || !strings.Contains(err.Error(), "filter word FwPowerCmp in cur") {
		t.Fatalf("got %v", err)
	}
	if walkFilterOK(&lFilter{Clauses: []lClause{c}}) == nil {
		t.Error("walkFilterOK admits a spilled P/T clause")
	}
}

// TestFilterSpillSubtype2: a second subtype beside a compare (both want the
// Pred slot) is a program of the compare and an FwSubtype.
func TestFilterSpillSubtype2(t *testing.T) {
	c, err := spillClause(t, "Spider.Hero+cmcLE3")
	if err != nil {
		t.Fatal(err)
	}
	lits, kf := progOf(t, spillBuilder(), c, kop.ZoneLibrary)
	if len(lits) != 2 || lits[0].W != kop.FwCMCCmp || lits[1].W != kop.FwSubtype || lits[1].Arg != 5 {
		t.Errorf("program %+v, want cmc then subtype Hero (5)", lits)
	}
	if kf.Sub != 4 {
		t.Errorf("Sub %d, want Spider (3+1) in the struct", kf.Sub)
	}
	if c, err := spillClause(t, "Spider.Hero+MultiColor"); err != nil {
		t.Fatal(err)
	} else if _, err := spillBuilder().clause(c, kop.ZoneLibrary); err == nil || !strings.Contains(err.Error(), "two subtypes beside a compare") {
		t.Errorf("a compare with no word: got %v, want the old refusal", err)
	}
}

// TestFilterSpillKeepsOldLowerings: what lowered before is untouched (no
// spill, and its LIR text carries no program), and a pair with no word for one
// side still refuses as it did.
func TestFilterSpillKeepsOldLowerings(t *testing.T) {
	for _, expr := range []string{"Creature.powerGE4+powerLE6", "Creature.cmcLE3", "Creature.YouCtrl+powerGE2", "Creature.withoutFlying"} {
		c, err := spillClause(t, expr)
		if err != nil {
			t.Fatalf("%s: %v", expr, err)
		}
		if c.NSpill != 0 || c.CmpStat == cmpSpill || strings.Contains(c.String(), "prog") {
			t.Errorf("%s spilled: %s", expr, c)
		}
	}
	// the pairs the power/toughness box cannot fold (NE, a bound past 14, another stat) spill
	for expr, nw := range map[string][2]kop.FilterWord{"Creature.powerNE1+toughnessEQ1": {kop.FwPowerCmp, kop.FwToughnessCmp},
		"Creature.powerGE15+toughnessEQ1": {kop.FwPowerCmp, kop.FwToughnessCmp}, "Creature.cmcEQ1+powerEQ1": {kop.FwCMCCmp, kop.FwPowerCmp}} {
		c, err := spillClause(t, expr)
		if err != nil || c.NSpill != 2 || c.Spill[0].W != nw[0] || c.Spill[1].W != nw[1] {
			t.Errorf("%s: %v, %s", expr, err, c)
		}
	}
	for _, expr := range []string{"Permanent.cmcLE3+MultiColor+YouCtrl", "Creature.totalPT_GE5+totalPT_LE8", "Creature.YouCtrl+counters_GE1_P1P1+attackedThisTurn", "Creature.powerGE2+cmcLEX"} {
		if _, err := spillClause(t, expr); err == nil || !strings.Contains(err.Error(), "two comparisons in one clause") {
			t.Errorf("%s: got %v, want the two-comparisons refusal", expr, err)
		}
	}
	// the Historic split still reads the refusal's text: its three alternatives
	if f, err := parseFilter("Permanent.Historic+cmcGE3", func(string) bool { return false }, func(string) bool { return false }); err != nil || len(f.Clauses) != 3 {
		t.Errorf("Historic+cmcGE3: %v, %v", f, err)
	}
}

// TestFilterSpillCountProg: a Count$Valid over a filter the struct cannot
// hold lowers to ExCount (ValueRef VCountFilter) whose filter's predicate is
// the program — the count value reads every zone's objects through the same
// filter matcher, so the spill needs no count-specific wiring; the head's
// census rows that still refuse (aggregates, zone unions, SVar bounds) are
// other mechanisms, not this one.
func TestFilterSpillCountProg(t *testing.T) {
	f, err := parseFilter("Creature.powerGE2+cmcLE3", func(w string) bool { return w == "Spider" || w == "Hero" }, func(string) bool { return false })
	if err != nil {
		t.Fatal(err)
	}
	cb := spillBuilder()
	vr, err := cb.value(lValue{Kind: vCount, F: f})
	if err != nil {
		t.Fatalf("Count$Valid Creature.powerGE2+cmcLE3: %v", err)
	}
	if vr.K != kop.VCountFilter {
		t.Fatalf("value kind %d, want VCountFilter (%d)", vr.K, kop.VCountFilter)
	}
	kf := cb.pt.filters[vr.N]
	if kf.Pred != kop.PredProg {
		t.Fatalf("Pred %d, want PredProg", kf.Pred)
	}
	lits := cb.pt.progs[kf.PredN].Lits
	want := []kop.FLit{{W: kop.FwPowerCmp, Op: kop.CmpGE, V: kop.Const(2)}, {W: kop.FwCMCCmp, Op: kop.CmpLE, V: kop.Const(3)}}
	if len(lits) != 2 || lits[0] != want[0] || lits[1] != want[1] {
		t.Errorf("program %+v, want %+v", lits, want)
	}
	// a graveyard count lowers the same way (ExCountGraveyard's filter), on
	// the same interned program: the literals do not name a zone
	vr2, err := cb.value(lValue{Kind: vCountGraveyard, F: f})
	if err != nil {
		t.Fatalf("Count$ValidGraveyard: %v", err)
	}
	if kf := cb.pt.filters[vr2.N]; kf.Pred != kop.PredProg {
		t.Errorf("graveyard count Pred %d, want PredProg", kf.Pred)
	} else if cb.pt.filters[vr.N].PredN != kf.PredN {
		t.Errorf("the two counts interned programs %d and %d, want one", cb.pt.filters[vr.N].PredN, kf.PredN)
	}
	// the wordless pair still refuses, as before the spill (at parse time)
	if _, perr := parseFilter("Creature.powerGE2+cmcLEX", func(w string) bool { return false }, func(string) bool { return false }); perr == nil || !strings.Contains(perr.Error(), "two comparisons in one clause") {
		t.Errorf("Creature.powerGE2+cmcLEX: got %v, want the two-comparisons refusal", perr)
	}
}

// TestFilterSpillCountersOpsMirror: a spilled counter compare admits exactly
// the ops the struct's counterPred reads (GE to 127, LT from 1, LE to 126,
// EQ 0) and refuses the rest — a program never wider than the struct's read
// (filter_spill.go's cmpLit, mirroring build.go's counterPred).
func TestFilterSpillCountersOpsMirror(t *testing.T) {
	for expr, spilled := range map[string]bool{
		"Card.Self+counters_GE1_LEVEL+counters_LT2_LEVEL":   true,
		"Card.Self+counters_GE1_LEVEL+counters_LE126_LEVEL": true,
		"Card.Self+counters_GE1_LEVEL+counters_EQ0_LEVEL":   true,
		"Card.Self+counters_GE1_LEVEL+counters_LT0_LEVEL":   false, // cmp(LT, 0) is never true
		"Card.Self+counters_GE1_LEVEL+counters_GT1_LEVEL":   false,
		"Card.Self+counters_GE1_LEVEL+counters_NE1_LEVEL":   false,
		"Card.Self+counters_GE1_LEVEL+counters_LE127_LEVEL": false, // past the byte, as counterPred refuses
		"Card.Self+counters_GE1_LEVEL+counters_GE200_LEVEL": false,
	} {
		c, err := spillClause(t, expr)
		if !spilled {
			if err == nil || !strings.Contains(err.Error(), "two comparisons in one clause") {
				t.Errorf("%s: got %v, want the two-comparisons refusal", expr, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", expr, err)
			continue
		}
		if c.CmpStat != cmpSpill || c.NSpill != 2 {
			t.Errorf("%s: spilled %v (%s), want a two-literal program", expr, c.CmpStat == cmpSpill, c)
		}
	}
}
