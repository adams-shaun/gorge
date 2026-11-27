package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A cost program with two or more filtered Sac atoms (kgen lower_costprog.go:
// Viscerid Drone's "Sacrifice a creature and a Swamp"): each atom is its own
// sacrifice part, paid in program order, and one permanent pays one part
// (CR 118.3). gorge's offer requires a distinct assignment of every unit
// (pay.SacrificeRemainderFeasible), and its sacAsk walks the parts in order,
// offering for a part with a later part only the candidates that leave a
// complete assignment (pay.FeasibleSacrificeChoices); the last part offers
// every candidate not already chosen. The kernel asks one pick per unit, as
// it does for a single part, with the atom's own filter.

// multiSacMax bounds a program's sacrifice units (kgen costProgSacUnits).
const multiSacMax = 8

// multiSacProg is ability a's program when it has two or more filtered Sac
// atoms, else nil.
func multiSacProg(k *prep, a *kop.Ability) []kop.CostAtom {
	if a == nil || a.CostProg == 0 || k.gen == nil || !k.gen.uses.has(useCostProg) {
		return nil
	}
	prog := costProgOf(k, a)
	n := 0
	for i := range prog {
		if prog[i].K == kop.CostAtomSac && prog[i].F != 0 {
			n++
		}
	}
	if n < 2 {
		return nil
	}
	return prog
}

// sacUnitCand: the permanent at sl may pay a unit of atom at for player p.
func (s *State) sacUnitCand(k *prep, at *kop.CostAtom, sl Slot, p uint8, src Slot) bool {
	return s.matchPerm(k, at.F, sl, p, src) && !s.sacBlocked(k, sl)
}

// sacAssign reports whether the units from atom ai's unit u on can each be
// given a distinct permanent none of used[:nu] holds (gorge
// SacrificeRemainderFeasible's search, in the same order).
func (s *State) sacAssign(k *prep, prog []kop.CostAtom, ai, u int, p uint8, src Slot, used *[multiSacMax]Obj, nu int) bool {
	for ai < len(prog) && (prog[ai].K != kop.CostAtomSac || prog[ai].F == 0 || u >= int(prog[ai].N.N)) {
		ai, u = ai+1, 0
	}
	if ai == len(prog) {
		return true
	}
	if nu == multiSacMax {
		return false
	}
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		sl := m.first()
		o := s.perms[sl].obj
		if sacUsed(used, nu, o) || !s.sacUnitCand(k, &prog[ai], sl, p, src) {
			continue
		}
		used[nu] = o
		if s.sacAssign(k, prog, ai, u+1, p, src, used, nu+1) {
			return true
		}
	}
	return false
}

func sacUsed(used *[multiSacMax]Obj, nu int, o Obj) bool {
	for i := 0; i < nu; i++ {
		if used[i] == o {
			return true
		}
	}
	return false
}

// multiSacFeasible is the offer's check: every unit of every Sac atom has a
// distinct permanent.
func (s *State) multiSacFeasible(k *prep, prog []kop.CostAtom, p uint8, src Slot) bool {
	var used [multiSacMax]Obj
	return s.sacAssign(k, prog, 0, 0, p, src, &used, 0)
}

// multiSacUnits is the program's total sacrifice units.
func multiSacUnits(prog []kop.CostAtom) uint8 {
	n := 0
	for i := range prog {
		if prog[i].K == kop.CostAtomSac && prog[i].F != 0 {
			n += int(prog[i].N.N)
		}
	}
	return uint8(n)
}

// multiSacPicks is the build's sacrifice picks so far, in pick order.
func (s *State) multiSacPicks(used *[multiSacMax]Obj) int {
	n := 0
	for i := 0; i < int(s.bld().pickN) && n < multiSacMax; i++ {
		if s.bld().pickKs[i] == pkSac {
			used[n] = s.bld().picks[i]
			n++
		}
	}
	return n
}

// multiSacCands lists the current unit's candidates (costCands for a
// multi-sacrifice program) and returns how many units of the current atom
// are still owed. A unit with a later Sac atom offers only the candidates
// that leave a complete assignment (gorge sacAsk's FeasibleSacrificeChoices).
func (s *State) multiSacCands(k *prep, prog []kop.CostAtom, p uint8, src Slot, d *Decision) int {
	var used [multiSacMax]Obj
	done := s.multiSacPicks(&used)
	ai, u := 0, done
	for ai < len(prog) {
		if at := &prog[ai]; at.K == kop.CostAtomSac && at.F != 0 {
			if u < int(at.N.N) {
				break
			}
			u -= int(at.N.N)
		}
		ai++
	}
	if ai == len(prog) {
		return 0
	}
	later := false
	for j := ai + 1; j < len(prog); j++ {
		later = later || prog[j].K == kop.CostAtomSac && prog[j].F != 0
	}
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		sl := m.first()
		o := s.perms[sl].obj
		if sacUsed(&used, done, o) || !s.sacUnitCand(k, &prog[ai], sl, p, src) {
			continue
		}
		if later {
			used[done] = o
			if !s.sacAssign(k, prog, ai, u+1, p, src, &used, done+1) {
				continue
			}
		}
		d.add(MakeAction(AkChoose, TCode(o), 0, 0))
	}
	return int(prog[ai].N.N) - u
}

// pickMultiSac is the build's multi-sacrifice program while its sacrifice
// picks are asked, else nil.
func (s *State) pickMultiSac(k *prep) []kop.CostAtom {
	if s.bld().pickK != pkSac || k.gen == nil || !k.gen.uses.has(useCostProg) || s.bld().e.kind != seAbility {
		return nil
	}
	return multiSacProg(k, s.buildAbility(k))
}
