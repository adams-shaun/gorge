package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The Ring (CR 701.54), copied from gorge (effects/ring.go effRingTemptsYou,
// events foldRingTemptsYou / foldRingEmblemPush / ringEmblemAbility,
// rules/trigger_match.go checkRingEmblemTriggers, rules/combat
// RingBearerOutpowers, trigmatch ringTemptsMatches). Cold and out of line: a
// table with no ExtRingTemptsYou never takes these paths (genPrep.anyRing),
// and without one nobody has a Ring-bearer, so every read sees none.
//
// State: two cold regions of MaxSeats entries each, so State does not grow:
// coldRing the seat's tempted count (gorge Player.RingTempted, saturating at
// 255; the levels read it only as >= 1..4) and coldRingBearer its
// Ring-bearer's object + 1 as an objWord (0 none; gorge Player.RingBearer).
// Both are in the state hash (marks.go).
//
//   - 701.54a: ExtRingTemptsYou counts one temptation and the player chooses a
//     creature they control as their Ring-bearer: the creatures in battlefield
//     order, none (the count still rises, 701.54d: the bearer becomes nobody),
//     one (designated unasked: gorge's strict-supersets rule) or several (the
//     raRingBearer ask, ChGenRingBearer, the current bearer first when it is
//     still theirs, gorge's default). TrigRingTemptsYou then fires for the
//     battlefield listeners, the bearer as the trigger's object.
//   - 701.54b: the designation ends when the creature leaves the battlefield
//     (ringLeft, removePerm) or another player gains control of it (ringCtrl,
//     ctrlMove).
//   - 701.54c, emblem ability 1: the bearer is legendary (ringTy, the
//     tyPost switch: gorge's layer-4 effect) and can't be blocked by creatures
//     with greater power (ringOutpowers, canBlock).
//   - 701.54c, levels 2-4 are source-less triggers the emblem's owner controls
//     (abDelayed entries with obj NoObj and trigAmt ringLootAmt / ringBlkAmt /
//     ringHitAmt, the monarch's shape; gorge RingEmblemPush):
//     2 whenever the bearer attacks, its controller draws a card then discards a
//       card (ringAttack, after the declaration's listeners);
//     3 whenever the bearer becomes blocked by a creature (one trigger per
//       blocker), that creature's controller sacrifices it at end of combat: the
//       resolution registers one end-of-combat record (dlyEndCombat, as the
//       token riders do: gorge's incarnation-tracked __kwDecayedSacrifice still
//       puts its trigger on the stack over a blocker that left, and resolves
//       doing nothing to the new object, CR 400.7; resolveSacUntracked's version
//       check);
//     4 whenever the bearer deals combat damage to a player, each opponent loses
//       3 life (ringHit, per combat damage pass, inside the damage trigger walk).

// ChGenRingBearer is the Ring-bearer ask's ChoiceKind: source = the creature
// chosen (gorge's "ring_bearer" KChoose option). A free ordinal below the
// explicit tail.
const ChGenRingBearer ChoiceKind = 214

const (
	ringLootAmt = -5 // the abDelayed entry's trigAmt marking the level-2 loot trigger (the monarch's are -3 and -4)
	ringBlkAmt  = -6 // ... the level-3 trigger; trigObj is the blocker, srcVer its incarnation when it became blocked
	ringHitAmt  = -7 // ... the level-4 trigger

	ringLvLoot = 2 // the tempted count each level needs
	ringLvBlk  = 3
	ringLvHit  = 4
)

// DebugNoRingLevels (TestRingTemptsLockstepMutation): the Ring's triggered
// levels 2-4 never fire.
var DebugNoRingLevels bool

// ringCold is the seats' tempted counts, or nil when the table never tempts.
func (s *State) ringCold() []uint8 { return coldRing.of(s) }

// ringBearers is the seats' Ring-bearers (object + 1, 0 none), or nil when the
// table never tempts.
func (s *State) ringBearers() []objWord { return coldRingBearer.of(s) }

// ringSeats is the length of both Ring regions: one entry per seat when the
// table tempts.
func ringSeats(g *genPrep) int {
	if g.anyRing {
		return MaxSeats
	}
	return 0
}

var (
	coldRing       = newCold[uint8]("ring", ringSeats)
	coldRingBearer = newCold[objWord]("ringBearer", ringSeats)
)

// prepRing sets anyRing: some program (or emblem) lets the Ring tempt a
// player.
func prepRing(t *kop.Table, g *genPrep) {
	has := func(a *kop.Ability) bool {
		for i := range a.Prog {
			if in := &a.Prog[i]; in.Op == kop.OpExt && in.Ext() == kop.ExtRingTemptsYou {
				return true
			}
		}
		return false
	}
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if has(&t.Cards[i].Abilities[j]) {
				g.anyRing = true
			}
		}
	}
	for i := range t.Emblems {
		for j := range t.Emblems[i].Abilities {
			if has(&t.Emblems[i].Abilities[j]) {
				g.anyRing = true
			}
		}
	}
	g.anyTokEndCombat = g.anyTokEndCombat || g.anyRing // the level-3 sacrifice rides the end-of-combat records (trig_tokenateot.go)
}

