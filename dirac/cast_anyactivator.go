package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// "Any player may activate this ability" (Activator$ Player; Feral Hydra's
// {3}: put a +1/+1 counter on it; CR 602.2). gorge's offer walk visits every
// seat's battlefield and keeps an ability whose Activator$ admits the player
// (legal_walk_battlefield.go activatorAllows). The kernel models only the
// narrow shape kgen lowers (lower_anyactivator.go): a battlefield ability
// whose cost is plain mana and whose program reads nothing of the activator,
// so the ability is offered to the permanent's controller by the ordinary
// walk and to every other player here, priced against the activator's own
// sources and pool. The stack entry's controller is s.prio (beginActivate),
// the activator (CR 602.2a).

// prepAnyAct arms genPrep.anyAnyAct when some ability carries AnyActivator.
func prepAnyAct(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if t.Cards[i].Abilities[j].AnyActivator {
				g.anyAnyAct = true
				return
			}
		}
	}
}

// offerAnyActivations lists, for player p, the any-player abilities of the
// permanents the other players control.
func (s *State) offerAnyActivations(k *prep, d *Decision, p uint8, sorc bool, av *avail, haveAv *bool) {
	for q := uint8(0); q < s.nSeats(); q++ {
		if q == p || s.out(q) {
			continue
		}
		for m := s.ctrlM[q]; m.any(); m = m.rest() {
			sl := m.first()
			cid := s.cardAt(sl)
			if k.cards[cid].nAct == 0 || s.perms[sl].ovr.flags&(ovLoseAll|ovLoseAllEOT) != 0 {
				continue
			}
			cd := &k.t.Cards[cid]
			for ai := range cd.Abilities {
				a := &cd.Abilities[ai]
				if a.Kind != kop.AbActivated || !a.AnyActivator {
					continue
				}
				if !*haveAv {
					s.availFor(k, p, av)
					*haveAv = true
				}
				if s.canActivateAny(k, sl, a, p, sorc, av) {
					d.add(MakeAction(AkActivate, TCode(s.perms[sl].obj), 0, uint8(ai)))
				}
			}
		}
	}
}

// canActivateAny is canActivate for an AnyActivator ability (plain mana
// cost, no target, no limit: kgen's gate) of the permanent at sl, activated
// by player who.
func (s *State) canActivateAny(k *prep, sl Slot, a *kop.Ability, who uint8, sorc bool, av *avail) bool {
	if !ownActivatable(a) || s.genLost(k, sl) {
		return false
	}
	p := &s.perms[sl]
	if k.gen.anyNetLock && s.netLocked(p.obj) {
		return false
	}
	if k.gen.anyCantAct && s.actForbiddenBy(k, sl, who, false, false) {
		return false
	}
	if k.gen.anyWaterbend || k.gen.anyXBoundAb {
		if s.castHeldOut(p.obj) {
			return false
		}
	}
	if k.anyGate && !s.gateOK(sl, a) {
		return false
	}
	mana := a.Cost.Mana
	if k.gen.anyAbReduce {
		mana = s.abReduced(k, a, sl, who, mana)
	}
	if mana.MV() > 0 {
		pd := payDesc{class: pdAct, card: s.cardAt(sl), src: sl, instant: a.Cost.Flags&kop.CostSorcery == 0}
		if !av.affords(&mana, pipMaskOf(&mana), 0) && !(s.rbAny(who) && s.actPoolAffordsDesc(k, who, &mana, pd, av)) {
			return false
		}
	}
	return true
}
