package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The granted-lure lockstep (CR 509.1c): "All creatures able to block
// CARDNAME do so." given by a Pump KW$ until end of turn (Alluring Scent,
// Bloodscent, Roar of Challenge, Declare Dominance, Shinen's channel) and by
// a Continuous static (Lure and Seton's Desire on the enchanted creature,
// Gift of the Deity's green half), against a defender with ground creatures.
// The expected answer at every decision is gorge's.
var klureGrantDecks = [2][]cardset.Entry{
	{
		{Name: "Lure", Count: 2}, {Name: "Gift of the Deity", Count: 2}, {Name: "Seton's Desire", Count: 2},
		{Name: "Alluring Scent", Count: 2}, {Name: "Bloodscent", Count: 2}, {Name: "Roar of Challenge", Count: 2},
		{Name: "Declare Dominance", Count: 1}, {Name: "Shinen of Life's Roar", Count: 2},
		{Name: "Grizzly Bears", Count: 7}, {Name: "Llanowar Elves", Count: 2}, {Name: "Forest", Count: 16},
	},
	{
		{Name: "Grizzly Bears", Count: 6}, {Name: "Hill Giant", Count: 5}, {Name: "Raging Goblin", Count: 5},
		{Name: "Giant Growth", Count: 3}, {Name: "Prized Unicorn", Count: 2}, {Name: "Mountain", Count: 9}, {Name: "Forest", Count: 10},
	},
}

// klureGrantDecksB: the other carriers of the class (Mortipede's own pump, Nemesis
// Mask on an Equipment, Noble Quarry's bestowed static, Revenge of the
// Hunted's miracle, Taunting Arbormage's kicked trigger, Taunting Challenge).
var klureGrantDecksB = [2][]cardset.Entry{
	{
		{Name: "Mortipede", Count: 3}, {Name: "Nemesis Mask", Count: 2}, {Name: "Noble Quarry", Count: 3},
		{Name: "Revenge of the Hunted", Count: 2}, {Name: "Taunting Arbormage", Count: 3}, {Name: "Taunting Challenge", Count: 2},
		{Name: "Grizzly Bears", Count: 5}, {Name: "Llanowar Elves", Count: 2}, {Name: "Forest", Count: 14}, {Name: "Swamp", Count: 4},
	},
	klureGrantDecks[1],
}

func TestLureGrantLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "luregrant", klureGrantDecks, 800, "KLUREGRANT_SEEDS")
	lockstepSet(t, "luregrant", klureGrantDecksB, 800, "KLUREGRANT_SEEDS")
}

// TestLureGrantLockstepMutation: with the grants dropped (the StatLure statics
// and the Pump marks), some seed diverges, so the pool exercises them.
func TestLureGrantLockstepMutation(t *testing.T) {
	t.Parallel()
	dropStatics := func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := make([]kop.Ability, 0, len(cards[i].Abilities))
			for _, a := range cards[i].Abilities {
				if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatLure {
					a.Static.Kind = kop.StatNone
				}
				abs = append(abs, a)
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
	}
	for i, decks := range [][2][]cardset.Entry{klureGrantDecks, klureGrantDecksB} {
		if n := lockstepRun(t, "luregrant", decks, 300, false, dropStatics); n == 0 {
			t.Errorf("deck pair %d: no seed diverged with the lure statics dropped", i)
		}
	}
}

// TestLureGrantPumpMutation: with the Pump marks dropped, some seed diverges.
func TestLureGrantPumpMutation(t *testing.T) {
	t.Parallel()
	dropMarks := func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := make([]kop.Ability, len(cards[i].Abilities))
			copy(abs, cards[i].Abilities)
			for j := range abs {
				prog := make([]kop.Instr, 0, len(abs[j].Prog))
				for _, in := range abs[j].Prog {
					if in.Op == kop.OpExt && in.Ext() == kop.ExtMustBeBlocked && in.C == kop.MustBeBlockedLure {
						continue
					}
					prog = append(prog, in)
				}
				abs[j].Prog = prog
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
	}
	for i, decks := range [][2][]cardset.Entry{klureGrantDecks, klureGrantDecksB} {
		if n := lockstepRun(t, "luregrant", decks, 300, false, dropMarks); n == 0 {
			t.Errorf("deck pair %d: no seed diverged with the lure marks dropped", i)
		}
	}
}
