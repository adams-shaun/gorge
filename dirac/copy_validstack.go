package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// gorge's CopySpellAbility over Defined$ ValidStack (effCopySpellAbility's
// plural arm, validStackCopyTargets): Ulalek, Fused Atrocity's "copy all
// spells you control, then copy all other activated and triggered abilities
// you control. You may choose new targets for the copies." Each sweep reads
// the stack bottom first and copies every match once; the copies go on the
// stack above the originals (CR 707.10), so the matches are the entries below
// the resolving one as the sweep began, and a copy made by the sweep is never
// itself swept.
//
// The sweep resumes across the copies' new-target elections in the
// resolution's own cursor bytes, the mass redirect's (ExtChangeTargetsMass):
// res.ksrAt is the next stack index to consider and res.left the number of
// entries below the resolving one when the sweep began. The copies are minted
// between those entries and the resolving one, so the indices the cursor
// walks never move.

// prepCopyStackValid sets the genPrep gates an ExtCopyStackValid reaches: a
// spell sweep may copy a permanent spell (its copy becomes a token, CR
// 707.10f), an ability sweep names stack abilities, and C bit 0 elects.
func prepCopyStackValid(g *genPrep, in *kop.Instr) {
	if in.C&2 == 0 {
		g.anyPermCopy = true
	} else {
		g.anyStackTgt, g.anyCopyAb = true, true
	}
	if in.C&1 != 0 { // seat-ok: an instruction flag bit
		g.anyCopyElect = true
	}
}

// copyStackValid is ExtCopyStackValid: C bit 1 clear copies every spell the
// resolving controller controls (Spell.YouCtrl), set every activated and
// triggered ability they control but the resolving one and its family
// (Ability.YouCtrl+otherAbility); C bit 0 (MayChooseTarget$ True) poses each
// copy's new-target election as it is made (gorge's mint, ask, mint, ask).
//
//kmanifest:ext ExtCopyStackValid
func (s *State) copyStackValid(k *prep, in *kop.Instr) (ok, done bool) {
	s.res.ksrAt, s.res.left = 0, s.sp-1
	return s.copyStackValidNext(k, in), true
}

// copyStackValidNext copies the matches from the cursor on; false: an
// election is posed, and copyResume re-enters it once the election closes.
// Every election owes a re-entry (left 1), so the held count of copies this
// resolution made (copyHold.made) survives each release and copierUnder lifts
// all of them above the resolving entry, as gorge stacks them; the sweep's
// end (copyStackValidEnd) settles the count.
func (s *State) copyStackValidNext(k *prep, in *kop.Instr) bool {
	r := &s.res
	abs := in.C&2 != 0
	for {
		i := s.copyStackValidFrom(k, abs, int(r.ksrAt))
		if i < 0 {
			s.copyStackValidEnd(k)
			return true
		}
		r.ksrAt = uint8(i) + 1
		if abs {
			if !s.mintAbilityCopy(i) {
				s.copyStackValidEnd(k)
				return true
			}
		} else if !s.mintSpellCopyExc(k, i, r.e.ctrl, 0) {
			s.copyStackValidEnd(k)
			return true
		}
		if in.C&1 != 0 && s.copyElect(k, 1) { // seat-ok: an instruction flag bit
			return false
		}
	}
}

// copyStackValidEnd ends the sweep at res.pc: the cursor is cleared, and the
// held count of copies made goes back to zero unless the next instruction is
// the chained ability sweep (Ulalek's SubAbility$), whose elections must lift
// the spell sweep's copies too: gorge puts every copy of the resolution above
// it.
func (s *State) copyStackValidEnd(k *prep) {
	s.res.ksrAt, s.res.left = 0, 0
	if len(s.cold.copyHeld) == 0 {
		return
	}
	p, pc := s.prog(k), int(s.res.pc)
	if p[pc].C&2 == 0 && pc+1 < len(p) && p[pc+1].Ext() == kop.ExtCopyStackValid && p[pc+1].C&2 != 0 {
		return
	}
	s.cold.copyHeld[0].made = 0
}

// copyStackValidFrom is the first stack index at or after i, below the bound
// res.left, that the sweep copies, or -1.
func (s *State) copyStackValidFrom(k *prep, abs bool, i int) int {
	r := &s.res
	for ; i < int(r.left); i++ {
		e := &s.stack[i]
		if e.ctrl != r.e.ctrl || (e.kind == seSpell) == abs {
			continue
		}
		if !abs {
			return i
		}
		if e.obj == r.e.obj && e.card == r.e.card && e.ab == r.e.ab {
			// otherAbility: the resolving ability's family, its copies and an
			// earlier incarnation's instances included (gorge validStackAdmits:
			// same Source ObjID, which survives a zone change, and same Ability)
			continue
		}
		if e.ab < abCascade {
			a := &k.t.Cards[e.card].Abilities[e.ab]
			if a.CantCopy || a.Kind == kop.AbMana {
				continue // gorge SpellCopyAllowed; a mana ability can't be copied (CR 707.10)
			}
		}
		return i
	}
	return -1
}
