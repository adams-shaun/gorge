package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// --- library (lib[libLo:libHi], top at libLo: gorge's Zone(ZLibrary)[0]) ---

func (pl *player) libLen() int { return int(pl.libHi - pl.libLo) }

func (pl *player) libCompact() {
	if pl.libLo == 0 {
		return
	}
	n := copy(pl.lib[:], pl.lib[pl.libLo:pl.libHi])
	pl.libLo, pl.libHi = 0, uint8(n)
}

func (pl *player) libBottom(o Obj) {
	if int(pl.libHi) == len(pl.lib) {
		pl.libCompact()
	}
	pl.lib[pl.libHi] = o
	pl.libHi++
}

func (pl *player) libTop(o Obj) {
	if pl.libLo == 0 {
		n := pl.libHi
		copy(pl.lib[1:n+1], pl.lib[:n])
		pl.libHi++
		pl.lib[0] = o
		return
	}
	pl.libLo--
	pl.lib[pl.libLo] = o
}

// libIndex is o's index from the top of the library (o must be in it).
func (pl *player) libIndex(o Obj) uint8 {
	for i := pl.libLo; i < pl.libHi; i++ {
		if pl.lib[i] == o {
			return i - pl.libLo
		}
	}
	panic("kernel: object not in library")
}

// libInsert puts o at index at from the top (clamped to the bottom).
func (pl *player) libInsert(o Obj, at uint8) {
	if int(at) >= pl.libLen() {
		pl.libBottom(o)
		return
	}
	if int(pl.libHi) == len(pl.lib) {
		pl.libCompact()
	}
	i := pl.libLo + at
	copy(pl.lib[i+1:pl.libHi+1], pl.lib[i:pl.libHi])
	pl.lib[i] = o
	pl.libHi++
}

// zoneFull reports whether p's card zone z has no room for one more object.
func (pl *player) zoneFull(z kop.Zone) bool {
	switch z {
	case kop.ZoneGraveyard:
		return int(pl.graveN) >= len(pl.grave)
	case kop.ZoneExile:
		return int(pl.exileN) >= len(pl.exile)
	case kop.ZoneHand:
		return int(pl.handN) >= len(pl.hand)
	case kop.ZoneLibrary:
		return pl.libLen() >= len(pl.lib)
	}
	return false
}

func (pl *player) libRemove(o Obj) {
	for i := pl.libLo; i < pl.libHi; i++ {
		if pl.lib[i] == o {
			copy(pl.lib[i:pl.libHi-1], pl.lib[i+1:pl.libHi])
			pl.libHi--
			return
		}
	}
	panic("kernel: object not in library")
}

// shuffleLib is gorge's rng.Shuffle over the library in gorge order.
func (s *State) shuffleLib(p uint8) {
	pl := &s.pl[p]
	l := pl.lib[pl.libLo:pl.libHi]
	for i := len(l) - 1; i > 0; i-- {
		j := s.rng.IntN(i + 1)
		l[i], l[j] = l[j], l[i]
	}
}

// --- hand / graveyard / exile: ordered arrays ---

func removeFrom(a []Obj, n *uint8, o Obj) {
	m := int(*n)
	for i := 0; i < m; i++ {
		if a[i] == o {
			copy(a[i:m-1], a[i+1:m])
			*n--
			return
		}
	}
	panic("kernel: object not in zone")
}

// detach takes o out of its current (non-battlefield) zone and returns the
// zone it left (the origin callers need for a graveyard-leave trigger that
// gorge checks after the move).
func (s *State) detach(o Obj) kop.Zone {
	s.touch(o)
	ob := &s.objs[o]
	from := ob.zone
	if s.g.on && from == kop.ZoneGraveyard && s.subs[kop.TrigLeaves].any() {
		s.fireLeavesGraveyard(s.k(), o, ob.card, ob.owner())
	}
	pl := &s.pl[ob.owner()]
	switch from {
	case kop.ZoneLibrary:
		pl.libRemove(o)
	case kop.ZoneHand:
		removeFrom(pl.hand[:], &pl.handN, o)
	case kop.ZoneGraveyard:
		removeFrom(pl.grave[:], &pl.graveN, o)
	case kop.ZoneExile:
		removeFrom(pl.exile[:], &pl.exileN, o)
		ob.of &^= ofAdventure // off an adventure (face_castface.go)
		if s.g.linkHide != 0 {
			s.hideUnlinkObj(o) // a Hideaway card leaves its link (kw_hideaway.go)
		}
		if s.g.nexw != 0 {
			s.exwLeaveExile(o) // its reverse exiled-with link ends (zone_exiledwith.go)
		}
	case kop.ZoneBattlefield:
		panic("kernel: detach of a permanent (use leave)")
	case kop.ZoneSideboard:
		// outside the game: no list to leave (zone_sideboard.go)
	case kop.ZoneCommand:
		// the command zone is a place: no list to leave
	}
	ob.zone = kop.ZoneNone
	return from
}

