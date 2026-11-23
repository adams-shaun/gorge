package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Entering and leaving, turn bookkeeping, the as-enters choices and the
// Leyline ask of a generated table.

// ctlGenAsk is the control state of a generated-table decision posed
// between priority decisions (an as-enters choice, a Leyline).
const ctlGenAsk uint8 = 200

// ask kinds of genState.ask
const (
	akColor   uint8 = 1
	akType    uint8 = 2
	akEnchant uint8 = 3 // a returning Aura's host (s.g.pre only: CR 303.4f)
	akClone   uint8 = 4 // an "enter as a copy" choice, cast or returned (s.g.pre only; copy_asenters.go)
	akPayLife uint8 = 5 // a shock land's as-enters "pay N life or enter tapped" (repl_shockland.go)
	akMode    uint8 = 6 // a Siege's as-enters mode (face_siegemode.go)
	akReveal  uint8 = 7 // a reveal land's as-enters "reveal or enter tapped" (repl_revealland.go)
	akRevPick uint8 = 8 // the card a reveal land's paid election reveals
	akNumber  uint8 = 9 // an as-enters number (OpExt ExtChooseNumber; repl_choosenumber.go)
	// akEntryOrder is the CR 616.1 order between an as-enters copy and the
	// other replacements over the same entry, asked with the spell still on
	// the stack (s.g.pre only; repl_entryclone.go)
	akEntryOrder uint8 = 10
	akName       uint8 = 11 // an as-enters nonland card name (OpExt ExtChooseName; repl_choosename.go)
	akNameAny    uint8 = 12 // an as-enters name of any card, lands included (ExtChooseName B = 1)
	akNameLook   uint8 = 13 // akNameAny after a look_ack at the chosen opponent's hand (ExtChooseName B = 2; Sorcerous Spyglass)
	akNameHand   uint8 = 15 // a nonland name among the opponents' revealed hands (ExtChooseName B = 3; Alhammarret, High Arbiter)
	// akEtbOrder is the CR 616.1 order between a cast spell's effect-carried
	// enters-with counters and the entrant's own prepared enter body, asked
	// with the spell still on the stack (s.g.pre only; repl_etborder.go)
	akEtbOrder uint8 = 14
	akPlayer   uint8 = 16 // an as-enters opponent (OpExt ExtChoosePlayer; repl_chooseplayer.go)
	// akUnleash is 33, past akCard (17, repl_choosecard.go), akPayETB (30,
	// repl_paybeforeetb.go) and the EnterCtrPick pair (31, 32,
	// repl_enterctrpick.go): the ask kinds continue across their files.
	akUnleash uint8 = 33 // an Unleash entry's counter election (CR 702.86a; kw_unleash.go)
)

// placeEntryCounters places an entry-characteristic counter grant on the
// entrant at sl, with the AddCounter matcher skipping the entrant's own
// general-subset replacements (CR 614.12, gorge's ownEntryCounterExcluded).
// Gorge settles a permanent's entry counters against the would-enter board
// (engine/rules/entry_counters.go), so a line the entrant itself carries
// applies to its own entry only when it selects the entrant alone (Mowu's
// "on CARDNAME"); a general subset such as Yoshimaru's "a creature you
// control" does not. The entrant stays in statM: its other statics, notably a
// CantPutCounter prohibition, are live on gorge's post-entry preview.
func (s *State) placeEntryCounters(k *prep, sl Slot, kind, n int, ctrl uint8) int {
	if n <= 0 {
		return 0
	}
	saved := s.g.ctrSettle
	s.g.ctrSettle = uint8(sl) + 1
	m := s.placeCounters(k, sl, kind, n, ctrl, ctrPutBy)
	s.g.ctrSettle = saved
	return m
}

// enterGen runs after the core's enter placed o at sl, before its ETB triggers:
// it resets the slot's generated state, records how the permanent arrived,
// and applies the as-enters replacements (its own and other permanents').
func (s *State) enterGen(k *prep, sl Slot, o Obj, from kop.Zone, face uint8) {
	if face == 0 {
		s.payETBStray(k, o)         // repl_paybeforeetb.go
		s.unleashEntryAsk(k, sl, o) // kw_unleash.go: the CR 702.86a counter ask, gorge's parked move (a face-down entry poses nothing, CR 708.5)
	}
	s.touch(o)
	s.clearFx(sl)
	s.arenaClrSlot(sl) // the slot takes a new permanent: no mechanic's mark for the last one survives (arena.go)
	if s.g.entExc != 0 {
		x := s.g.entExc
		s.g.entExc = 0
		s.copyExcAdd(k, sl, Slot(x>>8), uint8(x))
	}
	if s.osAnyOf(osMExw) {
		s.exwEnter(o, from) // a source entering other than from the stack clears its forward links (zone_exiledwith.go)
	}
	if s.g.nchPick != 0 {
		s.charmPickSlotLeft(sl) // modal_charmpick.go: a new permanent starts with no pick log (CR 400.7)
	}
	p := &s.perms[sl]
	p.chosen = 255
	s.g.cast.clr(sl)
	if s.res.active && s.res.e.kind == seSpell && s.res.e.obj == o {
		if s.objs[o].of&(ofCopy|ofToken) == 0 || s.copyCastEntry(k, &s.res.e) {
			s.g.cast.set(sl) // a stack copy is never cast (CR 707.10), but a Play copy is (CR 707.12; cast_copycast.go)
		}
		if s.res.e.flags&sfKicked != 0 && !(k.gen.anyOptCost && castCollectedNever(k, &s.res.e)) {
			s.mSlSet(k, mechKicked, sl)
			if k.t.Cards[s.res.e.card].HasMultikicker && int(sl) < len(s.cold.mk) {
				s.cold.mk[sl] = uint8(s.multikickCount(&s.res.e)) // CR 702.43: the permanent reads its spell's count
			}
		}
		if sneakMarks(&s.res.e) {
			s.mSlSet(k, mechSneaked, sl) // kw_sneak.go
		}
		if escapeMarks(&s.res.e) {
			s.mSlSet(k, mechEscaped, sl) // kw_escape.go
		}
	} else if int(o) < len(s.cold.mkObj) {
		// an entry that is not the resolving spell (reanimate, blink, return)
		// is a new object with no payments (CR 400.7, 702.43): the last cast's
		// count must not survive to the ETB trigger's target bound (exprs.go)
		s.cold.mkObj[o] = 0
	}
	s.g.zoneTurn.set(o)
	if s.g.enteredOn {
		s.noteEnter(o, kop.ZoneBattlefield, from, false, false)
	}
	if s.typesGen(k, sl)&kop.TLand == 0 {
		s.stashClr(k, o) // only a land played from exile keeps its stash counter (gorge)
	}
	if k.gen.anyEntryMerge && face == 0 {
		s.entryWinOpen(k, sl, p) // ctr_entryperkind.go: the printed and requested counters start the entry's one put per kind
	} else if n := s.g.enterCtr.n; n > 0 {
		if face == 0 {
			s.placeEntryCounters(k, sl, int(s.g.enterCtr.kind), int(n), p.ctrl)
			if k2 := s.g.enterCtr.kind2; k2 != 0 {
				s.g.enterCtr.kind2 = 0
				s.placeEntryCounters(k, sl, int(k2)-1, int(n), p.ctrl) // the pair's second kind, script order
			}
		}
		s.g.enterCtr.n, s.g.enterCtr.kind2 = 0, 0
	}
	if k.gen.anyEffEtbCtr && face == 0 && s.res.active && s.res.e.kind == seSpell && s.res.e.obj == o {
		s.effEtbEnter(k, sl, p) // ctr_effectentry.go: after the printed counters, before the entry's triggers
	}
	if k.gen.anyFinality && face == 0 && s.res.active && s.res.e.kind == seSpell && s.res.e.obj == o {
		s.finalityEnter(k, sl, p) // trig_mayplaystatic.go: the FINALITY rider, beside the P1P1 one
	}
	if s.g.tokPT.on && s.objs[o].of&ofToken != 0 {
		// TokenPower$/TokenToughness$: the token enters with that P/T, so its
		// enters checks (a watcher's power limit) read it (act_tokenpt.go)
		if s.g.tokPT.left > 0 {
			s.g.tokPT.left--
		} else {
			s.g.tokPT.on = false
		}
		if !DebugTokenXPTSkip {
			s.applyOverridePT(k, sl, int(s.g.tokPT.idx), s.res.e.card, 0, s.g.tokPT.p, s.g.tokPT.t)
		}
	}
	if s.g.ewPend {
		s.g.ewPend = false
		s.enterWithApply(k, sl) // zone_assimilate.go
	}
	cid := s.cardAt(sl)
	g := k.gen
	if !g.has(cid, gfEnterRepl) && !s.statM.any() {
		if g.anyEntryMerge {
			s.entryWinClose(k, sl, p)
		}
		return
	}
	// as-enters replacements are applied before the object is on the
	// battlefield: counts made by them do not see it
	s.occ.clr(sl)
	s.ctrlM[p.ctrl].clr(sl)
	saveRep := s.g.replaced
	s.g.replaced = o
	var ovr uint8
	if g.anyEntUntap || g.anyEntAttack {
		// A face-down entrant runs no own enter program, but the other
		// permanents' bodies still apply to it (entryComp skips its own).
		ovr = s.entryOrderEnter(k, sl, o) // the CR 616.1 order ask's result, or a stall (repl_entryorder.go)
	}
	// The entrant's own enters programs and the other permanents' enter
	// replacements apply in the order the answered CR 616.1 asks named, when
	// any were posed (repl_entryclone.go); otherwise the own line comes
	// first, as before.
	if s.g.pre.ordN > 0 && s.g.pre.obj == o {
		s.runEntryOrdered(k, sl, o, cid, p.ctrl, face, g)
	} else {
		s.runOwnEnterPrograms(k, sl, o, cid, p.ctrl, face)
		s.runOthersEnterPrograms(k, sl, g, s.statM)
	}
	s.g.replaced = saveRep
	shock := ovr != 0 && face == 0 && g.has(cid, gfEnterRepl) && s.shockCardAmount(k, cid) > 0
	if shock {
		s.shockEntryOrdered(sl)
	}
	switch ovr {
	case entOvrTap:
		if !shock {
			p.flags |= pfTapped // the last body applied taps it
		} // a shock land's own body is that tap: the election's answer applies it
	case entOvrUntap:
		p.flags &^= pfTapped // the last body applied untaps it
		if shock {
			s.shockUntapWins(sl) // the election is still asked, its tap came first
		}
	}
	s.occ.set(sl)
	s.ctrlM[p.ctrl].set(sl)
	if g.anyEntryMerge {
		s.entryWinClose(k, sl, p)
	}
	s.refreshTy(k)
}

