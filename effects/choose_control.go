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
func definedCardPool(c *Ctx, raw string) ([]state.Target, string) {
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
		// The state does not retain craft/exile provenance. Fail closed rather
		// than substituting every exiled card or the resolution's targets.
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
		candidates, qualifier = definedCardPool(c, raw)
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

// choiceMatches adds the resolution-local remembered predicate to the normal
// object matcher. IsRemembered is deliberately here rather than global filter
// state: a choice must see the objects this resolution remembered, never a
// similarly named object elsewhere in the game. Each alternative's
// IsRemembered / !IsRemembered conjunct is decided against Ctx.Remembered and
// the rest of that alternative goes to the ordinary matcher, so a compound
// such as Chaos Defiler's Card.IsRemembered+withoutIndestructible keeps both
// halves.
func choiceMatches(g *state.Game, c *Ctx, spec string, o *state.Object) bool {
	sc := c.SpecContext(c.Controller)
	if !strings.Contains(spec, "IsRemembered") {
		return MatchesObjectCtx(g, spec, o, sc)
	}
	remembered := targetIn(c.Remembered, state.Target{Obj: o.ID})
	for _, alt := range strings.Split(spec, ",") {
		base, preds, qualified := strings.Cut(strings.TrimSpace(alt), ".")
		ok := true
		var rest []string
		if qualified {
			for _, p := range strings.Split(preds, "+") {
				switch strings.TrimSpace(p) {
				case "IsRemembered":
					ok = ok && remembered
				case "!IsRemembered":
					ok = ok && !remembered
				default:
					rest = append(rest, p)
				}
			}
		}
		if !ok {
			continue
		}
		if len(rest) > 0 {
			base += "." + strings.Join(rest, "+")
		}
		if MatchesObjectCtx(g, base, o, sc) {
			return true
		}
	}
	return false
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

func choiceRecord(h Host, c *Ctx, sa *cards.SA, picked []state.Target) {
	c.Choice = append([]state.Target(nil), picked...)
	c.Chosen = append(c.Chosen, picked...)
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

func effChooseCard(h Host, c *Ctx, sa *cards.SA) {
	choosers := choiceChoosers(h, c, sa)
	i := c.ChoiceTarget
	if c.ChoiceDone {
		choiceRecord(h, c, sa, c.Choice)
		c.ChoiceDone, c.Choice = false, nil
		i++
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
			choiceRecord(h, c, sa, randomChoices(h, choices, max))
			continue
		}
		d := &decision.Decision{Player: choosers[i], Kind: decision.KChoose, Source: c.Source, Min: min, Max: max, ResumeKind: "choice", ResumeSA: sa, ResumeTarget: i, ResumeChoices: append([]state.Target(nil), c.Chosen...), ResumeChosenValid: c.ChosenValid, ResumeRemembered: append([]state.Target(nil), c.Remembered...), Prompt: sa.Params["ChoiceTitle"]}
		for j, t := range choices {
			d.Options = append(d.Options, decision.Option{Index: j, Kind: "card", Obj: t.Obj, Player: choosers[i]})
		}
		if d.Prompt == "" {
			d.Prompt = "Choose card"
		}
		if h.Ask(d) {
			return
		}
		choiceRecord(h, c, sa, choices[:min])
	}
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
		choiceRecord(h, c, sa, c.Choice)
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
			choiceRecord(h, c, sa, randomChoices(h, choices, max))
			continue
		}
		d := &decision.Decision{Player: choosers[i], Kind: decision.KChoose, Source: c.Source, Min: min, Max: max, ResumeKind: "choice", ResumeSA: sa, ResumeTarget: i, ResumeChoices: append([]state.Target(nil), c.Chosen...), ResumeChosenValid: c.ChosenValid, ResumeRemembered: append([]state.Target(nil), c.Remembered...), Prompt: sa.Params["ChoiceTitle"]}
		for j, t := range choices {
			d.Options = append(d.Options, decision.Option{Index: j, Kind: "player", Player: t.Player})
		}
		if d.Prompt == "" {
			d.Prompt = "Choose player"
		}
		if h.Ask(d) {
			return
		}
		choiceRecord(h, c, sa, choices[:min])
	}
}

