package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// chooseMana is the one-off pick among a source's distinct mana abilities.
// The existing chooseFor values occupy 0 through 5.
const (
	chooseMana chooseFor = iota + 6
	chooseManaColor
)

// manaActivation is the one outstanding choice among a permanent's distinct
// mana abilities. It is plain data so Clone can preserve a choice at an
// intent boundary. cast says the answer resumes CR 601.2g payment rather
// than the ordinary priority round.
type manaActivation struct {
	player    state.PlayerID
	source    state.ObjID
	abilities []*cards.SA
	cast      bool
}

// manaColorActivation holds an already-paid mana ability while its controller
// chooses the colour that Produced$ Any (or Combo Any) will add.
type manaColorActivation struct {
	player  state.PlayerID
	source  state.ObjID
	ability *cards.SA
	cast    bool
}

// availableManaAbilities returns exactly the individual mana abilities that
// p may activate from id now. Keeping the CantBeActivated gate here makes the
// priority action, payment window, and the eventual chosen activation share
// one member-by-member eligibility set.
func (e *Engine) availableManaAbilities(p state.PlayerID, id state.ObjID) []*cards.SA {
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil {
		return nil
	}
	var out []*cards.SA
	for _, ma := range o.Face().ManaAbilities() {
		if !e.abilityRestricted(p, id, ma) && e.manaAbilityPayable(p, id, ma) {
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
		d.Options = append(d.Options, decision.Option{Index: i, Kind: "mana", Obj: source,
			Ability: i, Label: "Add " + strings.TrimSpace(ma.Params["Produced"])})
	}
	e.manaActivation = &manaActivation{player: p, source: source, abilities: abilities, cast: cast}
	e.choosing = chooseMana
	e.ask(d)
}

// manaAbilityPayable is the mana-ability equivalent of the cast cost gate.
// A source with a sacrifice cost is not offered unless this synchronous path
// can pay it without a chooser. The corpus's self-sacrifice abilities have
// exactly one matching candidate; a hypothetical choice among several is
// conservatively withheld until mana-ability sacrifice choices are modelled.
func (e *Engine) manaAbilityPayable(p state.PlayerID, source state.ObjID, ma *cards.SA) bool {
	o := e.G.Obj(source)
	if o == nil || o.Face() == nil {
		return false
	}
	cost := ParseCost(ma.Params["Cost"])
	if cost.X != 0 || (cost.Tap && o.Tapped) || !cost.payable(e.G.Players[p].Pool, e.G.Players[p].Life) {
		return false
	}
	for _, part := range cost.SubCounter {
		if o.Counter(part.Spec) < part.N {
			return false
		}
	}
	_, ok := e.manaSacrifices(p, source, cost)
	return ok
}

// manaSacrifices finds the forced sacrifice for each cost part. An exact
// candidate count means no player choice is being hidden: each candidate is
// paid in deterministic battlefield order. More candidates than required are
// intentionally not offered by manaAbilityPayable until a KChoose continuation
// can collect that cost choice.
func (e *Engine) manaSacrifices(p state.PlayerID, source state.ObjID, cost Cost) ([]state.ObjID, bool) {
	var sacs []state.ObjID
	reserved := map[state.ObjID]bool{}
	for _, part := range cost.Sac {
		var candidates []state.ObjID
		for _, id := range e.G.Zone(state.ZBattlefield, p) {
			if !reserved[id] && effects.MatchesSpecFrom(e.G, part.Spec, id, p, source) {
				candidates = append(candidates, id)
			}
		}
		if len(candidates) != int(part.N) {
			return nil, false
		}
		for _, id := range candidates {
			reserved[id] = true
			sacs = append(sacs, id)
		}
	}
	return sacs, true
}

// resolveManaAbility pays this ability's actual activation cost, then resolves
// it outside the stack. In particular, a Sac-only ability neither taps nor
// leaves its sacrifice unpaid.
func (e *Engine) resolveManaAbility(p state.PlayerID, source state.ObjID, ma *cards.SA) {
	if !e.manaAbilityPayable(p, source, ma) {
		return
	}
	cost := ParseCost(ma.Params["Cost"])
	sacs, _ := e.manaSacrifices(p, source, cost)
	if !e.payMana(p, cost) {
		return
	}
	if cost.Tap {
		e.emit(events.Event{Kind: events.Tap, Obj: source})
	}
	for _, part := range cost.SubCounter {
		e.emit(events.Event{Kind: events.CounterChange, Obj: source, Counter: part.Spec, Amount: -part.N})
	}
	for _, id := range sacs {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZGraveyard, Text: "sacrificed"})
	}
	e.resolveManaEffect(p, source, ma, false)
}

func (e *Engine) resolveManaEffect(p state.PlayerID, source state.ObjID, ma *cards.SA, cast bool) {
	produced := strings.TrimSpace(ma.Params["Produced"])
	if produced == "Any" || produced == "Combo Any" {
		d := &decision.Decision{Player: p, Kind: decision.KChoose, Min: 1, Max: 1,
			Prompt: "Choose a colour of mana", Source: source}
		for i, color := range []string{"W", "U", "B", "R", "G"} {
			d.Options = append(d.Options, decision.Option{Index: i, Kind: "mana", Obj: source, Label: "Add " + color})
		}
		e.manaColorActivation = &manaColorActivation{player: p, source: source, ability: ma, cast: cast}
		e.choosing = chooseManaColor
		e.ask(d)
		return
	}
	e.resolveManaEffectColor(p, source, ma, produced)
}

func (e *Engine) resolveManaEffectColor(p state.PlayerID, source state.ObjID, ma *cards.SA, produced string) {
	o := e.G.Obj(source)
	if o == nil || o.Face() == nil {
		return
	}
	copy := *ma
	copy.Params = make(map[string]string, len(ma.Params)+1)
	for k, v := range ma.Params {
		copy.Params[k] = v
	}
	copy.Params["Produced"] = produced
	e.resolveAbility(source, p, nil, &copy, o.Face().SVars)
}

// answerManaColor completes a Produced$ Any choice after the activation cost
// has already been paid. It returns whether the suspended activation belonged
// to a cast-time mana window.
func (e *Engine) answerManaColor(chosen []decision.Option) bool {
	ma := e.manaColorActivation
	e.manaColorActivation = nil
	e.choosing = chooseNone
	if ma == nil || len(chosen) != 1 {
		return false
	}
	color := strings.TrimPrefix(chosen[0].Label, "Add ")
	if len(color) != 1 || !strings.Contains("WUBRG", color) {
		return ma.cast
	}
	e.resolveManaEffectColor(ma.player, ma.source, ma.ability, color)
	return ma.cast
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
