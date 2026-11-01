package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Breach the Multiverse lockstep: each player mills ten, the
// caster chooses a creature or planeswalker card in each player's graveyard
// (itself first), all of them enter under the caster's control, then each
// creature the caster controls becomes a Phyrexian. The expected answer at
// every decision is gorge's.
var kbreachMatchups = [][2][]cardset.Entry{{
	{
		{Name: "Breach the Multiverse", Count: 5}, {Name: "Sol Ring", Count: 2}, {Name: "Mind Stone", Count: 2},
		{Name: "Llanowar Elves", Count: 2}, {Name: "Phyrexian Rager", Count: 3}, {Name: "Grizzly Bears", Count: 4}, {Name: "Elvish Visionary", Count: 2}, {Name: "Nantuko Husk", Count: 1},
		{Name: "Jace Beleren", Count: 2}, {Name: "Liliana of the Veil", Count: 1},
		{Name: "Swamp", Count: 11}, {Name: "Forest", Count: 5},
	},
	{
		{Name: "Breach the Multiverse", Count: 3}, {Name: "Sol Ring", Count: 2}, {Name: "Mind Stone", Count: 2},
		{Name: "Llanowar Elves", Count: 2}, {Name: "Wall of Omens", Count: 3}, {Name: "Phyrexian Rager", Count: 2},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Elvish Visionary", Count: 2}, {Name: "Prodigal Sorcerer", Count: 1},
		{Name: "Jace Beleren", Count: 1}, {Name: "Liliana of the Veil", Count: 2},
		{Name: "Swamp", Count: 10}, {Name: "Forest", Count: 5}, {Name: "Island", Count: 2},
	},
}}

type kbreachTally struct {
	resolutions, returned, fromOpp, fromOwn int
}

// kbreachCheck counts the gorge side's Breach resolutions (the imprint
// cleared), the cards returned from a graveyard, and those of them that
// changed hands (a card owned by the other player).
func kbreachCheck(tal *kbreachTally) func(*GorgeSide, Side) *Divergence {
	var cur *GorgeSide
	seen := 0
	return func(ref *GorgeSide, _ Side) *Divergence {
		if ref != cur {
			cur, seen = ref, 0
		}
		evs := ref.E.L.Events
		for ; seen < len(evs); seen++ {
			switch ev := evs[seen]; {
			case ev.Kind == events.Imprint && ev.Text == "clear":
				tal.resolutions++
			case ev.Kind == events.MoveZone && ev.From == state.ZGraveyard && ev.To == state.ZBattlefield:
				tal.returned++
			case ev.Kind == events.ControlChange && ev.Text == "GainControl":
				tal.fromOpp++
			}
		}
		return nil
	}
}

func TestBreachLockstep(t *testing.T) {
	t.Parallel()
	var tal kbreachTally
	sonRun(t, "breach", kbreachMatchups, 600, true, nil, kbreachCheck(&tal))
	tal.fromOwn = tal.returned - tal.fromOpp
	t.Logf("600 games: %d Breach resolutions, %d cards returned (%d from an opponent's graveyard, %d from the caster's own)", tal.resolutions, tal.returned, tal.fromOpp, tal.fromOwn)
	if tal.resolutions < 150 || tal.fromOpp < 100 || tal.fromOwn < 100 {
		t.Errorf("resolutions %d (want >= 150), from opponent %d (want >= 100), own %d (want >= 100)", tal.resolutions, tal.fromOpp, tal.fromOwn)
	}
}

// TestBreachLockstepMutation: with the mass return dropped, or the
// Phyrexian grant dropped, some seed diverges.
func TestBreachLockstepMutation(t *testing.T) {
	t.Parallel()
	for _, drop := range []func(kop.Instr) bool{
		func(in kop.Instr) bool { return in.Ext() == kop.ExtBreachReturn },
		func(in kop.Instr) bool { return in.Op == kop.OpOverrideAll },
	} {
		n := sonRun(t, "breach", kbreachMatchups, 100, false, func(tbl *kop.Table) {
			cards := append([]kop.CardDef(nil), tbl.Cards...)
			for i := range cards {
				abs := append([]kop.Ability(nil), cards[i].Abilities...)
				for j := range abs {
					p := append([]kop.Instr(nil), abs[j].Prog...)
					for x := range p {
						if drop(p[x]) {
							p[x] = kop.Instr{Op: kop.OpNop}
						}
					}
					abs[j].Prog = p
				}
				cards[i].Abilities = abs
			}
			tbl.Cards = cards
		}, nil)
		if n == 0 {
			t.Fatal("no seed diverged with an instruction dropped")
		}
	}
}
