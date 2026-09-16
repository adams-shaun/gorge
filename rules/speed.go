package rules

import (
	"sort"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// speed.go implements the "Start your engines!" mechanic (CR 702.163; 47
// corpus files carry the keyword). Speed is a per-seat count
// (state.Player.Speed) that never resets; max speed is 4. Two engine rules:
//
//   - CR 702.163a: while a player controls a permanent with "Start your
//     engines!", if they have no speed (0), it becomes 1. Emitted from the
//     one emit path when such a permanent ENTERS the battlefield under them
//     (checkSpeedStart, called from Engine.emit's MoveZone fold).
//   - CR 702.163b: a player's speed increases once each turn when an
//     opponent loses life, up to max speed -- including another player's
//     turn. A player with NO speed (0 -- no Start your engines! permanent
//     yet) does not gain: the increase rule is speed's own, and it applies
//     to players who have speed. Emitted after the folded LifeChange loss
//     (checkSpeedGain).
//
// The once-per-turn gate is derived from the event log (the SpeedChange
// events already emitted this turn), never from a live counter, so a replay
// re-executes to the same gains. Max speed is a cap: a seat at 4 gains
// nothing further, and events.Apply clamps defensively too.

// maxSpeed is CR 702.163b: speed cannot go above 4.
const maxSpeed = 4

// checkSpeedGain is called from Engine.emit after every folded LifeChange
// with a negative Amount (a loss) and after every positive player-D Damage
// event (combat and spell/ability damage fold straight to the life total,
// and a Damage event that reaches emit has already been through prevention
// -- a prevented hit is a Note, never a Damage -- so a positive player-arm
// Damage event IS a landed loss). Every living player WITH speed gains one
// speed when an opponent of theirs lost the life (any other seat -- CR
// 800.4k: in a free-for-all every other player is an opponent); at most one
// gain per turn, and never past max speed. More than one player may qualify
// for the same loss on another player's turn. The gain itself is an ordinary
// event (re-entrant emit: a SpeedChange triggers nothing).
func (e *Engine) checkSpeedGain(ev events.Event) {
	if e.G.Over {
		return
	}
	loser := ev.Player
	for _, p := range e.G.AliveFrom(0) {
		if p == loser {
			// The active player losing their own life is not "an opponent
			// loses life".
			continue
		}
		if e.G.Players[p].Speed == 0 {
			// CR 702.163a: a player with NO speed (no Start your engines!
			// permanent yet) does not take the increase; the increase rule
			// governs speed a player HAS.
			continue
		}
		if e.G.Players[p].Speed >= maxSpeed {
			continue
		}
		if e.speedGainedThisTurn(p) {
			continue
		}
		e.emit(events.Event{Kind: events.SpeedChange, Player: p, Amount: 1,
			Text: "speed"})
	}
}

// checkSpeedStart is called from Engine.emit after a battlefield-entry Move
// whose object carries kw:Start your engines: if its controller has no speed,
// it becomes 1 (CR 702.163a). A controller at speed 1..4 is untouched.
func (e *Engine) checkSpeedStart(id state.ObjID) {
	if e.G.Over {
		return
	}
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZBattlefield || o.Face() == nil {
		return
	}
	if !e.HasKeyword(id, "Start your engines") {
		return
	}
	p := o.Controller
	if int(p) >= len(e.G.Players) || e.G.Players[p].Lost || e.G.Players[p].Speed != 0 {
		return
	}
	e.emit(events.Event{Kind: events.SpeedChange, Player: p, Amount: 1,
		Text: "start your engines"})
}

// speedGainedThisTurn reports whether p already took the TURN'S INCREASE
// since the turn began: the latest TurnChange in the log bounds the scan,
// exactly the way castThisTurn and drawsThisTurn derive their per-turn
// counts. Only gains with Text "speed" count: the "start your engines"
// grant (CR 702.163a, speed starting at 1 when a first Start your engines!
// permanent enters) is a distinct rule from the turn's increase (CR
// 702.163b), so the turn the first permanent enters still earns its
// increase when an opponent loses life -- two events, both SpeedChange,
// distinguished by the Text the two emit sites write.
func (e *Engine) speedGainedThisTurn(p state.PlayerID) bool {
	for i := len(e.L.Events) - 1; i >= 0; i-- {
		ev := e.L.Events[i]
		if ev.Kind == events.TurnChange {
			return false
		}
		if ev.Kind == events.SpeedChange && ev.Player == p && ev.Amount > 0 && ev.Text == "speed" {
			return true
		}
	}
	return false
}

// maxSpeedAbilities collects the abilities a max-speed static (CR 702.163c,
// "Max speed — ...") grants a permanent whose controller HAS max speed. The
// shape is `S:Mode$ Continuous | Affected$ Card.Self | Condition$ MaxSpeed |
// AddAbility$ <svar>` (Amonkhet Raceway): the granted ability is the SVar the
// AddAbility$ names, an ordinary AB$ line read off the face's SVar table. Any
// other Condition$ value (or none) contributes nothing: a condition this
// build cannot evaluate is not silently treated as always-on.
func (e *Engine) maxSpeedAbilities(p state.PlayerID, id state.ObjID) []*cards.SA {
	if e.G.Players[p].Speed < maxSpeed {
		return nil
	}
	o := e.G.Obj(id)
	if o == nil {
		return nil
	}
	f := o.Face()
	if f == nil {
		return nil
	}
	var out []*cards.SA
	for _, st := range f.Statics {
		if st.Mode != "Continuous" || st.Params["AddAbility"] == "" {
			continue
		}
		if st.Params["Condition"] != "MaxSpeed" {
			continue
		}
		if ab := cards.ResolveSVar(f.SVars, st.Params["AddAbility"]); ab != nil && ab.Kind == "AB" {
			out = append(out, ab)
		}
	}
	return out
}

// beginGrantedActivation activates an SVar-anchored granted ability: the
// max-speed static's "granted" option (rules/speed.go's own offer) and the
// AddAbilities grant's "ability" option (rules/legal.go, beginActivation's
// SVar branch) both route here. It pays the granted ability's own Cost$
// (mana through the ordinary payMana, the T part a Tap), mints the ability
// object via the delayed-shape DelayedPush (whose Counter carries the SVar
// name -- the one existing event whose Apply case can resolve an arbitrary
// SVar-named ability onto the stack; the ^uint32(0) registration id matches
// nothing, so nothing is removed), and asks the ability's targets exactly
// the way a trigger's post-push ask does. A stale option degrades to a
// no-op.
func (e *Engine) beginGrantedActivation(p state.PlayerID, opt decision.Option) {
	o := e.G.Obj(opt.Obj)
	if o == nil || o.Zone != state.ZBattlefield || o.Face() == nil {
		return
	}
	f := o.Face()
	ab := cards.ResolveSVar(f.SVars, opt.SVar)
	if ab == nil || ab.Kind != "AB" {
		return
	}
	cost := e.offerCostFor(p, opt.Obj, ParseCost(ab.Params["Cost"]), true)
	if !cost.payable(e.G.Players[p].Pool, e.G.Players[p].Life) {
		return
	}
	if !e.payMana(p, cost) {
		return
	}
	if cost.Life != 0 {
		e.emit(events.Event{Kind: events.LifeChange, Player: p, Amount: -cost.Life})
	}
	if cost.Tap {
		e.emit(events.Event{Kind: events.Tap, Obj: opt.Obj})
	}
	e.emit(events.Event{Kind: events.DelayedPush, Player: p, Obj: opt.Obj,
		Amount: -1, Counter: opt.SVar, Text: "granted ability"})
	if len(e.G.Stack) > 0 {
		id := e.G.Stack[len(e.G.Stack)-1]
		if so := e.G.Obj(id); so != nil && so.Ability != nil && so.Ability.Line == ab.Line && ab.Params["ValidTgts"] != "" {
			// Line, not pointer identity: the mint re-parsed the SVar body into
			// its own *cards.SA, so the freshly minted object carries a different
			// pointer than this function's ab even though both parsed the same
			// line. Line is the identity the two parses share.
			e.askTarget(p, id, ab)
		}
	}
}

// abSVarName returns the SVar table key whose raw body is exactly the line
// ab was parsed from, in first-match order over the face's SVar table. The
// table is a Go map, so this walk sorts the keys first (determinism rule: no
// map range may reach an option list) -- and the corpus is the guarantee
// there IS an answer: every AddAbility$ value names a key whose body is
// exactly that AB.
func abSVarName(f *cards.Face, ab *cards.SA) string {
	keys := make([]string, 0, len(f.SVars))
	for k := range f.SVars {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if f.SVars[k] == ab.Line {
			return k
		}
	}
	return ""
}
