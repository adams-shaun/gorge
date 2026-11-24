package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// fwChosenCtrl: FwChosenCtrl, the battlefield permanent is controlled by the
// player the reading source permanent chose as it entered (gorge
// chosenCtrlMatches, engine/effects/filter_type.go:264: the source's Chosen
// player target against the candidate's controller). No source or no chosen
// player fails closed, as gorge's "no chosen player never matches".
//
//kmanifest:fword FwChosenCtrl
func (s *State) fwChosenCtrl(c *fctx, l *kop.FLit) bool {
	if !c.perm() {
		return false
	}
	ch := s.chosenPlayerOf(c.src)
	if ch == 255 {
		// an as-enters program runs with its permanent off occ (Canker
		// Abomination's counter count): read the entrant's perm record.
		// Only the entry being run (g.replaced) qualifies, so a stale byte
		// on a slot another object now holds is never read as a choice.
		o := s.g.replaced
		if o != NoObj && s.objs[o].zone == kop.ZoneBattlefield {
			sl := s.objs[o].slot
			if c.src == NoSlot || c.src == sl || c.srcObj == o {
				if p := s.perms[sl].chosen; p < objWord(s.nSeats()) {
					ch = uint8(p)
				}
			}
		}
	}
	return ch != 255 && s.perms[c.sl].ctrl == ch
}
