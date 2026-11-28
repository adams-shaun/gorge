package cards

import (
	"sync/atomic"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

// FilterBinder lowers one filter text to its typed form. It returns nil and a
// refusal name when it cannot read the text. It must be a pure function of the text.
type FilterBinder func(text string) (f *ir.Filter, refusal string)

// filterBinder is the registered binder and filterBinderOwner its owner's
// name; filterBinderSealed is set by the first FilterOf read, after which a
// registration would leave earlier reads answered by another binder.
var (
	filterBinder       FilterBinder
	filterBinderOwner  string
	filterBinderSealed atomic.Bool
)

// RegisterFilterBinder installs the binder. Package init only: it panics on a
// second owner, and once any FilterOf call has run. A second registration
// under the same owner is allowed (a test binary can link two copies of one
// package, each running its init).
func RegisterFilterBinder(owner string, b FilterBinder) {
	if filterBinderSealed.Load() {
		panic("cards: RegisterFilterBinder after a FilterOf read: " + owner)
	}
	if filterBinder != nil && filterBinderOwner != owner {
		panic("cards: FilterBinder registered by " + filterBinderOwner + " and " + owner)
	}
	filterBinder, filterBinderOwner = b, owner
}

// filterKeyMask holds the object-filter keys FilterOf answers for.
var filterKeyMask = ParamMaskOf(
	PKAffected, PKChangeType, PKConditionPresent, PKConditionPresent2, PKIsPresent, PKIsPresent2,
	PKValid, PKValidCard, PKValidCards, PKValidCause, PKValidSource, PKValidTarget, PKValidTgts,
)

// IsFilterKey reports whether k is one of the 13 object-filter keys.
func IsFilterKey(k ParamKey) bool {
	w := int(k) >> 6
	return w < paramMaskWords && filterKeyMask[w]&(1<<(k&63)) != 0
}

// boundFilter is one filter slot's cached binder answer.
type boundFilter struct {
	f       *ir.Filter
	refusal string
}

// ParamSetFilter is FilterOf for a view that carries a node's (set, map) pair.
func ParamSetFilter(ps *ParamSet, m map[string]string, k ParamKey) (*ir.Filter, bool) {
	if !filterBinderSealed.Load() {
		filterBinderSealed.Store(true)
	}
	bind := filterBinder
	if bind == nil || !IsFilterKey(k) {
		return nil, false
	}
	if !ps.bound(m) {
		text, ok := m[paramKeyNames[k]]
		if !ok {
			return nil, false
		}
		f, _ := bind(text)
		return f, f != nil
	}
	i, ok := ps.index(k)
	if !ok {
		return nil, false
	}
	slots := ps.filt.Load()
	if slots == nil {
		fresh := make([]Slot, len(ps.vals))
		if !ps.filt.CompareAndSwap(nil, &fresh) {
			slots = ps.filt.Load()
		} else {
			slots = &fresh
		}
	}
	slot, text := &(*slots)[i], ps.vals[i]
	v, hit := slot.Load(text)
	if !hit {
		f, refusal := bind(text)
		v = slot.Store(text, &boundFilter{f: f, refusal: refusal})
	}
	b := v.(*boundFilter)
	return b.f, b.f != nil
}

// FilterOf is key k's value lowered by the registered FilterBinder: nil and
// false when no binder is registered, k is absent or not a filter key, or the
// binder refused the text. A bound node answers the same pointer every call.
func (sa *SA) FilterOf(k ParamKey) (*ir.Filter, bool) { return ParamSetFilter(sa.ps, sa.Params, k) }

// FilterOf is key k's typed filter (SA.FilterOf).
func (t Trigger) FilterOf(k ParamKey) (*ir.Filter, bool) { return ParamSetFilter(t.ps, t.Params, k) }

// FilterOf is key k's typed filter (SA.FilterOf).
func (s Static) FilterOf(k ParamKey) (*ir.Filter, bool) { return ParamSetFilter(s.ps, s.Params, k) }

// FilterOf is key k's typed filter (SA.FilterOf).
func (r Repl) FilterOf(k ParamKey) (*ir.Filter, bool) { return ParamSetFilter(r.ps, r.Params, k) }
