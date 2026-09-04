package events

import "github.com/adams-shaun/gorge/state"

// Emit is the engine's only mutation path: append to the log, then fold into
// state. Replay calls Apply directly with logged events, so post-replay state
// equals post-play state by construction.
func Emit(g *state.Game, l *Log, e Event) Event {
	stored := l.Append(e)
	Apply(g, stored)
	return stored
}

// Apply folds one event into state. It must stay a pure function of (g, e):
// no randomness, no clock, no reads outside g.
func Apply(g *state.Game, e Event) {
	switch e.Kind {
	case GameStart, DecisionAsk, DecisionMade, Note, Resolve:
		// Markers. Resolve is deliberately inert: the resolving object leaves
		// the stack through its own MoveZone event, and popping here as well
		// would drop a second object.

	case Shuffle:
		if validPlayer(g, e.Player) {
			g.SetZone(state.ZLibrary, e.Player, append([]state.ObjID(nil), e.IDs...))
		}

	case MoveZone, Draw, PutOnStack:
		Move(g, e.Obj, e.From, e.To)

	case LifeChange:
		if validPlayer(g, e.Player) {
			g.Players[e.Player].Life += e.Amount
		}

	case Damage:
		if o := g.Obj(e.Obj); o != nil {
			o.Damage += e.Amount
			if o.Damage < 0 {
				o.Damage = 0
			}
		} else if validPlayer(g, e.Player) {
			g.Players[e.Player].Life -= e.Amount
		}

	case Tap:
		if o := g.Obj(e.Obj); o != nil {
			o.Tapped = true
		}
	case Untap:
		if o := g.Obj(e.Obj); o != nil {
			o.Tapped = false
		}

	case StepChange:
		g.Step = e.Step

	case TurnChange:
		if validPlayer(g, e.Player) {
			g.Turn = e.Amount
			g.Active = e.Player
			g.Players[e.Player].LandsPlayed = 0
			// g.Zone(ZBattlefield, e.Player) can only ever hold IDs that Move
			// already confirmed are real objects, so this nil check is
			// currently unreachable in practice -- but it is one line, it
			// matches every other zone-walk in this switch (DeclareAttackers,
			// DeclareBlockers) that guards g.Obj before dereferencing, and it
			// stops that invariant from becoming a silent, easy-to-reopen
			// panic if a future Kind ever populates a zone list some other
			// way. Found in the same audit as Ruling T20-e.
			for _, id := range g.Zone(state.ZBattlefield, e.Player) {
				if o := g.Obj(id); o != nil {
					o.SummonSick = false
				}
			}
		}

	case Priority:
		if validPlayer(g, e.Player) {
			g.Priority = e.Player
			passes := e.Amount
			if passes < 0 {
				passes = 0
			}
			g.Passes = passes
		}

	case ManaAdd:
		if validPlayer(g, e.Player) {
			idx := state.MC
			if e.Counter != "" {
				idx = state.ManaIndex(e.Counter[0])
			}
			g.Players[e.Player].Pool[idx] += e.Amount
		}

	case ManaClear:
		if validPlayer(g, e.Player) {
			g.Players[e.Player].Pool = state.Mana{}
		}

	case CounterChange:
		if o := g.Obj(e.Obj); o != nil {
			o.AddCounter(e.Counter, e.Amount)
		}

	case DeclareAttackers:
		// e.Player names the attacking player for every ID in this event, so
		// it is validated once, like TurnChange/Priority above, rather than
		// per object. Nothing reads Object.Attacking yet, so an unvalidated
		// value cannot panic today -- but it is the same untrusted-seat-id
		// pattern as Ruling T20-e, found in the same audit, and closing it
		// now costs nothing for a well-formed event (Player is always valid
		// there).
		if !validPlayer(g, e.Player) {
			break
		}
		for _, id := range e.IDs {
			if o := g.Obj(id); o != nil {
				o.IsAttacking = true
				o.Attacking = e.Player
			}
		}

	case DeclareBlockers:
		for _, pr := range e.Pairs {
			a := g.Obj(pr[0])
			if a == nil || g.Obj(pr[1]) == nil {
				continue
			}
			a.BlockedBy = append(a.BlockedBy, pr[1])
		}

	case PlayerLost:
		if validPlayer(g, e.Player) {
			g.Players[e.Player].Lost = true
		}

	case GameOver:
		// Ruling T22-g (fix round 1): the first GameOver wins; a later one
		// on an already-finished game is a no-op. Without this guard, a log
		// carrying two GameOver events (a duplicate, a replay quirk, a
		// tampered log) produced a game simultaneously won (Winner set by
		// the first) and drawn (Draw set by the second) -- Over alone
		// cannot express "this event doesn't apply", so the guard has to
		// live here, ahead of everything else in this case.
		if g.Over {
			break
		}
		// Ruling T22-a (Amount is the draw/winner discriminator, the same
		// trick TargetsChosen already uses to tell its two target shapes
		// apart), pinned down fully by T22-g: exactly two shapes are
		// defined, and everything else is not a GameOver this build
		// recognizes at all.
		//   Amount == 0: a win. Winner is set when Player validates; a
		//     malformed Player still ends the game (Over), just names no
		//     winner -- Winner keeps its zero value rather than reading as
		//     "seat 0 won" (see the untouched-on-a-bad-Player case below).
		//   Amount == 1: CR 104.4a's draw. Winner is irrelevant; Draw says
		//     so explicitly, since PlayerID(0) is both Winner's zero value
		//     and a real seat and so cannot mean "nobody" on its own.
		//   anything else: not a shape this Kind defines. Previously any
		//     other Amount still set Over unconditionally and, since
		//     Winner's own guard only ever checked validPlayer, a tampered
		//     event naming an out-of-range Player under some third Amount
		//     read as "seat 0 won" regardless -- exactly the ambiguity this
		//     whole discriminator exists to remove. Changing nothing at all
		//     is the safe response to a shape this build cannot interpret.
		switch e.Amount {
		case 0:
			g.Over = true
			if validPlayer(g, e.Player) {
				g.Winner = e.Player
			}
		case 1:
			g.Over = true
			g.Draw = true
		}

	case LandPlayed:
		if validPlayer(g, e.Player) {
			g.Players[e.Player].LandsPlayed++
		}

	case TargetsChosen:
		if o := g.Obj(e.Obj); o != nil {
			if e.Amount == 1 {
				if validPlayer(g, e.Player) {
					o.Targets = []state.Target{{Player: e.Player, IsPlayer: true}}
				}
			} else {
				var targets []state.Target
				for _, id := range e.IDs {
					targets = append(targets, state.Target{Obj: id})
				}
				o.Targets = targets
			}
		}

	case FlipFace:
		if o := g.Obj(e.Obj); o != nil && o.Card != nil &&
			e.Amount >= 0 && int(e.Amount) < len(o.Card.Faces) {
			o.FaceIdx = uint8(e.Amount)
		}

	case ClockTick:
		g.Clock++

	case TriggerPush:
		// Ruling T20-a: the ability object is minted here, inside Apply, so
		// a log-only replay creates the exact same object a live game did --
		// not via a direct, unlogged AddObject call from rules.Engine. e.Obj
		// names the permanent whose trigger fired; a permanent that no
		// longer exists, or an out-of-range trigger index (stale data, a
		// tampered log), degrades to a no-op rather than panicking.
		//
		// Ruling T20-e: Player must be checked too, same as every sibling
		// case in this switch that indexes a Player-keyed slice (LifeChange,
		// TurnChange, Priority, ManaAdd, ManaClear). This case skipped it,
		// and an out-of-range Player flows straight into g.AddObject below,
		// then into Move's zoneOwner/SetZone/zoneIndex path, which indexes
		// g.zones (sized numZones*len(g.Players) at NewGame) with no bounds
		// check of its own -- panic: index out of range. Unreachable via
		// ordinary self-play (Player is always sourced from a real object's
		// controller) but directly reachable replaying an external,
		// corrupted, or tampered log -- exactly the case this event exists
		// to support.
		if !validPlayer(g, e.Player) {
			break
		}
		src := g.Obj(e.Obj)
		if src == nil {
			break
		}
		f := src.Face()
		if f == nil || e.Amount < 0 || int(e.Amount) >= len(f.Triggers) {
			break
		}
		o := g.AddObject(nil, e.Player)
		// Move first (it resets Remembered, among other stack-only fields,
		// for anything other than a battlefield destination -- see Move's
		// own default case below), then set what the ability actually
		// carries. Setting these before Move would have them wiped by that
		// same reset; ordering them after is what makes Remembered actually
		// survive onto the stack (Ruling T20-c).
		Move(g, o.ID, state.ZLibrary, state.ZStack)
		o.Ability = f.Triggers[e.Amount].Effect
		o.Source = e.Obj
		for _, id := range e.IDs {
			o.Remembered = append(o.Remembered, state.Target{Obj: id})
		}

	case EndCombatReset:
		// No Player or Obj to validate: this clears every object in the
		// arena unconditionally, the same as ClockTick touches no
		// Player/Obj-indexed field either.
		for i := range g.Objs {
			g.Objs[i].IsAttacking = false
			g.Objs[i].BlockedBy = nil
		}
	}
}

