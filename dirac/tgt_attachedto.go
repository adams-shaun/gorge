package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// (gorge, 51ed6bce3) a ChangeZone whose AttachedTo$ is a
// card filter ("Creature.YouCtrl", One Last Job) names a class of bearers, not
// one, so the controller CHOOSES which legal bearer the returned card attaches
// to (CR 303.4f for an Aura, 301.5c for an Equipment): changeZoneAttachChoose
// poses a "card" KChoose over the legal candidates when there are two or more,
// attaches to the only one without an ask, and for an Aura with none leaves it
// in its zone (the empty named set, CR 303.4g). The candidates are the
// battlefield permanents the filter matches under the resolving controller; an
// Aura's are further narrowed by its own Enchant restriction (attachableTo,
// here enchantCands). The instruction carries the filter in F beside
// RetAttachTo (dirac/kgen lower_changezoneattach.go); F zero is the single-referent
// spelling (czAttachHost).

// DebugBearerFirst is the mutation switch: the first candidate is attached
// without asking (the earlier behaviour). TestOneLastJobLockstep's
// mutation run requires the lockstep to diverge.
var DebugBearerFirst bool

// raGenCzBearer is the bearer ask (ChGenChooseCard, one pick of the offered
// permanents); the card waits in its zone, as for raGenEnchant.
const raGenCzBearer = raGenManifestDread + 72

// czBearerCands writes the permanents the returning card o may be attached to
// by the filter in.F, in slot order, and returns their number. The filter's
// "You" is the resolving controller; the Aura's Enchant restriction is read
// for the controller it enters under (its owner, as czAttachHost).
func (s *State) czBearerCands(k *prep, in *kop.Instr, o Obj, out *[MaxOptions]uint8) int {
	c := s.objs[o].card
	ctrl, you := s.objs[o].owner(), s.res.e.ctrl
	n := 0
	if k.cards[c].types&kop.TAura != 0 {
		if k.t.Cards[c].Enchant.Kind != kop.TgtObject {
			return 0
		}
		var buf [MaxOptions]uint8
		for i, m := 0, s.enchantCands(k, c, ctrl, &buf); i < m; i++ {
			if s.matchPerm(k, in.F, s.objs[buf[i]].slot, you, NoSlot) {
				out[n] = buf[i]
				n++
			}
		}
		return n
	}
	for m := s.occ; m.any() && n < MaxOptions; m = m.rest() {
		sl := m.first()
		if s.objs[s.perms[sl].obj].of&ofPending != 0 {
			continue
		}
		if s.matchPerm(k, in.F, sl, you, NoSlot) {
			out[n] = uint8(s.perms[sl].obj)
			n++
		}
	}
	return n
}

// czBearerHost is czAttachHost for a filter bearer: the one legal permanent
// (host), the ask posed for two or more (stay, with res.ask = raGenCzBearer),
// or none (stay; an Aura remains in its zone, an Equipment enters unattached).
//
//go:noinline
func (s *State) czBearerHost(k *prep, in *kop.Instr, o Obj) (host Slot, stay bool) {
	var buf [MaxOptions]uint8
	n := s.czBearerCands(k, in, o, &buf)
	switch {
	case n == 0:
		return NoSlot, k.cards[s.objs[o].card].types&kop.TAura != 0
	case n == 1 || DebugBearerFirst:
		return s.objs[buf[0]].slot, false
	case k.cards[s.objs[o].card].types&kop.TAura == 0:
		return NoSlot, false // an Equipment enters first, then the controller chooses (czBearerAfter)
	}
	s.res.ask, s.res.askP = raGenCzBearer, s.res.e.ctrl
	return NoSlot, true
}

// czBearerAfter is the Equipment half, run once the returned card has entered
// unattached: gorge's changeZoneAttachedTo rider asks after the move (an Aura
// is asked before it, through the entry gate's named set), and the chosen
// permanent receives the attach. It reports whether the ask is posed.
//
//go:noinline
func (s *State) czBearerAfter(k *prep, in *kop.Instr, o Obj) bool {
	var buf [MaxOptions]uint8
	if s.objs[o].zone != kop.ZoneBattlefield || s.czBearerCands(k, in, o, &buf) < 2 {
		return false
	}
	s.res.ask, s.res.askP = raGenCzBearer, s.res.e.ctrl
	return true
}

// askOptionsBearer builds the bearer ask's options; false: not its ask.
//
//go:noinline
func (s *State) askOptionsBearer(k *prep, d *Decision) bool {
	if s.res.ask != raGenCzBearer {
		return false
	}
	in := &s.prog(k)[s.res.pc]
	d.Sub = ChGenChooseCard
	var buf [MaxOptions]uint8
	n := s.czBearerCands(k, in, s.objRefObj(in.Obj), &buf)
	for i := 0; i < n; i++ {
		d.add(MakeAction(AkChoose, TCode(buf[i]), 0, 0))
	}
	return true
}

// applyAskBearer applies the bearer answer: the return runs again with the
// chosen permanent as its host. false: not this ask.
//
//go:noinline
func (s *State) applyAskBearer(k *prep, a Action) bool {
	if s.res.ask != raGenCzBearer {
		return false
	}
	in := &s.prog(k)[s.res.pc]
	s.res.ask = raNone
	if o := s.objRefObj(in.Obj); s.objs[o].zone == kop.ZoneBattlefield {
		s.attachTo(k, s.objs[o].slot, s.objs[a.Source()].slot) // the Equipment has entered
		s.continueRes(k, 0)
	} else if !s.execReturn(k, in, s.objs[a.Source()].slot) {
		s.continueRes(k, 0)
	}
	return true
}
