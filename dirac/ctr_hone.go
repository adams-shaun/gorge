package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// CR 122.1j: a hone counter on an Equipment gives +1/+0 to any
// creature that Equipment is attached to. gorge emits it as a layer-7c
// additive effect (engine/rules/layers.go staticEffects, Affects
// "Creature.EquippedBy"): a counter's effect, not an ability of the
// permanent, so it does not need the Equipment to still be an Equipment and
// is not lost with the permanent's abilities.

// prepHone sets genPrep.anyHone when some card program, emblem, replacement
// or cost can place a hone counter. Over-inclusion only enables the read in
// ptGen; missing a placing op would drop the bonus, so every op that names
// a counter kind to put is covered (the operand map of kctrnamedFlipInstr).
func prepHone(t *kop.Table, g *genPrep) { g.anyHone = tablePlacesCtr(t, kop.CtrHone) }

// prepM2M1 sets genPrep.anyM2M1 when some program can place a -2/-1 counter
// (CR 122.1a): ptGen reads them only then.
func prepM2M1(t *kop.Table, g *genPrep) { g.anyM2M1 = tablePlacesCtr(t, kop.CtrM2M1) }

// tablePlacesCtr reports whether some card program, emblem, replacement or
// cost of t can place a counter of kind want.
func tablePlacesCtr(t *kop.Table, want kop.CounterKind) bool {
	prog := func(p []kop.Instr) bool {
		for i := range p {
			switch in := &p[i]; in.Op {
			case kop.OpPutCounter, kop.OpPutCounterAll, kop.OpEnterWithCounters, kop.OpDoubleCountersAll,
				kop.OpBolster, kop.OpDigCounters, kop.OpRemoveCounterAll:
				if kop.CounterKind(in.C) == want {
					return true
				}
			case kop.OpSupport:
				if kop.CounterKind(in.C&0xff) == want {
					return true
				}
			case kop.OpExileEx, kop.OpReturnEx, kop.OpMoveCounter:
				if kop.CounterKind(in.B.N) == want {
					return true
				}
			case kop.OpExt:
				if in.Ext() == kop.ExtSearchCounters && kop.CounterKind(in.C) == want ||
					in.Ext() == kop.ExtDividedCounters && kop.CounterKind(in.B.N) == want {
					return true
				}
			}
		}
		return false
	}
	abil := func(a *kop.Ability) bool {
		return prog(a.Prog) || a.Kind == kop.AbReplacement && a.Repl.Kind == kop.ReplEntersWithCounters && kop.CounterKind(a.Repl.C) == want ||
			a.Cost.Flags&kop.CostAddCtr != 0 && a.Cost.Ctr == want
	}
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if abil(&t.Cards[i].Abilities[j]) {
				return true
			}
		}
	}
	for i := range t.Emblems {
		for j := range t.Emblems[i].Abilities {
			if abil(&t.Emblems[i].Abilities[j]) {
				return true
			}
		}
	}
	return false
}

// honePower is the +N/+0 the creature at sl gets from hone counters on the
// permanents attached to it. The attached permanent need not be an Equipment
// nor keep its abilities (a counter's effect); a non-creature gets nothing.
//
//go:noinline
func (s *State) honePower(k *prep, sl Slot) int {
	if !s.isCreature(k, sl) {
		return 0
	}
	n := 0
	for m := s.occ; m.any(); m = m.rest() {
		q := m.first()
		if s.perms[q].attached == sl {
			n += s.perms[q].ctrN(s, int(kop.CtrHone))
		}
	}
	return n
}
