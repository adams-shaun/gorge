package rules

import (
	"fmt"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// oracleSetupToken is one token a seat's setup places on the battlefield.
//
// Setup is placement, not creation (CR 111.2 still makes the seat the owner
// and controller): the token is minted with a TokenCreate keyed by its Forge
// token-script stem (Game.Tokens), but no trigger the mint queues fires
// ("when this enters", "whenever you create a token") and no CreateToken
// replacement (Doubling Season) rewrites it -- XMage's cheat placement sees
// neither. Counters is the token's COMPLETE counter set after setup (a placed
// token enters with none of its own, so unlike the card field Counters it is
// not added on top of entry counters). A scenario lists the token's whole
// state; nothing the script grants on entry is invented here.
type oracleSetupToken struct {
	Script   string           `json:"script"`
	Counters map[string]int32 `json:"counters,omitempty"`
}

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
func (r *oracleRun) placeSetupTokens(sc oracleScenario, p int) error {
	for _, tk := range sc.Setup[fmt.Sprintf("p%d", p)].Tokens {
		if r.reg.Tokens[tk.Script] == nil {
			return harnessf("setup: p%d token script %q is not in the corpus", p, tk.Script)
		}
		id, err := r.mintSetupToken(p, tk.Script)
		if err != nil {
			return err
		}
		byKind := map[string]int32{}
		for kind, n := range tk.Counters {
			byKind[normCounter(kind)] += n
		}
		for _, kind := range sortedSetupKeys(byKind) {
			r.e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: kind, Amount: byKind[kind]})
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
