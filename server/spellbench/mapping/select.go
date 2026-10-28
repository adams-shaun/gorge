package mapping

import (
	"fmt"
	"slices"
	"strings"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/server/spellbench/observe"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
)

var handDestPurpose = map[string]string{"Hand": "put_into_hand", "Battlefield": "put_onto_battlefield",
	"Graveyard": "put_into_graveyard"}

func init() {
	Register("choose/cleanup_discard", visibleSelect("discard", false))
	Register("modes/discard", mixedSelect("discard", true))
	Register("choose/untap", visibleSelect("untap", true))
	Register("choose/keep", visibleSelect("legend_rule", true))
	Register("choose/search", newSearch)
	Register("choose/hand_move", newHandMove)
	Register("choose/hidden_pick", newHiddenPick)
	Register("choose/vote_card", mixedSelect("vote", true))
	Register("modes/mode", newModes)
}

// openLookIfHidden opens the seat's look when any option names a card in a
// zone hidden from it. x_gorge_view_v1 rekeys every option object, so a pose
// with hidden options needs the look even when its candidates do not name
// one (a yes/no ask about a revealed top card).
func openLookIfHidden(env *Env, d *decision.Decision) {
	for _, o := range d.Options {
		if o.Obj == 0 {
			continue
		}
		if obj := env.G.E.G.Obj(o.Obj); obj != nil && !observe.Visible(d.Player, obj) {
			env.OpenLook(d.Player)
			return
		}
	}
}

// mixedSelect is visibleSelect for a pick whose cards may sit in a zone hidden
// from the chooser: each hidden option is referenced through the seat's look,
// as hiddenSelect does, while visible options keep their visible ids. The
// purposes are the protocol's select_object vocabulary.
func mixedSelect(purp string, withSource bool) Builder {
	return func(env *Env, d *decision.Decision) (Transaction, error) {
		var src *protocol.ObjectRef
		if withSource {
			var err error
			if src, err = ResolveSource(env, d); err != nil {
				return nil, err
			}
		}
		lo, hi := uint32(d.Min), uint32(d.Max)
		refs := make([]protocol.ObjectRef, len(d.Options))
		var known []protocol.Known
		look := false
		for i, o := range d.Options {
			obj := env.G.E.G.Obj(o.Obj)
			if obj == nil {
				return nil, fmt.Errorf("%w: %s option %d has no object", ErrUnmapped, purp, o.Index)
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
				refs[i] = r
				known = append(known, observe.KnownEntry(r, "revealed", nil))
				continue
			}
			r, err := env.Obs.Ref(d.Player, o.Obj)
			if err != nil || r == nil {
				return nil, fmt.Errorf("%w: %s object not visible", ErrUnmapped, purp)
			}
			refs[i] = *r
		}
		observe.SortKnown(known)
		return NewPick(env, PickSpec{D: d, Options: allOptions(d), Known: known, Look: look,
			Context: protocol.Context{Kind: "choice", Source: src, Purpose: &purp},
			Sem: func(opt int, sel uint32) (Cand, error) {
				r := refs[opt]
				return Cand{Sem: protocol.SelectObject(src, purp, protocol.ObjectTarget(r), sel, lo, hi)}, nil
			},
			Finish: func(sel uint32) protocol.Semantic { return protocol.FinishSelection(src, purp, sel) }}), nil
	}
}

// newHiddenPick: a hidden-origin ChangeZone pick (zone_hidden's "hidden_pick"
// resume). Its cards may be public (a graveyard) or revealed from a hidden
// zone, so mixedSelect decides each option's reference; the destination names
// the select_object purpose.
func newHiddenPick(env *Env, d *decision.Decision) (Transaction, error) {
	purp := "other"
	if d.ResumeSA != nil {
		switch strings.ToLower(strings.TrimSpace(d.ResumeSA.Params["Destination"])) {
		case "hand":
			purp = "put_into_hand"
		case "battlefield":
			purp = "put_onto_battlefield"
		case "graveyard":
			purp = "put_into_graveyard"
		case "exile":
			purp = "exile"
		case "library":
			purp = "search"
		}
	}
	return mixedSelect(purp, true)(env, d)
}

func visibleSelect(purp string, withSource bool) Builder {
	return func(env *Env, d *decision.Decision) (Transaction, error) {
		var src *protocol.ObjectRef
		if withSource {
			var err error
			if src, err = ResolveSource(env, d); err != nil {
				return nil, err
			}
		}
		lo, hi := uint32(d.Min), uint32(d.Max)
		return NewPick(env, PickSpec{D: d, Options: allOptions(d),
			Context: protocol.Context{Kind: "choice", Source: src, Purpose: &purp},
			Sem: func(opt int, sel uint32) (Cand, error) {
				r, err := env.Obs.Ref(d.Player, d.Options[opt].Obj)
				if err != nil || r == nil {
					return Cand{}, fmt.Errorf("%w: %s object not visible", ErrUnmapped, purp)
				}
				return Cand{Sem: protocol.SelectObject(src, purp, protocol.ObjectTarget(*r), sel, lo, hi)}, nil
			},
			Finish: func(sel uint32) protocol.Semantic { return protocol.FinishSelection(src, purp, sel) }}), nil
	}
}