// place puts o (not in any zone) into a non-battlefield zone of its owner.
func (s *State) place(o Obj, z kop.Zone, bottom bool) {
	s.placeFrom(o, z, bottom, kop.ZoneNone, false)
}

// placeFrom is place with the pre-move zone and whether o was a creature
// when it left the battlefield.
func (s *State) placeFrom(o Obj, z kop.Zone, bottom bool, from kop.Zone, wasCreature bool) {
	var k *prep
	if s.g.on {
		k = s.k()
	}
	s.placeFromK(k, o, z, bottom, from, wasCreature)
}

// placeFromK is placeFrom for a caller that holds the table's prep (k is nil
// off the generic path): the hot movers pass it so the move does not reload it.
func (s *State) placeFromK(k *prep, o Obj, z kop.Zone, bottom bool, from kop.Zone, wasCreature bool) {
	if p := &s.g.pre; p.kind == akCmdZone && p.obj == o && p.state == preCmdZoneOK {
		*p = preAsk{} // the answered CR 903.9b move is happening
	} else if (z == kop.ZoneHand || z == kop.ZoneLibrary) && s.cmdZoneCommander(o) {
		// a move no resolving effect parked: the guard the ticket asks for (a
		// cost, an SBA, a cleanup, or a move path the gate does not reach)
		s.stall(StallLowering)
		return
	}
	if k != nil && k.mayhem {
		s.mObjClr(k, mechMayhem, o) // a new object nobody discarded (CR 400.7)
	}
	s.touch(o)
	ob := &s.objs[o]
	ob.ver++
	ob.slot = NoSlot // off a face-down spell's stack mark (slotFaceDownSpell)
	s.g.zoneTurn.set(o)
	if k != nil {
		s.clearFB(k, o) // a new object: a granted flashback ends
	}
	if ob.of&ofToken != 0 && !s.g.on {
		ob.zone = kop.ZoneNone // a token ceases to exist (CR 111.7)
		return
	}
	pl := &s.pl[ob.owner()]
	if z == kop.ZoneGraveyard && s.g.on {
		if k.gen != nil && k.gen.anyExileAny {
			z = s.exileAnyDest(k, o) // Rest in Peace from any zone (repl_exileonleave.go)
		}
	}
	if z == kop.ZoneGraveyard && s.g.on && !s.g.graveKept && s.graveRedirect(k, ob.card, z) == kop.ZoneLibrary {
		// Progenitus: onto the library top, then its owner shuffles (gorge's
		// ChangeZone object path: one shuffle per replaced move; the top, not
		// the bottom, is what matches gorge's shuffle in lockstep, pair 9856)
		if pl.zoneFull(kop.ZoneLibrary) {
			ob.zone = kop.ZoneNone
			s.capFull(StallZone)
			return
		}
		ob.zone = kop.ZoneLibrary
		pl.libTop(o)
		if s.g.enteredOn {
			s.noteEnter(o, kop.ZoneLibrary, from, wasCreature)
		}
		s.shuffleLib(ob.owner())
		return
	}
	if pl.zoneFull(z) {
		// tokens that left the battlefield together (a wipe of a token
		// army) sit in their owner's zone until the next state-based
		// check (ceaseTokensGen) and can outgrow its MaxDeck array: the
		// insert is dropped and the game stalls at its next step
		// (capFull), as every other full pool does
		ob.zone = kop.ZoneNone
		s.capFull(StallZone)
		return
	}
	ob.zone = z
	switch z {
	case kop.ZoneLibrary:
		if bottom {
			pl.libBottom(o)
		} else {
			pl.libTop(o)
		}
	case kop.ZoneHand:
		pl.hand[pl.handN] = o
		ob.of |= ofInHand
		pl.handN++
	case kop.ZoneGraveyard:
		pl.grave[pl.graveN] = o
		pl.graveN++
	case kop.ZoneExile:
		pl.exile[pl.exileN] = o
		pl.exileN++
		if k != nil && k.gen != nil && k.gen.anyExW {
			s.exwExile(k, o) // the exiled-with link (zone_exiledwith.go)
		}
	case kop.ZoneStack, kop.ZoneNone, kop.ZoneCommand:
		// the command zone is a place: no list to add to (zone_cmdzone.go)
	default:
		panic("kernel: place into unsupported zone")
	}
	if s.g.enteredOn {
		s.noteEnter(o, z, from, wasCreature)
	}
	if from == kop.ZoneGraveyard && k != nil && k.gen != nil && k.gen.anyLeavesGrave {
		s.fireLeavesGraveTo(k, o, z) // trig_leavesgrave.go
	}
}

