package pay

import (
	"github.com/adams-shaun/gorge/engine/events"
	costvocab "github.com/adams-shaun/gorge/engine/rules/cost"
	"github.com/adams-shaun/gorge/engine/state"
)

// MillCostPayable reports whether payer p can pay every Mill<N> cost
// component in full: CR 701.17b, "the player can't pay a cost that includes
// milling a number of cards greater than the number of cards in their
// library" (CR 118.3). The requirement is the SUM of the parts, so a composed
// Mill<1> Mill<1> needs two cards. A cost with no Mill part is payable; a
// negative part (MillCostTotal's !ok) never is. Every Mill offer gate --
// NonManaCastableP, ManaCostPayableFull and unlessComponentsPayable -- reads
// this one helper, so the three cannot disagree.
func MillCostPayable(g *state.Game, p state.PlayerID, parts []costvocab.CostPart) bool {
	if len(parts) == 0 {
		return true
	}
	total, ok := costvocab.MillCostTotal(parts)
	return ok && total <= int64(len(g.Zone(state.ZLibrary, p)))
}

// PayMillCost settles every Mill<N> cost component: the payer mills the SUM
// of the parts' requirements from the top of their own library, one real
// MoveZone event per card in deterministic top-first order. The moves share a
// mill batch so MilledAll triggers once for this cost payment. No choice is
// involved, so nothing is asked. The offer gates (MillCostPayable) withhold a
// cost the library cannot cover (CR 701.17b), so the clamp below is only a
// guard for a library a mana ability shrank mid-payment; it never makes a
// short library payable.
func PayMillCost(e Engine, p state.PlayerID, parts []costvocab.CostPart) {
	total, ok := costvocab.MillCostTotal(parts)
	if !ok || total <= 0 {
		return
	}
	lib := e.Game().Zone(state.ZLibrary, p)
	if int64(len(lib)) < total {
		total = int64(len(lib))
	}
	// Snapshot the ids before emitting: each MoveZone mutates the library
	// the slice was read from.
	ids := append([]state.ObjID(nil), lib[:total]...)
	e.Batch(BatchMill, true)
	for _, id := range ids {
		e.Emit(events.Mill(id, p))
	}
	e.Batch(BatchMill, false)
}

// PayDiscardCost settles every hand->graveyard discard component of a cost
// payment as ONE discard action: the payer discards the settled cards
// together, so Mode$ DiscardedAll fires once for the payment with its
// TriggerCount$Amount (and Remembered/Captured set) equal to the number of
// matching cards (CR 701.8), exactly as PayMillCost batches a multi-card Mill
// cost. Every cost discard goes through this helper -- a cast, an activated
// ability, a mana-ability activation and a triggered mandatory cost all reach
// it -- so the four sites cannot diverge and a new one cannot forget the
// bracket. The bracket is opened and closed entirely inside this call and the
// emission loop cannot suspend, so successive cost actions never coalesce and
// an enclosing api:Discard batch (engine/effects/cardflow.go's effDiscard) simply
// nests by depth. cycling names the cycling ability when the discard is paid
// for one (CR 702.29), tagging each card's event for Mode$ Cycled; an empty
// keyword emits the plain cost form. An absent object is skipped, exactly as
// the triggered-cost site's own guard did.
func PayDiscardCost(e Engine, ids []state.ObjID, cycling string) {
	if len(ids) == 0 {
		return
	}
	e.Batch(BatchDiscard, true)
	for _, id := range ids {
		if e.Game().Obj(id) == nil {
			continue
		}
		if cycling != "" {
			e.Emit(events.DiscardCostCycling(id, cycling))
		} else {
			e.Emit(events.DiscardCost(id))
		}
	}
	e.Batch(BatchDiscard, false)
}
