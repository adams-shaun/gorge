package effects

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func init() {
	Register("ChooseCard", effChooseCard)
	Register("ChoosePlayer", effChoosePlayer)
	Register("GainControl", effGainControl)
	Register("ControlSpell", effControlSpell)
	Register("ChangeTargets", effChangeTargets)
	Register("RepeatEach", effRepeatEach)
	Register("Branch", effBranch)
}

// choiceBounds reads Forge's shared Amount$/MinAmount$/Mandatory$ vocabulary.
// ChooseCard is optional unless Mandatory$ True says otherwise; ChoosePlayer
// has no Mandatory$ vocabulary in the corpus and requires its choice by
// default. MinAmount$ is the explicit lower bound for either primitive.
func choiceBounds(h Host, c *Ctx, sa *cards.SA, cardChoice bool) (int, int) {
	max := int(Num(h, c, sa, "Amount", 1))
	if max < 0 {
		max = 0
	}
	min := max
	if cardChoice && !strings.EqualFold(sa.Params["Mandatory"], "True") {
		min = 0
	}
	if _, ok := sa.Params["MinAmount"]; ok {
		min = int(Num(h, c, sa, "MinAmount", 0))
	}
	if strings.EqualFold(sa.Params["Mandatory"], "False") || strings.EqualFold(sa.Params["Optional"], "True") {
		min = 0
	}
	if min < 0 {
		min = 0
	}
	if min > max {
		min = max
	}
	return min, max
}

func choiceZones(sa *cards.SA) map[state.Zone]bool {
	if strings.EqualFold(sa.Params["AllCards"], "True") {
		return nil
	}
	s := strings.TrimSpace(sa.Params["ChoiceZone"])
	if s == "" {
		return map[state.Zone]bool{state.ZBattlefield: true}
	}
	out := map[state.Zone]bool{}
	for _, z := range strings.Split(s, ",") {
		switch strings.TrimSpace(z) {
		case "Battlefield":
			out[state.ZBattlefield] = true
		case "Hand":
			out[state.ZHand] = true
		case "Library":
			out[state.ZLibrary] = true
		case "Graveyard":
			out[state.ZGraveyard] = true
		case "Exile":
			out[state.ZExile] = true
		case "Stack":
			out[state.ZStack] = true
		}
	}
	return out
}

// definedCardPool resolves the object-set role carried by DefinedCards$.
// Unlike Defined(), an unknown role must not fall back to the resolution's
// targets: that would widen a constrained choice to unrelated objects.
func definedCardPool(g *state.Game, c *Ctx, raw string) ([]state.Target, string) {
	root, qualifier, _ := strings.Cut(strings.TrimSpace(raw), ".")
	switch root {
	case "Targeted", "TargetedCard", "ParentTargeted":
		return objectsOf(c.Targets), qualifier
	case "Remembered", "RememberedLKI":
		return objectsOf(c.Remembered), qualifier
	case "TriggeredCards", "TriggeredAttackers", "TriggeredBlockers":
		return objectsOf(c.Remembered), qualifier
	case "TriggeredSources":
		if c.TriggerSource != 0 {
			return []state.Target{{Obj: c.TriggerSource}}, qualifier
		}
		return nil, qualifier
	case "ExiledWith":
		// Forge's hostCard.getExiledCards is the source's ChangeZone exile
		// association, not ImprintCards$ and not every card in the shared exile
		// zone. The list is event-backed by Imprint's "exiled-with"
		// discriminator and cardChoices still intersects ChoiceZone$.
		if o := g.Obj(c.Source); o != nil {
			out := make([]state.Target, 0, len(o.ExiledCards))
			for _, id := range o.ExiledCards {
				out = append(out, state.Target{Obj: id})
			}
			return out, qualifier
		}
		return nil, qualifier
	default:
		return nil, qualifier
	}
}

func definedCardQualifierMatches(g *state.Game, c *Ctx, qualifier string, o *state.Object) bool {
	if qualifier == "" {
		return true
	}
	if qualifier == "ControlledBy ChosenPlayer" {
		for _, t := range c.Chosen {
			if t.IsPlayer && t.Player == o.Controller {
				return true
			}
		}
		return false
	}
	return choiceMatches(g, c, "Card."+qualifier, o)
}

func cardChoices(h Host, c *Ctx, sa *cards.SA, chooser state.PlayerID) []state.Target {
	g, spec := h.Game(), sa.Params["Choices"]
	var candidates []state.Target
	zones := choiceZones(sa)
	if raw := strings.TrimSpace(sa.Params["DefinedCards"]); raw != "" {
		var qualifier string
		candidates, qualifier = definedCardPool(g, c, raw)
		// A DefinedCards$ set already supplies its zone. ChoiceZone$, when
		// present, remains an additional restriction on that set.
		if _, explicit := sa.Params["ChoiceZone"]; !explicit {
			zones = nil
		}
		out := candidates[:0]
		for _, t := range candidates {
			o := g.Obj(t.Obj)
			if o != nil && definedCardQualifierMatches(g, c, qualifier, o) {
				out = append(out, t)
			}
		}
		candidates = out
	} else {
		for i := range g.Objs {
			candidates = append(candidates, state.Target{Obj: g.Objs[i].ID})
		}
	}

	var out []state.Target
	for _, t := range candidates {
		o := g.Obj(t.Obj)
		if o == nil || (zones != nil && !zones[o.Zone]) {
			continue
		}
		if !controlledByChoicePlayer(g, c, sa.Params["ControlledByPlayer"], chooser, o) {
			continue
		}
		// The choice's filter is evaluated from the chooser's perspective:
		// `Choices$ Card.YouOwn` means the chooser's card, not the spell's
		// controller's card.
		cc := *c
		cc.Controller = chooser
		if spec == "" || choiceMatches(g, &cc, spec, o) {
			out = append(out, t)
		}
	}
	return out
}

