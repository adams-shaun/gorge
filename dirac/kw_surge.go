package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Surge (CR 702.117a): "You may cast this spell for its surge cost if you or
// a teammate has cast another spell this turn." Cold, reached only when the
// table has a card with printed surge (genPrep.anySurge).
//
// gorge's hand walk (legal_walk_hand.go) offers the "surged" cast option
// when the face has the keyword, pay.SpellsCastThisTurn(p) > 0 (the caster's
// PutOnStack events since the turn began, reversals undone) and
// offerCastable prices the surge cost alone, under the plain cast's timing
// gate but before (and independent of) its affordability gate; beginCast's
// castModeSurged replaces the printed cost with the surge cost (an
// alternative cost, CR 118.9) and modeFlags stamps FlagSurged. The kernel
// reads the caster's spells-cast count off the history bank
// (hist[p][histSpells], written by histCast as each cast is paid; anySurge
// turns anyHist on). Two players have no teammates.
//
// gorge's payment planner plans no surged cast; its potential planner prices
// the composed surge cost (potential_plan.go potentialModeBaseCostSurged),
// the kicked cast's shape: beginCast taps that witness at priority and the
// cast pays from the floating pool, as the kicked cast does (cast.go).
//
// The cast rides AkCast mode 0 with castSurgeTgt in the target byte;
// legal.go sets genState.impCast and beginCast marks the build entry with
// modeImpended, which reads as "surged" on a HasSurge card (kgen refuses
// surge beside every other keyword that claims the bit, and on a modal
// spell, whose chosen modes share stackEntry.mode). A stack copy clears the
// bit (copy_gen.go). The FlagSurged readers (Card.Self+surged, Condition$
// Surge) stay refused in kgen.

// castSurgeTgt in an AkCast's target byte (mode 0, from the hand) is the cast
// for the card's surge cost (offerSurgeHand).
const castSurgeTgt = 6

// FeatCastSurgeTgt is the target byte of a surged cast (canon/translate.go).
const FeatCastSurgeTgt = castSurgeTgt

// DebugSurgeAnyTurn drops the "another spell this turn" gate
// (TestMBCSurgeGateMutation: the lockstep must catch it).
var DebugSurgeAnyTurn bool

// surged reports whether e is a spell cast for its surge cost.
func (e *stackEntry) surged(k *prep) bool {
	return e.kind == seSpell && k.t.Cards[e.card].HasSurge && e.mode&(modeImpended|seFaceDown) == modeImpended
}

// offerSurgeHand offers hand card o's surged cast (gorge's "surged" option).
// The caller has checked the card's timing and the cast prohibition; it runs
// before the plain affordability gate.
//
//go:noinline
func (s *State) offerSurgeHand(k *prep, d *Decision, p uint8, o Obj, c CardID, av *avail) {
	if s.g.hist[SeatIx(p)][histSpells] == 0 && !DebugSurgeAnyTurn {
		return // no other spell cast this turn (CR 702.117a)
	}
	sc := s.adjustCost(k, p, c, o, k.t.Cards[c].SurgeCost) // the modifiers apply to an alternative cost too (CR 601.2f)
	// The surged cast is in gorge's list when the floating pool alone pays
	// the surge cost (gorge's own offer prices it with offerCastable over the
	// pool, legal_walk_hand.go) or its payment planner prices it
	// (potential_plan.go plans a mode cast the plain planner does not
	// witness). The planner has two routes: its tier-1 census (the printed
	// sources and the pool, no granted or sacrifice-only source) pays
	// outright, or its prefix script activates a census gap first
	// (potential_script.go). Either route's payment must leave the spell a
	// legal target (potentialWitnessReaches, cast_tgtkeep.go): pair 35's sole
	// Spawn is the spell's only target and the payment sacrifices it, while
	// pair 61's second Spawn stands. The lowering surface reads the same
	// routes, but its ordinary walk holds the script for the gaps walk
	// (scriptOffers prices it); see the gate below.
	if !s.surgePoolPays(k, p, c, o, &sc) {
		// The lowering surface's ordinary walk reads only the planner's
		// tier-1 census: a scripted play is the gaps walk's, which
		// scriptOffers prices with its own prefix search. Without this gate
		// the full census's granted unit offers a surged cast whose script
		// gorge cannot price (Boulder Salvo: the surge cost's {R} must come
		// from Paradise Mantle's granted tap, and the search's printed-cost
		// colour read never asks for it), while the kernel's census counts
		// the grant. surgeScriptPays still reads the full census in the gaps
		// walk and on the autopay surface, where the search is the route.
		if s.cfg&cfgGorgeLowering != 0 && s.g.availX&(availGaps|availAny) == 0 {
			if !s.surgePlannerPrices(k, p, c, o, &sc) {
				return
			}
		} else if !s.surgePlannerPrices(k, p, c, o, &sc) && !s.surgeScriptPays(k, p, c, o, av, &sc) {
			return
		}
		if !s.surgeKeepsTarget(k, p, c, o, &sc) {
			return
		}
		// The manual surface's census counts every granted and sacrifice-only
		// source, so the payment's taps can still strand the spell's only
		// target: Castle Raptors's "As long as it is untapped, it gets +0/+2"
		// falls to a toughness its marked damage covers, and tapping it for
		// Paradise Mantle's grant kills it. Gorge's prefix search plays the
		// tap and finds no target under the cast, so the surged offer is
		// withheld on both surfaces (surgeKeepsTarget's autopay branch above).
		if !s.surgeKeepsTarget(k, p, c, o, &sc) {
			return
		}
	}
	if k.sacMana && !s.kickWitness(k, p, c, o, &sc) {
		return
	}
	if !s.spellTargetsFeasible(k, c, p) || !s.spellCostsFeasible(k, c, p, o) || s.castHeldOut(o) {
		return
	}
	d.add(MakeAction(AkCast, TCode(o), castSurgeTgt, 0))
}