func controlPlayer(h Host, c *Ctx, sa *cards.SA) state.PlayerID {
	v := strings.TrimSpace(sa.Params["NewController"])
	if v == "" || v == "You" || v == "True" {
		return c.Controller
	}
	if v == "ChosenPlayer" {
		for _, t := range c.Chosen {
			if t.IsPlayer {
				return t.Player
			}
		}
		// A choice made by an earlier, independently resolving ability is
		// event-backed on its source rather than present in this fresh Ctx.
		if o := h.Game().Obj(c.Source); o != nil {
			for _, t := range o.Chosen {
				if t.IsPlayer {
					return t.Player
				}
			}
		}
	}
	if v == "Player.IsRemembered" {
		for _, t := range c.Remembered {
			if t.IsPlayer {
				return t.Player
			}
		}
	}
	for _, t := range Defined(h, c, &cards.SA{Params: map[string]string{"Defined": v}}) {
		return PlayerOf(h, c, t)
	}
	return c.Controller
}
func effGainControl(h Host, c *Ctx, sa *cards.SA) {
	g := h.Game()
	dur, unknown := ParseControlDuration(sa.Params["LoseControl"])
	if unknown != "" {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Text: "GainControl LoseControl$ " + unknown + " unimplemented"})
		return
	}
	base := ControlGrant{You: c.Controller, Source: c.Source, Duration: dur, SVars: c.SVars}
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
	p := controlPlayer(h, c, sa)
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
	}
}
func effControlSpell(h Host, c *Ctx, sa *cards.SA) {
	p := controlPlayer(h, c, sa)
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
	for _, t := range h.LegalTargets(chooser, target.ID, subject) {
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
	if h.Ask(d) {
		return
	}
	// A no-ask host takes the conservative Optional answer: no target changes.
	c.ChoiceDone = true
	c.Choice = nil
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
	if cur := c.Repeat; cur != nil && cur.SA == sa {
		// Re-entry after an iteration suspended: continue with the subjects
		// the loop started with, after the one that asked, and keep what the
		// completed iteration remembered.
		c.Repeat = nil
		subjects, start = cur.Subjects, cur.Next
		if cur.HasLast && start > 0 && start <= len(subjects) {
			c.Remembered = rememberIteration(c.Remembered, cur.Last, subjects[start-1])
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
	for i := start; i < len(subjects); i++ {
		t := subjects[i]
		cc := *c
		cc.Repeat = nil
		// Forge binds the current loop subject as Remembered; the resolving
		// source/controller remain those of the outer spell or ability.
		cc.Remembered = []state.Target{t}
		Resolve(h, &cc, sub)
		if h.Suspended() {
			h.SuspendRepeat(RepeatSuspension{
				RepeatCursor: RepeatCursor{SA: sa, Subjects: copyTargets(subjects), Next: i + 1},
				Body:         copyTargets(cc.Remembered),
				Outer:        copyTargets(c.Remembered),
				Chosen:       copyTargets(c.Chosen),
				ChosenValid:  c.ChosenValid,
			})
			return
		}
		c.Remembered = rememberIteration(c.Remembered, cc.Remembered, t)
	}
}

// rememberIteration folds what one RepeatEach iteration remembered back into
// the loop's own Remembered. Forge adds the subject to the host's remembered
// list for the iteration and removes only the subject afterwards, so
// anything the iteration remembered (RememberChosen$, RememberDiscarded$, ...)
// is still remembered by the sub-abilities after the loop. The result is a
// fresh slice: outer may share a backing array with a stack object.
func rememberIteration(outer, body []state.Target, subject state.Target) []state.Target {
	out := copyTargets(outer)
	dropped := false
	for _, t := range body {
		if !dropped && t == subject {
			dropped = true
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
	cmp := sa.Params["BranchConditionSVarCompare"]
	op := ""
	for _, x := range []string{"GE", "GT", "LE", "LT", "EQ"} {
		if strings.HasPrefix(cmp, x) {
			op = x
			cmp = strings.TrimPrefix(cmp, x)
			break
		}
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
	yes := op == ""
	switch op {
	case "GE":
		yes = v >= int32(n)
	case "GT":
		yes = v > int32(n)
	case "LE":
		yes = v <= int32(n)
	case "LT":
		yes = v < int32(n)
	case "EQ":
		yes = v == int32(n)
	}
	name := sa.Params["FalseSubAbility"]
	if yes {
		name = sa.Params["TrueSubAbility"]
	}
	if sub := cards.ResolveSVar(c.SVars, name); sub != nil {
		Resolve(h, c, sub)
	}
}