// draw draws one card for p. Drawing from an empty library loses at once
// (gorge emits PlayerLost from the draw itself), unless p can't draw (CR
// 121.6: gorge replaces the empty-library mark of a player under
// drawForbidden with a Note, replacement.go).
func (s *State) draw(p uint8) {
	s.drawExcl(p, drawChain{})
}

// drawExcl is draw with the draw replacements already applied to this
// draw's lineage (repl_draw.go); the zero chain is an ordinary draw.
func (s *State) drawExcl(p uint8, ch drawChain) {
	pl := &s.pl[p]
	if s.cfgx&cfgxCantDraw != 0 && s.playerBanned(s.k(), kop.StatCantDraw) {
		return // static_cantdraw.go: before the empty-library mark
	}
	if pl.libLo == pl.libHi {
		// the player loses at the next state-based check (CR 704.5b), not
		// mid-resolution (Strix Lookout's draw then its other effects)
		pl.drewEmpty = true
		return
	}
	if s.cfgx&cfgxDrawMore != 0 && s.drawMore(p, ch) {
		return // repl_draw.go: the replacement drew in this draw's place
	}
	if int(pl.handN) >= len(pl.hand) {
		s.capFull(StallZone) // the hand is full of bounced tokens (placeFrom)
		return
	}
	s.g.drawFirst = false
	o := pl.lib[pl.libLo]
	pl.libLo++
	s.touch(o)
	ob := &s.objs[o]
	ob.zone = kop.ZoneHand
	ob.ver++
	pl.hand[pl.handN] = o
	ob.of |= ofInHand
	pl.handN++
	pl.drawnTurn++
	if s.g.enteredOn {
		s.noteEnter(o, kop.ZoneHand, kop.ZoneLibrary, false)
	}
	if s.subs[kop.TrigDrawn].any() || s.cfgx&cfgxGraveDrawn != 0 {
		s.fireDrawn(p)
	}
	if pl.drawnTurn == 1 && s.turn > 0 {
		s.queueMiracle(o) // kw_miracle.go: after the T: lines, as gorge's offerMiracle
	}
}

func (s *State) lose(p uint8) {
	if s.pl[p].lost || s.cantLose(s.k(), p) {
		return
	}
	s.pl[p].lost = true
	s.checkOver()
}

func (s *State) checkOver() {
	if MaxSeats > 2 {
		s.checkOverN()
		return
	}
	l0, l1 := s.pl[0].lost, s.pl[1].lost
	if !l0 && !l1 {
		return
	}
	s.over = true
	s.ctl = ctlOver
	switch {
	case l0 && l1:
		s.win = -1
	case l0:
		s.win = 1
	default:
		s.win = 0
	}
}

// newToken mints a token object of card c owned by p. While the object
// table has room it appends; once it is full it reuses the lowest id of a
// token or copy that has ceased to exist (CR 111.7, 704.5d) and that nothing
// names any more (freeTokenObj, zone_objreuse.go).
func (s *State) newToken(c CardID, p uint8) Obj {
	if int(s.nobj) < MaxObj {
		o := Obj(s.nobj)
		s.nobj++
		s.objs[o] = object{card: c, chosen: 255, of: ofToken, slot: NoSlot, acts: p << objOwnerShift}
		s.touch(o)
		return o
	}
	o := s.freeTokenObj()
	if o == NoObj {
		// every caller checks nearCapacity (objTableFull) first
		panic("kernel: object table full")
	}
	s.reuseObj(o)
	s.objs[o] = object{card: c, chosen: 255, of: ofToken, slot: NoSlot, acts: p << objOwnerShift, ver: s.objs[o].ver + 1}
	s.touch(o)
	return o
}

