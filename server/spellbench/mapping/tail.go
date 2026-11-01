package mapping

// The choose-tail builders: the named-option asks whose kind is not one of the
// older choose classes and whose shape the one-pick catch-alls cannot express.
// Each maps to an existing v2 kind only:
//
//   - card/pair/reveal/enlist/proliferate are selections (select_object over
//     the offered objects or players, finish_selection to decline). They are
//     the Min 0 asks the single-pick catch-alls deliberately leave on the
//     unmapped fallback: finish_selection is what makes the empty answer
//     reachable (Section 7.5).
//   - convoke/activation_creature (and the other cast contributions) are one
//     select_object per contribution object, with the contribution option the
//     engine's own probe accepts at that step. One creature can carry both a
//     coloured convoke option and a generic one, so the candidate is per
//     OBJECT, not per native option: the first option of that object whose
//     single-pick probe still reduces the outstanding cost.
//   - move_counter is a choose_number over the offered amounts.

import (
	"fmt"
	"strings"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/server/spellbench/observe"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
)

func init() {
	Register("choose/card", tailSelect("other", true))
	Register("choose/pair", tailSelect("other", true))
	Register("choose/reveal", tailSelect("reveal", true))
	Register("choose/enlist", tailSelect("tap", true))
	Register("choose/proliferate", tailSelect("other", true))
	Register("choose/payment", newPayment)
}

// tailSelect is mixedSelect for the choose-tail kinds: the same select_object
// / finish_selection pick, but a label-only option with no object is the
// decline the finish candidate already expresses, and a player option is a
// player target. Hidden options take the look path with Known entries, as
// mixedSelect does for a revealed card.
func tailSelect(purp string, withSource bool) Builder {
	return func(env *Env, d *decision.Decision) (Transaction, error) {
		var src *protocol.ObjectRef
		if withSource {
			var err error
			if src, err = ResolveSource(env, d); err != nil {
				return nil, err
			}
		}
		lo, hi := uint32(d.Min), uint32(d.Max)
		refs := make(map[int]protocol.TargetRef, len(d.Options))
		hidden := map[int]bool{}
		opts := make([]int, 0, len(d.Options))
		var known []protocol.Known
		look := false
		for _, o := range d.Options {
			switch {
			case o.Obj != 0:
				obj := env.G.R.Board().Obj(o.Obj)
				if obj == nil {
					return nil, fmt.Errorf("%w:%s option %d has no object", ErrUnmapped, purp, o.Index)
				}
				if !observe.Visible(d.Player, obj) {
					if !look {
						env.OpenLook(d.Player)
						look = true
					}
					r, err := env.Obs.LookRef(d.Player, o.Obj)
					if err != nil {
						return nil, err
					}
					refs[o.Index] = protocol.ObjectTarget(r)
					hidden[o.Index] = true
					known = append(known, observe.KnownEntry(r, "revealed", knownPosition(env, obj)))
					opts = append(opts, o.Index)
					continue
				}
				r, err := env.Obs.Ref(d.Player, o.Obj)
				if err != nil || r == nil {
					return nil, fmt.Errorf("%w:%s object not visible", ErrUnmapped, purp)
				}
				refs[o.Index] = protocol.ObjectTarget(*r)
				opts = append(opts, o.Index)
			case o.Kind == "player" || d.ResumeKind == "proliferate":
				// A player recipient (proliferate) is a player target; the
				// engine reads the chosen option's Player, not an object.
				refs[o.Index] = protocol.PlayerTarget(observe.Seat(o.Player))
				opts = append(opts, o.Index)
			case d.Min == 0:
				// A label-only option is the decline ("Don't enlist"): the
				// finish_selection candidate commits the empty answer.
			default:
				return nil, fmt.Errorf("%w:%s option %d names no object", ErrUnmapped, purp, o.Index)
			}
		}
		if len(opts) == 0 {
			return nil, fmt.Errorf("%w:%s no selectable option", ErrUnmapped, purp)
		}
		observe.SortKnown(known)
		return NewPick(env, PickSpec{D: d, Options: opts, Known: known, Look: look,
			Context: protocol.Context{Kind: "choice", Source: src, Purpose: &purp},
			Sem: func(opt int, sel uint32) (Cand, error) {
				c := Cand{Sem: protocol.SelectObject(src, purp, refs[opt], sel, lo, hi)}
				if hidden[opt] {
					r := refs[opt].Object
					c.Hidden, c.SortName, c.SortID = true, *r.CardName, r.ObjectID
				}
				return c, nil
			},
			Finish: func(sel uint32) protocol.Semantic { return protocol.FinishSelection(src, purp, sel) }}), nil
	}
}

