package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func (e *Engine) castSpell(p state.PlayerID, id state.ObjID) {
	o := e.G.Obj(id)
	f := o.Face()
	// adjustedCost, not the printed ManaCost: legalActions already gated this
	// option on being able to pay the RaiseCost/ReduceCost-adjusted amount, so
	// paying anything else here would charge a different amount than the one
	// the player was shown -- silently undercharging a raised cost, or (since
	// payMana no-ops rather than erroring on an unpayable cost) silently
	// letting a reduced-cost spell resolve for free when the pool only holds
	// the reduced amount. This does not yet cover AlternativeCost: which
	// "cast" option (base vs. alternative) was chosen is not threaded through
	// decision.Option, so an alternative-cost cast is still charged the base
	// adjusted cost. Statics.go's alternativeCosts only affects which options
	// legalActions offers, not what paying one of them charges.
	e.payMana(p, e.adjustedCost(p, id))
	// PutOnStack's own Move() reads the object's CURRENT Controller to pick
	// which stack list to push onto; a hand card is only ever cast by its
	// own owner in M1 (no stealing effects yet), so Controller already
	// equals p here and needs no write. The same Move() also resets
	// Targets to nil for any object leaving to a non-battlefield zone, so a
	// freshly-cast spell needs no explicit reset either.
	e.emit(events.Event{Kind: events.PutOnStack, Obj: id, Player: p,
		From: state.ZHand, To: state.ZStack, Text: f.Name})

	if sa := f.SpellAbility(); sa != nil && sa.Params["ValidTgts"] != "" {
		e.askTarget(p, id, sa)
		return
	}
	// No trailing Priority emit here (Ruling T14-e): legal.go's "cast" case
	// already emits Priority{Player: e.G.Priority, Amount: 0} immediately
	// before calling castSpell, and e.G.Priority at that point is the
	// caster (Submit already validated in.Player == d.Player == the seat
	// that was asked). CR 117.3c: the caster keeps priority, so re-emitting
	// here with e.G.Active would both duplicate that event and hand
	// priority to the wrong seat whenever a non-active player casts.
}

// manaLetters is state.Mana's index order (MW, MU, MB, MR, MG, MC) spelled
// out as the WUBRGC symbols events.ManaAdd's Counter field expects.
var manaLetters = [...]string{"W", "U", "B", "R", "G", "C"}

// payMana spends cost from p's pool. Every state mutation goes through
// events, so payment cannot be a direct field write to Players[p].Pool: it
// emits one ManaAdd event per colour bucket actually spent, with a negative
// Amount. That reuses the existing ManaAdd kind rather than adding a new one
// -- Apply's ManaAdd case is a plain "+=", so a negative Amount already
// subtracts correctly, the same trick Ruling F4 uses for clearing damage.
// legalActions only ever offers "cast" when CanPay already holds, so the
// !ok case below is unreached in practice; it is a no-op rather than an
// overspend if it is ever reached some other way.
func (e *Engine) payMana(p state.PlayerID, cost Cost) {
	before := e.G.Players[p].Pool
	after, ok := cost.Pay(before)
	if !ok {
		return
	}
	for i, letter := range manaLetters {
		if spent := before[i] - after[i]; spent != 0 {
			e.emit(events.Event{Kind: events.ManaAdd, Player: p, Counter: letter, Amount: -spent})
		}
	}
}

// askTarget offers every legal target for a spell. M1 handles the single-target
// case; TargetMin/TargetMax widen it once multi-target cards are in scope.
func (e *Engine) askTarget(p state.PlayerID, source state.ObjID, sa *cards.SA) {
	spec := sa.Params["ValidTgts"]
	d := &decision.Decision{Player: p, Kind: decision.KTarget, Min: 1, Max: 1,
		Prompt: "Choose a target for " + e.G.Obj(source).Face().Name,
		Source: source}
	add := func(kind, label string, obj state.ObjID, pl state.PlayerID) {
		d.Options = append(d.Options, decision.Option{
			Index: len(d.Options), Kind: kind, Label: label, Obj: obj, Player: pl})
	}
	if targetsPlayers(spec) {
		for _, q := range e.G.AliveFrom(0) {
			add("player", e.G.Players[q].Name, 0, q)
		}
	}
	if targetsPermanents(spec) {
		for _, q := range e.G.AliveFrom(0) {
			for _, oid := range e.G.Zone(state.ZBattlefield, q) {
				o := e.G.Obj(oid)
				if effects.MatchesSpec(e.G, spec, oid, p) {
					add("permanent", o.Face().Name+" ("+e.G.Players[q].Name+")", oid, q)
				}
			}
		}
	}
	if len(d.Options) == 0 {
		// CR 608.2b: a spell with no legal targets is countered on resolution.
		// The spike models that as an immediate move to the graveyard.
		e.emit(events.Event{Kind: events.MoveZone, Obj: source,
			From: state.ZStack, To: state.ZGraveyard, Text: "countered: no legal targets"})
		// Ruling T14-e: p, the casting player, not e.G.Active -- CR 117.3c,
		// the caster keeps priority even when it fizzles.
		e.emit(events.Event{Kind: events.Priority, Player: p, Amount: 0})
		return
	}
	e.ask(d)
}

