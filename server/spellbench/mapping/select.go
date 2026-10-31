package mapping

import (
	"fmt"
	"slices"
	"strings"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/server/spellbench/observe"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
)

var handDestPurpose = map[string]string{"Hand": "put_into_hand", "Battlefield": "put_onto_battlefield",
	"Graveyard": "put_into_graveyard", "Exile": "exile"}

func init() {
	Register("choose/cleanup_discard", visibleSelect("discard", false))
	Register("modes/discard", mixedSelect("discard", true))
	Register("choose/untap", visibleSelect("untap", true))
	Register("choose/keep", visibleSelect("legend_rule", true))
	Register("choose/search", newSearch)
	Register("choose/hand_move", newHandMove)
	Register("choose/hidden_pick", newHiddenPick)
	Register("choose/twopiles", mixedSelect("other", true))
	Register("choose/pile_pick", newTwoPilePick)
	Register("choose/vote_card", mixedSelect("vote", true))
	Register("modes/mode", newModes)
}

// openLookIfHidden opens the seat's look when any option names a card in a
// zone hidden from it. x_gorge_view_v1 rekeys every option object, so a pose
// with hidden options needs the look even when its candidates do not name
// one (a yes/no ask about a revealed top card).
// knownPosition is the Section 6.7 position_from_top every known LIBRARY
// entry must carry (validate V5). A card in any other zone has no library
// position, so it stays nil (a hand entry).
func knownPosition(env *Env, obj *state.Object) *uint32 {
	if obj == nil || obj.Zone != state.ZLibrary {
		return nil
	}
	lib := env.G.R.Board().Zone(state.ZLibrary, obj.Owner)
	if at := slices.Index(lib, obj.ID); at >= 0 {
		p := uint32(at)
		return &p
	}
	return nil
}

func openLookIfHidden(env *Env, d *decision.Decision) {
	for _, o := range d.Options {
		if o.Obj == 0 {
			continue
		}
		if obj := env.G.R.Board().Obj(o.Obj); obj != nil && !observe.Visible(d.Player, obj) {
			env.OpenLook(d.Player)
			return
		}
	}
}

// mixedSelect is visibleSelect for a pick whose cards may sit in a zone hidden
// from the chooser: each hidden option is referenced through the seat's look,
// as hiddenSelect does, while visible options keep their visible ids. The
// purposes are the protocol's select_object vocabulary.
//
// The ask is the reveal: effects/cardflow.go's discardAsk poses its KModes
// "discard" decision to the chooser (Mode$ RevealYouChoose and siblings) over
// the discarder's hand, already narrowed to the cards the effect showed. So a
// hidden option here is exactly a card this seat was revealed, and it takes
// the look path plus a "revealed" known entry; no separate reveal event
// exists for that shape.
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
		hidden := make([]bool, len(d.Options))
		var known []protocol.Known
		look := false
		for i, o := range d.Options {
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
				refs[i] = r
				hidden[i] = true
				known = append(known, observe.KnownEntry(r, "revealed", knownPosition(env, obj)))
				continue
			}
			r, err := env.Obs.Ref(d.Player, o.Obj)
			if err != nil || r == nil {
				return nil, fmt.Errorf("%w:%s object not visible", ErrUnmapped, purp)
			}
			refs[i] = *r
		}
		observe.SortKnown(known)
		return NewPick(env, PickSpec{D: d, Options: allOptions(d), Known: known, Look: look,
			Context: protocol.Context{Kind: "choice", Source: src, Purpose: &purp},
			Sem: func(opt int, sel uint32) (Cand, error) {
				r := refs[opt]
				c := Cand{Sem: protocol.SelectObject(src, purp, protocol.ObjectTarget(r), sel, lo, hi)}
				if hidden[opt] {
					// Section 7.1: a candidate naming a hidden-zone card
					// carries the (card_name, object_id) order Finalize and V5
					// impose, as hiddenSelect does.
					c.Hidden, c.SortName, c.SortID = true, cardName(r), r.ObjectID
				}
				return c, nil
			},
			Finish: func(sel uint32) protocol.Semantic { return protocol.FinishSelection(src, purp, sel) }}), nil
	}
}

