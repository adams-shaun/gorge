package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Battles / Sieges (CR 310, 704.5v; gorge rules battle*.go). 0 B of State:
//
//   - the defense counters are kop.CtrDefense in perm.ck/cn (the sparse
//     slots; StallCounters when the overflow table is full);
//   - the protector is perm.chosen = protChosen|seat (CR 310.10: in two
//     players the sole opponent, gorge applySiegeProtector poses no ask);
//   - an attacked battle is perm.atk = the battle's Obj, as a planeswalker;
//   - the defeat queue is a mayPlay record with until = kop.DurDefeated
//     (obj/ver = the exiled battle, ctrl = its owner).
//
// Everything is behind prep.anyBattle or merged into a mask a hot path
// already tests.

// protChosen is perm.chosen's high nibble for a recorded protector; the low
// bit is the seat. No colour reader (ch > 4) or subtype reader (ch >=
// len(Subtypes), MaxSubtypes 192) can mistake it for a choice, and the
// default 255 does not match protector's 0xFE mask.
const protChosen = 0xF0

// protector returns the protector recorded on the battle at sl.
func (s *State) protector(sl Slot) (uint8, bool) {
	ch := s.perms[sl].chosen
	return ch & 1, ch&0xFE == protChosen
}

// FeatAttackDefender is the defending player an attack declared at target
// code tgt names (gorge's attacker option Player, CR 508.1b): the player
// itself, a planeswalker's controller, or a battle's protector (CR 310.7).
func (s *State) FeatAttackDefender(tgt uint8) uint8 {
	if isPlayerTarget(tgt) {
		return CodeSeat(tgt)
	}
	if sl := s.objs[tgt].slot; sl != NoSlot {
		if pr, ok := s.protector(sl); ok && s.typesAt(s.k(), sl)&kop.TBattle != 0 {
			return pr
		}
	}
	return s.ctrlOfObj(Obj(tgt))
}

// battleAttackable reports whether the battle at sl may be attacked by the
// active player (gorge battleDefenders: a face-up Battle with a protector
// that is not the attacker). In two players that is a battle the opponent
// protects, whoever controls it.
func (s *State) battleAttackable(k *prep, sl Slot, active uint8) bool {
	pr, ok := s.protector(sl)
	return ok && pr != active && s.typesAt(k, sl)&kop.TBattle != 0
}

// DebugBattleOwnProtector makes a battle protect for its controller instead
// of the sole opponent (mutation testing only: the lockstep must catch it).
var DebugBattleOwnProtector bool

// DebugBattleMarkDamage marks battle damage instead of removing defense
// counters (mutation testing only: the lockstep must catch it).
var DebugBattleMarkDamage bool

// DebugBattleNoDefeatAsk leaves a defeated battle in exile without queueing
// its owner's transformed cast (mutation testing only).
var DebugBattleNoDefeatAsk bool

// defeatPosed marks a DurDefeated record whose ask is on screen. It shares
// mayPlay.kind with the play-permission riders, which no DurDefeated record
// carries (ownWalkerRecord keeps it off every generic play surface).
const defeatPosed uint8 = 1

// defeatDeferred marks a record queued by an SBA that ran inside a step's
// action or resolution (combat damage, a replacement tail, cleanup): gorge's
// startDefeatedCast already ran at the top of that step, so the ask waits for
// the next step. A record queued by the priority check itself is not marked.
const defeatDeferred uint8 = 1 << 1

// bflDefeatCast marks the build of a CR 310.11 transformed cast. The ask is
// answered while the OTHER player holds priority (the owner already passed),
// so gorge's stack push leaves that holder one pass from resolution; the
// kernel's finishBuild would otherwise reset passes to 0 and ask the owner
// again. Bit 8: bit 7 is bflTre since Thousand Moons Smithy (the two
// shared bit 7 and a Treasure-paid cast took the defeat-cast path).
const bflDefeatCast uint16 = 1 << 8

// sfDefeatPass marks a CR 310.11 cast whose owner had already passed (the
// other player held priority at the ask): finishBuild starts the pass count
// at 1 so that holder's next pass resolves. stackEntry.flags bit 3 is free.
const sfDefeatPass uint8 = 1 << 3

// enterBattle gives the battle at sl its entry state after enterGen (which
// resets perm.chosen): the protector (CR 310.10, the sole opponent in two
// players). The defense counters are put with the other entry counters.
func (s *State) enterBattle(k *prep, sl Slot, ctrl uint8) {
	pr := s.nextAlive(ctrl) // two live seats only (stallclass.go prepTwoSeat: a battle)
	if DebugBattleOwnProtector {
		pr = ctrl
	}
	s.perms[sl].chosen = protChosen | pr
}