// controlledByChoicePlayer applies ChooseCard's ControlledByPlayer$, the
// player whose objects the chooser picks among. Corpus values: Chooser 30,
// Remembered 4 (the RepeatEach subject: Winnowing, Tragic Arrogance), Left 2,
// Right 1 (Juggle the Performance), You 1. Left is the next living player in
// turn order and Right the previous one. An unrecognised value offers
// nothing rather than every object in the zone.
func controlledByChoicePlayer(g *state.Game, c *Ctx, v string, chooser state.PlayerID, o *state.Object) bool {
	switch strings.TrimSpace(v) {
	case "":
		return true
	case "Chooser":
		return o.Controller == chooser
	case "You":
		return o.Controller == c.Controller
	case "Remembered":
		return targetIn(c.Remembered, state.Target{Player: o.Controller, IsPlayer: true})
	case "Left":
		return o.Controller == g.NextAlive(chooser)
	case "Right":
		alive := g.AliveFrom(chooser)
		return len(alive) > 0 && o.Controller == alive[len(alive)-1]
	}
	return false
}

func randomChoices(h Host, choices []state.Target, n int) []state.Target {
	pool := append([]state.Target(nil), choices...)
	if n > len(pool) {
		n = len(pool)
	}
	picked := make([]state.Target, 0, n)
	for len(picked) < n {
		i := h.Rand(len(pool))
		picked = append(picked, pool[i])
		pool = append(pool[:i], pool[i+1:]...)
	}
	return picked
}

// choiceMatches is the ordinary object matcher evaluated from the chooser's
// perspective: the caller overrides Ctx.Controller to the chooser, so
// Choices$ Card.YouOwn means the chooser's card, not the spell's controller's.
// IsRemembered needs no resolution-local special case any more -- the general
// filter implements it against the resolution's Remembered set (plus the
// source's event-backed list), which is exactly the binding a choice's
// "a card you remembered earlier" filter wants.
func choiceMatches(g *state.Game, c *Ctx, spec string, o *state.Object) bool {
	return MatchesObjectCtx(g, spec, o, c.SpecContext(c.Controller))
}

func choiceChoosers(h Host, c *Ctx, sa *cards.SA) []state.PlayerID {
	seen := map[state.PlayerID]bool{}
	var out []state.PlayerID
	for _, t := range Defined(h, c, sa) {
		p := PlayerOf(h, c, t)
		if !seen[p] && int(p) < len(h.Game().Players) && !h.Game().Players[p].Lost {
			seen[p] = true
			out = append(out, p)
		}
	}
	if len(out) == 0 && sa.Params["Defined"] == "" {
		return []state.PlayerID{c.Controller}
	}
	return out
}

func choiceRecord(h Host, c *Ctx, sa *cards.SA, picked []state.Target, playerChoice bool) {
	c.Choice = append([]state.Target(nil), picked...)
	if playerChoice {
		// Forge's ChoosePlayerEffect calls host.setChosenPlayer(chosen) once
		// per chooser: a single player field, last chooser wins, and the card
		// entries an earlier ChooseCard chose are untouched (its separate
		// field). Drop the old player entries, keep the card entries.
		c.Chosen = append(keepChosenCards(c.Chosen), picked...)
	} else {
		c.Chosen = append(c.Chosen, picked...)
	}
	c.ChosenValid = true
	if strings.EqualFold(sa.Params["RememberChosen"], "True") {
		c.Remembered = append(c.Remembered, picked...)
	}
	if c.Source == 0 {
		return
	}
	ids := make([]state.ObjID, 0, len(picked))
	for _, t := range picked {
		if t.IsPlayer {
			ids = append(ids, state.PlayerRef(t.Player))
		} else {
			ids = append(ids, t.Obj)
		}
	}
	h.Emit(events.Event{Kind: events.Choose, Obj: c.Source, Counter: "chosen", IDs: ids})
	if strings.EqualFold(sa.Params["RememberChosen"], "True") {
		h.Emit(events.Event{Kind: events.Choose, Obj: c.Source, Counter: "remembered", IDs: ids})
	}
}

// keepChosenPlayers returns only player entries, the half a ChooseCard keeps.
func keepChosenPlayers(ts []state.Target) []state.Target {
	var out []state.Target
	for _, t := range ts {
		if t.IsPlayer {
			out = append(out, t)
		}
	}
	return out
}

// keepChosenCards returns only object entries, the separate chosen-cards
// field Forge leaves untouched when ChoosePlayer replaces its chosen player.
func keepChosenCards(ts []state.Target) []state.Target {
	var out []state.Target
	for _, t := range ts {
		if !t.IsPlayer {
			out = append(out, t)
		}
	}
	return out
}