// newHiddenPick: a hidden-origin ChangeZone pick (zone_hidden's "hidden_pick"
// resume). Its cards may be public (a graveyard) or revealed from a hidden
// zone, so mixedSelect decides each option's reference; the destination names
// the select_object purpose.
// newTwoPilePick maps the pile pick after a TwoPiles split (KChoose Min 1 Max
// 1 over "pile-a"/"pile-b") to the protocol's counted choose_option shape.
// The pile CONTENTS are not on the pick decision, so the protocol's
// choose_pile (which carries both card lists) cannot be built here; the
// purpose names the effect election and each option keeps the engine's label.
func newTwoPilePick(env *Env, d *decision.Decision) (Transaction, error) {
	src, err := ResolveSource(env, d)
	if err != nil {
		return nil, err
	}
	const purp = "effect_option"
	count := uint32(len(d.Options))
	return SingleChoice(env, d, choice(src, purp), func(o decision.Option) (protocol.Semantic, bool, error) {
		label := o.Label
		return protocol.ChooseOption(src, purp, uint32(o.Index), count, &label), true, nil
	})
}

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
					return Cand{}, fmt.Errorf("%w:%s object not visible", ErrUnmapped, purp)
				}
				return Cand{Sem: protocol.SelectObject(src, purp, protocol.ObjectTarget(*r), sel, lo, hi)}, nil
			},
			Finish: func(sel uint32) protocol.Semantic { return protocol.FinishSelection(src, purp, sel) }}), nil
	}
}

// hiddenSelect presents a pick whose options may sit in a zone hidden from the
// chooser (a library, or the other seat's hand) under look ids. An option the
// chooser already sees keeps its visible id, as mixedSelect does: a search
// whose Origin$ is Library,Graveyard offers both, and a look id for the
// graveyard card would be refused (it already has one).
func hiddenSelect(env *Env, d *decision.Decision, purp, how string) (Transaction, error) {
	src, err := ResolveSource(env, d)
	if err != nil {
		return nil, err
	}
	refs := make([]protocol.ObjectRef, len(d.Options))
	hidden := make([]bool, len(d.Options))
	var known []protocol.Known
	look := false
	for i, o := range d.Options {
		obj := env.G.R.Board().Obj(o.Obj)
		if obj != nil && observe.Visible(d.Player, obj) {
			r, err := env.Obs.Ref(d.Player, o.Obj)
			if err != nil {
				return nil, err
			}
			if r == nil {
				return nil, fmt.Errorf("%w:%s object not visible", ErrUnmapped, purp)
			}
			refs[i] = *r
			continue
		}
		if !look {
			env.OpenLook(d.Player)
			look = true
		}
		r, err := env.Obs.LookRef(d.Player, o.Obj)
		if err != nil {
			return nil, err
		}
		refs[i] = r
		hidden[i] = true
		known = append(known, observe.KnownEntry(r, how, nil))
	}
	observe.SortKnown(known)
	lo, hi := uint32(d.Min), uint32(d.Max)
	return NewPick(env, PickSpec{D: d, Options: allOptions(d), Known: known, Look: look,
		Context: protocol.Context{Kind: "choice", Source: src, Purpose: &purp},
		Sem: func(opt int, sel uint32) (Cand, error) {
			r := refs[opt]
			c := Cand{Sem: protocol.SelectObject(src, purp, protocol.ObjectTarget(r), sel, lo, hi)}
			if hidden[opt] {
				c.Hidden, c.SortName, c.SortID = true, cardName(r), r.ObjectID
			}
			return c, nil
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
	owner := env.G.R.Board().Obj(d.Options[0].Obj).Owner
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
	// Section 7.3's choose_spell_mode constraint is minimum <= maximum <=
	// mode_count (protocol/kinds.go checkConstraints). A CanRepeatModes$ ask
	// may legitimately want more picks than the spell has distinct modes
	// (Season of Loss is CharmNum$ 5 over three Choices$); v2 has no
	// repeatable flag, so no candidate for it can pass the validator.
	// Decline the decision instead of posing candidates every host rejects.
	// Clamping is not an option: the pick after mode_count would carry
	// selected_count == maximum and fail selected_count < maximum.
	if lo > hi || hi > n {
		return nil, fmt.Errorf("%w:modes_over_printed(min=%d,max=%d,printed=%d,repeat=%t)", ErrUnmapped, lo, hi, n, d.Repeatable)
	}
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
