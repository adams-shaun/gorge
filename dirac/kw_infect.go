package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// --- infect and toxic (CR 702.90, 702.164): poison counters ---

// prepInfect sets genPrep.anyInfect when a card has printed infect or
// toxic, and infectStall when such a table also has a shape the kernel's
// conversion does not order like gorge: a damage replacement-order ask
// (rbDblDmg, rbPrevNC: the parked and per-event paths deal plain damage) or a
// counter-added trigger (gorge queues it right after each damage event's
// own triggers). Proliferate gives players' poison counters one more
// (act_proliferate.go) as gorge does, so it no longer stalls.
func prepInfect(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		if t.Cards[i].Infect || t.Cards[i].Toxic != 0 || t.Cards[i].Wither {
			g.anyInfect = true
		}
	}
	if tableGrantsWither(t) {
		g.anyInfect, g.anyWitherGrant = true, true // a granted wither (kw_wither.go)
	}
	if !g.anyInfect {
		return
	}
	if g.anyRepl&(rbDblDmg|rbPrevNC|rbPlusDmg|rbPrevCol) != 0 {
		g.infectStall = true
	}
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			a := &t.Cards[i].Abilities[j]
			if a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigCounterAddedOnce {
				g.infectStall = true
			}
		}
	}
}

// Poison is player p's poison counters.
func (s *State) Poison(p uint8) int { return int(s.g.poison[SeatIx(p)]) }

// addPoison gives player p n poison counters (saturating at 255).
func (s *State) addPoison(p uint8, n int) {
	if n <= 0 {
		return
	}
	if k := s.k(); k.gen != nil && k.gen.anyRepl&rbDblCtrYou != 0 {
		for c := s.dblCtrYou(k, s.counterAdder()); c > 0 && n <= 0xff; c-- {
			n *= 2 // trig_leavereturn.go
		}
	}
	if s.g.noPoison != 0 || s.k().gen.anyOnePoison {
		if n = s.onePoison(s.k(), p, n); n <= 0 {
			return // trig_leavereturn.go (Melira)
		}
	}
	s.g.poison[p] = uint8(min(int(s.g.poison[p])+n, 255))
}

// poisonSBA: a player with ten or more poison counters loses (CR 704.5c;
// gorge checkLoseConditions).
//
//go:noinline
func (s *State) poisonSBA(k *prep) {
	for p := uint8(0); p < s.nSeats(); p++ {
		if s.g.poison[p] >= 10 && !s.pl[p].lost && !s.cantLose(k, p) {
			s.pl[p].lost = true
			s.monarchLeft(k, p) // CR 725.4 (act_monarch.go)
		}
	}
}

// infectAt: the permanent at sl has printed infect and has not lost its
// abilities (gorge HasKeyword reads the derived keywords).
func (s *State) infectAt(k *prep, sl Slot) bool {
	return k.t.Cards[s.cardAt(sl)].Infect && !s.genLost(k, sl)
}

// toxicAt is the permanent at sl's printed toxic N, 0 once it has lost its
// abilities (gorge ToxicValue).
func (s *State) toxicAt(k *prep, sl Slot) int {
	n := 0
	if c := k.t.Cards[s.cardAt(sl)].Toxic; c != 0 && !s.genLost(k, sl) {
		n = int(c)
	}
	if k.gen.anyColGrant {
		_, _, t := s.cgOf(sl) // a chosen-colour toxic grant (static_colourgrant.go)
		n += t
	}
	return n
}

// srcInfect is the resolving source's infect: the permanent's while it is
// on the battlefield, else its card's printed infect (srcKw's rule).
func (s *State) srcInfect(k *prep) bool {
	if sl := s.objRef(k, kop.RefSelf); sl != NoSlot {
		return s.infectAt(k, sl)
	}
	return k.t.Cards[s.res.e.card].Infect
}

// infectCounters puts infect damage n on creature sl as -1/-1 counters
// (CR 702.90b; gorge convertInfectDamage's CounterChange, through the
// counter replacements).
func (s *State) infectCounters(k *prep, sl Slot, n int) {
	if n > 0 {
		s.g.dmgTurn.set(s.perms[sl].obj) // infect damage is dealt damage (gorge events.foldDamage records it before the counter branch)
		s.recordDmgAmt(k, sl, n)         // a reader card's amount (value_dmgamount.go)
	}
	s.g.lastCK = kop.CtrM1M1
	adder := s.res.e.ctrl // the damage source's controller (gorge infect.go publishes it)
	if !s.res.active {
		adder = s.combatFoe(k, sl) // combat: the opposing creature dealt it
	}
	m := s.placeCounters(k, sl, int(kop.CtrM1M1), n, adder, ctrPutBy)
	if m > 0 && k.anyCtrAdded {
		s.ctrNoteBy(k, sl, int(kop.CtrM1M1), adder)
	}
}

