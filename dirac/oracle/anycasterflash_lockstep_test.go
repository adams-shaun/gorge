package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Caster$ Player CastWithFlash (Quick Sliver, Vernal Equinox, Tidal Barracuda):
// the grant reaches every player, so an opponent of the static's controller
// may cast the matching spell off sorcery timing too. Four seats, gorge
// leading; no divergence and no kernel stop.
func TestFourSeatAnyCasterFlashGrantLockstep(t *testing.T) {
	grants := pod4Deck(deckOf(3, "Quick Sliver"), deckOf(3, "Vernal Equinox"), deckOf(3, "Tidal Barracuda"))
	body := pod4Deck(deckOf(4, "Muscle Sliver"), deckOf(4, "Gemhide Sliver"), deckOf(3, "Mulch"), deckOf(3, "Accumulate Wisdom"),
		deckOf(3, "Cloud Sprite"), deckOf(3, "Elite Vanguard"), deckOf(2, "Giant Growth"), deckOf(2, "Cancel"))
	lands := pod4Deck(deckOf(5, "Plains"), deckOf(5, "Island"), deckOf(5, "Forest"))
	a := pod4Deck(grants, body, lands)
	b := pod4Deck(deckOf(3, "Quick Sliver"), body, deckOf(6, "Grizzly Bears"), lands)
	if len(a) < 40 {
		a = append(a, deckOf(40-len(a), "Grizzly Bears")...)
	}
	if len(b) < 40 {
		b = append(b, deckOf(40-len(b), "Grizzly Bears")...)
	}
	a, b = a[:40], b[:40]
	if n := pod4FixtureLockstep(t, "anycasterflash", [4][]string{a, b, a, b}, 30, true, nil); n != 0 {
		t.Fatalf("%d diverged", n)
	}
}

// TestFourSeatAnyCasterFlashGrantMutation proves the pool reaches the grant:
// with every grant narrowed back to its controller (the flag cleared), some
// four-seat game diverges from gorge.
func TestFourSeatAnyCasterFlashGrantMutation(t *testing.T) {
	grants := pod4Deck(deckOf(4, "Quick Sliver"), deckOf(4, "Vernal Equinox"), deckOf(4, "Tidal Barracuda"))
	rest := pod4Deck(deckOf(4, "Muscle Sliver"), deckOf(4, "Gemhide Sliver"), deckOf(3, "Cloud Sprite"), deckOf(3, "Elite Vanguard"),
		deckOf(3, "Giant Growth"), deckOf(3, "Cancel"), deckOf(6, "Plains"), deckOf(6, "Island"))
	a := pod4Deck(grants, rest)[:40]
	n := pod4FixtureLockstep(t, "anycasterflash", [4][]string{a, a, a, a}, 40, false, func(tbl *kop.Table) {
		for i := range tbl.Cards {
			for j := range tbl.Cards[i].Abilities {
				if st := &tbl.Cards[i].Abilities[j].Static; st.Kind == kop.StatCastWithFlash {
					st.StFlags &^= kop.StFlashAnyCaster
				}
			}
		}
	})
	if n == 0 {
		t.Fatal("clearing the any-caster flag changed no game: the pool does not reach the grant")
	}
}
