package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// The exotic <Ref>$<Property> count heads (task rv2b-countheads). Each head
// reads the state the property names, off the object or player the ref
// resolves, instead of degrading to the fail-closed zero. The tests below
// drive the REAL corpus spellings (TriggeredCard$CardNumColors,
// TriggeredTarget$LifeTotal) through the compiled-card path where the corpus
// names one, and pin the evaluator directly where a synthetic board makes the
// precondition unambiguous.

// CardNumColors over a referenced object: the colour count of THAT object,
// not of the resolving source. Evaluated per referenced object, so a ref that
// names several sums.
func TestRefPropertyCardNumColorsReadsTheReferencedObject(t *testing.T) {
	h, c := fixtureHost(t)
	// A three-colour referenced card in the graveyard (the c.Remembered
	// spelling TriggeredCard/Targeted resolve to) and a colourless source.
	multi := mkCard(t, "Name:Multi\nManaCost:1 U R G\nTypes:Creature\nPT:1/1\nOracle:x\n")
	m := h.g.AddObject(multi, 1)
	if got := h.ObjectColors(m); len(got) != 3 {
		t.Fatalf("precondition: referenced card colours = %q, want 3 distinct", got)
	}
	c.Remembered = []state.Target{{Obj: m.ID}}
	if got := EvalCount(h, c, "Remembered$CardNumColors"); got != 3 {
		t.Errorf("Remembered$CardNumColors = %d, want 3 (the referenced card's colours)", got)
	}
	// The Targeted spelling reads the SAME referenced object; the source
	// (a single-colourless fixture) must not leak in.
	c.Targets = []state.Target{{Obj: m.ID}}
	if got := EvalCount(h, c, "Targeted$CardNumColors"); got != 3 {
		t.Errorf("Targeted$CardNumColors = %d, want 3", got)
	}
	// A colourless referenced object is a real zero, not an unresolvable one.
	plain := mkCard(t, "Name:Plain\nTypes:Artifact\nOracle:x\n")
	p := h.g.AddObject(plain, 0)
	c.Targets = []state.Target{{Obj: p.ID}}
	if got := EvalCount(h, c, "Targeted$CardNumColors"); got != 0 {
		t.Errorf("colourless Targeted$CardNumColors = %d, want 0", got)
	}
}

// LifeTotal over a referenced player: the referenced player's current life
// (the TriggeredTarget spelling the DamageDone family uses), with the /Op
// suffix riding the ordinary read like every other head.
func TestRefPropertyLifeTotalReadsTheReferencedPlayer(t *testing.T) {
	h, c := fixtureHost(t)
	h.g.Players[1].Life = 13
	if h.g.Players[1].Life == h.g.Players[0].Life {
		t.Fatal("precondition: the two players' life must differ")
	}
	c.TriggerTarget = state.Target{Player: 1, IsPlayer: true}
	if got := EvalCount(h, c, "TriggeredTarget$LifeTotal"); got != 13 {
		t.Errorf("TriggeredTarget$LifeTotal = %d, want 13", got)
	}
	if got := EvalCount(h, c, "TriggeredTarget$LifeTotal/HalfUp"); got != 7 {
		t.Errorf("TriggeredTarget$LifeTotal/HalfUp = %d, want 7", got)
	}
	// A second player ref the engine already resolves (DefendingPlayer):
	// the same LifeTotal reader, off the other role.
	h.g.Players[0].Life = 4
	c.DefendingPlayer = state.Target{Player: 0, IsPlayer: true}
	if got := EvalCount(h, c, "TriggeredDefendingPlayer$LifeTotal"); got != 4 {
		t.Errorf("TriggeredDefendingPlayer$LifeTotal = %d, want 4", got)
	}
}