func effChooseCard(h Host, c *Ctx, sa *cards.SA) {
	choosers := choiceChoosers(h, c, sa)
	// Reveal$ True (Planetary Annihilation's "each player chooses six lands
	// they keep" is public knowledge — CR 701.x's open choice): each chooser's
	// ANSWERED choice is revealed to every seat with the same ids-Note
	// effReveal's public reveal emits (empty Text, view.Describe renders
	// "player N reveals ...", RedactEvents passes it through unchanged). The
	// reveal fires per chooser as their choice is recorded — both on the
	// answered re-entry and on the no-host fallback below — so every seat
	// learns the kept set before the next chooser picks. Player entries
	// (a ChoosePlayer follow-up) reveal nothing: a player is not hidden.
	reveal := strings.EqualFold(strings.TrimSpace(sa.Params["Reveal"]), "True")
	i := c.ChoiceTarget
	if c.ChoiceDone {
		answered := c.Choice
		choiceRecord(h, c, sa, c.Choice, false)
		c.ChoiceDone, c.Choice = false, nil
		// c.ChoiceTarget is the asking chooser's index, so choosers[i] is who
		// answered this.
		if reveal && i < len(choosers) {
			emitChosenReveal(h, choosers[i], answered)
		}
		i++
	} else if i == 0 && c.Choice == nil {
		// Fresh entry: Forge's ChooseCardEffect ends in host.setChosenCards(allChosen)
		// -- the union across THIS SA's choosers REPLACING the cards a previous
		// choice SA left. Forge's chosen player is a separate field that
		// setChosenCards does not touch, so only the object entries are reset
		// here -- the bug this closes is the same KIND accumulating across SAs
		// (a second ChooseCard's Defined$ ChosenCard follow-up saw the first
		// SA's cards too). The union across this SA's choosers is
		// choiceRecord's append (Forge accumulates allChosen the same way
		// inside one SA).
		c.Chosen = keepChosenPlayers(c.Chosen)
	}
	minBase, maxBase := choiceBounds(h, c, sa, true)
	for ; i < len(choosers); i++ {
		choices := cardChoices(h, c, sa, choosers[i])
		min, max := minBase, maxBase
		if max > len(choices) {
			max = len(choices)
		}
		if min > max {
			min = max
		}
		if strings.EqualFold(sa.Params["AtRandom"], "True") {
			choiceRecord(h, c, sa, randomChoices(h, choices, max), false)
			continue
		}
		d := &decision.Decision{Player: choosers[i], Kind: decision.KChoose, Source: c.Source, Min: min, Max: max, ResumeKind: "choice", ResumeSA: sa, ResumeTarget: i, ResumeChoices: append([]state.Target(nil), c.Chosen...), ResumeChosenValid: c.ChosenValid, ResumeRemembered: append([]state.Target(nil), c.Remembered...), Prompt: sa.Params["ChoiceTitle"]}
		for j, t := range choices {
			d.Options = append(d.Options, decision.Option{Index: j, Kind: "card", Obj: t.Obj, Player: choosers[i]})
		}
		if d.Prompt == "" {
			d.Prompt = "Choose card"
		}
		if Ask(h, d) == AskAsked {
			return
		}
		recorded := choices[:min]
		choiceRecord(h, c, sa, recorded, false)
		if reveal {
			emitChosenReveal(h, choosers[i], recorded)
		}
	}
}

// emitChosenReveal is ChooseCard's Reveal$ True emission: one public ids-Note
// naming the chosen CARDS (player entries are skipped — a chosen player is
// not hidden information), the same payload shape effReveal's public reveal
// uses so every seat's transcript reads "player N reveals ...".
func emitChosenReveal(h Host, chooser state.PlayerID, picked []state.Target) {
	var ids []state.ObjID
	for _, t := range picked {
		if !t.IsPlayer && t.Obj != 0 {
			ids = append(ids, t.Obj)
		}
	}
	if len(ids) == 0 {
		return
	}
	h.Emit(events.Event{Kind: events.Note, Player: chooser, IDs: ids})
}

// choosePlayerSpec is the player restriction of a ChoosePlayer. Choices$ is
// the explicit pool; a script without it may still narrow the pool through
// ValidTgts$ (Bill Ferny's "Choose an opponent": ValidTgts$ Opponent). With
// neither, the empty spec means every living player (Valleymaker).
func choosePlayerSpec(sa *cards.SA) string {
	if spec := strings.TrimSpace(sa.Params["Choices"]); spec != "" {
		return spec
	}
	return strings.TrimSpace(sa.Params["ValidTgts"])
}

func effChoosePlayer(h Host, c *Ctx, sa *cards.SA) {
	choosers := choiceChoosers(h, c, sa)
	i := c.ChoiceTarget
	if c.ChoiceDone {
		choiceRecord(h, c, sa, c.Choice, true)
		c.ChoiceDone, c.Choice = false, nil
		i++
	}
	minBase, maxBase := choiceBounds(h, c, sa, false)
	g, spec := h.Game(), choosePlayerSpec(sa)
	// A ChoosePlayer that carries ValidTgts$ chose its player as a target when
	// the ability was put on the stack (Bill Ferny's "target opponent"): the
	// choice is that target, if it is still a matching player.
	var targeted map[state.PlayerID]bool
	if _, ok := sa.Params["ValidTgts"]; ok && sa.Params["Choices"] == "" {
		for _, t := range c.Targets {
			if t.IsPlayer && int(t.Player) < len(g.Players) && MatchesPlayerSpec(g, spec, t.Player, c.Controller) {
				if targeted == nil {
					targeted = map[state.PlayerID]bool{}
				}
				targeted[t.Player] = true
			}
		}
	}
	for ; i < len(choosers); i++ {
		var choices []state.Target
		for _, p := range g.AliveFrom(choosers[i]) {
			if targeted != nil && !targeted[p] {
				continue
			}
			if spec == "" {
				// "Choose a player" with no restriction (Valleymaker): every
				// living player is a legal choice.
			} else if spec == "NonChosenPlayer" {
				seen := false
				for _, t := range c.Remembered {
					if t.IsPlayer && t.Player == p {
						seen = true
						break
					}
				}
				if seen {
					continue
				}
			} else if !MatchesPlayerSpec(g, spec, p, choosers[i]) {
				continue
			}
			choices = append(choices, state.Target{Player: p, IsPlayer: true})
		}
		min, max := minBase, maxBase
		if max > len(choices) {
			max = len(choices)
		}
		if min > max {
			min = max
		}
		if strings.EqualFold(sa.Params["Random"], "True") {
			choiceRecord(h, c, sa, randomChoices(h, choices, max), true)
			continue
		}
		d := &decision.Decision{Player: choosers[i], Kind: decision.KChoose, Source: c.Source, Min: min, Max: max, ResumeKind: "choice", ResumeSA: sa, ResumeTarget: i, ResumeChoices: append([]state.Target(nil), c.Chosen...), ResumeChosenValid: c.ChosenValid, ResumeRemembered: append([]state.Target(nil), c.Remembered...), Prompt: sa.Params["ChoiceTitle"]}
		for j, t := range choices {
			d.Options = append(d.Options, decision.Option{Index: j, Kind: "player", Player: t.Player})
		}
		if d.Prompt == "" {
			d.Prompt = "Choose player"
		}
		if Ask(h, d) == AskAsked {
			return
		}
		choiceRecord(h, c, sa, choices[:min], true)
	}
}

