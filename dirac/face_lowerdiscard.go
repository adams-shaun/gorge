package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A face route's lowering that discards the card it casts. gorge lowers a
// face play's potential witness at priority, answering every ask the taps
// stack by its first option, and drops the play when the cast is then not
// offered (oracle gorge.go lowerCompletes, "lower dead end"). A witness that
// pays a life cost (Haunted Screen's "Pay 1 life: Add {U}") beside a
// life-loss discard trigger (Marina Vendrell's Grimoire) asks the payer for a
// discard; its first option is the hand's first card, so when that is the
// card being cast the cast is gone and gorge never makes it, though a later
// option would have kept it.

// faceLowerDiscardsCast reports whether lowering the witness of the face cast
// of object o (its face up, priced at cost) discards o: the witness's taps
// run on a copy, the triggers they queue resolve as the lowering's pass round
// resolves them and a discard ask is answered with the first option. It prices
// only a witness that pays a life cost, the one the lowering's trigger answer
// can touch.
func (s *State) faceLowerDiscardsCast(k *prep, p uint8, o Obj, cost *kop.ManaCost) bool {
	var dry planner
	dry.try, dry.dry = true, true
	if !s.tapForPKFound(k, p, cost, &dry) || !plannerPaysLife(&dry) {
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
	t.resolvePrefixStack(k, sp0, true)
	for guard := 0; guard < prefixDiscardLeft && t.prefixDiscardAsk(p); guard++ {
		var d Decision
		t.resolveOptions(k, &d)
		if d.N == 0 {
			break
		}
		t.Apply(d.Options[0])
		t.resolvePrefixStack(k, sp0, true)
	}
	return t.objs[o].zone != z0
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