// ringCount is how many times the Ring has tempted p (gorge RingTempted).
func (s *State) ringCount(p uint8) int {
	if c := s.ringCold(); c != nil && int(p) < MaxSeats {
		return int(c[p])
	}
	return 0
}

// exRingTempted is ExRingTempted: the controller's tempted count.
func (s *State) exRingTempted(ctx evalCtx) int { return s.ringCount(ctx.ctrl) }

// ringBearerObj is p's Ring-bearer, NoObj when p has none.
func (s *State) ringBearerObj(p uint8) Obj {
	if b := s.ringBearers(); b != nil && int(p) < MaxSeats && b[p] != 0 {
		return Obj(b[p] - 1)
	}
	return NoObj
}

// ringBearerSlot is the battlefield slot of p's Ring-bearer, NoSlot when p has
// none (the designation is cleared as the creature leaves or changes
// controller, so a held one is on the battlefield under p).
func (s *State) ringBearerSlot(p uint8) Slot {
	if o := s.ringBearerObj(p); o != NoObj && s.objs[o].zone == kop.ZoneBattlefield {
		return s.objs[o].slot
	}
	return NoSlot
}

// isRingbearer is PredRingbearer for the permanent at sl (gorge IsRingBearer:
// its controller's Ring-bearer).
func (s *State) isRingbearer(sl Slot) bool {
	b := s.ringBearers()
	if b == nil {
		return false
	}
	p := s.perms[sl].ctrl
	return int(p) < MaxSeats && b[p] == objWord(s.perms[sl].obj)+1
}

// ringSlots is the battlefield's Ring-bearers (tyPostSlots: refreshTy keeps
// their legendary type).
//
//go:noinline
func (s *State) ringSlots() (m slotMask) {
	for p := uint8(0); p < MaxSeats; p++ {
		if sl := s.ringBearerSlot(p); sl != NoSlot {
			m = m.with(sl)
		}
	}
	return m
}

// ringTy is the Ring emblem's first ability's type half: a Ring-bearer is
// legendary (CR 701.54c).
func (s *State) ringTy(sl Slot, t kop.TypeMask) kop.TypeMask {
	if s.isRingbearer(sl) {
		return t | kop.TLegendary
	}
	return t
}

// ringLeft ends the designation naming o as it leaves the battlefield (CR
// 701.54b; gorge clearRingBearers).
func (s *State) ringLeft(o Obj) {
	if b := s.ringBearers(); b != nil {
		for p := range b {
			if b[p] == objWord(o)+1 {
				b[p] = 0
			}
		}
	}
}

// ringCtrl ends the designation naming o when player to, who is not the seat
// that designated it, gains control of it (CR 701.54b; gorge foldControlChange).
func (s *State) ringCtrl(o Obj, to uint8) {
	if b := s.ringBearers(); b != nil {
		for p := range b {
			if b[p] == objWord(o)+1 && uint8(p) != to {
				b[p] = 0
			}
		}
	}
}

