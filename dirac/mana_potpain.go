package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// potPainNodeCap bounds potUnitsPainAffords's walk over the painful units'
// counts; a board past it keeps the per-unit answer.
const potPainNodeCap = 4096

// potUnitsPainAffords is the potential census's summed lethal guard (gorge
// pay/search.go: a count vector whose summed life plus damage reaches the
// payer's life is never a plan, CR 119.4). potUnitsAvail caps each recorded
// class alone at (life-1)/pain, so six taps of Manabarbs' one damage each
// look payable at 4 life when their classes are separate (a Mountain's {R},
// a Swamp's {B}, a Forest's {G}); the plan may take only life-1 pain in all.
// It reports whether some split of that budget over the painful classes
// still affords cost. A budget the units never reach, or a board past the
// node cap, keeps the per-unit answer (true).
func potUnitsPainAffords(units *[maxPotUnits]potUnit, life int, cost *kop.ManaCost) bool {
	if units[0].mask != 0 && units[0].n == 0 {
		return true // the overflow marker: the record is incomplete
	}
	budget := life - 1
	total := 0
	for i := 0; i < maxPotUnits && units[i].mask != 0; i++ {
		if u := units[i]; u.pain > 0 && !u.pick {
			total += int(u.n) * int(u.pain)
		}
	}
	if total <= budget {
		return true
	}
	tmp := *units
	nodes := 0
	return potPainWalk(&tmp, 0, budget, life, cost, &nodes)
}

// potPainWalk picks the count of painful class i (largest first) within the
// remaining pain budget and affords-checks the full choice at the end.
func potPainWalk(tmp *[maxPotUnits]potUnit, i, budget, life int, cost *kop.ManaCost, nodes *int) bool {
	for ; i < maxPotUnits && tmp[i].mask != 0; i++ {
		if tmp[i].pain > 0 && !tmp[i].pick {
			break
		}
	}
	if i >= maxPotUnits || tmp[i].mask == 0 {
		var px avail
		potUnitsAdd(&px, tmp, life) // the record is complete (the caller's potUnitsAvail held); a zeroed count is no overflow marker here
		return px.affords(cost, pipMaskOf(cost), 0)
	}
	*nodes++
	if *nodes > potPainNodeCap {
		return true
	}
	full := tmp[i].n
	w := int(tmp[i].pain)
	for c := min(int(full), budget/w); c >= 0; c-- {
		tmp[i].n = uint8(c)
		if potPainWalk(tmp, i+1, budget-c*w, life, cost, nodes) {
			tmp[i].n = full
			return true
		}
	}
	tmp[i].n = full
	return false
}

// plainCastPainOK is the plain hand cast's summed lethal guard under a
// TapsForMana watcher: gorge prices the cast through its potential planner,
// which never takes a plan whose summed life plus damage reaches the payer's
// life (CR 119.4), while the real census caps each colour's sources alone
// (Manabarbs at 3 life: a Billiard Room's {B}, an Unknown Shores' {C} and a
// Plains' {W} each look payable beside the other two). The same census read
// as kickedPotentialPainOK; it keeps the gate's answer (true) off the autopay
// surfaces, with no such watcher, for an {X} cost, or while the payer holds a
// multi-unit fee gap (filtFeeGaps): gorge's fee-order fallback floats such a
// filter's funding first and pays through it with no damage (Overflowing
// Basin's {1},{T}: {G}{U} beside an Island under Scald at 1 life), a route
// the potential census, which records the filter's gross unit, cannot see. A
// one-unit fee source (Unknown Shores' {1},{T}: any colour) takes no such
// fallback and keeps the census.
func (s *State) plainCastPainOK(k *prep, p uint8, c CardID, o Obj, cost *kop.ManaCost) bool {
	if s.cfg&(cfgGorgeAutopay|cfgGorgeLowering) == 0 || k.gen == nil || !k.gen.anyTapsMana || cost.X != 0 || s.filtFeeGaps(k, s.scriptFilt(k, p)).any() {
		return true
	}
	if s.g.availX&availGaps != 0 {
		// The gaps walk lists the candidates gorge's prefix search tries: a land a Taps
		// watcher defers is no planner unit, so the guard does not bind its taps, and
		// the search itself prices the play (an instant is offered with the triggers
		// still on the stack, CR 603.3; a sorcery-speed one dies at resolution,
		// deferGapWalk's t.over).
		return true
	}
	return s.kickedPotentialPainOK(k, p, c, o, cost)
}

// castPaysUnderPain is castGateOrPoolPays and the plain cast's summed lethal
// guard in one answer, so priorityOptions reads one condition.
func (s *State) castPaysUnderPain(k *prep, p uint8, c CardID, o Obj, cost *kop.ManaCost, cav *avail) bool {
	return s.castGateOrPoolPays(k, p, c, o, cost, cav) && s.plainCastPainOK(k, p, c, o, cost)
}