// handleTarget records the chosen target(s) via a TargetsChosen event (Ruling
// T14-b) rather than writing state.Object.Targets directly: apply.go clears
// Targets on a zone change but nothing else ever set it, so a direct write
// here would leave a replayed game with no targets while the live game had
// them.
func (e *Engine) handleTarget(d *decision.Decision, in decision.Intent) {
	opt := d.Chosen(in)[0]
	ev := events.Event{Kind: events.TargetsChosen, Obj: d.Source}
	if opt.Kind == "player" {
		ev.Amount = 1
		ev.Player = opt.Player
	} else {
		ev.IDs = []state.ObjID{opt.Obj}
	}
	e.emit(ev)
	// Ruling T14-e: the submitting player, not e.G.Active -- CR 117.3c, the
	// player who chose the target (the caster) keeps priority.
	e.emit(events.Event{Kind: events.Priority, Player: in.Player, Amount: 0})
}

// resolveTop resolves the object on top of the stack and moves it to
// wherever it goes next.
func (e *Engine) resolveTop() {
	id := e.G.Stack[len(e.G.Stack)-1]
	o := e.G.Obj(id)

	if o.Ability != nil {
		// A triggered or activated ability with no printed card: Ruling
		// T14-c / F3 -- Face() returns nil for these, so this branch must
		// run before anything below touches it. Task 20 is what actually
		// puts objects like this on the stack; nothing in Task 14 creates
		// one, so this path is defensive rather than exercised here.
		// CR 608.2m: a resolved ability just ceases to exist rather than
		// moving to a card zone. This build has no "ceases to exist" zone,
		// so it is parked in exile as the closest existing approximation;
		// Task 20 owns getting this exactly right.
		e.emit(events.Event{Kind: events.Resolve, Obj: id})
		// No Face means no SVar table either, so this passes nil.
		e.resolveAbility(id, o.Controller, o.Targets, o.Ability, nil)
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZStack, To: state.ZExile})
		return
	}

	f := o.Face()
	e.emit(events.Event{Kind: events.Resolve, Obj: id, Text: f.Name})
	if sa := f.SpellAbility(); sa != nil {
		e.resolveAbility(id, o.Controller, o.Targets, sa, f.SVars)
	}
	if f.IsPermanent() {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZStack, To: state.ZBattlefield})
	} else {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZStack, To: state.ZGraveyard})
	}
}

// resolveAbility walks an SA chain, running each API's implementation. svars
// is the resolving face's SVar table (nil for an ability-only stack object),
// which is what lets Num's SVar indirection and primitives like Charm and
// Repeat -- which run a sub-ability named by SVar rather than the
// auto-linked "SubAbility$" -- actually resolve something outside a test.
func (e *Engine) resolveAbility(source state.ObjID, controller state.PlayerID,
	targets []state.Target, sa *cards.SA, svars map[string]string) {
	ctx := &effects.Ctx{Source: source, Controller: controller, Targets: targets}
	effects.SetSVars(ctx, svars)
	effects.Resolve(e, ctx, sa)
}

// Game, Emit and Rand satisfy effects.Host, which is how effects reach the
// engine without importing it.
func (e *Engine) Game() *state.Game    { return e.G }
func (e *Engine) Emit(ev events.Event) { e.emit(ev) }
func (e *Engine) Rand(n int) int       { return e.rng.IntN(n) }

// targetsPlayers and targetsPermanents read the coarse shape of a ValidTgts
// spec. The per-object predicate work is effects.MatchesSpec.
func targetsPlayers(spec string) bool {
	return strings.Contains(spec, "Player") || strings.Contains(spec, "Any") ||
		strings.Contains(spec, "Opponent") || strings.Contains(spec, "You")
}

func targetsPermanents(spec string) bool {
	for _, t := range [...]string{"Creature", "Any", "Permanent", "Artifact",
		"Enchantment", "Land", "Planeswalker", "Card"} {
		if strings.Contains(spec, t) {
			return true
		}
	}
	return false
}
