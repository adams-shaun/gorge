package rules

import (
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// kw:Unleash (CR 702.86): "You may have this creature enter the battlefield
// with a +1/+1 counter on it. While it has a +1/+1 counter on it, it can't
// block."
//
// The choice half is the Riot precedent (CR 702.108): a Forge construct
// choice with no card script, so rules reads the K:Unleash keyword directly.
// The cast path poses it through the shared as-enters machinery
// (collectETBChoices/etbAsk/etbAnswer, rules/cast.go); every non-cast
// battlefield entry (reanimation, blink, search, a direct MoveZone) is caught
// here by applyUnleashReplacement, which parks the move until the answer is
// logged -- the exact shape applyRiotReplacement (rules/replacement.go)
// practises, and the answer handler in rules/turn.go's chooseUnleash arm
// mirrors too. Apply's Move folds the answer ("counter" -> a +1/+1 counter)
// on battlefield entry (events/apply.go), so every entry path and a log-only
// replay agree.
//
// The can't-block half is read in canBlock (rules/combat.go) beside the
// Suspected designation -- the blocker-side gate every option and validation
// path already shares.

// chooseUnleash is the chooseFor state while a parked non-cast entry waits
// on its Unleash answer. Numbered above chooseAttackPay; only pairwise
// distinctness matters (rules/cast.go's chooseFor doc).
const chooseUnleash chooseFor = chooseAttackPay + 1

// unleashOptions are the two answers of the as-enters Unleash choice, in the
// fixed order the cast path (collectETBChoices) and the non-cast path
// (applyUnleashReplacement) both offer: index 0 takes the counter, index 1
// declines it. Index 0 is also the deterministic bot/host answer (option 0),
// which CR 702.86's "you may" permits.
func unleashOptions(id state.ObjID, p state.PlayerID) []decision.Option {
	return []decision.Option{
		{Index: 0, Kind: "unleash", Label: "Enter with a +1/+1 counter", Obj: id, Player: p},
		{Index: 1, Kind: "unleash", Label: "Enter without a counter", Obj: id, Player: p},
	}
}

// applyUnleashReplacement parks every non-cast battlefield entry of an
// Unleash creature before it happens, mirroring applyRiotReplacement. The
// same overwrite guard applies: never park on an ask while another decision
// is outstanding, and never re-ask an object whose choice was already
// recorded (a cast-path entry arrives with UnleashChoice set by the Choose
// event etbAnswer emitted, so the guard is what keeps the two paths from
// asking twice). Entries of face-down objects (a manifest) are vanilla 2/2
// creatures (CR 708.5): the printed face does not exist while face down, so
// the printed-keyword read below never matches -- and the Choose event is
// not Secret, so it must not name the hidden card in the public transcript.
func (e *Engine) applyUnleashReplacement(ev events.Event) bool {
	if ev.To != state.ZBattlefield || e.unleashMove != nil || e.pending != nil {
		return false
	}
	o := e.G.Obj(ev.Obj)
	if o == nil || o.Zone == state.ZBattlefield || o.Face() == nil ||
		!o.Face().HasKeyword("Unleash") || o.UnleashChoice != "" {
		return false
	}
	move := ev
	e.unleashMove = &move
	d := &decision.Decision{Player: o.Controller, Kind: decision.KChoose, Min: 1, Max: 1,
		Source: o.ID, Prompt: "Choose how this creature enters (with a +1/+1 counter or without)",
		Options: unleashOptions(o.ID, o.Controller)}
	e.choosing = chooseUnleash
	e.ask(d)
	return true
}

func init() {
	effects.RegisterNonAPI("kw:Unleash")
}
