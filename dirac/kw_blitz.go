package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Blitz (CR 702.152a): "You may cast this spell by paying [cost] rather than
// its mana cost. If you do, it gains haste and 'When this creature dies, draw
// a card.' Sacrifice it at the beginning of the next end step." Cold, reached
// only when the table has a card with printed blitz (genPrep.anyBlitz()). The
// printed K:Blitz is CardDef.HasBlitz and BlitzCost (kop.KwBlitz).
//
// The hand offers the blitzed cast (gorge's "blitzed" option, altcast_modes.go
// altBlitz). Unlike dash it is no potential play: gorge's planner prices no
// blitzed cast, so under gorge's autopay and lowering surfaces only a
// floating pool that pays BlitzCost offers it (the cleaved cast's rule,
// kw_cleave.go; gorge legal_walk_hand.go's blitzCosts offer). It rides AkCast
// mode 0 with castBlitzTgt in the target byte; legal.go sets genState.impCast
// and beginCast marks the build entry with modeImpended, which reads as
// "blitzed" on a HasBlitz card (kgen refuses blitz beside every other cast
// cost route). buildCost prices BlitzCost. A stack copy clears the mode bit
// (copy_gen.go), so a copy is not blitzed. Gorge offers no blitzed cast from
// the command zone (the row has no cmdLoop), and kgen refuses the graveyard
// cast of Spell.Blitz and a granted Blitz.
//
// The permanent that cast produces (gorge altcast.go altCostEnter, blitz.go
// blitzEnter) gains haste until end of turn, "when this creature dies, draw a
// card" and a sacrifice at the beginning of the next end step:
//   - haste is an until-end-of-turn grant (the dash shape);
//   - the sacrifice is the untracked next-end-step record (dlyUntracked,
//     gorge's __kwBlitzSacrifice is not incarnation-tracked): put on the stack
//     even over a permanent that left, and resolved by resolveSacUntracked;
//   - the draw is a leave-keyed record (dlyBlitzDies, act opBlitzDraw): gorge's
//     granted trigger lives exactly as long as the permanent is on the
//     battlefield, so any departure consumes the record, and one into a
//     graveyard queues the draw for the cast's controller (blitzLeft).

const _ = kop.KwBlitz

// useBlitz is genPrep.uses' bit for a table with a printed blitz card: the
// guards below read it, so the feature adds no genPrep bool of its own (the
// pinned fixed-State inventory, TestNoNewFixedStateFields).
var useBlitz = newGenUse(func(t *kop.Table) bool {
	for i := range t.Cards {
		if t.Cards[i].HasBlitz {
			return true
		}
	}
	return false
})

// anyBlitz reports whether the table has a card with printed blitz.
func (g *genPrep) anyBlitz() bool { return g.uses.has(useBlitz) }

// castBlitzTgt in an AkCast's target byte (mode 0, from the hand) is the cast
// for the card's blitz cost (offerBlitzHand).
const castBlitzTgt = 8

// FeatCastBlitzTgt is the target byte of a blitzed cast (canon/translate.go).
const FeatCastBlitzTgt = castBlitzTgt

// opBlitzDraw is the leave-keyed record's act and the draw trigger's trigAmt
// (kernel-internal, beside opDashReturn; never in an op dispatch).
const opBlitzDraw = 233

// dlyBlitzDies is delayed.when for a record fired by its object leaving the
// battlefield and kept until then (not dropped at a turn's end).
const dlyBlitzDies = 8

// DebugBlitzNoHaste skips the haste grant, DebugBlitzNoSac the end-step
// sacrifice and DebugBlitzNoDraw the dies-draw record (TestBlitzMutation: the
// lockstep must catch each).
var DebugBlitzNoHaste, DebugBlitzNoSac, DebugBlitzNoDraw bool

// blitzed reports whether e is a spell cast for its blitz cost.
func (e *stackEntry) blitzed(k *prep) bool {
	return k.t.Cards[e.card].HasBlitz && e.impended()
}

// offerBlitzHand offers hand card o's blitzed cast (gorge's "blitzed" option).
// The caller has checked the card's timing and the cast prohibition; it runs
// before the plain affordability gate, like cleave's.
//
//go:noinline
func (s *State) offerBlitzHand(k *prep, d *Decision, p uint8, o Obj, c CardID, av *avail) {
	cd := &k.t.Cards[c]
	bc := s.adjustCost(k, p, c, o, cd.BlitzCost) // the modifiers apply to an alternative cost too (CR 601.2f)
	var fav avail
	cav := s.castGrantAvail(k, p, o, av, &fav)
	if !s.castAffordsR(k, p, c, s.restrOriginOfObj(o), &bc, cav) || k.sacMana && !s.kickWitness(k, p, c, o, &bc) {
		return
	}
	if !s.spellTargetsFeasible(k, c, p) || !s.spellCostsFeasible(k, c, p, o) || s.castHeldOut(o) {
		return
	}
	if s.cfg&(cfgGorgeAutopay|cfgGorgeLowering) != 0 && s.g.availX&availWalk == 0 && !s.beamPoolPays(k, p, o, cd.BlitzCost) {
		// gorge's planner prices no blitzed cast (it is no potential play):
		// the option exists only when the floating pool pays it, as a cleaved
		// cast's (kw_cleave.go)
		return
	}
	d.add(MakeAction(AkCast, TCode(o), castBlitzTgt, 0))
}

// blitzEnter runs after the resolving permanent spell e entered at slot sl: a
// blitzed one gains haste until end of turn, arms its sacrifice for the
// beginning of the next end step and its dies-draw record.
//
//go:noinline
func (s *State) blitzEnter(sl Slot, e *stackEntry) {
	ob := &s.objs[e.obj]
	if ob.zone != kop.ZoneBattlefield {
		return
	}
	s.touchSl(sl)
	if !DebugBlitzNoHaste {
		s.grantEOT(sl, kop.KwHaste)
	}
	if int(s.ndly)+2 > MaxDelayed {
		s.capFull(StallDelayed)
		return
	}
	if !DebugBlitzNoDraw {
		s.dly[s.ndly] = delayed{act: opBlitzDraw, ctrl: e.ctrl, obj: e.obj, ver: objWord(ob.ver), when: dlyBlitzDies, creator: NoObj}
		s.ndly++
	}
	if !DebugBlitzNoSac {
		s.dly[s.ndly] = delayed{act: uint8(kop.OpSacrifice), ctrl: e.ctrl, obj: e.obj, ver: objWord(ob.ver), when: dlyUntracked, creator: NoObj}
		s.ndly++
	}
}

// blitzLeft consumes the dies-draw records of object o, which just left the
// battlefield for zone z: one into a graveyard queues the draw trigger for the
// record's controller (CR 702.152a), any other departure drops it.
//
//go:noinline
func (s *State) blitzLeft(o Obj, z kop.Zone) {
	w := uint8(0)
	n := s.ndly
	for i := uint8(0); i < n; i++ {
		d := s.dly[i]
		if d.when == dlyBlitzDies && d.obj == o {
			if z == kop.ZoneGraveyard {
				s.enqueue(o, s.objs[o].ver, s.objs[o].card, abDelayed, d.ctrl, o, d.ctrl, int16(d.act))
			}
			continue
		}
		s.dly[w] = d
		w++
	}
	for i := w; i < n; i++ {
		s.dly[i] = delayed{}
	}
	s.ndly = w
}
