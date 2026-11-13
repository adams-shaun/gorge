package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// the cold readers behind four Count$ heads --
// Agency Coroner (Sacrificed$Valid Card.IsSuspected), Magebane Lizard
// (ThisTurnCast Card.nonCreature+ControlledBy TriggeredActivator), April
// O'Neil (ThisTurnCast Card.YouCtrl$CardTypes) and Luxurious Locomotive
// (Count$CrewSize). Every reader is reached only from its table's
// expression (genPrep.anySuspect / castRingOn / the F2CrewedSrc filter), so
// a table without one of the heads never enters this file.

// DebugCountHeads10Off makes the four heads read 0: the mutation test
// flips it and requires a divergence. Test-only and cold. The sacrifice,
// cast-controller and cast-type reads consult it here; the crew read is
// zeroed at its F2CrewedSrc mark test (mcombatF2, filter_combatwords.go).
var DebugCountHeads10Off bool

// sacrificedLKICount answers RefSacrificed + the exact filter
// Card.IsSuspected, or a Card.<creature type> / Creature.<creature type>
// filter, from the sacrifice-time LKI snapshot (gorge sacrificedValidCount,
// effects/count_ref.go): the designation and the derived type line the
// sacrificed object had before the zone change laid the card over it
// (CR 608.2h, CR 701.60b). ok=false means the filter is not one of those
// reads, so the caller keeps its ordinary card match (kgen refuses every
// other Sacrificed$Valid spec).
func (s *State) sacrificedLKICount(k *prep, f kop.FilterID, li uint8, typeRead bool) (int, bool) {
	fl := &k.t.Filters[f]
	if !typeRead {
		if fl.Types != 0 || fl.NotTypes != 0 || fl.Ctrl != kop.CtrlAny || fl.Colors != 0 || fl.NotColors != 0 ||
			fl.Flags != 0 || fl.Zone != kop.ZoneNone || fl.Owner != 0 ||
			fl.Sub != 0 || fl.NotSub != 0 || fl.Flags2 != 0 || fl.PredV != (kop.ValueRef{}) || fl.Or != 0 {
			return 0, false
		}
		if DebugCountHeads10Off {
			return 0, true
		}
		if (s.lk(li).bits&lkiSuspected != 0) == (fl.PredN == 1) {
			return 1, true
		}
		return 0, true
	}
	if !sacrificedTypeFilter(fl) {
		return 0, false
	}
	if DebugCountHeads10Off || DebugGowronPostSac {
		return 0, true
	}
	l := s.lk(li)
	if fl.Types != 0 && l.types&fl.Types == 0 {
		return 0, true
	}
	if int(fl.Sub-1) >= len(k.t.Subtypes) || !l.subs.Has(int(fl.Sub-1)) {
		return 0, true
	}
	return 1, true
}

// sacrificedTypeFilter reports whether fl is one of the exact type reads
// kgen lowers for Sacrificed$Valid: a single subtype requirement, optionally
// also the Creature card type (Card.<creature type> / Creature.<creature
// type>), and nothing else.
func sacrificedTypeFilter(fl *kop.Filter) bool {
	return fl.Sub != 0 && (fl.Types == 0 || fl.Types == kop.TCreature) &&
		fl.NotTypes == 0 && fl.NotSub == 0 && fl.Ctrl == kop.CtrlAny && fl.Owner == 0 &&
		fl.Colors == 0 && fl.NotColors == 0 && fl.Pred == kop.PredNone && fl.PredN == 0 &&
		fl.Flags == 0 && fl.Flags2 == 0 && fl.Zone == kop.ZoneNone &&
		fl.PredV == (kop.ValueRef{}) && fl.Or == 0
}

// DebugGowronPostSac reads a sacrificed creature's type from the live object
// after the zone change instead of the sacrifice-time snapshot: the mutation
// test flips it and requires a divergence (a Klingon De-evolved into a
// Salamander is printed Klingon again in the graveyard).
var DebugGowronPostSac bool

