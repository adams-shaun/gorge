package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// tokenAtEOT registers a created token's AtEOT$ rider (gorge token.go ->
// scheduleAtEOT: one next-end-step promise per minted token, in mint
// order), as the copy-token rider does (execCopyToken's dly entry): the
// token is sacrificed or exiled at the beginning of the next end step.
//
//go:noinline
func (s *State) tokenAtEOT(in *kop.Instr, t Obj) {
	if in.B.N&(kop.TokSacAtEnd|kop.TokExileAtEnd) == kop.TokSacAtEnd|kop.TokExileAtEnd {
		s.tokenAtEOTExt(in, t) // an extended rider (trig_tokenateot.go)
		return
	}
	if in.B.N&kop.TokExileAtEnd == 0 && s.g.tokMade > 1 && s.sacAtEndJoin(t, dlyUntracked) {
		return // a later token of one resolution joins the first one's trigger
	}
	s.atEOTRider(t, in.B.N&kop.TokExileAtEnd != 0)
}

// dlyGrpFollow is delayed.when for a token whose "sacrifice them at the
// beginning of the next end step" is carried by the trigger of the first token
// its resolution made (gorge scheduleAtEOTTokens: one delayed triggered
// ability naming every token, CR 603.7). The record is never put on the stack
// itself; creator names the head token, whose trigger sacrifices it as well.
const dlyGrpFollow = 6

// sacAtEndJoin registers token t, made after another token by the same
// resolution, as a follower of the previous token's sacrifice, and reports
// whether it did. when is the head record's trigger step: dlyUntracked for
// the next end step, dlyEndCombat for this turn's end of combat.
//
//go:noinline
func (s *State) sacAtEndJoin(t Obj, when uint8) bool {
	if s.ndly == 0 || s.ndly >= MaxDelayed {
		return false
	}
	prev := &s.dly[s.ndly-1]
	head := prev.obj
	switch {
	case prev.when == dlyGrpFollow:
		head = prev.creator
	case prev.when != when || prev.creator != s.res.e.obj || prev.obj == t:
		return false
	}
	s.dly[s.ndly] = delayed{act: uint8(kop.OpSacrifice), ctrl: s.res.e.ctrl, obj: t, ver: objWord(s.objs[t].ver), when: dlyGrpFollow, creator: head}
	s.ndly++
	return true
}

// sacAtEndFollowers sacrifices the followers of head's end-step trigger as it
// resolves, consuming their records, together with head itself when hit
// already holds its slot (gorge's one Sacrifice over the group: the sacrifices
// are simultaneous, so each token's "whenever you sacrifice another creature"
// looks back at the others, CR 603.10a). A group of one is the plain
// sacrifice.
//
//go:noinline
func (s *State) sacAtEndFollowers(k *prep, head Obj, hit slotMask) {
	var fol [MaxDelayed]Obj
	nf := 0
	w := uint8(0)
	for i := uint8(0); i < s.ndly; i++ {
		d := s.dly[i]
		if d.when == dlyGrpFollow && d.creator == head {
			fol[nf] = d.obj
			nf++
			continue
		}
		s.dly[w] = d
		w++
	}
	for i := w; i < s.ndly; i++ {
		s.dly[i] = delayed{}
	}
	s.ndly = w
	for i := 0; i < nf; i++ {
		ob := &s.objs[fol[i]]
		if ob.zone != kop.ZoneBattlefield || s.sacBlocked(k, ob.slot) {
			continue
		}
		hit = hit.with(ob.slot)
	}
	switch hit.count() {
	case 0:
	case 1:
		sl := hit.first()
		s.fireSac(k, sl)
		s.leaveParked(k, sl)
	default:
		s.fireSacAll(k, hit)
		s.leaveAll(k, hit, kop.ZoneGraveyard)
	}
}

// atEOTRider registers one next-end-step delayed action on object t (gorge
// scheduleAtEOTRider): sacrifice it, or exile it. The sacrifice is gorge's
// untracked __kwEncoreSacrifice body, still put on the stack when the token
// already left (dlyUntracked); the exile is the incarnation-
// tracked __kwWarpExile, dropped then.
func (s *State) atEOTRider(t Obj, exile bool) {
	act := kop.OpSacrifice
	if exile {
		act = kop.OpExile
	} else {
		s.sacAtEndUntracked(t, s.res.e.ctrl)
		return
	}
	if s.ndly < MaxDelayed {
		s.dly[s.ndly] = delayed{act: uint8(act), ctrl: s.res.e.ctrl, obj: t, ver: objWord(s.objs[t].ver), creator: s.res.e.obj}
		s.ndly++
	} else {
		s.capFull(StallDelayed)
	}
}

// a created token's AtEOT$ Destroy and AtEOT$ SacrificeCombat
// riders (gorge engine/effects/ateot.go atEOTRiders). AtEOT$ YourSacrifice is
// gorge's plain Sacrifice (kgen maps it to TokSacAtEnd).
//
// Both ride OpCreateTokenEx with TokSacAtEnd|TokExileAtEnd set together, the
// kind in B.N's high byte (kop.TokAtEOTDestroy, kop.TokAtEOTSacCombat). Each
// token registers one s.dly record:
//   - Destroy: {OpDestroy, when 0}, tracked like the exile rider (__kwAtEOTDestroy,
//     incarnation-tracked: a stale promise is dropped). It resolves through
//     destroy, so indestructible, regeneration and shields apply.
//   - SacrificeCombat: {OpSacrifice, when dlyEndCombat}, untracked like the end
//     step sacrifice (__kwEncoreSacrifice: the trigger still goes on the stack
//     over a token that left, and the sacrifice then skips it), fired at this
//     turn's end of combat (fireDlyEndCombat). gorge's thisTurn expiry (|TT=)
//     is mirrored by fireDelayed dropping a record still pooled at the end step.

