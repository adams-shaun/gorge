package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The two-alternative "and/or" Kicker (CR 702.33b; gorge's per-alternative
// kicked casts, rules cast_altcost.go twoPartKickerCosts and
// legal_walk_hand.go's kicked1/kicked2/kickedboth options; CardDef.Kicker2):
//
//   - the selection rides an AkCast's target byte, guarded by mode bit 0 and
//     the card's KwKicker + Kicker2: castKickAlt1Tgt / castKickAlt2Tgt /
//     castKickBothTgt (4, 6 and 7 are free on a mode-1 hand cast; the mode
//     byte is full and castModes is full, as the canon seat layer's
//     ActCastKickedAlt kind is). kgen sets Kicker2 only with Kicker, and a
//     Kicker2 carrier prints no flashback/escape/retrace/miracle/warp route
//     the byte would name, so it stays unambiguous; the guard checks that
//     like escapeCast does (kw_escape.go).
//   - beginCast sets sfKicked (mode bit 0) and stamps the selection bits on
//     the spell object's byte (setKickAlt, filter_kickedalt.go -- the
//     multikicker count's per-object region, one kicked family per card),
//     which the build's pricing (addKickerPayments reads it; CR 601.2f: the
//     reduction comes off the total WITH the additional cost in it), the
//     Card.Self+kicked N filters (PredKickedAlt) and Count$TimesKicked
//     (ExKicked, the popcount) read. Paying part one sets bit0, part two
//     bit1, both both; a plain cast sets no bit.
//   - a copy keeps the spell's bits (CR 707.10, as the multikicker count's).

// The selection markers, untyped so they ride both the beginCast target byte
// (kickAltSelOf, cast.go) and the action's TCode target field (kickAltTgtOf,
// kickAltActionSel) without a narrowing conversion (the obj-code cast
// ratchet). 4 and 6/7 keep the even/odd spacing of the mode-1 byte's free
// values (castImpTgt is 2 on mode 0, castEscapeTgt 3 and castPrintedEscapeTgt
// 5 on mode 2, retraceTgt 1 under a mode guard).
const (
	castKickAlt1Tgt = 4
	castKickAlt2Tgt = 6
	castKickBothTgt = 7
)

// kickAltSelOf is the selection bits a kick-alternative cast's target byte
// names (1 part one, 2 part two, 3 both), or 0.
func kickAltSelOf(tgt uint8) uint8 {
	switch tgt {
	case castKickAlt1Tgt:
		return 1
	case castKickAlt2Tgt:
		return 2
	case castKickBothTgt:
		return 3
	}
	return 0
}

// kickAltTgtOf is the selection bits' target code (the offers') or 0.
func kickAltTgtOf(sel uint8) TCode {
	switch sel {
	case 1:
		return castKickAlt1Tgt
	case 2:
		return castKickAlt2Tgt
	case 3:
		return castKickBothTgt
	}
	return 0
}

// kickedAltCast reports whether an AkCast of card c in mode, target tgt is a
// per-alternative kicked cast (kw_kickalt.go): a mode-1 cast whose target
// byte carries one of the three markers on a card the two-alternative Kicker
// lowered. The mode-1 target byte has no other reader (4, 6 and 7 are free
// there), so the card's keywords narrow it the way escapeCast's do.
func kickedAltCast(k *prep, c CardID, mode, tgt uint8) bool {
	if mode&1 == 0 { // seat-ok: a cast-mode bit
		return false
	}
	cd := &k.t.Cards[c]
	return kickAltSelOf(tgt) != 0 && hasKicker2(cd) && cd.Keywords&kop.KwKicker != 0
}

// hasKicker2 reports whether cd carries the two-alternative Kicker's second
// half: a payable mana half, set only with Kicker (kgen lowers both or
// neither).
func hasKicker2(cd *kop.CardDef) bool {
	return cd.Kicker2 != kop.ManaCost{}
}

// FeatKicker2 is hasKicker2 for the oracle's table read (canon/translate.go).
func FeatKicker2(cd *kop.CardDef) bool { return hasKicker2(cd) }

// FeatKickAltSel is the selection bits a kick-alternative cast's target byte
// names, the oracle's read of an AkCast (kw_kickalt.go).
func FeatKickAltSel(tgt uint8) uint8 { return kickAltSelOf(tgt) }

// FeatKickAltTgt is the selection bits' target code, the offers'.
func FeatKickAltTgt(sel uint8) TCode { return kickAltTgtOf(sel) }

// FeatKicker2Card reports whether o's card carries the two-alternative
// Kicker's second half (the oracle's position read; cost_optional.go's
// FeatOptCostCard shape).
func (s *State) FeatKicker2Card(o Obj) bool {
	k := s.k()
	if k == nil {
		return false
	}
	return hasKicker2(&k.t.Cards[s.objs[o].card])
}

// kickAltActionSel is the selection an AkCast action pays: the markers of a
// per-alternative kicked cast (kw_kickalt.go), 0 for every other action.
func (s *State) kickAltActionSel(a Action) uint8 {
	if a.Kind() != AkCast || a.Mode()&1 == 0 { // seat-ok: a cast-mode bit
		return 0
	}
	o := Obj(a.Source())
	k := s.k()
	if k == nil || int(o) >= len(s.objs) {
		return 0
	}
	cd := &k.t.Cards[s.objs[o].card]
	if !hasKicker2(cd) || cd.Keywords&kop.KwKicker == 0 {
		return 0
	}
	switch a.Target() { // the markers ride the TCode field itself (kw_kickalt.go)
	case castKickAlt1Tgt:
		return 1
	case castKickAlt2Tgt:
		return 2
	case castKickBothTgt:
		return 3
	}
	return 0
}

// kickAltBits is the kicker alternatives object o's cast paid
// (filter_kickedalt.go's byte in the multikicker count's region), or 0 when
// the region is not allocated.
func (s *State) kickAltBits(o Obj) uint8 {
	ka := coldMkObj.of(s)
	if int(o) < len(ka) {
		return ka[o]
	}
	return 0
}

// kickAltCount is Count$TimesKicked on a two-alternative Kicker card: the
// paid alternatives, the popcount of the kickAlt byte (0/1/2). ok is false
// for every other card or context, so the multikicker count read runs.
func (s *State) kickAltCount(ctx evalCtx) (int, bool) {
	ka := coldMkObj.of(s)
	if len(ka) == 0 {
		return 0, false
	}
	o := ctx.src
	if e := ctx.e; e != nil && e.kind == seSpell {
		o = e.obj // the spell on the stack reads its own cast's byte
	}
	if o == NoObj || int(o) >= len(ka) {
		return 0, false
	}
	ob := &s.objs[o]
	if ob.zone != kop.ZoneBattlefield && ob.zone != kop.ZoneStack {
		return 0, false // a permanent that left the battlefield reads 0 (gorge's Object.TimesKicked resets)
	}
	k := s.k()
	if k == nil || !hasKicker2(&k.t.Cards[ob.card]) {
		return 0, false
	}
	return bits.OnesCount8(ka[o]), true
}