// controlPlayer resolves NewController$, the player who gains control. ok is
// false when the value names nobody this resolution can bind; the control
// change then does not happen (Forge's getDefinedPlayers yields no player),
// rather than silently handing control to the effect's own controller --
// which for "that player gains control of CARDNAME" (Karona, Drooling Ogre,
// Contested War Zone) is a no-op that looks like success.
func controlPlayer(h Host, c *Ctx, sa *cards.SA) (state.PlayerID, bool) {
	g := h.Game()
	v := strings.TrimSpace(sa.Params["NewController"])
	switch v {
	case "", "You", "True":
		return c.Controller, true
	case "ChosenPlayer", "Player.Chosen":
		for _, t := range c.Chosen {
			if t.IsPlayer {
				return t.Player, true
			}
		}
		// A choice made by an earlier, independently resolving ability is
		// event-backed on its source rather than present in this fresh Ctx.
		if o := g.Obj(c.Source); o != nil {
			for _, t := range o.Chosen {
				if t.IsPlayer {
					return t.Player, true
				}
			}
		}
		return 0, false
	case "Player.IsRemembered":
		for _, t := range c.Remembered {
			if t.IsPlayer {
				return t.Player, true
			}
		}
		return 0, false
	case "ImprintedController":
		// Forge's addPlayer(host.getImprintedCards(), "ImprintedController")
		// returns the first imprinted card's current controller.
		if src := g.Obj(c.Source); src != nil {
			for _, id := range src.Imprinted {
				if o := g.Obj(id); o != nil {
					return o.Controller, true
				}
			}
		}
		return 0, false
	case "TriggeredSourceController":
		// DamageDone's source: "that creature's controller".
		if o := g.Obj(c.TriggerSource); o != nil {
			return o.Controller, true
		}
		return 0, false
	case "TriggeredTarget":
		if t := c.TriggerTarget; t.IsPlayer {
			return t.Player, true
		} else if o := g.Obj(t.Obj); o != nil {
			return o.Controller, true
		}
		return 0, false
	}
	// The next seat in turn order (Forge's getNextPlayerAfter -- "the player
	// to your right" is the seat that plays BEFORE you, the last of the
	// alive seats reachable from the controller; "left" is the next one).
	// An unbound form (no other living seat) names nobody. Checked BEFORE the
	// whitelist switch below, whose default would otherwise decline these.
	if v == "NextPlayerToYourRight" || v == "NextPlayerToYourLeft" {
		alive := g.AliveFrom(c.Controller)
		if len(alive) < 2 {
			return 0, false
		}
		if v == "NextPlayerToYourRight" {
			return alive[len(alive)-1], true
		}
		return alive[1], true
	}
	if strings.HasPrefix(v, "Player.withMost") {
		// Forge resolves a Player.<property> defined player by matching the
		// property against every seat and taking the first match in seat
		// order; the withMost* family is implemented in the shared player
		// filter (MatchesPlayerSpecFrom), so this walk cannot disagree with
		// a trigger restriction or attack declaration using the same spec.
		for _, p := range g.AliveFrom(0) {
			if MatchesPlayerSpecFrom(g, v, p, c.Controller, c.Source) {
				return p, true
			}
		}
		return 0, false
	}
	switch v {
	case "Remembered", "RememberedController", "TriggeredPlayer", "TriggeredActivator",
		"TriggeredAttackingPlayer", "TriggeredDefendingPlayer", "TriggeredCardController",
		"Opponent", "Player.Opponent", "Targeted", "TargetedPlayer", "TargetedController", "ParentTarget":
	default:
		// Defined() falls back to the resolution's targets for a form it does
		// not model; that is never a meaningful new controller.
		return 0, false
	}
	ts := Defined(h, c, &cards.SA{Params: map[string]string{"Defined": v}})
	// A player named directly wins over an object's controller ("target
	// player gains control of target creature" lists both targets).
	for _, t := range ts {
		if t.IsPlayer && int(t.Player) < len(g.Players) {
			return t.Player, true
		}
	}
	for _, t := range ts {
		if o := g.Obj(t.Obj); o != nil {
			return o.Controller, true
		}
	}
	return 0, false
}