// ringOutpowers is the Ring emblem's evasion: the attacker a is its
// controller's Ring-bearer and blocker b's power is greater than a's (gorge
// combat.RingBearerOutpowers).
func (s *State) ringOutpowers(k *prep, b, a Slot) bool {
	if !s.isRingbearer(a) {
		return false
	}
	pb, _ := s.ptGen(k, b)
	pa, _ := s.ptGen(k, a)
	return pb > pa
}

// ringEligible writes the creatures p controls in battlefield order, the
// bearer's candidates (gorge effRingTemptsYou's eligible list).
func (s *State) ringEligible(k *prep, p uint8, buf *[MaxPerm]Slot) int {
	var ord [MaxPerm]Slot
	n := s.battlefieldOrder(p, s.ctrlM[p].and(s.occ), ord[:], 0)
	m := 0
	for _, sl := range ord[:n] {
		if s.isCreature(k, sl) {
			buf[m] = sl
			m++
		}
	}
	return m
}

// ringDefault is the deterministic bearer choice (gorge ringDefaultBearer):
// the player's current Ring-bearer while they still control it on the
// battlefield (it need not still be a creature), else the first creature in
// battlefield order; NoSlot when neither exists.
func (s *State) ringDefault(k *prep, p uint8) Slot {
	if sl := s.ringBearerSlot(p); sl != NoSlot && s.perms[sl].ctrl == p {
		return sl
	}
	var buf [MaxPerm]Slot
	if s.ringEligible(k, p, &buf) > 0 {
		return buf[0]
	}
	return NoSlot
}

// ringTempts is ExtRingTemptsYou (gorge effRingTemptsYou): the player Obj is
// tempted. It reports ok false when it suspended on the bearer ask.
//
//kmanifest:ext ExtRingTemptsYou
func (s *State) ringTempts(k *prep, in *kop.Instr) (ok, done bool) {
	p := s.playerRef(k, in.Obj)
	if p == 255 || s.ringCold() == nil {
		return true, true
	}
	var buf [MaxPerm]Slot
	switch n := s.ringEligible(k, p, &buf); n {
	case 0:
		s.ringDesignate(k, p, NoSlot) // CR 701.54d: the temptation still counts
	case 1:
		s.ringDesignate(k, p, buf[0])
	default:
		s.res.ask, s.res.askP = raRingBearer, p
		return false, true
	}
	return true, true
}

// ringDesignate is one temptation's fold (gorge foldRingTemptsYou): the count
// rises, sl (NoSlot: nobody) becomes p's Ring-bearer, and the Mode$
// RingTemptsYou triggers fire.
func (s *State) ringDesignate(k *prep, p uint8, sl Slot) {
	c, b := s.ringCold(), s.ringBearers()
	if c[p] < 255 {
		c[p]++
	}
	bearer := NoObj
	b[p] = 0
	if sl != NoSlot {
		bearer = s.perms[sl].obj
		b[p] = objWord(bearer) + 1
		s.touchSl(sl)
	}
	var ord [MaxPerm]Slot
	n := s.listenerOrder(s.occ, &ord)
	isRing := func(a *kop.Ability) bool { return a.Trig.Event == kop.TrigRingTemptsYou }
	for _, ls := range ord[:n] {
		s.slotTriggers(k, ls, isRing, func(l trigListener, a *kop.Ability) {
			t := &a.Trig
			if !whoseOK(t.Whose, l.ctrl, p) {
				return
			}
			if t.Filter != 0 && (bearer == NoObj || !s.matchPerm(k, t.Filter, sl, l.ctrl, l.sl)) {
				return // ValidCard$ names the chosen Ring-bearer; with none the trigger never fires (CR 701.54d)
			}
			if !s.trigOK(k, l, a) {
				return
			}
			s.queueTrig(k, l, a, bearer, p, 0)
		})
	}
}

