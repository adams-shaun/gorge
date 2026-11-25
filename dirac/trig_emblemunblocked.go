package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugIgnoreUnblockedEmblem makes the unblocked-attacker emblem hook queue
// nothing (the oracle's mutation test: the lockstep must diverge).
var DebugIgnoreUnblockedEmblem bool

// fireEmblemsUnblocked fires the end-of-turn emblems' unblocked-attacker
// triggers (gorge checkEffectCombatTriggers over queueAttackerUnblockedTrigger,
// Mode$ AttackerUnblocked; Gaze of Pain, Delif's Cone) after fireUnblocked's
// listener walk: for each live emblem of a seat still in the game, once per
// unblocked attacker in atk (the attacking player's battlefield order) that
// matches the line's ValidCard$ filter (read for the emblem's owner), as the
// creating card's latent command-zone copy with the attacker as its trigger
// object. An emblem that remembers an object (kop.Emblem.Capture; State.g.embN
// holds the object + 1) fires only for that object (Card.IsRemembered over the
// registration's Remembered set; a lapsed capture is 0 and fires for none).
//
//go:noinline
func (s *State) fireEmblemsUnblocked(k *prep, atk []Slot) {
	if DebugIgnoreUnblockedEmblem {
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
				if ea.Kind != kop.AbTriggered || ea.Trig.Event != kop.TrigBecomesBlocked || ea.Trig.Flags2&kop.Tf2Unblocked == 0 {
					continue
				}
				c := s.embCard(pp, i)
				cd := &k.t.Cards[c]
				for ab := range cd.Abilities {
					la := &cd.Abilities[ab]
					if la.Kind != kop.AbTriggered || la.AFlags&kop.AfLatent == 0 || la.Trig.Event != kop.TrigBecomesBlocked ||
						la.Trig.Flags2&kop.Tf2Unblocked == 0 || la.Trig.Zone != kop.ZoneCommand {
						continue
					}
					o := s.embObj(pp, i)
					for _, as := range atk {
						ao := s.perms[as].obj
						if e.Capture != kop.RefNone && int(s.embNum(pp, i)) != int(ao)+1 {
							continue
						}
						if ea.Trig.Filter != 0 && !s.matchPermGen(k, ea.Trig.Filter, as, pp, NoSlot, layerFull) {
							continue
						}
						if st := DebugEffEOTTrigStats; st != nil {
							st.UnblockedFires++
						}
						s.enqueueLKI(o, s.objs[o].ver+1, c, uint8(ab), pp, ao, s.perms[as].ctrl, 0)
					}
					break
				}
			}
		}
	}
}