// objReferenced reports whether a stack entry, pending trigger or any
// longer-lived record still names o (its object index must not be reused
// yet; heldReferenced lists the records beyond the stack).
func (s *State) objReferenced(o Obj) bool {
	for i := 0; i < int(s.sp); i++ {
		e := &s.stack[i]
		if e.obj == o || e.trigObj == o || (e.li != 0 && s.lk(e.li).obj == o) {
			return true // the LKI of a sacrificed cost object (ConditionDefined$ Sacrificed)
		}
	}
	for i := 0; i < int(s.npend); i++ {
		if s.pend[i].obj == o || s.pend[i].trigObj == o {
			return true
		}
	}
	if s.res.active && (s.res.e.obj == o || s.res.e.trigObj == o || (s.res.e.li != 0 && s.lk(s.res.e.li).obj == o)) {
		return true
	}
	if s.g.ndp != 0 && s.dprogReferenced(o) {
		return true // a delayed program's source (trig_delayedspell.go)
	}
	return s.enteredReferenced(o) || s.heldReferenced(o)
}

// enter puts o onto the battlefield under ctrl's control and fires ETB
// triggers. o must already be out of its previous zone; from is that zone
// (kop.ZoneNone when the caller does not know it), recorded for
// Count$ThisTurnEntered_from_.
func (s *State) enter(k *prep, o Obj, ctrl uint8, tapped bool, from kop.Zone) Slot {
	sl := s.enterNF(k, o, ctrl, tapped, from, 0)
	if from == kop.ZoneGraveyard {
		s.fireLeavesGraveyardSelf(k, sl, o) // trig_leftgraveyard.go
	}
	host := NoSlot
	if s.g.entHost != 0 {
		host = s.attachEntering(k, sl) // zone_handmoveaura.go
	}
	if s.anyETB(k) && !(s.g.pre.ordMid && s.g.pre.obj == o) { // a copy waiting on gorge's second order ask fires them once applied (repl_entryclone.go)
		s.fireObj(k, kop.TrigETB, sl, o, ctrl, 0)
	}
	s.attachedEntered(k, sl, host)
	if k.anyRoom && from == kop.ZoneStack {
		s.roomEntryUnlock(k, sl, o) // the cast door's UnlockDoor (face_rooms.go)
	}
	return sl
}