func effGainControl(h Host, c *Ctx, sa *cards.SA) {
	g := h.Game()
	dur, unknown := ParseControlDuration(sa.Params["LoseControl"])
	if unknown != "" {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Text: "GainControl LoseControl$ " + unknown + " unimplemented"})
		return
	}
	base := ControlGrant{You: c.Controller, Source: c.Source, Duration: dur, SVars: c.SVars,
		AddKeywords: cards.SplitKeywordList(sa.Params["AddKWs"])}
	if src := g.Obj(c.Source); src != nil && src.Zone == state.ZBattlefield {
		base.SourceStamp = src.Timestamp
	}
	if dur.Unattached {
		// "For as long as that Aura is attached to it": the Aura is the
		// attaching object that fired the trigger.
		base.Aura = c.TriggerSource
		if base.Aura == 0 {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Text: "GainControl UntilSourceUnattached has no triggering attachment"})
			return
		}
	}
	if dur.StaticCheck {
		body, ok := c.SVars[sa.Params["StaticCommandCheckSVar"]]
		cmp := strings.TrimSpace(sa.Params["StaticCommandSVarCompare"])
		if !ok || len(cmp) < 3 || !strings.Contains("EQ NE LT LE GT GE", strings.ToUpper(cmp[:2])) {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Text: "GainControl StaticCommandCheck is not evaluable"})
			return
		}
		base.CheckSVar, base.Compare = body, cmp
	}

	var ts []state.Target
	if spec := sa.Params["AllValid"]; spec != "" {
		for i := range g.Objs {
			o := &g.Objs[i]
			if o.Zone == state.ZBattlefield && MatchesObjectCtx(g, spec, o, c.SpecContext(c.Controller)) {
				ts = append(ts, state.Target{Obj: o.ID})
			}
		}
	} else {
		ts = Defined(h, c, sa)
	}
	p, ok := controlPlayer(h, c, sa)
	if !ok {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Text: "GainControl NewController$ " + sa.Params["NewController"] + " names no player"})
		return
	}
	for _, t := range ts {
		if t.IsPlayer {
			continue
		}
		o := g.Obj(t.Obj)
		if o == nil || o.Zone != state.ZBattlefield {
			continue
		}
		gr := base
		gr.Obj, gr.ObjStamp, gr.Previous, gr.Controller = o.ID, o.Timestamp, o.Controller, p
		// CR 611.2b: an effect whose "for as long as" duration has already
		// ended when it would begin does nothing (Vedalken Shackles untapped
		// in response, Kellogg sacrificed in response).
		if ControlGrantEnded(h, gr) {
			continue
		}
		h.Emit(events.Event{Kind: events.ControlChange, Obj: o.ID, Player: p})
		h.RegisterControl(gr)
		if strings.EqualFold(sa.Params["Untap"], "True") {
			h.Emit(events.Event{Kind: events.Untap, Obj: o.ID})
		}
		if strings.EqualFold(sa.Params["RememberControlled"], "True") {
			// Forge (ControlGainEffect): source.addRemembered(tgtC) once per
			// gained permanent -- the persistent host-card list a later
			// Card.IsRemembered spec ("the permanents you gained control of
			// this way", e.g. Ambition's Cost's follow-up or a broker deck's
			// next trigger) matches. Recorded at ctx level for the same walk's
			// SubAbility$ and event-backed on the source for later reads. The
			// two dedupes are per level: the ctx append dedupes against the
			// walk's set, the persistent event dedupes against the source's
			// list ONLY -- a ctx entry that already names the gained object can
			// be the trigger's REFERENT capture (Kain's GainControl of itself:
			// ValidSource$ Card.Self put Kain in ctx.Remembered before this
			// leg ran), which must not suppress the persistent write a later
			// Remembered$ count/condition reads.
			tgt := state.Target{Obj: o.ID}
			if !targetIn(c.Remembered, tgt) {
				c.Remembered = append(c.Remembered, tgt)
			}
			if src := h.Game().Obj(c.Source); src == nil || !targetIn(src.Remembered, tgt) {
				eventRemember(h, c, o.ID)
			}
		}
	}
}
func effControlSpell(h Host, c *Ctx, sa *cards.SA) {
	// Mode$ (Commandeer's "Gain"): what the control transfer targets. "Gain"
	// — the corpus's only value — takes control of the target SPELL on the
	// stack (the ControlChange below is already stack-scoped), which is the
	// behaviour this primitive always had; Forge's ControlSpellEffect reads
	// the same param and branches on it (Gain vs the permanent shapes). An
	// unrecognised value is a loud Note and no transfer, the fail-closed
	// direction — a control change applied to the wrong kind of object is
	// not recoverable.
	mode := strings.TrimSpace(sa.Params["Mode"])
	switch mode {
	case "", "Gain":
	default:
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "unhandled ControlSpell Mode$ " + mode})
		return
	}
	p, ok := controlPlayer(h, c, sa)
	if !ok {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Text: "ControlSpell NewController$ " + sa.Params["NewController"] + " names no player"})
		return
	}
	for _, t := range Defined(h, c, sa) {
		if !t.IsPlayer {
			if o := h.Game().Obj(t.Obj); o != nil && o.Zone == state.ZStack {
				h.Emit(events.Event{Kind: events.ControlChange, Obj: o.ID, Player: p})
			}
		}
	}
}

func effectSA(o *state.Object) *cards.SA {
	if o.Ability != nil {
		return o.Ability
	}
	if f := o.Face(); f != nil {
		return f.SpellAbility()
	}
	return nil
}

func targetIn(ts []state.Target, want state.Target) bool {
	for _, t := range ts {
		if t.IsPlayer == want.IsPlayer && ((t.IsPlayer && t.Player == want.Player) || (!t.IsPlayer && t.Obj == want.Obj)) {
			return true
		}
	}
	return false
}

func targetAllowed(h Host, c *Ctx, restriction string, t state.Target) bool {
	if restriction == "" {
		return true
	}
	if t.IsPlayer {
		return MatchesPlayerSpec(h.Game(), restriction, t.Player, c.Controller)
	}
	return MatchesObjectCtx(h.Game(), restriction, h.Game().Obj(t.Obj), c.SpecContext(c.Controller))
}

