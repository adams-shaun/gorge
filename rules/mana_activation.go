package rules

import (
	"fmt"
	"strconv"
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
// chooses the colour that Produced$ Any (or Combo Any) will add. triggers is
// the CR 605.3b triggered-mana batch still to resolve after the answer. When
// trigger is non-nil the choice belongs to that triggered mana ability instead
// (ability is then the Mana sub-ability in its chain, and player the player
// receiving the mana).
type manaColorActivation struct {
	player   state.PlayerID
	source   state.ObjID
	ability  *cards.SA
	cast     bool
	triggers []pendingTrigger
	trigger  *pendingTrigger
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
		e.emit(events.DiscardCost(id))
	}
	var manaTriggers []pendingTrigger
	if md.cost.Tap {
		manaTriggers = e.emitManaTap(md.player, md.source, md.ability)
	}
	for _, part := range md.cost.SubCounter {
		e.emit(events.Event{Kind: events.CounterChange, Obj: md.source, Counter: part.Spec, Amount: -part.N})
	}
	for _, id := range md.sacs {
		e.emit(events.Sacrifice(id))
	}
	e.manaDiscardActivation = nil
	e.choosing = chooseNone
	e.resolveManaEffect(md.player, md.source, md.ability, md.cast, manaTriggers)
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

// emitManaTap preserves "tapped for mana" as transient engine context while
// trigger matching runs. Tap's existing event payload stays unchanged, so a
// mana activation without a matching trigger keeps its historic event chain.
func (e *Engine) emitManaTap(p state.PlayerID, source state.ObjID, sa *cards.SA) []pendingTrigger {
	// A TapsForMana trigger can restrict the mana's Produced$ value. Keep the
	// activating SA alongside the synchronous tap marker so matching sees the
	// same output declaration that the activation will resolve. This context is
	// rebuilt by replay because replay takes the same activation path.
	produced := ""
	if sa != nil {
		produced = strings.TrimSpace(sa.Params["Produced"])
	}
	before := len(e.pendingTriggers)
	e.tappingForMana, e.tappingManaProduced = source, produced
	e.emitTap(source, p, false)
	e.tappingForMana, e.tappingManaProduced = 0, ""

	// CR 605.3b: a triggered mana ability resolves immediately after the mana
	// ability that caused it, without using the stack. Separate only newly
	// matched triggers from this Tap event; older pending triggers and ordinary
	// tap reactions such as Manabarbs remain in the normal APNAP queue.
	matched := e.pendingTriggers[before:]
	kept := e.pendingTriggers[:before]
	var immediate []pendingTrigger
	for _, pt := range matched {
		if e.isTriggeredManaAbility(pt) {
			immediate = append(immediate, pt)
			continue
		}
		kept = append(kept, pt)
	}
	e.pendingTriggers = kept
	return immediate
}

// isTriggeredManaAbility recognises Forge's Static$ True marker for a
// TapsForMana trigger. Forge uses that marker for CR 605.1b triggered mana
// abilities; checking the real trigger by source/index avoids treating every
// TapsForMana reaction (notably Manabarbs) as immediate. A target anywhere in
// the linked effect chain keeps the trigger on the stack, as CR 605.1b
// requires a mana ability not to require a target.
func (e *Engine) isTriggeredManaAbility(pt pendingTrigger) bool {
	o := e.G.Obj(pt.Source)
	if o == nil || o.Face() == nil || pt.Idx < 0 || pt.Idx >= len(o.Face().Triggers) {
		return false
	}
	t := o.Face().Triggers[pt.Idx]
	if t.Mode != "TapsForMana" || !strings.EqualFold(t.Params["Static"], "True") {
		return false
	}
	for sa := pt.SA; sa != nil; sa = sa.Sub {
		if strings.TrimSpace(sa.Params["ValidTgts"]) != "" {
			return false
		}
	}
	return pt.SA != nil
}

// resolveTriggeredManaAbilities executes the CR 605.3b batch directly after
// the activated mana ability has resolved. These abilities never mint stack
// objects; their ordinary effect events are enough for deterministic replay.
//
// A triggered mana ability whose mana is a colour choice -- Produced$ Any or
// Combo Any (Fertile Ground, Regal Behemoth, Market Festival) or a Combo
// colour list -- is not resolved as colourless: the rest of the batch is
// parked and the player receiving the mana chooses, exactly as the activated
// path's askManaColor asks. The answer resolves that ability with the chosen
// colour and continues the batch (answerManaColor). cast is the payment
// window flag the parked activation carries back to the caller.
func (e *Engine) resolveTriggeredManaAbilities(triggers []pendingTrigger, cast bool) {
	for i := range triggers {
		pt := triggers[i]
		if int(pt.Controller) >= len(e.G.Players) || e.G.Players[pt.Controller].Lost {
			continue
		}
		if src := e.G.Obj(pt.Source); src != nil && src.Face() != nil {
			effects.SetSVars(&pt.Ctx, src.Face().SVars)
		}
		if e.askTriggeredManaColor(pt, triggers[i+1:], cast) {
			return
		}
		effects.Resolve(e, &pt.Ctx, pt.SA)
	}
}

// askTriggeredManaColor poses the colour choice for the first colour-choice
// Mana sub-ability in pt's chain, parking pt and the rest of its batch, and
// reports whether it asked. The chooser is the first player the Mana
// sub-ability adds mana for (effects.ManaRecipients, which reads Defined$):
// Fertile Ground on an opponent's land asks that land's controller.
func (e *Engine) askTriggeredManaColor(pt pendingTrigger, rest []pendingTrigger, cast bool) bool {
	mana, colours := triggeredManaColourChoice(pt.SA)
	if mana == nil {
		return false
	}
	chooser := pt.Controller
	if ps := effects.ManaRecipients(e, &pt.Ctx, mana); len(ps) > 0 {
		chooser = ps[0]
	}
	d := &decision.Decision{Player: chooser, Kind: decision.KChoose, Min: 1, Max: 1,
		Prompt: manaColourPrompt(mana), Source: pt.Source}
	for i, color := range colours {
		d.Options = append(d.Options, decision.Option{Index: i, Kind: "mana", Obj: pt.Source, Label: "Add " + color})
	}
	parked := pt
	e.manaColorActivation = &manaColorActivation{player: chooser, source: pt.Source, ability: mana,
		cast: cast, triggers: rest, trigger: &parked}
	e.choosing = chooseManaColor
	e.ask(d)
	return true
}

// triggeredManaColourChoice finds the first Mana sub-ability in a triggered
// ability's chain whose Produced$ is a colour choice, with the colours it
// offers. nil when the chain adds only fixed mana (or none).
func triggeredManaColourChoice(sa *cards.SA) (*cards.SA, []string) {
	for d := 0; sa != nil && d < 32; d, sa = d+1, sa.Sub {
		if sa.API != "Mana" {
			continue
		}
		produced := strings.TrimSpace(sa.Params["Produced"])
		if produced == "Any" || produced == "Combo Any" {
			return sa, []string{"W", "U", "B", "R", "G"}
		}
		if colours, ok := effects.ComboColours(produced); ok {
			return sa, colours
		}
	}
	return nil, nil
}

// withProduced copies the chain from head down to target, with target's
// Produced$ rewritten to the chosen colour. Corpus SAs are shared immutable
// data, so the rewrite never touches them.
func withProduced(head, target *cards.SA, produced string) *cards.SA {
	if head == nil {
		return nil
	}
	cp := *head
	if head == target {
		cp.Params = make(map[string]string, len(head.Params))
		for k, v := range head.Params {
			cp.Params[k] = v
		}
		cp.Params["Produced"] = produced
		return &cp
	}
	cp.Sub = withProduced(head.Sub, target, produced)
	return &cp
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
	var manaTriggers []pendingTrigger
	if cost.Tap {
		manaTriggers = e.emitManaTap(p, source, ma)
	}
	for _, part := range cost.SubCounter {
		e.emit(events.Event{Kind: events.CounterChange, Obj: source, Counter: part.Spec, Amount: -part.N})
	}
	for _, id := range sacs {
		e.emit(events.Sacrifice(id))
	}
	e.resolveManaEffect(p, source, ma, cast, manaTriggers)
}

func (e *Engine) resolveManaEffect(p state.PlayerID, source state.ObjID, ma *cards.SA, cast bool, triggers []pendingTrigger) {
	produced := strings.TrimSpace(ma.Params["Produced"])
	if produced == "Any" || produced == "Combo Any" {
		e.askManaColor(p, source, ma, cast, triggers, []string{"W", "U", "B", "R", "G"})
		return
	}
	// A "Combo <colours>" shape is "add one of these", not "add each of
	// these": it asks, but restricted to exactly the colours it names --
	// "Combo R G" offers R and G, never the other three. The classifier
	// (effects.ComboColours) rejects "Combo Any" (kept on the five-colour
	// branch above) and every combo it cannot resolve to a plain colour list,
	// which then falls to resolveManaEffectColor and fails closed in effMana.
	if colours, ok := effects.ComboColours(produced); ok {
		e.askManaColor(p, source, ma, cast, triggers, colours)
		return
	}
	e.resolveManaEffectColor(p, source, ma, produced)
	e.resolveTriggeredManaAbilities(triggers, cast)
}

// askManaColor poses the colour choice for a Produced value that names a
// fixed set (the five colours for Any/Combo Any, or the named colours of a
// "Combo <colours>" shape) and pauses until it is answered.
func (e *Engine) askManaColor(p state.PlayerID, source state.ObjID, ma *cards.SA, cast bool, triggers []pendingTrigger, colours []string) {
	d := &decision.Decision{Player: p, Kind: decision.KChoose, Min: 1, Max: 1,
		Prompt: manaColourPrompt(ma), Source: source}
	for i, color := range colours {
		d.Options = append(d.Options, decision.Option{Index: i, Kind: "mana", Obj: source, Label: "Add " + color})
	}
	e.manaColorActivation = &manaColorActivation{player: p, source: source, ability: ma, cast: cast, triggers: triggers}
	e.choosing = chooseManaColor
	e.ask(d)
}

// manaColourPrompt names the amount of mana the ability adds when the script
// carries an EXPLICIT, positive literal Amount$, so a player choosing the
// colour of "Add three mana of any one color" (Lion's Eye Diamond) sees the
// whole deal instead of a bare "Choose a colour of mana" that reads like the
// card only offered one colour of mana. Every other shape keeps the generic
// prompt: an absent Amount$ (the prompt must not invent "1" for the pool that
// effMana will actually resolve — the brief's boundary is explicit-literal
// only), a non-literal amount (X, Y, an SVar or inline Count$ expression) the
// ask site cannot price, and a non-positive literal — a wrong number in the
// prompt is worse than no number. The "any one color" wording is kept
// verbatim from the oracle shape (Produced$ Any / Combo Any, CR 107.4) so the
// player sees that the ONE choice covers all of the mana; a restricted
// "Combo <colours>" shape is not "any" colour, so its prompt only names the
// amount.
func manaColourPrompt(ma *cards.SA) string {
	generic := "Choose a colour of mana"
	raw, ok := ma.Params["Amount"]
	if !ok {
		// No Amount$ param: stay generic — the prompt must not invent an
		// amount the script never stated.
		return generic
	}
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n <= 0 {
		// A non-literal amount (X, Y, an SVar or inline Count$ expression)
		// or a non-positive literal: the ask site cannot price it, and a
		// wrong number in the prompt is worse than no number.
		return generic
	}
	switch strings.TrimSpace(ma.Params["Produced"]) {
	case "Any", "Combo Any":
		return fmt.Sprintf("Add %d mana of any one color — choose the colour", n)
	default:
		return fmt.Sprintf("Add %d mana — choose the colour", n)
	}
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
	if ma.trigger != nil {
		pt := *ma.trigger
		pt.SA = withProduced(pt.SA, ma.ability, color)
		// A later colour-choice Mana sub-ability in the same chain asks in
		// turn; otherwise the ability resolves and the batch continues.
		if e.askTriggeredManaColor(pt, ma.triggers, ma.cast) {
			return ma.cast
		}
		effects.Resolve(e, &pt.Ctx, pt.SA)
		e.resolveTriggeredManaAbilities(ma.triggers, ma.cast)
		return ma.cast
	}
	e.resolveManaEffectColor(ma.player, ma.source, ma.ability, color)
	e.resolveTriggeredManaAbilities(ma.triggers, ma.cast)
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
