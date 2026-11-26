package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// A printed activated ability's announced ExileFromGrave<X/Spec> in lockstep
// against gorge (CR 601.2b, 107.3a): alone, with no printed {X} (Taigam,
// Sidisi's Hand's {B}, {T}; Painbringer's {T}; Winter, Cursed Rider's
// exhaust), where gorge's xAsk poses the X bounded by the matching graveyard
// cards, and beside a printed {X} (Necropolis Fiend), where those cards cap
// the mana bound (kernel cost_exilex.go). The expected answer at every
// decision is gorge's.
const exileXActPool = "exilexact"

var exileXActDecks = [2][]cardset.Entry{
	{
		{Name: "Taigam, Sidisi's Hand", Count: 4}, {Name: "Painbringer", Count: 4}, {Name: "Winter, Cursed Rider", Count: 3},
		{Name: "Necropolis Fiend", Count: 2}, {Name: "Ornithopter", Count: 6}, {Name: "Scathe Zombies", Count: 5},
		{Name: "Island", Count: 8}, {Name: "Swamp", Count: 8},
	},
	{
		{Name: "Painbringer", Count: 4}, {Name: "Taigam, Sidisi's Hand", Count: 2}, {Name: "Ornithopter", Count: 4},
		{Name: "Grizzly Bears", Count: 6}, {Name: "Hill Giant", Count: 4},
		{Name: "Swamp", Count: 10}, {Name: "Island", Count: 4}, {Name: "Forest", Count: 6},
	},
}

// The 200 seeds play as two tests of 100 so each fits the 60 s test budget.
func TestExileXActLowers(t *testing.T) {
	t.Parallel()
	lockstepSetRange(t, exileXActPool, exileXActDecks, 1, 100)
}

func TestExileXActLowersHigh(t *testing.T) {
	t.Parallel()
	lockstepSetRange(t, exileXActPool, exileXActDecks, 101, 200)
}

// TestExileXActDecksPlayable names a refused carrier here, not only by a
// stall far into the games.
func TestExileXActDecksPlayable(t *testing.T) {
	t.Parallel()
	_, set, gd := lockstepPool(t, exileXActPool, exileXActDecks)
	for s, d := range gd {
		if !SetDeckSupported(set, d) {
			t.Errorf("seat %d: deck not playable in the %s pool", s, exileXActPool)
		}
	}
}

// TestExileXActMutation: with the activated abilities' exile part dropped,
// or the announced-only part's X ask offering X = 0 alone, some seed diverges, so the pool plays the
// announced exile of an activated ability.
// It is not parallel: it sets the kernel's DebugExileXAnnouncedOnly.
func TestExileXActMutation(t *testing.T) {
	n := lockstepRun(t, exileXActPool, exileXActDecks, 60, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				if abs[j].Kind != kop.AbSpell && abs[j].Cost.Flags&kop.CostExileGrave != 0 && abs[j].Cost.N == 0 {
					abs[j].Cost.Flags &^= kop.CostExileGrave
					abs[j].Cost.Filter = 0
				}
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
	})
	if n == 0 {
		t.Error("no exile part: no seed diverged: the pool never pays an activated ability's announced exile")
	}
	dirac.DebugExileXAnnouncedOnly = true
	defer func() { dirac.DebugExileXAnnouncedOnly = false }()
	if lockstepRun(t, exileXActPool, exileXActDecks, 60, false, nil) == 0 {
		t.Error("announced only: no seed diverged: the pool never announces X above 0 with no printed {X}")
	}
}