// ringOptions poses the bearer ask: the creatures p controls in battlefield
// order, the default (a bearer still theirs) first (gorge's option order).
//
//kmanifest:askopts raRingBearer
func (s *State) ringOptions(k *prep, d *Decision) {
	p := s.res.askP
	var buf [MaxPerm]Slot
	n := s.ringEligible(k, p, &buf)
	d.Sub = ChGenRingBearer
	def := s.ringDefault(k, p)
	for _, sl := range buf[:n] {
		if sl == def {
			d.add(MakeAction(AkChoose, TCode(s.perms[sl].obj), 0, 0))
		}
	}
	for _, sl := range buf[:n] {
		if sl != def {
			d.add(MakeAction(AkChoose, TCode(s.perms[sl].obj), 0, 0))
		}
	}
}

// ringAnswer applies the pick: the chosen creature while it is still one
// p controls, else the default (gorge ringAnsweredBearer).
//
//kmanifest:askapply raRingBearer
func (s *State) ringAnswer(k *prep, a Action) {
	p := s.res.askP
	s.res.ask = raNone
	pick := NoSlot
	if a.Kind() == AkChoose {
		if ob := &s.objs[Obj(a.Source())]; ob.zone == kop.ZoneBattlefield && s.perms[ob.slot].ctrl == p && s.isCreature(k, ob.slot) {
			pick = ob.slot
		}
	}
	if pick == NoSlot {
		pick = s.ringDefault(k, p)
	}
	s.ringDesignate(k, p, pick)
	s.continueRes(k, 0)
}

// ringTrigEntry reports whether e is one of the Ring emblem's source-less
// triggers.
func ringTrigEntry(e *stackEntry) bool {
	return e.obj == NoObj && (e.trigAmt == ringLootAmt || e.trigAmt == ringBlkAmt || e.trigAmt == ringHitAmt)
}

// ringLevelOn reports whether the emblem's level triggers can fire for p: its
// count has reached lv and it has a bearer on the battlefield.
func (s *State) ringLevelOn(p uint8, lv int) (Slot, bool) {
	if DebugNoRingLevels || s.pl[SeatIx(p)].lost || s.ringCount(p) < lv {
		return NoSlot, false
	}
	sl := s.ringBearerSlot(p)
	return sl, sl != NoSlot
}

// ringAttack queues the level-2 trigger when a seat's Ring-bearer is among the
// declared attackers (gorge checkRingEmblemTriggers on DeclareAttackers),
// after the declaration's listeners.
//
//go:noinline
func (s *State) ringAttack(k *prep) {
	for p := uint8(0); p < MaxSeats; p++ {
		if sl, ok := s.ringLevelOn(p, ringLvLoot); ok && s.perms[sl].flags&pfAttacking != 0 && s.perms[sl].ctrl == s.active {
			s.enqueue(NoObj, 0, 0, abDelayed, p, NoObj, p, ringLootAmt)
		}
	}
}

// ringBlocked queues one level-3 trigger per blocker declared against a seat's
// Ring-bearer, in declaration order (gorge: one entry per (bearer, blocker)
// pair of the DeclareBlockers event). The trigger remembers the blocker and
// its incarnation (CR 400.7).
//
//go:noinline
func (s *State) ringBlocked(k *prep) {
	var ps [2 * MaxPerm]blkPair
	nb := s.blockPairs(&ps)
	for p := uint8(0); p < MaxSeats; p++ {
		sl, ok := s.ringLevelOn(p, ringLvBlk)
		if !ok {
			continue
		}
		for _, pr := range ps[:nb] {
			if pr.a == sl {
				b := s.perms[pr.b].obj
				s.enqueue(NoObj, s.objs[b].ver, 0, abDelayed, p, b, p, ringBlkAmt)
			}
		}
	}
}

// ringHit queues the level-4 trigger for the creature at src, which dealt
// combat damage to a player this pass, when it is its controller's
// Ring-bearer (gorge checkRingEmblemTriggers on a combat Damage event).
//
//go:noinline
func (s *State) ringHit(src Slot) {
	p := s.perms[src].ctrl
	if sl, ok := s.ringLevelOn(p, ringLvHit); ok && sl == src {
		s.enqueue(NoObj, 0, 0, abDelayed, p, NoObj, p, ringHitAmt)
	}
}