// runOwnEnterPrograms applies the entering card's own enters programs (its
// as-enters copy, its enters-with bodies), with the permanent at sl as the
// source.
func (s *State) runOwnEnterPrograms(k *prep, sl Slot, o Obj, cid CardID, ctrl, face uint8) {
	if face != 0 || !k.gen.has(cid, gfEnterRepl) {
		return
	}
	cd := &k.t.Cards[cid]
	// the entering permanent's own X (CR 107.3f): its cast X when it is
	// the resolving spell, 0 when another effect put it onto the
	// battlefield, never that effect's X (Mind into Matter's)
	selfX := s.res.e.x
	if s.res.active {
		s.res.e.x = objWord(s.spellX(o))
	}
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind == kop.AbReplacement && a.Repl.Kind == kop.ReplEntersProgram {
			ctx := evalCtx{src: o, srcSl: sl, ctrl: ctrl}
			if s.res.active {
				ctx.e = &s.res.e
			}
			ok := a.Repl.Cond == 0 || s.condEval(k, a.Repl.Cond, ctx)
			if debugEntersTappedMutate != 0 && a.Repl.Cond != 0 {
				if debugEntersTappedMutate == 1 {
					ok = true
				} else if !ok {
					// mutation: evaluate with the entering object on the
					// battlefield (gorge excludes it; the kernel cleared it
					// from occ/ctrlM above)
					s.occ.set(sl)
					s.ctrlM[ctrl].set(sl)
					ok = s.condEval(k, a.Repl.Cond, ctx)
					s.occ.clr(sl)
					s.ctrlM[ctrl].clr(sl)
				}
			}
			if ok {
				s.runReplProg(k, sl, cid, uint8(i), 0)
			}
		}
	}
	s.res.e.x = selfX
}

// DebugFaceDownFilterPrinted is the lockstep's mutation switch
// (TestFaceDownEntryCreatureFilterMutation): a face-down ENTRANT is matched
// by its printed card characteristics during the other permanents'
// enter-replacement filter, so a `Creature.faceDown` entry filter over a
// cloaked or manifested noncreature refuses it again (gorge's pre-fix
// printed-face read), and the same games diverge.
var DebugFaceDownFilterPrinted bool

// runOthersEnterPrograms applies the "as this enters" replacement of each
// permanent in m that matches the entrant at sl.
func (s *State) runOthersEnterPrograms(k *prep, sl Slot, g *genPrep, m slotMask) {
	for ; m.any(); m = m.rest() {
		src := m.first()
		if s.genLost(k, src) {
			continue
		}
		scid := s.cardAt(src)
		if !g.has(scid, gfOthersRep) {
			continue
		}
		sp := &s.perms[src]
		cd := &k.t.Cards[scid]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind != kop.AbReplacement || a.Repl.Kind != kop.ReplOthersEnterProgram {
				continue
			}
			s.occ.set(sl) // the filter reads the entering object
			// The mutation hook presents the entrant face up for the match
			// (enterNF's gtyFaceDown | TCreature mark, its ovGenTy flag and
			// walk flag), so the filter reads the printed card the way
			// gorge's pre-fix filter did: bytes restored before the next
			// ability, no mark and no changed byte left behind.
			fdPrinted := DebugFaceDownFilterPrinted && s.faceDown(k, sl)
			var (
				saveGty   kop.TypeMask
				saveFlags uint8
				saveWalk  uint8
			)
			if fdPrinted {
				p := &s.perms[sl]
				saveGty, saveFlags, saveWalk = p.gty, p.ovr.flags, p.ovr.walk
				p.gty = 0
				p.ovr.flags &^= ovGenTy
				p.ovr.walk = 0
			}
			ok := s.matchPermGen(k, a.Repl.Filter, sl, sp.ctrl, src, layerFull)
			if fdPrinted {
				p := &s.perms[sl]
				p.gty, p.ovr.flags, p.ovr.walk = saveGty, saveFlags, saveWalk
			}
			s.occ.clr(sl)
			if ok && a.Repl.Cond != 0 && DebugKiraMutate != 2 {
				// IsPresent$ (Kira Nerys): the entering object is not yet among
				// the permanents it counts (cleared from occ above)
				ok = s.condEval(k, a.Repl.Cond, evalCtx{src: sp.obj, srcSl: src, ctrl: sp.ctrl})
			}
			if ok {
				s.runReplProgAs(k, sl, scid, uint8(i), 0, src)
			}
		}
	}
}

// runReplProg runs ability ab of card c (a replacement program) from pc
// on the entering permanent at sl, with that permanent as the source.
func (s *State) runReplProg(k *prep, sl Slot, c CardID, ab, pc uint8) {
	s.runReplProgAs(k, sl, c, ab, pc, sl)
}

// coldReplSrc is the slot+1 of the permanent whose as-enters replacement
// program is running (runReplProgAs), 0 outside one: an others-enter counter
// amount reads that source, not the entrant (enterCountValue). It is a cold
// region, not a genState field, because the fixed-State inventory is pinned
// (TestNoNewFixedStateFields); only a table with an others-enter replacement
// carries the byte.
var coldReplSrc = newCold[uint8]("replSrc", func(g *genPrep) int {
	for _, f := range g.flags {
		if f&gfOthersRep != 0 {
			return 1
		}
	}
	return 0
})

// replSrcOf is coldReplSrc's byte: 0 for a table without the region.
func (s *State) replSrcOf() uint8 {
	if r := coldReplSrc.of(s); len(r) > 0 {
		return r[0]
	}
	return 0
}

// coldEntryNoRun is 1 while an entry's enter programs run for an entry no
// resolution run serves: the exile return of a departing player's sweep
// (returnLinked outside a resolution). gorge runs such a replacement body
// straight off the emit with no run, so every ask the body poses defaults to
// its first option (engine/rules/resolution_ask.go's no-host degrade) instead
// of being queued and posed. Like coldReplSrc, a cold region, not a genState
// field, because the fixed-State inventory is pinned (TestNoNewFixedStateFields);
// only a table with an enter replacement carries the byte.
var coldEntryNoRun = newCold[uint8]("entryNoRun", func(g *genPrep) int {
	for _, f := range g.flags {
		if f&gfEnterRepl != 0 {
			return 1
		}
	}
	return 0
})

// entryNoRun reports the coldEntryNoRun byte: false for a table without it.
func (s *State) entryNoRun() bool {
	if r := coldEntryNoRun.of(s); len(r) > 0 {
		return r[0] == 1
	}
	return false
}

// setEntryNoRun sets the coldEntryNoRun byte (a table without it keeps none).
func (s *State) setEntryNoRun(v uint8) {
	if r := coldEntryNoRun.of(s); len(r) > 0 {
		r[0] = v
	}
}

func (s *State) setReplSrc(v uint8) {
	if r := coldReplSrc.of(s); len(r) > 0 {
		r[0] = v
	}
}