// castRingCountHeadsMatch is thisTurnCastSpec's cold arm for the
// bits. It matches the pushed object where it now
// lives (castRingMatch), then:
//
//   - CastSpecCtrlTrig (gorge rules/host_cast_ledger.go L69-180: the
//     specCtx's TriggerContext, +ControlledBy TriggeredActivator) requires
//     the object's CURRENT controller to be the trigger's activating
//     player: the battlefield permanent's controller, the stack entry's
//     controller, or the owner off both (gorge resets Controller to Owner,
//     apply_zone.go);
//   - CastSpecTypes (gorge aggregateCastSourceProperty, count_agg.go) folds
//     the object's current card types into *tm instead of counting it.
func (s *State) castRingCountHeadsMatch(k *prep, x *kop.Expr, o Obj, ctx evalCtx, tm *kop.TypeMask) bool {
	if DebugCountHeads10Off {
		return false
	}
	if !s.castRingMatch(k, x.F, o, ctx) {
		return false
	}
	if x.N&kop.CastSpecCtrlTrig != 0 && !s.castRingCtrlIs(o, ctx) {
		return false
	}
	if x.N&kop.CastSpecTypes != 0 {
		*tm |= s.castRingTypes(k, o)
	}
	return true
}

// castRingCtrlIs reports whether o's current controller is the trigger's
// activating player (ctx.e.trigPl; a SpellCast trigger's caster).
func (s *State) castRingCtrlIs(o Obj, ctx evalCtx) bool {
	if ctx.e == nil {
		return false
	}
	trig := ctx.e.trigPl
	if trig > 1 {
		return false
	}
	ob := &s.objs[o]
	switch ob.zone {
	case kop.ZoneBattlefield:
		return ob.slot != NoSlot && s.perms[ob.slot].ctrl == trig
	case kop.ZoneStack:
		for i := int(s.sp) - 1; i >= 0; i-- {
			if st := &s.stack[i]; st.kind == seSpell && st.obj == o {
				return st.ctrl == trig
			}
		}
		return false
	}
	return ob.owner() == trig
}

// castRingTypes is the object's current card types as gorge's
// aggregateCastSourceProperty reads them: o.Face().Types (the copied face
// on the battlefield, the printed card everywhere else), masked to the
// CR 205.2a card-type vocabulary the kernel can hold. The pick uses the
// same zone walk as castRingMatch.
func (s *State) castRingTypes(k *prep, o Obj) kop.TypeMask {
	ob := &s.objs[o]
	switch ob.zone {
	case kop.ZoneBattlefield:
		if ob.slot != NoSlot {
			return k.cards[s.cardAt(ob.slot)].types
		}
		return 0
	case kop.ZoneStack:
		for i := int(s.sp) - 1; i >= 0; i-- {
			if st := &s.stack[i]; st.kind == seSpell && st.obj == o {
				return k.cards[st.card].types
			}
		}
		return 0
	}
	if s.ceasedCopy(ob) {
		return 0
	}
	return k.cards[ob.card].types
}

// castTypeCount is the CastSpecTypes read: the popcount of the matching
// objects' card-type union, masked to the CR 205.2a card types the kernel
// holds (gorge cardTypeWords' representable members; Kindred included).
//
//go:noinline
func castTypeCount(tm kop.TypeMask) int { return bits.OnesCount16(uint16(tm & cardTypeBits)) }

// six value heads that read an object or a number the
// kernel already tracks. Cold: every path here is off the transition loop.
//
// DebugCountHeads11Off neutralises the new reads for the mutation test
// (TestValueHeadsTrackedObjectMutation): ExPow answers its base, the ExExiledWithN
// mana-value sum answers the plain count, the sacrifice mana-value
// accumulator adds 0, and refCountersAll's trigger-object read answers 0.
var DebugCountHeads11Off bool

