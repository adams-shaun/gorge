package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// exec runs one instruction. It returns false when the op suspended on a
// decision (res.ask set); the op is then completed by applyResolve.
func (s *State) exec(k *prep, in *kop.Instr) bool {
	if k.gen != nil {
		if ok, done := s.execGen(k, in); done {
			return ok
		}
	}
	e := &s.res.e
	switch in.Op {
	case kop.OpNop:
	case kop.OpDealDamage:
		code := s.refCode(k, in.Obj)
		if code != NoTarget {
			s.damageTo(k, code, s.value(k, in.A))
		}
	case kop.OpDraw:
		p1, p2 := s.players(k, in.Obj)
		n := s.value(k, in.A)
		for _, p := range [2]uint8{p1, p2} {
			if p == 255 {
				continue
			}
			for i := 0; i < n && !s.over; i++ {
				if in.C&kop.MvRemember != 0 {
					s.drawRemember(p, in.C&kop.DrawRememberAll != 0) // RememberDrawn$ (zone_rememberwrites.go)
				} else {
					s.draw(p)
				}
			}
		}
	case kop.OpDestroy:
		if sl := s.objRef(k, in.Obj); sl != NoSlot {
			if in.C&2 != 0 {
				s.destroyRemember(k, sl, in.C) // RememberDestroyed$ (act_remember.go)
				break
			}
			if in.C&1 != 0 {
				s.destroyNoRegen(k, sl)
				break
			}
			s.destroy(k, sl)
		}
	case kop.OpExile:
		s.moveRef(k, in.Obj, kop.ZoneExile, false)
	case kop.OpBounce:
		s.moveRef(k, in.Obj, kop.ZoneHand, false)
	case kop.OpToLibrary:
		switch in.C {
		case kop.LibTopShuffle, kop.LibTopShuffleGrave:
			s.toLibraryShuffle(k, in, false)
		case kop.LibBottomShuffle:
			s.toLibraryShuffle(k, in, true)
		case kop.LibTopShuffleSelf:
			s.toLibraryShuffleSelf(k, in)
		case kop.LibThird:
			s.toLibraryThird(k, in) // act_lockpermanent.go
		case kop.LibBottomRememberGrave:
			s.toLibraryRemember(k, in) // zone_tolibraryremember.go
		case kop.LibTopGrave, kop.LibBottomGrave:
			s.toLibraryGrave(k, in) // zone_tolibrarygrave.go
		case kop.LibBottomRememberBF:
			s.toLibraryRememberBF(k, in) // zone_tolibrarybfremember.go
		case 0:
			s.moveRef(k, in.Obj, kop.ZoneLibrary, false)
		case 1:
			s.moveRef(k, in.Obj, kop.ZoneLibrary, true)
		default:
			o := s.objRefObj(in.Obj)
			if o != NoObj {
				s.moveRef(k, in.Obj, kop.ZoneLibrary, true)
				if s.res.ask != raGenCmdZone {
					s.shuffleLib(s.objs[o].owner())
				}
			}
		}
	case kop.OpPumpUntilEOT:
		if sl := s.objRef(k, in.Obj); sl != NoSlot {
			s.touchSl(sl)
			if !s.eotAdd(k, sl, s.value(k, in.A), s.value(k, in.B)) {
				break
			}
		}
	case kop.OpGrantKeyword:
		if sl := s.objRef(k, in.Obj); sl != NoSlot {
			s.grantEOT(sl, kop.KwMask(uint16(in.C))|kop.KwMask(uint16(in.B.N))<<16)
			s.promoKw(k, sl, kop.KwMask(uint16(in.C))|kop.KwMask(uint16(in.B.N))<<16)
			s.touchSl(sl)
		}
	case kop.OpPutCounter:
		if !s.execPutCounter(k, in) {
			return false // the CR 616.1 order ask (repl_ctrorder.go)
		}
	case kop.OpRemoveCounter:
		if sl := s.objRef(k, in.Obj); sl != NoSlot {
			n := s.value(k, in.A)
			s.touchSl(sl)
			s.perms[sl].seeCtr(s) // gorge lists a kind removed to none at 0
			s.perms[sl].subCtrN(s, int(in.C), n)
		}
	case kop.OpCreateToken:
		p := s.playerRef(k, in.Obj)
		if p == 255 {
			p = e.ctrl
		}
		tc, n, ok := s.replTokensCard(k, p, CardID(in.C), s.value(k, in.A))
		if !ok {
			return true // StallTokRewrite
		}
		for i := 0; i < n; i++ {
			if s.nearCapacity(k) {
				s.stall(StallBoard)
				return true
			}
			o := s.newToken(tc, p)
			sl := s.enter(k, o, p, in.B.N&kop.TrTapped != 0, kop.ZoneLibrary)
			if in.B.N&kop.TrHasteEOT != 0 {
				s.grantEOT(sl, kop.KwHaste)
				s.promoKw(k, sl, kop.KwHaste)
				s.touchSl(sl)
			}
			s.res.lastTok = o
		}
	case kop.OpGainLife:
		p1, p2 := s.players(k, in.Obj)
		n := s.value(k, in.A)
		if p1 != 255 {
			s.gainLife(k, p1, n)
		}
		if p2 != 255 {
			s.gainLife(k, p2, n)
		}
	case kop.OpLoseLife:
		p1, p2 := s.players(k, in.Obj)
		n := s.value(k, in.A)
		lost := 0
		if p1 != 255 {
			s.loseLife(k, p1, n)
			lost += max(n, 0)
		}
		if p2 != 255 {
			s.loseLife(k, p2, n)
			lost += max(n, 0)
		}
		if k.gen != nil {
			s.g.lostAmt = int16(lost) // ExLastLifeLost (Exsanguinate's "the life lost this way")
		}
	case kop.OpDiscard:
		if in.Obj == kop.RefEachPlayer && in.F == 0 && in.C == kop.ChooseSelf {
			// each player discards, controller first (gorge effDiscard's
			// Defined$ Player loop; act_eachdiscard.go)
			return s.startEachDiscard(k, in, s.res.e.ctrl)
		}
		p := s.playerRef(k, in.Obj)
		if p == 255 {
			break
		}
		n := s.value(k, in.A)
		if in.C&kop.OptAsk != 0 { // Optional$ (act_optionalask.go)
			if !s.optDiscard(k, in, p, n) {
				return false
			}
			break
		}
		if n < 1 {
			// gorge discardBounds keeps a resolved zero at zero: an X of 0
			// (Last Rites, Death Cloud) discards nothing and asks nothing
			break
		}
		pl := &s.pl[p]
		if in.C == kop.ChooseRandom {
			// gorge effDiscardRandom draws once per pick even when the
			// pick is forced (one eligible card), so this arm must run
			// before the zero-draw whole-hand shortcut below
			// (act_discardrandom.go).
			s.discRandom(k, p, n)
			break
		}
		if n >= s.handDiscardable(p) {
			for i := uint8(0); i < pl.handN; {
				if s.objs[pl.hand[i]].of&ofCopy != 0 || s.park(k, pl.hand[i], parkDiscard, 0) {
					i++
					continue
				}
				s.discardObj(k, pl.hand[i])
			}
			break
		}
		s.res.ask = raDiscard
		s.res.peekI = 0
		s.res.askP = p
		if in.C == kop.ChooseOpp {
			s.res.askP = e.ctrl // the source's controller chooses (gorge discardChooser)
		}
		s.res.left = uint8(n)
		s.res.chosen = p
		return false
	case kop.OpScry, kop.OpSurveil:
		p := s.playerRef(k, in.Obj)
		if p == 255 {
			break
		}
		n := s.value(k, in.A)
		amt := n // the instruction's own count: 0 is no scry/surveil
		pl := &s.pl[p]
		if n > pl.libLen() {
			n = pl.libLen()
		}
		if n > len(s.res.peek) {
			s.capFull(StallTables) // loud: res.peek holds eight
			break
		}
		if k.gen != nil && (k.gen.anyScryTrig || k.gen.anyScryTally) {
			s.scrySurveilStart(k, in.Op, p, n, amt)
		}
		if n <= 0 {
			break
		}
		for i := 0; i < n; i++ {
			s.res.peek[i] = pl.lib[int(pl.libLo)+i]
		}
		s.res.peekN, s.res.peekI, s.res.peekSel = uint8(n), 0, 0
		s.res.restSel, s.res.topN, s.res.restPh = 0, 0, false
		if in.Op == kop.OpScry {
			s.res.ask = raScryTop
		} else {
			s.res.ask = raSurveil
		}
		s.res.askP = p
		return false
	case kop.OpMill:
		p1, p2 := s.players(k, in.Obj)
		n := s.value(k, in.A)
		if in.C&kop.MvMayMill != 0 && n > 0 {
			return s.mayMill(p1, p2, n)
		}
		for _, p := range [2]uint8{p1, p2} {
			if p == 255 {
				continue
			}
			pl := &s.pl[p]
			if k.gen != nil && k.gen.anyRepl&rbGraveLib != 0 {
				s.millShuffling(k, p, n, in.C&kop.MvRemember != 0)
				continue
			}
			if k.gen != nil && len(s.cold.parked) != 0 && s.parkMills(k, p, n, in.C&kop.MvRemember != 0) {
				continue
			}
			for i := 0; i < n && pl.libLo < pl.libHi; i++ {
				o := pl.lib[pl.libLo]
				pl.libLo++
				s.touch(o)
				s.objs[o].zone = kop.ZoneNone
				s.placeFromK(k, o, kop.ZoneGraveyard, false, kop.ZoneLibrary, false)
				if in.C&kop.MvRemember != 0 {
					s.remember(o) // RememberMilled$ (Sphinx's Tutelage)
				}
			}
		}
	case kop.OpFight, kop.OpBite:
		a := s.objRef(k, in.Obj)
		if in.Obj == kop.RefDelayedRemembered && a != NoSlot && s.reflexStale() {
			a = NoSlot // the reflexive capture changed zones since (zone_rememberwrites.go)
		}
		if in.Op == kop.OpBite && s.biteEachTarget(k, in, a) {
			break
		}
		b := s.objRef(k, kop.Ref(in.C))
		track := in.Op == kop.OpFight && in.A.K == kop.VConst && in.A.N == 1 // ExcessSVar$ (value_fightexcess.go)
		if track {
			s.g.excess = 0 // a fight that does not happen binds nothing
		}
		if a == NoSlot || b == NoSlot || !s.isCreature(k, a) || !s.isCreature(k, b) &&
			(in.Op == kop.OpFight || s.typesAt(k, b)&kop.TPlaneswalker == 0) {
			break // a bite may hit a planeswalker (Bite Down)
		}
		// one simultaneous damage event (gorge effFight's damage batch):
		// both powers are read before either hit (CR 701.14a), each hit's
		// source is its creature (hitBatch)
		var hb hitBatch
		pa := s.power(k, a)
		if DebugDmgAmountPlusOne && in.Op == kop.OpBite {
			pa++
		}
		if in.Op == kop.OpFight && a == b {
			hb.add(s, a, a, 2*pa) // a creature fighting itself (CR 701.14c)
		} else {
			hb.add(s, a, b, pa)
			if in.Op == kop.OpFight {
				hb.add(s, b, a, s.power(k, b))
			}
		}
		lethal := 0
		if track {
			lethal = s.excessLethalGen(k, b) // captured before any hit lands
		}
		// The damage the fight effect sees, after replacement and prevention
		// (CR 120.4a, gorge effFight): gorge's EmitDamage returns the event
		// that actually landed, so a hit parked on the replacement-order ask
		// still reports its post-replacement amount here — the answer
		// re-executes the batch (interp_ask.go) and h.n is the landed damage.
		if parked := s.dealHits(k, hb.h[:hb.n]); !parked && track {
			s.fightExcessRec(lethal, int(hb.h[0].n))
		}
	case kop.OpTap:
		if sl := s.objRef(k, in.Obj); sl != NoSlot {
			was := s.perms[sl].flags & pfTapped
			s.perms[sl].flags |= pfTapped
			s.touchSl(sl)
			if was == 0 && k.gen != nil && s.subs[kop.TrigTaps].any() {
				s.fireTapsBy(k, sl, s.effTapper(sl))
			}
		}
	case kop.OpUntap:
		if sl := s.objRef(k, in.Obj); sl != NoSlot {
			if s.untapBlocked(k, sl) {
				break
			}
			if p := &s.perms[sl]; p.flags&pfTapped != 0 && p.ctrLow(int(kop.CtrStun)) > 0 {
				p.seeCtr(s)
				p.subCtrN(s, int(kop.CtrStun), 1) // a stun counter is removed instead (CR 122.1d)
				s.touchSl(sl)
				break
			}
			s.untapEffect(k, sl)
			s.touchSl(sl)
		}
	case kop.OpSacrifice:
		p := s.playerRef(k, in.Obj)
		if p == 255 {
			break
		}
		n := s.value(k, in.A)
		if in.C&kop.OptAsk != 0 { // Optional$ (act_optionalask.go)
			if !s.optSac(k, in, p, n) {
				return false
			}
			break
		}
		if n <= 0 {
			break // gorge effSacrifice: an Amount$ of 0 or less sacrifices nothing and asks nothing
		}
		src := s.resSrcSlot(k)
		cnt := s.countSac(k, p, in.F, src)
		if cnt <= n {
			if cnt >= 2 {
				s.sacForcedBatch(k, in.F, p, src) // act_repeatplayer.go
				break
			}
			for m := s.ctrlM[p]; m.any(); {
				sl := m.first()
				m = m.rest()
				if s.sacMatch(k, in.F, sl, p, src) {
					s.fireSac(k, sl)
					s.leaveParked(k, sl)
					break // the one match counted above: a later slot is matched against the battlefield as it now is (greatestPower...ByTargeted would pick the next-greatest)
				}
			}
			break
		}
		s.res.ask = raSacrifice
		s.res.askP = p
		s.res.left = uint8(n)
		return false
	case kop.OpCounter:
		if k.gen != nil && k.gen.anyStackTgt && s.counterStackTgt(k, in) {
			break // a stack-ability target (tgt_stackability.go)
		}
		o := s.objRefObj(in.Obj)
		if o != NoObj {
			if i := s.stackIndexOf(o); i >= 0 && (in.C&kop.CtrDestMove != 0 || !(s.stack[i].kind == seSpell && s.uncounterable(k, i))) {
				s.g.stkCt = s.stack[i].ctrl + 1 // "its controller" reads the spell as it last existed (CR 608.2h)
				if in.B.N != 0 {
					s.counterRiders(k, i, int(in.B.N)) // tgt_countershapes.go
				}
				if in.C != 0 {
					s.counterTo(k, i, in.C&^kop.CtrDestMove)
					break
				}
				s.counterAt(k, i)
			}
		}
	case kop.OpAttach:
		src := s.objRef(k, kop.RefSelf)
		dst := s.objRef(k, in.Obj)
		if src != NoSlot && dst != NoSlot {
			s.attachTo(k, src, dst)
			s.touchSl(src)
		}
	case kop.OpCharm:
		if in.C&kop.CharmRepeat != 0 {
			if !s.charmRepeatEnter(k, in.C) {
				s.res.pc += uint8(s.value(k, in.A))
			}
		} else if e.mode&(1<<uint(in.C)) == 0 {
			s.res.pc += uint8(s.value(k, in.A))
		}
	case kop.OpIfKicked:
		if e.flags&sfKicked == 0 {
			s.res.pc += uint8(s.value(k, in.A))
		}
	case kop.OpDamageAll:
		n := s.value(k, in.A)
		if k.gen != nil && s.g.dmgSeqOn {
			s.damageSeqNext(k, n)
			break
		}
		if in.B.N&6 != 0 {
			s.damageAllOpp(k, in, e.ctrl, n) // trig_gravedrawn.go
			break
		}
		src := s.objRef(k, kop.RefSelf)
		var hit slotMask
		for m := s.occ; m.any(); m = m.rest() {
			sl := m.first()
			if s.isCreature(k, sl) && s.matchPerm(k, in.F, sl, e.ctrl, src) {
				hit = hit.with(sl)
			}
		}
		if k.gen != nil && k.gen.anyRepl&(rbDblDmg|rbPrevNC|rbPrevDmgTo|rbPrevCol) != 0 {
			// a sweep's events are dealt one at a time: park on each event two
			// replacements match (CR 616.1) and resume the rest after the
			// answer. The recipients freeze in gorge's sweep order
			// (effDamageAll: seat 0's battlefield in entry order, then seat
			// 1's, then the players).
			s.damageSeqStart(k, hit, in.B.N&1 != 0)
			s.dmgSeqCtrlFirst(e.ctrl)
			s.damageSeqNext(k, n)
			break
		}
		// gorge's effDamageAll sweeps the permanents seat 0's battlefield in
		// entry order first, then seat 1's: damageTo fires per-referent
		// damage triggers as it goes, so the sweep order is the queue order
		// (the replacement path freezes the same order in damageSeqStart).
		var ord [MaxPerm]Slot
		hi := 0
		for p := uint8(0); p < 2; p++ {
			hi = s.battlefieldOrder(p, hit.and(s.ctrlM[p]), ord[:], hi)
		}
		for _, sl := range ord[:hi] {
			s.damageTo(k, uint8(s.perms[sl].obj), n)
		}
		if in.B.N&1 != 0 {
			// then each player, the controller first (gorge validPlayers:
			// AliveFrom the controller)
			s.damageTo(k, PlayerTarget0+e.ctrl, n)
			s.damageTo(k, PlayerTarget0+1-e.ctrl, n)
		}
	case kop.OpDestroyAll:
		if in.C != 0 {
			s.destroyAllRemember(k, in)
			break
		}
		src := s.objRef(k, kop.RefSelf)
		var hit slotMask
		for m := s.occ; m.any(); m = m.rest() {
			sl := m.first()
			if s.matchPerm(k, in.F, sl, e.ctrl, src) && s.kw(k, sl)&kop.KwIndestructible == 0 {
				hit = hit.with(sl)
			}
		}
		hit = s.destroyHealAll(k, hit)
		if s.anyRegen(hit) {
			hit = s.regenAll(hit)
		}
		if k.gen != nil && k.gen.anyRepl&rbShield != 0 {
			hit = s.shieldAll(hit) // CR 122.1c, after regeneration (gorge effDestroyAll)
		}
		s.leaveAll(k, hit, kop.ZoneGraveyard)
	case kop.OpPumpAll:
		if in.Obj != kop.RefNone {
			s.pumpAllScoped(k, in) // zone_rememberparams.go
			break
		}
		src := s.objRef(k, kop.RefSelf)
		a, b := s.value(k, in.A), s.value(k, in.B)
		for m := s.occ; m.any(); m = m.rest() {
			sl := m.first()
			if !s.matchPerm(k, in.F, sl, e.ctrl, src) {
				continue
			}
			s.touchSl(sl)
			// P/T pumps are creature-only; a keyword grant lands on every
			// matching permanent, as gorge effPump does (Restoration
			// Magic's "Permanents you control gain hexproof and
			// indestructible").
			if s.isCreature(k, sl) {
				if !s.eotAdd(k, sl, a, b) {
					break
				}
			}
			if in.C != 0 {
				kw := kop.KwMask(uint16(in.C))
				s.grantEOT(sl, kw)
				s.promoKw(k, sl, kw)
			}
		}
	case kop.OpAddMana:
		p := s.playerRef(k, in.Obj)
		if p == 255 {
			p = e.ctrl
		}
		if in.B.N == 3 && in.B.K == kop.VConst {
			s.unspentRedOn(p) // Effect UnspentMana Red (value_fightexcess.go)
			break
		}
		n := s.value(k, in.A)
		if k.gen != nil && k.gen.anySpellMana {
			s.dbManaMut(e, &n) // mana_dbresolve.go
		}
		c := kop.ColorMask(in.C)
		col := kop.C
		for i := 0; i < kop.NumManaKinds; i++ {
			if c&(1<<i) != 0 {
				col = i // TODO: any-colour choice; takes the last colour of the mask
			}
		}
		if in.B.N == 1 && in.B.K == kop.VConst {
			// firebending (CR 702.189a): an amount <= 0 adds nothing (gorge
			// mana_effect.go returns early), so a Firebending Student with
			// negative power must not wrap the uint8 pool or fbR
			n = max(n, 0)
			s.firebendAdd(p, n) // kw_firebend.go
		} else if in.B.N == 2 && in.B.K == kop.VConst {
			n = max(n, 0) // PersistentMana$ True: the units are also tallied in State.pm (mana_persistent.go)
			s.persistAdd(p, col, n)
		}
		if in.F != 0 {
			s.addManaRestr(k, p, col, n, in.F) // mana_resolverestriction.go
			break
		}
		s.poolAdd(p, col, n)
	case kop.OpReturn:
		o := s.objRefObj(in.Obj)
		if o == NoObj {
			break
		}
		if k.gen != nil {
			s.g.retMiss = true // until the move: an OpOverride rider tests it
		}
		ob := &s.objs[o]
		if ob.zone == kop.ZoneBattlefield || ob.zone == kop.ZoneStack || ob.zone == kop.ZoneNone || s.tokenLeftBattlefield(o) || returnLeftOrigin(in, ob) || s.instantStays(k, o, kop.Zone(in.C)) {
			break
		}
		if in.Obj == kop.RefSelf && s.res.e.kind == seAbility && ob.ver != s.res.e.srcVer {
			break // the activated ability's source returned and left again: a new object (CR 400.7; Postmortem Professor)
		}
		if kop.Zone(in.C) == kop.ZoneBattlefield && s.cloneReturnPark(k, o, ob.owner()) {
			return false // the clone election is asked with the card still in its zone (copy_cloneenter.go)
		}
		s.execReturnSimple(k, in, o)
	case kop.OpRepeat:
		n := s.value(k, in.A)
		body := uint8(in.B.N)
		if n <= 0 {
			s.res.pc += body
			break
		}
		r := &s.res
		if r.loopD >= 2 {
			panic("kernel: OpRepeat nested deeper than 2")
		}
		r.loopN[r.loopD] = uint8(n)
		r.loopS[r.loopD] = r.pc + 1
		r.loopPC[r.loopD] = r.pc + 1 + body
		r.loopD++
	case kop.OpSkipIf:
		if !s.cond(k, in) {
			s.res.pc += uint8(in.B.N)
		}
	case kop.OpLoseGame:
		if p := s.playerRef(k, in.Obj); p != 255 {
			if in.C != 0 {
				s.winGame(k, p)
				break
			}
			s.lose(p)
		}
	case kop.OpShuffle:
		if p := s.playerRef(k, in.Obj); p != 255 {
			s.shuffleLib(p)
		}
	case kop.OpEndTurn, kop.OpAddTurn:
		if in.Op == kop.OpAddTurn {
			p := s.playerRef(k, in.Obj)
			if p == 255 {
				p = e.ctrl
			}
			s.pl[p].extraTurns++
			break
		}
		panic("kernel: op not implemented: " + in.Op.String())
	case kop.OpDelayedTrigger:
		if in.C == opLeaveReturn {
			s.leaveReturnReg(e, in) // trig_leavereturn.go
			break
		}
		o := s.objRefObj(in.Obj)
		if o == NoObj {
			s.delayEmpty(e)
		} else if s.ndly < MaxDelayed {
			s.dly[s.ndly] = delayed{act: uint8(in.C), ctrl: e.ctrl, obj: o, ver: s.objs[o].ver, creator: NoObj}
			s.ndly++
		} else {
			s.capFull(StallDelayed)
		}
	case kop.OpGainControlEOT:
		// the record path of OpGainControl (CR 613.1b): until end of
		// turn, untap it, it gains haste
		if sl := s.objRef(k, in.Obj); sl != NoSlot {
			s.touchSl(sl)
			if !s.gainControl(sl, e.ctrl, e.ctrl, kop.DurEOT, NoObj, 0, false) {
				break
			}
			p := &s.perms[sl]
			if blocked := s.untapBlocked(k, sl); !blocked && p.flags&pfTapped != 0 && p.ctrLow(int(kop.CtrStun)) > 0 {
				p.seeCtr(s)
				p.subCtrN(s, int(kop.CtrStun), 1) // a stun counter is removed instead (CR 122.1d; gorge TryUntap)
			} else if !blocked {
				s.untapEffect(k, sl)
			}
			s.grantEOT(sl, kop.KwHaste)
			s.promoKw(k, sl, kop.KwHaste)
		}
	case kop.OpSetPT:
		if sl := s.objRef(k, in.Obj); sl != NoSlot {
			s.touchSl(sl)
			p := &s.perms[sl]
			p.ovr.setP, p.ovr.setT = int8(s.value(k, in.A)), int8(s.value(k, in.B))
			if in.C == 1 {
				p.ovr.flags |= ovSetPTEOT
			} else {
				p.ovr.flags |= ovSetPT
			}
		}
	case kop.OpLoseAbilities:
		if sl := s.objRef(k, in.Obj); sl != NoSlot {
			s.touchSl(sl)
			p := &s.perms[sl]
			s.loseAllOnset(k, NoSlot, sl)
			if !s.setLossFx(sl, s.tick(), in.C == 1) {
				s.stall(StallTables)
			}
			if in.C == 1 {
				p.ovr.flags |= ovLoseAllEOT
			} else {
				p.ovr.flags |= ovLoseAll
			}
		}
	case kop.OpAnimate:
		if sl := s.objRef(k, in.Obj); sl != NoSlot {
			s.touchSl(sl)
			p := &s.perms[sl]
			p.ovr.types = s.typesAt(k, sl) | kop.TCreature
			p.ovr.flags |= ovTypesEOT | ovSetPTEOT
			p.ovr.setP, p.ovr.setT = int8(s.value(k, in.A)), int8(s.value(k, in.B))
			p.ovr.kw |= kop.KwMask(uint16(in.C))
			s.promoKw(k, sl, kop.KwMask(uint16(in.C)))
		}
	case kop.OpRegenerate:
		if sl := s.objRef(k, in.Obj); sl != NoSlot {
			s.addRegenShield(sl)
		}
	case kop.OpProliferate:
		return s.proliferate(k, in) // a generated table's (act_proliferate.go)
	default:
		panic("kernel: op not implemented: " + in.Op.String())
	}
	return true
}

