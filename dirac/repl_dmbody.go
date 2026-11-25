package dirac

// The ReplDamageInstead ask bodies (kop.ReplDmgInsSac: Dralnu, Lich Lord's
// sacrifice; kop.ReplDmgInsVind: Phyrexian Vindicator's prevent-and-deal).
// gorge runs the replacement body at the proposal (rules/replacement.go
// applyNonMoveReplacements):
//
//   - the Vindicator's body is effects.effImmediateTrigger: it queues the
//     Execute's DealDamage as a CR 603.12 reflexive trigger that goes on the
//     stack the next time a player would receive priority, its target chosen
//     as it is put there. So the kernel asks nothing in the event: the body
//     enqueues the card's latent ability (kop.Repl.Trig) carrying the
//     event's amount, and the damage is prevented (or stands, CR 615.12).
//   - Dralnu's body is a Sacrifice: its controller picks the permanents
//     (a KChoose of the eligible count) while more are eligible than owed,
//     else every remaining one goes. The kernel cannot run it where the
//     event is applied, so it parks the event on its ask (the way parkNC
//     parks one on the replacement order):
//   - a noncombat damage op (damageTo): the event is parked in s.g.nc, the
//     op suspends on runProg's nc check (res.ask raGenReplOrder), and the
//     answer applies the body and runs the parked event (dealNC).
//   - a combat pass: replOrderFreeze freezes the pass when an event matches
//     the ask (replOrderPossible names rb2DmgInst), and the frozen walk
//     poses it when it reaches the event (ro.bdy): the event is left
//     undealt, the walk's remaining events dealt after the answer, then
//     dealAsked takes the event off the recomputed pass.
//
// A frozen pass is recomputed on every resume, so a recompute never applies
// a body (dmgBody defers under ro.on): the walk applies the all-remaining and
// the no-ask cases once at its reach (sacWalk, vindWalk). A source the
// sacrifice removed still deals its frozen event (dealGoneEvent: gorge reads
// its last known information).
//
// gorge serves no ask tape when a damage pass is re-entered from a division's
// answer, so there the sacrifice takes its stand-in, the first picks in
// battlefield order, without asking (sacNoAsk, replHdr.dvAns).

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The sacrifice ask mirrors gorge's canonical decision (oracle canon.go's
// decisionKinds map): a DkChoose over ChSacrifice whose options are raGenSac's
// (interp_ask.go) — the controller's permanents minus the picked ones and the
// sac-blocked, in slot order, one AkChoose per object. gorge poses it only
// when more permanents are eligible than the amount owes, so the kernel's
// first ask of a batch is the one the oracle compares.

// The ask bodies' codes (ncDmg.body and replHdr.bdy).
const (
	bodySac  uint8 = 1 // kop.ReplDmgInsSac: the controller sacrifices that many permanents
	bodyVind uint8 = 2 // kop.ReplDmgInsVind: the host deals that much damage to a chosen target
)

// bodyAskOf reads a body's ask bits into its code; 0 when the body applies
// without a choice.
func bodyAskOf(body int) uint8 {
	switch {
	case body&kop.ReplDmgInsSac != 0:
		return bodySac
	case body&kop.ReplDmgInsVind != 0:
		return bodyVind
	}
	return 0
}

// ncAsk is the ask body a damage event met (replDamageAsk's out-param): the
// caller that can suspend it (damageTo) parks the event on its ask; every
// other replDamage caller treats a body as a loud stall (a site that cannot
// suspend, gorge asks).
type ncAsk struct {
	body uint8 // bodyAskOf's code
	hc   uint8 // the answerer: the host's controller
}