// runReplProgAs runs it with the permanent at src as the source (an
// "other permanents enter" replacement). A choice stops it: the rest runs
// once the choice is answered.
func (s *State) runReplProgAs(k *prep, sl Slot, c CardID, ab, pc uint8, src Slot) {
	prog := k.t.Cards[c].Abilities[ab].Prog
	saved := s.res
	saveSrc := s.replSrcOf()
	s.setReplSrc(uint8(src) + 1)
	defer func() { s.setReplSrc(saveSrc) }()
	if !s.res.active {
		// a value context for the program: the source as a resolving entry
		sp := &s.perms[src]
		s.res = resCtx{e: stackEntry{kind: seAbility, obj: sp.obj, srcVer: s.objs[sp.obj].ver, card: s.cardAt(src), ab: ab, ctrl: sp.ctrl, trigObj: NoObj}}
		s.res.e.tgt = noTargets
	} else if c := s.perms[src].ctrl; c != s.res.e.ctrl {
		// the program's "you" is its source's controller (CR 109.5), not
		// the resolving effect's: Dust Animus returned by an opponent's
		// Mystical Tether counts its own controller's untapped lands
		was := s.res.e.ctrl
		s.res.e.ctrl = c
		defer func() { s.res.e.ctrl = was }()
	}
	for i := int(pc); i < len(prog); i++ {
		in := &prog[i]
		switch in.Op {
		case kop.OpBecomeCopy:
			s.becomeCopyEnter(k, sl, in)
		case kop.OpChooseColor, kop.OpChooseType, kop.OpExt:
			kind := akColor
			switch {
			case in.Op == kop.OpChooseType:
				kind = akType
			case in.Op == kop.OpExt:
				switch in.Ext() {
				case kop.ExtChooseNumber:
					kind = akNumber
				case kop.ExtChooseName:
					kind = nameAskKind(in)
				case kop.ExtChooseCardEnter:
					kind = akCard // repl_choosecard.go
				case kop.ExtEnterCtrPick, kop.ExtPutCounterChoice:
					kind = akCtrPick // repl_enterctrpick.go, ctr_choice.go
				case kop.ExtChoosePlayer:
					kind = akPlayer
				default:
					s.execGen(k, in)
					continue
				}
			}
			if kind == akNameHand && s.nameHandNone(k, s.perms[sl].ctrl) {
				continue // nothing nonland was revealed: gorge poses no ask
			}
			if kind == akCard && s.enterCardNone(k, sl, c) {
				continue // no object to choose: gorge poses no ask (repl_choosecard.go)
			}
			if kind == akPlayer {
				// gorge asks it with the permanent on the battlefield, never
				// on the stack (repl_chooseplayer.go): queued, or answered
				// in place for a single opponent
				if p := s.soleOpponent(s.perms[sl].ctrl); p != 255 {
					s.perms[sl].chosen = objWord(p)
					s.touchSl(sl)
					continue
				}
				if s.entryNoRun() {
					// no resolution run serves the entry (the sweep's exile
					// return): gorge's body defaults the ask to its first
					// option -- the first living opponent -- and the program
					// continues in place (coldEntryNoRun)
					if q := s.oppsFrom(s.perms[sl].ctrl); q.n > 0 {
						s.perms[sl].chosen = objWord(q.s[0])
						s.touchSl(sl)
					}
					continue
				}
				s.queueAskAt(sl, kind, c, ab, uint8(i+1))
				if !saved.active {
					s.res = saved
				}
				return
			}
			if kind != akCard && kind != akCtrPick && s.g.pre.state == 2 && s.g.pre.obj == s.perms[sl].obj {
				s.perms[sl].chosen = s.g.pre.val // asked on the stack
				s.touchSl(sl)
				s.g.pre.state = 0
				continue
			}
			if kind == akColor && s.entryAskOutstanding() {
				// gorge's one-at-a-time entry path (engine/rules/replacement_etb.go)
				// queues only the CR 303.4f Aura choice behind another decision;
				// every other as-enters ask keeps the pre-existing path, so while
				// a decision is outstanding the entry happens unasked and the
				// recorded colour is the deterministic first colour of the
				// option list (engine/effects/choose.go's first-WUBRG fallback).
				// The queue's parked and queued asks are decisions gorge has
				// already posed: it poses the entry ask at the entry boundary,
				// mid-batch, and the later asks of the batch fall back.
				s.touchSl(sl)
				ex := uint8(in.C) & 31 // the op's Exclude$ colours (W U B R G = bits 0..4)
				fb := uint8(5)
				for cv := uint8(0); cv < 5; cv++ {
					if ex&(1<<cv) == 0 {
						fb = cv
						break
					}
				}
				s.perms[sl].chosen = objWord(fb)
				continue
			}
			ra := &k.t.Cards[c].Abilities[ab]
			if s.entryNoRun() && kind != akCtrPick && kind != akCtrPick2 &&
				!(ra.Repl.Kind == kop.ReplEntersProgram && ra.Repl.C != kop.ReplAfterEnter &&
					i == 0 && progFirstAskKind(&ra.Prog[0]) == kind) {
				// no resolution run serves the entry (the sweep's exile
				// return): gorge's body defaults the ask to its first option
				// and the program continues in place. The one ask that stays
				// queued is the entrant's own enters program's first op when
				// it is a CR 614.12 entry choice: gorge's walker asks it with
				// the card still in its origin zone, posed even with no run
				// (repl_etbchoice's park), so it stays queued. An R:
				// AfterEnter line's choice is not one of those -- enterChoiceKind
				// skips it, so gorge never walker-poses it, and the line's
				// body runs straight off the entry emit with no run and
				// defaults it -- and neither is another permanent's program's
				// first op: the walker poses the entrant's own choice only.
				s.entryAskDefault(k, sl, c, kind, in)
				continue
			}
			s.queueAskAt(sl, kind, c, ab, uint8(i+1))
			if !saved.active {
				s.res = saved
			}
			return
		case kop.OpChooseBranch:
			if in.C != 2 {
				s.execGen(k, in)
				break
			}
			s.siegeModeRepl(sl, c, ab, i) // face_siegemode.go
			if !saved.active {
				s.res = saved
			}
			return
		case kop.OpSkipUnless:
			i += s.replSkip(k, in) // repl_entersunless.go
		case kop.OpUnlessPaid:
			// the shock lands' as-enters pay-life election: answered from
			// preAsk, or queued here mid-resolution (repl_shockland.go)
			res, skip := s.shockAt(k, sl, c, ab, in, i)
			switch res {
			case shockNo:
				s.execGen(k, in)
			case shockQueued:
				if !saved.active {
					s.res = saved
				}
				return
			default:
				i += skip
			}
		default:
			s.execGen(k, in)
		}
	}
	if !saved.active {
		s.res = saved
	}
}

// entryAskOutstanding reports a decision gorge has pending while an enters
// program's as-enters choice arises: a posed ask (res.ask), a parked one
// (preAsk -- gorge poses it at the entry boundary, mid-batch), or a queued
// one (the ask queue gorge drains before the next priority). gorge's
// one-at-a-time entry path makes a non-enchant as-enters choice unasked
// against all of them (engine/rules/replacement_etb.go).
func (s *State) entryAskOutstanding() bool {
	return s.res.ask != raNone || s.g.pre.state != 0 || s.g.nask > 0
}

// progFirstAskKind is the CR 614.12 entry-choice kind of an enters program's
// first op, the one gorge's as-enters-choice walker asks with the card still
// in its origin zone and the move parked (repl_etbchoice's walk: a colour, a
// type, a number, a card name). Its kind, 0 when the first op is not such a
// choice: that body runs straight off the entry emit, and with no resolution
// run serving the entry its asks default instead of being posed.
func progFirstAskKind(in *kop.Instr) uint8 {
	switch in.Op {
	case kop.OpChooseColor:
		return akColor
	case kop.OpChooseType:
		return akType
	case kop.OpExt:
		switch in.Ext() {
		case kop.ExtChooseNumber:
			return akNumber
		case kop.ExtChooseName:
			return nameAskKind(in)
		}
	}
	return 0
}

// entryAskDefault answers an as-enters ask of an entry no resolution run
// serves with gorge's no-host degrade, the pose's first option, and the enters
// program continues in place: the ask is never queued (engine/rules/
// resolution_ask.go's Ask, and the asking primitives' deterministic
// fallbacks). The option list is built from the op at hand (c, in), never
// through the queue-reading builders: a multi-return sweep can hold an earlier
// entry's CR 614.12 entry choice in the queue, and those builders would read
// that card's bounds and colours (numberBounds, colorExclude) or nothing at
// all (enterCardOptions, which reads the queue the no-run path never filled).
// The look/name pair (akNameLook) takes the acknowledgment -- a no-op: the
// chosen byte still holds the looked-at player between the two -- and then the
// name itself.
func (s *State) entryAskDefault(k *prep, sl Slot, c CardID, kind uint8, in *kop.Instr) {
	if kind == akNameLook {
		kind = akNameAny
	}
	var d Decision
	d.Kind = DkChoose
	d.Player = s.perms[sl].ctrl
	switch kind {
	case akCard:
		// gorge's ChooseCard no-host fallback takes the first candidate
		// (choose_control.go's choices[:min]; the one ExtChooseCardEnter
		// lowering is mandatory, Min 1)
		s.enterCards(k, c, s.perms[sl].obj, s.perms[sl].ctrl, &d)
	case akColor:
		// the op's own Exclude$ colours, not colorExclude's queued-ask
		// lookup: gorge's ChooseColor fallback is the first offered colour
		// (choose.go's opts[0])
		ex := uint8(in.C) & 31 // W U B R G = bits 0..4
		for cv := uint8(0); cv < 5; cv++ {
			if ex&(1<<cv) == 0 {
				d.add(MakeAction(AkChoose, 0, 0, cv))
			}
		}
	case akNumber:
		// the op's own bounds, not numberBounds's queued-ask lookup:
		// gorge's ChooseNumber fallback is the list's floor (opts[0])
		for n := int(in.B.N); n <= int(in.C); n++ {
			d.add(MakeAction(AkChoose, 0, 0, uint8(n)))
		}
	default:
		s.askOptions(k, &d, kind) // the name and type builders read no queue
	}
	// The list is never empty here (Decision.Options is a fixed array; the
	// count rides d.N): the akCard arm runs only with a candidate
	// (enterCardNone skips the op), and gorge's no-host fallback over an
	// empty restricted list records the zero code itself -- the first colour
	// W, the number 0 -- which a zero option decodes to.
	a := d.Options[0]
	p := &s.perms[sl]
	s.touchSl(sl)
	if kind == akCard {
		p.chosen = objWord(a.Source()) // the chosen object rides the action's code
	} else if kind == akColor || kind == akNumber || kind == akPlayer || isNameAsk(kind) {
		p.chosen = objWord(a.Mode())
	} else {
		v := int(a.Target())<<8 | int(a.Mode())
		p.chosen = objWord(s.typeVocabCode(k, v))
	}
}

// queueAsk queues an as-enters choice for the permanent at sl (no program
// continuation).
func (s *State) queueAsk(sl Slot, kind uint8) { s.queueAskAt(sl, kind, NoCard, 0, 0) }

func (s *State) queueAskAt(sl Slot, kind uint8, c CardID, ab, pc uint8) {
	if int(s.g.nask) >= len(s.g.ask) {
		s.capFull(StallAsk)
		return
	}
	if s.g.nask == 0 {
		s.g.askPrio = s.prio
	}
	a := &s.g.ask[s.g.nask]
	a.sl, a.ver, a.kind, a.card, a.ab, a.pc, a.ts = sl, s.objs[s.perms[sl].obj].ver, kind, c, objWord(ab), pc, 0
	s.g.nask++
}