// cond evaluates OpSkipIf's condition C with operand A.
func (s *State) cond(k *prep, in *kop.Instr) bool {
	switch in.C {
	case 0: // the first target is still legal
		return s.tgtCode(0) != NoTarget
	case 1: // value A > 0
		return s.value(k, in.A) > 0
	case 2: // the spell was not kicked
		return s.res.e.flags&sfKicked == 0
	}
	panic("kernel: SkipIf condition not implemented")
}

func (s *State) destroy(k *prep, sl Slot) {
	if s.kw(k, sl)&kop.KwIndestructible != 0 {
		return
	}
	if s.destroyHeal(k, sl) {
		return
	}
	if s.canRegen(sl) {
		s.regenerate(sl)
		return
	}
	if k.gen != nil && k.gen.anyRepl&rbShield != 0 && s.shieldDestroy(sl) {
		return // CR 122.1c, after regeneration (gorge effDestroy)
	}
	s.leaveParked(k, sl)
}

// moveRef moves the object a Ref names to zone z (its owner's).
func (s *State) moveRef(k *prep, r kop.Ref, z kop.Zone, bottom bool) {
	o := s.objRefObj(r)
	if o == NoObj || s.freeQueued(o) || s.freeBegun(o) {
		// gorge begins a Play's free cast at the answer, so an "if you don't
		// cast it, move it" tail finds a spell, not a card in its zone
		return
	}
	ob := &s.objs[o]
	switch ob.zone {
	case kop.ZoneBattlefield:
		if r == kop.RefSelf && ob.ver != s.res.e.srcVer && s.res.e.kind != seSpell {
			return
		}
		s.leave(k, ob.slot, z, bottom) // leave applies the CR 903.9b gate itself
	case kop.ZoneStack:
		if s.cmdZoneReplace(k, o, &z, &bottom) {
			return // a CR 903.9b commander move parked before it happens
		}
		if i := s.stackIndexOf(o); i >= 0 {
			copy(s.stack[i:s.sp-1], s.stack[i+1:s.sp])
			s.sp--
		}
		s.touch(o)
		ob.zone = kop.ZoneNone
		if k.gen != nil && k.gen.anyFaces {
			z = s.stackExitFace(k, o, z, false) // face_castface.go
		}
		s.placeFromK(k, o, z, bottom, kop.ZoneStack, false)
	case kop.ZoneNone:
	default:
		if r == kop.RefSelf && (s.res.e.kind == seAbility || s.res.e.kind == seTrigger) && ob.ver != s.res.e.srcVer {
			return // the ability's source left its zone: a new object (CR 400.7; Foul Familiar destroyed in response, Encroaching Dragonstorm destroyed with its return trigger on the stack)
		}
		if s.tokenLeftBattlefield(o) {
			return // CR 111.8 (zone_tokenleft.go)
		}
		if s.cmdZoneReplace(k, o, &z, &bottom) {
			return // a CR 903.9b commander move parked before it happens
		}
		from := s.detach(o)
		s.placeFromK(k, o, z, bottom, from, false)
	}
}

