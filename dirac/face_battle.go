package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Battles / Sieges (CR 310, 704.5v; gorge rules battle*.go). 0 B of State:
//
//   - the defense counters are kop.CtrDefense in perm.ck/cn (the sparse
//     slots; StallCounters when the overflow table is full);
//   - the protector is perm.chosen = protChosen|seat (CR 310.10: the
//     controller chooses an opponent as it enters; one living opponent is
//     recorded unasked, gorge applySiegeProtector's strict-supersets rule, and
//     more than one is the queued akProtector ask);
//   - an attacked battle is perm.atk = the battle's Obj, as a planeswalker;
//   - the defeat is CR 310.12b's intrinsic Siege trigger: when the last
//     defense counter goes, a kernel-native abSiegeDefeat trigger (the
//     abMadness shape, no CardDef ability) is queued; as it resolves it exiles
//     the Siege and asks its controller the transformed free cast
//     (raDefeatCast, ChDefeatCast; gorge askSiegeDefeat).
//
// Everything is behind prep.anyBattle or merged into a mask a hot path
// already tests.

// protChosen is perm.chosen's high nibble for a recorded protector; the low
// bits, protSeat, are the seat (one bit at two seats, two at four). No colour
// reader (ch > 4) or subtype reader (ch >= len(Subtypes), MaxSubtypes 192) can
// mistake it for a choice, and the default 255 does not match (255 &^
// protSeat is 0xFE or 0xFC).
const protChosen = 0xF0

// protSeat masks the protector's seat under protChosen.
const protSeat = MaxSeats - 1

// protector returns the protector recorded on the battle at sl.
func (s *State) protector(sl Slot) (uint8, bool) {
	ch := s.perms[sl].chosen
	return uint8(ch & protSeat), ch&^protSeat == protChosen
}