// queueAskRet queues the host ask of Aura card o waiting in exile to
// return (sl NoSlot, the card's object in ab; pc set once gorge would drop
// the answer). gorge asks it at once, in the middle of the batch of
// returns, so the hosts are the permanents on the battlefield now (entry
// stamp up to s.ts): one returned later in the same batch (another
// Sheltered by Ghosts' exile) is no answer.
func (s *State) queueAskRet(o Obj) {
	if int(s.g.nask) >= len(s.g.ask) {
		s.capFull(StallAsk)
		return
	}
	if s.g.nask == 0 {
		s.g.askPrio = s.prio
	}
	a := &s.g.ask[s.g.nask]
	a.sl, a.ver, a.kind, a.card, a.ab, a.pc, a.ts = NoSlot, s.objs[o].ver, akEnchant, s.objs[o].card, objWord(o), 0, s.ts
	s.g.nask++
}

// retAuraAsk poses the queued host ask a of a returning Aura (queueAskRet),
// or settles it without one: the card left exile (dropped), nothing it can
// enchant (it stays in exile, CR 303.4g) or one host (it enters attached).
func (s *State) retAuraAsk(k *prep, d *Decision, a *askRec) bool {
	o := Obj(a.ab)
	x := &s.objs[o]
	var buf [MaxOptions]TCode
	n := 0
	if x.zone == kop.ZoneExile && x.ver == a.ver {
		m := s.enchantCands(k, x.card, x.owner(), &buf)
		for i := 0; i < m; i++ {
			if s.perms[s.objs[buf[i]].slot].ts <= a.ts {
				buf[n] = buf[i]
				n++
			}
		}
	}
	if n > 1 {
		d.Player, d.Kind, d.Sub = x.owner(), DkChoose, ChGenEnchant
		for i := 0; i < n; i++ {
			d.add(MakeAction(AkChoose, buf[i], 0, 0))
		}
		return true
	}
	s.popAsk()
	if n == 1 {
		s.enterRetAura(k, o, s.objs[buf[0]].slot)
	}
	return false
}

// enterRetAura puts the returning Aura o onto the battlefield from exile
// attached to host (CR 303.4f).
func (s *State) enterRetAura(k *prep, o Obj, host Slot) {
	from := s.detach(o)
	s.enterOnto(host)
	sl := s.enterFromGrantedZone(k, o, s.objs[o].owner(), false, from)
	s.attachedTo(k, sl, host)
}

// leaveGen runs at the end of the core's removePerm for the object o that left
// slot sl: a permanent exiled "until this leaves" comes back, at once or, in
// an open departure frame, at its end (zone_untilleavesbatch.go).
func (s *State) leaveGen(k *prep, sl Slot, o Obj) {
	if k.gen.anyAegis {
		s.aegisLeaves(k, sl, o) // zone_exilethenattach.go: before returnLinked drops the link's bit
	}
	// The kept Imprinted list is deliberately NOT dropped here: gorge keeps
	// Object.Imprinted through the departure for the leaves trigger that
	// reads it, and clears it at the next battlefield entry (impEnter).
	if k.gen.anyFaces {
		s.faceReset(k, o) // face_multiface.go
	}
	s.clearFx(sl)
	if s.g.batchLatch&batchHeld != 0 {
		s.holdReturns(sl, o)
	} else {
		// An until-leaves return outside an open departure frame is still one
		// entry batch (CR 603.6a, gorge's zone batch): the frame is opened and
		// closed around the return itself, so its cards' enters checks wait for
		// the last one to be back — a newcomer's "whenever another ... enters"
		// sees every other card of the return (Oreplate Pangolin returning
		// beside two other artifacts when Pinnacle Starcage leaves). A return
		// that parks an ask keeps the frame open across it, as holdClose's
		// retHold does for an op-opened frame.
		hr := s.holdEntries()
		s.holdReturns(sl, o)
		s.holdClose(k, hr) // flushReturns returns the source; nothing is held when it has no live link
	}
	s.refreshTy(k)
}

// returnLinked returns what the departed source o exiled until it left; srcCtrl
// is its last controller.
func (s *State) returnLinked(k *prep, srcCtrl uint8, o Obj) {
	for i := 0; i < int(s.g.nlinks); i++ {
		l := s.g.links[i]
		if l.src != o || s.objs[o].ver != l.sver+1 || k.gen.anyRemPersist && k.t.Cards[s.objs[o].card].RemPersist || s.g.linkHide>>uint(i)&1 != 0 {
			continue // (a RemPersist link is its leaves trigger's: zone_rememberpersist.go; a Hideaway link stays: kw_hideaway.go)
		}
		toHand := s.g.linkHand|s.g.linkHide|s.g.linkAeg != 0 && s.dropLinkHand(i) // act_mayask.go
		copy(s.g.links[i:], s.g.links[i+1:s.g.nlinks])
		s.g.nlinks--
		i--
		x := &s.objs[l.obj]
		if toHand {
			if x.zone == kop.ZoneExile {
				from := s.detach(l.obj)
				s.placeFromK(k, l.obj, kop.ZoneHand, false, from, false, false)
			}
			continue
		}
		if x.zone == kop.ZoneExile && s.auraWithHosts(k, l.obj) > 1 {
			// a returning Aura (Witness Protection under Banishing Light):
			// gorge asks its host with the card still in exile (CR 303.4f).
			// During a resolution the kernel parks the choice (preReturn)
			// and enters the card once answered (raGenRetChoice; pair 5605).
			// A cost's pick still being answered when the host leaves -- the
			// sacrificed host is what exiled it -- parks it too: the pending
			// answer continues the resolution, and runProg poses the host ask
			// before it ends (pair 1265). Outside a resolution (its source
			// died to a state-based action) every returning Aura queues its
			// own host ask before the next priority (queueAskRet) instead of
			// entering unattached to fall to CR 704.5m: they are asked in
			// entry order, one answer each.
			parked, queued := false, false
			if s.res.active && s.g.pre.state == 0 {
				s.g.pre = preAsk{obj: l.obj, kind: akEnchant, state: preReturn}
				parked = true
			} else if !s.res.active {
				s.queueAskRet(l.obj)
				queued = true
			}
			if parked || queued {
				if k.gen.anyLinkCtr {
					s.linkRetStall(k, srcCtrl, o, l.obj)
				}
				continue
			}
		}
		if n := s.auraWithHosts(k, l.obj); x.zone == kop.ZoneExile && (n == 1 || n > 1 && !s.res.active) {
			// a returning Aura with one permanent it can enchant enters
			// attached to it (CR 303.4f; Witness Protection under a
			// destroyed Banishing Light, pair 64152)
			ctrN := 0
			if k.gen.anyLinkCtr {
				ctrN = s.linkRetCtrs(k, srcCtrl, o, l.obj) // before the card takes the source's slot
			}
			host := s.firstHost(k, l.obj)
			from := s.detach(l.obj)
			nsl := s.enterLinkRetRun(k, l.obj, x.owner(), from)
			s.attachTo(k, nsl, host)
			if ctrN > 0 {
				s.linkRetPut(k, ctrN, nsl)
			}
			continue
		}
		if x.zone == kop.ZoneExile && k.cards[x.card].types&kop.TAura != 0 && k.t.Cards[x.card].Enchant.Kind == kop.TgtObject && s.auraWithHosts(k, l.obj) == 0 {
			continue // nothing it can enchant: it stays in exile (CR 303.4g; pair 74242)
		}
		if x.zone == kop.ZoneExile {
			if s.instantStays(k, l.obj, kop.ZoneBattlefield) {
				// CR 400.4a: an instant or sorcery card that would enter the
				// battlefield stays in exile (gorge sweepExileReturn's
				// effects.InstantCannotEnter gate): a card exiled face down
				// while manifested (Emerge from the Cocoon under a Sheltered
				// by Ghosts) never comes back, so its return rider never
				// runs (CR 610.3: nothing enters "this way")
				continue
			}
			// the clone election comes first: a card whose enters replacement
			// opens with OpBecomeCopy (Naga Fleshcrafter) has no
			// enterChoiceKind, and enterChoiceKind alone would miss it
			kind := uint8(0)
			if !DebugCloneReturnNoAsk && k.gen.has(x.card, gfEnterRepl) {
				kind = s.cloneAskKind(k, x.card)
			}
			if kind == 0 {
				kind = s.enterChoiceKind(k, x.card)
			}
			if kind != 0 && s.res.active && (s.res.ask == raNone || s.res.ask == raGenPayPick || s.res.ask == raGenTopBottom) && s.g.pre.state == 0 {
				// during a resolution gorge asks the as-enters choice with
				// the card still in exile (Heraldic Banner, pair 544); it
				// enters once answered (runProg, raGenRetChoice); so does one whose
				// source a cost's pick still being answered sacrificed (In the Pale
				// Moonlight's Malfunctioning Holodeck), as the Aura's host ask above;
				// so does one whose exiler leaves inside a top/bottom answer (Temporal
				// Cleansing), before continueRes clears the ask: parkReturnAsk poses it.
				// A "second from top or bottom" answer (raGenTopBottom) moves
				// the source inside its own answer and gorge asks the return's
				// choice right after that move (CR 616.1, 610.3d; ecl 2517's
				// Liminal Hold returning a Rimefire Torque): the answer resumes
				// through continueRes, which poses the parked ask.
				s.g.pre = preAsk{obj: l.obj, kind: kind, state: preReturn, link: kind == akClone}
				if k.gen.anyLinkCtr {
					s.linkRetStall(k, srcCtrl, o, l.obj)
				}
				continue
			}
			if kind := s.enterChoiceKind(k, x.card); kind != 0 && s.parkReturnCost(k, srcCtrl, o, l.obj, kind) {
				continue // a cost's sacrifice outside a resolution: asked before priority (zone_untilleavescost.go)
			}
			ctrN := 0
			if k.gen.anyLinkCtr {
				ctrN = s.linkRetCtrs(k, srcCtrl, o, l.obj) // before the card takes the source's slot
			}
			from := s.detach(l.obj)
			nsl := s.enterLinkRetRun(k, l.obj, x.owner(), from)
			if ctrN > 0 {
				s.linkRetPut(k, ctrN, nsl)
			}
		}
	}
}

