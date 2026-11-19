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
// The kernel lists, per player, the distinct sources of the turn's pushes
// with a push count each (genState.actRing, actPack: source Obj+1 in the low
// ten bits, the count in the high six), recorded only for tables with an
// ExThisTurnActGrave expression (genPrep.actRingOn), so FDN and BLB hash and
// size no differently. The read is a sum over records, so one entry per
// source counts what one per push did. A seventeenth distinct source activated
// by one player in one turn, or a 64th push of one source (object.acts stalls at
// 63 first, StallActs), stalls with StallActRing rather than truncating;
// before 2026-11-19 the ninth push did, which random-policy four-seat
// Commander pods reached in 11 of 73 edh games (a repeatable pump). Not modelled: a proposal reversed
// before payment (gorge ActivationReversed drops the record); only a failed
// waterbend payment reverses an activation here and no pool reads this ring
// together with waterbend.

// actRingN is the distinct sources per player per turn the ring records (8
// before 2026-11-19, when one entry was one push).
const actRingN = 16

// actPack packs a ledger entry: source Obj+1 below actCntShift, the push
// count above it (0 = an empty entry). The grant ledger (act_grantledger.go)
// packs its hosts the same way.
const (
	actObjMask  Obj = 1<<actCntShift - 1
	actCntShift     = 10
	actCntMax       = 1<<(16-actCntShift) - 1
)

// Obj+1 fits below the count: MaxObj ids and NoObj stay under 1<<actCntShift.
var _ = [1]struct{}{}[(int(NoObj)+1)>>actCntShift]

// actBump adds one push of source o to ring r: the existing entry's count, or
// the first empty entry. It reports false when neither is possible.
func actBump(r []Obj, o Obj) bool {
	for i := range r {
		switch {
		case r[i] == 0:
			r[i] = o + 1 | 1<<actCntShift
			return true
		case r[i]&actObjMask == o+1:
			if r[i]>>actCntShift == actCntMax {
				return false
			}
			r[i] += 1 << actCntShift
			return true
		}
	}
	return false
}

// noteAct records the push of an ability of source o by activator (pushBuild).
//
//go:noinline
func (s *State) noteAct(o Obj, activator uint8) {
	if !actBump(s.g.actRing[activator][:], o) {
		s.capFull(StallActRing)
	}
}

// thisTurnActGrave evaluates ExThisTurnActGrave: the evaluating controller's
// activations whose source object is in a graveyard now (a token or copy
// that left the game is not).
//
//go:noinline
func (s *State) thisTurnActGrave(ctx evalCtx) int {
	n := 0
	for _, e := range s.g.actRing[ctx.ctrl] {
		if e == 0 {
			break // entries fill from the front and are never removed mid-turn
		}
		ob := &s.objs[e&actObjMask-1]
		if ob.zone == kop.ZoneGraveyard && !s.ceasedCopy(ob) {
			n += int(e >> actCntShift)
		}
	}
	return n
}
