// ManaAdded trigger mode.
//
// "Whenever a land's ability causes you to add one or more mana of the
// chosen color, add an additional one mana of that color" (Caged Sun; CR
// 605.1b): a triggered mana ability. It matches the ManaAdd events the mana
// ability's resolution emits, and rules' mana_activation.go lifts the
// Static$ True instances into the CR 605.3b immediate batch the way it lifts
// TapsForMana ones.
//
// The ManaAdd event does not name the producing object: that rides the
// synchronous manaProducer fact the mana-ability resolution establishes
// around its emissions (engine/rules/mana_activation.go). An add with no
// producer fact fails closed -- in particular Caged Sun's own bonus mana is
// added by a triggered ability's body, whose resolution sets no producer, so
// it cannot re-trigger the card.

package trigmatch

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// manaAddedProduced is the Produced$ grammar the corpus carriers print: empty
// (unrestricted), one literal symbol, or ChosenColor (the trigger source's
// own as-enters ChooseColor answer). A declaration whose output is a choice
// (Any, Combo ..., a Special selector) is not knowable at the ManaAdd
// boundary and fails closed.
func manaAddedProduced(e Board, t cards.Trigger, source state.ObjID, added byte) bool {
	// parseParams trims every parameter value at load, so the stored value
	// is read directly.
	want := t.ParamStr(cards.PKProduced)
	if want == "" {
		return true
	}
	if t.ProducedChosenColour() {
		o := e.Game().Obj(source)
		return o != nil && len(o.ChosenColor) == 1 && o.ChosenColor[0] == added
	}
	if len(want) != 1 {
		return false
	}
	return want[0] == added
}

// manaAddedMatches matches one ManaAdd event. The colour an add carries is
// read through state.ManaSlot, the one pool-slot reader for a Counter that
// may carry a producer tag, the snow "S" prefix or their composition.
func manaAddedMatches(e Board, t cards.Trigger, source state.ObjID, ev events.Event, _ *state.Object) bool {
	if ev.Kind != events.ManaAdd {
		return false
	}
	f := e.Facts()
	if f.ManaProducer == 0 {
		// No ability is producing mana right now: nothing "causes" the add
		// in the printed text's sense, so no ManaAdded line may fire.
		return false
	}
	// ValidSA$ names the ability that produced the mana: SpellAbility.ManaAbility
	// (a CR 605.1a mana ability) or its negation (cards.ManaAbilityMark is the
	// mark's one reader). Any other ValidSA$ shape is not a mark and fails
	// closed.
	if v := t.ParamStr(cards.PKValidSA); v != "" {
		present, wantMana := cards.ManaAbilityMark(v)
		if !present || wantMana != f.ManaAddFromAbility {
			return false
		}
	}
	// Player$ names the player the mana was added to (the pool owner).
	if v := t.ParamStr(cards.PKPlayer); v != "" &&
		!effects.MatchesPlayerSpec(e.Game(), v, ev.Player, e.ControllerOf(source)) {
		return false
	}
	// ValidSource$ names the producing ability's source.
	if v := t.ParamStr(cards.PKValidSource); v != "" &&
		!e.MatchesSpec(v, f.ManaProducer, source, e.ControllerOf(source), SpecOpts{}) {
		return false
	}
	slot := state.ManaSlot(ev.Counter)
	if slot < 0 || slot >= len(effects.ManaSymbols) {
		// A counter form that names no pool slot is not "mana of a colour".
		return false
	}
	return manaAddedProduced(e, t, source, cards.ManaSymbol(slot))
}

func init() {
	// Registered through a func literal calling the pack-level matcher so the
	// census can read the callee (registerTrigMatcher takes a method
	// expression or a literal whose first call names the matcher).
	registerTrigMatcher(func(e Board, t cards.Trigger, source state.ObjID, ev events.Event, lki *state.Object) bool {
		return manaAddedMatches(e, t, source, ev, lki)
	}, "ManaAdded")
}
