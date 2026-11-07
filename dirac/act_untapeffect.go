package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// untapEffect is an effect's real untap of the permanent at sl, after the
// stun-counter substitution (CR 122.1d) and untapBlocked have passed: the
// tapped flag clears, the untap's locks end, and, as untapCost does, a
// permanent that was tapped fires its Untaps triggers (gorge TryUntap's
// Untap event; CR 701.21c, 603.2), each owned by its listener's controller.
func (s *State) untapEffect(k *prep, sl Slot) {
	p := &s.perms[sl]
	was := p.flags & pfTapped
	p.flags &^= pfTapped
	s.untapEnds(k, p.obj) // the lock and the host-tapped grants end at the first untap (static_animatewhiletapped.go)
	if was != 0 && k.gen != nil && s.subs[kop.TrigUntaps].any() {
		s.fireObjGen(k, kop.TrigUntaps, sl, p.obj, p.ctrl, 0)
	}
}
