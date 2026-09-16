package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func init() {
	Register("Untap", effUntap)
	// Rules intercepts this internal keyword-expansion API while its triggered
	// ability resolves; registration keeps the expanded face's primitive set
	// supported and the no-engine effects fallback harmless.
	Register("CumulativeUpkeep", func(Host, *Ctx, *cards.SA) {})
}

// TryUntap is the shared CR 122.1d event proposal for effects and the untap
// step: a stun counter is removed instead of untapping the permanent.
func TryUntap(h Host, id state.ObjID) {
	o := h.Game().Obj(id)
	if o == nil || o.Zone != state.ZBattlefield || !o.Tapped {
		return
	}
	if o.Counter("STUN") > 0 {
		h.Emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "STUN", Amount: -1})
		return
	}
	h.Emit(events.Event{Kind: events.Untap, Obj: id})
}

func untapBattlefieldCondition(h Host, c *Ctx, sa *cards.SA) bool {
	spec, ok := sa.Params["ConditionPresent"]
	if !ok || sa.Params["ConditionDefined"] != "" {
		return true
	}
	if len(UnknownPredicates(spec)) > 0 {
		return false
	}
	n := 0
	g := h.Game()
	for _, p := range g.AliveFrom(0) {
		for _, id := range g.Zone(state.ZBattlefield, p) {
			o := g.Obj(id)
			if o != nil && MatchesObjectCtx(g, spec, o, c.SpecContext(c.Controller)) {
				n++
			}
		}
	}
	op, want, ok := parseConditionCompare(sa.Params["ConditionCompare"])
	if sa.Params["ConditionCompare"] == "" {
		return n > 0
	}
	if !ok {
		return false
	}
	switch op {
	case "EQ":
		return n == want
	case "NE":
		return n != want
	case "LT":
		return n < want
	case "LE":
		return n <= want
	case "GT":
		return n > want
	case "GE":
		return n >= want
	}
	return false
}

// untapTypeCandidates resolves the corpus's UntapType$ family in deterministic
// seat/zone order. A Defined$ player narrows which battlefield is searched;
// otherwise the type/controller predicates themselves determine membership.
func untapTypeCandidates(h Host, c *Ctx, sa *cards.SA) []state.ObjID {
	g := h.Game()
	owners := g.AliveFrom(0)
	if def := Defined(h, c, sa); len(def) > 0 {
		var ps []state.PlayerID
		for _, t := range def {
			if t.IsPlayer {
				seen := false
				for _, p := range ps {
					seen = seen || p == t.Player
				}
				if !seen {
					ps = append(ps, t.Player)
				}
			}
		}
		if len(ps) > 0 {
			owners = ps
		}
	}
	spec := sa.Params["UntapType"]
	var out []state.ObjID
	for _, p := range owners {
		for _, id := range g.Zone(state.ZBattlefield, p) {
			if MatchesSpecCtx(g, spec, id, c.SpecContext(c.Controller)) {
				out = append(out, id)
			}
		}
	}
	return out
}

// effUntap supports both ordinary listed/targeted untaps and Forge's
// UntapType$ + Amount$ chooser family. UntapExactly$ fixes Min=Max; UntapUpTo$
// permits zero through Amount. The resumed answer is scoped and consumed here,
// so a nested Untap poses its own choice.
func effUntap(h Host, c *Ctx, sa *cards.SA) {
	if !untapBattlefieldCondition(h, c, sa) {
		return
	}
	if sa.Params["UntapType"] == "" {
		for _, t := range Defined(h, c, sa) {
			if !t.IsPlayer {
				TryUntap(h, t.Obj)
			}
		}
		return
	}

	var chosen []state.ObjID
	if c.UntapDone {
		chosen = append(chosen, c.Untap...)
		c.Untap, c.UntapDone = nil, false
	} else {
		candidates := untapTypeCandidates(h, c, sa)
		n := int(Num(h, c, sa, "Amount", 1))
		if n <= 0 || len(candidates) == 0 {
			return
		}
		if n > len(candidates) {
			n = len(candidates)
		}
		upTo := strings.EqualFold(sa.Params["UntapUpTo"], "True")
		needsAsk := upTo || len(candidates) > n
		if needsAsk {
			min := n
			if upTo {
				min = 0
			}
			d := &decision.Decision{Player: c.Controller, Kind: decision.KChoose,
				Min: min, Max: n, Source: c.Source, ResumeKind: "untap", ResumeSA: sa,
				Prompt: "Choose permanents to untap"}
			for _, id := range candidates {
				o := h.Game().Obj(id)
				d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "untap", Obj: id, Label: o.Face().Name})
			}
			if h.Ask(d) {
				return
			}
		}
		chosen = candidates[:n]
	}
	for _, id := range chosen {
		TryUntap(h, id)
	}
}
