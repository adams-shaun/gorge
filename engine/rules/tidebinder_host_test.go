package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Oracle: only the countered ability's SOURCE loses all abilities. The
// Tidebinder whose trigger did the countering (the host) keeps Flash.
func TestTidebinderCountersOtherTidebinderTrigger(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	tbCard := lookup(t, reg, "Tishana's Tidebinder")
	cfg := Config{Seed: 7, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{
			append([]*cards.Card{tbCard, tbCard}, mountainDeck(t, 38)...),
			mountainDeck(t, 40),
		},
		Tokens: reg.Tokens,
	}
	e := New(seatZeroStart(cfg))
	e.Advance()
	first := findAndMoveToHand(t, e, 0, "Tishana's Tidebinder")
	var second state.ObjID
	for _, id := range e.G.Zone(state.ZLibrary, 0) {
		if id != first && e.G.Obj(id).Face().Name == "Tishana's Tidebinder" {
			second = id
			break
		}
	}
	if second == 0 {
		t.Fatal("need two distinct Tidebinders")
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: second, From: state.ZLibrary, To: state.ZHand})
	e.pending = nil
	e.Advance()
	driveToStep(t, e, 3, 0, state.StepMain1)
	addMana(t, e, 0, "UUUUUU")
	submitChoices(t, e, castOptionIdx(e.Pending(), "cast", first))
	flashed := false
	for i := 0; i < 40; i++ {
		d := e.Pending()
		if d == nil {
			e.Advance()
			continue
		}
		if d.Kind == decision.KPriority {
			if len(e.G.Stack) == 0 {
				break
			}
			// The first Tidebinder's ETB trigger is on the stack: flash in the
			// second one in response.
			if !flashed && e.G.Obj(first).Zone == state.ZBattlefield {
				if idx := castOptionIdx(d, "cast", second); idx >= 0 {
					flashed = true
					submitChoices(t, e, idx)
					continue
				}
			}
			for _, o := range d.Options {
				if o.Kind == "pass" {
					submitChoices(t, e, o.Index)
					break
				}
			}
			continue
		}
		// Targets: prefer the other Tidebinder's trigger (an ability on the stack).
		pick := 0
		for _, o := range d.Options {
			if o.Obj != 0 && e.G.Obj(o.Obj) != nil && e.G.Obj(o.Obj).Source == first {
				pick = o.Index
			}
		}
		submitChoices(t, e, pick)
	}
	if !flashed {
		t.Fatal("never flashed the second Tidebinder in")
	}
	if z := e.G.Obj(second).Zone; z != state.ZBattlefield {
		t.Fatalf("second Tidebinder zone = %v, want battlefield", z)
	}
	hasFlash := func(id state.ObjID) bool { return slices.Contains(e.Derived(id).Keywords, "Flash") }
	if !hasFlash(second) {
		t.Errorf("the host Tidebinder lost Flash; only the countered ability's source should lose abilities")
	}
	if hasFlash(first) {
		t.Errorf("the countered ability's source Tidebinder kept Flash; it should lose all abilities")
	}
}
