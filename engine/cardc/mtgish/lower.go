package mtgish

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

// lowerFunc lowers one tagged node to an IR value (an *ir.X). Front-end
// tickets register one per Enum.Variant from an init in their own fe_*.go.
type lowerFunc func(c *Ctx, n node) (any, error)

var (
	registry = map[string]lowerFunc{}
	// overlay holds test-only lowerings (withStubs); lower consults it before
	// the registry. Tests that fill it must not run in parallel.
	overlay map[string]lowerFunc
)

// register adds the lowering for key ("Enum.Variant"). A duplicate panics.
func register(key string, f lowerFunc) {
	if _, dup := registry[key]; dup {
		panic("mtgish: duplicate lowering for " + key)
	}
	registry[key] = f
}

// Registered reports whether key has a registered lowering.
func Registered(key string) bool {
	_, ok := registry[key]
	return ok
}

// RegisteredKeys lists the registered keys, sorted.
func RegisteredKeys() []string {
	out := make([]string, 0, len(registry))
	for k := range registry {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// RefusalError is a variant with no registered lowering.
type RefusalError struct{ Key, Path string }

func (e *RefusalError) Error() string {
	return "cardc: no lowering for " + e.Key + " at " + e.Path
}

type declared struct {
	stem string
	slot uint8
}

// Ctx is the state of lowering one card face.
type Ctx struct {
	Card    string
	Face    int
	path    []string
	targets []declared
}

// Src is the path of keys from the rule down to the node being lowered.
func (c *Ctx) Src() string { return strings.Join(c.path, "/") }

// lower dispatches x on its Enum.Variant key.
func (c *Ctx) lower(x any) (any, error) {
	_, _, n, ok := tagged(x)
	if !ok {
		return nil, fmt.Errorf("cardc: %s: not a tagged node: %T", c.Src(), x)
	}
	key := qualified(x)
	c.path = append(c.path, key)
	defer func() { c.path = c.path[:len(c.path)-1] }()
	f, ok := overlay[key]
	if !ok {
		f, ok = registry[key]
	}
	if !ok {
		return nil, &RefusalError{Key: key, Path: c.Src()}
	}
	v, err := f(c, n)
	if err == nil {
		setSrc(v, c.Src())
	}
	return v, err
}

// setSrc stamps the lowering path on a node a lowering returned, unless the
// lowering set one itself. Every lowering gets Src without writing it.
func setSrc(v any, src string) {
	var p *string
	switch x := v.(type) {
	case *ir.Ref:
		p = &x.Src
	case *ir.Filter:
		p = &x.Src
	case *ir.Value:
		p = &x.Src
	case *ir.Cond:
		p = &x.Src
	case *ir.Compare:
		p = &x.Src
	case *ir.Target:
		p = &x.Src
	case *ir.Ability:
		p = &x.Src
	case *ir.Keyword:
		p = &x.Src
	case *ir.Trigger:
		p = &x.Src
	case *ir.Program:
		p = &x.Src
	case *ir.Step:
		p = &x.Src
	case *ir.Cost:
		p = &x.Src
	case *ir.CostPart:
		p = &x.Src
	case *ir.PT:
		p = &x.Src
	}
	if p != nil && *p == "" {
		*p = src
	}
}

func lowerAs[T any](c *Ctx, x any) (*T, error) {
	v, err := c.lower(x)
	if err != nil {
		return nil, err
	}
	t, ok := v.(*T)
	if !ok {
		return nil, fmt.Errorf("%s lowers to %T, want %T", qualified(x), v, t)
	}
	return t, nil
}

// Ref lowers x to an *ir.Ref.
func (c *Ctx) Ref(x any) (*ir.Ref, error) { return lowerAs[ir.Ref](c, x) }

// Filter lowers x to an *ir.Filter.
func (c *Ctx) Filter(x any) (*ir.Filter, error) { return lowerAs[ir.Filter](c, x) }

// Value lowers x to an *ir.Value.
func (c *Ctx) Value(x any) (*ir.Value, error) { return lowerAs[ir.Value](c, x) }

// Cond lowers x to an *ir.Cond.
func (c *Ctx) Cond(x any) (*ir.Cond, error) { return lowerAs[ir.Cond](c, x) }

// Compare lowers x to an *ir.Compare.
func (c *Ctx) Compare(x any) (*ir.Compare, error) { return lowerAs[ir.Compare](c, x) }

// Target lowers x to an *ir.Target.
func (c *Ctx) Target(x any) (*ir.Target, error) { return lowerAs[ir.Target](c, x) }

// Ability lowers x to an *ir.Ability.
func (c *Ctx) Ability(x any) (*ir.Ability, error) { return lowerAs[ir.Ability](c, x) }

// DeclareTarget records a target declaration and returns its slot, the index
// in declaration order. stem is the name the data's Ref_ variants use for
// the target, which is not always the Target variant's name:
// UptoOneTargetPermanent declares TargetPermanent.
func (c *Ctx) DeclareTarget(stem string) uint8 {
	slot := uint8(len(c.targets))
	c.targets = append(c.targets, declared{stem: stem, slot: slot})
	return slot
}

// ResetTargets empties the declarations; the driver calls it before each
// rule, so slots are per ability.
func (c *Ctx) ResetTargets() { c.targets = c.targets[:0] }

// TargetSlot resolves a Ref_ variant ("Ref_TargetPermanent2") to a declared
// slot. A trailing decimal k means the k-th (1-based) declaration of that
// stem; without digits it is the latest declaration of the exact stem. An
// _<n> suffix (Ref_AnyTargets_1) and an undeclared stem report false.
func (c *Ctx) TargetSlot(refVariant string) (uint8, bool) {
	stem := strings.TrimPrefix(refVariant, "Ref_")
	if strings.Contains(stem, "_") {
		return 0, false
	}
	i := len(stem)
	for i > 0 && stem[i-1] >= '0' && stem[i-1] <= '9' {
		i--
	}
	if i == len(stem) {
		for j := len(c.targets) - 1; j >= 0; j-- {
			if c.targets[j].stem == stem {
				return c.targets[j].slot, true
			}
		}
		return 0, false
	}
	k, err := strconv.Atoi(stem[i:])
	if err != nil || k < 1 || i == 0 {
		return 0, false
	}
	stem = stem[:i]
	for _, d := range c.targets {
		if d.stem == stem {
			k--
			if k == 0 {
				return d.slot, true
			}
		}
	}
	return 0, false
}
