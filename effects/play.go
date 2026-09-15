package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func init() { Register("Play", effPlay) }

// effPlay implements a "play a card from a zone without paying its mana
// cost" effect (Forge's Play API; CR 601.2/117.3a's "play" verb covers
// casting a spell or playing a land from a non-hand zone). The candidates are
// gathered the same way the rest of the engine gathers a Defined$/filter
// population:
//
//   - Defined$ (ExiledWith, Remembered, Targeted, ...) names the card(s)
//     directly and is resolved through context.go's Defined.
//   - Valid$ / ValidZone$ name a filter and a zone to scan (e.g. Spinerock
//     Knoll's Valid$ Card.ExiledWithSource + ValidZone$ Exile).
//   - ValidTgts$ means the ability already targeted a card at placement, so
//     the recorded target is the population.
//
// Whichever population results, the effect poses a KModes "play" choice over
// the candidate cards (or a yes/no when exactly one is offered), and the
// answered choice is carried back through Ctx.Play so rules' resumeResolution
// can begin a zero-cost cast of the chosen card from its current zone. The
// WithoutManaCost$ semantics (cast the card for free) are applied by the cast
// flow, not here, because mana is paid in rules where the cost grammar lives.
func effPlay(h Host, c *Ctx, sa *cards.SA) {
	if c.PlayDone && c.Play != 0 {
		// Re-entry after the answer: rules' resumeResolution has already begun
		// the cast of c.Play (the "play" resume arm calls beginCast), so there
		// is nothing for this effect to do but let the suspension finish.
		c.PlayDone = false
		c.Play = 0
		return
	}
	var candidates []state.ObjID
	g := h.Game()

	// A targeted Play (ValidTgts$ on the same SA, e.g. Conduit of Worlds)
	// plays the card it targeted at placement.
	if _, ok := sa.Params["ValidTgts"]; ok {
		for _, t := range c.Targets {
			candidates = append(candidates, t.Obj)
		}
	} else if spec, ok := sa.Params["Defined"]; ok && strings.TrimSpace(spec) != "" {
		// Population by Defined$ (ExiledWith / Remembered / ...).
		dd := &cards.SA{Params: map[string]string{"Defined": spec}}
		for _, t := range Defined(h, c, dd) {
			candidates = append(candidates, t.Obj)
		}
	} else {
		// Population by Valid$ + ValidZone$.
		valid := strings.TrimSpace(sa.Params["Valid"])
		if valid == "" {
			valid = "Card"
		}
		var zones []state.Zone
		if z := strings.TrimSpace(sa.Params["ValidZone"]); z != "" {
			for _, part := range strings.Split(z, ",") {
				if zn, ok := ZoneFromString(strings.TrimSpace(part)); ok {
					zones = append(zones, zn)
				}
			}
		}
		if len(zones) == 0 {
			return
		}
		for _, zn := range zones {
			for _, id := range g.Zone(zn, c.Controller) {
				if MatchesSpecFrom(g, valid, id, c.Controller, c.Source) {
					candidates = append(candidates, id)
				}
			}
		}
	}

	// Dedupe while preserving order.
	seen := map[state.ObjID]bool{}
	var uniq []state.ObjID
	for _, id := range candidates {
		if id != 0 && !seen[id] {
			seen[id] = true
			uniq = append(uniq, id)
		}
	}
	candidates = uniq

	d := &decision.Decision{Player: c.Controller, Kind: decision.KModes,
		Min: 1, Max: 1, Source: c.Source, ResumeKind: "play",
		ResumeSA: sa, Prompt: "Play a card from this zone"}
	for _, id := range candidates {
		label := "Play it"
		if o := g.Obj(id); o != nil && o.Face() != nil {
			label = "Play " + o.Face().Name
		}
		d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "mode",
			Label: label, Obj: id, Player: c.Controller})
	}
	if len(d.Options) == 0 {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "Play found no card to play"})
		return
	}
	if h.Ask(d) {
		return // resolution suspended; the answer re-enters rules' "play" arm.
	}
	// Fuzz/no-engine host: play the first candidate deterministically (R-9).
	h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
		Text: "Play chose the first candidate (no engine host to ask)"})
	c.Play = candidates[0]
}

// ZoneFromString maps a Forge zone name to a state.Zone. Only the zones a
// Play effect actually scans are spelled out; ok is false (and z is zero,
// state.ZLibrary) for an unrecognised name so the caller scans nothing rather
// than scanning every zone.
func ZoneFromString(s string) (state.Zone, bool) {
	switch s {
	case "Exile":
		return state.ZExile, true
	case "Graveyard":
		return state.ZGraveyard, true
	case "Hand":
		return state.ZHand, true
	case "Library":
		return state.ZLibrary, true
	case "Battlefield":
		return state.ZBattlefield, true
	}
	return 0, false
}
