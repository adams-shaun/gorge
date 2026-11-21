package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// CR 601.2g-h: a spell's payment may activate a mana ability whose cost
// carries generic mana (Surveillance Room's "{1}, {T}: Add one mana of any
// color"). The kernel models that shape as a filter (filterTapMana) and
// gorge's planner as a unit alternative; on a clean board the kernel's
// script route already prices the flash cast it funds (the cast is offered
// only when the fee's {1} is payable from the other sources: case C). The
// recorded std-msh game that was still missed at this test's HEAD (gorge
// offered cast(Captain Marvel), the kernel did not, at a priority decision)
// was resolved by the origin/main merge before it was fixed here: the
// committed repro row cast_census_drops_a_generic_cost_second_mana_ability
// (pool std-msh) replays it clean. The miss was in the route's engagement,
// not in this census.
func TestCastCensusCountsAFeeManaAbility(t *testing.T) {
	// A: the Citadel's {W} and the Room's {1},{T} {W} pip, four generic from
	// the three Islands and the Hideout's {C}: the cast is affordable.
	// B: without the Room there is one {W} unit: no cast.
	// C: the Room without the Hideout: the fee's {1} has no fourth generic:
	//    no cast (the fee is real, not a free unit).
	cases := []struct {
		name    string
		islands int
		room    bool
		hideout bool
		want    bool
	}{
		{"room_and_hideout", 3, true, true, true},
		{"no_room", 3, false, true, false},
		{"room_without_hideout", 3, true, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			set := cardsettest.Synth(t, "std-msh")
			deck := []cardset.Entry{
				{Name: "Captain Marvel, Earth's Protector", Count: 1},
				{Name: "Asgardian Citadel", Count: 1},
				{Name: "Island", Count: tc.islands},
			}
			if tc.room {
				deck = append(deck, cardset.Entry{Name: "Surveillance Room", Count: 1})
			}
			if tc.hideout {
				deck = append(deck, cardset.Entry{Name: "Villainous Hideout", Count: 1})
			}
			tbl, da, _, err := set.Matchup(deck, deck)
			if err != nil {
				t.Fatal(err)
			}
			id := map[string]CardID{}
			for _, c := range da {
				id[tbl.Cards[c].Name] = c
			}
			sc := newScenTableLib(t, tbl, id["Island"], Config{NoElide: true})
			asc := sc.perm(id["Asgardian Citadel"], 0)
			sc.s.perms[sc.slot(asc)].flags &^= pfTapped // it enters tapped; the game's untap step cleared it
			if tc.room {
				sc.perm(id["Surveillance Room"], 0)
			}
			for i := 0; i < tc.islands; i++ {
				sc.perm(id["Island"], 0)
			}
			if tc.hideout {
				sc.perm(id["Villainous Hideout"], 0)
			}
			cm := sc.hand(id["Captain Marvel, Earth's Protector"], 0)
			sc.s.step = kop.StepUpkeep // flash casts outside a main phase (CR 702.49a)
			d := sc.s.Step()
			if got := castOffered(&d, cm); got != tc.want {
				t.Errorf("cast offered=%v, want %v", got, tc.want)
			}
		})
	}
}
