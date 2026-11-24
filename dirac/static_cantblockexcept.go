package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Agility Bobblehead's "can't be blocked this turn except by creatures with
// haste" (gorge: an Effect granting a CantBlockBy static on the remembered
// attacker against blockers without haste; CR 509.1b, 702.10a, 611.2c).
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
// (Kick in the Door; Tower of Coireall, Doorman, Joven's Tools and Firefright
// Mage lack ForgetOnMoved$ and kgen refuses them) The general
// ValidBlocker$ form rides the same store as kind exkBlocker: the record holds
// a battlefield filter (Instr C bit 8, Instr F) and canBlock refuses every
// blocker the filter matches, evaluated on the blocker's CURRENT characteristics
// at block declaration like gorge's ValidBlocker$ match. Several records of one
// attacker each forbid on their own, so a union ValidBlocker$ is one filter
// with alternatives.
//
// (Duct Crawler, Screeching Griffin, Spin Engine, Kozilek's Pathfinder,
// Shrewd Hatchling, Auriok Siege Sled) "target creature can't block this
// creature this turn" is kind exkNoBlock: Obj the named blocker, W the
// attacker (the source). The record is the pair (attacker, blocker) at their
// current versions, so it lapses when either leaves the battlefield
// (UntilHostLeavesPlayOrEOT, ForgetOnMoved$ Battlefield) and at cleanup.
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

// coldExH is the turn's records, for a table with ExtExHaste; an empty one is
// simply full.
var coldExH = newCold[exhRec]("exH", func(g *genPrep) int {
	if g.anyExHaste {
		return maxExH
	}
	return 0
}).view(func(c *coldArrays) *[]exhRec { return &c.exH })

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
	if in.C&exCNoBlock != 0 { // seat-ok: an instruction flag bit
		s.exNoBlock(k, in, sl)
		return
	}
	kind := uint8(1)
	var f kop.FilterID
	if in.C&1 != 0 { // seat-ok: an instruction flag bit
		kind = 0x80 | uint8(in.C>>1&7)<<4 | uint8(in.C>>4&15)
	}
	if in.C&exCFilter != 0 { // seat-ok: an instruction flag bit
		kind, f = exkBlocker, in.F
	}
	s.exhAddF(sl, kind, f)
}

// exCNoBlock is the Instr C bit of the named-blocker form: Obj the blocker, W
// the attacker.
const exCNoBlock = 1 << 9

// exkNoBlock is the exhRec kind of the named-blocker form.
const exkNoBlock uint8 = 5

// exNoBlock records that the creature at bl can't block the attacker W names
// (the effect's source) this turn.
func (s *State) exNoBlock(k *prep, in *kop.Instr, bl Slot) {
	as := s.objRef(k, in.W)
	if as == NoSlot {
		return // the source already left: its duration is over
	}
	a, b := s.perms[as].obj, s.perms[bl].obj
	free := -1
	for i := range s.cold.exH {
		r := &s.cold.exH[i]
		if r.kind == exkNoBlock && r.obj == a && r.ver == s.objs[a].ver && r.b == b && r.bver == s.objs[b].ver {
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
		s.cold.exH[free] = exhRec{obj: a, ver: s.objs[a].ver, kind: exkNoBlock, b: b, bver: s.objs[b].ver}
	}
	s.touchSl(as)
}

// exCFilter is the Instr C bit of the filter form: F is the battlefield
// filter of the blockers the record forbids.
const exCFilter = 1 << 8

// exkBlocker is the exhRec kind of the filter form (the other kinds: 1 haste,
// 2 and 3 combat_asgngrant.go's, 0x80 and up the power form).
const exkBlocker uint8 = 4

// exhAdd records kind on the permanent at sl: the same grant again is a
// no-op, a full store a loud stall.
func (s *State) exhAdd(sl Slot, kind uint8) { s.exhAddF(sl, kind, 0) }

// exhAddF is exhAdd with the record's blocker filter (kind exkBlocker).
func (s *State) exhAddF(sl Slot, kind uint8, f kop.FilterID) {
	o := s.perms[sl].obj
	free := -1
	for i := range s.cold.exH {
		r := &s.cold.exH[i]
		if r.kind == kind && r.f == f && r.obj == o && r.ver == s.objs[o].ver {
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
		s.cold.exH[free] = exhRec{obj: o, ver: s.objs[o].ver, kind: kind, f: f}
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
		} else if r.kind == exkNoBlock {
			if s.perms[b].obj == r.b && s.objs[r.b].ver == r.bver {
				return true
			}
		} else if r.kind == exkBlocker {
			if s.matchPermGen(k, r.f, b, s.perms[a].ctrl, a, layerFull) {
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