// FeatAttackDefender is the defending player an attack declared at target
// code tgt names (gorge's attacker option Player, CR 508.1b): the player
// itself, a planeswalker's controller, or a battle's protector (CR 310.7).
func (s *State) FeatAttackDefender(tgt TCode) uint8 {
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

// DebugBattleNoDefeatAsk leaves a defeated battle in exile without asking its
// controller the transformed cast (mutation testing only).
var DebugBattleNoDefeatAsk bool

// abSiegeDefeat marks the kernel-native Siege defeat trigger (CR 310.12b; no
// CardDef ability): its source and trigger object are the Siege, which
// resolveSiegeDefeat exiles. abCascade (cast.go) sits just below, so every ab
// at or above it stays kernel-native.
const abSiegeDefeat uint8 = 248

// protectorCands is the living opponents of battle controller ctrl who may
// protect it, in seat order from 0 (gorge protectorOpponents over AliveFrom(0)):
// the one eligibility home of the entry ask and the re-choice after a
// protector leaves.
func (s *State) protectorCands(ctrl uint8) seatSeq {
	q := s.aliveFrom(0)
	n := uint8(0)
	for i := uint8(0); i < q.n; i++ {
		if q.s[i] != ctrl {
			q.s[n] = q.s[i]
			n++
		}
	}
	q.n = n
	return q
}

// enterBattle gives the battle at sl its entry state after enterGen (which
// resets perm.chosen): the protector (CR 310.10). One living opponent is
// recorded unasked. With several, a cast battle's controller chose while the
// spell was still on the stack (protectorPreKind, as gorge parks the entry
// move on the ask); any other entry records the first and queues the choice
// (akProtector, answered as the other as-enters asks). The defense counters
// are put with the other entry counters.
func (s *State) enterBattle(k *prep, sl Slot, ctrl uint8) {
	q := s.protectorCands(ctrl)
	pr := ctrl // no living opponent: gorge records no protector
	if q.n > 0 {
		pr = q.s[0]
	}
	if DebugBattleOwnProtector {
		pr = ctrl
	}
	ask := q.n > 1 && !DebugBattleOwnProtector
	if p := &s.g.pre; p.state == 2 && p.kind == akProtector && p.obj == s.perms[sl].obj {
		pr, ask = uint8(p.val)&protSeat, false // asked on the stack
		*p = preAsk{}
	}
	s.perms[sl].chosen = objWord(protChosen | pr)
	if ask {
		s.queueAskAt(sl, akProtector, NoCard, 0, 0)
	}
}

// protectorPreKind is akProtector when card c is a battle spell about to
// resolve under ctrl with more than one living opponent to protect it: gorge
// parks the entry move on the ask (applySiegeProtector), so the kernel poses it
// through the pre-entry ask (preAskFor) with the spell still on the stack.
func (s *State) protectorPreKind(k *prep, c CardID, ctrl uint8) uint8 {
	if k.gen.anyBattle && k.t.Cards[c].Types&kop.TBattle != 0 && s.protectorCands(ctrl).n > 1 && !DebugBattleOwnProtector {
		return akProtector
	}
	return 0
}

// akProtector is the queued as-enters protector ask (CR 310.10).
const akProtector uint8 = 34

// protectorOptions poses the protector ask: one option per living opponent of
// the battle's controller d.Player in seat order (gorge's "protector"
// options), mode = the seat.
func (s *State) protectorOptions(d *Decision) {
	d.Sub = ChGenPlayer
	q := s.protectorCands(uint8(d.Player))
	for i := uint8(0); i < q.n; i++ {
		d.add(MakeAction(AkChoose, 0, 0, q.s[i]))
	}
}

// protectorStore records the answered protector on the battle at sl.
func (s *State) protectorStore(sl Slot, seat uint8) {
	s.perms[sl].chosen = objWord(protChosen | seat)
	s.touchSl(sl)
}

// rechooseProtectors gives each battle protected by seat p, who has just left
// the game, the first living opponent of its controller as protector (CR
// 310.10; gorge rechooseDepartedBattleProtector derives it, never asks). A
// controller with no living opponent keeps the stale protector.
func (s *State) rechooseProtectors(k *prep, p uint8) {
	for m := s.occ; m.any(); m = m.rest() {
		sl := m.first()
		if pr, ok := s.protector(sl); !ok || pr != p || s.typesAt(k, sl)&kop.TBattle == 0 {
			continue
		}
		if q := s.protectorCands(s.perms[sl].ctrl); q.n > 0 {
			s.protectorStore(sl, q.s[0])
		}
	}
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

// battleOffer is the first battle pair addBattleOffers would add for the
// creature at atk (NoSlot, false when none): the same walk, so
// anyAttacker's empty-offer read (cantAtkEveryPair, combat_cantattack.go)
// and declareMustAttack's battle declaration match the decision's own
// enumeration.
func (s *State) battleOffer(k *prep, atk Slot) (Slot, bool) {
	sick := s.atkHasteSick(k, atk)
	for p := uint8(0); p < s.nSeats(); p++ {
		if s.out(p) {
			continue
		}
		if sick && !s.atkHasteOK(k, atk, p) {
			continue // a CanAttackIfHaste exemption names the defender seat (static_canattackifhaste.go)
		}
		for q := s.ctrlM[p]; q.any(); q = q.rest() {
			if sl := q.first(); s.battleAttackable(k, sl, s.active) {
				return sl, true
			}
		}
	}
	return NoSlot, false
}

// someBattleOffers reports whether a battle pair stands for the creature at
// atk (battleOffer's existence half).
func (s *State) someBattleOffers(k *prep, atk Slot) bool {
	_, ok := s.battleOffer(k, atk)
	return ok
}

// addBattleOffers appends the attack of attacker o on each attackable
// battle: ascending seat, then battlefield order (gorge attackOffersCompute
// walks every seat's battlefield).
func (s *State) addBattleOffers(k *prep, d *Decision, o Obj, atk Slot) {
	var order [MaxPerm]Slot
	n := 0
	sick := s.atkHasteSick(k, atk)
	for p := uint8(0); p < s.nSeats(); p++ {
		if s.out(p) {
			continue
		}
		if sick && !s.atkHasteOK(k, atk, p) {
			continue // a CanAttackIfHaste exemption names the defender seat (static_canattackifhaste.go)
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
		d.add(MakeAction(AkAttack, TCode(o), TCode(s.perms[sl].obj), 0))
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
		had := p.ctrN(s, int(kop.CtrDefense))
		p.subCtrN(s, int(kop.CtrDefense), n)
		if had > 0 && n >= had {
			s.queueSiegeDefeat(k, sl) // CR 310.12b: the last defense counter went
		}
		return
	}
	p.subCtrN(s, int(kop.CtrLoyalty), n)
}

// defeatedBattles is the set of face-up battles with no defense that the
// state-based action puts into the graveyard (CR 704.5v; gorge
// battleZeroDefense): a battle that is the source of a defeat trigger that has
// not left the stack stays (siegePending).
func (s *State) defeatedBattles(k *prep) (m slotMask) {
	for q := s.occ; q.any(); q = q.rest() {
		sl := q.first()
		if s.typesAt(k, sl)&kop.TBattle != 0 && s.perms[sl].ctrN(s, int(kop.CtrDefense)) == 0 && !s.siegePending(s.perms[sl].obj) {
			m = m.with(sl)
		}
	}
	return m
}

// siegePending reports whether Siege o is the source of a defeat trigger that
// has triggered and not yet left the stack: queued or on it (gorge
// siegeDefeatPending keys on the object alone).
func (s *State) siegePending(o Obj) bool {
	for i := 0; i < int(s.npend); i++ {
		if pt := &s.pend[i]; pt.ab == abSiegeDefeat && pt.obj == o {
			return true
		}
	}
	for i := 0; i < int(s.sp); i++ {
		if e := &s.stack[i]; e.kind != seSpell && e.ab == abSiegeDefeat && e.obj == o {
			return true
		}
	}
	return false
}

// queueSiegeDefeat queues the defeat trigger of the Siege at sl, whose last
// defense counter was just removed (gorge queueSiegeDefeat): only a 2-face
// transforming card whose front face is the battle.
func (s *State) queueSiegeDefeat(k *prep, sl Slot) {
	p := &s.perms[sl]
	cd := &k.t.Cards[p.card]
	if cd.FaceMode != kop.FaceTransform || len(cd.Faces) != 2 || cd.Faces[0] != p.card {
		return
	}
	s.enqueue(p.obj, s.objs[p.obj].ver, p.card, abSiegeDefeat, p.ctrl, p.obj, p.ctrl, 0)
}

// resolveCastTrigger resolves the kernel-native keyword triggers whose
// resolution asks a cast: Madness (kw_madness.go), Miracle (kw_miracle.go) and
// the Siege defeat. false: ab is none of them.
func (s *State) resolveCastTrigger(k *prep, ab uint8) bool {
	switch ab {
	case abMadness:
		s.resolveMadness(k)
	case abMiracle:
		s.resolveMiracle(k)
	case abSiegeDefeat:
		s.resolveSiegeDefeat(k)
	default:
		return false
	}
	return true
}

// resolveSiegeDefeat resolves the defeat trigger (gorge askSiegeDefeat): the
// Siege is exiled, then its controller is asked whether to cast it
// transformed without paying its mana cost. A Siege that is no longer the
// permanent that triggered it does nothing; an exiled one whose cast a static
// forbids, or whose back face has no legal target, is not offered.
//
//go:noinline
func (s *State) resolveSiegeDefeat(k *prep) {
	e := &s.res.e
	ob := &s.objs[e.obj]
	if ob.zone != kop.ZoneBattlefield || ob.ver != e.srcVer {
		s.finishResolve(k, false)
		return
	}
	s.leave(k, ob.slot, kop.ZoneExile, false)
	cd := &k.t.Cards[ob.card]
	if DebugBattleNoDefeatAsk || ob.zone != kop.ZoneExile || cd.FaceMode != kop.FaceTransform || len(cd.Faces) != 2 || cd.Faces[0] != ob.card {
		s.finishResolve(k, false)
		return
	}
	if k.gen.anyCantCast && s.castForbiddenFree(k, e.ctrl, cd.Faces[0], e.obj) {
		s.finishResolve(k, false) // gorge castRestrictedFree reads it front face up
		return
	}
	if !s.spellTargetsFeasible(k, cd.Faces[1], e.ctrl) {
		s.finishResolve(k, false) // gorge asks only when the back face's targets exist
		return
	}
	s.res.ask, s.res.askP = raDefeatCast, e.ctrl
	s.ctl = ctlResolve
}

// defeatCastOptions fills the transformed-cast ask (gorge's KChoose
// defeat_cast_yes / defeat_cast_no).
//
//kmanifest:askopts raDefeatCast
func (s *State) defeatCastOptions(k *prep, d *Decision) {
	d.Sub = ChDefeatCast
	d.add(MakeAction(AkChoose, TCode(s.res.e.obj), 0, 1))
	d.add(MakeAction(AkChoose, TCode(s.res.e.obj), 0, 0))
}

// defeatCastApply applies the ask's answer (gorge tapeSiegeDefeatAnswer): the
// trigger leaves the stack, and a yes flips the exiled card to its back face
// and casts it free (CR 310.12b/712), bypassing the timing gate, with the
// priority holder the resolution left getting priority after (CR 117.3b).
//
//kmanifest:askapply raDefeatCast
func (s *State) defeatCastApply(k *prep, a Action) {
	s.res.ask = raNone
	o, p := s.res.e.obj, s.res.e.ctrl
	s.finishResolve(k, false)
	if a.Mode() == 0 || s.objs[o].zone != kop.ZoneExile {
		return // defeat_cast_no: it stays in exile, front face up
	}
	holder := s.prio
	s.prio = p
	s.turnFaceCast(k, o, 1)
	s.beginCast(k, o, castFace|castFree, 0)
	s.prio = holder
}

// dropMay removes may record i, keeping the queue order.
func (s *State) dropMay(i int) {
	s.mayShiftDown(i)
	s.g.nmay--
}
