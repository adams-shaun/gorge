package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Retrace (CR 702.81a), cold: reached only when the table has a card with
// printed Retrace (prep.retrace).
//
// A Retrace card in its owner's graveyard may be cast by paying its printed
// mana cost PLUS the additional cost of discarding a land card (gorge rules
// cast_begin.go's castModeRetrace arm through retraceExtra; legal_walk_alt.go's
// graveyard walk, Option Kind "cast" Mode "retrace"). Unlike the
// alternative-cost keyword family Retrace is no cost substitution: the
// printed base (castOfferBase's composition) stays and only the additional
// discard folds on, so the cast's mana price is the card's printed Cost and
// the discard is its pick cost (CardDef.RetraceCost, CostDiscardFiltered).
//
// The cast is an ordinary one from the graveyard: the spell rests in its
// owner's graveyard on resolution (no sfFlashback exile), and gorge's
// wasCastFromGraveyard reads true through the object's latest cast zone
// (cast_fromgrave.go), not through a flag -- gorge stamps no cast flag for
// it (nomanacost.go lists "retrace" among the printed-mana-cost modes).
// The kernel action is AkCast with retraceTgt in the target byte and mode
// bit 2; beginCast marks the entry trigPlRetrace, which buildPickCost reads
// for the RetraceCost. A copy keeps no mark (mintSpellCopy, CR 707.10).
//
// A card with a stamped or live FLASHBACK grant beside its printed Retrace
// casts both ways (gorge lists both options): the flashback instance is the
// plain mode-2 cast (target byte 0, the granted instance's cost, sfFlashback
// and the resolution exile), the retrace one the marked action. kgen refuses
// the printed-keyword collisions (Flashback, Harmonize, Mayhem, BeamMeUp),
// so the target byte alone separates the two instances of one card.

// retraceTgt is the retrace cast's target-byte mark, shared with canon
// (translate.go) and the oracle (sbt.go), which canonicalise it as gorge's
// "retrace" option (CastModeOther, like "harmonize"). 1 is free for a
// mode-2 cast: the plain flashback and the beam me up permission indexes
// never sit on a Retrace card (kgen refuses those keywords beside it).
const retraceTgt uint8 = 1

// FeatRetraceTgt is the target byte of a retrace cast (canon/translate.go
// and the oracle read the canonical mark through it).
const FeatRetraceTgt = retraceTgt

// retraceEntry reports whether the pending spell e is a retrace cast. The
// mark is set by beginCast for a printed Retrace card and for a granted one
// (kop.StatRetraceGrant; kw_offkwgrant.go), so it alone identifies the cast.
func retraceEntry(e *stackEntry) bool {
	return e.trigPl == trigPlRetrace
}

// DebugRetraceFree is the mutation switch (dirac/oracle TestRetraceMutation):
// the cast's pick cost asks nothing, so the cast resolves without charging
// the discard and the lockstep must diverge on the land never discarded.
var DebugRetraceFree bool

// DebugRetraceBarMorph re-adds the morph bar retraceGrantBarred dropped
// (dirac/oracle TestRetraceGrantMorphMutation): the granted retrace offer on a
// morph card is withheld again and the recorded game ends in the unmodelled
// retracegrant stall.
var DebugRetraceBarMorph bool

// retraceCost is the cast's pick cost: the additional discard-a-land cost
// (gorge retraceExtra through the one definition the offer gate prices). A
// card without printed Retrace takes the granting static's own cost (Six's
// grant builds the same Discard<1/Land>; kw_offkwgrant.go).
func (s *State) retraceCost(k *prep, e *stackEntry) *kop.Cost {
	if DebugRetraceFree {
		return nil
	}
	if k.t.Cards[e.card].Retrace {
		if rc := &k.t.Cards[e.card].RetraceCost; rc.Flags != 0 {
			return rc
		}
		return nil // never built (kgen refuses a Retrace card without its cost)
	}
	// the spell is on the stack by the time the pick cost is built, so the
	// card is read as a card, wherever it lies (its graveyard filter is the
	// grant's zone, but the object has left it): pass NoObj so the matcher
	// skips the zone check, as fbStatic reads a flashback grant's card.
	owner := s.objs[e.obj].owner()
	if ab := s.offKwStatic(k, e.ctrl, owner, e.card, kop.ZoneGraveyard, NoObj); ab != nil && ab.Cost.Flags != 0 {
		return &ab.Cost
	}
	return nil
}

