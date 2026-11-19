package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// trigEAdaptEngine seats the real corpus Biomancer's Familiar (summoning
// sickness cleared: its ability taps) and the named inline adapt creatures on
// seat 0's battlefield. Counters named in withCounter land as one +1/+1
// counter, so those creatures cannot adapt by the printed rule.
func trigEAdaptEngine(t *testing.T, seed uint64, adapters []string, withCounter map[string]bool) (*Engine, Config, state.ObjID, map[string]state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	fam, ok := reg.Lookup("Biomancer's Familiar")
	if !ok {
		t.Fatal("corpus fixture: Biomancer's Familiar missing")
	}
	if m := reg.Unsupported(fam, effects.Supported()); len(m) != 0 {
		t.Fatalf("precondition: Biomancer's Familiar unsupported: %v", m)
	}
	deck := []*cards.Card{fam}
	for _, n := range adapters {
		deck = append(deck, card(t, "Name:Fix Adapter "+n+"\nManaCost:G\nTypes:Creature Beast\nPT:1/1\n"+
			"A:AB$ PutCounter | Cost$ 1 | Adapt$ 2 | SpellDescription$ Adapt 2.\nOracle:x\n"))
	}
	cfg := seatZeroStart(Config{Seed: seed, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{
			append(deck, mountainDeck(t, 40-len(deck))...),
			mountainDeck(t, 40),
		},
		Tokens: map[string]*cards.Card{},
	})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	famID := moveByName(t, e, 0, "Biomancer's Familiar", state.ZBattlefield)
	e.G.Obj(famID).SummonSick = false
	ids := map[string]state.ObjID{}
	for _, n := range adapters {
		id := moveByName(t, e, 0, "Fix Adapter "+n, state.ZBattlefield)
		ids[n] = id
		if withCounter[n] {
			e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "P1P1", Amount: 1})
		}
	}
	return e, cfg, famID, ids
}

// trigEActivateFamiliar activates Biomancer's Familiar targeting target.
func trigEActivateFamiliar(t *testing.T, e *Engine, fam, target state.ObjID) {
	t.Helper()
	opt := effectAbilityOption(t, e, fam)
	submitChoices(t, e, opt.Index)
	for i := 0; i < 6; i++ {
		d := e.Pending()
		if d == nil || d.Kind == decision.KPriority {
			break
		}
		pick := d.Options[0].Index
		for _, o := range d.Options {
			if o.Obj == target {
				pick = o.Index
			}
		}
		submitChoices(t, e, pick)
	}
	passUntilStackEmpty(t, e, 20)
}

// trigEAdaptNow pays {1} and resolves id's Adapt ability, failing if it is not
// offered.
func trigEAdaptNow(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	addMana(t, e, 0, "C")
	opt, ok := findAbilityOption(e, id, 0)
	if !ok {
		t.Fatalf("adapt not offered for %d: %+v", id, e.Pending())
	}
	submitChoices(t, e, opt.Index)
	passUntilStackEmpty(t, e, 20)
}

func trigEAdaptedCount(e *Engine, id state.ObjID) int {
	n := 0
	for _, ev := range e.L.Events {
		if events.IsAlterAttribute(ev, effects.AdaptedMark) && ev.Obj == id {
			n++
		}
	}
	return n
}

// TestEDHTrigEBiomancerAdapt plays Biomancer's Familiar's printed text: its
// Effect grants one creature "adapts as though it had no +1/+1 counters"
// (stat:CanAdapt), and the Effect's Mode$ Adapt trigger ends the grant when
// that creature adapts. A creature that already holds +1/+1 counters cannot
// adapt without the grant, and a creature the grant does not name -- here one
// with no counters, which adapts normally -- fires the Adapt trigger without
// spending the grant (the trigger's ValidCard$ Card.IsRemembered).
func TestEDHTrigEBiomancerAdapt(t *testing.T) {
	t.Parallel()
	e, _, fam, ids := trigEAdaptEngine(t, 9511, []string{"A", "B", "C"}, map[string]bool{"A": true, "B": true})
	a, b, c := ids["A"], ids["B"], ids["C"]
	for _, id := range []state.ObjID{a, b} {
		if got := e.G.Obj(id).Counter("P1P1"); got != 1 {
			t.Fatalf("precondition: creature %d holds %d +1/+1 counters, want 1", id, got)
		}
	}
	// Precondition: with counters and no grant, adapt is not offered, while the
	// counterless creature's is.
	addMana(t, e, 0, "CCC")
	if _, ok := findAbilityOption(e, a, 0); ok {
		t.Fatal("precondition: adapt offered to a creature holding +1/+1 counters before any grant")
	}
	if _, ok := findAbilityOption(e, c, 0); !ok {
		t.Fatalf("precondition: adapt not offered to the counterless creature: %+v", e.Pending())
	}
	e.G.Players[0].Pool = state.Mana{}

	trigEActivateFamiliar(t, e, fam, a)
	if !e.RestrictionApplies(effects.ModeCanAdapt, a) || e.RestrictionApplies(effects.ModeCanAdapt, b) || e.RestrictionApplies(effects.ModeCanAdapt, c) {
		t.Fatalf("grant coverage after the activation: A=%v B=%v C=%v, want A only",
			e.RestrictionApplies(effects.ModeCanAdapt, a), e.RestrictionApplies(effects.ModeCanAdapt, b), e.RestrictionApplies(effects.ModeCanAdapt, c))
	}

	// The counterless creature adapts normally: it fires Adapt, but the
	// trigger names only the remembered creature, so the grant survives.
	trigEAdaptNow(t, e, c)
	if got := e.G.Obj(c).Counter("P1P1"); got != 2 {
		t.Fatalf("counterless creature holds %d +1/+1 counters after adapting, want 2", got)
	}
	if trigEAdaptedCount(e, c) != 1 {
		t.Fatalf("counterless creature's adapt emitted %d Adapted marks, want 1", trigEAdaptedCount(e, c))
	}
	if !e.RestrictionApplies(effects.ModeCanAdapt, a) {
		t.Fatal("the grant ended when an unnamed creature adapted")
	}

	// The unnamed creature with counters still cannot adapt; the named one can.
	addMana(t, e, 0, "CC")
	if _, ok := findAbilityOption(e, b, 0); ok {
		t.Fatal("adapt offered to an unnamed creature holding +1/+1 counters")
	}
	if _, ok := findAbilityOption(e, a, 0); !ok {
		t.Fatalf("adapt not offered to the named creature despite its counters: %+v", e.Pending())
	}
	e.G.Players[0].Pool = state.Mana{}
	trigEAdaptNow(t, e, a)
	if got := e.G.Obj(a).Counter("P1P1"); got != 3 {
		t.Fatalf("named creature holds %d +1/+1 counters after adapting, want 3 (1 + Adapt 2)", got)
	}
	if trigEAdaptedCount(e, a) != 1 {
		t.Fatalf("named creature's adapt emitted %d Adapted marks, want 1", trigEAdaptedCount(e, a))
	}

	// "The next time": its Adapt trigger ended the grant, so the same
	// creature cannot adapt again with counters on it.
	if e.RestrictionApplies(effects.ModeCanAdapt, a) {
		t.Fatal("the grant outlived the adapt it was for")
	}
	addMana(t, e, 0, "CC")
	if _, ok := findAbilityOption(e, a, 0); ok {
		t.Fatal("adapt offered a second time to the named creature after its grant was spent")
	}
}