// enterLinkRetRun enters the returning card o and marks the entry's enter
// programs no-run when no resolution run serves the return (a departed
// player's sweep, or a loss site's monarch handoff outside a resolution):
// gorge runs such a replacement body straight off the emit with no run, so
// every ask it poses defaults to its first option and the program continues
// in place (coldEntryNoRun).
//
//go:noinline
func (s *State) enterLinkRetRun(k *prep, o Obj, p uint8, from kop.Zone) Slot {
	noRun := !s.res.active
	if noRun {
		s.setEntryNoRun(1)
	}
	nsl := s.enterLinkRet(k, o, p, from)
	if noRun {
		s.setEntryNoRun(0)
	}
	return nsl
}

// firstHost is the first permanent the returning Aura card o can enchant
// in gorge's battlefield order (player 0's list, then player 1's, each in
// entry order), NoSlot when none.
func (s *State) firstHost(k *prep, o Obj) Slot {
	var buf [MaxOptions]TCode
	host := NoSlot
	n := s.enchantCands(k, s.objs[o].card, s.objs[o].owner(), &buf)
	for i := 0; i < n; i++ {
		if h := s.objs[buf[i]].slot; host == NoSlot || bfBefore(&s.perms[h], &s.perms[host]) {
			host = h
		}
	}
	return host
}

// bfBefore reports whether p comes before q in gorge's battlefield order.
func bfBefore(p, q *perm) bool {
	return p.ctrl < q.ctrl || p.ctrl == q.ctrl && p.ts < q.ts
}

// auraWithHosts is the number of objects returning Aura card o could
// enchant, or 0 when o is no Aura.
func (s *State) auraWithHosts(k *prep, o Obj) int {
	c := s.objs[o].card
	if k.cards[c].types&kop.TAura == 0 || k.t.Cards[c].Enchant.Kind != kop.TgtObject {
		return 0
	}
	var buf [MaxOptions]TCode
	return s.enchantCands(k, c, s.objs[o].owner(), &buf)
}

// newTurnGen resets the per-turn generated state (the core's beginTurn).
func (s *State) newTurnGen() {
	s.g.died = 0
	s.g.fog = false
	s.g.noPoison = 0
	s.g.plusCtr[0], s.g.plusCtr[1] = 0, 0
	s.turnReplEnd(false) // repl_turnrepl.go
	if s.ndly != 0 {
		s.dropTurnLeave() // trig_leavereturn.go: "this turn" leave records end
	}
	clear(s.g.prevCol[:])
	clear(s.g.prevX[:])
	clear(s.g.redir[:])
	if s.g.ctrAdd.pl|s.g.ctrAdd.n != 0 {
		s.ctrAddedReset() // ctr_addedthisturn.go
	}
	s.g.attacked = [MaxSeats]uint8{}
	s.g.oppAtt = 0
	s.g.isCast = [MaxSeats]uint8{}
	s.g.hist = [MaxSeats][3]uint8{}
	clear(s.cold.castRing)
	s.g.actRing = [MaxSeats][actRingN]Obj{}
	s.clearGrantActs()
	if k := s.k(); k.gen != nil && k.gen.anyTurnF {
		s.g.turnF = 0 // coin flips and Exhaust activations end with the turn (static_turnbits.go)
	}
	s.g.spent = [MaxSeats]uint16{}
	if k := s.k(); k.gen != nil && k.gen.anyGameOnce {
		s.keepGameOnce() // GameActivationLimit records outlive the turn (trig_params.go)
	} else {
		s.g.nused = 0
	}
	s.g.zoneTurn = objBits{}
	s.g.dmgTurn = objBits{}                        // WasDealtDamageThisTurn clears with the marked damage (gorge TurnChange)
	s.g.turnF &^= turnDamagedMask(allSeatsDamaged) // the seats' WasDealtDamageThisTurn clear with it (gorge TurnChange)
	clear(s.g.dmgAmt[:])                           // DamageReceivedThisTurn clears with it (gorge TurnChange; value_dmgamount.go)
	s.g.enteredN = 0
	s.g.enteredFull = false
	s.mSlZero(s.k(), mechExileEx)
	s.mSlZero(s.k(), mechNoRegen) // repl_noregen.go
	clear(s.cold.exMark)
	s.mustBlockNewTurn(s.k()) // combat_mustbeblocked.go
	if k := s.k(); k.gen != nil && (k.gen.anyPermMark || k.gen.anyTappedTurn || k.gen.anyBtTurn) {
		s.permMarkNewTurn(k) // filter_permmarks.go
	}
	if k := s.k(); k.gen != nil && (k.gen.anyAttackedObj || k.gen.anyCombatHit) {
		s.turnTalliesNewTurn(k) // value_turntallies.go
	}
	if k := s.k(); k.gen != nil && k.gen.anyAddPhase {
		s.addPhaseNewTurn() // act_addphase.go
	}
	s.mObjZero(s.k(), mechMayhem) // the discard-this-turn marks end with the turn (CR 702.187a; gorge's TurnChange bound)
	s.g.trigTurn = 0
	s.g.dilemmas = [MaxSeats]uint8{}
	if s.g.nchPick != 0 {
		s.charmPickTurn() // modal_charmpick.go: ChoiceRestriction$ ThisTurn picks end
	}
	s.g.crime = crimeKeep(s.g.crime)
	if s.g.ndp != 0 {
		s.dropDelayedSpell()       // ThisTurn$ SpellCast registrations (trig_delayedspell.go)
		s.dropDelayedDies()        // ThisTurn$ dies registrations (trig_delayeddies.go)
		s.dropDelayedBeginCombat() // ThisTurn$ beginning-of-combat registrations (trig_delaybegincombat.go)
	}
}

// touchBits marks every object in b changed. A generated-state clear that
// changes a view of an object without writing the object itself must call
// it, so a reader caching per object (dirac/feat) rebuilds that view.
func (s *State) touchBits(b *objBits) {
	for i, w := range b {
		for w != 0 {
			s.touch(Obj(i*64 + bits.TrailingZeros64(w)))
			w &= w - 1
		}
	}
}

// cleanupGen ends "until end of turn" generated effects (the core's
// finishCleanup).
func (s *State) cleanupGen(k *prep) {
	s.g.plusCtr[0], s.g.plusCtr[1] = 0, 0 // Brad Boimler's until-end-of-turn counters-plus-one expires
	s.turnReplEnd(true)                   // repl_turnrepl.go: Prairie Dog's and Malicious Eclipse's too
	// A granted flashback is part of a graveyard card's view although no
	// object field names it: mark every card whose grant this clear ends
	// before dropping the bitsets.
	s.fbClearAll(k)
	s.encoreClearAll(k)
	s.embalmClearAll(k)
	if s.g.nfx > 0 {
		var ended slotMask
		for i := 0; i < int(s.g.nfx); i++ {
			if r := &s.g.fx[i]; ovCleanupEnds(k, r) {
				ended = ended.with(r.sl)
			}
		}
		s.dropFx(func(i int) bool { return ovCleanupEnds(k, &s.g.fx[i]) })
		if k.gen.anyNTOv {
			ended = ended.or(s.dropNextTurnOv()) // Duration$ UntilYourNextTurn (static_control.go)
		}
		s.copyExcKwRestore(k, ended)
	}
	if k.gen.anyEOTEmb {
		s.dropEOTEmblems(k)
	}
	if k.gen.anyExHaste {
		s.exhClear() // static_cantblockexcept.go
	}
	if k.gen.anyColGrant {
		s.cgClear()       // static_colourgrant.go
		s.colProtClear(k) // static_colourprot.go
	}
	if k.gen.anyKwMark {
		s.kwMarkClear(k) // kw_kwmark.go
	}
	if k.gen.anyDmgMark {
		s.dmgMarkEnd() // Duration$ UntilYourNextTurn (repl_dmgmark.go)
	}
	if k.gen.anyNTEmb {
		s.dropNextTurnEmblems(k) // Duration$ UntilYourNextTurn (static_effectdelivered.go)
	}
	if k.gen.anyNTEndEmb {
		s.dropEndOfNextTurnEmblems(k) // Duration$ UntilTheEndOfYourNextTurn (static_emblemdurations.go)
	}
	n := 0
	for i := 0; i < int(s.g.nmay); i++ {
		r := *s.mayAt(i)
		keep := r.dur() == kop.DurYourNextTurn && !(s.active == r.ctrl && mayWord(s.turn) > r.turn) ||
			r.until == kop.DurWarp && s.warpRecastLive(r) ||
			r.until == kop.DurAirbend && s.warpRecastLive(r) ||
			r.until == kop.DurPlot && s.plotLive(r) ||
			r.until == kop.DurSuspend && s.suspendLive(r) || // a suspended card stays while exiled, its haste while it stays (kw_suspend.go)
			r.until == kop.DurForetell && s.plotLive(r) ||
			r.until == kop.DurPrepared && s.preparedSrc(r) ||
			r.dur() == kop.DurWhileSrcMay && s.mayWhileSrcKeep(r) || // cast_whilesource.go
			r.dur() == kop.DurWhileValid && s.plotLive(r) ||
			r.until == kop.DurPermanent && s.plotLive(r) // a while-exiled permission (cast_whileexiled.go)
		if keep {
			r.until &^= mayUntilUsed // a grant's cap counts per turn (cast_mayplaylimit.go)
			*s.mayAt(n) = r
			n++
		}
	}
	s.g.nmay = uint8(n)
	s.refreshTy(k)
}