// dlyEndCombat is delayed.when for a record fired at this turn's end of
// combat step.
const dlyEndCombat = 4

// DebugTokAtEOTDestroyIsLeave is a test mutation: a delayed OpDestroy moves
// the object to the graveyard (the arm before these riders) instead of
// destroying it.
var DebugTokAtEOTDestroyIsLeave bool

// tokenAtEOTExt registers the extended rider of token t (both AtEOT bits
// set in in.B.N, the kind in the high byte). A later SacrificeCombat token of
// one resolution joins the first token's sacrifice (sacAtEndJoin, the grouping
// tokenAtEOT documents), so the resolution's tokens fall to one trigger.
//
//go:noinline
func (s *State) tokenAtEOTExt(in *kop.Instr, t Obj) {
	if in.B.N>>8 == kop.TokAtEOTSacCombat && s.g.tokMade > 1 && s.sacAtEndJoin(t, dlyEndCombat) {
		return // a later token of one resolution joins the first one's sacrifice
	}
	d := delayed{act: uint8(kop.OpDestroy), ctrl: s.res.e.ctrl, obj: t, ver: objWord(s.objs[t].ver), creator: s.res.e.obj}
	if in.B.N>>8 == kop.TokAtEOTSacCombat {
		d.act, d.when = uint8(kop.OpSacrifice), dlyEndCombat
	}
	if s.ndly >= MaxDelayed {
		s.capFull(StallDelayed)
		return
	}
	s.dly[s.ndly] = d
	s.ndly++
}

// destroyDelayed is resolveDelayed's OpDestroy: destroy the object (CR
// 701.8), which indestructible, regeneration and a shield counter stop.
//
//go:noinline
func (s *State) destroyDelayed(k *prep, sl Slot) {
	if DebugTokAtEOTDestroyIsLeave {
		s.leave(k, sl, kop.ZoneGraveyard, false)
		return
	}
	s.destroy(k, sl)
}

// fireDlyEndCombat enqueues the dlyEndCombat records as the end of combat
// step begins, after the s.g.dp end-of-combat programs (gorge fires its
// delayed registrations in one registration-ordered list), untracked as
// fireDelayed enqueues dlyUntracked ones, and keeps the other records in order.
//
//go:noinline
func (s *State) fireDlyEndCombat(k *prep) {
	n := s.ndly
	var keep uint8
	for i := uint8(0); i < n; i++ {
		d := s.dly[i]
		if d.when != dlyEndCombat {
			s.dly[keep] = d
			keep++
			continue
		}
		amt := int16(d.act)
		if kop.Op(d.act) == kop.OpSacrifice {
			amt = opSacUntracked // __kwEncoreSacrifice: sacrifice the group by id
		}
		s.enqueue(dlySource(&d), uint8(d.ver), s.objs[d.obj].card, abDelayed, d.ctrl, d.obj, d.ctrl, amt)
	}
	for i := keep; i < n; i++ {
		s.dly[i] = delayed{}
	}
	s.ndly = keep
}

// prepTokAtEOT sets anyTokEndCombat when a table's CreateTokenEx carries the
// SacrificeCombat rider.
func prepTokAtEOT(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Op == kop.OpCreateTokenEx && in.B.N&(kop.TokSacAtEnd|kop.TokExileAtEnd) == kop.TokSacAtEnd|kop.TokExileAtEnd && in.B.N>>8 == kop.TokAtEOTSacCombat {
					g.anyTokEndCombat = true
				}
			}
		}
	}
}

// CopyPermanent and Token AtEOT$ Sacrifice (Reflection of
// Kiki-Jiki; Thatcher Revolt, Hungry for More).
//
// gorge's scheduleAtEOT registers the copy's sacrifice as the builtin
// __kwEncoreSacrifice body, which apply_delayed.go does NOT track by
// incarnation: at the next end step the delayed trigger is put on the stack
// even when the token already left the battlefield, and does nothing on
// resolution (CR 603.7c). The kernel's generated delayed records are
// dropped at fireDelayed when their object changed incarnation (the
// AtEOTTrig$ shape, a trigger the token itself carries), so this record is
// marked dlyUntracked: it is always enqueued, and resolveDelayed's
// incarnation check makes it a no-op.

// dlyUntracked is delayed.when for a next-end-step record put on the stack
// even when its object left (fireDelayed).
const dlyUntracked = 2

// sacAtEndUntracked registers token t's untracked next-end-step sacrifice
// under ctrl.
//
//go:noinline
func (s *State) sacAtEndUntracked(t Obj, ctrl uint8) {
	if s.ndly < MaxDelayed {
		s.dly[s.ndly] = delayed{act: uint8(kop.OpSacrifice), ctrl: ctrl, obj: t, ver: objWord(s.objs[t].ver), when: dlyUntracked, creator: s.res.e.obj}
		s.ndly++
	} else {
		s.capFull(StallDelayed)
	}
}
