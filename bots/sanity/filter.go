// Package sanity is the common-sense layer of the hand-written hosted bots.
// It is a seat decorator, applied by the registry factories of the heuristic
// policies only (bot, forge-heuristic, sb-tactical) and never by
// the search or trained policies, nor by shared bot code the search policies
// reuse as their fallback or opponent model.
//
// The one rule: do not offer a cast or activation whose FIRST targeting step is
// a hostile effect (direct damage; destroy, exile, graveyard or sacrifice
// removal) when no legal target is controlled by an opponent but at least one
// is the seat's own. The engine publishes the counts as
// decision.Option.TargetHints (rules.Engine.EnsureTargetHints, requested through
// seat.TargetHintConsumer); the engine itself filters nothing. A small
// per-card allow-list exempts cards whose self-target is the point.
package sanity

import (
	"slices"
	"strings"

	"github.com/adams-shaun/gorge/engine/decision"
)

// Allow lists, SORTED, the card names the guard never drops: spells and
// abilities whose use on the seat's own permanent is deliberate (sacrifice
// outlets, blink, land thinning). It is data; see allow.go.
func allowed(name string) bool {
	_, ok := slices.BinarySearch(allowList, name)
	return ok
}

// cardName extracts the card name from a cast or ability option label:
// "Cast <name>[ (mode)]" and "<name>: <description>".
func cardName(o *decision.Option) string {
	l := o.Label
	if o.Kind == "cast" {
		l = strings.TrimPrefix(l, "Cast ")
		if i := strings.Index(l, " ("); i >= 0 {
			l = l[:i]
		}
		return l
	}
	if i := strings.Index(l, ": "); i >= 0 {
		return l[:i]
	}
	return l
}

// Drops reports whether the guard removes this option.
func Drops(o *decision.Option) bool {
	if o.Kind != "cast" && o.Kind != "ability" {
		return false
	}
	return dropsHint(o.TargetHints, o)
}

func dropsHint(h decision.TargetHints, o *decision.Option) bool {
	if !h.Present() || !h.Hostile() || h.Opp() != 0 || h.Own() == 0 {
		return false
	}
	return !allowed(cardName(o))
}

// Filtered is a decision with the guard's drops applied. keep[i] is the
// original index of filtered option i; a nil keep means nothing was dropped
// and D is the input.
type Filtered struct {
	D    decision.Decision
	Keep []int
}

// Filter removes the options Drops selects from a priority decision, with
// everything that indexes them: Option.Index is renumbered, a payment action's
// BaseOptionIndex is remapped, and an action linked to a dropped option is
// dropped with it. An action with no linked option (a floating-mana plan) is
// dropped when its own hint says so. Pass is
// never dropped. The input's slices are not modified.
func Filter(d decision.Decision) Filtered {
	if d.Kind != decision.KPriority {
		return Filtered{D: d}
	}
	any := false
	for i := range d.Options {
		if Drops(&d.Options[i]) {
			any = true
			break
		}
	}
	if !any {
		// A floating-mana payment action carries its own hint.
		for i := range d.PaymentActions {
			if pa := &d.PaymentActions[i]; pa.BaseOptionIndex == nil &&
				dropsHint(pa.TargetHints, &decision.Option{Kind: "cast", Label: pa.Label}) {
				any = true
				break
			}
		}
	}
	if !any {
		return Filtered{D: d}
	}
	remap := make([]int, len(d.Options))
	keep := make([]int, 0, len(d.Options))
	opts := make([]decision.Option, 0, len(d.Options))
	for i := range d.Options {
		if Drops(&d.Options[i]) {
			remap[i] = -1
			continue
		}
		remap[i] = len(opts)
		keep = append(keep, i)
		o := d.Options[i]
		o.Index = len(opts)
		opts = append(opts, o)
	}
	out := d
	out.Options = opts
	if len(d.PaymentActions) > 0 {
		acts := make([]decision.PaymentAction, 0, len(d.PaymentActions))
		for _, pa := range d.PaymentActions {
			if pa.BaseOptionIndex != nil {
				b := *pa.BaseOptionIndex
				if b < 0 || b >= len(remap) || remap[b] < 0 {
					continue
				}
				nb := remap[b]
				pa.BaseOptionIndex = &nb
			} else if dropsHint(pa.TargetHints, &decision.Option{Kind: "cast", Label: pa.Label}) {
				continue
			}
			acts = append(acts, pa)
		}
		out.PaymentActions = acts
	}
	return Filtered{D: out, Keep: keep}
}

// MapBack rewrites an intent chosen against the filtered decision into the
// original decision's option numbering. It returns false when a choice is out
// of range (the caller returns the intent unchanged and lets validation
// refuse it, as it would have).
func (f Filtered) MapBack(in decision.Intent) decision.Intent {
	if f.Keep == nil {
		return in
	}
	choices := make([]int, len(in.Choices))
	for k, c := range in.Choices {
		if c < 0 || c >= len(f.Keep) {
			return in
		}
		choices[k] = f.Keep[c]
	}
	in.Choices = choices
	return in
}

// MapIn is the inverse for an intent (a refused answer) that was made against
// the ORIGINAL numbering; choices naming a dropped option are left as they are.
func (f Filtered) MapIn(in decision.Intent) decision.Intent {
	if f.Keep == nil {
		return in
	}
	choices := make([]int, len(in.Choices))
	for k, c := range in.Choices {
		i, ok := slices.BinarySearch(f.Keep, c)
		if !ok {
			return in
		}
		choices[k] = i
	}
	in.Choices = choices
	return in
}

// CardName is the card name an option's label carries (exported for tools).
func CardName(o *decision.Option) string { return cardName(o) }