// fireDelayedGen puts the delayed programs on the stack at the beginning of
// the end step (as gorge's delayed triggers).
func (s *State) fireDelayedGen(k *prep) {
	if k.gen.anyDelayUp {
		s.fireDelayedWhen(k, 0)
		return
	}
	n := s.g.ndp
	// ndp drops after the loop, not before: until each program's LKI index
	// is copied into its pending trigger, the programs hold their records
	// (an enqueue's snapshot sweeps the pool, zone_lki.go)
	for i := 0; i < int(n); i++ {
		d := s.g.dp[i]
		m := s.npend
		ro, remVer, remSet := s.delayRem(k, &d)
		var amt int16
		if d.n&dpCountMask == 2 {
			amt = s.dpSecond(&d) // Mobilize 2's second token (kw_mobilize.go)
		}
		s.enqueue(d.src, d.sv, d.card, d.ab, d.ctrl, ro, d.ctrl, amt)
		if s.npend > m {
			s.pend[m].li = s.g.dpLKI[i]
			s.pend[m].remVer, s.pend[m].skw = objWord(remVer), s.pend[m].skw&^skwRem|boolBit(remSet, skwRem)
			if d.n&dpMany != 0 {
				s.dpManyCarry(&d, &s.pend[m]) // trig_delayedmany.go
			}
		}
	}
	s.g.ndp = 0
}

// delayRem is what delayed program d remembers as it fires, with the version it
// was registered at and whether a read must match it. An object that changed
// zones since is a new one, with nothing to return (exiled from the graveyard,
// CR 400.7), but a copy of it reads last known information (CR 608.2h).
func (s *State) delayRem(k *prep, d *dprog) (Obj, uint8, bool) {
	ro := d.objs[0]
	switch {
	case ro == NoObj:
	case delayCopies(k, d.card, d.ab):
		return ro, d.vers[0], false
	case s.objs[ro].ver == d.vers[0]:
		return ro, d.vers[0], true
	}
	return NoObj, d.vers[0], false
}

// delayCopies reports whether a card's delayed ability copies its remembered
// object.
func delayCopies(k *prep, card CardID, ab uint8) bool {
	for i := range k.t.Cards[card].Abilities[ab].Prog {
		in := &k.t.Cards[card].Abilities[ab].Prog[i]
		if (in.Op == kop.OpCopyAsToken || in.Op == kop.OpCopyAsTokenEx) && in.Obj == kop.RefDelayedRemembered {
			return true
		}
	}
	return false
}

// stepGen runs before each of the core's control states: it poses a pending
// generated-table decision or starts a queued action. It returns true with
// d filled when a decision is posed.
func (s *State) stepGen(k *prep, d *Decision) bool {
	if s.ctl == ctlGenAsk {
		return s.genAskOptions(k, d)
	}
	s.refreshTy(k)
	if s.g.stalled != 0 {
		s.stall(s.g.stalled) // a pool was full (capFull): more simultaneous triggers than MaxPending, ...
		d.Kind = DkNone
		return true
	}
	if s.g.trigTurn > maxTrigTurn && !s.res.active {
		// a mandatory loop that never ends the game (Marauding Blight-Priest
		// with Bloodthirsty Conqueror against "can't lose"): CR 104.4b draw
		s.stall(StallLoop)
		d.Kind = DkNone
		return true
	}
	if (s.ctl == ctlPriority || s.ctl == ctlPlaceTriggers) && s.sp >= MaxStack && !s.res.active {
		s.stall(StallStack) // nothing more fits on the stack: the kernel's capacity
		d.Kind = DkNone
		return true
	}
	if s.ctl != ctlPriority || s.res.active {
		return false
	}
	if s.poseReturnCost(k, d) {
		return true // zone_untilleavescost.go
	}
	if s.g.endTurn {
		s.clearCombat() // CR 723.1b: every creature is removed from combat
		// a state-based action still due is performed before the step: inside
		// it, CR 514.3a would give priority for it
		if s.sba(k) || s.ctl != ctlPriority {
			return false
		}
		if s.g.nask == 0 {
			s.g.endTurn = false
			if k.gen != nil && k.gen.anyTokEndCombat {
				s.dropDlyEndCombat() // gorge's |TT= expiry: the turn ends before its end of combat
			}
			s.endStep(kop.StepCleanup)
			return false
		}
	}
	if s.g.ro.n > 0 {
		// a noncombat damage event parked on gorge's replacement-order ask
		// (parkNC): asked once the resolution is done, before priority
		s.g.ro.nca = true
		s.g.ctlRet = s.ctl
		s.ctl = ctlGenAsk
		return s.genAskOptions(k, d)
	}
	if s.g.nask > 0 || s.g.leyOn {
		s.g.ctlRet = s.ctl
		s.ctl = ctlGenAsk
		if s.g.nask > 0 && s.g.priSave == 0 {
			s.g.priSave = 0x80 | s.prio
			s.prio = s.g.askPrio
		}
		return s.genAskOptions(k, d)
	}
	if s.g.nfq > 0 && !s.g.hold && int(s.sp) < MaxStack && s.holdable() {
		// gorge casts them while the ability (Etali's trigger) or spell
		// (Walk with the Ancestors' discover) is still resolving: it stays
		// on the stack under them until the last cast
		h := s.res.e
		if h.kind != seSpell {
			h.flags |= sfHold // a spell's bit is sfPromised: its hold is res.e (spellHeld)
		}
		s.push(h)
		s.g.hold = true
		if s.g.priSave == 0 {
			// the casts are made mid-resolution: the holder is unchanged
			s.g.priSave = 0x80 | s.prio
			s.prio = s.g.holdPrio
		}
	}
	if s.g.nfq == 0 && s.res.ask == raGenFreeParked {
		s.resumeFreeCast(k) // the casts are made: the parked tail runs (act_discoverfreecast.go)
		return false
	}
	if s.g.nfq == 0 && s.g.hold {
		s.dropHold(k)
	}
	for s.g.nfq > 0 {
		o := s.g.freeQ[0]
		copy(s.g.freeQ[:], s.g.freeQ[1:s.g.nfq])
		s.g.nfq--
		c := s.objs[o].card
		// freeP: the caster, the zone, ReplaceGraveyard, CopyCard, paid and
		// ForgetPlayed (cast_freecast.go)
		ctrl := uint8(s.g.freeP & freeSeatMask)
		if k.cards[c].land && s.g.freeP&freeLandsOn != 0 {
			// a picked land (OpCastFree freeLands) is played, not cast
			if !s.freeLandPlay(k, o, ctrl) {
				return s.stepGen(k, d) // the entry's ask is posed now (cast_freeland.go), or the latched stall
			}
			continue
		}
		if s.g.freeP&freeCopy != 0 {
			paid := s.g.freeP&freePaid != 0
			// gorge casts the copy whatever became of the original and mints
			// it at the cast's start, so begin first: a paid cast the pool
			// cannot cover is begun and reversed after its mana window
			// (CR 733.1, cast_exilegrant.go), and a required target aborts it
			// at the target ask first, tapping nothing. The build also has to
			// be live for the census the abort reads (windowUnpricedOpen).
			if !s.beginCopyCast(k, o, ctrl) {
				if s.over {
					return false
				}
				continue
			}
			if paid && s.copyCastAbortsBeforeWindow(k, c, ctrl, o) ||
				!s.spellTargetsFeasible(k, c, ctrl) || !s.spellCostsFeasible(k, c, ctrl, o) {
				// the copy is minted and ceases in its holding zone, no
				// source tapped: gorge's abort at the target ask
				s.reverseCast(k)
				continue
			}
			if paid {
				// gorge pays a Play's cast through the manual mana window, no
				// payment plan (cast_exilegrant.go)
				s.bld().bfl |= bflLowered
			} else {
				s.bld().e.flags |= sfFree
			}
			return false
		}
		if s.objs[o].zone != kop.Zone((s.g.freeP>>freeZoneShift)&7) || !s.spellTargetsFeasibleFrom(k, c, ctrl, o) {
			continue // no legal target any more: the card stays in its zone
		}
		// freePaid without freeCopy is a Play without WithoutManaCost$ True
		// (Vaan, Street Thief's dug card, Chandra Acolyte's targeted graveyard
		// card): gorge's beginPlay begins the ordinary cast paying its printed
		// cost (CR 601.2, alternative timing). Mid-resolution no payment plan
		// exists (gorge plans offers only), so the cast is lowered: the manual
		// CR 601.2g window pays what the pool does not, and a cost neither can
		// cover reverses the cast (CR 733.1), the card back in its zone and
		// still remembered (the forget only commits with the cast).
		if s.g.freeP&freePaid != 0 {
			holder := s.prio
			s.prio = ctrl // the caster (beginCast reads it)
			s.beginCast(k, o, 0, 0)
			s.prio = holder
			// gorge's target ask gates an unpayable paid cast before its menu
			// (CR 601.2c, 733.1) and the CR 601.2g window reverses one it
			// cannot cover; both read s.g.freeP&freePaid at the cast flow's
			// target stage (cast.go bsTargets, playTargetAborts).
			s.bld().bfl |= bflLowered // no plan: the window pays (cast_exilegrant.go's copy does the same)
			return false
		}
		holder := s.prio
		s.prio = ctrl // the caster (beginCast reads it)
		s.beginCast(k, o, castFree, 0)
		s.prio = holder
		return false
	}
	// the queue is drained: the play's register has served its casts (a
	// committed card was dropped in cast.go finishBuild; a refused or
	// reversed one stays remembered). The whole register dies with the
	// queue: ReplaceGraveyard$ Exile belongs to the played spells (gorge
	// state.ExilesInsteadOfGraveyard is per cast), and a stale bit would
	// exile a later unrelated free cast's spell on leave instead of its
	// owner's graveyard (CR 701.5a).
	s.g.freeP = 0

	if s.res.ask == raGenFreeParked {
		s.resumeFreeCast(k) // the rest were refused: the parked tail runs
		return false
	}
	if s.g.hold {
		s.dropHold(k) // the rest were refused (no legal target): the ability is done
	}
	return false
}

