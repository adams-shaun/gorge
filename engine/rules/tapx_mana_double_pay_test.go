package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// cr-g92: Volrath's Gardens' "{2}, Tap an untapped creature you control"
// beside Llanowar Elves as the only green source and one other untapped
// creature. The payment planner witnesses the {2} by tapping the Elves and a
// colourless rock; the tap-cost ask runs first and (like Forge's
// CostTapType) still offers the Elves. Choosing them must not let one
// creature pay both the tap cost and the mana: the plan is revalidated
// against the election, falls back to the manual window (which withholds the
// elected creature), and the activation reverses under CR 733.1 because the
// rock alone cannot pay {2}.

const tapxManaRock = "Name:Mana Rock\nTypes:Artifact\n" +
	"A:AB$ Mana | Cost$ T | Produced$ C | SpellDescription$ Add {C}.\nOracle:x\n"

func tapxBoard(t *testing.T, seed uint64) (e *Engine, gardens, elf, bear, rock state.ObjID) {
	t.Helper()
	e, _, ids := manaTapBoard(t, seed, "Volrath's Gardens", "Llanowar Elves", "Grizzly Bears")
	gardens, elf, bear = ids["Volrath's Gardens"], ids["Llanowar Elves"], ids["Grizzly Bears"]
	rock = onBoard(t, e, 0, tapxManaRock)
	e.pending = nil
	e.priorityRound()
	edrSeatZeroPriority(t, e)
	if e.G.Obj(elf).Tapped || e.G.Obj(bear).Tapped || e.G.Obj(rock).Tapped {
		t.Fatal("fixture: every permanent must start untapped")
	}
	return e, gardens, elf, bear, rock
}

func tapxGardensOption(t *testing.T, e *Engine, gardens state.ObjID) int {
	t.Helper()
	for _, o := range e.Pending().Options {
		if o.Kind == "ability" && o.Obj == gardens {
			return o.Index
		}
	}
	t.Fatalf("Volrath's Gardens not offered: %+v", e.Pending().Options)
	return -1
}

func tapxOption(t *testing.T, d *decision.Decision, kind string, obj state.ObjID) int {
	t.Helper()
	if d == nil {
		t.Fatalf("no decision pending while looking for %q %d", kind, obj)
	}
	for _, o := range d.Options {
		if o.Kind == kind && o.Obj == obj {
			return o.Index
		}
	}
	t.Fatalf("no %q option for %d in %+v", kind, obj, d.Options)
	return -1
}

func TestTapXCostNotDoubleUsedForMana(t *testing.T) {
	t.Parallel()

	t.Run("choosing_the_only_green_source_reverses", func(t *testing.T) {
		t.Parallel()
		e, gardens, elf, bear, rock := tapxBoard(t, 9911)
		life := e.G.Players[0].Life
		submitChoices(t, e, tapxGardensOption(t, e, gardens))
		d := e.Pending()
		if d == nil || d.Kind != decision.KChoose || d.Min != 1 || d.Max != 1 {
			t.Fatalf("tap election = %+v, want KChoose Min=Max=1", d)
		}
		// Forge's CostTapType offers every untapped creature; the elected one
		// is then required to actually tap, so a mana plan that named it is
		// no longer payable. Pin the offer so this test cannot pass by
		// silently filtering the Elves out of the ask.
		elfChoice := tapxOption(t, d, "tapcost", elf)
		tapxOption(t, d, "tapcost", bear)

		submitChoices(t, e, elfChoice)
		d = e.Pending()
		if d == nil || d.Kind != decision.KChoose || d.PaymentFallback == nil ||
			d.PaymentFallback.Reason != paymentFallbackSourceChanged {
			t.Fatalf("pending after electing the planned Elves = %+v, want the manual window with source_changed", d)
		}
		// The elected creature is committed to the tap cost, so the manual
		// window must not offer it for mana as well.
		for _, o := range d.Options {
			if o.Kind == "activate" && o.Obj == elf {
				t.Fatalf("manual window re-offered the elected Elves for mana: %+v", d.Options)
			}
		}
		// Tap the rock too: even every remaining source cannot pay {2}, so
		// the activation is genuinely unpayable and reverses (CR 733.1).
		submitChoices(t, e, tapxOption(t, d, "activate", rock))
		d = e.Pending()
		submitChoices(t, e, tapxOption(t, d, "done", 0))

		if len(e.G.Stack) != 0 {
			t.Fatalf("activation stayed on the stack: %d objects", len(e.G.Stack))
		}
		if got := e.G.Players[0].Life; got != life {
			t.Fatalf("life %d -> %d: the reversed activation gained life", life, got)
		}
		if e.G.Obj(gardens).Tapped || e.G.Obj(elf).Tapped || e.G.Obj(bear).Tapped {
			t.Fatalf("reversed activation left a cost paid: gardens=%v elf=%v bear=%v",
				e.G.Obj(gardens).Tapped, e.G.Obj(elf).Tapped, e.G.Obj(bear).Tapped)
		}
	})

	t.Run("other_creature_pays_the_cost", func(t *testing.T) {
		t.Parallel()
		e, gardens, elf, bear, rock := tapxBoard(t, 9912)
		life := e.G.Players[0].Life
		submitChoices(t, e, tapxGardensOption(t, e, gardens))
		d := e.Pending()
		submitChoices(t, e, tapxOption(t, d, "tapcost", bear))
		passUntilStackEmpty(t, e, 20)

		if got := e.G.Players[0].Life; got != life+2 {
			t.Fatalf("life %d -> %d, want +2 after the ability resolves", life, got)
		}
		if !e.G.Obj(bear).Tapped {
			t.Error("the elected Grizzly Bears was not tapped for the cost")
		}
		if !e.G.Obj(elf).Tapped || !e.G.Obj(rock).Tapped {
			t.Errorf("the witness did not tap the Elves and rock: elf=%v rock=%v",
				e.G.Obj(elf).Tapped, e.G.Obj(rock).Tapped)
		}
	})
}