func recordTargets(h Host, obj state.ObjID, ts []state.Target) {
	for i, t := range ts {
		amount := int32(0)
		if i > 0 {
			amount = 2
		}
		ev := events.Event{Kind: events.TargetsChosen, Obj: obj, Amount: amount}
		if t.IsPlayer {
			ev.Player = t.Player
			ev.Amount++
		} else {
			ev.IDs = []state.ObjID{t.Obj}
		}
		h.Emit(ev)
	}
}

func changeTargetChooser(h Host, c *Ctx, sa *cards.SA) state.PlayerID {
	v := strings.TrimSpace(sa.Params["Chooser"])
	if v == "" || v == "You" {
		return c.Controller
	}
	for _, t := range Defined(h, c, &cards.SA{Params: map[string]string{"Defined": v}}) {
		return PlayerOf(h, c, t)
	}
	return c.Controller
}

// effChangeTargets derives candidates through Host.LegalTargets, the exact
// rules-side target census used at announcement. The redirect's own
// TargetRestriction narrows that set; it can never widen the subject spell's
// legality. Fixed magnets and random redirects use the same intersection.
func effChangeTargets(h Host, c *Ctx, sa *cards.SA) {
	var target *state.Object
	for _, t := range Defined(h, c, sa) {
		if !t.IsPlayer {
			if o := h.Game().Obj(t.Obj); o != nil && o.Zone == state.ZStack {
				target = o
				break
			}
		}
	}
	if target == nil {
		return
	}
	if c.ChoiceDone {
		if len(c.Choice) > 0 { // an empty Optional answer means keep every target
			out := append([]state.Target(nil), c.Choice...)
			if strings.EqualFold(sa.Params["ChangeSingleTarget"], "True") || len(out) < len(target.Targets) {
				out = append(out, target.Targets[len(out):]...)
			}
			recordTargets(h, target.ID, out)
		}
		c.ChoiceDone = false
		c.Choice = nil
		return
	}
	subject := effectSA(target)
	if subject == nil || len(target.Targets) == 0 {
		return
	}
	chooser := changeTargetChooser(h, c, sa)
	restriction := strings.TrimSpace(sa.Params["TargetRestriction"])
	if strings.EqualFold(sa.Params["RandomTarget"], "True") && sa.Params["RandomTargetRestriction"] != "" {
		restriction = sa.Params["RandomTargetRestriction"]
	}
	var candidates []state.Target
	// CR 115.7: a changed target must be one the spell or ability could
	// legally target, which is decided from its own controller's side
	// ("target creature an opponent controls" names the redirected spell's
	// opponents, not the redirecting player's).
	for _, t := range h.LegalTargets(target.Controller, target.ID, subject) {
		if targetAllowed(h, c, restriction, t) {
			candidates = append(candidates, t)
		}
	}

	if magnet := strings.TrimSpace(sa.Params["DefinedMagnet"]); magnet != "" {
		var picked []state.Target
		for _, t := range Defined(h, c, &cards.SA{Params: map[string]string{"Defined": magnet}}) {
			if targetIn(candidates, t) {
				picked = append(picked, t)
				break
			}
		}
		if len(picked) > 0 {
			picked = append(picked, target.Targets[1:]...)
			recordTargets(h, target.ID, picked)
		}
		return
	}

	count := len(target.Targets)
	if strings.EqualFold(sa.Params["ChangeSingleTarget"], "True") {
		count = 1
	}
	if strings.EqualFold(sa.Params["RandomTarget"], "True") {
		pool := append([]state.Target(nil), candidates...)
		var picked []state.Target
		for len(picked) < count && len(pool) > 0 {
			i := h.Rand(len(pool))
			picked = append(picked, pool[i])
			pool = append(pool[:i], pool[i+1:]...)
		}
		if len(picked) > 0 {
			if len(picked) < len(target.Targets) {
				picked = append(picked, target.Targets[len(picked):]...)
			}
			recordTargets(h, target.ID, picked)
		}
		return
	}

	min, max := count, count
	if strings.EqualFold(sa.Params["Optional"], "True") {
		min = 0
	}
	if max > len(candidates) {
		max = len(candidates)
		if min > max {
			min = max
		}
	}
	d := &decision.Decision{Player: chooser, Kind: decision.KChoose, Source: c.Source, Min: min, Max: max, ResumeKind: "choice", ResumeSA: sa, Prompt: "Choose new target"}
	for _, t := range candidates {
		o := decision.Option{Index: len(d.Options)}
		if t.IsPlayer {
			o.Kind, o.Player = "player", t.Player
		} else {
			o.Kind, o.Obj = "card", t.Obj
			if obj := h.Game().Obj(t.Obj); obj != nil {
				o.Player = obj.Controller
			}
		}
		d.Options = append(d.Options, o)
	}
	if Ask(h, d) == AskAsked {
		return
	}
	// A no-ask host (or a redirect with no legal new target) takes the
	// conservative Optional answer: no target changes. It must not leave
	// ChoiceDone set, which a later choice in the same chain would read as
	// its own answer.
}

