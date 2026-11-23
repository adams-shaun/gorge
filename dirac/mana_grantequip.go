package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The play's own Equipment losing its host to a tap on a prefix path (gorge
// scriptSearch.visit's offered scan at every node, CR 704.5g, 602.2b).
//
// A granted mana ability that taps the equipped creature can change what that
// creature is while it is tapped: Castle Raptors's "As long as it is untapped,
// it gets +0/+2" falls to a toughness its marked damage already covers, so
// tapping it for Paradise Mantle's granted "{T}: Add one mana of any color"
// kills it and Mantle falls off. The Equip being paid for has no creature to
// attach to afterwards, and gorge's search, which scans the play's offer at
// every node, finds none. The search here re-reads the offer only where the
// path grew the stack, answered an ask or sacrificed (waitsForStack), so a
// host that died to its own tap priced the Equip as payable.

// armAttachWatch records the play's own source's host at the search root: the
// Equipment (or Aura) an activation is played from, attached to a battlefield
// permanent.
func (g *grantSrch) armAttachWatch(s *State) {
	g.attWatch = false
	if !g.d.hasPlay || g.d.play.Kind() != AkActivate {
		return
	}
	o := Obj(g.d.play.Source())
	if o == NoObj || int(o) >= len(s.objs) || s.objs[o].zone != kop.ZoneBattlefield {
		return
	}
	if at := s.perms[s.objs[o].slot].attached; at != NoSlot && s.perms[at].flags&pfTapped == 0 {
		g.attWatch, g.attHost = true, at
	}
}

// hostTapped: the play's own source was attached at the root to an untapped
// host that this path has tapped, so the host's characteristics may have
// changed and the play's offer is read again with the state-based actions run.
func (g *grantSrch) hostTapped(t *State) bool {
	if !g.attWatch {
		return false
	}
	o := Obj(g.d.play.Source())
	if o == NoObj || int(o) >= len(t.objs) || t.objs[o].ver != g.pver || t.objs[o].zone != kop.ZoneBattlefield {
		return false
	}
	return t.occ.has(g.attHost) && t.perms[g.attHost].flags&pfTapped != 0
}

// hostGone: after the state-based actions the host's tap led to, the play is
// no longer offered.
func (g *grantSrch) hostGone(t *State) bool {
	c := t.fork()
	c.touchSl(g.attHost) // the search set pfTapped directly: the scan's clean-board cache must see the tap
	c.sba(g.k)
	var d Decision
	c.plainOptions(g.k, &d, 0)
	gone := !hasOption(&d, g.d.play)
	t.unfork()
	return gone
}