// enterNF is enter without the enters check (an entrant of a batch holds
// it: entBatch).
func (s *State) enterNF(k *prep, o Obj, ctrl uint8, tapped bool, from kop.Zone, face uint8) Slot {
	free := s.occ.not()
	if k.phasing {
		free = s.freeSlots() // a phased-out permanent keeps its slot (kw_phasing.go)
	}
	if !free.any() {
		panic("kernel: battlefield full")
	}
	sl := free.first()
	// CR 614.12: while this object's entry counters are placed, its own
	// general AddCounter replacement does not apply to them (gorge
	// ownEntryCounterExcluded). The marker spans the whole fold: the printed
	// counters below and enterGen's entry window, own/foreign bodies and
	// effect-borne counters.
	savedEntrySelf := s.g.entrySelf
	s.g.entrySelf = uint8(sl) + 1
	s.touch(o)
	ob := &s.objs[o]
	ob.zone = kop.ZoneBattlefield
	ob.slot = sl
	ob.ver++
	s.g.dmgTurn.clr(o) // entering is a new object (CR 400.7); gorge clears WasDealtDamageThisTurn here, phases keep it (kw_phasing.go phaseIn)
	if k.anyRoom {
		s.roomEnter(k, o, from) // a Room's doors as it enters (face_rooms.go)
	}
	if k.anyDayNight && face == 0 && s.g.dayNight&3 == dnNight {
		s.dnEnter(k, o) // CR 702.145b (kw_daynight.go)
	}
	pc := &k.cards[ob.card]
	if s.nkwx != 0 {
		s.dropKwRepX(sl)
	}
	if s.nptx != 0 {
		s.dropPtX(sl)
	}
	s.ts++
	fl := pfSick
	if sl >= s.permHi {
		s.permHi = sl + 1
	}
	s.perms[sl] = perm{obj: o, card: ob.card, ctrl: ctrl, flags: fl, attached: NoSlot, blocking: NoSlot, ts: s.ts}
	s.lstamp[sl] = s.ts // the layer timestamp starts at the entry (static_attachtimestamp.go)
	p := &s.perms[sl]
	if face != 0 {
		// CR 708.2: a 2/2 colourless creature with no name or abilities.
		// The type rides on ovGenTy (typesAt) and the walk derives the
		// rest (ovr.walk), so no hot accessor tests face-down
		// (face_facedown.go)
		p.gty = kop.TCreature | gtyFaceDown
		p.ovr.flags |= ovGenTy
		p.ovr.walk = walkOn
	}
	if face == 0 {
		if tapped || pc.enterTap {
			p.flags |= pfTapped
		}
		if pc.enterCtr != 0 && !(k.gen != nil && k.gen.anyEntryMerge) { // anyEntryMerge: enterGen's window places it (ctr_entryperkind.go)
			s.placeEntryCounters(k, sl, int(pc.enterCK), int(pc.enterCtr), p.ctrl)
		}
		if pc.sagaN != 0 {
			// CR 702.151a: a Saga enters with a lore counter (gorge
			// events.EntryCounterGrants), placed through the counter
			// replacements; the CounterAddedOnce loop below fires the chapters
			s.placeCounters(k, sl, int(kop.CtrLore), 1, p.ctrl, ctrPutBy)
		}
		if pc.types&(kop.TPlaneswalker|kop.TBattle) != 0 {
			if pc.types&kop.TPlaneswalker != 0 {
				s.placeCounters(k, sl, int(kop.CtrLoyalty), int(k.t.Cards[ob.card].Loyalty), p.ctrl, ctrPutBy)
			}
			if pc.types&kop.TBattle != 0 {
				s.enterDefense(k, sl, ob.card) // CR 310.4 (face_battle.go)
			}
		}
	}
	s.occ.set(sl)
	s.ctrlM[ctrl].set(sl)
	if face == 0 {
		s.subscribe(sl, pc, true)
	}
	if k.gen != nil {
		s.enterGen(k, sl, o, from, face)
		if k.gen.anyBattle && face == 0 && pc.types&kop.TBattle != 0 {
			s.enterBattle(k, sl, ctrl) // after enterGen resets perm.chosen
		}
	}
	if face == 0 && k.anyCtrAdded {
		s.ctrNoteEnter(k, sl) // ctr_addedthisturn.go
	}
	if face == 0 && k.gen != nil && s.subs[kop.TrigCounterAddedOnce].any() {
		// the counters it entered with were put on it: gorge's entry
		// counter notice follows the zone change event, but its triggers
		// are queued before the enters triggers (Exemplar of Light before
		// Dazzling Angel, the pair 13397)
		for c := 0; c < int(kop.NumCounterKinds3); c++ {
			if n := p.ctrN(s, c); n > 0 && s.occ.has(sl) && kop.IsCounterKind(c) {
				s.g.lastCK = kop.CounterKind(c)
				s.g.ctrEntry = true
				s.fireObj(k, kop.TrigCounterAddedOnce, sl, o, ctrl, int16(n))
				s.g.ctrEntry = false
			}
		}
	}
	if k.engines {
		s.speedStart(k, SpeedCntStartEntry) // kw_speed.go
	}
	s.g.entrySelf = savedEntrySelf
	return sl
}

// joined stamps p as the newest permanent of its controller's
// battlefield: gorge appends a permanent to its new controller's
// battlefield list on a control change, as on entering. perm.ts is that
// list's order (FeatBattlefield, the payment planner's units, the damage
// division's attackers). The layer timestamp (State.lstamp) keeps its
// value: a control change gives no new CR 613.7 timestamp (CR 613.7d;
// gorge changes Controller only, never Timestamp or LayerStamp).
func (s *State) joined(p *perm) {
	s.ts++
	p.ts = s.ts
}

func (s *State) subscribe(sl Slot, pc *pcard, on bool) {
	m := pc.trigMask
	for m != 0 {
		ev := bits.TrailingZeros32(m)
		m &= m - 1
		if on {
			s.subs[ev].set(sl)
		} else {
			s.subs[ev].clr(sl)
		}
	}
	if pc.hasStat {
		if on {
			s.statM.set(sl)
		} else {
			s.statM.clr(sl)
		}
	}
	if pc.grantStat {
		if on {
			s.grantM.set(sl)
		} else {
			s.grantM.clr(sl)
		}
	}
	if pc.types&kop.TLegendary != 0 {
		if on {
			s.legM.set(sl)
		} else {
			s.legM.clr(sl)
		}
	}
}

