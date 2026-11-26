package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestStaticZonesSqueeMutation proves the staticzones pool exercises Squee,
// the Immortal's two-zone self may-play (AffectedZone$ Exile,Graveyard +
// EffectZone$ Exile,Graveyard, the lowered one-static-per-zone grant): a
// kernel whose self StatMayPlayFrom statics grant nothing makes gorge's
// "may cast Squee from your graveyard or from exile" a divergence, so some
// seed diverges.
func TestStaticZonesSqueeMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	n := lockstepRun(t, "staticzones", kstaticzonesDecks, 64, false, func(tbl *kop.Table) {
		dropped := false
		for i := range tbl.Cards {
			if tbl.Cards[i].Name != "Squee, the Immortal" {
				continue
			}
			for j := range tbl.Cards[i].Abilities {
				if a := &tbl.Cards[i].Abilities[j]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatMayPlayFrom {
					a.Static.Kind = kop.StatNone
					dropped = true
				}
			}
		}
		if !dropped {
			t.Fatal("precondition: Squee, the Immortal's table has no StatMayPlayFrom static to drop")
		}
	})
	if n == 0 {
		t.Fatal("no seed diverged with Squee's self may-play statics dropped: the pool does not exercise the two-zone self grant")
	}
}
