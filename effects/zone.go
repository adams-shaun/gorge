package effects

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func init() {
	Register("ChangeZone", effChangeZone)
	Register("ChangeZoneAll", effChangeZoneAll)
	Register("Destroy", effDestroy)
	Register("DestroyAll", effDestroyAll)
	Register("Sacrifice", effSacrifice)
}

// ParseZone maps a Forge zone name to a state.Zone. Unknown names resolve to
// the graveyard, which is where the overwhelming majority of movement goes
// and is a safe default for an unmodelled destination.
func ParseZone(s string) state.Zone {
	switch strings.TrimSpace(s) {
	case "Hand":
		return state.ZHand
	case "Battlefield":
		return state.ZBattlefield
	case "Library":
		return state.ZLibrary
	case "Exile":
		return state.ZExile
	case "Stack":
		return state.ZStack
	case "Command":
		return state.ZCommand
	case "Ceased":
		return state.ZCeased
	}
	return state.ZGraveyard
}

func effChangeZone(h Host, c *Ctx, sa *cards.SA) {
	to := ParseZone(sa.Params["Destination"])
	// WithCountersType$/WithCountersAmount$ make the move put counters on the
	// permanent it lands on the battlefield with -- the Undying expansion's
	// "return to the battlefield with a +1/+1 counter" (cards/keywords.go). The
	// CounterChange is emitted AFTER the MoveZone, so it lands on the moved
	// (new) object's back at its destination, exactly as Move waiting to run
	// first would want, and the counter survives onto the permanent because it
	// is added post-move. Counter (not the Move carrying it along) is what
	// keeps events/apply.go's Move from knowing anything about counters.
	withKind := sa.Params["WithCountersType"]
	withAmt := int32(1)
	if v := strings.TrimSpace(sa.Params["WithCountersAmount"]); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			// Malformed WithCountersAmount must be loud, not silently default to
			// 1 (the reviewer's item): a wrong counter count on a Returning
			// permanent is a hard-to-spot board-shape bug. A Note event (the way
			// Resolve surfaces an unimplemented API) keeps this deterministic and
			// replay-log-visible rather than dropping to a log line the event log
			// cannot account for. The movement still proceeds with the safe
			// default 1.
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "malformed WithCountersAmount " + v})
		} else {
			withAmt = int32(n)
		}
	}
	for _, t := range Defined(h, c, sa) {
		if t.IsPlayer {
			continue
		}
		o := h.Game().Obj(t.Obj)
		if o == nil {
			continue
		}
		// Origin$, when given, is a precondition: the object must actually be
		// where the script expects, or the movement does not happen. This is
		// also this build's only CR 608.2b guard for ChangeZone: a target
		// moved away by an earlier effect in the same resolution, or by a
		// response that has already resolved, is simply skipped rather than
		// moved a second time or moved from the wrong zone.
		if from, ok := sa.Params["Origin"]; ok && o.Zone != ParseZone(from) {
			continue
		}
		h.Emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: o.Zone, To: to})
		// RememberChanged$ True (Forge's spelling on the ChangeZone in the
		// Flickerwisp delayed-trigger family): the moved object joins the
		// ability's Remembered, so a DelayedTrigger that runs as a later
		// SubAbility of this same chain captures it (TrigBounce's Defined$
		// DelayTriggerRememberedLKI resolves against it when the delayed
		// trigger fires). The value is a parameter of the ongoing resolution
		// (Ctx), not game state, so mutating it here is fine -- the recall
		// is persisted into the DelayedRegister event, not written to state
		// directly.
		if strings.EqualFold(sa.Params["RememberChanged"], "True") {
			c.Remembered = append(c.Remembered, state.Target{Obj: o.ID})
		}
		if withKind != "" && to == state.ZBattlefield {
			h.Emit(events.Event{Kind: events.CounterChange, Obj: o.ID, Counter: withKind, Amount: withAmt})
		}
	}
}

func effChangeZoneAll(h Host, c *Ctx, sa *cards.SA) {
	from, to := ParseZone(sa.Params["Origin"]), ParseZone(sa.Params["Destination"])
	spec := sa.Params["ChangeType"]
	if spec == "" {
		spec = "Card"
	}
	g := h.Game()
	for _, p := range g.AliveFrom(0) {
		// Snapshot the zone: emitting move events mutates it underneath us.
		ids := append([]state.ObjID(nil), g.Zone(from, p)...)
		for _, id := range ids {
			if MatchesSpecFrom(g, spec, id, c.Controller, c.Source) {
				h.Emit(events.Event{Kind: events.MoveZone, Obj: id, From: from, To: to})
			}
		}
	}
}

// effDestroy is a single-target removal effect: exactly the shape CR 608.2b
// target rechecking exists for. Today the only recheck is "does the target
// still exist, and is it still on the battlefield" -- a target that stayed on
// the battlefield but became newly ineligible some other way (e.g. it gained
// Indestructible in response, or protection from the source) between
// targeting and resolution is not rechecked. See the Task 18 report.
func effDestroy(h Host, c *Ctx, sa *cards.SA) {
	for _, t := range Defined(h, c, sa) {
		o := h.Game().Obj(t.Obj)
		if t.IsPlayer || o == nil || o.Zone != state.ZBattlefield {
			continue
		}
		if h.HasKeyword(o.ID, "Indestructible") {
			continue
		}
		// NoRegen$ is compared against "True", not against empty: an explicit
		// NoRegen$ False PERMITS regeneration, and reading it as "set, so
		// suppress" would invert the card. The corpus splits 144 True / 1
		// False (creepy_doll.txt), and that one is unreachable today because
		// cards/link.go auto-links only SubAbility$, not the WinSubAbility$ it
		// hangs off -- so this is correctness insurance for when that changes,
		// not a live fix.
		if sa.Params["NoRegen"] != "True" && ReplaceDestruction(h, o.ID) {
			continue
		}
		h.Emit(events.Event{Kind: events.MoveZone, Obj: o.ID,
			From: state.ZBattlefield, To: state.ZGraveyard, Text: "destroyed"})
	}
}

