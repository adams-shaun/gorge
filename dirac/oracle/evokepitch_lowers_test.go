package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The hand-exile evoke in lockstep against gorge (CR 702.74a, 118.9): the MH2
// Incarnations' "Evoke--Exile a <colour> card from your hand"
// (K:Evoke:ExileFromHand<1/Card.<Colour>+Other/...>), which kgen lowers as
// the evoked cast's hand-exile pick (lower_evoke.go evokePickCost, kernel
// CardDef.EvokePick: the offer is gated on another card of the colour in
// hand, and the cast asks which one to exile; the permanent is sacrificed
// as it enters like any evoked one). Seat 0 holds Solitude and Endurance
// with white and green fodder, seat 1 Fury and Grief with red and black. The
// expected answer at every decision is gorge's. (Subtlety, the blue one, is
// beside the pool: its ChangeZone Origin$ Stack body still refuses.)
const evokePitchPool = "evokepitch"

var evokePitchDecks = [2][]cardset.Entry{
	{
		{Name: "Solitude", Count: 4}, {Name: "Endurance", Count: 4},
		{Name: "Savannah Lions", Count: 6}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Plains", Count: 10}, {Name: "Forest", Count: 10},
	},
	{
		{Name: "Fury", Count: 4}, {Name: "Grief", Count: 4},
		{Name: "Raging Goblin", Count: 5}, {Name: "Scathe Zombies", Count: 5}, {Name: "Hill Giant", Count: 2},
		{Name: "Mountain", Count: 10}, {Name: "Swamp", Count: 10},
	},
}

func TestEvokePitchLowers(t *testing.T) {
	t.Parallel()
	lockstepSet(t, evokePitchPool, evokePitchDecks, 200, "KEVOKEPITCH_SEEDS")
}

// TestEvokePitchDecksPlayable names a refused Incarnation here, not only by a
// stall far into the games.
func TestEvokePitchDecksPlayable(t *testing.T) {
	t.Parallel()
	_, set, gd := lockstepPool(t, evokePitchPool, evokePitchDecks)
	for s, d := range gd {
		if !SetDeckSupported(set, d) {
			t.Errorf("seat %d: deck not playable in the %s pool", s, evokePitchPool)
		}
	}
}

// TestEvokePitchMutation: with the hand-exile pick dropped (the evoked cast
// free, offered with no fodder and asking no exile), or the evoke dropped,
// some seed diverges, so the pool plays the pitch-evoke cast.
func TestEvokePitchMutation(t *testing.T) {
	t.Parallel()
	for _, m := range []struct {
		name string
		fn   func(*kop.CardDef)
	}{
		{"no pick", func(d *kop.CardDef) { d.EvokePick = kop.Cost{} }},
		{"no evoke", func(d *kop.CardDef) { d.HasEvoke, d.EvokePick = false, kop.Cost{} }},
	} {
		n := lockstepRun(t, evokePitchPool, evokePitchDecks, 120, false, func(tbl *kop.Table) {
			cards := append([]kop.CardDef(nil), tbl.Cards...)
			for i := range cards {
				if cards[i].EvokePick.Flags != 0 {
					m.fn(&cards[i])
				}
			}
			tbl.Cards = cards
		})
		if n == 0 {
			t.Errorf("%s: no seed diverged: the pool never plays a pitch-evoke cast", m.name)
		}
	}
}
