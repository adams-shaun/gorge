package effects

import (
	"slices"
	"strconv"
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

// cloneETBTypedWords are the rider's fixed card-script words with no
// dense-code owner of their own (the API, zone, replacement-event, card-type
// and Defined-spec words this classifier reads go through cards.APICodeForName,
// parseZoneCodes, cards.ReplEventOf, cards.TypeMaskOfName and
// definedSpecCodes): named once, compared as values.
var (
	cloneETBTypedRememberSelf   = "Self"
	cloneETBTypedResultUpdated  = "Updated"
	cloneETBTypedRememberSuffix = ".IsRemembered"
	cloneETBTypedNewCardHead    = "ReplacedNewCard"
	cloneETBTypedReplacedCard   = "ReplacedCard"
	cloneETBTypedDBKey          = "DB"
)

// cloneETBTypedEffectKeys are the keys an admitted rider link may carry (the
// rider's SVar replacement reference and its remembered self), sorted for the
// unreadKeys scan; a descriptive-only extra (SpellDescription$) does not
// decline an otherwise exact shape.
var cloneETBTypedEffectKeys = [...]string{
	cards.PKRememberObjects.String(), cards.PKReplacementEffects.String(),
}

// cloneETBTypedCounterKeys are the keys an admitted PutCounter link may carry
// beside the line's own DB$ head, sorted for the unreadKeys scan.
var cloneETBTypedCounterKeys = [...]string{
	cards.PKCounterNum.String(), cards.PKCounterType.String(), cloneETBTypedDBKey,
	cards.PKDefined.String(), cards.PKETB.String(), cards.PKSubAbility.String(),
}

// cloneETBTypedCounterKinds are the counter kinds the rider may place.
var cloneETBTypedCounterKinds = [...]string{"P1P1", "LOYALTY"}

// cloneETBTypedLinks is the rider's per-link walk bound.
const cloneETBTypedLinks = 4

// cloneETBTypedDefinedOf maps a link's Defined$ onto the type its counter is
// gated on: the part after the dot of a `ReplacedNewCard.<Type>` spec (the
// copied card's resulting type), or the empty gate for the whole-object
// `ReplacedCard` spec. ok is false for any other shape.
func cloneETBTypedDefinedOf(spec string) (string, bool) {
	if head, typ, cut := strings.Cut(spec, "."); cut && head == cloneETBTypedNewCardHead &&
		cards.TypeMaskOfName(typ) != 0 {
		return typ, true
	}
	return "", spec == cloneETBTypedReplacedCard
}

// CloneETBTypedCounters classifies link as the type-conditional counter rider
// and returns its (type, kind, count) links in printed order. ok is false for
// every other Effect shape, which keeps the body withheld.
func CloneETBTypedCounters(link *cards.SA, svars map[string]string) (out []cloneETBTypedCounter, ok bool) {
	if link == nil || cards.APICodeForName(link.API) != cards.APIEffect || link.Sub != nil || len(link.Params) == 0 {
		return nil, false
	}
	if len(unreadKeys(link, cloneETBTypedEffectKeys[:])) != 0 {
		return nil, false
	}
	if strings.TrimSpace(link.ParamStr(cards.PKRememberObjects)) != cloneETBTypedRememberSelf {
		return nil, false
	}
	name := strings.TrimSpace(EffectOf(link).ReplacementEffects)
	if name == "" || strings.ContainsAny(name, ", \t") {
		return nil, false
	}
	event, params := parseReplacementLine(svars, name)
	if cards.ReplEventOf(event) != cards.ReplMoved ||
		parseZoneCodes.Code(params["Destination"]) != parseZoneBattlefield ||
		params["ReplacementResult"] != cloneETBTypedResultUpdated {
		return nil, false
	}
	// ValidCard$ names the types the replacement upgrades, each gated on the
	// remembered (the copy-making object itself).
	var valid []string
	for part := range strings.SplitSeq(params["ValidCard"], ",") {
		typ, cut := strings.CutSuffix(strings.TrimSpace(part), cloneETBTypedRememberSuffix)
		mask := cards.TypeMaskOfName(typ)
		if !cut || (mask != cards.TypeCreature && mask != cards.TypePlaneswalker) {
			return nil, false
		}
		valid = append(valid, typ)
	}
	if len(valid) == 0 {
		return nil, false
	}
	// The replacement's body is walked as SVar TEXT (cloneETBDBLine), not
	// through the PutCounter compile: the Effect primitive that reaches this
	// rider reads no PutCounter parameter of its own.
	name = params["ReplaceWith"]
	for d := 0; name != ""; d++ {
		if d >= cloneETBTypedLinks {
			return nil, false
		}
		_, lp := parseReplacementLine(svars, name)
		api := cards.APICodeForName(lp["DB"])
		if api == cards.APIChangeZone {
			// The chain's terminal self-exile of the effect from the command
			// zone: the effect token this build does not mint.
			if len(lp) != 4 || lp["Defined"] != cloneETBTypedRememberSelf ||
				parseZoneCodes.Code(lp["Origin"]) != parseZoneCommand ||
				parseZoneCodes.Code(lp["Destination"]) != parseZoneExile {
				return nil, false
			}
			return out, len(out) > 0
		}
		if api != cards.APIPutCounter || !isTrue(lp["ETB"]) {
			return nil, false
		}
		// lp is the SVar line's own key/value map, not an ability Params map:
		// every admitted key must be in the whitelist (DB$ head included).
		for k := range lp {
			if !slices.Contains(cloneETBTypedCounterKeys[:], k) {
				return nil, false
			}
		}
		typ, known := cloneETBTypedDefinedOf(lp["Defined"])
		kind := lp["CounterType"]
		if !known || !slices.Contains(cloneETBTypedCounterKinds[:], kind) {
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
		n, err := strconv.Atoi(lp["CounterNum"])
		if !inValid || err != nil || n < 1 {
			return nil, false
		}
		out = append(out, cloneETBTypedCounter{Type: typ, Kind: kind, N: int32(n)})
		name = lp["SubAbility"]
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
	sc := c.TableSpecContext(c.Controller)
	for _, r := range rider {
		// The gate is a bare type word (layer 4, always published); the bind
		// is the shared text-to-tables rule, so a wider gate reads its layer.
		bindSpecLayerTables(h, &sc.Layers, r.Type, false)
		if MatchesSpecCtx(g, r.Type, c.Source, sc) {
			h.Emit(events.Event{Kind: events.CounterChange, Obj: c.Source, Counter: r.Kind, Amount: r.N})
		}
	}
	return true
}
