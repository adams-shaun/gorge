package dirac

import (
	"fmt"
	"os"
	"runtime"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Gift (CR 702.174, Bloomburrow), copied from gorge (rules cast_asks.go
// giftAsk, cast_targets.go foldGiftPromise, stack.go's gift splice,
// altcast.go altCostEnter, legal_targets.go targetOfferMin):
//
//   - bsGift: a spell whose card has Gift (CardDef.Gift) asks, after the
//     push and the either-or additional cost and before every other cost
//     and target choice, whether its controller promises the opponent a
//     gift (DkChoose/ChGenGift: mode 0 don't, 1 promise). The answer is the
//     sfPromised bit of the entry's flags (shared with sfHold), on the stack at once.
//   - the promise is read by CondPromised (Card.Self+PromisedGift),
//     ExPromisedGift (Count$PromisedGift.a.b) and TgfGiftRange slots; on a
//     permanent spell it is read as the permanent enters (the resolving
//     entry is its spell): its "if the gift was promised" enter triggers.
//   - an instant's or sorcery's gift is the head of its program (kgen
//     giftHead: OpGiveGift, then the GiftAbility body); a permanent's is
//     its AfGift enter trigger, whose gift is given as it is put on the
//     stack (giftPushed).
//   - OpGiveGift and giftPushed fire TrigGiveGift (Jolly Gerbils). The
//     event has no slot subscription mask: a gift scans the battlefield.

// sfPromised: the gift was promised (CR 702.174a). stackEntry has no spare
// byte or flag bit, so it shares sfHold's bit: sfHold marks only a
// non-spell entry and the promise only a spell, so every reader goes
// through promised, which checks the kind.
const sfPromised = sfHold

// promised: e is a spell whose gift was promised.
func (e *stackEntry) promised() bool {
	return e.kind == seSpell && e.flags&sfPromised != 0
}

// nSubEvents is the number of trigger events with a slot subscription
// mask (State.subs). TrigGiveGift and the events after it
// (TrigProliferate, TrigTokenCreated: act_proliferate.go; TrigBecomeMonstrous:
// act_monstrosity.go) have none and must stay the last events: the
// array below fails to compile otherwise.
const nSubEvents = kop.TrigGiveGift

var _ = [1]struct{}{}[kop.TrigEventCount-1-kop.TrigFacesDilemma]

// giftOptions poses the gift election (gorge giftAsk: decline first, then
// one option per opponent).
func (s *State) giftOptions(d *Decision) {
	d.Kind, d.Sub, d.Player = DkChoose, ChGenGift, s.bld().e.ctrl
	d.add(MakeAction(AkChoose, 0, 0, 0))
	d.add(MakeAction(AkChoose, 0, 0, 1))
}

// answerGift records the election on the entry and moves on.
func (s *State) answerGift(k *prep, a Action) {
	if a.Mode() == 1 {
		s.bld().e.flags |= sfPromised
	}
	s.nextStage(k)
}

// promisedCtx: the gift of ctx's entry was promised; with no entry (an
// enter trigger's check), the source is the permanent its resolving spell
// is putting onto the battlefield and that spell's gift was promised.
func (s *State) promisedCtx(ctx evalCtx) bool {
	if ctx.e != nil {
		return ctx.e.promised()
	}
	r := &s.res
	return r.active && r.e.obj == ctx.src && r.e.promised()
}

// giveGift: player p gives a gift (CR 702.174b): "whenever you give a gift"
// triggers (gorge's events.GiveGift marker).
func (s *State) giveGift(k *prep, p uint8) {
	if k.gen == nil {
		return
	}
	var ord [MaxPerm]Slot
	n := s.listenerOrder(s.occ, &ord)
	for _, sl := range ord[:n] {
		s.slotTriggers(k, sl, func(a *kop.Ability) bool { return a.Trig.Event == kop.TrigGiveGift }, func(l trigListener, a *kop.Ability) {
			s.playerTrigGen(k, kop.TrigGiveGift, l, a, p, 0)
		})
	}
}

// giftPushed: a permanent's gift trigger is put on the stack; its gift is
// given then (gorge trigger_queue.go: GiveGift is recorded when the
// trigger is pushed).
func (s *State) giftPushed(k *prep, pt *pendingTrig) {
	if pt.ab < abCascade && k.t.Cards[pt.card].Abilities[pt.ab].AFlags&kop.AfGift != 0 {
		s.giveGift(k, pt.ctrl)
	}
}

// giftRange is a TgfGiftRange slot's object count under the entry's
// election.
func giftRange(t *kop.TargetSpec, promised bool) (lo, hi uint8) {
	if promised {
		return t.MinKicked, t.MaxKicked
	}
	return t.Min, t.Max
}

// giftTargetsFeasible is abilityTargetsFeasible for an ability with a
// TgfGiftRange slot, before the election (gorge targetsAvailable: the
// election is one choice for the whole spell, so the cast is offered when
// every mandatory slot can be announced under either election).
func (s *State) giftTargetsFeasible(k *prep, a *kop.Ability, p uint8, src Slot) bool {
	for pr := 0; pr < 2; pr++ {
		ok := true
		for i := 0; i < int(a.NTargets) && ok; i++ {
			t := &a.Targets[i]
			if t.UpTo || t.TFlags&kop.TgfAtResolution != 0 {
				continue
			}
			if t.TFlags&kop.TgfGiftRange == 0 {
				ok = s.hasTarget(k, t, p, src, NoObj)
				continue
			}
			lo, _ := giftRange(t, pr == 1)
			if lo == 0 {
				continue
			}
			var buf [MaxOptions]uint8
			ok = s.targetCands(k, t, p, src, NoObj, &buf) >= int(lo)
		}
		if ok {
			return true
		}
	}
	return false
}

// giftSlotStart sets a TgfGiftRange slot's count from the entry's election
// and reports false when the elected count can't be met: gorge reverses
// the cast (CR 601.2c, 733.1: "cast aborted: no legal target").
func (s *State) giftSlotStart(k *prep, t *kop.TargetSpec, ti uint8) bool {
	e := &s.bld().e
	mt := &s.g.mt
	mt.min, mt.max = giftRange(t, e.promised())
	if mt.min == 0 || e.kind != seSpell {
		return true
	}
	var buf [MaxOptions]uint8
	if s.multiCands(k, t, ti, &buf) >= int(mt.min) {
		return true
	}
	mt.active = false
	s.reverseCast(k)
	return false
}

// giftNoSlot: a gift-gated mid-resolution slot takes no object under the
// entry's election (Into the Flood Maw's second slot when no gift was
// promised): nothing is asked.
func (s *State) giftNoSlot(t *kop.TargetSpec, e *stackEntry) bool {
	_, hi := giftRange(t, e.promised())
	return hi == 0
}

// reverseCast undoes the cast or activation being built and returns to
// priority (CR 733.1: gorge abortCast): a pushed spell moves back from the
// stack to the end of the zone it was cast from; an identical undo with no
// progress between is held out of the window after the second (gorge
// F05-2 / CR 733.2).
func (s *State) reverseCast(k *prep) {
	e := &s.bld().e
	tokenGone := s.bld().bfl&bflTokenGone != 0
	if os.Getenv("DIRAC_RC_TRACE") != "" {
		_, f, l, _ := runtime.Caller(1)
		pt := 0
		for i := range s.perms {
			if s.perms[i].flags&pfTapped != 0 {
				pt++
			}
		}
		fmt.Fprintf(os.Stderr, "RC %s:%d kind=%d pool=%v taps=%d\n", f, l, e.kind, s.pl[e.ctrl].pool, pt)
	}
	s.ctl = ctlPriority
	s.undos++
	if s.bld().bfl&bflUsedMark != 0 && s.g.nused > 0 {
		// a cast that never happened leaves its once-per-turn permission
		// unspent (gorge counts only the plays that were cast)
		s.g.nused--
	}
	if s.bld().bfl&bflDilemma != 0 {
		s.unfaceDilemma() // trig_dilemma.go
	}
	s.g.evSel = 0 // a spell's or activation's evidence picks (CR 733.1: nothing was exiled)
	if s.bld().bfl&bflCrime != 0 && DebugCrimeStats != nil {
		if s.npend != 0 {
			DebugCrimeStats.ReversedQueued++
		}
		DebugCrimeStats.Reversed++ // the ledger keeps the bit, no trigger was queued (act_commitcrime.go)
	}
	// gorge's cast and activate priority actions reset the pass count
	// before the cast flow begins (rules legal_priority.go: the Priority
	// event with Amount 0), and abortCast does not restore it: after a
	// reversal the holder's pass no longer completes the round
	s.passes = 0
	noProg := s.noProgress()
	if s.bld().bfl&bflPushed != 0 {
		noProg = s.bld().bfl&bflNoProg != 0 // the push's own touches are no progress
	}
	s.abortCount(e.obj, noProg)
	if s.heldObj == e.obj && noProg {
		s.heldN++
	} else {
		s.heldObj, s.heldN = e.obj, 1
	}
	if e.kind == seSpell && s.bld().bfl&bflPushed != 0 {
		s.unpushSpell(k)
	} else if e.kind == seAbility && s.bld().bfl&bflPushed != 0 && (k.gen != nil || s.bld().stage == bsEvidence) {
		s.unpushActivation() // cost_collectevidence.go
	} else if e.kind == seSpell {
		// the end of its graveyard too (a flashback cast, pair 2359)
		pl := &s.pl[s.objs[e.obj].owner()]
		switch s.objs[e.obj].zone {
		case kop.ZoneHand:
			s.touch(e.obj)
			removeFrom(pl.hand[:], &pl.handN, e.obj)
			pl.hand[pl.handN] = e.obj
			pl.handN++
		case kop.ZoneGraveyard:
			s.touch(e.obj)
			removeFrom(pl.grave[:], &pl.graveN, e.obj)
			pl.grave[pl.graveN] = e.obj
			pl.graveN++
			if k.gen != nil && e.flags&sfFlashback != 0 && (k.cards[e.card].kw&kop.KwFlashback == 0 && !k.t.Cards[e.card].Harmonize || e.flags&sfFbGranted != 0) {
				s.setFBGrants(k, e.obj, s.bld().fbGrants) // gorge keeps the granted flashback, once per grant
			}
		}
	}
	if e.kind == seSpell && k.gen != nil && k.gen.anyFaces {
		s.unturnFace(k, e.obj, e.mode) // a face cast's card is its front again (face_castface.go)
	}
	if e.kind == seSpell && s.g.on && s.objs[e.obj].zone == kop.ZoneExile && s.mObj(k, mechStash, e.obj) {
		// a stashed card cast from exile went to the stack and
		// back: a new object, without its stash counter (CR 400.7)
		s.touch(e.obj)
		s.mObjClr(k, mechStash, e.obj)
	}
	if e.kind == seSpell && k.gen != nil && k.gen.anyWarp && s.objs[e.obj].zone == kop.ZoneExile {
		// a reversed cast never happened: a warp permission stays
		// (kw_warp.go)
		s.refreshWarpRecast(e.obj)
		s.refreshAirbend(e.obj) // act_airbend.go
	}
	s.heldTurn, s.heldStep, s.heldTouch = uint16(s.turn), s.step, s.touchN
	if tokenGone {
		s.heldTouch-- // gorge's token ceases after the reversal: progress (bflTokenGone)
	}
}
