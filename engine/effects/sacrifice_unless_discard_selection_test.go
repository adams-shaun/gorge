package effects_test

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// payUnlessDiscardSelection submits the pay branch of the staged unless ask
// and returns the decision still pending afterwards.
func payUnlessDiscardSelection(t *testing.T, spec string, hand int) (permanent state.ObjID, handIDs []state.ObjID, e interface {
	Pending() *decision.Decision
}, draws func() uint64, zone func(state.ObjID) state.Zone) {
	t.Helper()
	src := strings.Replace(sacrificeUnlessDiscardSrc, "Discard<1/Card>", spec, 1)
	if src == sacrificeUnlessDiscardSrc {
		t.Fatalf("precondition: fixture has no Discard<1/Card> to replace")
	}
	eng, perm, ids := sacrificeUnlessDiscardGameSrc(t, src, hand)
	ask := drainUntilUnlessPay(t, eng)
	pay, ok := unlessModeOption(ask, decision.ModeUnlessPay)
	if !ok {
		t.Fatalf("no pay option for %s with %d cards in hand: %+v", spec, hand, ask.Options)
	}
	before := eng.RNGDraws()
	if err := eng.Submit(decision.Intent{Seq: ask.Seq, Player: ask.Player, Choices: []int{pay}}); err != nil {
		t.Fatalf("submit pay: %v", err)
	}
	return perm, ids, eng, func() uint64 { return eng.RNGDraws() - before },
		func(id state.ObjID) state.Zone { return eng.G.Obj(id).Zone }
}

// TestSacrificeUnlessRandomDiscardPays: Discard<1/Random> (Balduvian Horde)
// offers the pay branch with cards in hand, the engine's RNG (not the payer)
// picks exactly one, and the permanent stays.
func TestSacrificeUnlessRandomDiscardPays(t *testing.T) {
	perm, hand, e, draws, zone := payUnlessDiscardSelection(t, "Discard<1/Random>", 3)
	if d := e.Pending(); d != nil && d.Kind == decision.KChoose {
		t.Fatalf("a Random discard posed a payer choice: %+v", d)
	}
	gone := 0
	for _, id := range hand {
		if zone(id) == state.ZGraveyard {
			gone++
		}
	}
	if gone != 1 {
		t.Fatalf("discarded %d of 3 hand cards, want exactly 1", gone)
	}
	if got := draws(); got != 1 {
		t.Fatalf("RNG draws = %d, want 1", got)
	}
	if z := zone(perm); z != state.ZBattlefield {
		t.Fatalf("permanent zone = %v, want battlefield after paying", z)
	}
}

// TestSacrificeUnlessHandDiscardPays: Discard<1/Hand> (Perplex, Miss
// Highwater) discards the whole hand without a choice, and an empty hand
// still pays by discarding nothing.
func TestSacrificeUnlessHandDiscardPays(t *testing.T) {
	t.Run("whole hand", func(t *testing.T) {
		perm, hand, e, _, zone := payUnlessDiscardSelection(t, "Discard<1/Hand>", 3)
		if d := e.Pending(); d != nil && d.Kind == decision.KChoose {
			t.Fatalf("a Hand discard posed a payer choice: %+v", d)
		}
		for _, id := range hand {
			if z := zone(id); z != state.ZGraveyard {
				t.Fatalf("hand card zone = %v, want graveyard", z)
			}
		}
		if z := zone(perm); z != state.ZBattlefield {
			t.Fatalf("permanent zone = %v, want battlefield after paying", z)
		}
	})
	t.Run("empty hand pays with nothing", func(t *testing.T) {
		perm, hand, _, _, zone := payUnlessDiscardSelection(t, "Discard<1/Hand>", 0)
		if len(hand) != 0 {
			t.Fatalf("precondition: hand = %d, want 0", len(hand))
		}
		if z := zone(perm); z != state.ZBattlefield {
			t.Fatalf("permanent zone = %v, want battlefield after paying", z)
		}
	})
}
