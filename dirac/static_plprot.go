package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The player protection-from-everything grant (kop.StatPlayerProtAll, The
// One Ring's and The Stasis Coffin's "you gain protection from everything
// until your next turn"). gorge registers it as a continuous Player.You
// keyword static (registerPlayerPumpGrant) that rules' playerKeywords
// gates read: playerProtectedFrom (CR 702.16e damage, CR 702.16b targeting)
// with sourceHasQuality("everything") matching every source. The kernel
// lowers the same grant to a runtime emblem of the resolving controller
// carrying StatPlayerProtAll (kgen lower_effect.go pumpPlayerGrant), the
// instruction a Duration$ UntilYourNextTurn Effect static lowers to; the
// emblem's UntilNextTurn ends it at the cleanup before the controller's
// next turn (dropNextTurnEmblems), gorge's UntilTurn = next - 1 boundary,
// and it outlives a source that left the battlefield exactly as gorge's
// UntilTurn effect does (rules' layers_effect.go active(): a
// SourceIncarnation-less UntilTurn effect is dropped only at its boundary).
//
// The readers sit behind genPrep.anyPlProtAll (prepPlProtAll: a table
// emblem statics scan; no printed card carries the kind -- the emblem is
// created at resolution), so a table without it pays nothing. Two grants
// are two emblems and two statics, gorge's two continuous effects; the
// event they answer is one, so one prevention, as gorge's emit swallows it
// once.
//
// Not modelled, the same approximation plProtected names
// (static_playerstats.go): gorge's prevention needs a source at all
// (emit.go `src != 0`), a damage event whose source id is zero; every
// kernel damage call names a live source controller, and the pool's games
// never hit the shape.

// DebugPlProtAllOff is TestPlayerProtAllMutation's switch: the static is
// ignored, so the lockstep against gorge must diverge on the first damage
// event to the protected player. Read in the two readers only.
var DebugPlProtAllOff bool

// prepPlProtAll sets genPrep.anyPlProtAll when the table carries a
// StatPlayerProtAll static.
func prepPlProtAll(t *kop.Table, g *genPrep) {
	for i := range t.Emblems {
		for j := range t.Emblems[i].Abilities {
			if a := &t.Emblems[i].Abilities[j]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatPlayerProtAll {
				g.anyPlProtAll = true
				return
			}
		}
	}
}

// plProtAll reports whether player p carries a functioning
// StatPlayerProtAll static (a runtime emblem of p's).
func (s *State) plProtAll(k *prep, p uint8) bool {
	if k.gen == nil || !k.gen.anyPlProtAll || DebugPlProtAllOff {
		return false
	}
	for i := 0; i < int(s.g.nemb[p]); i++ {
		e := &k.t.Emblems[s.embIdx(p, i)-1]
		for j := range e.Abilities {
			a := &e.Abilities[j]
			if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatPlayerProtAll {
				return true
			}
		}
	}
	return false
}
