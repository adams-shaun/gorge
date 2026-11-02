package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// novijenSagesEngine puts the real corpus Novijen Sages (a 0/0 with Graft 4:
// "{1}, Remove two +1/+1 counters from among creatures you control: Draw a
// card.", RemoveAnyCounter<2/P1P1/Creature>) and a Grizzly Bears on seat 0's
// battlefield, the Sages carrying sagesN +1/+1 counters and the Bears bearN,
// with {C} floating.
func novijenSagesEngine(t *testing.T, sagesN, bearN int32) (e *Engine, sages, bear state.ObjID) {
	t.Helper()
	e = handEngine(t,
		corpusAlternativeCard(t, "Novijen Sages"),
		corpusAlternativeCard(t, "Grizzly Bears"),
		corpusAlternativeCard(t, "Grizzly Bears"),
	)
	hand := e.G.Zone(state.ZHand, 0)
	sages, bear = hand[0], hand[1]
	placeOnBattlefield(t, e, bear)
	placeOnBattlefield(t, e, sages)
	if d := sagesN - e.G.Obj(sages).Counter("P1P1"); d != 0 {
		e.emit(events.Event{Kind: events.CounterChange, Obj: sages, Counter: "P1P1", Amount: d})
	}
	e.emit(events.Event{Kind: events.CounterChange, Obj: bear, Counter: "P1P1", Amount: bearN})
	e.pending = nil
	addMana(t, e, 0, "C")
	if o := e.G.Obj(sages); o.Zone != state.ZBattlefield || o.Counter("P1P1") != sagesN {
		t.Fatalf("setup: Sages zone %v counters %d, want battlefield with %d", o.Zone, o.Counter("P1P1"), sagesN)
	}
	return e, sages, bear
}

func sagesDrawOption(e *Engine, sages state.ObjID) int {
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		return -1
	}
	for _, o := range d.Options {
		if o.Kind == "ability" && o.Obj == sages {
			return o.Index
		}
	}
	return -1
}

// TestFixedKindCountersFromAmongCreatures: "remove two +1/+1 counters from
// among creatures you control" is a TOTAL across the payer's creatures (CR
// 118.3, 601.2h), the same reading as Quilled Greatwurm's wildcard part, so
// the Sages and the Bears carrying one counter each pay it, one counter from
// each.
func TestFixedKindCountersFromAmongCreatures(t *testing.T) {
	t.Parallel()
	e, sages, bear := novijenSagesEngine(t, 1, 1)
	idx := sagesDrawOption(e, sages)
	if idx < 0 {
		t.Fatalf("Novijen Sages' draw not offered with 1+1 counters among two creatures: pending=%+v", e.Pending())
	}
	hand := len(e.G.Zone(state.ZHand, 0))
	submitChoices(t, e, idx)
	for i := 0; i < 6; i++ {
		d := e.Pending()
		if d == nil || d.Kind != decision.KChoose {
			break
		}
		pick := -1
		for _, o := range d.Options {
			if o.Kind == "subcounter" {
				pick = o.Index
				break
			}
		}
		if pick < 0 {
			t.Fatalf("counter-removal ask without a subcounter option: %+v", d)
		}
		submitChoices(t, e, pick)
	}
	if got := e.G.Obj(bear).Counter("P1P1"); got != 0 {
		t.Fatalf("Bears' counters after payment = %d, want 0", got)
	}
	if got := e.G.Obj(sages).Counter("P1P1"); got != 0 {
		t.Fatalf("Sages' counters after payment = %d, want 0", got)
	}
	passUntilStackEmpty(t, e, 30)
	if got := len(e.G.Zone(state.ZHand, 0)); got != hand+1 {
		t.Fatalf("hand after the draw = %d, want %d", got, hand+1)
	}
}

// TestFixedKindCountersAmongShortfallNotOffered is the boundary: one counter
// among the payer's creatures cannot pay two, so the ability stays unoffered.
func TestFixedKindCountersAmongShortfallNotOffered(t *testing.T) {
	t.Parallel()
	e, sages, _ := novijenSagesEngine(t, 1, 0)
	if idx := sagesDrawOption(e, sages); idx >= 0 {
		t.Fatalf("Novijen Sages' draw offered with only one counter among creatures")
	}
}

// TestFixedKindCountersOnOnePermanentStillOffered: two counters on one
// creature keep paying the part (the pre-existing single-permanent case).
func TestFixedKindCountersOnOnePermanentStillOffered(t *testing.T) {
	t.Parallel()
	e, sages, _ := novijenSagesEngine(t, 2, 0)
	if idx := sagesDrawOption(e, sages); idx < 0 {
		t.Fatalf("Novijen Sages' draw not offered with two counters on the Sages: pending=%+v", e.Pending())
	}
}

// TestJetfireManaAbilityRemovesCountersFromAmongArtifacts: Jetfire,
// Ingenious Scientist's `Cost$ RemoveAnyCounter<X1+/P1P1/Artifact>` mana
// ability removes X +1/+1 counters from among ARTIFACTS you control (CR
// 118.3: a total across permanents), so the Jetfire and an Ornithopter
// carrying one counter each can pay X = 2, adding {C}{C}.
func TestJetfireManaAbilityRemovesCountersFromAmongArtifacts(t *testing.T) {
	t.Parallel()
	e := handEngine(t,
		corpusAlternativeCard(t, "Jetfire, Ingenious Scientist"),
		corpusAlternativeCard(t, "Ornithopter"),
		corpusAlternativeCard(t, "Grizzly Bears"),
	)
	hand := e.G.Zone(state.ZHand, 0)
	jet, orni := hand[0], hand[1]
	placeOnBattlefield(t, e, orni)
	placeOnBattlefield(t, e, jet)
	for _, id := range []state.ObjID{jet, orni} {
		if d := 1 - e.G.Obj(id).Counter("P1P1"); d != 0 {
			e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "P1P1", Amount: d})
		}
	}
	e.pending = nil
	toMain1(t, e)
	e.priorityRound()
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("no priority decision: %+v", d)
	}
	idx := -1
	for _, o := range d.Options {
		if (o.Kind == "ability" || o.Kind == "mana") && o.Obj == jet {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("Jetfire's counter-removal mana ability is not offered with one counter on each of two artifacts: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	for i := 0; i < 12; i++ {
		d := e.Pending()
		if d == nil || (d.Kind != decision.KChoose && d.Kind != decision.KTarget) {
			break
		}
		pick := 0
		// The X announcement offers amounts; take the largest (2).
		for _, o := range d.Options {
			if o.Kind == "x" || o.Kind == "number" {
				pick = o.Index
			}
		}
		for _, o := range d.Options {
			if o.Kind == "subcounter" {
				pick = o.Index
				break
			}
		}
		submitChoices(t, e, pick)
	}
	// The ability targets a player, so it is not a mana ability (CR 605.1a):
	// it resolves from the stack.
	passUntilStackEmpty(t, e, 30)
	if got := e.G.Obj(jet).Counter("P1P1") + e.G.Obj(orni).Counter("P1P1"); got != 0 {
		t.Errorf("counters left on the two artifacts = %d, want 0 (pending %+v)", got, e.Pending())
	}
	if got := e.G.Players[0].Pool.Total(); got != 2 {
		t.Errorf("mana floating = %d, want 2", got)
	}
}
