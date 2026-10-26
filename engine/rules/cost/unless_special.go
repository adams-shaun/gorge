package cost

import (
	"math"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/state"
)

// unless_special.go is the unless-cost grammar's non-mana, non-pick-by-count
// tail (CR 118.12a: an "unless [player] [cost]" cost is any cost the payer
// may pay, 118.3: payable only with the resources). Each head parses to the
// SAME Cost field the ordinary cost parser fills, so the pay layer's shared
// candidate helpers read one shape:
//
//   - AddCounter<N/Kind/Defined[/Desc]> -> AddCounter{N, Spec: kind (the
//     canonical counter spelling), Target: Defined, Desc}. Defined names the
//     recipient: CARDNAME (the source), Card.targetedBy (the resolving
//     ability's target) or a spec the payer chooses among.
//   - Waterbend<N> -> Waterbend (CR 701.67a). Unlike the ability-cost reader
//     the {N} is NOT folded into Generic: the unless payment's waterbend
//     step owns the whole amount (taps plus floating mana).
//   - Blight<N> -> Blight{N, "Creature.YouCtrl"} (CR 701.68a).
//   - CollectEvidence<N> -> Evidence{N}.
//   - ExileFromStack<1/Card.Self> -> Exile{1, "Card.Self", Zone: ZStack}: the
//     resolving spell itself, a choice-free exile part.
//
// A dynamic amount (Waterbend<X>, Blight<X>, CollectEvidence<NAME>) and a
// zero amount stay hard declines, like every other unbound token.

// parseUnlessSpecialCost adds the part t names to c. matched reports that t
// is one of the heads above; ok=false then means this instance is malformed
// or dynamic and the whole cost is unpriceable.
func parseUnlessSpecialCost(c *Cost, t costTok) (matched, ok bool) {
	if !t.angle {
		return false, false
	}
	switch t.id {
	case hAddCounter:
		part, ok := matchUnlessAddCounter(t.inner)
		if ok {
			c.AddCounter = append(c.AddCounter, part)
		}
		return true, ok
	case hWaterbend:
		n, ok := unlessLiteralAmount(t.inner)
		if ok {
			c.Waterbend = AddClampedGeneric(c.Waterbend, int64(n))
		}
		return true, ok
	case hBlight:
		n, ok := unlessLiteralAmount(t.inner)
		if ok {
			c.Blight = append(c.Blight, CostPart{N: n, Spec: "Creature.YouCtrl"})
		}
		return true, ok
	case hCollectEvidence:
		n, ok := unlessLiteralAmount(t.inner)
		if ok {
			c.Evidence = append(c.Evidence, CostPart{N: n})
		}
		return true, ok
	case hExileFromStack:
		n, spec, _, found := countSpecDesc(t.inner, isDigits)
		if !found || n != "1" || spec != "Card.Self" {
			return true, false
		}
		c.Exile = append(c.Exile, CostPart{N: 1, Spec: spec, Zone: state.ZStack})
		return true, true
	}
	return false, false
}

// unlessLiteralAmount reads a positive literal amount (a dynamic X or a zero
// is not one).
func unlessLiteralAmount(s string) (int32, bool) {
	if !isDigits(s) {
		return 0, false
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n <= 0 || n > int64(math.MaxInt32) {
		return 0, false
	}
	return int32(n), true
}

// matchUnlessAddCounter reads AddCounter's inner "N/Kind/Defined[/Desc]".
func matchUnlessAddCounter(inner string) (CostPart, bool) {
	f := strings.SplitN(inner, "/", 4)
	if len(f) < 3 || !isDigits(f[0]) || !isCounterKind(f[1]) || f[2] == "" {
		return CostPart{}, false
	}
	n, err := strconv.ParseInt(f[0], 10, 64)
	if err != nil || n <= 0 || n > int64(math.MaxInt32) {
		return CostPart{}, false
	}
	part := CostPart{N: int32(n), Spec: state.CanonicalCounterKind(f[1]), Target: strings.ReplaceAll(f[2], ";", ",")}
	if len(f) == 4 {
		part.Desc = f[3]
	}
	return part, true
}

// unlessSpecialShapeOK rejects the one mixed shape the waterbend step cannot
// settle atomically: Waterbend beside a mana, life or life-half component
// (the step spends the floating pool itself). No corpus UnlessCost$ mixes
// them; the fail-closed direction is a hard decline.
func unlessSpecialShapeOK(c Cost) bool {
	return c.Waterbend == 0 || (!c.HasManaPayment() && c.Life == 0 && !c.LifeHalfUp && len(c.LifeX) == 0)
}
