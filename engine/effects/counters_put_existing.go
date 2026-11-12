package effects

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// putCounterExisting runs the CounterType$ ExistingCounter shape: ExistingCounter is not a counter kind but "a kind of counter
// the recipient already has" (CR 122.1, 122.6). With EachExistingCounter$
// True (Powerful Broker, Maulfist Revolutionary, Skyship Plunderer: "for each
// kind of counter on target permanent or player, give that permanent or
// player another counter of that kind") every kind the recipient carries
// takes CounterNum$ more; without it (Animation Module, Ichormoon Gauntlet,
// The Caves of Androzani: "choose a counter on ...") the controller picks ONE
// of those kinds through the shared counter_kind ask. A recipient with no
// counters takes nothing. The kinds are read in the recipient's counter-slice
// order before any placement (never a map walk), so the walk is
// deterministic and a kind added mid-walk is never revisited.
func putCounterExisting(h Host, c *Ctx, sa *cards.SA, pc *PutCounterParams, n int32) {
	g := h.Game()
	var placed []state.Target
	for ti, t := range Defined(h, c, sa) {
		var cs []state.Counter
		var p state.PlayerID
		if t.IsPlayer {
			p = PlayerOf(h, c, t)
			if int(p) < 0 || int(p) >= len(g.Players) {
				continue
			}
			cs = g.Players[p].Counters
		} else {
			o := g.Obj(t.Obj)
			if o == nil {
				continue
			}
			cs = o.Counters
		}
		kinds := counterKinds("", len(cs),
			func(i int) string { return cs[i].Kind },
			func(i int) int32 { return cs[i].N })
		if len(kinds) == 0 {
			continue
		}
		if !pc.EachExistingCounter {
			kinds = []string{putCounterExistingPick(h, c, sa, ti, kinds)}
		}
		for _, k := range kinds {
			if t.IsPlayer {
				emitPutCounterChange(h, c, sa, events.Event{Kind: events.PlayerCounterChange, Player: p, Counter: k, Amount: n})
			} else {
				emitPutCounterChange(h, c, sa, events.Event{Kind: events.CounterChange, Obj: t.Obj, Counter: k, Amount: n})
			}
		}
		// RememberPut$'s contract (effPutCounter's loop): only a recipient
		// that actually took a counter; a player only under RememberPut$.
		if n > 0 && (!t.IsPlayer || pc.RememberPut) {
			if t.IsPlayer {
				t = state.Target{Player: p, IsPlayer: true}
			}
			placed = append(placed, t)
		}
	}
	rememberPlaced(c, sa, placed)
}

// putCounterExistingPick is the "choose a counter on it" ask over the kinds
// recipient ti carries: no ask when there is one kind; otherwise the shared
// counter_kind KChoose (Min 1, Max 1, botpolicy's first-option arm), whose
// unserved stand-in is the same first kind.
func putCounterExistingPick(h Host, c *Ctx, sa *cards.SA, ti int, kinds []string) string {
	if len(kinds) == 1 {
		return kinds[0]
	}
	d := &decision.Decision{Player: c.Controller, Kind: decision.KChoose, Min: 1, Max: 1, Source: c.Source,
		ResumeKind: "counter_kind", ResumeSA: sa, ResumeTarget: ti, Prompt: "Choose a kind of counter"}
	for i, k := range kinds {
		d.Options = append(d.Options, decision.Option{Index: i, Kind: "counter", Label: k, Player: c.Controller})
	}
	if ans, ok := AskTape(h, d); ok {
		if k := counterAnswerLabel(ans); k != "" {
			return k
		}
	}
	return kinds[0]
}

// putCounterExistingChoices runs CounterType$ ExistingCounter with Choices$
// (Aven Courier, Contractual Safeguard): there Choices$ is the SOURCE of the
// kind -- "choose a counter on a permanent you control" -- never the
// recipient pick the bare-Choices$ shape makes. The source set is every
// battlefield object matching Choices$ (zone order); one kind is chosen from
// the kinds they carry through the shared counter_kind ask (no ask for one
// kind, none at all when there are no counters). The recipients are
// PutOnDefined$ (resolved through the ordinary Defined machinery) or
// PutOnEachOther$ (every battlefield object matching the spec except the
// objects carrying the chosen kind in the source set), in zone order;
// OnlyNewKind$ True skips a recipient that already has the kind.
//
// Deviation, deliberate: Forge picks ONE source object and then excludes just
// it for "each other". Here the source-object choice is collapsed into the
// kind choice over the union of the source set, so "each other" excludes
// every source object that carries the chosen kind (CR 122.1: the kind, not
// the object, is what the effect keys on).
func putCounterExistingChoices(h Host, c *Ctx, sa *cards.SA, pc *PutCounterParams, n int32) {
	if n <= 0 {
		return
	}
	g := h.Game()
	sctx := c.SpecContext(c.Controller)
	var source []state.ObjID
	for _, p := range g.AliveFrom(0) {
		for _, id := range g.Zone(state.ZBattlefield, p) {
			if MatchesSpecCtx(g, pc.Choices, id, sctx) {
				source = append(source, id)
			}
		}
	}
	var seen []state.Counter
	for _, id := range source {
		if o := g.Obj(id); o != nil {
			seen = append(seen, o.Counters...)
		}
	}
	kinds := distinctCounterKinds(counterKinds("", len(seen),
		func(i int) string { return seen[i].Kind },
		func(i int) int32 { return seen[i].N }))
	if len(kinds) == 0 {
		return
	}
	kind := putCounterExistingPick(h, c, sa, 0, kinds)

	var recipients []state.ObjID
	switch {
	case pc.PutOnEachOther != "":
		carriers := map[state.ObjID]bool{}
		for _, id := range source {
			if o := g.Obj(id); o != nil && o.Counter(kind) > 0 {
				carriers[id] = true
			}
		}
		for _, p := range g.AliveFrom(0) {
			for _, id := range g.Zone(state.ZBattlefield, p) {
				if !carriers[id] && MatchesSpecCtx(g, pc.PutOnEachOther, id, sctx) {
					recipients = append(recipients, id)
				}
			}
		}
	case pc.PutOnDefined != "":
		for _, t := range DefinedSpec(h, c, pc.PutOnDefined) {
			if !t.IsPlayer && g.Obj(t.Obj) != nil {
				recipients = append(recipients, t.Obj)
			}
		}
	}
	var placed []state.Target
	for _, id := range recipients {
		if pc.OnlyNewKind && g.Obj(id).Counter(kind) > 0 {
			continue
		}
		emitPutCounterChange(h, c, sa, events.Event{Kind: events.CounterChange, Obj: id, Counter: kind, Amount: n})
		placed = append(placed, state.Target{Obj: id})
	}
	rememberPlaced(c, sa, placed)
}

// distinctCounterKinds drops repeats, keeping first-seen order.
func distinctCounterKinds(in []string) []string {
	out := make([]string, 0, len(in))
	for _, k := range in {
		dup := false
		for _, o := range out {
			if o == k {
				dup = true
				break
			}
		}
		if !dup {
			out = append(out, k)
		}
	}
	return out
}
