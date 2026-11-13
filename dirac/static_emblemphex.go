package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The player hexproof grant on a runtime emblem (Blossoming Calm: "you gain
// hexproof until your next turn"). gorge registers it as a continuous
// Player.You keyword static (registerPlayerPumpGrant) that rules'
// playerKeywords gates read (CR 702.11c: an opponent's spell or ability
// cannot target the player; their own can). The kernel lowers the same grant
// to a next-turn emblem of the resolving controller carrying a plain
// StatPlayerHexproof (A == 0; kgen lower_effect.go playerGrantStatics), the
// emblem's UntilNextTurn ending it at the cleanup before the controller's
// next turn (dropNextTurnEmblems), as static_plprot.go's StatPlayerProtAll
// emblem does. playerHexproof (repl_gen.go) reads it beside the permanent
// sources, so targetOK and the resolution recheck see it.
//
// The reader sits behind genPrep.anyEmbPlHex (prepEmbPlHex: a table emblem
// statics scan), so a table without such an emblem pays nothing.

// DebugEmbPlHexOff is the switch of dirac/oracle's
// TestBlossomingCalmEmbPlHexMutation: the emblem's hexproof is ignored, so the
// lockstep against gorge must diverge when an opponent could target the
// player. Read in embPlHex only.
var DebugEmbPlHexOff bool

// prepEmbPlHex sets genPrep.anyEmbPlHex when the table carries an emblem
// StatPlayerHexproof static.
func prepEmbPlHex(t *kop.Table, g *genPrep) {
	for i := range t.Emblems {
		for j := range t.Emblems[i].Abilities {
			if a := &t.Emblems[i].Abilities[j]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatPlayerHexproof {
				g.anyEmbPlHex = true
				return
			}
		}
	}
}

// embPlHex reports whether player p carries a StatPlayerHexproof static on
// one of their runtime emblems.
func (s *State) embPlHex(k *prep, p uint8) bool {
	if k.gen == nil || !k.gen.anyEmbPlHex || DebugEmbPlHexOff {
		return false
	}
	for i := 0; i < int(s.g.nemb[p]); i++ {
		e := &k.t.Emblems[s.embIdx(p, i)-1]
		for j := range e.Abilities {
			a := &e.Abilities[j]
			if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatPlayerHexproof {
				return true
			}
		}
	}
	return false
}