// damageBetween: creature a deals n damage to creature b (a bite, or
// DamageSource$): a batch of one hit (hitBatch).
func (s *State) damageBetween(k *prep, a, b Slot, n int) {
	var hb hitBatch
	hb.add(s, a, b, n)
	s.dealHits(k, hb.h[:hb.n])
}

// discardObj discards hand card o.
func (s *State) discardObj(k *prep, o Obj) { s.discardObjAs(k, o, true) }

// discardObjAs is discardObj; effect false marks a cost discard, which no
// ReplDiscardOppToBattlefield card replaces (repl_discardetb.go).
func (s *State) discardObjAs(k *prep, o Obj, effect bool) {
	from := s.detach(o)
	mad := s.madnessExiled(k, o) // CR 702.35a: discarded into exile instead
	if effect && k.gen != nil && k.gen.anyRepl&rbDiscETB != 0 && s.discardToBattlefield(k, o, from) {
		mad = false
	} else if mad {
		s.placeFromK(k, o, kop.ZoneExile, false, from, false)
	} else {
		s.placeFromK(k, o, kop.ZoneGraveyard, false, from, false)
	}
	s.noteDisc(o) // value_discardedthisturn.go
	if !mad && k.mayhem {
		s.mayhemMark(k, o) // kw_mayhem.go
	}
	s.g.disc = o
	if k.gen != nil && k.gen.anyRemDisc {
		s.discardRemember(k, o)
	}
	if s.subs[kop.TrigDiscarded].any() {
		s.firePlayer(k, kop.TrigDiscarded, s.objs[o].owner(), 1)
	}
	if k.gen != nil && k.gen.anySelfDisc {
		s.fireSelfDisc(k, o) // trig_params.go
	}
	if k.gen != nil && k.gen.anyDiscAll {
		s.discAllNote(k, o)
	}
	if mad {
		s.queueMadness(o)
	}
}