// ringHitLive reports whether some seat's level-4 trigger could fire: the damage
// walk then runs even with no DamageDone listener.
func (s *State) ringHitLive() bool {
	for p := uint8(0); p < MaxSeats; p++ {
		if _, ok := s.ringLevelOn(p, ringLvHit); ok {
			return true
		}
	}
	return false
}

// resolveRingTrig resolves one of the emblem's triggers. A loot that
// suspended on an ask (the draw's dredge ask, the discard pick) leaves
// s.res.ask set, and the answer finishes the trigger itself.
//
//go:noinline
func (s *State) resolveRingTrig(k *prep, e *stackEntry) {
	switch e.trigAmt {
	case ringLootAmt:
		if s.pl[SeatIx(e.ctrl)].lost {
			return
		}
		if !s.dredgeResPose(k, e.ctrl, dredgeResRing, 1) {
			s.ctl = ctlResolve
			return // the dredge ask is posed (kw_dredge.go); its answer finishes the trigger
		}
		s.draw(e.ctrl)
		s.ringLootTail(k, e.ctrl, false)
	case ringBlkAmt:
		// gorge effRingBlockerSacrifice: a blocker still on the battlefield as
		// the object that blocked registers its end-of-combat sacrifice
		ob := &s.objs[e.trigObj]
		if ob.zone != kop.ZoneBattlefield || ob.ver != e.srcVer {
			return
		}
		if s.ndly >= MaxDelayed {
			s.capFull(StallDelayed)
			return
		}
		s.dly[s.ndly] = delayed{act: uint8(kop.OpSacrifice), ctrl: e.ctrl, obj: e.trigObj, ver: objWord(ob.ver), when: dlyEndCombat, creator: NoObj}
		s.ndly++
	case ringHitAmt:
		for p := uint8(0); p < MaxSeats; p++ {
			if p != e.ctrl && !s.pl[SeatIx(p)].lost {
				s.loseLife(k, p, 3)
			}
		}
	}
}

// ringLootTail is the loot's discard half (gorge's Discard Mode$ TgtChoose, 1
// card): the whole hand when it holds no more, else the player picks
// (raRingDiscard). resume: called from the answered dredge ask, which finishes
// the trigger itself.
func (s *State) ringLootTail(k *prep, p uint8, resume bool) {
	pl := &s.pl[p]
	if pl.handN > 1 {
		s.res.ask, s.res.askP = raRingDiscard, p
		s.ctl = ctlResolve
		return
	}
	for pl.handN > 0 {
		s.discardObj(k, pl.hand[0])
	}
	if resume {
		s.finishRingTrig(k)
	}
}

// finishRingTrig ends a source-less trigger whose resolution went through an ask.
func (s *State) finishRingTrig(k *prep) {
	if k.gen != nil && s.parkReturnAsk() {
		return // a discard parked a return's ask (zone_untilleavesbatch.go)
	}
	if s.g.nask != 0 && s.midResAsk(k) {
		return // the answered card's as-enters choice
	}
	s.finishResolve(k, false)
}

// ringDiscardOptions poses the loot's discard: the hand in order.
//
//kmanifest:askopts raRingDiscard
func (s *State) ringDiscardOptions(k *prep, d *Decision) {
	d.Sub = ChDiscard
	pl := &s.pl[s.res.askP]
	for i := 0; i < int(pl.handN); i++ {
		if o := pl.hand[i]; s.objs[o].of&ofCopy == 0 { // the loot's discard has no DiscardValid$ filter
			d.add(MakeAction(AkChoose, TCode(o), 0, 0))
		}
	}
}

// ringDiscardAnswer discards the pick and finishes the trigger.
//
//kmanifest:askapply raRingDiscard
func (s *State) ringDiscardAnswer(k *prep, a Action) {
	s.res.ask = raNone
	s.discardPicks(k, []Obj{Obj(a.Source())}, 0)
	s.finishRingTrig(k)
}
