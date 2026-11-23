package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugRemLKIPrinted is the mutation: sacRemember accumulates
// the sacrificed card's PRINTED toughness instead of its derived
// last-known one. Tests only.
var DebugRemLKIPrinted bool

// remLKIPT is ExRemLKIPT (RememberedLKI$CardPower / $CardToughness over a
// RememberSacrificed$ set): the characteristic sum sacRemember accumulated
// before each permanent left. X.N is checked by kgen only (one program reads
// one of the two, and the Sacrifice sums that one).
func (s *State) remLKIPT() int { return int(s.res.remLK) }

// remLKAdd adds one sacrificed permanent's last-known power or toughness
// (the Sacrifice's B.N bits 2 and 4) to the resolution's sum, saturating at
// +-32767. fireSac runs it through sacRemember before the permanent moves.
//
//go:noinline
func (s *State) remLKAdd(k *prep, sl Slot, bits int) {
	pw, t := s.ptGen(k, sl)
	if bits&kop.SacSumMV != 0 {
		// Sacrificed$CardManaCost: the printed mana value,
		// the same read gorge's SacrificedInfo.ManaValue captured.
		pw = s.objMV(k, s.cardAt(sl), s.perms[sl].obj)
		if DebugCountHeads11Off {
			pw = 0
		}
	} else if bits&kop.SacSumToughness != 0 {
		pw = t
		if DebugRemLKIPrinted {
			pw = int(k.cards[s.cardAt(sl)].tough)
		}
	}
	s.remLKSum(pw)
}

// remLKSum adds pw to the resolution's remLK sum with remLKAdd's saturation;
// the battle-link pick's writer (pickRemLKPower) passes the power it captured
// before the move.
func (s *State) remLKSum(pw int) {
	v := int(s.res.remLK) + pw
	if v > 32767 {
		v = 32767
	} else if v < -32767 {
		v = -32767
	}
	s.res.remLK = int16(v)
}

// pickRemLKPower is the battle-link remember pick's writer (Yannik,
// Scavenging Sentinel): the picked card's last-known power joins the
// resolution's remLK sum before it leaves (CR 608.2h, gorge
// TriggerRemembered$CardPower), the read the reflexive trigger captures as
// it is put on the stack. The gate is the table's own ExRemLKIPT read
// (genPrep.anyRemLKIPick) plus the pick's shape: a battlefield pick linked to
// its host (SrBattleLink) that remembers. A token is never written: it ceases
// in exile (CR 111.7) and gorge's read over the remembered set skips it.
// ok is false with the pre-move power when the pick is the writer's shape.
//
//go:noinline
func (s *State) pickRemLKPower(k *prep, o Obj) (pw int, ok bool) {
	if k.gen == nil || !k.gen.anyRemLKIPick || !s.res.active || s.objs[o].of&ofToken != 0 {
		return 0, false
	}
	pr := s.prog(k)
	if int(s.res.pc) >= len(pr) {
		return 0, false
	}
	in := &pr[s.res.pc]
	if !battleLinkPick(k.t, in) || s.g.pickF&kop.SrRemember == 0 {
		return 0, false
	}
	sl := s.objs[o].slot
	if sl == NoSlot {
		return 0, false
	}
	pw, _ = s.ptGen(k, sl)
	return pw, true
}

// tableReadsRemLKIPT reports some expr of t is ExRemLKIPT (genPrep.anyRemLKIPick,
// the battle-link pick writer's gate: the pick accumulates only where this
// table reads the accumulator).
func tableReadsRemLKIPT(t *kop.Table) bool {
	for i := range t.Exprs {
		if t.Exprs[i].Kind == kop.ExRemLKIPT {
			return true
		}
	}
	return false
}
