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
	chooseManaDiscard
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

// manaDiscardActivation holds a synchronous mana ability while its discard
// cost is chosen. It is separate from pendingCast so activating mana during a
// spell's CR 601.2g payment window never overwrites the outer cast flow.
type manaDiscardActivation struct {
	player   state.PlayerID
	source   state.ObjID
	ability  *cards.SA
	cost     Cost
	sacs     []state.ObjID
	discards []state.ObjID
	part     int
	cast     bool
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
		e.resolveManaAbility(p, source, abilities[0], cast)
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
// can pay it without a chooser. Discard costs have their own continuation:
// ordinary discard asks, while random and discard-your-hand do not.
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
	if _, ok := e.manaSacrifices(p, source, cost); !ok {
		return false
	}
	_, ok := e.manaDiscards(p, source, cost)
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

// manaDiscards performs the pure offer-side feasibility walk for a mana
// ability's discard cost. It reserves deterministic candidates but consumes
// no RNG; the payment continuation makes the actual choice.
func (e *Engine) manaDiscards(p state.PlayerID, source state.ObjID, cost Cost) ([]state.ObjID, bool) {
	var discards []state.ObjID
	reserved := map[state.ObjID]bool{}
	for _, part := range cost.Discard {
		candidates := e.discardCandidates(p, source, part, false, reserved)
		if strings.EqualFold(part.Spec, "Hand") {
			for _, id := range candidates {
				reserved[id] = true
				discards = append(discards, id)
			}
			continue
		}
		n := int(part.N)
		if n <= 0 || n > len(candidates) {
			return nil, false
		}
		for i := 0; i < n; i++ {
			id := candidates[0]
			reserved[id] = true
			discards = append(discards, id)
			candidates = candidates[1:]
		}
	}
	return discards, true
}

// continueManaDiscard walks a mana ability's discard parts without putting
// the ability on the stack. Ordinary parts ask their controller; Random and
// Hand select internally using the same rules as discardAsk.
func (e *Engine) continueManaDiscard() {
	md := e.manaDiscardActivation
	if md == nil {
		return
	}
	for md.part < len(md.cost.Discard) {
		part := md.cost.Discard[md.part]
		reserved := make(map[state.ObjID]bool, len(md.discards))
		for _, id := range md.discards {
			reserved[id] = true
		}
		candidates := e.discardCandidates(md.player, md.source, part, false, reserved)
		if strings.EqualFold(part.Spec, "Hand") {
			md.discards = append(md.discards, candidates...)
			md.part++
			continue
		}
		n := int(part.N)
		if n <= 0 || n > len(candidates) {
			e.manaDiscardActivation = nil
			e.choosing = chooseNone
			return
		}
		if strings.EqualFold(part.Spec, "Random") {
			for i := 0; i < n; i++ {
				pick := e.Rand(len(candidates))
				md.discards = append(md.discards, candidates[pick])
				candidates = append(candidates[:pick], candidates[pick+1:]...)
			}
			md.part++
			continue
		}
		d := &decision.Decision{Player: md.player, Kind: decision.KChoose, Min: n, Max: n,
			Prompt: "Discard a card to pay the mana ability cost", Source: md.source}
		for _, id := range candidates {
			d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "mana_discard",
				Obj: id, Label: e.G.Obj(id).Face().Name})
		}
		e.choosing = chooseManaDiscard
		e.ask(d)
		return
	}
	e.commitManaDiscard()
}

func (e *Engine) commitManaDiscard() {
	md := e.manaDiscardActivation
	if md == nil || !e.payMana(md.player, md.cost) {
		e.manaDiscardActivation = nil
		e.choosing = chooseNone
		return
	}
	for _, id := range md.discards {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZGraveyard, Text: "discarded as a cost"})
	}
	if md.cost.Tap {
		e.emit(events.Event{Kind: events.Tap, Obj: md.source})
	}
	for _, part := range md.cost.SubCounter {
		e.emit(events.Event{Kind: events.CounterChange, Obj: md.source, Counter: part.Spec, Amount: -part.N})
	}
	for _, id := range md.sacs {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZGraveyard, Text: "sacrificed"})
	}
	e.manaDiscardActivation = nil
	e.choosing = chooseNone
	e.resolveManaEffect(md.player, md.source, md.ability, md.cast)
}

// answerManaDiscard records one ordinary discard part and continues payment.
// It reports whether this mana activation belongs to an outer cast window.
func (e *Engine) answerManaDiscard(chosen []decision.Option) bool {
	md := e.manaDiscardActivation
	if md == nil {
		return false
	}
	for _, opt := range chosen {
		md.discards = append(md.discards, opt.Obj)
	}
	md.part++
	cast := md.cast
	e.continueManaDiscard()
	return cast
}

// resolveManaAbility pays this ability's actual activation cost, then resolves
// it outside the stack. In particular, Sac and Discard costs are emitted
// before the mana effect, and no phantom generic mana is charged.
func (e *Engine) resolveManaAbility(p state.PlayerID, source state.ObjID, ma *cards.SA, cast bool) {
	if !e.manaAbilityPayable(p, source, ma) {
		return
	}
	cost := ParseCost(ma.Params["Cost"])
	sacs, _ := e.manaSacrifices(p, source, cost)
	if len(cost.Discard) > 0 {
		e.manaDiscardActivation = &manaDiscardActivation{player: p, source: source,
			ability: ma, cost: cost, sacs: sacs, cast: cast}
		e.continueManaDiscard()
		return
	}
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
	e.resolveManaEffect(p, source, ma, cast)
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
		e.resolveManaAbility(ma.player, ma.source, ma.abilities[idx], ma.cast)
	}
	return ma.cast
}
