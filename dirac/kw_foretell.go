package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Foretell (CR 702.143a; gorge rules legal_walk_hand.go's "foretell" cast
// option, cast_commit.go payCast's foretell branch, legal_walk_alt.go
// exileCastsWalk's "foretell_cast", altcast.go foretellCastAvailable):
//
//   - the ACTION: during its controller's turn, whenever they have priority
//     (no sorcery timing, unlike plot), they may pay {2} -- never the
//     keyword's cost -- and exile the card from their hand face down. It is
//     not a cast: nothing is pushed, no cast trigger fires. The kernel
//     offers it as AkPlot with mode plotForetell (a special action of a hand
//     card that pays a fixed cost and exiles the card), only while the
//     floating pool pays it (gorge's potential planner prices no foretell),
//     and pays it as the plot action is (kw_plot.go payAndExile): a cost static of the
//     spell scope never reaches it (gorge's own Foretell scope), and
//     CardDef.ForetellCost is not read.
//   - the designation: a DurForetell mayPlay record stamped with the turn
//     (genState.may), like plot's DurPlot; ownWalkerRecord keeps it off the
//     generic play surfaces and cleanupGen drops it with the exile (a new
//     object, CR 400.7).
//   - the CAST: on a later turn the owner may cast the card from exile for
//     CardDef.ForetellCost, with the card's own timing, targets and
//     prohibitions; gorge offers it from the real-pool walk only, so the
//     floating pool alone must pay it (as warp's recast). It is AkCast mode
//     0 with castForetellTgt in the target byte from exile; legal.go sets
//     genState.impCast and beginCast marks the entry trigPlForetold, which
//     buildCostGen prices (CR 601.2f modifiers apply; the entry's origin zone
//     is exile for the wasCastFromExile reads).
//
// A stack copy clears the mark (copy_gen.go): a copy is not cast.

// plotForetell is AkPlot's mode for the foretell action.
const plotForetell uint8 = 1

// castForetellTgt in an AkCast's target byte (mode 0, from exile) is the
// cast for the card's foretell cost (offerForetellCasts).
const castForetellTgt = 5

// FeatCastForetellTgt is castForetellTgt for the oracle adapter.
const FeatCastForetellTgt = castForetellTgt

// FeatPlotForetell is plotForetell for the oracle adapter.
const FeatPlotForetell = plotForetell

// DebugForetellSameTurn relaxes the later-turn rule on the foretold cast
// (mutation testing only: the lockstep must catch it).
var DebugForetellSameTurn bool

// DebugForetellSorcery makes the foretell action wait for sorcery timing
// (mutation testing only: the lockstep must catch it).
var DebugForetellSorcery bool

// foretellCost is the cost of casting c from a foretell exile: the card's
// foretell cost, less any reduction (CR 601.2f; gorge prices the cast under
// the Spell scope with the foretell_cast mode).
func (s *State) foretellCost(k *prep, p uint8, c CardID, o Obj) kop.ManaCost {
	return s.adjustCostGen(k, p, c, o, k.t.Cards[c].ForetellCost)
}

// offerForetellHand offers hand card o's foretell special action. The caller
// has checked the card has foretell and that p is the active player. gorge
// offers the action from the real-pool walk only: the potential planner
// prices no "foretell" mode (potentialModeBaseCost), so unlike plot's it is a
// play only while the floating pool (and the restricted mana a special action
// admits) pays {2}; a pool that must be tapped first sees it after the taps.
//
//go:noinline
func (s *State) offerForetellHand(k *prep, d *Decision, p uint8, o Obj, c CardID) {
	pc := kop.ManaCost{Generic: 2}
	a := s.poolAvailPlus(p, nil)
	if s.rbAny(p) {
		if m := s.restrPoolNonSpell(k, p); m != 0 {
			s.restrAvailAdd(p, m, &a) // mana_specialaction.go: only a non-spell restriction funds it
		}
	}
	if !a.affords(&pc, pipMaskOf(&pc), 0) {
		return
	}
	if s.castHeldOut(o) || k.gen.anyCantCast && s.castForbidden(k, p, c, o) {
		return // gorge castSuppressed / castRestricted gate the hand card first
	}
	d.add(MakeAction(AkPlot, TCode(o), 0, plotForetell))
}

// foretellAction performs the foretell special action (CR 702.143a): pay {2},
// move the card from hand to exile and arm the later cast. Not a cast:
// nothing is pushed and no cast trigger fires.
func (s *State) foretellAction(k *prep, o Obj, p uint8) {
	s.payAndExile(k, p, o, kop.ManaCost{Generic: 2})
	ob := &s.objs[o]
	if s.mayFull() {
		s.capFull(StallMayPlay)
		return
	}
	*s.mayAt(int(s.g.nmay)) = mayPlay{obj: o, ver: ob.ver, ctrl: ob.owner(), until: kop.DurForetell, turn: s.turn}
	s.g.nmay++
	s.passes = 0
}

// foretellRecord reports whether p holds a live DurForetell record for o on a
// turn strictly after the action's (gorge foretellCastAvailable's TurnChange
// after the exile).
func (s *State) foretellRecord(p uint8, o Obj) bool {
	ob := &s.objs[o]
	for i := 0; i < int(s.g.nmay); i++ {
		r := *s.mayAt(i)
		if r.obj == o && r.ctrl == p && r.ver == ob.ver && r.until == kop.DurForetell && (s.turn > r.turn || DebugForetellSameTurn) {
			return true
		}
	}
	return false
}

// foretold reports whether exiled card o carries a live foretell record: it
// was exiled face down by the action and has not moved since (gorge's
// Object.FaceDown, set by the action's face-down exile move and cleared by any
// later one; the lockstep projection reads it).
func (s *State) foretold(o Obj) bool {
	ob := &s.objs[o]
	for i := 0; i < int(s.g.nmay); i++ {
		if r := s.mayAt(i); r.obj == o && r.ver == ob.ver && r.until == kop.DurForetell {
			return true
		}
	}
	return false
}

// offerForetellCasts offers p's foretold cards' cast from exile (gorge rules
// legal_walk_alt.go exileCastsWalk's "foretell_cast"): only on a later turn
// than the action, with the card's own timing, prohibitions and castable
// targets, for its foretell cost paid by the floating pool alone.
func (s *State) offerForetellCasts(k *prep, d *Decision, p uint8, sorc bool) {
	pl := &s.pl[p]
	for i := 0; i < int(pl.exileN); i++ {
		o := pl.exile[i]
		c := s.objs[o].card
		if !s.foretellRecord(p, o) {
			continue
		}
		if pc := &k.cards[c]; !pc.instant && !sorc && !s.castFlash(k, p, c) {
			continue
		}
		if s.castHeldOut(o) || k.gen.anyCantCast && s.castForbidden(k, p, c, o) {
			continue
		}
		if !s.spellTargetsFeasible(k, c, p) || !s.spellCostsFeasible(k, c, p, o) {
			continue
		}
		if !s.foretellPoolPays(k, p, o, c) {
			continue
		}
		d.add(MakeAction(AkCast, TCode(o), castForetellTgt, 0))
	}
}

// foretellPoolPays reports whether p's floating pool, with the restricted
// mana a spell of o's origin may spend, pays the foretold cast (FeatPoolPays
// for the foretell cost).
func (s *State) foretellPoolPays(k *prep, p uint8, o Obj, c CardID) bool {
	a := s.poolAvailPlus(p, nil)
	if s.rbAny(p) {
		s.restrAvail(k, SeatIx(p), c, s.restrOriginOfObj(o), &a)
	}
	cost := s.adjustFit(k, &a, SeatIx(p), c, o, k.t.Cards[c].ForetellCost, k.gen != nil)
	return a.affords(&cost, pipMaskOf(&cost), 0)
}