// exprPow is kop.ExPow: Sub[0] raised to Sub[1], saturating at +-32767 (the
// same clamp remLKAdd uses). gorge count_operand.go applyCountOp `Pow.`
// exponentiates logarithmically, clamping each multiply; a negative exponent
// fails closed there (applyCountOpOperandOK), which the kernel answers 0.
//
//go:noinline
func (s *State) exprPow(k *prep, x *kop.Expr, ctx evalCtx) int {
	base := s.evalV(k, x.Sub[0], ctx)
	if DebugCountHeads11Off {
		return base
	}
	exp := s.evalV(k, x.Sub[1], ctx)
	if exp < 0 {
		return 0
	}
	r := 1
	for exp > 0 {
		if exp&1 != 0 { // seat-ok: an exponent bit
			r = satMulCount(r, base)
		}
		exp >>= 1
		if exp > 0 {
			base = satMulCount(base, base)
		}
	}
	return r
}

// satMulCount multiplies two counts, clamping to +-32767.
func satMulCount(a, b int) int {
	v := a * b
	if v > 32767 {
		return 32767
	}
	if v < -32767 {
		return -32767
	}
	return v
}

// exwMVSum is kop.ExExiledWithN with N 1: the printed mana values of the
// cards in every seat's exile zone exiled with the reading source, summed
// (gorge count_ref_property.go's CardManaCost over exiledWithSet,
// engine/effects/count_ref.go; the same set exwCount counts, and
// state.SacrificedInfo-style printed reads). a card is never a spell on the
// stack here, so objMV's announced-X term is always 0.
//
//go:noinline
func (s *State) exwMVSum(k *prep, ctx evalCtx) int {
	if ctx.src == NoObj {
		return 0
	}
	trig, fire := false, uint8(0)
	if ctx.e != nil {
		trig, fire = ctx.e.kind == seTrigger, ctx.e.srcVer
	}
	n := 0
	for p := 0; p < int(s.nSeats()); p++ {
		if s.out(uint8(p)) {
			continue
		}
		pl := &s.pl[p]
		for j := 0; j < int(pl.exileN); j++ {
			o := pl.exile[j]
			if !s.exwHas(o, ctx.src, trig, fire) {
				continue
			}
			if DebugCountHeads11Off {
				n++
				continue
			}
			n += s.objMV(k, s.objs[o].card, o)
		}
	}
	return n
}

// exwSharedTypes is kop.ExExiledWithN with N 2: the most cards exiled with
// the reading source that have one card type in common, at least 1 (gorge
// count_ref_property.go's SharesCardTypes over exiledWithSet; CR 205.2a).
// Every card type is counted off the printed face, as gorge reads an exiled
// card.
//
//go:noinline
func (s *State) exwSharedTypes(k *prep, ctx evalCtx) int {
	if ctx.src == NoObj {
		return 0
	}
	trig, fire := false, uint8(0)
	if ctx.e != nil {
		trig, fire = ctx.e.kind == seTrigger, ctx.e.srcVer
	}
	const cardTypes = kop.TLand | kop.TCreature | kop.TArtifact | kop.TEnchantment | kop.TPlaneswalker |
		kop.TInstant | kop.TSorcery | kop.TBattle | kop.TKindred
	best := 1
	for b := kop.TypeMask(1); b != 0 && b <= cardTypes; b <<= 1 {
		if b&cardTypes == 0 {
			continue
		}
		n := 0
		for p := 0; p < int(s.nSeats()); p++ {
			if s.out(uint8(p)) {
				continue
			}
			pl := &s.pl[p]
			for j := 0; j < int(pl.exileN); j++ {
				if o := pl.exile[j]; s.exwHas(o, ctx.src, trig, fire) && k.t.Cards[s.objs[o].card].Types&b != 0 {
					n++
				}
			}
		}
		best = max(best, n)
	}
	return best
}

