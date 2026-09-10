package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// manaActivation is the one outstanding choice among a permanent's distinct
// mana abilities. It is plain data so Clone can preserve a choice at an
// intent boundary. cast says the answer resumes CR 601.2g payment rather
// than the ordinary priority round.
// chooseMana is the one-off pick among a source's distinct mana abilities.
// The existing chooseFor values occupy 0 through 5; it resumes either
// priority or a cast-time mana window.
const chooseMana chooseFor = iota + 6

type manaActivation struct {
	player    state.PlayerID
	source    state.ObjID
	abilities []*cards.SA
	cast      bool
}

// availableManaAbilities returns exactly the individual mana abilities that
// p may activate from id now. Keeping the CantBeActivated gate here makes the
// priority action, payment window, and the eventual chosen activation share
// one member-by-member eligibility set.
func (e *Engine) availableManaAbilities(p state.PlayerID, id state.ObjID) []*cards.SA {
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil || o.Tapped {
		return nil
	}
	var out []*cards.SA
	for _, ma := range o.Face().ManaAbilities() {
		if !e.abilityRestricted(p, id, ma) {
			out = append(out, ma)
		}
	}
	return out
}

// activateMana activates one of source's currently available mana abilities.
// A singleton retains the old no-extra-decision path. Several abilities are
// distinct activated abilities sharing one tap cost, so their controller must
// choose one before the source is tapped.
func (e *Engine) activateMana(p state.PlayerID, source state.ObjID, cast bool) {
	abilities := e.availableManaAbilities(p, source)
	if len(abilities) == 0 {
		return
	}
	if len(abilities) == 1 {
		e.resolveManaAbility(p, source, abilities[0])
		return
	}
	o := e.G.Obj(source)
	d := &decision.Decision{Player: p, Kind: decision.KChoose, Min: 1, Max: 1,
		Prompt: "Choose a mana ability of " + o.Face().Name, Source: source}
	for i, ma := range abilities {
		d.Options = append(d.Options, decision.Option{Index: i, Kind: "mana",
			Ability: i, Label: "Add " + strings.TrimSpace(ma.Params["Produced"])})
	}
	e.manaActivation = &manaActivation{player: p, source: source, abilities: abilities, cast: cast}
	e.choosing = chooseMana
	e.ask(d)
}

// resolveManaAbility pays the shared tap cost once and resolves exactly the
// selected ability. Mana abilities do not use the stack, so this completes
// before either priority resumes or the cast payment window is re-priced.
func (e *Engine) resolveManaAbility(p state.PlayerID, source state.ObjID, ma *cards.SA) {
	o := e.G.Obj(source)
	if o == nil || o.Face() == nil || o.Tapped {
		return
	}
	e.emit(events.Event{Kind: events.Tap, Obj: source})
	e.resolveAbility(source, p, nil, ma, o.Face().SVars)
}

// answerManaActivation completes a multi-ability choice and returns whether
// it came from the cast-time mana window.
func (e *Engine) answerManaActivation(chosen []decision.Option) bool {
	ma := e.manaActivation
	e.manaActivation = nil
	e.choosing = chooseNone
	if ma == nil || len(chosen) != 1 {
		return false
	}
	idx := chosen[0].Ability
	if idx >= 0 && idx < len(ma.abilities) {
		e.resolveManaAbility(ma.player, ma.source, ma.abilities[idx])
	}
	return ma.cast
}