// infectDamageTo is damageTo's tail for an infect source: poison counters
// to a player, -1/-1 counters to a creature (gorge emitPlayerDamage /
// emitObjectDamage's infect markers); a noncreature takes ordinary damage.
//
//go:noinline
func (s *State) infectDamageTo(k *prep, code uint8, n int, kw kop.KwMask) {
	e := &s.res.e
	if isPlayerOrNone(code) {
		s.addPoison(CodeSeat(code), n)
	} else {
		ob := &s.objs[code]
		if ob.zone != kop.ZoneBattlefield {
			return
		}
		sl := ob.slot
		s.touch(Obj(code))
		p := &s.perms[sl]
		if s.isCreature(k, sl) {
			s.dmgCountersIf(k, sl, n)
			s.infectCounters(k, sl, n)
		} else {
			s.dmgPerm(k, sl, p, n)
		}
		if kw&kop.KwDeathtouch != 0 {
			p.markDT(1)
		}
	}
	s.res.lastDmg = int16(n)
	if kw&kop.KwLifelink != 0 {
		s.gainLife(k, s.srcCtrl(k), n)
	}
	if k.gen.anyPermMark {
		s.markDealt(k, s.objRef(k, kop.RefSelf), false) // filter_permmarks.go
	}
	if s.subs[kop.TrigDamageDone].any() || s.subs[kop.TrigDamageDealtOnce].any() || k.gen != nil && k.gen.graveDmg {
		if sl := s.objRef(k, kop.RefSelf); sl != NoSlot {
			s.fireDamage(k, sl, int16(n), isPlayerOrNone(code), false, code)
		} else if e.obj != NoObj {
			s.fireDamageSrc(k, e.obj, e.ctrl, int16(n), dmgPl(code), code, nil)
		}
	}
}

// infectBetween is damageBetween for an infect creature a damaging
// creature b: -1/-1 counters instead of marked damage.
//
//go:noinline
func (s *State) infectBetween(k *prep, a, b Slot, n int, akw kop.KwMask) {
	s.dmgCountersIf(k, b, n)
	s.infectCounters(k, b, n)
	if akw&kop.KwDeathtouch != 0 {
		s.perms[b].markDT(1)
	}
	s.res.lastDmg = int16(n) // its lifelink: dealHits' gain
}

// infectCombat converts a combat damage pass's infect and toxic events
// before it is applied (gorge runCombatAssignments): an infect source's
// damage to a creature becomes -1/-1 counters (taken out of toPerm) and to
// a player poison counters (out of toPl; infM keeps combatEventsGen from
// charging it as life loss); a toxic source's damage to a player also
// gives toxic N poison counters. Damage triggers and lifelink still read
// the whole event.
//
//go:noinline
func (s *State) infectCombat(k *prep, r *dmgRec) {
	for i := 0; i < r.nev; i++ {
		ev := &r.ev[i]
		if ev.n <= 0 {
			continue
		}
		if ev.dst == NoSlot {
			t := s.perms[ev.src].atk
			if isObjTarget(t) {
				continue
			}
			pl := CodeSeat(t)
			if s.infectAt(k, ev.src) {
				r.toPl[pl] -= ev.n
				r.infM = r.infM.with(ev.src)
				r.poison[pl] += ev.n
			}
			r.poison[pl] += int16(s.toxicAt(k, ev.src))
			continue
		}
		if !s.combatHitIsCounters(k, ev) {
			continue
		}
		r.toPerm[ev.dst] -= ev.n
		s.dmgCountersIf(k, ev.dst, int(ev.n))
		s.infectCounters(k, ev.dst, int(ev.n))
		if r.toPerm[ev.dst] <= 0 && r.dtPerm.has(ev.dst) {
			s.perms[ev.dst].markDT(s.dtEvents(k, r, ev.dst))
			r.dtPerm.clr(ev.dst)
		}
	}
}

// combatHitIsCounters: combat event ev puts -1/-1 counters on a creature
// (an infect or wither source) instead of marking damage.
func (s *State) combatHitIsCounters(k *prep, ev *dmgEv) bool {
	return k.gen.anyInfect && (s.infectAt(k, ev.src) || s.witherAt(k, ev.src)) && s.isCreature(k, ev.dst)
}

// prepPoison: a table whose programs give poison counters (OpPoison) or
// whose ward costs poison counters (CardDef.WardPoison, paid at resolution
// through addPoison) checks the ten-poison state-based action (anyInfect gates poisonSBA)
// and, beside proliferate (gorge proliferates players' poison), stalls as
// an infect table does.
func prepPoison(t *kop.Table, g *genPrep) {
	poison, prolif := false, false
	for i := range t.Cards {
		poison = poison || t.Cards[i].WardPoison != 0
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				poison = poison || in.Op == kop.OpPoison
				prolif = prolif || in.Op == kop.OpProliferate
			}
		}
	}
	if poison {
		g.anyInfect = true
		g.infectStall = g.infectStall || prolif
	}
}

// combatFoe is the controller of the creature that dealt combat damage to
// the creature at sl: the active player when sl blocks, else the player its
// attack is against (the controller of a blocker of that attack, CR 509.1a).
func (s *State) combatFoe(k *prep, sl Slot) uint8 {
	c := s.perms[sl].ctrl
	if MaxSeats == 2 {
		return NextSeat(c)
	}
	if c != s.active {
		return s.active
	}
	if p := s.atkSeat(k, sl); p != NoTarget {
		return p
	}
	return s.nextAlive(c)
}
