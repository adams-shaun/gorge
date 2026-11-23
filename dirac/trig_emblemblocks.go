package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugIgnoreBlocksEmblem makes the blocks emblem hook queue nothing (the
// oracle's mutation test: the lockstep must diverge).
var DebugIgnoreBlocksEmblem bool

// fireEmblemsBlocks fires the end-of-turn emblems' blocks triggers (gorge
// checkEffectCombatTriggers over queueBlocksTrigger, Mode$ Blocks; Battle
// Cry, Benefactor's Draught) after fireBlocksGen's listener walk: for each
// live emblem of a seat still in the game, once per declared (attacker,
// blocker) pair in declaration order whose BLOCKER matches the line's
// ValidCard$ filter (read for the emblem's owner), as the creating card's
// latent command-zone copy. The trigger object is the blocker and the trigger
// player its controller (a printed Blocks line's is the attacker: an emblem
// has no blocker of its own to anchor to, and every Effect Blocks body reads
// the blocker; kgen refuses the attacker reads, lower_trigrefs.go).
//
//go:noinline
func (s *State) fireEmblemsBlocks(k *prep, ps []blkPair) {
	if DebugIgnoreBlocksEmblem {
		return
	}
	for pp := uint8(0); pp < s.nSeats(); pp++ {
		if s.out(pp) {
			continue
		}
		for i := 0; i < int(s.g.nemb[pp]); i++ {
			e := &k.t.Emblems[s.embIdx(pp, i)-1]
			if !e.UntilEOT {
				continue
			}
			for j := range e.Abilities {
				ea := &e.Abilities[j]
				if ea.Kind != kop.AbTriggered || ea.Trig.Event != kop.TrigBlocks {
					continue
				}
				c := s.embCard(pp, i)
				cd := &k.t.Cards[c]
				for ab := range cd.Abilities {
					la := &cd.Abilities[ab]
					if la.Kind != kop.AbTriggered || la.AFlags&kop.AfLatent == 0 || la.Trig.Event != kop.TrigBlocks || la.Trig.Zone != kop.ZoneCommand {
						continue
					}
					o := s.embObj(pp, i)
					for _, pr := range ps {
						if ea.Trig.Filter != 0 && !s.matchPermGen(k, ea.Trig.Filter, pr.b, pp, NoSlot, layerFull) {
							continue
						}
						if st := DebugEffEOTTrigStats; st != nil {
							st.BlocksFires++
						}
						bo := s.perms[pr.b].obj
						s.enqueueLKI(o, s.objs[o].ver+1, c, uint8(ab), pp, bo, s.perms[pr.b].ctrl, 0)
					}
					break
				}
			}
		}
	}
}
