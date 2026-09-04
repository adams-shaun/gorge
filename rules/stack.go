package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// castSpell moves opt.Obj from hand to the stack, having paid for it first.
//
// Which cost it pays is opt.AltCostIndex, not always adjustedCost: Ruling
// T19b-b. legalActions gates each "cast" option on that specific option's
// own cost being payable -- the base (RaiseCost/ReduceCost-adjusted) cost for
// AltCostIndex == 0, or alternativeCosts(p, id)[AltCostIndex-1] otherwise --
// so castSpell must charge that same cost, not unconditionally the base one.
// Previously it always charged adjustedCost regardless of which option was
// chosen; combined with payMana's old silent no-op on an unpayable cost, an
// alt-cost option -- offered because the ALTERNATIVE cost was payable, not
// the base one -- would fail to pay anything at all and still reach the
// stack: a free cast reachable from ordinary, well-formed card data (e.g. a
// 4-mana spell with an alternative {U} cost, cast with a pool of one blue).
//
// If the chosen cost cannot actually be paid, payMana now reports that and
// this aborts cleanly: no MoveZone/PutOnStack is emitted, so the card stays
// in hand, and payMana itself never emits anything unless the whole cost
// clears (Cost.Pay is all-or-nothing), so no partial mana is deducted either.
func (e *Engine) castSpell(p state.PlayerID, opt decision.Option) {
	id := opt.Obj
	o := e.G.Obj(id)
	f := o.Face()

	cost := e.adjustedCost(p, id)
	if opt.AltCostIndex > 0 {
		if alts := e.alternativeCosts(p, id); opt.AltCostIndex-1 < len(alts) {
			cost = alts[opt.AltCostIndex-1]
		}
		// An out-of-range AltCostIndex (stale option from a board state that
		// no longer holds the granting static) falls back to the base cost
		// rather than indexing out of bounds or paying nothing.
	}
	if !e.payMana(p, cost) {
		return
	}
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

// payMana spends cost from p's pool and reports whether it could. Every
// state mutation goes through events, so payment cannot be a direct field
// write to Players[p].Pool: it emits one ManaAdd event per colour bucket
// actually spent, with a negative Amount. That reuses the existing ManaAdd
// kind rather than adding a new one -- Apply's ManaAdd case is a plain "+=",
// so a negative Amount already subtracts correctly, the same trick Ruling F4
// uses for clearing damage.
//
// Ruling T19b-b: this used to be silent on failure (a no-op the caller could
// not observe), on the theory that legalActions always gates "cast" on
// CanPay first so failure here was unreachable. That theory held for the
// base cost but not for an alternative one: legalActions gates an alt-cost
// option on the ALTERNATIVE cost being payable, so a caller that (as
// castSpell used to) paid the base cost regardless of which option was
// chosen could genuinely hit this path with real, well-formed card data --
// and a silent no-op there is what let the spell go on the stack anyway,
// having paid nothing. Reporting failure explicitly is what lets castSpell
// abort the cast instead.
func (e *Engine) payMana(p state.PlayerID, cost Cost) bool {
	before := e.G.Players[p].Pool
	after, ok := cost.Pay(before)
	if !ok {
		return false
	}
	for i, letter := range manaLetters {
		if spent := before[i] - after[i]; spent != 0 {
			e.emit(events.Event{Kind: events.ManaAdd, Player: p, Counter: letter, Amount: -spent})
		}
	}
	return true
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
//
// CR 608.2b: before anything else runs, every target recorded when this
// spell or ability was put on the stack is rechecked against legalTargets
// below -- a target legal when chosen can stop being legal by the time its
// spell reaches the top of the stack, most commonly a creature that died to
// something else in the meantime. With no legal target left, this does not
// resolve at all: it leaves the stack (a spell to its owner's graveyard, an
// ability to exile per CR 608.2m just below) with no effect -- not even a
// SubAbility chained onto it that names no target of its own (Defined$ You
// and the like). That distinguishes this from the per-target nil-checks
// primitives like effDealDamage already had (Task 18, effects/damage.go):
// those already skip a target that individually vanished, but nothing
// before this stopped the OTHER, untargeted parts of the same spell's
// script from running anyway once every target it had was gone. With only
// some targets still legal, resolution proceeds against exactly that
// narrowed set -- CR 608.2b's "resolves, doing as much as possible".
func (e *Engine) resolveTop() {
	id := e.G.Stack[len(e.G.Stack)-1]
	o := e.G.Obj(id)

	if o.Ability != nil {
		// A triggered or activated ability with no printed card: Ruling
		// T14-c / F3 -- Face() returns nil for these, so this branch must
		// run before anything below touches it. Task 20 is what actually
		// puts objects like this on the stack.
		//
		// No triggered or activated ability this build produces ever
		// actually populates Targets (only Remembered): Task 20's
		// checkTriggers never calls askTarget, which is the only place
		// TargetsChosen is ever emitted from. So this is unreachable in
		// practice today, but a stack object is a stack object, and CR
		// 608.2b's "spell or ability" covers this shape too if a later
		// task ever gives a triggered ability a player-chosen target.
		targets := o.Targets
		// Fix round 2 (re-review N1): the gate is `spec != ""` -- "this
		// ability declares a targeting requirement" -- not `len(targets) > 0`
		// -- "this ability happens to have targets right now". The old form
		// used the latter as a proxy for the former, so an ability that NEEDS
		// a target but has none recorded skipped CR 608.2b's recheck entirely
		// and resolved. Zero recorded targets is zero LEGAL targets, which is
		// exactly what 608.2b counters.
		if spec := o.Ability.Params["ValidTgts"]; spec != "" {
			legal := e.legalTargets(targets, spec, o.Controller)
			if len(legal) == 0 {
				e.emit(events.Event{Kind: events.MoveZone, Obj: id,
					From: state.ZStack, To: state.ZExile, Text: "fizzled: no legal targets remain"})
				return
			}
			targets = legal
		}
		// CR 608.2m: a resolved ability just ceases to exist rather than
		// moving to a card zone. This build has no "ceases to exist" zone,
		// so it is parked in exile as the closest existing approximation.
		e.emit(events.Event{Kind: events.Resolve, Obj: id})
		// The ability object itself has no Face, so its SVar table (needed
		// for Num's SVar indirection, e.g. Goblin Piledriver's "NumAtt$ +X")
		// comes from the permanent that granted it (o.Source) instead.
		// SVars are static card-script text that never changes after
		// parsing, so reading them live from the source's current Face at
		// resolution time is equivalent to a snapshot taken when the
		// trigger was queued, with no need for a new field to carry one
		// through the stack. A source that has since left the battlefield
		// (or ceased to exist) has nothing to read here and degrades to a
		// nil SVar table, same as before this ability object existed at
		// all, rather than panicking.
		var svars map[string]string
		if src := e.G.Obj(o.Source); src != nil {
			if sf := src.Face(); sf != nil {
				svars = sf.SVars
			}
		}
		// Ruling T20-b: Source must be o.Source (the permanent that has this
		// ability), not id (the transient stack-object wrapper) -- Defined$
		// Self, the most common Defined$ value in real trigger scripts,
		// resolves to Ctx.Source, and a wrapper ID means "Self" refers to a
		// stack object with no Face() that leaves play the instant this
		// resolves, so the effect would silently apply to nothing. The SVar
		// lookup two lines above already gets this right by reading from
		// o.Source; this was a one-line inconsistency, not a second design.
		ctx := &effects.Ctx{Source: o.Source, Controller: o.Controller,
			Targets: targets, Remembered: o.Remembered}
		effects.SetSVars(ctx, svars)
		effects.Resolve(e, ctx, o.Ability)
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZStack, To: state.ZExile})
		return
	}

	f := o.Face()
	sa := f.SpellAbility()
	targets := o.Targets
	if sa != nil {
		// Fix round 2 (re-review N1), the same correction as the ability
		// branch above, and the one that was actually reachable. Widening the
		// departed-player release hook in fix round 1 turned a stall into a
		// spell that RESOLVES with no targets at all: the caster is
		// eliminated while their target decision is outstanding, the hook
		// clears it so the match can continue, and the spell is left on the
		// stack with Targets nil. Under the old `len(targets) > 0` gate that
		// skipped the recheck and ran the whole script -- untargeted riders
		// included -- gaining the ELIMINATED caster 7 life in the re-review's
		// own reproduction. CR 608.2b counters a spell with no legal targets;
		// CR 800.4a says a departed player's spell ceases to exist. Neither
		// permits it to resolve.
		if spec := sa.Params["ValidTgts"]; spec != "" {
			legal := e.legalTargets(targets, spec, o.Controller)
			if len(legal) == 0 {
				// CR 608.2b: every target became illegal. This spell does
				// not resolve -- no Resolve event, no script runs -- it goes
				// straight to the graveyard, the same zone it would reach
				// after an ordinary resolution (an Aura instead reaching
				// the battlefield would be wrong here: CR 704.5m's "nothing
				// legal to attach to" is exactly this case for that spell
				// shape, and the graveyard is where it belongs).
				e.emit(events.Event{Kind: events.MoveZone, Obj: id,
					From: state.ZStack, To: state.ZGraveyard, Text: "fizzled: no legal targets remain"})
				return
			}
			targets = legal
		}
	}
	e.emit(events.Event{Kind: events.Resolve, Obj: id, Text: f.Name})
	if sa != nil {
		e.resolveAbility(id, o.Controller, targets, sa, f.SVars)
	}
	if f.IsPermanent() {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZStack, To: state.ZBattlefield})
	} else {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZStack, To: state.ZGraveyard})
	}
}

