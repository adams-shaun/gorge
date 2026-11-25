package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Effect RememberObjects$ RememberedCard with ForgetOnMoved$ Exile (gorge's
// effectRememberedCodes reads RememberedCard as Remembered): Klaw, Master of
// Sound and a creature wearing Dream-Thief's Bandana exile the top card of the
// damaged player's library face down and let their controller play it while
// it stays exiled; Shadow of the Enemy exiles a graveyard's creature cards and
// lets its caster cast them. Doom Blade fills the graveyards.
var remCardMayPlayDecks = [2][]cardset.Entry{
	{
		{Name: "Klaw, Master of Sound", Count: 6}, {Name: "Dream-Thief's Bandana", Count: 4}, {Name: "Shadow of the Enemy", Count: 2},
		{Name: "Walking Corpse", Count: 6}, {Name: "Doom Blade", Count: 3}, {Name: "Vampire Bats", Count: 2},
		{Name: "Swamp", Count: 17},
	},
	{
		{Name: "Klaw, Master of Sound", Count: 5}, {Name: "Dream-Thief's Bandana", Count: 3}, {Name: "Shadow of the Enemy", Count: 3},
		{Name: "Walking Corpse", Count: 7}, {Name: "Doom Blade", Count: 4}, {Name: "Vampire Bats", Count: 1},
		{Name: "Swamp", Count: 17},
	},
}

func TestRememberedCardMayPlayLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	lockstepRun(t, "remcardmayplay", remCardMayPlayDecks, 200, true, nil)
}

// TestRememberedCardMayPlayLockstepMutation proves the pool exercises each
// grant: with the may-play dropped from the face-down Dig carriers, or from
// Shadow of the Enemy, some seed diverges.
func TestRememberedCardMayPlayLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	for _, names := range [][]string{{"Klaw, Master of Sound", "Dream-Thief's Bandana"}, {"Shadow of the Enemy"}} {
		n := lockstepRun(t, "remcardmayplay", remCardMayPlayDecks, 200, false, func(tbl *kop.Table) {
			for _, name := range names {
				if mutateCard(t, tbl, name, func(_ *kop.Ability, in *kop.Instr) {
					if in.Op == kop.OpMayPlay {
						in.Op = kop.OpNop
					}
				}) == 0 {
					t.Fatalf("%s has no MayPlay to drop", name)
				}
			}
		})
		if n == 0 {
			t.Errorf("%v: no seed diverged with the may-play dropped", names)
		}
	}
}