// hiddenSelect presents options whose objects sit in a zone hidden from the
// chooser (a library, or the other seat's hand) under look ids.
func hiddenSelect(env *Env, d *decision.Decision, purp, how string) (Transaction, error) {
	src, err := ResolveSource(env, d)
	if err != nil {
		return nil, err
	}
	env.OpenLook(d.Player)
	refs := map[int]protocol.ObjectRef{}
	var known []protocol.Known
	for i, o := range d.Options {
		r, err := env.Obs.LookRef(d.Player, o.Obj)
		if err != nil {
			return nil, err
		}
		refs[i] = r
		known = append(known, observe.KnownEntry(r, how, nil))
	}
	observe.SortKnown(known)
	lo, hi := uint32(d.Min), uint32(d.Max)
	return NewPick(env, PickSpec{D: d, Options: allOptions(d), Known: known, Look: true,
		Context: protocol.Context{Kind: "choice", Source: src, Purpose: &purp},
		Sem: func(opt int, sel uint32) (Cand, error) {
			r := refs[opt]
			return Cand{Sem: protocol.SelectObject(src, purp, protocol.ObjectTarget(r), sel, lo, hi),
				Hidden: true, SortName: *r.CardName, SortID: r.ObjectID}, nil
		},
		Finish: func(sel uint32) protocol.Semantic { return protocol.FinishSelection(src, purp, sel) }}), nil
}

func newSearch(env *Env, d *decision.Decision) (Transaction, error) {
	return hiddenSelect(env, d, "search", "searching")
}

func newHandMove(env *Env, d *decision.Decision) (Transaction, error) {
	dest := ""
	if d.ResumeSA != nil {
		dest = d.ResumeSA.Params["Destination"]
	}
	owner := env.G.E.G.Obj(d.Options[0].Obj).Owner
	switch {
	case dest == "Library":
		if f, ok := builders["hand_move/library"]; ok {
			return f(env, d)
		}
	case dest == "Exile" && owner != d.Player:
		return hiddenSelect(env, d, "exile", "revealed")
	case owner == d.Player && handDestPurpose[dest] != "":
		return visibleSelect(handDestPurpose[dest], true)(env, d)
	}
	return nil, fmt.Errorf("%w:hand_move/%s", ErrUnmapped, dest)
}

// newModes poses one choose_spell_mode per pick. gorge offers only the
// eligible modes (Thraben Charm without a creature to target starts at its
// second mode), so a candidate names the printed mode and count (Section
// 7.3), the numbering the stack entry's modes use (Task 12).
func newModes(env *Env, d *decision.Decision) (Transaction, error) {
	src, err := MustSource(env, d)
	if err != nil {
		return nil, err
	}
	index, n, err := PrintedModes(d)
	if err != nil {
		return nil, err
	}
	lo, hi := uint32(d.Min), uint32(d.Max)
	purp := "modes"
	return NewPick(env, PickSpec{D: d, Options: allOptions(d), Context: protocol.Context{Kind: "choice", Source: &src},
		Sem: func(opt int, sel uint32) (Cand, error) {
			return Cand{Sem: protocol.ChooseSpellMode(src, index[opt], n, sel, lo, hi)}, nil
		},
		Finish: func(sel uint32) protocol.Semantic { return protocol.FinishSelection(&src, purp, sel) }}), nil
}

// PrintedModes maps each offered mode option to its index among the modal
// ability's printed modes (ResumeSA's Choices, in Oracle order), and returns
// the printed mode count. gorge names each offered mode in ResumeModes; a
// decision without them offers every printed mode, densely.
func PrintedModes(d *decision.Decision) ([]uint32, uint32, error) {
	index := make([]uint32, len(d.Options))
	for i := range index {
		index[i] = uint32(i)
	}
	if len(d.ResumeModes) == 0 || d.ResumeSA == nil {
		return index, uint32(len(d.Options)), nil
	}
	var printed []string
	for _, c := range strings.Split(d.ResumeSA.Params["Choices"], ",") {
		printed = append(printed, strings.TrimSpace(c))
	}
	if len(d.ResumeModes) != len(d.Options) {
		return nil, 0, fmt.Errorf("%w:modes_layout", ErrUnmapped)
	}
	for i, name := range d.ResumeModes {
		k := slices.Index(printed, strings.TrimSpace(name))
		if k < 0 {
			return nil, 0, fmt.Errorf("%w:modes_layout", ErrUnmapped)
		}
		index[i] = uint32(k)
	}
	return index, uint32(len(printed)), nil
}