// --- suspended resolution decisions ---

func (s *State) resolveOptions(k *prep, d *Decision) bool {
	r := &s.res
	d.Player = r.askP
	switch r.ask {
	case raDiscard:
		// gorge KModes "discard" (Min=Max=N): the picks are recorded and
		// the cards discarded together in pick order once all are chosen,
		// so the hand (gorge's option order) is stable across the picks
		d.Kind, d.Sub = DkChoose, ChDiscard
		pl := &s.pl[r.chosen]
		for i := 0; i < int(pl.handN); i++ {
			o := pl.hand[i]
			taken := false
			for j := 0; j < int(r.peekI); j++ {
				if r.arr[j] == o {
					taken = true
				}
			}
			if !taken && s.objs[o].of&ofCopy == 0 && (k.gen == nil || s.discardOK(k, o)) {
				d.add(MakeAction(AkChoose, uint8(o), 0, 0))
			}
		}
	case raScryTop, raSurveil:
		// gorge's KArrange as ordered picks: each pick puts the next card
		// on top (first pick = new top); AkDone sends the rest to the
		// bottom (scry) / graveyard (surveil) in library order.
		//
		// With Config.ArrangeRestOrder, closing the top picks while two or
		// more cards remain opens a second phase: the remaining cards are
		// ordered by the same picks (the first goes first to the
		// graveyard / the bottom), all but the last, which is implied; no
		// Done.
		d.Kind, d.Sub = DkChoose, ChArrange
		for i := 0; i < int(r.peekN); i++ {
			if (r.peekSel|r.restSel)&(1<<i) == 0 {
				d.add(MakeAction(AkChoose, uint8(r.peek[i]), 0, 0))
			}
		}
		if !r.restPh {
			d.add(MakeAction(AkDone, 0, 0, 0))
		}
	case raSacrifice:
		d.Kind, d.Sub = DkChoose, ChSacrifice
		for m := s.ctrlM[r.askP]; m.any(); m = m.rest() {
			sl := m.first()
			if s.sacMatch(k, s.prog(k)[r.pc].F, sl, r.askP, s.resSrcSlot(k)) && !s.g.sacM.has(sl) {
				d.add(MakeAction(AkChoose, uint8(s.perms[sl].obj), 0, 0))
			}
		}
	case raOptional, raGeneric:
		d.Kind, d.Sub = DkChoose, ChOptional
		if r.ask == raGeneric {
			d.Sub = ChWardPay
		}
		d.add(MakeAction(AkChoose, 0, 0, 0))
		d.add(MakeAction(AkChoose, 0, 0, 1))
	case raOptDiscMay:
		// gorge's may-election (cardflow.go: KChoose Min 1 Max 1, kinds
		// "yes"/"no"): yes re-poses the Min 1 pick, no discards nothing.
		d.Kind, d.Sub = DkChoose, ChOptDiscMay
		d.add(MakeAction(AkChoose, 0, 0, 1))
		d.add(MakeAction(AkChoose, 0, 0, 0))
	case raOptSac:
		// gorge's optional sacrifice ask (zone_bulk.go: KChoose Min 0,
		// Max min(Amount, eligible)): the picks stay on the battlefield
		// until the answer commits, then leave in pick order.
		d.Kind, d.Sub = DkChoose, ChSacrifice
		in := &s.prog(k)[r.pc]
		for m := s.ctrlM[r.askP]; m.any(); m = m.rest() {
			sl := m.first()
			if !s.optaskPicked(s.perms[sl].obj) && s.sacMatch(k, in.F, sl, r.askP, s.resSrcSlot(k)) {
				d.add(MakeAction(AkChoose, uint8(s.perms[sl].obj), 0, 0))
			}
		}
		d.add(MakeAction(AkDone, 0, 0, 0)) // Min 0: Done is always offered
	case raOptDisc:
		// gorge's discard pick (KModes "discard" Min 0/1 Max min(NumCards,
		// eligible)): the same ChDiscard shape, Done offered from the Min.
		d.Kind, d.Sub = DkChoose, ChDiscard
		pl := &s.pl[r.chosen]
		for i := 0; i < int(pl.handN); i++ {
			o := pl.hand[i]
			if !s.optaskPicked(o) && s.discardOK(k, o) {
				d.add(MakeAction(AkChoose, uint8(o), 0, 0))
			}
		}
		if r.peekI >= uint8(s.optDiscardMin(k)) {
			d.add(MakeAction(AkDone, 0, 0, 0))
		}
	default:
		return s.resolveOptionsGen(k, d)
	}
	return true
}

