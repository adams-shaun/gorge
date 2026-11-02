package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// A permanent that adds mana whenever its controller taps for mana
// (Mode$ TapsForMana | Static$ True, one literal symbol) lets a source pay more
// than it makes: gorge folds the extra into the planned tap, so the kernel's
// availability, plan and tap must count it the same, including the cast that a
// single land only pays with the extra. One deck per carrier kind: any permanent
// (Groundchuck & Dirtbag), a creature (Badgermole Cub) and a tap that declares
// {C} (Forsaken Monument, beside a source that makes {C}).
var ktapextraDecks = map[string][2][]cardset.Entry{
	"any": {
		{
			{Name: "Groundchuck & Dirtbag", Count: 4}, {Name: "Grizzly Bears", Count: 10}, {Name: "Llanowar Elves", Count: 4},
			{Name: "Forest", Count: 14}, {Name: "Plains", Count: 8},
		},
		{{Name: "Grizzly Bears", Count: 10}, {Name: "Shock", Count: 10}, {Name: "Mountain", Count: 10}, {Name: "Forest", Count: 10}},
	},
	"creature": {
		{
			{Name: "Badgermole Cub", Count: 4}, {Name: "Llanowar Elves", Count: 6}, {Name: "Grizzly Bears", Count: 8},
			{Name: "Forest", Count: 14}, {Name: "Plains", Count: 8},
		},
		{{Name: "Grizzly Bears", Count: 10}, {Name: "Shock", Count: 10}, {Name: "Mountain", Count: 10}, {Name: "Forest", Count: 10}},
	},
	"colourless": {
		{
			{Name: "Forsaken Monument", Count: 4}, {Name: "Sol Ring", Count: 4}, {Name: "Mind Stone", Count: 4},
			{Name: "Hill Giant", Count: 8}, {Name: "Grizzly Bears", Count: 6}, {Name: "Forest", Count: 8}, {Name: "Mountain", Count: 6},
		},
		{{Name: "Grizzly Bears", Count: 10}, {Name: "Shock", Count: 10}, {Name: "Mountain", Count: 10}, {Name: "Forest", Count: 10}},
	},
}

func TestTapExtraLockstep(t *testing.T) {
	t.Parallel()
	for name, decks := range ktapextraDecks {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			lockstepSet(t, "tapextra", decks, 300, "KTAPEXTRA_SEEDS")
		})
	}
}

// TestTapExtraLockstepMutation: with the carrier's static dropped from the
// kernel's table, some seed diverges, so each pool does exercise the extra.
func TestTapExtraLockstepMutation(t *testing.T) {
	t.Parallel()
	for name, decks := range ktapextraDecks {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			n := lockstepRun(t, "tapextra", decks, 200, false, func(tbl *kop.Table) {
				cards := append([]kop.CardDef(nil), tbl.Cards...)
				for i := range cards {
					var keep []kop.Ability
					for _, a := range cards[i].Abilities {
						if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatTapExtra {
							continue
						}
						keep = append(keep, a)
					}
					cards[i].Abilities = keep
				}
				tbl.Cards = cards
			})
			if n == 0 {
				t.Fatal("no seed diverged with the tap-extra static dropped")
			}
		})
	}
}

// A source whose tap brings extra mana (an earthbent land beside Badgermole Cub) makes
// gorge's window census incomplete (CR 605.1b), so a waterbend announcement may close
// without a tap.
func TestTapExtraWaterbendCensusLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "waterbend_done_with_tap_extra", "std-tla", Config{})
}