// StallRetraceGrant: a StatRetraceGrant retrace offer on a card whose other
// cast route the retrace walk does not price, or on a table with a beam me
// up grant (the mode-2 target byte is a permission index there). The offer
// is withheld rather than mispriced (kw_offkwgrant.go).
const StallRetraceGrant = 181

func init() { stallNames[StallRetraceGrant] = "retracegrant" }

// retraceGrant is the StatRetraceGrant retrace static giving graveyard card o
// its keyword now, or nil.
func (s *State) retraceGrant(k *prep, o Obj) *kop.Ability {
	ob := &s.objs[o]
	if ob.zone != kop.ZoneGraveyard || k.t.Cards[ob.card].Retrace {
		return nil
	}
	return s.offKwStatic(k, ob.owner(), ob.owner(), ob.card, kop.ZoneGraveyard, o)
}

// retraceLive reports whether graveyard card o, without printed Retrace, has
// retrace a StatRetraceGrant static gives it now.
func (s *State) retraceLive(k *prep, o Obj) bool {
	return s.retraceGrant(k, o) != nil
}

// retraceGrantBarred reports whether a granted retrace cast is one the
// kernel does not model: the card prints another graveyard-cast route (the
// canon read is one mode-2 kind per card), a cast route or additional cost
// the retrace walk does not price, or the table has a beam me up grant (the
// target byte's permission index).
//
// A printed Retrace is safe because kgen's lowerRetrace refuses the card
// beside these; a granted one lands on any nonland permanent card in the
// graveyard, so the same refusal list is applied to the granted card here
// (buildPickCost's retrace branch returns the discard alone, so a spell's own
// additional parts would be dropped). Keep the two lists in step. Delve is
// neither: it applies to the spell on the stack whatever zone it was cast
// from (CR 702.66a), so offerRetraceCasts prices its graveyard credit as the
// other graveyard walks do and the cast build's bsDelve stage asks it.
// Kicker is neither either: a kicked cast is a hand-only offer (gorge
// legal_walk_hand.go, cast.go's hand walk; the kicker cost folds only in
// gorge's castModeKicked), so a Kicker card's retrace cast is the plain one in
// both engines -- the printed base plus the discard, the kicker unpaid, and
// neither engine offers a kicked retrace cast. Warp is neither: its cost is a
// hand-only permission (CR 702.185a, "cast this card from your hand"; gorge's
// warp walk is the hand's, and a graveyard cast carries no modeWarped mark), so
// a Warp card's retrace cast is the plain one -- the printed cost plus the
// discard, never warped, never exiled at the end step. Morph is neither: the
// face-down morph cast is a hand-and-command-zone offer (face_facedown.go's
// offerFaceDownHand; offerRetraceCasts has no face-down arm either), so a
// morph card's retrace cast is the plain face-up one -- the printed cost plus
// the discard, never face down (morph is not an additional cost on the
// face-up cast).
// X and Convoke are neither: the announced X (CR 107.3) and the convoke picks
// (CR 702.51a) apply to the spell whatever zone it was cast from, and the cast
// build's bsX and bsConvoke stages ask them for a retrace cast as for any
// other; offerRetraceCasts credits the picks in its price (convokeCredit,
// gorge castOfferBase) and retraceConvoked keeps the planner's pre-discard
// tapping off a convoked retrace cast. Improvise stays barred.
func retraceGrantBarred(k *prep, c CardID) bool {
	cd := &k.t.Cards[c]
	if DebugRetraceBarMorph && cd.MorphCost != (kop.ManaCost{}) {
		return true
	}
	return cd.Keywords&kop.KwFlashback != 0 || cd.Harmonize || cd.Mayhem || cd.BeamMeUp || cd.Escape ||
		cd.Improvise ||
		cd.HasPlot || cd.HasSneak || cd.Impending != 0 || cd.HasBestow ||
		cd.SpellCost.Flags != 0 || cd.HasAltAdd || cd.Offspring || cd.Gift ||
		cd.Casualty != 0 || cd.Bargain || cd.OptCost.Flags != 0 || cd.Teamwork != 0 || cd.HasModeCost || cd.HasMadness ||
		k.gen != nil && k.gen.anyBeam
}