func (s *State) applyResolve(k *prep, a Action) {
	r := &s.res
	if r.ask >= raGenBase {
		s.applyResolveGen(k, a)
		return
	}
	switch r.ask {
	case raDiscard:
		if int(r.peekI) < len(r.arr) {
			r.arr[r.peekI] = Obj(a.Source())
			r.peekI++
		} else {
			// len(r.arr) is MaxHand, so this is unreachable while the hand
			// itself is bounded by MaxHand: a discard pick is always held and
			// moved in answer order (CR 404.3).
			s.discardObj(k, Obj(a.Source()))
		}
		r.left--
		if r.left > 0 && int(s.pl[r.chosen].handN) > int(r.peekI) {
			return
		}
		in := &s.prog(k)[r.pc]
		chained := s.discardChained(k, in)
		if chained && s.g.repNext != 255 {
			// the next player chooses with these picks still in hand
			s.holdDiscard(r.arr[:r.peekI])
			r.peekI = 0
			p := s.g.repNext
			s.g.repNext = 255
			if !s.discardChain(k, in, p, 255) {
				return // the next player's ask is live
			}
			break
		}
		if chained {
			s.flushHeldDiscard(k, r.chosen^1)
		}
		// a competing pick waits for its order ask, and the picks behind it, or
		// behind a held pick's ask, wait too (discardPicks)
		s.discardPicks(k, r.arr[:r.peekI], 0)
		r.peekI = 0
	case raScryTop, raSurveil:
		if r.restPh {
			for i := 0; i < int(r.peekN); i++ {
				if r.peek[i] == Obj(a.Source()) {
					r.restSel |= 1 << i
				}
			}
			r.arr[r.peekI] = Obj(a.Source())
			r.peekI++
			if int(r.peekN)-int(r.peekI) > 1 { // the last card is implied
				return
			}
			s.finishPeek(k)
			break
		}
		if a.Kind() == AkChoose {
			for i := 0; i < int(r.peekN); i++ {
				if r.peek[i] == Obj(a.Source()) {
					r.peekSel |= 1 << i
				}
			}
			r.arr[r.peekI] = Obj(a.Source())
			r.peekI++
			if r.peekI < r.peekN {
				return
			}
		} else if s.cfgx&cfgxArrangeRest != 0 && int(r.peekN)-int(r.peekI) >= 2 {
			r.restPh, r.topN = true, r.peekI // AkDone: order the rest
			return
		}
		s.finishPeek(k)
	case raChooseType:
		if k.gen != nil && s.prog(k)[r.pc].C != 0 {
			r.chosen = cardTypeAnswer(a)
			r.bad |= resChosenCard
		} else {
			r.chosen = typeVocabSub(k, int(a.Target())<<8|int(a.Mode()))
		}
		r.bad |= resChosenType
		if o, sl := s.resolutionTypeSource(); sl != NoSlot {
			s.perms[sl].chosen = r.chosen
		} else if o != NoObj {
			s.objs[o].chosen = r.chosen
			s.touch(o)
		}
	case raSacrifice:
		if r.left >= 2 || s.g.nsac > 0 {
			if !s.sacPickBatch(k, Obj(a.Source())) { // act_repeatplayer.go
				return
			}
			break
		}
		o := Obj(a.Source())
		s.fireSac(k, s.objs[o].slot)
		s.leaveParked(k, s.objs[o].slot)
		r.left--
		if r.left > 0 {
			return
		}
	case raOptDiscMay:
		// "no" (mode 0) discards nothing; "yes" (mode 1) poses the Min 1
		// pick, or resolves inline when the whole hand (Mode$ Hand) or a
		// single eligible card (gorge's strict-supersets rule) is the only
		// real answer. In both inline shapes the op completes below.
		if a.Mode() == 1 && s.optDiscardAccept(k) {
			return // the pick ask is live
		}
		r.ask = raNone
	case raOptSac:
		if a.Kind() != AkDone && s.optaskPick(k, a, func(k *prep, o Obj) {
			s.fireSac(k, s.objs[o].slot)
			s.leave(k, s.objs[o].slot, kop.ZoneGraveyard, false)
		}) {
			return // re-pose: picks remain
		}
		s.optaskFireSac(k)
		if k.gen != nil { // generated tables ask player by player (startOptSac)
			// only an each-player op has a repNext chain: reading it for a
			// single-player ask (Crovax's RefController) would pick up a
			// stale OpRepeatEachPlayer remainder and re-ask the same player
			in := &s.prog(k)[r.pc]
			if in.Obj == kop.RefEachPlayer || in.Obj == kop.RefEachOpponent {
				p := s.g.repNext
				s.g.repNext = 255
				if !s.startOptSac(k, in, p) {
					return // the next player's ask is live
				}
			}
			s.continueRes(k, 0)
			return
		}
	case raOptDisc:
		if a.Kind() != AkDone && s.optaskPick(k, a, s.discardObj) {
			return // re-pose: picks remain
		}
		s.optaskFireDiscard(k)
	case raOptional:
		r.ask = raNone
		if a.Mode() == 0 {
			s.finishResolve(k, false)
			return
		}
		if r.e.kind == seTrigger && int(r.e.ab) < len(k.t.Cards[r.e.card].Abilities) && k.t.Cards[r.e.card].Abilities[r.e.ab].Trig.Flags2&kop.Tf2ResolvedLimit != 0 {
			s.countRL(&r.e) // an accepted optional limited line counts (act_support.go)
		}
		s.ctl = ctlResolve
		s.runProg(k)
		return
	case raGeneric: // ward: pay {N}?
		r.ask = raNone
		if r.e.trigAmt&wardMLBit != 0 {
			s.applyWardManaLife(k, a) // cost_unlesstapward.go
			return
		}
		s.payWardN(k, uint8(r.e.trigAmt), a.Mode() == 1)
		return
	}
	if len(s.cold.parked) != 0 && s.parkedWaiting() {
		s.ctl = ctlExileOrder
		return
	}
	r.ask = raNone
	if k.gen != nil && s.parkReturnAsk() {
		return // the answer made a departure frame park a return: ask it before the program goes on (zone_untilleavesbatch.go)
	}
	r.pc++
	s.loopBack(k)
	s.runProg(k)
}