func (s *State) genAskOptions(k *prep, d *Decision) bool {
	if s.res.ask == raDredgeRes && s.res.flags&(rfBodyOp|rfBodyTurn) != 0 {
		s.dredgeResOptions(k, d) // kw_dredge.go: a draw-replacement body's ask, before dredgeTurnAsked (the first body card's ask is still the draw step's first)
		return true
	}
	if s.dredgeTurnAsked() {
		s.dredgeOptions(k, s.active, d) // kw_dredge.go
		return true
	}
	if s.g.untapOn && s.g.untapAns == 0 {
		s.untapAskOptions(d)
		return true
	}
	if k.gen.anySuspend && s.g.nmay > 0 && s.suspendPosedOptions(d) {
		return true
	}
	if s.entryAsked(k) {
		s.entryOrderOptions(k, d)
		return true
	}
	if s.g.ro.n > 0 && (s.g.ro.on || s.g.ro.nca) {
		s.replOrderOptions(d)
		return true
	}
	if s.g.pre.state == preReturnCost {
		// a cost's sacrifice outside a resolution returns a card with an
		// as-enters choice: posed once the build is done and rebuilt
		// identically while it waits (zone_untilleavescost.go)
		d.Kind, d.Player = DkChoose, s.objs[s.g.pre.obj].owner()
		s.askOptions(k, d, s.g.pre.kind)
		return true
	}
	if s.g.pre.state == 1 {
		d.Kind = DkChoose
		if s.g.pre.kind == akEntryOrder {
			d.Player = s.cloneEntryCtrl()
			d.Sub = ChGenReplOrder // gorge's KReplacement order ask (repl_entryclone.go)
			s.cloneEntryOrderOptions(k, d)
			return true
		}
		if s.g.pre.kind == akEtbOrder {
			d.Player = s.stack[s.sp-1].ctrl
			d.Sub = ChGenReplOrder // gorge's KReplacement order ask (repl_etborder.go)
			s.etbOrderOptions(k, d)
			return true
		}
		if s.g.pre.ley {
			d.Player = s.g.leyP // the opening round's seat still holds the offer
		} else if s.g.pre.land {
			d.Player = s.preLandSeat() // the land's player still holds priority (a Play's picked land: its caster)
		} else {
			d.Player = s.stack[s.sp-1].ctrl
		}
		s.askOptions(k, d, s.g.pre.kind)
		return true
	}
	for s.g.leyOn {
		// one yes/no per Leyline in hand, hand order, starting player first
		// (gorge's opening_yes / opening_no); a Gemstone's hand-exile
		// pick comes first (zone_gemstone.go)
		if s.g.ley[2] != 0 {
			s.leyGemOptions(d)
			return true
		}
		if o := s.leyCard(k, s.g.leyP, s.g.ley[1]); o != NoObj {
			d.Kind, d.Sub, d.Player = DkChoose, ChGenLeyline, s.g.leyP
			d.add(MakeAction(AkChoose, TCode(o), 0, 1))
			d.add(MakeAction(AkChoose, TCode(o), 0, 0))
			return true
		}
		s.nextLeyline(k)
		if !s.g.leyOn {
			return false
		}
	}
	for s.g.nask > 0 {
		a := &s.g.ask[0]
		if a.sl == NoSlot {
			if s.retAuraAsk(k, d, a) {
				return true
			}
			continue
		}
		ob := &s.objs[s.perms[a.sl].obj]
		if !s.occ.has(a.sl) || ob.ver != a.ver {
			s.popAsk()
			continue
		}
		d.Player = s.perms[a.sl].ctrl
		d.Kind = DkChoose
		s.askOptions(k, d, a.kind)
		return true
	}
	s.ctl = s.g.ctlRet
	if s.g.priSave != 0 {
		s.prio, s.g.priSave = SeatIx(s.g.priSave), 0
	}
	return false
}

func (s *State) popAsk() {
	copy(s.g.ask[:], s.g.ask[1:s.g.nask])
	s.g.nask--
}

func (s *State) applyGen(k *prep, a Action) {
	if s.res.ask == raDredgeRes && s.res.flags&rfBodyTurn != 0 {
		s.dredgeResAnswer(k, a) // kw_dredge.go: a draw-replacement body's ask, before dredgeTurnAsked (the first body card's answer is still the draw step's first)
		return
	}
	if s.dredgeTurnAsked() {
		s.dredgeTurnAnswer(k, a) // kw_dredge.go
		return
	}
	if s.g.untapOn && s.g.untapAns == 0 {
		s.untapAnswer(a)
		return
	}
	if k.gen.anySuspend && s.g.nmay > 0 && s.applySuspendAnswer(k, a) {
		return
	}
	if s.entryAsked(k) {
		s.applyEntryLand(k, a)
		return
	}
	if s.g.ro.n > 0 && (s.g.ro.on || s.g.ro.nca) {
		s.applyReplOrder(a)
		return
	}
	if s.answerReturnCost(k, a) {
		return // zone_untilleavescost.go
	}
	if s.g.pre.state == 1 {
		if s.g.pre.kind == akNameLook {
			s.g.pre.kind = akNameAny // the look is acknowledged: the name is asked next (repl_choosename.go)
			return
		}
		if s.g.pre.kind == akEntryOrder {
			s.cloneEntryOrderAnswer(k, a) // the order answer (repl_entryclone.go)
			return
		}
		if s.g.pre.kind == akEtbOrder {
			s.etbOrderAnswer(k, a) // the order answer (repl_etborder.go)
			return
		}
		if s.g.pre.ask2 != 0 && s.g.pre.kind == s.g.pre.ask2 {
			// the copy template's own choice (copy_templatechoice.go)
			s.g.pre.val2 = a.Mode()
			if s.g.pre.kind == akType {
				s.g.pre.val2 = s.typeVocabCode(k, int(a.Target())<<8|int(a.Mode()))
			}
			s.g.pre.kind = akClone
		} else {
			s.g.pre.val = objWord(a.Mode())
			if s.g.pre.kind == akClone {
				s.g.pre.val = cloneAnswer(a)
				if !s.g.pre.land && s.cloneTemplateAsk(k) {
					return // the template's choice is asked next
				}
			}
			if s.g.pre.kind == akType {
				s.g.pre.val = objWord(s.typeVocabCode(k, int(a.Target())<<8|int(a.Mode())))
			}
		}
		s.g.pre.state = 2
		if s.g.pre.ley {
			s.leyPreEnter(k) // the opening hand's Leyline enters with the choice made
		} else {
			s.ctl = ctlPriority
			if s.g.pre.land {
				s.playLandNow(k, s.g.pre.obj, s.preLandSeat()) // the land enters with the choice made
			} else {
				s.resolveTop(k) // the permanent enters with the choice made
			}
		}
		// a second pre-ask posed by the resolution (the entry order) stays
		// posed; otherwise the answered choice is done. The CR 616.1
		// entry-order ask parked the play instead: the answer stays put
		// (state 2) and the entering program's record body applies it once
		// the order is picked (repl_entryorder.go, runReplProgAs).
		if s.g.pre.state != 1 && !s.entryAsked(k) {
			s.g.pre = preAsk{}
		}
		return
	}
	if s.g.leyOn {
		if s.g.ley[2] != 0 {
			s.leyGemAnswer(k, a) // zone_gemstone.go
			return
		}
		if a.Mode() == 1 {
			o := Obj(a.Source())
			// CR 614.12: the as-enters choice is part of entering, made while
			// the card is still in its origin zone. gorge asks it at the
			// entry boundary, so park the pre-ask with the card still in the
			// opening hand and let the answer enter it (leyPreEnter).
			if kind := s.enterChoiceKind(k, s.objs[o].card); kind != 0 && !DebugLeylineMoveBeforeAsk {
				s.g.pre = preAsk{obj: o, kind: kind, state: 1, ley: true}
				return
			}
			from := s.detach(o)
			s.leyGemEnter(k, o, s.enter(k, o, s.g.leyP, false, from))
		} else {
			s.g.ley[1]++ // declined: it stays in hand, ask the next one
		}
		return
	}
	if s.g.nask > 0 && s.g.ask[0].kind == akNameLook {
		s.g.ask[0].kind = akNameAny // the look is acknowledged: the name is asked next
		return
	}
	s.answerQueuedAsk(k, a)
	if s.g.nask == 0 {
		s.ctl = s.g.ctlRet
		if s.g.priSave != 0 {
			s.prio, s.g.priSave = SeatIx(s.g.priSave), 0
		}
	}
}