// enterDefense puts the printed Defense counters on the battle at sl through
// the counter replacements (CR 310.4; gorge EntryCounterGrants).
func (s *State) enterDefense(k *prep, sl Slot, c CardID) {
	p := &s.perms[sl]
	n := int(k.t.Cards[c].Defense)
	if n > 0 {
		s.placeCounters(k, sl, int(kop.CtrDefense), n, p.ctrl, ctrPutBy)
	}
}

// addBattleOffers appends the attack of attacker o on each attackable
// battle: ascending seat, then battlefield order (gorge attackOffersCompute
// walks every seat's battlefield).
func (s *State) addBattleOffers(k *prep, d *Decision, o uint8) {
	var order [MaxPerm]Slot
	n := 0
	for p := uint8(0); p < s.nSeats(); p++ {
		if s.out(p) {
			continue
		}
		var m slotMask
		for q := s.ctrlM[p]; q.any(); q = q.rest() {
			if sl := q.first(); s.battleAttackable(k, sl, s.active) {
				m = m.with(sl)
			}
		}
		n = s.battlefieldOrder(p, m, order[:], n)
	}
	for _, sl := range order[:n] {
		d.add(MakeAction(AkAttack, o, uint8(s.perms[sl].obj), 0))
	}
}

// dmgPerm deals n ordinary damage to the permanent at sl. Each result of
// CR 120.3 that its types call for applies: a planeswalker loses that many
// loyalty counters (120.3c), a battle that many defense counters (120.3h),
// and a creature, or a permanent that is none of these, marks it (120.3e).
// A planeswalker or battle that is also a creature takes both (gorge
// events.foldDamage).
func (s *State) dmgPerm(k *prep, sl Slot, p *perm, n int) {
	if n > 0 {
		s.g.dmgTurn.set(p.obj)   // WasDealtDamageThisTurn survives every result below (gorge events.foldDamage)
		s.recordDmgAmt(k, sl, n) // a reader card's amount (value_dmgamount.go)
	}
	if s.typesAt(k, sl)&(kop.TPlaneswalker|kop.TBattle) != 0 {
		s.dmgCounters(k, sl, p, n)
		if !s.isCreature(k, sl) {
			return
		}
	}
	p.damage = uint8(min(int(p.damage)+n, 255))
}

// dmgCountersIf removes the loyalty or defense counters n infect or wither
// damage costs the creature at sl when it is also a planeswalker or a
// battle (CR 120.3c, 120.3h); the -1/-1 counters are its creature half.
func (s *State) dmgCountersIf(k *prep, sl Slot, n int) {
	if s.typesAt(k, sl)&(kop.TPlaneswalker|kop.TBattle) != 0 {
		s.dmgCounters(k, sl, &s.perms[sl], n)
	}
}

// dmgCounters applies n damage to the walker or battle at sl as counters
// (CR 306.8 loyalty, CR 310.6/120.3 defense).
func (s *State) dmgCounters(k *prep, sl Slot, p *perm, n int) {
	if n > 0 {
		s.g.dmgTurn.set(p.obj) // a walker/battle was dealt damage too (gorge events.foldDamage)
	}
	p.seeCtr(s)
	if s.typesAt(k, sl)&kop.TBattle != 0 {
		if DebugBattleMarkDamage {
			if int(p.damage)+n > 255 {
				p.damage = 255
			} else {
				p.damage += uint8(n)
			}
			s.touchSl(sl)
			return
		}
		p.subCtrN(s, int(kop.CtrDefense), n)
		return
	}
	p.subCtrN(s, int(kop.CtrLoyalty), n)
}

// defeatedBattles is the set of face-up battles with no defense (gorge
// battleZeroDefense: a printed-face Battle at 0 defense).
func (s *State) defeatedBattles(k *prep) (m slotMask) {
	for q := s.occ; q.any(); q = q.rest() {
		sl := q.first()
		if s.typesAt(k, sl)&kop.TBattle != 0 && s.perms[sl].ctrN(s, int(kop.CtrDefense)) == 0 {
			m = m.with(sl)
		}
	}
	return m
}

// defeatQueue records the defeated battle o, just exiled, for its owner's
// transformed-cast ask (CR 310.11; gorge defeatedCasts): only a 2-face
// transforming card whose front face is the battle (gorge emit.go).
func (s *State) defeatQueue(k *prep, o Obj) {
	if DebugBattleNoDefeatAsk {
		return
	}
	ob := &s.objs[o]
	cd := &k.t.Cards[ob.card]
	if ob.zone != kop.ZoneExile || cd.FaceMode != kop.FaceTransform || len(cd.Faces) != 2 || cd.Faces[0] != ob.card {
		return
	}
	if s.mayFull() {
		s.capFull(StallMayPlay)
		return
	}
	kind := uint8(0)
	if s.ctl != ctlPriority && s.ctl != ctlResolve {
		// queued by an SBA inside a step action (combat damage, cleanup):
		// gorge's step-head startDefeatedCast already ran, so this waits for
		// the next step's head, after the priority round. A defeat queued by
		// a resolution's SBA tail is asked before this step's priority.
		kind = defeatDeferred
	}
	*s.mayAt(int(s.g.nmay)) = mayPlay{obj: o, ver: ob.ver, ctrl: ob.owner(), until: kop.DurDefeated, kind: kind}
	s.g.nmay++
}