// legalTargets is CR 608.2b's recheck, applied at resolution: the subset of
// targets that are still legal right now. An object target must still be on
// the battlefield -- matchesBase's own bare-type predicates (effects/
// filter.go), e.g. "Creature", read printed types straight off the Face
// with no zone check of their own, so a creature that died and is sitting
// in a graveyard would otherwise still look like a match -- and still
// satisfy spec, via the same effects.MatchesSpec call askTarget used to
// offer it as an option in the first place: no self-relative source, matching
// askTarget's own simplification, so a spec that would filter on Self/Other
// is exactly as (im)precise here as it was at cast time. A player target is
// legal for as long as they are still in the game; askTarget never applies
// MatchesPlayerSpec's finer You/Opponent distinction when it first offers
// every living player as an option (targetsPlayers below it), so this does
// not either -- rechecking against a filter the engine never enforced when
// the target was chosen would reject targets this build always considered
// fine.
func (e *Engine) legalTargets(targets []state.Target, spec string, you state.PlayerID) []state.Target {
	var legal []state.Target
	for _, t := range targets {
		if t.IsPlayer {
			if int(t.Player) < len(e.G.Players) && !e.G.Players[t.Player].Lost {
				legal = append(legal, t)
			}
			continue
		}
		if o := e.G.Obj(t.Obj); o != nil && o.Zone == state.ZBattlefield &&
			effects.MatchesSpec(e.G, spec, t.Obj, you) {
			legal = append(legal, t)
		}
	}
	return legal
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