// Move relocates an object between zones, preserving zone order and the
// one-object-one-zone invariant.
//
// The zone an object is removed from is always o.Zone — the object's own
// recorded location — never the caller-supplied from. from (and Event.From
// in the log) exist for the client and for replay to read, but a caller that
// gets it wrong must not be able to leave the object in its real zone while
// also adding it to to: that would put it in two zones at once, and a
// repeat of the same wrong move would duplicate it within one zone.
//
// Moving an object to the zone it is already in is not special-cased: it is
// removed from that zone and appended again, so it ends up at the end of the
// zone's order. That is deterministic and matches every other move.
func Move(g *state.Game, id state.ObjID, from, to state.Zone) {
	o := g.Obj(id)
	if o == nil || !o.Zone.Valid() || !to.Valid() {
		return
	}
	remove(g, id, o.Zone, zoneOwner(o, o.Zone))
	dst := zoneOwner(o, to)
	g.SetZone(to, dst, append(g.Zone(to, dst), id))

	o.Zone = to
	switch to {
	case state.ZBattlefield:
		o.SummonSick = true
		o.Damage = 0
		g.Clock++
		o.Timestamp = g.Clock
	default:
		// Leaving the battlefield or the stack resets everything that only
		// exists while a permanent or spell is in play.
		o.Tapped = false
		o.Damage = 0
		o.IsAttacking = false
		o.BlockedBy = nil
		o.Counters = nil
		o.Targets = nil
		o.Remembered = nil
	}
}

// validPlayer reports whether p indexes an existing seat.
func validPlayer(g *state.Game, p state.PlayerID) bool {
	return int(p) < len(g.Players)
}

// zoneOwner picks whose zone list an object belongs to: the battlefield and the
// stack are keyed by controller, every private zone by owner.
func zoneOwner(o *state.Object, z state.Zone) state.PlayerID {
	if z == state.ZBattlefield || z == state.ZStack {
		return o.Controller
	}
	return o.Owner
}

func remove(g *state.Game, id state.ObjID, z state.Zone, p state.PlayerID) {
	src := g.Zone(z, p)
	for i, x := range src {
		if x == id {
			out := make([]state.ObjID, 0, len(src)-1)
			out = append(out, src[:i]...)
			out = append(out, src[i+1:]...)
			g.SetZone(z, p, out)
			return
		}
	}
}