// answerQueuedAsk applies answer a to the first queued as-enters choice
// and runs the rest of its enters program.
func (s *State) answerQueuedAsk(k *prep, a Action) {
	ask := s.g.ask[0]
	if ask.kind == akNameLook {
		// The look at the opponent's hand is acknowledged before the name is
		// asked (gorge poses the two choices separately). It is answered
		// while the creating ability is still resolving, so the queued ask
		// stays: popping it here would read the acknowledgement as the name
		// answer and finish the resolution early (CR 608.2n).
		s.g.ask[0].kind = akNameAny
		return
	}
	s.popAsk()
	if ask.sl == NoSlot {
		// the host answered (retAuraAsk); every returning Aura has its own
		// queued ask, so the answer enters that Aura attached to the chosen
		// host (CR 303.4f)
		host := s.objs[a.Source()].slot
		s.enterRetAura(k, Obj(ask.ab), host)
		return
	}
	p := &s.perms[ask.sl]
	s.touchSl(ask.sl)
	if ask.kind == akPayLife || ask.kind == akReveal || ask.kind == akRevPick {
		if ask.kind == akPayLife {
			s.shockAnswer(k, ask, a) // repl_shockland.go
		} else if !s.revealAnswer(k, ask, a) { // repl_revealland.go
			return
		}
		if ask.ts&askEntryTapped != 0 {
			s.perms[ask.sl].flags |= pfTapped
		}
		if ask.ts&askLandPlay != 0 {
			// a played land's election: the answer completes the play
			s.pl[s.perms[ask.sl].ctrl].landsPlayed++
		}
		return
	}
	if ask.kind == akCtrPick || ask.kind == akCtrPick2 {
		if !s.ctrPickAnswer(k, ask, a) { // repl_enterctrpick.go
			return // the second pick is asked
		}
	} else if ask.kind == akUnleash {
		s.unleashAnswer(k, ask, a) // kw_unleash.go
	} else if ask.kind == akMode {
		s.siegeModeStore(ask.sl, a.Mode())
	} else if ask.kind == akProtector {
		s.protectorStore(ask.sl, a.Mode()) // face_battle.go
	} else if ask.kind == akCard {
		p.chosen = objWord(a.Source()) // the chosen object rides the action's code (repl_choosecard.go)
	} else if ask.kind == akColor || ask.kind == akNumber || ask.kind == akPlayer || isNameAsk(ask.kind) {
		p.chosen = objWord(a.Mode())
	} else {
		v := int(a.Target())<<8 | int(a.Mode())
		p.chosen = objWord(s.typeVocabCode(k, v))
	}
	if ask.card != NoCard {
		saveRep := s.g.replaced
		s.g.replaced = p.obj
		n0 := s.g.nask
		s.runReplProg(k, ask.sl, ask.card, uint8(ask.ab), ask.pc)
		s.g.replaced = saveRep
		if ask.ts&askETBDeferred != 0 {
			s.deferredETBFire(k, ask, n0) // ctr_choice.go
		}
	}
}

// leyPreEnter finishes a Leyline whose as-enters choice was asked while it
// was still in the opening hand: the answer is in s.g.pre.val, and the enters
// program consumes it as a played land's does. The opening round then asks
// the next Leyline or begins the turn (the generator is still in its ask
// phase with leyOn set).
func (s *State) leyPreEnter(k *prep) {
	o := s.g.pre.obj
	from := s.detach(o)
	s.leyGemEnter(k, o, s.enter(k, o, s.g.leyP, false, from))
	s.g.ctlRet, s.ctl = ctlStepBegin, ctlGenAsk
}

// DebugLeylineMoveBeforeAsk (TestLeylineOpeningChoiceBeforeMoveMutation):
// moves an opening-hand Leyline onto the battlefield before its as-enters
// choice, the timing this change fixes.
var DebugLeylineMoveBeforeAsk bool

// leylineStart begins the opening-hand Leyline asks (gorge's
// MayEffectFromOpeningHand) before the first turn; false when there is
// nothing to ask.
func (s *State) leylineStart(k *prep) bool {
	if k.gen == nil || !k.gen.leyline || s.g.leyDn {
		return false
	}
	s.g.leyDn = true
	if !s.anyLeyline(k) {
		return false
	}
	s.g.leyP = s.startPl
	s.g.ley = [MaxSeats]uint8{}
	s.g.leyOn = true
	s.g.ctlRet = ctlStepBegin
	s.ctl = ctlGenAsk
	return true
}

// anyLeyline reports whether any seat holds a Leyline in its opening hand:
// gorge's opening round asks every seat (AliveFrom the starting player).
func (s *State) anyLeyline(k *prep) bool {
	for p := uint8(0); p < s.nSeats(); p++ {
		if s.hasLeyline(k, p) {
			return true
		}
	}
	return false
}

func (s *State) hasLeyline(k *prep, p uint8) bool {
	pl := &s.pl[p]
	for i := 0; i < int(pl.handN); i++ {
		if s.leyEligible(k, p, pl.hand[i]) {
			return true
		}
	}
	return false
}

// leyCard is player p's Leyline in hand after skipping the first skip
// (the declined ones), NoObj when none is left.
func (s *State) leyCard(k *prep, p, skip uint8) Obj {
	pl := &s.pl[p]
	for i := 0; i < int(pl.handN); i++ {
		if o := pl.hand[i]; s.leyEligible(k, p, o) {
			if skip == 0 {
				return o
			}
			skip--
		}
	}
	return NoObj
}

// nextLeyline moves the Leyline asks to the next player, or starts the
// game. ley[0] is 1 once the starting player is done, ley[1] the current
// player's declined Leylines.
func (s *State) nextLeyline(k *prep) {
	if s.duel() {
		if s.g.ley[0] == 0 {
			s.g.ley = [MaxSeats]uint8{1, 0}
			s.g.leyP = s.nextSeat(s.startPl)
			return
		}
	} else if p := s.nextAlive(s.g.leyP); p != s.startPl {
		// gorge asks over AliveFrom(starting player)
		s.g.ley = [MaxSeats]uint8{1, 0}
		s.g.leyP = p
		return
	}
	s.g.leyOn = false
	s.g.exBase = s.startPl
	s.beginTurn(s.startPl)
}

// tappedMask is player p's tapped permanents.
func (s *State) tappedMask(p uint8) slotMask {
	var m slotMask
	for q := s.ctrlM[p]; q.any(); q = q.rest() {
		sl := q.first()
		if s.perms[sl].flags&pfTapped != 0 {
			m = m.with(sl)
		}
	}
	return m
}

// askOptions fills an as-enters colour (W U B R G as mode 0..4) or creature
// type (gorge's vocabulary index as target<<8 | mode) choice.
func (s *State) askOptions(k *prep, d *Decision, kind uint8) {
	if kind == akClone {
		s.cloneOptions(k, d)
		return
	}
	if kind == akPayLife {
		s.shockOptions(k, d) // repl_shockland.go
		return
	}
	if kind == akReveal || kind == akRevPick {
		s.revealOptions(k, d, kind) // repl_revealland.go
		return
	}
	if kind == akMode {
		siegeModeOptions(d) // face_siegemode.go
		return
	}
	if kind == akProtector {
		s.protectorOptions(d) // face_battle.go
		return
	}
	if kind == akEnchant {
		d.Sub = ChGenEnchant
		o := s.g.pre.obj
		who := s.objs[o].owner()
		if s.g.pre.state == preDigEnchant {
			who = s.res.askP
		}
		if s.g.pre.state == preDigEnchPlayer {
			who = s.res.askP
			var seats [MaxOptions]uint8
			n := s.playerEnchCands(k, s.objs[o].card, who, &seats)
			for i := 0; i < n; i++ {
				d.add(MakeAction(AkChoose, PlayerCode(seats[i]), 0, 0))
			}
			return
		}
		var buf [MaxOptions]TCode
		n := s.enchantCands(k, s.objs[o].card, who, &buf)
		for i := 0; i < n; i++ {
			d.add(MakeAction(AkChoose, buf[i], 0, 0))
		}
		return
	}
	if kind == akNumber {
		s.numberOptions(k, d) // repl_choosenumber.go
		return
	}
	if kind == akPlayer {
		s.playerOptions(d) // repl_chooseplayer.go
		return
	}
	if isNameAsk(kind) {
		s.nameOptions(k, d, kind) // repl_choosename.go
		return
	}
	if kind == akCard {
		s.enterCardOptions(k, d) // repl_choosecard.go
		return
	}
	if kind == akCtrPick || kind == akCtrPick2 {
		s.ctrPickOptions(k, d, kind) // repl_enterctrpick.go
		return
	}
	if kind == akUnleash {
		// kw_unleash.go: a cast creature's election rides s.g.pre (asked
		// with the spell on the stack); any other entry's is ask[0]
		if s.g.pre.state == 1 && s.g.pre.kind == akUnleash {
			s.unleashOptions(d, s.g.pre.obj)
			return
		}
		s.unleashOptions(d, s.perms[s.g.ask[0].sl].obj)
		return
	}
	if kind == akColor {
		d.Sub = ChGenColor
		ex := s.colorExclude(k) // act_mayask.go
		for c := uint8(0); c < 5; c++ {
			if ex&(1<<c) == 0 {
				d.add(MakeAction(AkChoose, 0, 0, c))
			}
		}
		return
	}
	d.Sub = ChGenType
	if k.gen.anyTypeValid && s.typeOptions(k, d) { // ValidTypes$ (act_choosetype.go)
		return
	}
	for v := range k.t.TypeVocab {
		d.add(MakeAction(AkChoose, 0, TCode(v>>8), uint8(v)))
	}
}

// dropHold removes the held entry once the free casts are made (a held
// spell then leaves the stack: spellHoldLeaves).
func (s *State) dropHold(k *prep) {
	spell := s.res.e.kind == seSpell
	if i := s.holdIndex(); i >= 0 {
		copy(s.stack[i:s.sp], s.stack[i+1:s.sp])
		s.sp--
	}
	if spell {
		s.spellHoldLeaves(k)
	}
	s.g.hold = false
	if s.g.priSave != 0 {
		s.prio, s.g.priSave = SeatIx(s.g.priSave), 0
	}
}