// leave takes the permanent at sl off the battlefield to zone z (owner's),
// firing leaves/dies triggers first (look-back: the permanent still sees
// itself). dies reports a creature going to the graveyard.
func (s *State) leave(k *prep, sl Slot, z kop.Zone, bottom bool) {
	if z == kop.ZoneGraveyard {
		z = s.dieZone(k, sl) // Fiery Annihilation / a dies-to-exile replacement
	}
	if s.cmdZoneReplace(k, s.perms[sl].obj, &z, &bottom) {
		return // a CR 903.9b commander move parked before it happens
	}
	if s.res.active && k.gen != nil {
		s.snapResTarget(s.perms[sl].obj) // CR 608.2h: a moved target's LKI (zone_lki.go)
	}
	s.fireLeave(k, sl, z)
	s.removePerm(k, sl, z, bottom)
}

func (s *State) fireLeave(k *prep, sl Slot, z kop.Zone) {
	if k.gen != nil && k.gen.anyPanharm {
		s.panharmDepart(k, sl) // trig_doubling.go
	}
	if k.gen != nil && k.gen.anyUnearth && z != kop.ZoneExile {
		z = s.unearthZone(sl, z) // cast_altfree.go
	}
	p := &s.perms[sl]
	n0 := s.npend
	merged := false
	if k.gen != nil && z == kop.ZoneGraveyard && !s.genLost(k, sl) {
		z = s.graveRedirect(k, p.card, z) // Progenitus does not die, while it has that ability
	}
	if z == kop.ZoneGraveyard {
		cr := s.isCreature(k, sl)
		if k.gen != nil && cr {
			s.g.died++
		}
		if cr && diesLeavesMerged(k) {
			merged = true // the Leaves walk below covers the Dies lines (trig_diesleaves.go)
			s.fireDiesLeaves(k, sl, p.obj, p.ctrl, czAmt(k, z))
		} else if (cr || k.gen != nil && k.gen.anyDiesNC) && (s.subs[kop.TrigDies].any() || k.gen != nil && k.gen.embObjEv&(1<<kop.TrigDies) != 0) {
			s.fireObj(k, kop.TrigDies, sl, p.obj, p.ctrl, 0)
		}
		if k.gen != nil && k.gen.graveDies && cr {
			s.fireGraveDies(k, sl)
		}
	}
	if !merged && (s.subs[kop.TrigLeaves].any() || k.gen != nil && k.gen.embObjEv&(1<<kop.TrigLeaves) != 0) {
		if k.gen != nil && k.gen.anyBatch {
			s.fireLeavesBatched(k, sl, z)
		} else {
			s.fireObj(k, kop.TrigLeaves, sl, p.obj, p.ctrl, czAmt(k, z))
		}
	}
	if s.subs[kop.TrigSacrificed].any() {
		s.sacLeaveOrder(k, p.obj, n0) // one walk over a sacrifice's triggers (trig_sacqueueorder.go)
	}
	if k.gen != nil && k.gen.anyEarthbend && (z == kop.ZoneGraveyard || z == kop.ZoneExile) {
		s.earthbendLeft(k, p.obj) // the earthbend return trigger queues after the dies/leaves triggers
	}
	if k.gen != nil && k.gen.anyLeaveRet && z == kop.ZoneGraveyard {
		s.leaveGraveFired(p.obj) // trig_leavereturn.go
	}
	if k.gen != nil && k.gen.anyDelayDies && z == kop.ZoneGraveyard && s.g.ndp != 0 {
		s.fireDiesProgs(p.obj, p.ctrl) // trig_delayeddies.go: after the dies/leaves triggers
	}
}