// offerRetraceCasts offers the graveyard casts of p's Retrace cards (gorge
// legal_walk_alt.go's retrace walk: spellTimingOK, castRestricted, castTargets,
// and the discard payable gate -- an option whose discard cannot be paid must
// never be offered). The mana is the printed cost priced with the walk's own
// reductions (gorge offerCastable's offerCostFor); the cast's pick cost is
// gated separately by pickCostPayable, and the floating-pool rule prices the
// offer like the mayhem cast's (gorge's payment planner plans no retrace
// mode).
func (s *State) offerRetraceCasts(k *prep, d *Decision, p uint8, sorc bool, av *avail) {
	pl := &s.pl[p]
	have := false
	for i := 0; i < int(pl.graveN); i++ {
		o := pl.grave[i]
		c := s.objs[o].card
		cd := &k.t.Cards[c]
		if !cd.Retrace {
			if !s.retraceLive(k, o) {
				continue
			}
			if retraceGrantBarred(k, c) {
				s.capFull(StallRetraceGrant) // a granted cast the kernel does not model
				continue
			}
		}
		if !k.cards[c].instant && !sorc && !s.castFlash(k, p, c) {
			continue
		}
		if k.gen != nil && k.gen.anyCantCast && s.castForbidden(k, p, c, o) {
			continue
		}
		if !have {
			s.availFor(k, p, av)
			have = true
		}
		base := cd.Cost
		if k.gen != nil && k.gen.anyConvoke && s.tapPays(k, p, c, o, false) {
			base = s.convokeCredit(k, p, c, o, base) // gorge castOfferBase: the graveyard cast convokes too (CR 702.51a)
		}
		fc := s.adjustCost(k, p, c, o, base)
		av.delve = s.tbDelveCredit(k, p, c, o, false) // the faces' credit (CR 702.66a)
		if !av.tbLeafCredited(&fc) {
			fc = s.delveOfferCredit(k, p, c, o, false, fc) // gorge's choke point credits the generic at this walk too
		}
		if !s.castAffordsR(k, p, c, s.restrOriginOfObj(o), &fc, av) || !s.spellTargetsFeasibleFrom(k, c, p, o) || !s.spellCostsFeasible(k, c, p, o) {
			continue
		}
		rc := &k.t.Cards[c].RetraceCost
		if !cd.Retrace {
			if ab := s.retraceGrant(k, o); ab != nil {
				rc = &ab.Cost
			}
		}
		if rc.Flags == 0 || !s.pickCostPayable(k, p, rc, o) {
			continue // the additional discard-a-land cost (CR 601.2h, 702.81a)
		}
		if s.castHeldOut(o) {
			continue
		}
		if s.cfg&(cfgGorgeAutopay|cfgGorgeLowering) != 0 && s.g.availX&availWalk == 0 && !s.FeatPoolPays(p, o, 2|8) {
			// gorge's payment planner plans no retrace mode (payment_plan.go
			// plans plain, may-play and flashback casts only): the cast is
			// offered only when the floating pool pays the printed base (the
			// mayhem cast's rule)
			continue
		}
		d.add(MakeAction(AkCast, TCode(o), TCode(retraceTgt), 2))
	}
}

// FeatRetraceCard reports whether o's card is a Retrace card (the oracle
// canonicalises its retrace cast as gorge's "retrace" option).
func (s *State) FeatRetraceCard(o Obj) bool {
	k := s.k()
	return k.t.Cards[s.objs[o].card].Retrace || s.retraceLive(k, o)
}

// retraceConvoked reports whether e is a retrace cast that announces convoke
// picks. Gorge's planner plans no retrace mode, so the cast's mana comes from
// the floating pool the offer gate priced at the convoke-credited base
// (FeatPoolPays mode 2|8), and the discard being chosen taps no source: the
// planner's pre-discard tapping (payCastParts) is for a planned cast and would
// tap lands for the unreduced cost the picks already pay.
func (s *State) retraceConvoked(k *prep, e *stackEntry) bool {
	return retraceEntry(e) && k.gen != nil && k.gen.anyConvoke && s.tapPaysEntry(k, e)
}