func repeatPlayers(h Host, c *Ctx, spec string) ([]state.PlayerID, bool) {
	selected := map[state.PlayerID]bool{}
	add := func(ts []state.Target) {
		for _, t := range ts {
			p := PlayerOf(h, c, t)
			if int(p) >= 0 && int(p) < len(h.Game().Players) && !h.Game().Players[p].Lost {
				selected[p] = true
			}
		}
	}
	switch spec {
	case "Player":
		for _, p := range h.Game().AliveFrom(c.Controller) {
			selected[p] = true
		}
	case "Opponent", "Player.Opponent":
		for _, p := range h.Game().AliveFrom(c.Controller) {
			selected[p] = p != c.Controller
		}
	case "You", "NonOpponent":
		selected[c.Controller] = true
	case "Targeted", "TargetedPlayer", "TargetedController":
		add(c.Targets)
	case "TargetedAndYou":
		add(c.Targets)
		selected[c.Controller] = true
	case "Remembered", "RememberedController":
		add(c.Remembered)
	case "NonTargetedController":
		add(c.Targets)
		for _, p := range h.Game().AliveFrom(c.Controller) {
			selected[p] = !selected[p]
		}
	case "OppNonRememberedController":
		add(c.Remembered)
		for _, p := range h.Game().AliveFrom(c.Controller) {
			selected[p] = p != c.Controller && !selected[p]
		}
	case ".Chosen,You", "Chosen,You":
		add(c.Chosen)
		selected[c.Controller] = true
	default:
		// The shared player filter covers Player.Chosen and other qualifiers
		// for which the engine has state. Unknown qualifiers fail closed and
		// are reported rather than silently broadening the loop.
		for _, p := range h.Game().AliveFrom(c.Controller) {
			if MatchesPlayerSpecFrom(h.Game(), spec, p, c.Controller, c.Source) {
				selected[p] = true
			}
		}
		if len(selected) == 0 {
			return nil, false
		}
	}
	var out []state.PlayerID
	for _, p := range h.Game().AliveFrom(c.Controller) {
		if selected[p] {
			out = append(out, p)
		}
	}
	return out, true
}

func repeatedCards(h Host, c *Ctx, sa *cards.SA) ([]state.Target, bool) {
	if spec := strings.TrimSpace(sa.Params["DefinedCards"]); spec != "" {
		switch strings.Split(spec, ".")[0] {
		case "Targeted":
			return objectsOf(c.Targets), true
		case "Remembered", "RememberedLKI", "RememberedCard", "DirectRemembered", "ImprintedLKI":
			return objectsOf(c.Remembered), true
		case "ChosenCard":
			return objectsOf(c.Chosen), true
		}
		return objectsOf(Defined(h, c, &cards.SA{Params: map[string]string{"Defined": spec}})), true
	}
	spec := strings.TrimSpace(sa.Params["RepeatCards"])
	if spec == "" {
		return nil, false
	}
	zones := map[state.Zone]bool{state.ZBattlefield: true}
	if raw := strings.TrimSpace(sa.Params["Zone"]); raw != "" {
		zones = map[state.Zone]bool{}
		for _, z := range strings.Split(raw, ",") {
			switch strings.TrimSpace(z) {
			case "Battlefield":
				zones[state.ZBattlefield] = true
			case "Hand":
				zones[state.ZHand] = true
			case "Library":
				zones[state.ZLibrary] = true
			case "Graveyard":
				zones[state.ZGraveyard] = true
			case "Exile":
				zones[state.ZExile] = true
			case "Stack":
				zones[state.ZStack] = true
			}
		}
	}
	var out []state.Target
	for _, z := range []state.Zone{state.ZBattlefield, state.ZHand, state.ZLibrary, state.ZGraveyard, state.ZExile, state.ZStack} {
		if !zones[z] {
			continue
		}
		players := h.Game().AliveFrom(c.Controller)
		if z == state.ZStack { // the stack is one shared zone, not one per seat
			players = []state.PlayerID{0}
		}
		for _, p := range players {
			for _, id := range h.Game().Zone(z, p) {
				if o := h.Game().Obj(id); o != nil && choiceMatches(h.Game(), c, spec, o) {
					out = append(out, state.Target{Obj: id})
				}
			}
		}
	}
	return out, true
}

