package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// handOrLibraryCard returns seat p's card named name, moving a library copy
// into the hand through a logged MoveZone (a hand copy is returned as is, so
// the helper never emits a no-op move).
func handOrLibraryCard(t *testing.T, e *Engine, p state.PlayerID, name string) state.ObjID {
	t.Helper()
	for _, id := range e.G.Zone(state.ZHand, p) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
			return id
		}
	}
	for _, id := range e.G.Zone(state.ZLibrary, p) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZHand, Player: p})
			return id
		}
	}
	t.Fatalf("%q not found in seat %d's hand or library", name, p)
	return 0
}

// TestElspethResplendentShieldCounter drives the real Elspeth Resplendent's
// [-3]: look at the top seven, put a permanent card with mana value 3 or less
// onto the battlefield with a shield counter on it. The script spells the kind
// `WithCounters$ Shield`; it must place the real "SHIELD" counter (CR 122.1c)
// and never the engine's mixed-case "Shield" regeneration marker. A destroy
// effect then removes the shield counter instead of the permanent
// (ReplaceShieldCounter).
func TestElspethResplendentShieldCounter(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	elspeth := lookup(t, reg, "Elspeth Resplendent")
	bears := lookup(t, reg, "Grizzly Bears")
	murder := lookup(t, reg, "Murder")
	forest := lookup(t, reg, "Forest")
	fill := func(n int) []*cards.Card {
		out := make([]*cards.Card, 0, n)
		for i := 0; i < n; i++ {
			out = append(out, forest)
		}
		return out
	}
	cfg := Config{Seed: 4242, Names: []string{"a", "b"}, Tokens: reg.Tokens,
		Decks: [][]*cards.Card{
			append([]*cards.Card{elspeth, bears, murder, murder}, fill(36)...),
			fill(40),
		}}
	cfg = seatZeroStart(cfg)
	e := New(cfg)
	e.Advance()
	toMain1(t, e)

	elf := moveByName(t, e, 0, "Elspeth Resplendent", state.ZBattlefield)
	if o := e.G.Obj(elf); o == nil || o.Zone != state.ZBattlefield || o.Counter("LOYALTY") != 5 {
		t.Fatalf("precondition: Elspeth zone %v loyalty %d, want battlefield and 5", o.Zone, o.Counter("LOYALTY"))
	}

	// Pin the eligible permanent that the -3 will take on top of the library.
	lib := digReorder(t, e, "Grizzly Bears")
	if len(lib) < 7 {
		t.Fatalf("precondition: library has %d cards, need the 7-card Dig window", len(lib))
	}
	bear := lib[0]
	if got := e.G.Obj(bear).Face().Name; got != "Grizzly Bears" {
		t.Fatalf("precondition: top card is %q, want Grizzly Bears", got)
	}
	if mv := e.G.Obj(bear).Face().ManaValue(); mv > 3 {
		t.Fatalf("precondition: Grizzly Bears mana value %d, want <= 3", mv)
	}

	addMana(t, e, 0, "")
	minus := abilityOption(t, e, elf, 1)
	if minus.Loyalty == nil || minus.Loyalty.Delta != -3 {
		t.Fatalf("[-3] option fact = %+v, want a -3 loyalty delta", minus.Loyalty)
	}
	submitChoices(t, e, minus.Index)

	ask := passUntilNonPriority(t, e, 30)
	if ask == nil || ask.Kind != decision.KChoose || ask.ResumeKind != "dig" {
		t.Fatalf("[-3] did not pose its Dig take ask: %+v", ask)
	}
	pick := -1
	for _, o := range ask.Options {
		if o.Obj == bear {
			pick = o.Index
		}
	}
	if pick < 0 {
		t.Fatalf("[-3] did not offer the pinned Grizzly Bears %d: %+v", bear, ask.Options)
	}
	submitChoices(t, e, pick)
	passUntilStackEmpty(t, e, 30)

	o := e.G.Obj(bear)
	if o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("postcondition: Grizzly Bears zone %v, want the battlefield", o.Zone)
	}
	if got := o.Counter("SHIELD"); got != 1 {
		t.Errorf("shield counters = %d, want 1 (CR 122.1c: the -3 enters it with a shield counter)", got)
	}
	if got := o.Counter("Shield"); got != 0 {
		t.Errorf("internal Shield marker = %d, want 0: `WithCounters$ Shield` must not become the regeneration marker", got)
	}

	// CR 122.1c's destruction half: the counter, not the permanent, is spent.
	murderID := handOrLibraryCard(t, e, 0, "Murder")
	addMana(t, e, 0, "BBC")
	castAt(t, e, murderID, bear)
	o = e.G.Obj(bear)
	if o.Zone != state.ZBattlefield {
		t.Fatalf("Murder destroyed the shielded permanent (zone %v) instead of removing a shield counter", o.Zone)
	}
	if got := o.Counter("SHIELD"); got != 0 {
		t.Errorf("shield counters after the replaced destruction = %d, want 0", got)
	}
	if got := o.Counter("Shield"); got != 0 {
		t.Errorf("internal Shield marker after the replaced destruction = %d, want 0", got)
	}
	replayCheck(t, e, cfg)
}