// dmgBodyScan is the battlefield scan for a matching DamageInstead line, ask
// bodies only when askOnly (the frozen walk asks only those; the plain
// bodies apply inline, repl_dminstead.go). It returns the matching host, its
// line and the number of matches: two is gorge's CR 616.1 order of two lines
// (the callers stall), so the caller reads a single hit only.
//
//go:noinline
func (s *State) dmgBodyScan(k *prep, srcCtrl uint8, srcSl Slot, srcCard CardID, rcp uint8, sl Slot, combat, askOnly bool) (Slot, *kop.Repl, int) {
	g := k.gen
	var host Slot = NoSlot
	var line *kop.Repl
	hits := 0
	for m := s.statM; m.any(); m = m.rest() {
		h := m.first()
		if g.replBits2[s.cardAt(h)]&rb2DmgInst == 0 || s.genLost(k, h) || s.faceDown(k, h) {
			continue
		}
		cd := &k.t.Cards[s.cardAt(h)]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind != kop.AbReplacement || a.Repl.Kind != kop.ReplDamageInstead {
				continue
			}
			b := int(uint16(a.Repl.A))&0xff | int(uint16(a.Repl.C))&(kop.ReplDmgInsSac|kop.ReplDmgInsVind)
			if askOnly && bodyAskOf(b) == 0 {
				continue
			}
			if !s.dmgInsteadMatch(k, &a.Repl, h, srcCtrl, srcSl, srcCard, rcp, sl, combat) {
				continue
			}
			host, line = h, &a.Repl
			hits++
		}
	}
	return host, line, hits
}

// dmgBody applies a matching ask body (dmgInstead's arm): the damage is
// replaced, and the body's choice is asked. On a frozen pass (s.g.ro.on) the
// walk poses the ask (replOrderCombat), so the recomputed pass defers here;
// a site that cannot suspend the ask (ask nil) stalls loudly; damageTo's
// call parks the event on its ask (*ask filled). The sacrifice body applies
// at once when every remaining eligible permanent is owed (gorge startSac's
// cnt > n: fewer are all sacrificed, in battlefield order, one batch).
func (s *State) dmgBody(k *prep, host Slot, line *kop.Repl, body int, rcp uint8, sl Slot, combat bool, n int, ask *ncAsk) bool {
	b := bodyAskOf(body)
	hc := s.perms[host].ctrl
	count := func() {
		for i := 0; i < len(DmgInsteadStats); i++ { // the body's own counters (the pool-reach proof)
			if body&(1<<i) != 0 {
				DmgInsteadStats[i].Add(1)
			}
		}
	}
	if b == bodyVind {
		// gorge (engine/effects/immediate.go): the Execute's reflexive trigger
		// is queued from the replacement's application and put on the stack the
		// next time a player would receive priority, its target chosen as it is
		// put there — so the body applies here, with no ask of its own, and the
		// resolution (or the combat pass) continues under it.
		count()
		if s.g.ro.on {
			return false // a frozen pass defers the body to the walk (vindWalk)
		}
		o := s.perms[host].obj
		s.enqueue(o, s.objs[o].ver, s.cardAt(host), line.Trig, hc, NoObj, hc, int16(n))
		if body&kop.ReplDmgInsPrevent != 0 && !s.noPrev(k, combat) {
			return true // the damage is prevented: nothing dealt
		}
		return false // CR 615.12 stand: the body ran and the caller deals n
	}
	if b == bodySac && !s.g.ro.on && s.sacNoAsk(k, hc, n, combat) {
		count() // the body applies now, without an ask
		s.sacFirst(k, hc, n)
		return true
	}
	if s.g.ro.on {
		// a frozen pass's every recompute defers: the walk poses the ask, or
		// applies the all-remaining case once at its reach (sacWalk); applying
		// it here would run the body again on each resume
		return false
	}
	if ask == nil {
		s.capFull(StallReplPlus) // a site that cannot suspend the body's ask
		count()
		return true
	}
	count() // the ask is posed (the frozen walk counts its own pose)
	*ask = ncAsk{body: b, hc: hc}
	return true
}

