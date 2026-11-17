package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// "<blocker> blocks CARDNAME this turn if able." (CR 509.1c; gorge
// api:MustBlock, effMustBlock: one MustBlock continuous duty per blocker,
// bound to the attacker frozen at resolution, MustBlockAttacker). gorge marks
// an offered pair Required (BlockMust) only when the blocker's duty names
// that attacker (combat.MustBlockPairRequired), and its blockRequiredCore
// counts one unit per duty blocker that blocks an attacker it is bound to.
// The kernel keeps each (blocker, attacker) duty as a record of the cold
// table mbaRecs and adds the pair to the maximum-requirements search of
// combat_mustbeblocked.go (mbSearch.req): the defender is offered a pair only
// while the maximum stays reachable and Done only once it is met.
//
// A record ends as the next turn begins (the duty lasts this turn), at the
// end of combat when its duty does (Duration$ UntilEndOfCombat), and when a
// new permanent takes either slot (arenaClrSlot, CR 400.7: the blocker or the
// attacker left the battlefield, and gorge's duty names the old objects).

// mbaRec is one duty: b the blocker's slot + 1 (0: a free record), a the
// attacker's slot, eoc the duty ends with this combat.
type mbaRec struct{ b, a, eoc uint8 }

// maxMba is how many duties one turn holds records for; a ninth stalls
// (StallMustBlock).
const maxMba = 8

// mbaRecs is the duty table (nil when no card of the table makes one).
func (s *State) mbaRecs() []mbaRec { return coldMba.of(s) }

var coldMba = newCold[mbaRec]("mba", func(g *genPrep) int {
	if g.uses.has(useMustBlockAtk) {
		return maxMba
	}
	return 0
})

// useMustBlockAtk: some instruction of the table is an ExtMustBlock.
var useMustBlockAtk = newGenUse(tableHasMustBlockAtk)

// tableHasMustBlockAtk reports whether some instruction of t is an
// ExtMustBlock.
func tableHasMustBlockAtk(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Op == kop.OpExt && in.Ext() == kop.ExtMustBlock {
					return true
				}
			}
		}
	}
	return false
}

// mustBlockAtkExt is ExtMustBlock: the battlefield object Obj must block the
// resolving source if able. Nothing is recorded when either is gone.
//
//kmanifest:ext ExtMustBlock
func (s *State) mustBlockAtkExt(k *prep, in *kop.Instr) (ok, done bool) {
	b, a := s.objRef(k, in.Obj), s.objRef(k, kop.RefSelf)
	if b == NoSlot || a == NoSlot {
		return true, true
	}
	eoc := uint8(0)
	if in.C == kop.DelayEndCombat { // Duration$ UntilEndOfCombat
		eoc = 1
	}
	r := s.mbaRecs()
	free := -1
	for i := range r {
		switch {
		case r[i].b == uint8(b)+1 && r[i].a == uint8(a):
			r[i].eoc &= eoc // a this-turn duty outlasts a this-combat one
			return true, true
		case r[i].b == 0 && free < 0:
			free = i
		}
	}
	if free < 0 {
		s.capFull(StallMustBlock)
		return true, true
	}
	r[free] = mbaRec{b: uint8(b) + 1, a: uint8(a), eoc: eoc}
	return true, true
}

// mbaBound is the attackers blocker b is bound to block.
func (s *State) mbaBound(b Slot) (m slotMask) {
	for _, r := range s.mbaRecs() {
		if r.b != 0 && r.b == uint8(b)+1 {
			m = m.with(Slot(r.a))
		}
	}
	return
}

// mbaDuty: some defending permanent is bound to an attacking creature.
func (s *State) mbaDuty() bool {
	def := s.ctrlM[s.blockDef()]
	for _, r := range s.mbaRecs() {
		if r.b != 0 && def.has(Slot(r.b-1)) && s.perms[r.a].flags&pfAttacking != 0 {
			return true
		}
	}
	return false
}

// mbaDrop ends every duty naming slot sl, as blocker or as attacker.
func (s *State) mbaDrop(sl Slot) {
	r := s.mbaRecs()
	for i := range r {
		if r[i].b != 0 && (r[i].b == uint8(sl)+1 || r[i].a == uint8(sl)) {
			r[i] = mbaRec{}
		}
	}
}

// mbaEndCombat ends the this-combat duties.
func (s *State) mbaEndCombat() {
	r := s.mbaRecs()
	for i := range r {
		if r[i].eoc != 0 {
			r[i] = mbaRec{}
		}
	}
}
