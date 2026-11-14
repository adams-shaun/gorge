package dirac

// A while-exiled owner's permission (Soul Partition, CR
// 611.2a): "For as long as that card remains exiled, its owner may play it.
// A spell cast by an opponent this way costs {2} more to cast." kgen lowers
// the Effect's two statics into one OpMayPlay over the remembered exiled
// card with kop.MayPlayOwner and the surcharge. The record is the owner's
// (gorge MayPlayPlayer$ CardOwner), lives while the card stays the same
// exiled object (cleanupGen keeps a DurPermanent record by plotLive), and
// carries mayKindRaise with the surcharge in its turn field when the owner
// is the effect controller's opponent (RaiseCost$ Activator$ Opponent). The
// surcharge is added to the spell's generic cost before the cost statics,
// as gorge's planner and settle price a may-play cast's RaiseCost$
// (mayPlayRaiseCost: settleBase plus the raise, then offerCostFor).

// mayKindRaise marks a permission record whose cast owes the surcharge in
// mayPlay.turn (outside kop.MayPlayKinds: bit 1 is free in a record).
const mayKindRaise uint8 = 2

// partitionRecord makes r the owner's while-exiled permission for o, with
// the surcharge raise owed when the owner is not the effect's controller.
// turn is only the raise when one is owed; otherwise it stays the record's
// creation turn, which the DurYourNextTurn cleanup reads (Expedited
// Inheritance's EffectOwner$ RememberedController grant).
func (s *State) partitionRecord(r *mayPlay, o Obj, ctrl uint8, raise int) {
	owner := s.objs[o].owner()
	r.ctrl = owner
	if raise > 0 && owner != ctrl {
		r.kind |= mayKindRaise
		r.turn = mayWord(raise)
	}
}

// partitionRaise is the surcharge adjustCostGen adds to object o's cost:
// the one being offered, or the one the cast being built owes.
//
//go:noinline
func (s *State) partitionRaise(o Obj) int {
	if s.g.raiseO != 0 {
		if DebugNoPartitionRaise {
			return 0
		}
		return int(s.g.raiseO)
	}
	if s.ctl == ctlBuild && o == s.bld().e.obj && !DebugNoPartitionRaise {
		return int(s.g.raiseB)
	}
	return 0
}

// DebugNoPartitionRaise drops the surcharge (the partition mutation test).
var DebugNoPartitionRaise bool
