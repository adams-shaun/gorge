package botpolicy

import (
	"fmt"
	"slices"
	"strings"

	"github.com/adams-shaun/gorge/engine/decision"
)

// MulliganPolicy is one named London-mulligan policy: it answers the
// keep/mulligan ask and the bottoming ask (engine/rules/mulligan.go's two
// KMulligan shapes). ok=false means "fall through to the default arm"
// (policy.go's Decide kmulligan case) exactly as mulliganByRule has always
// done for the coin rule.
//
// Adding a policy is ONE call, from any package's init(): RegisterMulligan(
// yourPolicy) -- typically a MulliganPolicy implementation or a
// mulliganFunc literal -- returns the id the runner stores in Board.Mulligan,
// and a runner selects it by name through MulliganByName. The registry is a
// slice (no map is ranged anywhere), so registration order is the id order.
type MulliganPolicy interface {
	// Name is the registry key a runner selects the policy by.
	Name() string
	// Answer answers the KMulligan ask d from the deciding seat's Board.
	Answer(b Board, d *decision.Decision) (choices []int, ok bool)
}

// mulliganFunc adapts two funcs to MulliganPolicy.
type mulliganFunc struct {
	name   string
	answer func(Board, *decision.Decision) ([]int, bool)
}

func (p mulliganFunc) Name() string { return p.name }
func (p mulliganFunc) Answer(b Board, d *decision.Decision) ([]int, bool) {
	return p.answer(b, d)
}

// mulliganRegistry is the ordered registry: index == MulliganRule id. The
// built-ins register first in init() so their ids stay the existing
// constants (Coin=0, Lands=1, Never=2, Constructed=3, Forge=4).
var mulliganRegistry []MulliganPolicy

// RegisterMulligan appends p and returns its id (its index, usable as a
// MulliganRule). It panics on an empty or duplicate Name: a policy that
// answers nothing silently, or shadows another by accident, is a build
// error, not a runtime fallback.
func RegisterMulligan(p MulliganPolicy) MulliganRule {
	name := p.Name()
	if name == "" {
		panic("botpolicy: RegisterMulligan: empty mulligan policy name")
	}
	for _, q := range mulliganRegistry {
		if q.Name() == name {
			panic("botpolicy: RegisterMulligan: duplicate mulligan policy name " + name)
		}
	}
	mulliganRegistry = append(mulliganRegistry, p)
	return MulliganRule(len(mulliganRegistry) - 1)
}

// MulliganByName resolves a runner's `-mulligan-policy <name>` selection to
// the id stored on Board.Mulligan. An unknown name's error lists the valid
// names.
func MulliganByName(name string) (MulliganRule, error) {
	for i, p := range mulliganRegistry {
		if p.Name() == name {
			return MulliganRule(i), nil
		}
	}
	return 0, fmt.Errorf("unknown mulligan policy %q (registered: %s)", name, strings.Join(MulliganNames(), ", "))
}

// MulliganNames lists the registered policies, sorted.
func MulliganNames() []string {
	names := make([]string, len(mulliganRegistry))
	for i, p := range mulliganRegistry {
		names[i] = p.Name()
	}
	slices.Sort(names)
	return names
}

// String returns the policy's registered name.
func (r MulliganRule) String() string {
	if int(r) < len(mulliganRegistry) {
		return mulliganRegistry[r].Name()
	}
	return fmt.Sprintf("MulliganRule(%d)", uint8(r))
}

func init() {
	RegisterMulligan(mulliganFunc{name: "coin", answer: func(b Board, d *decision.Decision) ([]int, bool) {
		// The default arm answers coin itself (policy.go / legacy.go draw
		// the 1/3 coin off the rng); the rng draw sequence must not change.
		return nil, false
	}})
	RegisterMulligan(mulliganFunc{name: "lands", answer: func(b Board, d *decision.Decision) ([]int, bool) {
		return mulliganAskAnswer(d, b.keepByLands, func() []int { return b.landBottoming(d) })
	}})
	RegisterMulligan(mulliganFunc{name: "never", answer: func(b Board, d *decision.Decision) ([]int, bool) {
		return mulliganAskAnswer(d, func(int) bool { return true }, func() []int { return b.chooseDiscard(d) })
	}})
	RegisterMulligan(mulliganFunc{name: "constructed", answer: func(b Board, d *decision.Decision) ([]int, bool) {
		return mulliganAskAnswer(d, b.keepConstructed, func() []int { return b.landBottoming(d) })
	}})
	RegisterMulligan(mulliganFunc{name: "forge", answer: func(b Board, d *decision.Decision) ([]int, bool) {
		return mulliganAskAnswer(d, b.keepForge, func() []int { return b.forgeBottoming(d) })
	}})
}

// mulliganAskAnswer walks the two KMulligan shapes for a built-in policy:
// keep decides the keep/mulligan ask on the bottoming penalty the prompt
// carries (keepBottomCount), bottom answers the bottoming ask. ok is false
// only for a shape that is neither, which the default arm decides.
func mulliganAskAnswer(d *decision.Decision, keep func(bottom int) bool, bottom func() []int) ([]int, bool) {
	if d.Options[0].Kind == "bottom" {
		return bottom(), true
	}
	keepIdx, mullIdx := -1, -1
	for _, o := range d.Options {
		switch o.Kind {
		case "keep":
			keepIdx = o.Index
		case "mulligan":
			mullIdx = o.Index
		}
	}
	if keepIdx < 0 {
		return nil, false // not a keep/mulligan shape: the default arm decides
	}
	if mullIdx >= 0 && !keep(keepBottomCount(d.Prompt)) {
		return []int{mullIdx}, true
	}
	return []int{keepIdx}, true
}