func effDestroyAll(h Host, c *Ctx, sa *cards.SA) {
	spec := sa.Params["ValidCards"]
	if spec == "" {
		spec = "Permanent"
	}
	g := h.Game()
	for _, p := range g.AliveFrom(0) {
		ids := append([]state.ObjID(nil), g.Zone(state.ZBattlefield, p)...)
		for _, id := range ids {
			if h.HasKeyword(id, "Indestructible") {
				continue
			}
			if MatchesSpecFrom(g, spec, id, c.Controller, c.Source) {
				// NoRegen$ != "True", not == "": see effDestroy above.
				if sa.Params["NoRegen"] != "True" && ReplaceDestruction(h, id) {
					continue
				}
				h.Emit(events.Event{Kind: events.MoveZone, Obj: id,
					From: state.ZBattlefield, To: state.ZGraveyard, Text: "destroyed"})
			}
		}
	}
}

// effSacrifice moves permanents to the graveyard. Sacrifice ignores
// Indestructible: sacrificing is not destruction (CR 701.16), so no
// HasKeyword/Indestructible gate and no ReplaceDestruction/regeneration
// consultation -- a regenerated creature does not survive being sacrificed.
// Same CR 608.2b caveat as effDestroy: only existence-and-zone is rechecked.
func effSacrifice(h Host, c *Ctx, sa *cards.SA) {
	g := h.Game()
	// SacValid$ narrows WHAT may be sacrificed ("Creature.nonToken",
	// "Artifact"). With no SacValid$ at all the default is "Permanent" (any
	// permanent). The older justification -- that the self-sacrifice and
	// at-end-of-step lines need "Permanent" because the object they sacrifice
	// may be an artifact, a land or a creature -- is empirically false: those
	// lines carry no Defined$ and no ValidTgts$, so Defined() resolves them
	// to the SOURCE object (effects/context.go) and they take the object-target
	// path below, where spec is never consulted at all. Measured at the corpus
	// pin (for the command, see the sc1b report): of 892 Sacrifice SAs, 328
	// carry no SacValid$; 327 of those resolve to an object (or an inherited
	// target) and never reach the default, and exactly one -- Expert-Level
	// Safe's DB$ Sacrifice | Defined$ You | ValidCard$ Card.Self -- reaches it.
	// So "Permanent" is a harmless default rather than a correct reading of
	// the corpus, and no player-targeted line carries SacValid$ Self. (That one
	// reachable line means its controller hands over whichever permanent sits
	// first in zone order -- for Expert-Level Safe, "this artifact" -- instead
	// of the no-op before this fix; see AGENTS.md.)
	spec := sa.Params["SacValid"]
	if spec == "" {
		spec = "Permanent"
	}
	for _, t := range Defined(h, c, sa) {
		if t.IsPlayer {
			// Bounds guard: g.Zone indexes g.zones[zoneIndex(z, p)] and
			// zoneIndex has no bounds check, so an out-of-range target-supplied
			// player id would panic with "index out of range" and halt the
			// table. Player targets normally come from askTarget or AliveFrom
			// and are bounded, but the package's idiom (see cardflow.go and
			// count.go) is not to trust a target blindly.
			if int(t.Player) >= len(g.Players) {
				continue
			}
			// A sacrifice aimed at a player: that player sacrifices one
			// matching permanent. Real Magic has the player choose; this
			// engine does not ask (the mid-resolution ask machinery is being
			// reworked elsewhere), so the stand-in is deterministic and
			// replay-stable: the first permanent in battlefield order that
			// satisfies SacValid$. "You" in the spec is the sacrificing
			// player, since they choose from their own permanents.
			ids := append([]state.ObjID(nil), g.Zone(state.ZBattlefield, t.Player)...)
			for _, id := range ids {
				if MatchesSpecFrom(g, spec, id, t.Player, c.Source) {
					h.Emit(events.Event{Kind: events.MoveZone, Obj: id,
						From: state.ZBattlefield, To: state.ZGraveyard, Text: "sacrificed"})
					break
				}
			}
			continue
		}
		o := g.Obj(t.Obj)
		if o == nil || o.Zone != state.ZBattlefield {
			continue
		}
		// A specific object target is sacrificed as-is: the choice of which
		// object was already made by the effect's targeting, so SacValid$'
		// "which one may be sacrificed" step does not re-filter a concrete
		// object (and would misfire on the corpus's SacValid$ Self lines,
		// where "Self" is not a type the filter grammar knows).
		h.Emit(events.Event{Kind: events.MoveZone, Obj: o.ID,
			From: state.ZBattlefield, To: state.ZGraveyard, Text: "sacrificed"})
	}
}
