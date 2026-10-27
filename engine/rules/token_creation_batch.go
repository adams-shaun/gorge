package rules

import (
	"github.com/adams-shaun/gorge/engine/state"
)

// tokenCreationKey names one fixed-Amount$ Type$ AddToken replacement's single
// claim on one token-creation batch: the batch, the replacement (its source
// object and Effect key), its extra TokenScript$, and the controller the extra
// is created for (Forge's ReplaceTokenEffect groups the event's token table by
// controller and adds the fixed count once per controller).
type tokenCreationKey struct {
	batch  uint64
	src    state.ObjID
	key    string
	script string
	ctrl   state.PlayerID
}

// BeginTokenCreation opens the creation batch of one Token resolution
// (effects.runTokenMints): its mints are ONE "create tokens" event for a fixed
// Type$ AddToken replacement (Quina, Xorn, Case of the Pilfered Proof; CR
// 614.1a, 616), although each is its own TokenCreate. Batches nest as a stack,
// so a Token resolution begun inside another's window gets its own batch and
// the outer one resumes after it. Engine memory only: a replay folds the same
// resolutions through the same brackets.
func (e *Engine) BeginTokenCreation() {
	e.tokenCreationSeq++
	e.tokenCreationOpen = append(e.tokenCreationOpen, e.tokenCreationSeq)
}

// EndTokenCreation closes the innermost creation batch.
func (e *Engine) EndTokenCreation() {
	e.popTokenCreation()
}

func (e *Engine) popTokenCreation() {
	if n := len(e.tokenCreationOpen); n > 0 {
		e.tokenCreationOpen = e.tokenCreationOpen[:n-1]
	}
	// The claims are only read while some batch is open or a mint of one is
	// still parked behind an ask; otherwise they are dead weight.
	if len(e.tokenCreationOpen) == 0 && len(e.replChoices) == 0 && e.tokenChoice == nil {
		e.tokenCreationDone = nil
	}
}

// curTokenCreation is the open creation batch's id, 0 when none is open
// (Investigate N, Amass, Incubate and the like: one event per action).
func (e *Engine) curTokenCreation() uint64 {
	if n := len(e.tokenCreationOpen); n > 0 {
		return e.tokenCreationOpen[n-1]
	}
	return 0
}

// withTokenCreation re-enters batch id around fn, for a parked mint's answer
// resuming after its Token resolution's bracket has closed.
func (e *Engine) withTokenCreation(id uint64, fn func()) {
	if id == 0 {
		fn()
		return
	}
	e.tokenCreationOpen = append(e.tokenCreationOpen, id)
	fn()
	e.popTokenCreation()
}

// claimTokenCreationAdd reports whether the fixed AddToken replacement m may add
// its block for ctrl now: always outside a batch, else only the first time in
// the open batch.
func (e *Engine) claimTokenCreationAdd(m replMatch, script string, ctrl state.PlayerID) bool {
	b := e.curTokenCreation()
	if b == 0 {
		return true
	}
	k := tokenCreationKey{batch: b, src: m.id, key: m.key, script: script, ctrl: ctrl}
	if e.tokenCreationDone[k] {
		return false
	}
	if e.tokenCreationDone == nil {
		e.tokenCreationDone = make(map[tokenCreationKey]bool)
	}
	e.tokenCreationDone[k] = true
	return true
}