// surgePoolPays reports whether p's floating pool alone pays the adjusted
// surge cost sc (gorge's own surged offer prices it with offerCastable over
// the pool, legal_walk_hand.go; the pool's restricted units a spell its
// restriction admits see it too, gorge's AvailableFor).
func (s *State) surgePoolPays(k *prep, p uint8, c CardID, o Obj, sc *kop.ManaCost) bool {
	a := s.poolAvailPlus(p, nil)
	if s.rbAny(p) {
		s.restrAvail(k, SeatIx(p), c, s.restrOriginOfObj(o), &a)
	}
	return a.affords(sc, pipMaskOf(sc), 0)
}

// surgePlannerPrices reports whether gorge's payment planner prices the
// surged cast of o: its census -- the printed taps and the pool, a granted
// ability and a sacrifice-only one excluded (potential_plan.go plans a mode
// cast the plain planner does not witness from the mana its sources tap;
// the plain planner's castGrantAvail census, which does count them, is
// gorge's ordinary cast's) -- pays the adjusted surge cost sc.
func (s *State) surgePlannerPrices(k *prep, p uint8, c CardID, o Obj, sc *kop.ManaCost) bool {
	f := s.fork()
	defer s.unfork()
	f.g.noGrant = true
	f.g.availX = availPlain
	save := k.sacOnlyMana
	k.sacOnlyMana = false
	defer func() { k.sacOnlyMana = save }()
	var t1 avail
	f.availFor(k, p, &t1)
	return t1.affords(sc, pipMaskOf(sc), 0)
}

// surgeScriptPays reports whether the prefix script's census pays the surged
// cast: the full offer census (granted and sacrifice-only mana abilities
// included) pays after activating the sources the planner's tier-1 census
// leaves as gaps (potential_script.go potentialPrefixScript). It is only the
// affordability half; the caller also asks surgeKeepsTarget.
func (s *State) surgeScriptPays(k *prep, p uint8, c CardID, o Obj, av *avail, sc *kop.ManaCost) bool {
	var fav avail
	cav := s.castGrantAvail(k, p, o, av, &fav)
	return s.castAffordsR(k, p, c, s.restrOriginOfObj(o), sc, cav)
}

// surgeKeepsTarget reports whether the surged cast keeps a legal target under
// the payment's census: some target candidate the payment's sources can leave
// standing (gorge's potentialWitnessReaches, cast_tgtkeep.go
// specKeepsTarget). The spell probe makes targetCands read the cast's source
// for protection (Mistmeadow Skulk's "protection from mana value 3 or
// greater" excludes Grip of the Roil, whose mana value stays its printed 3
// under the surge alternative cost, CR 202.3); without it a probe-less read
// counts the Skulk as a target and keeps a payment that eats the only one.
func (s *State) surgeKeepsTarget(k *prep, p uint8, c CardID, o Obj, sc *kop.ManaCost) bool {
	sp := tspec(k, c, spellAb(k, c), 0)
	if sp == nil {
		return true
	}
	f := s.fork()
	defer s.unfork()
	f.g.probe = probe{kind: probeSpell, src: NoSlot, card: c, ctrl: p}
	return f.specKeepsTarget(k, p, o, sp, sc)
}

// surgeCostOf is the surged cast's printed alternative cost.
func surgeCostOf(cd *kop.CardDef) kop.ManaCost { return cd.SurgeCost }
