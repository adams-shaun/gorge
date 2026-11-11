package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// dpExtPlayerUpkeep is the dprog.ext base of a record due as one player's
// upkeep begins (kop.DelayPlayerUpkeep): ext = dpExtPlayerUpkeep + the player.
// Exhaustion's "creatures and lands target opponent controls don't untap
// during their next untap step" ends with a Phase$ Untap trigger of the
// remembered player, which gorge registers as a delayed trigger: it goes on
// the stack as that player's upkeep begins (after the untap step has skipped
// the held permanents) and exiles the Effect. The kernel has no Effect object,
// so the program is empty and only poses the stack object, which resolves
// doing nothing.
const dpExtPlayerUpkeep uint8 = 4

// delayPlayerUpkeep registers OpDelayedProgram with kop.DelayPlayerUpkeep: an
// empty delayed program of the resolving controller due at the upkeep of the
// player in.Obj names. A target that is no longer a player registers nothing.
//
//go:noinline
func (s *State) delayPlayerUpkeep(k *prep, e *stackEntry, in *kop.Instr) {
	p := s.playerRef(k, in.Obj)
	if p == 255 {
		return
	}
	if s.g.ndp >= maxDProg {
		s.capFull(StallDProg)
		return
	}
	d := dprog{card: e.card, ab: uint8(in.C), ctrl: e.ctrl, src: e.obj, sv: s.objs[e.obj].ver, n: 1, ext: dpExtPlayerUpkeep + p}
	d.objs[0] = NoObj
	s.g.dp[s.g.ndp] = d
	s.g.dpLKI[s.g.ndp] = e.li
	s.g.ndp++
}
