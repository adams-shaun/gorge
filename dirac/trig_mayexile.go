package dirac

import (
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// a printed dies trigger's own optional "you may
// exile it" cost ExileAnyGrave<1/Card.TriggeredNewCard> (Sandbender
// Scavengers, Paramecia Coloniex; gorge's triggered-cost window: the
// pay/decline ask, then the triggered card moves to exile unasked — one
// candidate, no pick). The cost rides PayExileGrave with B.N PayBSubTrigNew
// (Instr.C's nibble is full) and execPay widens it to payKindTrigNew.
// Payable only while the card the triggering event captured is still in a
// graveyard at the version the trigger saw (a graveyard-hate response makes
// the cost unpayable: decline-only). Cold and out of line.

// payKindTrigNew is the pay kind (execPay's kind, pickZ) of a trigger's own
// ExileAnyGrave<1/Card.TriggeredNewCard> cost. The instruction's C nibble is
// full (PayExileGrave is the filter exile's), so the cost rides PayExileGrave
// with B.N = kop.PayBSubTrigNew and execPay widens it to this value.
const payKindTrigNew = 17

// DebugExileTrigNewAnyCard (TestExiletrignewMutation): the payment exiles
// the payer's top graveyard card instead of the triggered card.
var DebugExileTrigNewAnyCard bool

// ExileTrigNewStats counts a test run's trigger-own ExileAnyGrave payments.
type ExileTrigNewStats struct {
	Paid    atomic.Int64 // the cost paid: the triggered card exiled
	Decline atomic.Int64 // the payer declined
}

// DebugExileTrigNewStats, when set (dirac/oracle/exiletrignew_test.go),
// counts the payments.
var DebugExileTrigNewStats *ExileTrigNewStats

// trigNewInGrave is the triggered card while it is still in a graveyard at
// the version the trigger captured: e.li's snapshot holds the battlefield
// version (zone_lki.go; a dies trigger snapshots before the death), and the
// death places the card in its owner's graveyard one version later
// (trig_objmoved.go's trigObjMoved is the same read). A card that left (a
// graveyard-hate response) or came back is a new object: NoObj.
func (s *State) trigNewInGrave() Obj {
	e := &s.res.e
	o := e.trigObj
	if e.kind != seTrigger || o == NoObj || e.li == 0 {
		return NoObj
	}
	ob := &s.objs[o]
	if ob.zone != kop.ZoneGraveyard || ob.ver != s.lk(e.li).ver+1 {
		return NoObj
	}
	return o
}

// exileTrigNew pays the cost: the triggered card is exiled unasked, through
// PayExileGrave's cost exile (exileFromGrave) so the move's event shape
// matches gorge's "exiled as a cost" MoveZone.
func (s *State) exileTrigNew() {
	if DebugExileTrigNewAnyCard {
		pl := &s.pl[SeatIx(s.g.pickP)]
		if pl.graveN > 0 {
			s.exileFromGrave(pl.grave[pl.graveN-1]) // the payer's top card, not the triggered card
			return
		}
	}
	if o := s.trigNewInGrave(); o != NoObj {
		s.exileFromGrave(o)
	}
}

// noteExileTrigNewOutcome tallies the pay/decline answer (payDone).
func (s *State) noteExileTrigNewOutcome(paid bool) {
	if DebugExileTrigNewStats == nil || s.g.pickZ != payKindTrigNew {
		return
	}
	if paid {
		DebugExileTrigNewStats.Paid.Add(1)
	} else {
		DebugExileTrigNewStats.Decline.Add(1)
	}
}