// CardCounters.AGE / .ALL over a referenced object: the age counters a
// permanent accumulated (cumulative upkeep) and the full counter sum. AGE is
// a REAL counter kind in this engine (rules/cumulative.go emits a CounterChange
// "AGE"), so the head reads it off the object directly; ALL is the wildcard
// sum. The ref is TriggeredCard (the corpus spelling), bound to c.Remembered.
func TestRefPropertyCardCountersReadsTheReferencedObject(t *testing.T) {
	h, c := fixtureHost(t)
	o := h.g.Obj(c.Source)
	o.AddCounter("AGE", 3)
	o.AddCounter("P1P1", 2)
	if o.Counter("AGE") != 3 || o.Counter("P1P1") != 2 {
		t.Fatal("precondition: the source must carry 3 AGE and 2 P1P1 counters")
	}
	c.Remembered = []state.Target{{Obj: c.Source}}
	if got := EvalCount(h, c, "TriggeredCard$CardCounters.AGE"); got != 3 {
		t.Errorf("TriggeredCard$CardCounters.AGE = %d, want 3", got)
	}
	if got := EvalCount(h, c, "TriggeredCard$CardCounters.ALL"); got != 5 {
		t.Errorf("TriggeredCard$CardCounters.ALL = %d, want 5", got)
	}
	// The AGE read is on the REFERRED object, not the source: move the ref to
	// the other fixture, which carries no age counters.
	other := state.ObjID(2)
	if o2 := h.g.Obj(other); o2 == nil || o2.Counter("AGE") != 0 {
		t.Fatal("precondition: the other fixture must carry no age counters")
	}
	c.Remembered = []state.Target{{Obj: other}}
	if got := EvalCount(h, c, "TriggeredCard$CardCounters.AGE"); got != 0 {
		t.Errorf("other-object TriggeredCard$CardCounters.AGE = %d, want 0", got)
	}
}

// CastTotalManaSpent over a referenced object: the mana spent to cast THAT
// spell, not the resolving source's own spend.
func TestRefPropertyCastTotalManaSpentReadsTheReferencedObject(t *testing.T) {
	h, c := fixtureHost(t)
	o := h.g.Obj(c.Source)
	o.ManaSpent = 4
	o.ManaSnowSpent = 1
	c.Remembered = []state.Target{{Obj: c.Source}}
	if got := EvalCount(h, c, "TriggeredCard$CastTotalManaSpent"); got != 4 {
		t.Errorf("TriggeredCard$CastTotalManaSpent = %d, want 4", got)
	}
	if got := EvalCount(h, c, "TriggeredCard$CastTotalManaSpent Snow"); got != 1 {
		t.Errorf("TriggeredCard$CastTotalManaSpent Snow = %d, want 1", got)
	}
	// The referred object carries no spend: a real zero.
	other := state.ObjID(2)
	c.Remembered = []state.Target{{Obj: other}}
	if got := EvalCount(h, c, "TriggeredCard$CastTotalManaSpent"); got != 0 {
		t.Errorf("other-object TriggeredCard$CastTotalManaSpent = %d, want 0", got)
	}
}

// The synthetic eval above is not enough on its own: a reviewer can revert the
// handler and leave the synthetic board green. These two read the REAL corpus
// SA bodies the row's heads carry -- Moonveil Regent's TriggeredCard$CardNumColors
// and Quietus Spike's TriggeredTarget$LifeTotal/HalfUp -- so the compiled-script
// path is what answers.
func TestRefPropertyCardNumColorsReadsTheRealCorpusSVar(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup("Moonveil Regent")
	if !ok {
		t.Fatal("corpus missing Moonveil Regent")
	}
	body := card.Faces[0].SVars["X"]
	if body != "TriggeredCard$CardNumColors" {
		t.Fatalf("Moonveil Regent SVar X = %q, want TriggeredCard$CardNumColors", body)
	}
	h, c := fixtureHost(t)
	multi := mkCard(t, "Name:Multi\nManaCost:1 U R G\nTypes:Creature\nPT:1/1\nOracle:x\n")
	m := h.g.AddObject(multi, 0)
	if got := h.ObjectColors(m); len(got) != 3 {
		t.Fatalf("precondition: referenced card colours = %q, want 3 distinct", got)
	}
	c.Remembered = []state.Target{{Obj: m.ID}}
	if got := EvalCount(h, c, body); got != 3 {
		t.Errorf("real Moonveil Regent SVar = %d, want 3", got)
	}
}

func TestRefPropertyLifeTotalReadsTheRealCorpusSVar(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup("Quietus Spike")
	if !ok {
		t.Fatal("corpus missing Quietus Spike")
	}
	body := card.Faces[0].SVars["QuietusX"]
	if body != "TriggeredTarget$LifeTotal/HalfUp" {
		t.Fatalf("Quietus Spike SVar QuietusX = %q, want TriggeredTarget$LifeTotal/HalfUp", body)
	}
	h, c := fixtureHost(t)
	h.g.Players[1].Life = 13
	if h.g.Players[1].Life == h.g.Players[0].Life {
		t.Fatal("precondition: the two players' life must differ")
	}
	c.TriggerTarget = state.Target{Player: 1, IsPlayer: true}
	if got := EvalCount(h, c, body); got != 7 {
		t.Errorf("real Quietus Spike SVar = %d, want 7 (half of 13, rounded up)", got)
	}
}