func effRepeatEach(h Host, c *Ctx, sa *cards.SA) {
	if c.SVars == nil {
		return
	}
	sub := cards.ResolveSVar(c.SVars, sa.Params["RepeatSubAbility"])
	if sub == nil {
		return
	}
	var subjects []state.Target
	start := 0
	// DamageMap$ True (Price of Progress, Wing Storm, Baki's Curse -- 87
	// corpus files): the loop's damage is ONE damage batch. Forge accumulates
	// every iteration's dealDamage into a per-SA damage table and deals it
	// once after the loop (RepeatEachEffect.resolve's DamageMap halves); in
	// this build that is the existing damage-batch bracket -- opened around
	// the whole loop, closed after the last iteration -- so the loop's
	// DamageDealtOnce triggers latch once per batch instead of once per
	// iteration's own batch-of-one. The deal sites stay inside the body (each
	// DealDamage's own bracket nests inside this one; the batch is depth-
	// counted), and the events themselves are unchanged -- same order, same
	// amounts -- so a game without a batch-latched trigger replays exactly as
	// before. Opened only on the first pass: a mid-loop suspension leaves the
	// engine's open batch intact across the resume, and the re-entry pass
	// closes it when the loop completes, so the bracket is balanced however
	// many resumes interleave.
	batched := strings.EqualFold(strings.TrimSpace(sa.Params["DamageMap"]), "True")
	var batcher interface {
		BeginDamageBatch()
		EndDamageBatch()
	}
	if b, ok := h.(interface {
		BeginDamageBatch()
		EndDamageBatch()
	}); ok {
		batcher = b
	}
	firstPass := true
	if cur := c.Repeat; cur != nil && cur.SA == sa {
		// Re-entry after an iteration suspended: continue with the subjects
		// the loop started with, after the one that asked, and keep what the
		// completed iteration remembered.
		c.Repeat = nil
		subjects, start, firstPass = cur.Subjects, cur.Next, false
		if cur.HasLast && start > 0 && start <= len(subjects) {
			prev := subjects[start-1]
			c.Remembered = rememberIteration(c.Remembered, cur.Last, iterationBase(c, prev), prev)
		}
	} else {
		var ok bool
		switch {
		case sa.Params["RepeatPlayers"] != "":
			var ps []state.PlayerID
			ps, ok = repeatPlayers(h, c, sa.Params["RepeatPlayers"])
			for _, p := range ps {
				subjects = append(subjects, state.Target{Player: p, IsPlayer: true})
			}
		case sa.Params["RepeatSpellAbilities"] != "":
			subjects, ok = validStackTargets(h.Game(), sa.Params["RepeatSpellAbilities"], c), true
		case sa.Params["RepeatTargeted"] != "":
			subjects, ok = copyTargets(c.Targets), true
		default:
			subjects, ok = repeatedCards(h, c, sa)
		}
		if !ok {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Text: "RepeatEach selector unimplemented"})
			return
		}
	}
	if batched && firstPass && batcher != nil {
		batcher.BeginDamageBatch()
	}
	for i := start; i < len(subjects); i++ {
		t := subjects[i]
		cc := *c
		cc.Repeat = nil
		// Forge binds the current loop subject as Remembered; the resolving
		// source/controller remain those of the outer spell or ability.
		base := iterationBase(c, t)
		cc.Remembered = append(copyTargets(base), t)
		// UseImprinted$ names the same subject "Imprinted" for the body's
		// selectors (UnlessPayer$ ImprintedController, Defined$
		// ImprintedController). The suspension carries it so a resumed ask
		// inside the body still binds it.
		cc.RepeatSubject = t
		Resolve(h, &cc, sub)
		if h.Suspended() {
			h.SuspendRepeat(RepeatSuspension{
				RepeatCursor: RepeatCursor{SA: sa, Subjects: copyTargets(subjects), Next: i + 1},
				Body:         copyTargets(cc.Remembered),
				Subject:      t,
				Outer:        copyTargets(c.Remembered),
				Chosen:       copyTargets(c.Chosen),
				ChosenValid:  c.ChosenValid,
			})
			return
		}
		c.Remembered = rememberIteration(c.Remembered, cc.Remembered, base, t)
	}
	if batched && batcher != nil {
		// The loop completed: close the batch opened for it. A re-entry pass
		// closes the batch the FIRST pass opened (same SA, same host, so the
		// open/close conditions agree); every pass that suspends mid-loop
		// returns before this line and leaves the bracket to a later pass.
		batcher.EndDamageBatch()
	}
}

// iterationBase is what an iteration's Remembered holds besides its subject.
// Forge's RepeatEachEffect swaps only remembered PLAYERS out for a player
// loop, so the cards the resolution remembered stay visible to the body
// (Braids's "a permanent that shares a card type with it"). The event object
// a trigger captured is not part of that list in Forge and is left out here.
// A card or spell loop binds its subject alone.
func iterationBase(c *Ctx, subject state.Target) []state.Target {
	if !subject.IsPlayer {
		return nil
	}
	captured := copyTargets(c.Captured)
	var out []state.Target
	for _, t := range c.Remembered {
		if t.IsPlayer {
			continue
		}
		if i := indexTarget(captured, t); i >= 0 {
			captured = append(captured[:i], captured[i+1:]...)
			continue
		}
		out = append(out, t)
	}
	return out
}

func indexTarget(ts []state.Target, want state.Target) int {
	for i, t := range ts {
		if t == want {
			return i
		}
	}
	return -1
}

// rememberIteration folds what one RepeatEach iteration remembered back into
// the loop's own Remembered. Forge adds the subject to the host's remembered
// list for the iteration and removes only the subject afterwards, so
// anything the iteration remembered (RememberChosen$, RememberDiscarded$, ...)
// is still remembered by the sub-abilities after the loop. body is the
// iteration's final Remembered; base (the entries it started with besides
// the subject) and the subject are not additions. The result is a fresh
// slice: outer may share a backing array with a stack object.
func rememberIteration(outer, body, base []state.Target, subject state.Target) []state.Target {
	out := copyTargets(outer)
	start := append(copyTargets(base), subject)
	for _, t := range body {
		if i := indexTarget(start, t); i >= 0 {
			start = append(start[:i], start[i+1:]...)
			continue
		}
		out = append(out, t)
	}
	return out
}
func effBranch(h Host, c *Ctx, sa *cards.SA) {
	if c.SVars == nil {
		return
	}
	v := Num(h, c, &cards.SA{Params: map[string]string{"condition": sa.Params["BranchConditionSVar"]}}, "condition", 0)
	// Forge's BranchEffect defaults an absent BranchConditionSVarCompare$ to
	// GE1 (31 of the corpus's Branch lines rely on it: "if X is at least
	// one"). An operator this build does not know takes the false arm.
	cmp := strings.TrimSpace(sa.Params["BranchConditionSVarCompare"])
	if cmp == "" {
		cmp = "GE1"
	}
	op := ""
	if len(cmp) >= 2 {
		op, cmp = strings.ToUpper(cmp[:2]), cmp[2:]
	}
	n, err := strconv.Atoi(cmp)
	if err != nil {
		// Branch uses the same literal/SVar count vocabulary as its left side.
		// Inline PlayerCount...$Amount is Forge's spelling for a count head.
		raw := strings.TrimSuffix(cmp, "$Amount")
		if body, found := c.SVars[raw]; found {
			n = int(EvalCount(h, c, body))
		} else {
			n = int(EvalCount(h, c, "Count$"+raw))
		}
	}
	yes := compareCount(op, int(v), n)
	name := sa.Params["FalseSubAbility"]
	if yes {
		name = sa.Params["TrueSubAbility"]
	}
	if sub := cards.ResolveSVar(c.SVars, name); sub != nil {
		Resolve(h, c, sub)
	}
}
