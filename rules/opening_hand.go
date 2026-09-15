package rules

import (
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// openingRound serializes "may begin the game" choices before the London
// round. The source is an SVar name, rather than a card name, so every
// FromHand/FromOpeningHand script with the normal self-to-battlefield shape
// shares this flow.
// Existing chooseFor values occupy 0 through 9 (chooseManaExile is 9).
const chooseOpening chooseFor = iota + 10

type openingRound struct {
	start     state.PlayerID
	mulligans int
	effects   []openingEffect
	index     int
	exile     state.ObjID
}

type openingEffect struct {
	player state.PlayerID
	card   state.ObjID
	svar   string
}

// openingActionSVar finds the SVar which performs the pregame action. Forge's
// keyword parameter is normally that action's name (FromHand, RevealCard),
// but some scripts name its delayed child instead: Chancellor of the Tangle
// says ManaOnMain while RevealCard is the actual reveal-and-register action.
// Prefer an explicit pregame action, then a SVar which chains into the named
// child. Sorting keeps the fallback deterministic if a future script has more
// than one descriptive opening action.
func openingActionSVar(f *cards.Face, raw string) string {
	name, _, _ := strings.Cut(raw, ":")
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if sa := cards.ResolveSVar(f.SVars, name); sa != nil &&
		(strings.HasPrefix(name, "From") || strings.HasPrefix(name, "Exile") ||
			strings.Contains(strings.ToLower(sa.Params["SpellDescription"]), "opening hand")) {
		return name
	}
	keys := make([]string, 0, len(f.SVars))
	for key := range f.SVars {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		sa := cards.ResolveSVar(f.SVars, key)
		if sa != nil && sa.Params["SubAbility"] == name &&
			strings.Contains(strings.ToLower(sa.Params["SpellDescription"]), "opening hand") {
			return key
		}
	}
	for _, key := range keys {
		sa := cards.ResolveSVar(f.SVars, key)
		if sa != nil && strings.Contains(strings.ToLower(sa.Params["SpellDescription"]), "opening hand") {
			return key
		}
	}
	// A parameter that is itself an SVar remains the final conservative
	// fallback for scripts without Forge's descriptive field.
	if cards.ResolveSVar(f.SVars, name) != nil {
		return name
	}
	return ""
}

func cloneOpening(o openingRound) openingRound {
	o.effects = append([]openingEffect(nil), o.effects...)
	return o
}

func (e *Engine) newOpeningRound(start state.PlayerID, mulligans int) openingRound {
	r := openingRound{start: start, mulligans: mulligans}
	for _, p := range e.G.AliveFrom(start) {
		for _, id := range e.G.Zone(state.ZHand, p) {
			o := e.G.Obj(id)
			if o == nil || o.Face() == nil {
				continue
			}
			raw, ok := o.Face().KeywordParam("MayEffectFromOpeningHand")
			if !ok {
				continue
			}
			_, requirement, _ := strings.Cut(raw, ":")
			// !PlayFirst is Gemstone Caverns' and Impatient Iguana's gate.
			// Other FromHand shapes are unconditional; unsupported predicates
			// fail closed rather than accidentally granting a pregame effect.
			if strings.Contains(requirement, "!PlayFirst") && p == start {
				continue
			}
			svar := openingActionSVar(o.Face(), raw)
			if svar == "" {
				continue
			}
			r.effects = append(r.effects, openingEffect{player: p, card: id, svar: svar})
		}
	}
	return r
}

func (e *Engine) stepOpening() {
	if e.opening.exile != 0 {
		return
	}
	for e.opening.index < len(e.opening.effects) {
		ef := e.opening.effects[e.opening.index]
		o := e.G.Obj(ef.card)
		if o == nil || o.Zone != state.ZHand || o.Face() == nil {
			e.opening.index++
			continue
		}
		d := decision.New(ef.player, decision.KChoose, "Begin the game with "+o.Face().Name+"?", 1, 1,
			[]decision.Option{{Index: 0, Kind: "opening_yes", Obj: ef.card, Label: "Yes"},
				{Index: 1, Kind: "opening_no", Obj: ef.card, Label: "No"}})
		e.choosing = chooseOpening
		e.ask(d)
		return
	}
	e.finishOpening()
}

// applyOpeningEffect handles the common keyword expansion: move Self from the
// opening hand to the battlefield, then apply its immediate PutCounter sub.
// A following mandatory hand-to-exile ChangeZone is represented by a real
// KChoose continuation (Gemstone Caverns), rather than a deterministic card.
func (e *Engine) applyOpeningEffect(ef openingEffect) {
	o := e.G.Obj(ef.card)
	if o == nil || o.Face() == nil {
		return
	}
	sa := cards.ResolveSVar(o.Face().SVars, ef.svar)
	if sa == nil {
		return
	}
	if sa.API != "ChangeZone" || sa.Params["Origin"] != "Hand" || sa.Params["Destination"] != "Battlefield" {
		// Reveal, exile, token and delayed-effect opening scripts use the
		// ordinary effect registry too. The FromHand battlefield shape below
		// is split out only because Gemstone's mandatory follow-up needs its
		// own pregame card-choice continuation.
		ctx := &effects.Ctx{Source: ef.card, Controller: ef.player}
		effects.SetSVars(ctx, o.Face().SVars)
		effects.Resolve(e, ctx, sa)
		return
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: ef.card, From: state.ZHand, To: state.ZBattlefield, Text: "opening hand effect"})
	if sa.Sub != nil && sa.Sub.API == "PutCounter" {
		n := effects.Num(e, &effects.Ctx{Source: ef.card, Controller: ef.player}, sa.Sub, "CounterNum", 1)
		e.emit(events.Event{Kind: events.CounterChange, Obj: ef.card, Counter: sa.Sub.Params["CounterType"], Amount: n})
		sa = sa.Sub
	}
	if sa.Sub != nil && sa.Sub.API == "ChangeZone" && sa.Sub.Params["Origin"] == "Hand" && sa.Sub.Params["Destination"] == "Exile" {
		e.opening.exile = ef.card
		d := &decision.Decision{Player: ef.player, Kind: decision.KChoose, Min: 1, Max: 1,
			Prompt: "Exile a card from your hand", Source: ef.card}
		for _, id := range e.G.Zone(state.ZHand, ef.player) {
			d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "opening_exile", Obj: id, Label: e.G.Obj(id).Face().Name})
		}
		if len(d.Options) > 0 {
			e.choosing = chooseOpening
			e.ask(d)
		}
	}
}

func (e *Engine) handleOpening(d *decision.Decision, in decision.Intent) {
	chosen := d.Chosen(in)
	if len(chosen) == 0 {
		return
	}
	if e.opening.exile != 0 {
		e.emit(events.Event{Kind: events.MoveZone, Obj: chosen[0].Obj, From: state.ZHand, To: state.ZExile, Text: "opening hand effect"})
		e.opening.exile = 0
		e.opening.index++
		e.stepOpening()
		return
	}
	if chosen[0].Kind == "opening_yes" && e.opening.index < len(e.opening.effects) {
		e.applyOpeningEffect(e.opening.effects[e.opening.index])
		if e.opening.exile != 0 {
			return
		}
	}
	e.opening.index++
	e.stepOpening()
}

func (e *Engine) finishOpening() {
	start, mulligans := e.opening.start, e.opening.mulligans
	e.opening = openingRound{}
	if mulligans > 0 {
		e.pregame = true
		e.mulligan = newMulliganRound(e.G.AliveFrom(start), mulligans)
		return
	}
	e.beginTurn(start)
}

func init() { effects.RegisterNonAPI("kw:MayEffectFromOpeningHand") }
