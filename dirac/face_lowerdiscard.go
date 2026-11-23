package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A face route's lowering whose witness's resolution unmakes the cast. gorge
// lowers a face play's potential witness at priority, answering every ask the
// taps stack by its first option, and drops the play when the cast is then
// not offered (oracle gorge.go lowerCompletes, "lower dead end"). Two asks
// the answer can unmake the cast:
//   - a discard ask from a life-paying witness (Haunted Screen's "Pay 1
//     life: Add {U}") beside a life-loss discard trigger (Marina Vendrell's
//     Grimoire): its first option is the hand's first card, so when that is
//     the card being cast the cast is gone, though a later option would have
//     kept it;
//   - a trigger's own pay/decline election (Sorcerer's Broom's "Whenever you
//     sacrifice another permanent, you may pay {3}. If you do, create a
//     token") over the source the witness sacrifices (a Treasure): the first
//     option pays, the floated pool leaves the plan's prediction (gorge
//     payexec's pool_mismatch abort) whatever the remainder still affords.

// faceLowerUnmakesCast reports whether lowering the witness of the face cast
// of object o (its face up, priced at cost) leaves the cast unmakeable: a
// discard ask answered with the first option moved o out of its zone, or an
// answered ask moved the floated pool off the plan's prediction. It prices
// only a witness that pays a life cost or sacrifices a source, the witnesses
// whose resolution can pose either ask.
func (s *State) faceLowerUnmakesCast(k *prep, p uint8, o Obj, cost *kop.ManaCost) bool {
	var dry planner
	dry.try, dry.dry = true, true
	if !s.tapForPKFound(k, p, cost, &dry) || !(plannerPaysLife(&dry) || plannerSacrifices(&dry)) {
		return false
	}
	z0 := s.objs[o].zone
	t := s.fork()
	defer s.unfork()
	var pk planner
	pk.try = true
	sp0 := t.sp
	t.tapForPK(k, p, cost, 0, NoObj, false, false, &pk)
	if !pk.found {
		return false
	}
	var floated [kop.NumManaKinds]uint8
	copy(floated[:], t.pl[SeatIx(p)].pool[:])
	t.resolvePrefixStack(k, sp0, true)
	for guard := 0; guard < prefixDiscardLeft && (t.prefixDiscardAsk(p) || t.lowerPayAsk(p)); guard++ {
		var d Decision
		t.resolveOptions(k, &d)
		if d.N == 0 {
			break
		}
		a := d.Options[0]
		if d.N > 1 && t.lowerPayAsk(p) {
			// the pay arm (mode 1): gorge's trigger_cost ask lists pay first,
			// so its first-option answer pays (rules cumulative.go)
			a = d.Options[1]
		}
		t.Apply(a)
		t.resolvePrefixStack(k, sp0, true)
	}
	if t.objs[o].zone != z0 {
		return true // the discard ask answered with the hand's first card
	}
	for c := 0; c < kop.NumManaKinds; c++ {
		if floated[c] != t.pl[SeatIx(p)].pool[c] {
			return true // an answered election moved the pool off the plan's prediction
		}
	}
	return false
}

// lowerPayAsk reports whether the resolution the witness's taps queued now
// asks p a trigger's own pay/decline election (gorge's trigger_cost ask,
// rules cumulative.go): the ask whose first-option answer pays from the
// floated pool.
func (s *State) lowerPayAsk(p uint8) bool {
	return !s.over && s.ctl == ctlResolve && s.res.ask == raGenPay && s.res.askP == p
}

// plannerPaysLife reports whether the planner's chosen witness takes a unit
// that costs life or damage (a pain source, a paid-life ability).
func plannerPaysLife(pk *planner) bool {
	for i := 0; i < pk.bestN; i++ {
		if t := &pk.types[pk.bestTy[i]]; t.pain != 0 || t.life {
			return true
		}
	}
	return false
}

// plannerSacrifices reports whether the planner's chosen witness sacrifices
// a source (a tap-and-sacrifice unit, a sacrifice-only one): the sacrifice a
// Sacrificed trigger can pose the election lowerPayAsk answers.
func plannerSacrifices(pk *planner) bool {
	for i := 0; i < pk.bestN; i++ {
		if t := &pk.types[pk.bestTy[i]]; t.sac || t.sacOnly {
			return true
		}
	}
	return false
}