func (s *State) removePerm(k *prep, sl Slot, z kop.Zone, bottom bool) {
	if k.gen != nil && k.gen.anyUnearth && z != kop.ZoneExile {
		z = s.unearthZone(sl, z) // cast_altfree.go
	}
	p := &s.perms[sl]
	o := p.obj
	s.mSlClr(k, mechTappedTurn, sl) // the permanent leaves: its tapped-this-turn mark ends (filter_permmarks.go)
	// a permanent that lost its abilities has no "from anywhere" line (CR 613.1f)
	s.g.graveKept = k.gen != nil && k.gen.anyRepl&rbGraveLib != 0 && s.genLost(k, sl)
	// CR 603.10a: the departure-time creature status, read before the
	// permanent is cleared. Every departure records it (not only a death).
	wasCreature := s.g.enteredOn && s.isCreature(k, sl)
	leftCtrl := p.ctrl
	if k.gen != nil && (s.sp|s.npend != 0 || s.res.active) {
		s.settleFollowed(k, o)
	}
	if s.sp|s.npend != 0 || s.res.active || s.buildLive() || s.delaySpellRefs(o) {
		s.settleSrcKw(k, sl, o)
		s.snapSrcDepart(o) // CR 608.2h: its abilities read it as it last existed here (zone_lkisource.go)
	}
	if k.gen != nil && k.gen.anyPanharm {
		p.flags &^= pfPanhGone
	}
	s.touch(o)
	if s.g.nul != 0 {
		s.untapLockLeft(o) // the lock ends with its target (act_untaplock.go)
	}
	if p.flags&pfSick != 0 {
		s.objs[o].of |= ofSickLeft
	} else {
		s.objs[o].of &^= ofSickLeft
	}
	s.g.lingerM.clr(sl)
	s.g.declM.clr(sl)
	s.objs[o].acts &^= objActsMask // CR 400.7: the count ends with the permanent
	if s.g.netAny() {
		s.netDrop(o)      // the lock ends when the permanent leaves (act_lockpermanent.go)
		s.pgDropTarget(o) // so do the Panharmonicon grants it was given (act_grantpanharm.go)
	}
	if k.gen != nil && k.gen.anyMonstrosity {
		s.mSlClr(k, mechMonstrous, sl) // act_monstrosity.go
	}
	if goadOn(k) {
		s.goadLeave(k, sl) // act_goad.go
	}
	if k.gen != nil && k.gen.anySrcMark {
		s.srcMarkLeave(sl) // filter_combatwords.go
	}
	if k.gen != nil && k.gen.anyWhileHost {
		s.hostLeft(k, o) // filter_combatwords.go
	}
	if p.gty&gtyFaceDown != 0 {
		s.clearFaceDown(sl) // CR 708.9: leaving reveals the card (face_facedown.go)
	}
	s.subscribe(sl, &k.cards[p.card], false)
	if p.ovr.flags&ovCopy != 0 {
		s.subscribe(sl, &k.cards[p.ovr.copyOf], false) // the copied card's statics and triggers too (tables): whatever made the copy (a clone op, an as-enters clone, an Aegis)
	}
	if p.attached != NoSlot {
		s.recordAttLKI(o, p.attached)
	}
	// freeze the last bearer into the departed perm's
	// own record (gorge LastBearer / CR 608.2h). While the Aura was still on
	// the battlefield the host was parked in perm.bord by lingerPut.
	host := NoObj
	if p.attached != NoSlot {
		host = s.perms[p.attached].obj
	} else if p.flags&pfLastBear != 0 {
		host = Obj(p.bord)
	}
	if host != NoObj {
		p.chosen = uint8(host)
		p.flags |= pfLastBear
		s.recordLastBearLKI(o, host)
	} else {
		// It left attached to nothing: an earlier departure's bearer is stale
		// (gorge LastBearer = 0, engine/events/apply_zone.go:1325-1327).
		s.dropAttLKI(o)
	}
	s.occ.clr(sl)
	if MaxSeats == 2 {
		s.ctrlM[0].clr(sl)
		s.ctrlM[1].clr(sl)
	} else {
		for q := range s.ctrlM {
			s.ctrlM[q].clr(sl)
		}
	}
	if k.phasing {
		s.phasedHostLeft(sl, o) // its phased-out attachments (kw_phasing.go)
	}
	// unattach whatever was attached to it; an Aura is then put into the
	// graveyard by SBA, Equipment stays unattached.
	if p.flags&pfAttacking != 0 {
		s.nAttackers--
	}
	m := s.occ
	for m.any() {
		q := m.first()
		m = m.rest()
		qp := &s.perms[q]
		if qp.attached == sl {
			qp.attached = NoSlot
			s.touchSl(q)
			if s.g.on {
				s.lingerPut(q, o)
			}
		}
		if qp.blocking == sl {
			// the attacker left: the blocker stays blocking (CR 506.4) but blocks nothing
			qp.blocking = NoSlot
			s.touchSl(q)
		}
		if p.flags&pfBlocking != 0 && p.blocking == q && qp.nblock > 0 {
			qp.nblock--
			s.touchSl(q)
		}
	}
	if p.ck[ctrSlots-1] != 0 {
		s.ctrOverRelease(o)
	}
	ob := &s.objs[o]
	ob.slot = NoSlot
	ob.zone = kop.ZoneNone
	ob.chosen = 255 // gorge clears Object.ChosenType when the permanent leaves the battlefield
	if k.gen != nil {
		s.stashClr(k, o) // a land played from exile loses its stash counter as it leaves (a new object, CR 400.7)
	}
	s.placeFromK(k, o, z, bottom, kop.ZoneBattlefield, wasCreature)
	s.g.graveKept = false
	if s.g.enteredOn {
		s.noteLeftController(o, leftCtrl)
	}
	if s.g.nexw != 0 {
		s.exwSourceLeft(o) // refresh the links this source exiled
	}
	if k.gen != nil {
		s.leaveGen(k, sl, o)
	}
}