// wakeDefeat clears the deferred mark on every queued record: gorge asks the
// whole batch at one step head (or after a resolution), not one per step.
func (s *State) wakeDefeat() {
	for i := 0; i < int(s.g.nmay); i++ {
		if s.mayAt(i).until == kop.DurDefeated {
			s.mayAt(i).kind &^= defeatDeferred
		}
	}
}

// firstDefeat is the index of the first DurDefeated record (gorge's
// defeatedCasts queue order: records are appended in exile order), or -1.
func (s *State) firstDefeat() int {
	for i := 0; i < int(s.g.nmay); i++ {
		if s.mayAt(i).until == kop.DurDefeated {
			return i
		}
	}
	return -1
}

// dropMay removes may record i, keeping the queue order.
func (s *State) dropMay(i int) {
	s.mayShiftDown(i)
	s.g.nmay--
}

// defeatAsk poses the owner's CR 310.11 transformed-cast ask for the first
// queued defeated battle (gorge startDefeatedCast, turn.go): it skips stale
// records and cards a static forbids or whose back face has no legal target.
// It returns false when no ask is due, leaving control to the caller.
func (s *State) defeatAsk(k *prep, d *Decision) bool {
	for {
		i := s.firstDefeat()
		if i < 0 {
			return false
		}
		r := *s.mayAt(i)
		if r.kind&defeatDeferred != 0 {
			return false // queued during this step: the next step's head asks
		}
		ob := &s.objs[r.obj]
		cd := &k.t.Cards[ob.card]
		if ob.zone != kop.ZoneExile || ob.ver != r.ver || cd.FaceMode != kop.FaceTransform || len(cd.Faces) != 2 || cd.Faces[0] != ob.card {
			s.dropMay(i) // it changed zones or faces since (CR 400.7): no ask
			continue
		}
		if k.gen.anyCantCast && s.castForbiddenFree(k, r.ctrl, cd.Faces[0], r.obj) {
			s.dropMay(i) // gorge castRestrictedFree reads it front face up
			continue
		}
		if !s.spellTargetsFeasible(k, cd.Faces[1], r.ctrl) {
			s.dropMay(i) // gorge asks only when the back face's targets exist
			continue
		}
		s.mayAt(i).kind = defeatPosed
		s.g.ctlRet = s.ctl
		s.ctl = ctlGenAsk
		return s.genAskOptions(k, d)
	}
}

// defeatPosedOptions fills the ask for the posed DurDefeated record.
func (s *State) defeatPosedOptions(d *Decision) bool {
	i := s.firstDefeat()
	if i < 0 || s.mayAt(i).kind != defeatPosed {
		return false
	}
	r := *s.mayAt(i)
	d.Kind, d.Sub, d.Player = DkChoose, ChDefeatCast, r.ctrl
	d.add(MakeAction(AkChoose, uint8(r.obj), 0, 1))
	d.add(MakeAction(AkChoose, uint8(r.obj), 0, 0))
	return true
}

// applyDefeat applies the posed ask's answer (gorge defeatedCastAnswer): the
// record is consumed either way; yes flips the card to its back face and
// casts it free (CR 310.11/712), bypassing the timing gate.
func (s *State) applyDefeat(k *prep, a Action) bool {
	i := s.firstDefeat()
	if i < 0 || s.mayAt(i).kind != defeatPosed {
		return false
	}
	r := *s.mayAt(i)
	s.dropMay(i)
	s.ctl = s.g.ctlRet
	if a.Mode() == 0 {
		return true // defeat_cast_no: it stays in exile, front face up
	}
	holder := s.prio
	s.prio = r.ctrl
	s.turnFaceCast(k, r.obj, 1)
	s.beginCast(k, r.obj, castFace|castFree, 0)
	s.bld().bfl |= bflDefeatCast
	// The pass count at the ask is the count after the cast: when the other
	// player had already passed (s.passes != 0), the holder's next pass
	// resolves; when it was 0, the cast resets the count and both must pass.
	if s.passes != 0 {
		s.bld().e.flags |= sfDefeatPass
	}
	// The build still belongs to the caster (targets, payment); the priority
	// holder at the ask gets priority back when the cast completes
	// (finishBuild), as gorge's startDefeatedCast does.
	s.g.holdPrio = holder
	return true
}
