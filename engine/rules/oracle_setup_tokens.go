package rules

import (
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// validateSetupTokens rejects a token entry setup cannot place: no script, or
// a counter with no kind or a non-positive amount.
func validateSetupTokens(p int, s oracleSeat) error {
	for _, tk := range s.Tokens {
		if tk.Script == "" {
			return harnessf("setup: p%d token has no script", p)
		}
		for _, kind := range sortedSetupKeys(tk.Counters) {
			if kind == "" || tk.Counters[kind] <= 0 {
				return harnessf("setup: p%d token %q counters %q: want a named kind and a positive amount", p, tk.Script, kind)
			}
		}
	}
	return nil
}

// placeSetupTokens mints seat p's setup tokens in list order, so a later
// "pN:token:Name#k" ref binds as it would in the recorded game.
func (r *oracleRun) placeSetupTokens(tokens []oracleSetupToken, p int) error {
	for _, tk := range tokens {
		if r.reg.Tokens[tk.Script] == nil {
			return harnessf("setup: p%d token script %q is not in the corpus", p, tk.Script)
		}
		id, err := r.mintSetupToken(p, tk.Script)
		if err != nil {
			return err
		}
		// Sorted raw kinds: two spellings of one kind emit two additive changes.
		for _, kind := range sortedSetupKeys(tk.Counters) {
			r.e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: normCounter(kind), Amount: tk.Counters[kind]})
		}
		r.e.emit(events.Event{Kind: events.SetupSettled, Obj: id})
	}
	return nil
}

// mintSetupToken emits one TokenCreate with the replacement pass bypassed
// (restored by defer) and discards every trigger the mint queued.
func (r *oracleRun) mintSetupToken(p int, script string) (state.ObjID, error) {
	e := r.e
	pendingBefore := len(e.pendingTriggers)
	var ids []state.ObjID
	func() {
		prior := e.applyingReplacement
		e.applyingReplacement = true
		defer func() { e.applyingReplacement = prior }()
		ids = e.EmitTokenCreate(events.Event{Kind: events.TokenCreate, Player: state.PlayerID(p), Text: script})
	}()
	if len(e.pendingTriggers) > pendingBefore {
		e.pendingTriggers = e.pendingTriggers[:pendingBefore]
	}
	e.orderedTriggers = min(e.orderedTriggers, len(e.pendingTriggers))
	if len(ids) != 1 {
		return 0, harnessf("setup: p%d token script %q minted %d tokens, want 1", p, script, len(ids))
	}
	return ids[0], nil
}