// paymentKinds reports whether every option is a cast contribution
// (Convoke/Harmonize/Improvise/waterbend or TapCreaturesForMana). These are
// the asks whose options mix coloured and generic contributions for the same
// object, so the option route's labels cannot express them.
func paymentKinds(k map[string]bool) bool {
	if k["activation_creature"] || k["harmonize"] || k["improvise_generic"] || k["waterbend_generic"] {
		return true
	}
	for kind := range k {
		if strings.HasPrefix(kind, "convoke_") {
			return true
		}
	}
	return false
}

// paymentTx poses one select_object candidate per contribution object. The
// native option committed for that object is the first of its options whose
// single-pick probe the engine accepts, so a creature that can pay either a
// coloured pip or a generic one contributes whichever the outstanding cost
// still absorbs, and an object whose every contribution would reduce nothing
// is not offered.
type paymentTx struct {
	env    *Env
	d      *decision.Decision
	src    *protocol.ObjectRef
	purp   string
	order  []state.ObjID
	groups map[state.ObjID][]int
	refs   map[int]protocol.ObjectRef
	chosen []int
	pose   *Pose
	memo   map[string]bool
}

func newPayment(env *Env, d *decision.Decision) (Transaction, error) {
	src, err := ResolveSource(env, d)
	if err != nil {
		return nil, err
	}
	purp := "tap"
	for _, o := range d.Options {
		if strings.HasPrefix(o.Kind, "convoke_") {
			purp = "convoke"
			break
		}
	}
	t := &paymentTx{env: env, d: d, src: src, purp: purp,
		groups: map[state.ObjID][]int{}, refs: map[int]protocol.ObjectRef{}, memo: map[string]bool{}}
	for _, o := range d.Options {
		if o.Obj == 0 {
			return nil, fmt.Errorf("%w:payment option %d names no object", ErrUnmapped, o.Index)
		}
		if _, ok := t.groups[o.Obj]; !ok {
			t.order = append(t.order, o.Obj)
		}
		t.groups[o.Obj] = append(t.groups[o.Obj], o.Index)
		r, err := env.Obs.Ref(d.Player, o.Obj)
		if err != nil || r == nil {
			return nil, fmt.Errorf("%w:payment object not visible", ErrUnmapped)
		}
		t.refs[o.Index] = *r
	}
	if len(t.order) == 0 {
		return nil, fmt.Errorf("%w:payment has no contribution", ErrUnmapped)
	}
	return t, nil
}

func (t *paymentTx) accepts(xs []int) bool {
	k := key(xs)
	if v, ok := t.memo[k]; ok {
		return v
	}
	v := Accepts(t.env, Intent(t.d, xs...))
	t.memo[k] = v
	return v
}

func (t *paymentTx) chosenObject(obj state.ObjID) bool {
	for _, opt := range t.chosen {
		if t.d.Options[opt].Obj == obj {
			return true
		}
	}
	return false
}

func (t *paymentTx) Pose() (*Pose, error) {
	d := t.d
	p := &Pose{Seat: d.Player, Context: choice(t.src, t.purp), Native: d}
	fixed := d.Min > 0 && d.Min == d.Max
	if fixed {
		p.GroupStart, p.SubstepIndex, p.SubstepCount = len(t.chosen) == 0, uint32(len(t.chosen)), uint32(d.Max)
	} else {
		p.GroupStart, p.SubstepCount = true, 1
	}
	sel := uint32(len(t.chosen))
	if len(t.chosen) < d.Max {
		for _, obj := range t.order {
			if t.chosenObject(obj) {
				continue
			}
			for _, opt := range t.groups[obj] {
				trial := append(append([]int(nil), t.chosen...), opt)
				if !t.accepts(trial) {
					continue
				}
				p.Candidates = append(p.Candidates, Cand{
					Sem: protocol.SelectObject(t.src, t.purp, protocol.ObjectTarget(t.refs[opt]), sel, uint32(d.Min), uint32(d.Max)),
					Op:  NativeOp{Op: "choose", Option: opt}})
				break
			}
		}
	}
	if !fixed && len(t.chosen) >= d.Min && t.accepts(t.chosen) {
		p.Candidates = append(p.Candidates, Cand{
			Sem: protocol.FinishSelection(t.src, t.purp, sel), Op: NativeOp{Op: "finish", Option: -1}})
	}
	if err := Finalize(p); err != nil {
		return nil, err
	}
	t.pose = p
	return p, nil
}

func (t *paymentTx) Answer(i int) ([]decision.Intent, bool, error) {
	op := t.pose.Candidates[i].Op
	if op.Op == "finish" {
		return []decision.Intent{Intent(t.d, t.chosen...)}, true, nil
	}
	t.chosen = append(t.chosen, op.Option)
	if len(t.chosen) >= t.d.Max {
		return []decision.Intent{Intent(t.d, t.chosen...)}, true, nil
	}
	return nil, false, nil
}