// finishPeek applies an arrange answer (gorge engine/rules/arrange.go):
// the picked cards go back on top in pick order (the first pick on top);
// the rest go to the bottom (scry, the first highest) or to the graveyard
// (surveil): in the order the rest phase picked them, then the card left
// unpicked, or in library order when there was no rest phase.
func (s *State) finishPeek(k *prep) {
	r := &s.res
	p := r.askP
	pl := &s.pl[p]
	n := int(r.peekN)
	top := int(r.peekI) // the top picks
	if r.restPh {
		top = int(r.topN)
	}
	for i := 0; i < n; i++ {
		s.touch(r.peek[i])
		s.objs[r.peek[i]].zone = kop.ZoneNone
	}
	pl.libLo += uint8(n) // the peeked cards are the top n, in order
	var rest [8]Obj      // res.peek's window
	nr := 0
	for i := top; i < int(r.peekI); i++ { // the rest phase's picks
		rest[nr] = r.arr[i]
		nr++
	}
	for i := 0; i < n; i++ {
		if (r.peekSel|r.restSel)&(1<<i) == 0 {
			rest[nr] = r.peek[i]
			nr++
		}
	}
	remember := s.prog(k)[r.pc].C&kop.MvRemember != 0
	parked := -1
	if r.ask != raScryTop {
		// the move has not happened yet: gorge's replacement predicate reads
		// these cards in the library, where a competing order ask leaves them
		for i := 0; i < nr; i++ {
			s.objs[rest[i]].zone = kop.ZoneLibrary
		}
		parked = s.parkRests(k, rest[:nr], remember)
	}
	if parked >= 0 {
		for i := 0; i < parked; i++ { // earlier rest cards move at once
			s.putRest(k, rest[i])
		}
		// the asked ones wait in the library but off its list, as gorge's
		// look leaves them (zone 0, no position), on top of which the
		// kept cards go
		s.returnKept(k, p, top)
		return // the CR 616.1 ask is posed before those moves
	}
	if r.ask != raScryTop {
		for i := 0; i < nr; i++ {
			s.objs[rest[i]].zone = kop.ZoneNone
		}
	}
	s.returnKept(k, p, top)
	for _, o := range rest[:nr] {
		s.putRest(k, o)
	}
	if k.gen != nil && (k.gen.anyScryTrig || k.gen.anyScryTally) && r.ask == raScryTop {
		s.scryAnswered(k, p) // value_scrytally.go
	}
}