// --- triggers ---

// fireObj enqueues every subscribed trigger of event ev whose filter the
// object at slot evSl (object evObj, controlled by evCtrl) matches.
func (s *State) fireObj(k *prep, ev kop.TrigEvent, evSl Slot, evObj Obj, evCtrl uint8, amt int16) {
	if k.gen != nil {
		s.fireObjGen(k, ev, evSl, evObj, evCtrl, amt)
		return
	}
	var ord [MaxPerm]Slot
	n := s.listenerOrder(s.subs[ev].and(s.occ), &ord)
	for _, ls := range ord[:n] {
		lp := &s.perms[ls]
		if lp.ovr.flags&(ovLoseAll|ovLoseAllEOT) != 0 {
			continue
		}
		cd := &k.t.Cards[s.cardAt(ls)]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind != kop.AbTriggered || a.Trig.Event != ev {
				continue
			}
			if a.Trig.Flags&kop.TfSelf != 0 {
				if ls != evSl {
					continue
				}
			} else if evSl != NoSlot && !s.matchPerm(k, a.Trig.Filter, evSl, lp.ctrl, ls) {
				continue
			}
			s.enqueue(lp.obj, s.objs[lp.obj].ver, s.cardAt(ls), uint8(i), lp.ctrl, evObj, evCtrl, amt)
		}
	}
}

// firePlayer enqueues triggers of a player event (life gained, drew, ...).
func (s *State) firePlayer(k *prep, ev kop.TrigEvent, who uint8, amt int16) {
	if k.gen != nil {
		s.firePlayerGen(k, ev, who, amt)
		return
	}
	var ord [MaxPerm]Slot
	n := s.listenerOrder(s.subs[ev].and(s.occ), &ord)
	for _, ls := range ord[:n] {
		lp := &s.perms[ls]
		if lp.ovr.flags&(ovLoseAll|ovLoseAllEOT) != 0 {
			continue
		}
		cd := &k.t.Cards[s.cardAt(ls)]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind != kop.AbTriggered || a.Trig.Event != ev {
				continue
			}
			if !whoseOK(a.Trig.Whose, lp.ctrl, who) {
				continue
			}
			if a.Trig.Flags&kop.TfSecondDraw != 0 && !s.nthDrawOK(&a.Trig, who) {
				continue
			}
			s.enqueue(lp.obj, s.objs[lp.obj].ver, s.cardAt(ls), uint8(i), lp.ctrl, NoObj, who, amt)
		}
	}
}

func whoseOK(w kop.Whose, ctrl, who uint8) bool {
	switch w {
	case kop.WhoseYou:
		return who == ctrl
	case kop.WhoseOpp:
		return who != ctrl
	case kop.WhoseEnchCtrl:
		return false // read with the listener's slot (enchCtrlIs); elsewhere it fails closed
	}
	return true
}

func (s *State) fireDrawn(p uint8) { s.firePlayer(s.k(), kop.TrigDrawn, p, 1) }

func (s *State) enqueue(o Obj, ver uint8, c CardID, ab, ctrl uint8, tobj Obj, tpl uint8, amt int16) {
	if int(s.npend) >= MaxPending {
		if s.g.on {
			s.capFull(StallPending) // a generated table stalls at its next step (stepGen)
			return
		}
		panic("kernel: pending trigger queue full")
	}
	s.pend[s.npend] = pendingTrig{obj: o, srcVer: ver, card: c, ab: ab, ctrl: ctrl, trigObj: tobj, trigPl: tpl, trigAmt: amt}
	s.npend++
}