// sacPool is the permanents of p the sacrifice body may take: p's, minus the
// picked ones of the ask in flight (s.g.sacM) and the sac-blocked (a
// StatCantSacrifice). It is gorge's effSacrifice eligibility, the one set the
// options, the count and every no-ask arm read.
func (s *State) sacPool(k *prep, p uint8) slotMask {
	m := s.ctrlM[p].andNot(s.g.sacM)
	if k.gen == nil || !k.gen.anyCantSac {
		return m
	}
	for r := m; r.any(); r = r.rest() {
		if sl := r.first(); s.sacBlocked(k, sl) {
			m = m.without(sl)
		}
	}
	return m
}

// bodySacLeft is the number of permanents p still owes the sacrifice body's
// picks (the picked ones of the ask in flight aside: s.g.sacM).
func (s *State) bodySacLeft(k *prep, p uint8) int {
	var ord [MaxPerm]Slot
	return s.battlefieldOrder(p, s.sacPool(k, p), ord[:], 0)
}

// sacNoAsk reports that the sacrifice body of an event owing n picks applies
// without an ask: every remaining eligible permanent is owed (gorge startSac's
// cnt > n fails: they are all sacrificed), or the event is a combat event of
// a damage step whose division was answered. gorge re-enters that pass from
// the division's answer with no ask tape to serve, so the body takes its
// deterministic stand-in, the first picks in battlefield order (the
// sacrifice effect's "no engine host to ask" arm).
func (s *State) sacNoAsk(k *prep, hc uint8, n int, combat bool) bool {
	if s.bodySacLeft(k, hc) <= n {
		return true
	}
	if !combat {
		return false
	}
	if s.g.ro.on {
		return s.g.ro.dvAns
	}
	return s.combatArm().divDone.any()
}

// sacFirst sacrifices the first n permanents of p in battlefield order
// (all of them when fewer remain), one batch.
func (s *State) sacFirst(k *prep, p uint8, n int) {
	var ord [MaxPerm]Slot
	no := s.battlefieldOrder(p, s.sacPool(k, p), ord[:], 0)
	for _, sl := range ord[:min(no, n)] {
		s.addSac(sl)
	}
	s.flushSac(k)
}

// parkBodyNC parks the noncombat damage event whose body ask was met
// (damageTo, replDamageAsk's out-param): nothing is dealt now (nc.n -1; a
// CR 615.12 stand deals after the answer), the ask is posed when the
// resolution suspends (runProg's nc check) and answered by the body's
// controller (ro.p). The deal's notes read the source as it stood (the way
// parkNC's record does).
func (s *State) parkBodyNC(k *prep, code TCode, srcCtrl uint8, srcSl Slot, srcCard CardID, ba *ncAsk, n int, kw kop.KwMask) {
	nc := ncDmg{code: code, ctrl: srcCtrl, ll: kw&kop.KwLifelink != 0, dt: kw&kop.KwDeathtouch != 0,
		body: ba.body, hc: ba.hc, left: int16(n), n: -1}
	if srcSl != NoSlot {
		nc.src = s.perms[srcSl].obj
	} else {
		nc.src, nc.spell = s.res.e.obj, s.res.e.kind == seSpell
	}
	s.g.nc = nc
	s.g.ro.replHdr = replHdr{p: ba.hc}
}

// replBodyOptions poses the sacrifice body's decision, the canonical shape
// the oracle compares against gorge (the header's map): DkChoose over
// ChSacrifice with raGenSac's options — the controller's permanents minus
// the picked ones (s.g.sacM keeps them off until they leave) and the
// sac-blocked, in slot order, one AkChoose per object. The answerer is the
// replacement host's controller. The redirect body poses no decision of its
// own (its reflexive trigger's target ask is the kernel's ordinary trigger
// ask), so only bodySac reaches here.
func (s *State) replBodyOptions(d *Decision) {
	var hc uint8
	if s.g.nc.body != 0 {
		hc = s.g.nc.hc
	} else {
		hc = s.g.ro.p
	}
	d.Kind, d.Sub, d.Player = DkChoose, ChSacrifice, hc
	k := s.k()
	for m := s.sacPool(k, hc); m.any(); m = m.rest() {
		d.add(MakeAction(AkChoose, TCode(s.perms[m.first()].obj), 0, 0))
	}
}

