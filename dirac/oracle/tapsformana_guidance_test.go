package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Elvish Guidance's count-scaled tap extra played decision by decision against
// gorge (dirac/kgen lower_tapextra.go tapExtraDynamic): whenever the enchanted
// land is tapped for mana, its controller adds one {G} for each Elf on the
// battlefield. gorge's planner never folds a dynamic amount, so it defers the
// enchanted land and the extra floats at the tap; the kernel mirrors it with a
// StTapDynamic TapExtra (dirac mana_tapextra.go addGatedExtra) beside a
// class-only Taps line that defers the source.
//
// TestTapsForManaGuidanceLockstep plays the tapsformana pool deck; the
// mutation drops the card's StTapDynamic static from the kernel's table and
// requires a divergence, so the pool exercises the card rather than
// passing on games that never enchant a land.
func guidanceDecks() [2][]cardset.Entry {
	return [2][]cardset.Entry{
		{
			{Name: "Elvish Guidance", Count: 4},
			{Name: "Llanowar Elves", Count: 8},
			{Name: "Grizzly Bears", Count: 4},
			{Name: "Forest", Count: 24},
		},
		{
			{Name: "Llanowar Elves", Count: 8},
			{Name: "Grizzly Bears", Count: 8},
			{Name: "Forest", Count: 24},
		},
	}
}

func TestTapsForManaGuidanceLockstep(t *testing.T) {
	t.Parallel()
	if n := lockstepRun(t, "tapsformana", guidanceDecks(), 80, false, nil); n != 0 {
		t.Fatalf("%d divergent seeds", n)
	}
}

func TestTapsForManaGuidanceMutation(t *testing.T) {
	t.Parallel()
	dropped := 0
	n := lockstepRun(t, "tapsformana", guidanceDecks(), 80, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			var keep []kop.Ability
			for _, a := range cards[i].Abilities {
				if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatTapExtra &&
					a.Static.StFlags&kop.StTapDynamic != 0 {
					dropped++
					continue
				}
				keep = append(keep, a)
			}
			cards[i].Abilities = keep
		}
		tbl.Cards = cards
	})
	if dropped == 0 {
		t.Fatal("the kernel's table carries no StTapDynamic tap-extra static to drop")
	}
	if n == 0 {
		t.Fatal("no seed diverged with Elvish Guidance's count-scaled tap-extra static dropped from the kernel's table: the pool does not exercise the card")
	}
}