// tableRemembersIfPaidSac reports whether some program of t has a printed
// trigger's own IfPaid Sacrifice cost whose characteristic feeds
// resCtx.remLK (B.N the remember bit plus a SacSum* bit).
// genPrep.anyRemSac gates sacRemember, so a table with only this writer must
// set it too.
func tableRemembersIfPaidSac(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Op == kop.OpIfPaid && in.C&0xF == kop.PaySac &&
					in.B.N&(kop.SacSumPower|kop.SacSumToughness|kop.SacSumMV) != 0 {
					return true
				}
			}
		}
	}
	return false
}

// exwSourceLeft refreshes the incarnation stamp of the reverse exiled-with
// links the source made when the source leaves the battlefield
// gorge's Object.Incarnation advances on a battlefield
// crossing, so a trigger fired from the returned source still finds the
// cards its previous incarnation exiled (exiledBySourceAt's fireInc arm);
// the kernel's object ver advances on EVERY zone change, so a craft's
// self-exile plus transformed return is two ver steps where gorge counts one
// incarnation step. Counting the departure re-bases the stamp: the return's
// trigger then matches through the existing +1 fire-ver arm, while a
// non-trigger read after the return still sees the new incarnation.
//
// Only a multi-face source is re-based: the stamp shift answers the craft's
// self-exile plus transformed return. A single-face source (a Champion whose
// leaves trigger returns its exiled card, a flickered hideaway) keeps the
// stamp it was written with, which the trigger's fire ver already matches.
//
// Only a link live at the departure is re-based: the stamp is the ver the
// source held before this move. gorge re-bases the links of the incarnation that
// just left (apply_zone.go, the craft arm of the battlefield entry); the
// links of an earlier incarnation stay dead, so a crafted back face that was
// bounced, recast and crafted again counts the second material only.
func (s *State) exwSourceLeft(o Obj) {
	if len(s.k().t.Cards[s.objs[o].card].Faces) == 0 {
		return
	}
	ver := s.objs[o].ver
	for i := 0; i < int(s.g.nexw); i++ {
		r := &s.cold.exw[i]
		if r.src == o && r.fl&exwRev != 0 && r.sver == ver-1 {
			r.sver = ver
		}
	}
}

// exwCostExilePick records the ExileCtrlOrGrave<1/Creature> material a
// craft cost exiles at ACTIVATION (gorge stamps the
// association during cost payment, before any resolution). exwExile runs only
// inside a resolution, so the craft's material would never be linked and the
// back face's ExiledWith$CardManaCost would read 0. Only the craft-shaped
// cost is recorded (an ordinary ExileFromGrave cost emits a plain move with
// no ExiledWith marker), and only for a card a face of which reads the link.
func (s *State) exwCostExilePick(o Obj) {
	k := s.k()
	if k == nil || k.gen == nil || !k.gen.anyExW {
		return
	}
	pc := s.buildPickCost(k)
	if pc == nil || pc.Flags&kop.CostExileCtrlOrGrave == 0 {
		return
	}
	e := &s.bld().e
	if e.obj == NoObj {
		return
	}
	rec := k.gen.exwRec[e.card]
	if rec&exwRecRead == 0 {
		return
	}
	// the craft material always carries the reverse link: the reading
	// face's ExiledWith$ head is the provenance, whether or not the card text
	// also names ExiledWithSource (kop.ExwWord)
	fl := exwRev
	if s.objs[o].of&ofToken != 0 {
		fl |= exwTok
	}
	s.exwPut(o, e.obj, s.objs[e.obj].ver, fl, int(e.card))
}

// DebugPrimeOdd is TestCountHeadsDeliriumPrimeMutation's switch: "prime" becomes "odd".
var DebugPrimeOdd bool

// ownMainPhase: 1 during the controller's own Main1/Main2 (gorge
// count_body_heads.go:441, Count$InOwnMainPhase), else 0.
func (s *State) ownMainPhase(ctrl uint8) int {
	if s.active == ctrl && (s.step == kop.StepMain1 || s.step == kop.StepMain2) {
		return 1
	}
	return 0
}

