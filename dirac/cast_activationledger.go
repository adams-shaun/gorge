package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// gorge's activation ledger (rules activationsThisTurn,
// read by AbilitiesActivatedThisTurnMatching), bounded.
//
// gorge appends one record per activated ability pushed on the stack
// (CR 602.2a: the push; a mana ability never reaches the stack, so it is
// never recorded), cleared at TurnChange. A record names the source object
// and the activator; the spec Activated.YouCtrl+inZoneGraveyard tests the
// activator against the reader and the SOURCE's zone at READ time
// (matchesSpec "Card.inZoneGraveyard"). An ObjID survives zone changes, so
// a cycled card or a sacrificed Mind Stone counts once it sits in a
// graveyard, and a flashback-less graveyard activation counts while it
// stays there.
//
// The kernel lists each player's first actRingN pushes as source Obj+1
// (genState.actRing), recorded only for tables with an ExThisTurnActGrave
// expression (genPrep.actRingOn), so FDN and BLB hash and size no
// differently. A ninth activation by one player in one turn stalls with
// StallActRing rather than truncating. Not modelled: a proposal reversed
// before payment (gorge ActivationReversed drops the record); only a failed
// waterbend payment reverses an activation here and no pool reads this ring
// together with waterbend.

// actRingN is the activations per player per turn the ring records.
const actRingN = 8

// noteAct appends the push of an ability of source o by activator (pushBuild).
//
//go:noinline
func (s *State) noteAct(o Obj, activator uint8) {
	n := s.g.actN[activator]
	if n >= actRingN {
		s.capFull(StallActRing)
		return
	}
	s.g.actRing[activator][n] = o + 1
	s.g.actN[activator] = n + 1
}

// thisTurnActGrave evaluates ExThisTurnActGrave: the evaluating controller's
// activations whose source object is in a graveyard now (a token or copy
// that left the game is not).
//
//go:noinline
func (s *State) thisTurnActGrave(ctx evalCtx) int {
	n := 0
	for i := uint8(0); i < s.g.actN[ctx.ctrl]; i++ {
		ob := &s.objs[s.g.actRing[ctx.ctrl][i]-1]
		if ob.zone == kop.ZoneGraveyard && !s.ceasedCopy(ob) {
			n++
		}
	}
	return n
}
