package rules

import (
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// emitSacrifices emits one sacrifice ACTION's Sacrifice events inside one
// Mode$ SacrificedOnce batch (CR 603.2c). The rules-side cost sites use it so
// a payment that sacrifices several permanents queues the "one or more"
// watcher once; every effect-side sacrifice already enters BatchDepartures,
// whose bracket nests with this one. A single id is a batch of one, exactly
// the unbracketed reading.
//
// It captures each permanent's last-known information BEFORE the first
// Sacrifice event, while every permanent is still on the battlefield, and
// returns them in ids order. A simultaneous cost's snapshots are therefore all
// taken at the same instant (CR 608.2h), and a site that reads the sacrificed
// value after the move (a mana ability's Amount$ Sacrificed$CardPower) gets the
// battlefield value -- layer-derived power, a transformed DFC's back face --
// rather than the graveyard face the Move fold left behind.
func (e *Engine) emitSacrifices(ids []state.ObjID) []state.SacrificedInfo {
	if len(ids) == 0 {
		return nil
	}
	lki := make([]state.SacrificedInfo, 0, len(ids))
	for _, id := range ids {
		lki = append(lki, effects.SacrificedLKI(e, id))
	}
	e.beginSacrificeBatch()
	for _, id := range ids {
		e.emit(events.Sacrifice(id))
	}
	e.endSacrificeBatch()
	return lki
}
