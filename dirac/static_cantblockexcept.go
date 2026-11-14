package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Agility Bobblehead's "can't be blocked this turn except by creatures with
// haste" (gorge Effect Mode$ CantBlockBy | ValidAttacker$ Card.IsRemembered |
// ValidBlocker$ Creature.withoutHaste; CR 509.1b, 702.10a, 611.2c).
// ExtExHaste records, until the cleanup step, the attacker (obj, ver) in
// State.cold.exH; canBlock refuses a blocker without haste (its CURRENT
// keywords, so the same turn's KW$ Haste grant to the blocker's own side
// counts) for a recorded attacker. The filter is fixed to withoutHaste: no
// general ValidBlocker$ engine. A record is neither a keyword nor an
// ability, so a loss of all abilities leaves it and no projected keyword
// list shows it.
//
// Cavern Stomper / Cheeky House-Mouse ("can't be blocked by creatures with
// power 2 or less / 3 or greater this turn", ValidBlocker$ Creature.powerLE2
// / powerGE3) ride the same record with C bit 0: bits 1-3 the comparison
// (kop.CmpOp), bits 4-7 the constant N. gorge matches ValidBlocker$ against
// the would-be blocker AT BLOCK DECLARATION, so canBlock reads the blocker's
// CURRENT power (CR 509.1b, 611.2c), never one locked in at resolution.
//
// A record ends with its own permanent, whoever else the effect named
// (CR 611.2c). All behind genPrep.anyExHaste, so every other table's hot paths and hashes are
// unchanged.

// DebugExHasteOff is the mutation switch of TestTgtsetExHasteMutation: the
// record is never read, so the attacker is blockable by any creature.
var DebugExHasteOff bool

// maxExH bounds the live records of a turn; a further one is a loud stall,
// never a dropped restriction.
const maxExH = 16

// StallExHaste: a seventeenth Ext ExHaste record in one turn (State.cold.exH).
const StallExHaste = 112

func init() { stallNames[StallExHaste] = "exhaste" }

// prepExHaste sets anyExHaste.
func prepExHaste(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Ext() == kop.ExtExHaste {
					g.anyExHaste = true
				}
			}
		}
	}
}

// exHaste is ExtExHaste: the object in Obj can't be blocked this turn except
// by creatures with haste, or (C bit 0) by creatures whose power compares
// (C bits 1-3, kop.CmpOp) with the constant in C bits 4-7.
func (s *State) exHaste(k *prep, in *kop.Instr) {
	sl := s.objRef(k, in.Obj)
	if sl == NoSlot {
		return
	}
	kind := uint8(1)
	if in.C&1 != 0 { // seat-ok: an instruction flag bit
		kind = 0x80 | uint8(in.C>>1&7)<<4 | uint8(in.C>>4&15)
	}
	s.exhAdd(sl, kind)
}

// exhAdd records kind on the permanent at sl: the same grant again is a
// no-op, a full store a loud stall.
func (s *State) exhAdd(sl Slot, kind uint8) {
	o := s.perms[sl].obj
	free := -1
	for i := range s.cold.exH {
		r := &s.cold.exH[i]
		if r.kind == kind && r.obj == o && r.ver == s.objs[o].ver {
			free = -2 // the same grant again
			break
		}
		if r.kind == 0 && free < 0 {
			free = i
		}
	}
	if free == -1 {
		s.capFull(StallExHaste)
		return
	}
	if free >= 0 {
		s.cold.exH[free] = exhRec{obj: o, ver: s.objs[o].ver, kind: kind}
	}
	s.touchSl(sl)
}

// exhOf reports whether the permanent at sl has a live record of any kind.
func (s *State) exhOf(sl Slot) bool {
	if DebugExHasteOff {
		return false
	}
	o := s.perms[sl].obj
	for i := range s.cold.exH {
		if r := &s.cold.exH[i]; r.kind != 0 && r.obj == o && r.ver == s.objs[o].ver {
			return true
		}
	}
	return false
}

// exhAny reports whether any record is live.
func (s *State) exhAny() bool {
	for i := range s.cold.exH {
		if s.cold.exH[i] != (exhRec{}) {
			return true
		}
	}
	return false
}

// exhBlocks reports whether a record of attacker a forbids blocker b: a
// haste record refuses a blocker without haste (its current keywords), a
// power record one whose current power compares as recorded.
func (s *State) exhBlocks(k *prep, a, b Slot) bool {
	if DebugExHasteOff {
		return false
	}
	o := s.perms[a].obj
	for i := range s.cold.exH {
		r := &s.cold.exH[i]
		if r.kind == 0 || r.obj != o || r.ver != s.objs[o].ver {
			continue
		}
		if r.kind == 1 {
			if s.kw(k, b)&kop.KwHaste == 0 {
				return true
			}
		} else if r.kind >= 0x80 && cmp(kop.CmpOp(r.kind>>4&7), s.power(k, b), int(r.kind&15)) {
			return true
		}
	}
	return false
}

// exhClear ends the restrictions (the cleanup step, CR 514.2).
func (s *State) exhClear() {
	for i := range s.cold.exH {
		if r := &s.cold.exH[i]; r.kind != 0 {
			if s.objs[r.obj].zone == kop.ZoneBattlefield && s.objs[r.obj].ver == r.ver {
				s.touch(r.obj)
			}
			*r = exhRec{}
		}
	}
}
