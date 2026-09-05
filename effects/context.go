package effects

import (
	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/state"
)

// Defined resolves a Defined$ parameter to concrete targets. With no Defined$
// at all, Forge's own rule applies: an ability that declares ValidTgts$ (it
// has real targets to name) acts on the chosen ones; an ability with no
// ValidTgts$ acts on its own source (R-10's default).
//
// Every return here is a defensive copy, never a slice sharing a backing
// array with Ctx.Targets or Ctx.Remembered: Ctx is threaded by pointer through
// Resolve, so a caller that filters the returned slice in place (the ordinary
// out := s[:0]; for range append(out, ...) idiom) must not be able to corrupt
// state a later effect in the same Sub chain still relies on.
func Defined(h Host, c *Ctx, sa *cards.SA) []state.Target {
	g := h.Game()
	switch sa.Params["Defined"] {
	case "":
		// Forge's rule: an ability that names targets acts on them; one that
		// names none acts on its source. A sub-ability that wants its
		// parent's targets says so explicitly (Defined$ Targeted /
		// ParentTarget), which every script in the corpus does.
		if _, targeted := sa.Params["ValidTgts"]; targeted {
			return copyTargets(c.Targets)
		}
		return []state.Target{{Obj: c.Source}}
	case "You":
		return []state.Target{{Player: c.Controller, IsPlayer: true}}
	case "Self", "Parent":
		return []state.Target{{Obj: c.Source}}
	case "Remembered":
		return copyTargets(c.Remembered)
	case "Targeted", "ParentTarget":
		return copyTargets(c.Targets)
	case "TriggeredCard", "TriggeredCardLKICopy", "TriggeredNewCardLKICopy",
		"TriggeredSpellAbility", "TriggeredAttacker", "TriggeredSource":
		// M1 does not model LKI copies, new-object identity or the
		// ability-vs-card distinction separately: every one of these forms
		// names the same Remembered object entry a trigger captured.
		return objectsOf(c.Remembered)
	case "ReplacedCard":
		// The card a replacement is acting on (Rest in Peace shape: the R: line
		// intercepts a "would go to the graveyard" Move, ReplaceWith$ needs to
		// name the object the replaced event was about). "Replaced" is set only
		// on a replacement's own context, so outside a replacement -- and for a
		// replaced object that has since ceased to exist -- Defined falls back to
		// nil (nothing to act on) rather than the chosen targets.
		if c.Replaced != 0 && g.Obj(c.Replaced) != nil {
			return []state.Target{{Obj: c.Replaced}}
		}
		return nil
	case "TriggeredDefendingPlayer", "TriggeredPlayer":
		return playersOf(c.Remembered)
	case "TriggeredCardController":
		for _, t := range c.Remembered {
			if !t.IsPlayer {
				if o := g.Obj(t.Obj); o != nil {
					return []state.Target{{Player: o.Controller, IsPlayer: true}}
				}
			}
		}
		return nil
	case "Equipped", "Enchanted", "AttachedTo":
		// The corpus spells this three ways depending on whether the source
		// is Equipment, an Aura, or a generic script; all three name the
		// same field (Task 14 wires its producer).
		if o := g.Obj(c.Source); o != nil && o.AttachedTo != 0 && g.Obj(o.AttachedTo) != nil {
			return []state.Target{{Obj: o.AttachedTo}}
		}
		return nil
	case "Opponent":
		var out []state.Target
		for _, p := range g.AliveFrom(c.Controller) {
			if p != c.Controller {
				out = append(out, state.Target{Player: p, IsPlayer: true})
			}
		}
		return out
	case "Player":
		var out []state.Target
		for _, p := range g.AliveFrom(c.Controller) {
			out = append(out, state.Target{Player: p, IsPlayer: true})
		}
		return out
	}
	// Any Defined$ form M1 does not model falls back to the chosen targets
	// rather than silently acting on nothing.
	return copyTargets(c.Targets)
}

// objectsOf returns Remembered's object entries (IsPlayer false) as a fresh
// slice -- never aliasing Ctx.Remembered, for the reason copyTargets' own
// doc comment gives.
func objectsOf(ts []state.Target) []state.Target {
	var out []state.Target
	for _, t := range ts {
		if !t.IsPlayer {
			out = append(out, t)
		}
	}
	return out
}

// playersOf returns Remembered's player entries (IsPlayer true) as a fresh
// slice.
func playersOf(ts []state.Target) []state.Target {
	var out []state.Target
	for _, t := range ts {
		if t.IsPlayer {
			out = append(out, t)
		}
	}
	return out
}

// copyTargets returns a defensive copy of s: same elements, independent
// backing array. A nil s yields nil, not an empty-but-non-nil slice, so
// Defined's observable results are unchanged for every input — only the
// aliasing is fixed.
func copyTargets(s []state.Target) []state.Target {
	return append([]state.Target(nil), s...)
}

// PlayerOf resolves a target to a player: an explicit player target, or the
// controller of a targeted object.
func PlayerOf(h Host, c *Ctx, t state.Target) state.PlayerID {
	if t.IsPlayer {
		return t.Player
	}
	if o := h.Game().Obj(t.Obj); o != nil {
		return o.Controller
	}
	return c.Controller
}
