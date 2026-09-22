package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
)

// This file holds the three planechase effect verbs Forge's card corpus uses
// on plane cards and the "Will of the Planeswalkers" cycle (Path of the
// Pyromancer/Animist/Schemer/Enigma/Ghosthunter). This build has no planar
// deck and no planar zone (see the DigUntil row in AGENTS.md: "only Library
// is a real zone — the PlanarDeck carriers scan NO zone"), so the two real
// verbs degrade to a recorded no-op and the display spacer to nothing at all.
// Each is a genuine registration rather than a RegisterNonAPI marker: they
// are DB$ APIs the resolver dispatches, not keywords/triggers/statics.

func init() {
	Register("BlankLine", effBlankLine)
	Register("Planeswalk", effPlaneswalk)
	Register("ChaosEnsues", effChaosEnsues)
}

// effBlankLine is "DB$ BlankLine": a display-only spacer in Forge's card
// scripts. It resolves to nothing and has no rules meaning. It exists as a
// registered primitive only so the resolver does not emit its generic
// "unimplemented API BlankLine" Note while walking a chain that passes
// through it — the Path cycle's DBSpace SVar is a BlankLine whose
// SpellDescription$ is the literal artifact ",,,,,,". Deliberately SILENT:
// emitting a Note for a rules-free spacer would be noise in every log that
// carries one, and the point of the primitive is to disappear.
func effBlankLine(_ Host, _ *Ctx, _ *cards.SA) {}

// effPlaneswalk is CR 901.8's planeswalk action: move to the next plane of
// the planar deck and trigger "when you planeswalk away". A game with no
// planar deck has no next plane and no planeswalk-away trigger, so the
// correct degrade is a recorded no-op — the Note says the action resolved
// and why nothing moved. It keeps a defined$ planeswalk from wedging a
// resolution, and it is loud enough that a future planar-deck tier can find
// every call site by Text.
func effPlaneswalk(h Host, c *Ctx, sa *cards.SA) {
	// Optional$ True is the "you may planeswalk" election.  There is no
	// planar deck in this build, but the election still matters: it must be
	// visible to a host and a decline must still let Resolve walk the chained
	// SubAbility.  The answer is scoped to this SA and consumed on re-entry.
	if strings.EqualFold(strings.TrimSpace(sa.Params["Optional"]), "True") {
		answer := c.PlaneswalkOpt
		c.PlaneswalkOpt = ""
		if answer == "" {
			d := &decision.Decision{Player: c.Controller, Kind: decision.KChoose, Min: 1, Max: 1,
				Source: c.Source, ResumeKind: "planeswalk_optional", ResumeSA: sa,
				Prompt: "Planeswalk?", Options: []decision.Option{
					{Index: 0, Kind: "yes", Label: "Yes — planeswalk", Player: c.Controller},
					{Index: 1, Kind: "no", Label: "No", Player: c.Controller},
				}}
			if Ask(h, d) == AskAsked {
				return
			}
			// R-9: a host without a decision channel deterministically declines.
			answer = "no"
		}
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
			Text: "planeswalk election: " + answer})
		if answer != "yes" {
			return
		}
	}
	notePlanechaseNoDeck(h, c, "planeswalk")
}

// effChaosEnsues is CR 901.9's chaos action: when the planar die rolls the
// chaos symbol, the current plane's chaos ability triggers. With no planar
// deck there is no current plane and no chaos ability to trigger, so this
// records the same recorded no-op as Planeswalk. ChaosEnsues$'s own
// Defined$/Remembered$ riders are unread for the same reason: there is no
// plane object for them to name.
func effChaosEnsues(h Host, c *Ctx, _ *cards.SA) {
	notePlanechaseNoDeck(h, c, "chaos ensues")
}

// notePlanechaseNoDeck records the shared "resolved, but there is no planar
// deck" Note. The Optional$ rider some corpus carriers spell
// (tardis, start_the_tardis) is deliberately not consulted: an optional
// planechase action with nowhere to go is declined either way, and a
// deterministic decline needs no ask.
func notePlanechaseNoDeck(h Host, c *Ctx, action string) {
	text := action + " (no planar deck)"
	if c != nil && c.Source != 0 {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Text: text})
		return
	}
	h.Emit(events.Event{Kind: events.Note, Text: text})
}