// countLibrary is ExCountHand N 1 (gorge Count$ValidLibrary): every
// library's cards matching x.F, owner pp. Same walk as countHand over
// pl.lib[pl.libLo:pl.libHi].
//
//go:noinline
func (s *State) countLibrary(k *prep, x *kop.Expr, ctx evalCtx) int {
	v := 0
	for pp := uint8(0); pp < s.nSeats(); pp++ {
		if s.out(pp) {
			continue
		}
		pl := &s.pl[pp]
		for j := int(pl.libLo); j < int(pl.libHi); j++ {
			o := pl.lib[j]
			if s.matchCardGenAt(k, x.F, s.objs[o].card, pp, ctx.ctrl, o, &ctx) {
				v++
			}
		}
	}
	return v
}

// unlockedDoors is ExUnlockedDoors (gorge unlockdoor.go:228
// countUnlockedDoors, distinct=false): one per unlocked door of each Room
// permanent ctrl controls.
//
//go:noinline
func (s *State) unlockedDoors(k *prep, ctrl uint8) int {
	n := 0
	for m := s.ctrlM[SeatIx(ctrl)].and(s.occ); m.any(); m = m.rest() {
		c := s.perms[m.first()].card
		if roomDef(k, c) {
			n += bits.OnesCount8(roomMask(k, c))
		}
	}
	return n
}

// isPrimeN is gorge isPrime (count_body_simple.go:164): n < 2 false, trial
// division. DebugPrimeOdd: n%2 == 1.
func isPrimeN(n int) bool {
	if DebugPrimeOdd {
		return n%2 == 1
	}
	if n < 2 {
		return false
	}
	for i := 2; i <= n/i; i++ {
		if n%i == 0 {
			return false
		}
	}
	return true
}

// Count$ValidGraveyard <spec>$GreatestCardPower and the
// /Mod.N value modifier. Cold code, out of line.

// StallGravePower was a graveyard card's characteristic-defining power the
// off-battlefield reader could not evaluate. It is no longer raised:
// FeatCDAPT evaluates every lowered CDA amount as the layer walk does.
const StallGravePower = 127

func init() { stallNames[StallGravePower] = "gravepower" }

// DebugModOff is TestCountHeadsLeftAndModMutation's switch: ExMod returns Sub[0] unchanged.
var DebugModOff bool

// exprMod is ExMod (gorge count_operand.go:441-447): Sub[0] % N. Go's %
// takes the dividend's sign like gorge's int32 remainder; kgen emits N >= 1.
//
//go:noinline
func (s *State) exprMod(k *prep, x *kop.Expr, ctx evalCtx) int {
	v := s.evalV(k, x.Sub[0], ctx)
	if DebugModOff || x.N == 0 {
		return v
	}
	return v % int(x.N)
}

// graveGreatestPower is ExGraveAgg's AggGreatestPower arm: gorge
// extremePropertyValue (count_zone_fold.go) reads the card's derived power
// through h.Power -- off the battlefield the printed power, or the
// characteristic-defining amount of a `*` card (gorge's CDASetPT applies in
// every zone; FeatCDAPT). 0 when nothing matches.
//
//go:noinline
func (s *State) graveGreatestPower(k *prep, x *kop.Expr, ctx evalCtx) int {
	best := 0
	for pp := uint8(0); pp < s.nSeats(); pp++ {
		if s.out(pp) {
			continue
		}
		pl := &s.pl[pp]
		for j := 0; j < int(pl.graveN); j++ {
			o := pl.grave[j]
			if s.objs[o].of&ofToken != 0 {
				continue // a token waiting in a graveyard is no card (CR 111.7, 704.5d)
			}
			c := s.objs[o].card
			if !s.matchCardGenAt(k, x.F, c, pp, ctx.ctrl, o, &ctx) {
				continue
			}
			v := int(k.cards[c].power)
			if k.cda && k.gen != nil && k.gen.has(c, gfCDAPT) {
				v, _, _ = s.FeatCDAPT(o)
			}
			if v > best {
				best = v
			}
		}
	}
	return best
}
