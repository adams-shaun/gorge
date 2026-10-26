package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// copyPermanentDifferentNamesPicks resolves api:CopyPermanent's
// WithDifferentNames$ ("choose any number of ... with different names"):
// Forge's CopyPermanentEffect.java:210-221 picks one object at a time
// (chooseSingleEntityForEffect, optional), removes every remaining candidate
// that shares the picked object's name, and repeats until the player stops or
// the pool is empty. XMage's Battle for Bretagard target (a different-names
// TargetPermanent plus one CreateTokenCopyTargetEffect per chosen permanent)
// reads the same. One optional single-pick KChoose per iteration returns the
// chosen objects; the caller's mint loop copies each of them once.
//
// The ask is the same ResumeKind the single-pick Choices$ shape uses: the
// kernel's tape serves the successive answers in order, so no new decision
// kind or resume marker is needed.
func copyPermanentDifferentNamesPicks(h Host, c *Ctx, sa *cards.SA, chooser state.PlayerID, pool []state.Target) []state.Target {
	remaining := make([]state.Target, 0, len(pool))
	for _, t := range pool {
		if !t.IsPlayer && t.Obj != 0 {
			remaining = append(remaining, t)
		}
	}
	var picks []state.Target
	for len(remaining) > 0 {
		d := &decision.Decision{Player: chooser, Kind: decision.KChoose, Min: 0, Max: 1,
			Source: c.Source, ResumeKind: "copypermanent_choice", ResumeSA: sa,
			Prompt: "Choose a permanent with a different name to copy"}
		for _, t := range remaining {
			d.Options = append(d.Options, decision.Option{Index: len(d.Options), Obj: t.Obj, Kind: "permanent"})
		}
		if ans, ok := AskTape(h, d); ok {
			if len(ans) == 0 || ans[0].Obj == 0 {
				break
			}
			chosen := ans[0].Obj
			picked := -1
			for i, t := range remaining {
				if t.Obj == chosen {
					picked = i
					break
				}
			}
			if picked < 0 {
				break
			}
			picks = append(picks, remaining[picked])
			name := copyPermanentDerivedName(h, chosen)
			remaining = dropTargetsNamed(h, remaining, name)
			continue
		}
		if !askUnposable(d) {
			// No engine host served the ask (a legacy/no-host run): the
			// deterministic stand-in copies the first eligible object of each
			// name, in pool order, one loud Note per call.
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: chooser,
				Text: "CopyPermanent WithDifferentNames$ has no engine host; copying the first eligible permanent of each name"})
		}
		seen := make(map[string]bool, len(remaining))
		for _, t := range remaining {
			name := copyPermanentDerivedName(h, t.Obj)
			if seen[name] {
				continue
			}
			seen[name] = true
			picks = append(picks, t)
		}
		break
	}
	return picks
}

// copyPermanentDerivedName is the name WithDifferentNames$ compares: the
// host's layer-derived name (Chars.Name), lowercased and trimmed, so a token
// copy's copied name and a layer-3 rename read exactly the way the rules
// layer walk reads them.
func copyPermanentDerivedName(h Host, id state.ObjID) string {
	if id == 0 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(h.Chars(id).Name))
}

// dropTargetsNamed removes every target whose derived name equals name,
// preserving order. The chosen object is in the removed set too: Forge's
// sharesNameWith filter removes everything sharing the pick's name, including
// the pick itself.
func dropTargetsNamed(h Host, ts []state.Target, name string) []state.Target {
	out := ts[:0]
	for _, t := range ts {
		if copyPermanentDerivedName(h, t.Obj) != name {
			out = append(out, t)
		}
	}
	return out
}
