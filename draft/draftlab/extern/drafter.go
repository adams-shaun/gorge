package extern

import (
	"context"
	"fmt"
	"math/rand/v2"

	"github.com/adams-shaun/gorge/draft/draftlab"
	"github.com/adams-shaun/gorge/draft/draftlab/extern/wire"
	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/pickbot"
)

// Drafter wraps the agent as a draftlab.NamedDrafter: its adapter answers a
// draft seat's picks over the wire, falling back to the lab's
// rating-colour@1 drafter on every refusal (and, under FailFallback, on
// every session failure). The agent's hello must serve the "pick" kind;
// ErrRole otherwise.
func (a *Agent) Drafter(l *draftlab.Lab) (draftlab.NamedDrafter, error) {
	if !servesKind(a.hello, wire.KindPick) {
		return draftlab.NamedDrafter{}, fmt.Errorf("%w: agent %q serves %v, not %q",
			ErrRole, a.hello.Agent, a.hello.Kinds, wire.KindPick)
	}
	fb, err := l.Drafter(draftlab.AgentRef{Name: "rating-colour", Version: "1"})
	if err != nil {
		return draftlab.NamedDrafter{}, fmt.Errorf("extern: the pick fallback drafter: %w", err)
	}
	return draftlab.NamedDrafter{
		Ref: a.Ref(),
		Drafter: &extDrafter{
			a: a, lab: l,
			fallback: fb.Drafter,
		},
	}, nil
}

// extDrafter is one Agent's pick adapter. It implements pickbot.Drafter and
// draftlab.FallibleDrafter; the draft loop answers through PickErr.
type extDrafter struct {
	a        *Agent
	lab      *draftlab.Lab
	fallback pickbot.Drafter
}

// Pick answers one pick; an error (a refusal under StrictRefusals, or a
// session failure under FailError) is reported as "", which the draft loop
// then refuses.
func (d *extDrafter) Pick(v pickbot.PickView, r *rand.Rand) string {
	id, err := d.PickErr(v, r)
	if err != nil {
		return ""
	}
	return id
}

// PickErr answers one pick over the wire: the item carries the seat's
// PickView as DraftCard lists. An error answer, a pick outside the pack, a
// session failure (OnFailure) or — under StrictRefusals — any refusal is
// the error; otherwise the fallback drafter answers.
func (d *extDrafter) PickErr(v pickbot.PickView, r *rand.Rand) (string, error) {
	item := wire.Item{
		Kind:   wire.KindPick,
		Seed:   wire.Seed53(r.Uint64()),
		Set:    v.Set,
		Round:  v.Round,
		PickNo: v.PickNo,
		Pack:   draftCards(d.lab, v.Pack),
		Picks:  draftCards(d.lab, v.Picks),
		Seen:   draftCardLists(d.lab, v.Seen),
	}
	ans, err := d.a.call(context.Background(), nil, item)
	if err != nil {
		var out string
		err = d.a.failed(err, wire.KindPick, func() { out = d.fallback.Pick(v, r) })
		return out, err
	}
	if ans.Error != nil {
		var out string
		if err := d.a.refused(wire.KindPick, itemErrorReason(ans.Error), func() { out = d.fallback.Pick(v, r) }); err != nil {
			return "", err
		}
		return out, nil
	}
	if !packContains(v.Pack, ans.Pick) {
		reason := fmt.Sprintf("pick %q is not in the pack", ans.Pick)
		var out string
		if err := d.a.refused(wire.KindPick, reason, func() { out = d.fallback.Pick(v, r) }); err != nil {
			return "", err
		}
		return out, nil
	}
	return ans.Pick, nil
}

// servesKind reports whether the hello declares the kind.
func servesKind(hello wire.Message, kind string) bool {
	for _, k := range hello.Kinds {
		if k == kind {
			return true
		}
	}
	return false
}

// packContains reports whether id is one of the pack's card ids.
func packContains(pack []limited.CardRef, id string) bool {
	for _, c := range pack {
		if c.ID == id {
			return true
		}
	}
	return false
}

// itemErrorReason phrases an answer's error for a refusal log line.
func itemErrorReason(e *wire.ItemError) string {
	if e.Message == "" {
		return e.Code
	}
	return e.Code + ": " + e.Message
}
