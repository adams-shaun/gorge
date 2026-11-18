package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The as-enters copy's TYPE-CONDITIONAL counter rider (Spark Double: "except
// it enters with an additional +1/+1 counter on it if it's a creature, it
// enters with an additional loyalty counter on it if it's a planeswalker";
// Moritte of the Frost: "if it's a creature, it enters with two additional
// +1/+1 counters"). Forge spells it as a SubAbility$ DB$ Effect whose
// ReplacementEffects$ is an Event$ Moved replacement upgrading the SAME entry
// (ReplacementResult$ Updated) with a PutCounter chain, then self-exiling the
// effect from the command zone. This build's Clone body runs after the entry
// folded, so registering that replacement would upgrade a LATER entry
// instead; the rider is recognised by shape (CloneETBTypedCounters) and read
// as exactly what it says: counters placed on the entering copy, each link
// gated on the copy's resulting types (CR 707.9b, CR 614.1c -- a declined
// election never reaches the chain).

// cloneETBTypedCounter is one link of the rider: N counters of Kind when the
// entering copy matches Type.
type cloneETBTypedCounter struct {
	Type string
	Kind string
	N    int32
}

var cloneETBTypedEffectKeys = state.NewNameSet("RememberObjects", "ReplacementEffects")

var cloneETBTypedCounterKeys = state.NewNameSet("Defined", "CounterType", "ETB", "CounterNum", "SubAbility")

// cloneETBTypedDefined maps a link's Defined$ onto the type it is gated on;
// "" means the type comes from the replacement's single ValidCard$ type.
var cloneETBTypedDefined = map[string]string{
	"ReplacedNewCard.Creature":     "Creature",
	"ReplacedNewCard.Planeswalker": "Planeswalker",
	"ReplacedCard":                 "",
}

// cloneETBTypedCounterKinds are the counter kinds the rider may place.
var cloneETBTypedCounterKinds = state.NewNameSet("P1P1", "LOYALTY")

// cloneETBTypedLinks is the rider's per-link walk bound.
const cloneETBTypedLinks = 4

// CloneETBTypedCounters classifies link as the type-conditional counter rider
// and returns its (type, kind, count) links in printed order. ok is false for
// every other Effect shape, which keeps the body withheld.
func CloneETBTypedCounters(link *cards.SA, svars map[string]string) (out []cloneETBTypedCounter, ok bool) {
	if link == nil || link.API != "Effect" || link.Sub != nil || len(link.Params) == 0 {
		return nil, false
	}
	for k := range link.Params {
		if !cloneETBTypedEffectKeys.Has(k) && k != cards.PKSpellDescription.String() {
			return nil, false
		}
	}
	if strings.TrimSpace(link.ParamStr(cards.PKRememberObjects)) != "Self" {
		return nil, false
	}
	name := strings.TrimSpace(EffectOf(link).ReplacementEffects)
	if name == "" || strings.ContainsAny(name, ", \t") {
		return nil, false
	}
	event, params := parseReplacementLine(svars, name)
	if event != "Moved" || params["Destination"] != "Battlefield" || params["ReplacementResult"] != "Updated" {
		return nil, false
	}
	// ValidCard$ names the types the replacement upgrades, each gated on the
	// remembered (the copy-making object itself).
	var valid []string
	for part := range strings.SplitSeq(params["ValidCard"], ",") {
		typ, rest, cut := strings.Cut(strings.TrimSpace(part), ".")
		if !cut || rest != "IsRemembered" || (typ != "Creature" && typ != "Planeswalker") {
			return nil, false
		}
		valid = append(valid, typ)
	}
	if len(valid) == 0 {
		return nil, false
	}
	body := cards.ResolveSVar(svars, params["ReplaceWith"])
	for d := 0; body != nil; d, body = d+1, body.Sub {
		if d >= cloneETBTypedLinks {
			return nil, false
		}
		if body.API == "ChangeZone" {
			// The chain's terminal self-exile of the effect from the command
			// zone: the effect token this build does not mint.
			cz := body
			if cz.Sub != nil || len(cz.Params) != 3 || cz.ParamStr(cards.PKDefined) != "Self" ||
				cz.ParamStr(cards.PKOrigin) != "Command" || cz.ParamStr(cards.PKDestination) != "Exile" {
				return nil, false
			}
			return out, len(out) > 0
		}
		if !IsPutCounter(body) {
			return nil, false
		}
		for k := range body.Params {
			if !cloneETBTypedCounterKeys.Has(k) {
				return nil, false
			}
		}
		pc := PutCounterOf(body)
		typ, known := cloneETBTypedDefined[strings.TrimSpace(body.ParamStr(cards.PKDefined))]
		if !pc.ETB || len(pc.Unread) > 0 || !known || !cloneETBTypedCounterKinds.Has(pc.CounterType) {
			return nil, false
		}
		if typ == "" {
			if len(valid) != 1 {
				return nil, false
			}
			typ = valid[0]
		}
		inValid := false
		for _, v := range valid {
			inValid = inValid || v == typ
		}
		n, lit := literalInt(pc.CounterNum)
		if !inValid || !lit || n < 1 {
			return nil, false
		}
		out = append(out, cloneETBTypedCounter{Type: typ, Kind: pc.CounterType, N: int32(n)})
	}
	return nil, false
}

// applyCloneETBTypedCounters runs the rider when link is one reached on an
// answered, taken as-enters copy: each counter lands on the entering copy if
// its resulting types match the link's type. It reports whether link was the
// rider (the Effect body must then not register the replacement).
func applyCloneETBTypedCounters(h Host, c *Ctx, link *cards.SA) bool {
	if !c.CloneEnter.ETB || !c.CloneEnter.ChoiceValid || c.CloneEnter.Choice == 0 || link.Params == nil {
		return false
	}
	svars := c.SVars
	if svars == nil {
		if original := h.Game().Obj(c.Source); original != nil && original.Face() != nil {
			svars = original.Face().SVars
		}
	}
	rider, ok := CloneETBTypedCounters(link, svars)
	if !ok {
		return false
	}
	g := h.Game()
	if o := g.Obj(c.Source); o == nil || o.Zone != state.ZBattlefield {
		return true
	}
	// Re-resolve the PutCounter links beside their classified values so the
	// placement reads the link's own compiled parameters (putCounterAmount).
	_, params := parseReplacementLine(svars, strings.TrimSpace(EffectOf(link).ReplacementEffects))
	body := cards.ResolveSVar(svars, params["ReplaceWith"])
	sc := c.TableSpecContext(c.Controller)
	for _, r := range rider {
		if !IsPutCounter(body) {
			break
		}
		if MatchesSpecCtx(g, r.Type, c.Source, sc) {
			n := putCounterAmount(h, c, body, PutCounterOf(body), true, r.N, 0)
			emitPutCounterChange(h, c, body, events.Event{Kind: events.CounterChange, Obj: c.Source, Counter: r.Kind, Amount: n})
		}
		body = body.Sub
	}
	return true
}
