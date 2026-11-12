package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// dpExtRemUpkeep is the dprog.ext base of a record due at the next upkeep of
// any player that carries a remembered player (kop.DelayRemUpkeep): ext =
// dpExtRemUpkeep + the player. Arcane Denial's "its controller may draw up to
// two cards at the beginning of the next turn's upkeep" remembers the
// countered spell's controller, a player, which the kernel's delayed record
// (objects only) holds here. The player rides the fired trigger as its
// trigger player (RefTriggerPlayer); the program has no remembered object.
// The player-upkeep records (dpExtPlayerUpkeep + p) sit below this base.
const dpExtRemUpkeep uint8 = 16

// delayRemUpkeep registers OpDelayedProgram with kop.DelayRemUpkeep: a
// delayed program of the resolving controller due at the next upkeep of any
// player, whose trigger player is the player in.Obj names. A target that is
// no longer a player registers nothing.
//
//go:noinline
func (s *State) delayRemUpkeep(k *prep, e *stackEntry, in *kop.Instr) {
	p := s.playerRef(k, in.Obj)
	if p == 255 {
		return
	}
	if s.g.ndp >= maxDProg {
		s.capFull(StallDProg)
		return
	}
	d := dprog{card: e.card, ab: uint8(in.C), ctrl: e.ctrl, src: e.obj, sv: s.objs[e.obj].ver, n: 1, ext: dpExtRemUpkeep + p}
	d.objs[0] = NoObj
	s.g.dp[s.g.ndp] = d
	s.g.dpLKI[s.g.ndp] = e.li
	s.g.ndp++
}