// replBodyNCAnswer applies the answer to a parked sacrifice body's decision,
// true while the ask is posed again (a pick owed); the finish mirrors
// interp_ask's raGenReplOrder epilogue: the parked event dealt by dealNC
// (nothing for a replacement) and the resolution resumed.
func (s *State) replBodyNCAnswer(k *prep, a Action) bool {
	nc := s.g.nc
	if nc.body != bodySac {
		return false
	}
	s.addSac(s.objs[Obj(a.Source())].slot)
	nc.left--
	rem := s.bodySacLeft(k, nc.hc)
	if nc.left > 0 && rem > int(nc.left) {
		s.g.nc = nc
		return true // gorge's eligible count is still above the picks owed
	}
	if nc.left > 0 {
		// fewer remain than owed: the rest are all sacrificed (startSac)
		var ord [MaxPerm]Slot
		no := s.battlefieldOrder(nc.hc, s.sacPool(k, nc.hc), ord[:], 0)
		for _, sl := range ord[:no] {
			s.addSac(sl)
		}
	}
	s.flushSac(k)
	s.bodyNCEpilogue(k)
	return true
}

// bodyNCEpilogue is replBodyNCAnswer's finish: the parked event dealt (a
// CR 615.12 stand, dealNC reading the park still in s.g.nc), then the
// resolution resumed (interp_ask's raGenReplOrder epilogue).
func (s *State) bodyNCEpilogue(k *prep) {
	s.prevNextNC()
	s.spNC()
	s.g.ro.replHdr = replHdr{}
	s.dealNC(k)
	if s.g.dmgSeqOn {
		s.res.ask = raNone
		s.runProg(k) // the same damage op deals its next event
		return
	}
	s.continueRes(k, 0)
}

// replBodyCombatAnswer applies the answer to the frozen walk's sacrifice
// body: one pick, and the finish mirrors applyReplOrder's ro.on epilogue —
// the asked event dealt or prevented by dealAsked on the walk's resume
// (ro.fin).
func (s *State) replBodyCombatAnswer(k *prep, a Action) {
	ro := &s.g.ro
	if ro.bdy != bodySac {
		return
	}
	s.addSac(s.objs[Obj(a.Source())].slot)
	ro.left--
	rem := s.bodySacLeft(k, ro.p)
	if ro.left > 0 && rem > int(ro.left) {
		return // ask again
	}
	if ro.left > 0 {
		var ord [MaxPerm]Slot
		no := s.battlefieldOrder(ro.p, s.sacPool(k, ro.p), ord[:], 0)
		for _, sl := range ord[:no] {
			s.addSac(sl)
		}
	}
	s.flushSac(k)
	ro.an = 0 // the damage is replaced: the walk's resume deals nothing (dealAsked)
	s.combatBodyDone(&ro.replHdr)
}

// combatBodyDone is the frozen walk's ask body's finish: the walk's resume
// takes the asked event off the recomputed pass (replOrderCombat's ro.fin
// head, dealAsked), and the remaining events are dealt under it as gorge
// deals them under the pending ask.
func (s *State) combatBodyDone(ro *replHdr) {
	ro.bdy, ro.left = 0, 0
	ro.n, ro.fin = 0, true
	s.ctl = ctlDivide // the damage step continues (divA = NoSlot)
}