// returnKept puts the cards the arrange kept back on top of the library in
// pick order (the first pick on top) and records a surveil's kept ones when the
// move asks for it.
func (s *State) returnKept(k *prep, p uint8, top int) {
	pl := &s.pl[p]
	r := &s.res
	for i := top - 1; i >= 0; i-- {
		o := r.arr[i]
		s.objs[o].zone = kop.ZoneLibrary
		pl.libTop(o)
	}
	if r.ask == raSurveil && s.prog(k)[r.pc].C&kop.SurveilRememberKept != 0 {
		s.surveilRememberKept(top) // RememberKept$ (zone_rememberwrites.go)
	}
}

// putRest sends one card the arrange did not keep on top to the bottom
// (scry) or the graveyard (surveil).
func (s *State) putRest(k *prep, o Obj) {
	if s.res.ask == raScryTop {
		s.objs[o].zone = kop.ZoneLibrary
		s.pl[s.res.askP].libBottom(o)
		return
	}
	s.placeFromK(k, o, kop.ZoneGraveyard, false, kop.ZoneLibrary, false)
	if s.prog(k)[s.res.pc].C&kop.MvRemember != 0 {
		s.surveilRemember(o) // RememberMoved$ (zone_rememberwrites.go)
	}
}

// handDiscardable counts the cards in p's hand a discard may take. A token copy
// that bounced to the hand waits there for the state-based actions to cease it
// (ceaseTokensGen) and is no card in the hand to Forge, whose zone never lists a
// token off the battlefield; gorge's matcher rejects it too (CR 707.10h), so a
// discard neither offers it nor counts it toward taking the whole hand.
func (s *State) handDiscardable(p uint8) int {
	pl := &s.pl[p]
	n := int(pl.handN)
	for i := 0; i < int(pl.handN); i++ {
		if s.objs[pl.hand[i]].of&ofCopy != 0 {
			n--
		}
	}
	return n
}