// dmgBodyWalk reports whether the frozen walk's event fe still matches an
// ask body and poses it: the host re-matched as it stands (a host a
// recomputed pass's other replacement removed matches nothing), the
// recomputed pass's own amount for the event still there (a replacement that
// applied at the pass prevented it already — the sacrifice body's auto case,
// repl_dminstead.go), and no other replacement a candidate of the event
// (initStall keeps every damage replacement off a DamageInstead line; a hit
// through its holes is gorge's CR 616.1 order, unmodelled beside an ask).
// Only the sacrifice body poses here: the redirect body applies at the
// walk's reach instead (vindWalk).
func (s *State) dmgBodyWalk(k *prep, r *dmgRec, fe *fzEv) (b uint8, host Slot, hc uint8, ok bool) {
	if s.replBodyEvN(r, fe) <= 0 {
		return 0, NoSlot, 0, false
	}
	srcCtrl := s.perms[fe.src].ctrl
	rcp := uint8(255)
	var sl Slot = NoSlot
	if fe.dst == NoSlot {
		rcp = fe.apl
	} else {
		sl = fe.dst
		rcp = s.perms[fe.dst].ctrl
	}
	host, line, hits := s.dmgBodyScan(k, srcCtrl, fe.src, fe.card, rcp, sl, true, true)
	if hits == 0 {
		return 0, NoSlot, 0, false // gone (a recomputed pass's other replacement removed it), or no line
	}
	if hits > 1 {
		s.capFull(StallReplPlus) // two lines: gorge asks their CR 616.1 order
		return 0, NoSlot, 0, false
	}
	if c := s.replOrderCandsFz(k, fe); c != 0 {
		s.capFull(StallReplPlus) // a replacement beside the ask body: gorge asks their order
		return 0, NoSlot, 0, false
	}
	body := int(uint16(line.A))&0xff | int(uint16(line.C))&(kop.ReplDmgInsSac|kop.ReplDmgInsVind)
	b = bodyAskOf(body)
	hc = s.perms[host].ctrl
	if b != bodySac || s.sacNoAsk(k, hc, int(fe.n), true) {
		return 0, NoSlot, 0, false // not an ask (the walk applies a redirect at its reach, vindWalk), or the body applied at the recomputed pass
	}
	for i := 0; i < len(DmgInsteadStats); i++ { // the body's own counters (the pool-reach proof)
		if body&(1<<i) != 0 {
			DmgInsteadStats[i].Add(1)
		}
	}
	return b, host, hc, true
}

// vindWalk applies a frozen walk's redirect-body event at the walk's reach of
// it: gorge's replacement runs when the event is proposed (the reflexive
// trigger queued, the damage prevented or standing), the pass's remaining
// events dealt under it. stands: the walk deals the event (dealCombatEvent);
// prevented: the walk takes it out of the pass's accounting (preventFrozenEv).
func (s *State) vindWalk(k *prep, r *dmgRec, fe *fzEv) bool {
	if s.replBodyEvN(r, fe) <= 0 {
		return false
	}
	srcCtrl := s.perms[fe.src].ctrl
	rcp := uint8(255)
	var sl Slot = NoSlot
	if fe.dst == NoSlot {
		rcp = fe.apl
	} else {
		sl = fe.dst
		rcp = s.perms[fe.dst].ctrl
	}
	host, line, hits := s.dmgBodyScan(k, srcCtrl, fe.src, fe.card, rcp, sl, true, true)
	if hits != 1 {
		return false
	}
	body := int(uint16(line.A))&0xff | int(uint16(line.C))&(kop.ReplDmgInsSac|kop.ReplDmgInsVind)
	if bodyAskOf(body) != bodyVind {
		return false
	}
	for i := 0; i < len(DmgInsteadStats); i++ { // the body's own counters (the pool-reach proof)
		if body&(1<<i) != 0 {
			DmgInsteadStats[i].Add(1)
		}
	}
	hc := s.perms[host].ctrl
	o := s.perms[host].obj
	s.enqueue(o, s.objs[o].ver, s.cardAt(host), line.Trig, hc, NoObj, hc, fe.n)
	s.replOrderMark(fe.src, fe.dst)
	if body&kop.ReplDmgInsPrevent != 0 && !s.noPrev(k, true) {
		fe.n = 0
		s.preventFrozenEv(k, r, fe) // the damage is prevented
		return true
	}
	for i := 0; i < r.nev; i++ { // CR 615.12 stand: the walk deals the event
		if r.ev[i].src == fe.src && r.ev[i].dst == fe.dst {
			s.dealCombatEvent(k, r, i, fe)
			break
		}
	}
	return true
}

// sacWalk applies a frozen walk's sacrifice-body event whose amount covers
// every permanent its controller has left (gorge startSac's cnt > n fails:
// they are all sacrificed, one batch, no ask) at the walk's reach of it: the
// event is replaced (prevented in the pass's accounting) and the walk deals
// the rest. The ask case is dmgBodyWalk's.
func (s *State) sacWalk(k *prep, r *dmgRec, fe *fzEv) bool {
	if s.replBodyEvN(r, fe) <= 0 {
		return false
	}
	srcCtrl := s.perms[fe.src].ctrl
	rcp := uint8(255)
	var sl Slot = NoSlot
	if fe.dst == NoSlot {
		rcp = fe.apl
	} else {
		sl = fe.dst
		rcp = s.perms[fe.dst].ctrl
	}
	host, line, hits := s.dmgBodyScan(k, srcCtrl, fe.src, fe.card, rcp, sl, true, true)
	if hits != 1 {
		return false
	}
	body := int(uint16(line.A))&0xff | int(uint16(line.C))&(kop.ReplDmgInsSac|kop.ReplDmgInsVind)
	hc := s.perms[host].ctrl
	if bodyAskOf(body) != bodySac || !s.sacNoAsk(k, hc, int(fe.n), true) {
		return false
	}
	for i := 0; i < len(DmgInsteadStats); i++ { // the body's own counters (the pool-reach proof)
		if body&(1<<i) != 0 {
			DmgInsteadStats[i].Add(1)
		}
	}
	s.sacFirst(k, hc, int(fe.n))
	s.replOrderMark(fe.src, fe.dst)
	fe.n = 0 // replaced: preventFrozenEv takes the event off the recomputed pass
	s.preventFrozenEv(k, r, fe)
	return true
}

// dealGoneEvent deals frozen event fe whose source an ask body's sacrifice
// removed at the walk: the recomputed pass no longer holds it (Dralnu
// sacrificing itself loses its own damage to the blocked attacker), and
// gorge deals it as proposed, reading the source's last known information
// (the frozen card, controller, lifelink and deathtouch). A source still
// there is not this case: its event was prevented out of the pass.
func (s *State) dealGoneEvent(k *prep, fe *fzEv) {
	n := int(fe.n)
	if n <= 0 || s.occ.has(fe.src) && s.perms[fe.src].flags&(pfAttacking|pfBlocking) != 0 {
		return
	}
	fe.n = 0
	if fe.dst == NoSlot {
		s.noteDmgTaken(fe.apl, n)  // kw_bloodthirst.go
		s.notePlDamaged(k, fe.apl) // value_oppsdamaged.go
		s.loseLifeDmg(k, fe.apl, n)
		s.plDmgMark(fe.apl, n)
	} else if s.occ.has(fe.dst) {
		p := &s.perms[fe.dst]
		s.touchSl(fe.dst)
		s.dmgPerm(k, fe.dst, p, n)
		if fe.dt {
			p.markDT(1)
		}
	}
	fe.n = int16(n)
}

// replBodyEvN is the recomputed pass's amount for the frozen event fe.
func (s *State) replBodyEvN(r *dmgRec, fe *fzEv) int {
	for i := 0; i < r.nev; i++ {
		if ev := &r.ev[i]; ev.src == fe.src && ev.dst == fe.dst {
			return int(ev.n)
		}
	}
	return 0
}
