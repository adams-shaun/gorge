package mapping

import (
	"fmt"
	"slices"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/server/spellbench/observe"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
)

var colors = map[string]string{"white": "white", "blue": "blue", "black": "black", "red": "red", "green": "green"}

var boolPurpose = map[string]string{"search_confirm": "may_ability", "copy_optional": "may_ability",
	"repeat_optional": "may_ability", "search_mayshuffle": "other"}

// chooseTypePurposes maps a ChooseType Type$ category to the choose_name
// purpose the protocol vocabulary defines (server/spellbench/protocol/kinds.go).
// Keys are the engine's own spellings (lowercased); an unknown category is
// "other", the vocabulary's catch-all.
var chooseTypePurposes = map[string]string{
	"":                       "creature_type",
	"creature":               "creature_type",
	"creatureintargeteddeck": "creature_type",
	"basic land":             "basic_land_type",
	"land":                   "land_type",
	"nonbasic land":          "land_type",
	"card":                   "card_type",
	"shared":                 "card_type",
}

// chooseTypePurpose names the Type$ category an as-enters or mid-resolution
// ChooseType ask is posing. A decision without the SA (no in-tree producer)
// falls back to the family its option universe belongs to, so a projection
// never introduces the card_type value check where the engine did not say
// "card type"; an SA whose category the table does not know is "other".
func chooseTypePurpose(d *decision.Decision) string {
	if d == nil {
		return "other"
	}
	if d.ResumeSA == nil {
		return chooseTypeNamePurpose(d.Options)
	}
	if p, ok := chooseTypePurposes[strings.ToLower(strings.TrimSpace(d.ResumeSA.ParamStr(cards.PKType)))]; ok {
		return p
	}
	return "other"
}

func init() {
	Register("mulligan/keep", func(env *Env, d *decision.Decision) (Transaction, error) {
		hand := uint32(len(env.G.E.G.Zone(state.ZHand, d.Player)))
		taken := env.Obs.Mulls[d.Player]
		tx, err := SingleChoice(env, d, choice(nil, ""), func(o decision.Option) (protocol.Semantic, bool, error) {
			return protocol.Mulligan(hand, taken, o.Kind == "keep"), true, nil
		})
		if err != nil {
			return nil, err
		}
		st := tx.(*singleTx)
		st.after = func(opt int) {
			if d.Options[opt].Kind == "mulligan" {
				env.Obs.Mulls[d.Player]++
			}
		}
		return st, nil
	})
	Register("trigger_optional/optional", func(env *Env, d *decision.Decision) (Transaction, error) {
		src, err := ResolveSource(env, d)
		if err != nil {
			return nil, err
		}
		return SingleChoice(env, d, choice(src, "optional_trigger"), func(o decision.Option) (protocol.Semantic, bool, error) {
			return protocol.ChooseBoolean(src, "optional_trigger", o.Kind == "yes"), true, nil
		})
	})
	Register("trigger_optional/madness", func(env *Env, d *decision.Decision) (Transaction, error) {
		// optional_cast.card is the exiled card itself. ResolveSource would
		// answer the madness trigger's stack entry, whose source it is.
		r, err := env.Obs.Ref(d.Player, d.Source)
		if err != nil {
			return nil, err
		}
		if r == nil {
			return nil, ErrUnresolvableSource
		}
		card := *r
		return SingleChoice(env, d, choice(&card, ""), func(o decision.Option) (protocol.Semantic, bool, error) {
			return protocol.OptionalCast(card, "madness", o.Kind == "yes"), true, nil
		})
	})
	Register("starting_player/player", func(env *Env, d *decision.Decision) (Transaction, error) {
		return SingleChoice(env, d, choice(nil, ""), func(o decision.Option) (protocol.Semantic, bool, error) {
			return protocol.ChooseStartingPlayer(observe.Seat(o.Player)), o.Kind == "player", nil
		})
	})
	Register("choose/yesno", func(env *Env, d *decision.Decision) (Transaction, error) {
		openLookIfHidden(env, d)
		src, err := ResolveSource(env, d)
		if err != nil {
			return nil, err
		}
		p := boolPurpose[d.ResumeKind]
		if p == "" {
			p = "other"
		}
		return SingleChoice(env, d, choice(src, p), func(o decision.Option) (protocol.Semantic, bool, error) {
			return protocol.ChooseBoolean(src, p, o.Kind == "yes"), true, nil
		})
	})
	// choose/object and choose/option are the named-option catch-alls Route
	// places after every older choose class. An object ask names one visible
	// object (or player) per option; an option ask carries the option's Kind,
	// never its Label, which is presentation text.
	Register("choose/object", func(env *Env, d *decision.Decision) (Transaction, error) {
		// A named-object ask whose options include a hidden-zone card cannot
		// reference it with Obs.Ref. A library option is a card a resolving
		// look effect let this seat see; mixedSelect opens the seat's look and
		// poses the same select_object choices over look ids, with the Known
		// entries a v2 client needs.
		lookable := false
		for _, o := range d.Options {
			obj := env.G.E.G.Obj(o.Obj)
			if obj == nil || observe.Visible(d.Player, obj) {
				continue
			}
			// A library option is a card the resolving effect let this seat
			// look at (dig/search/reveal), so a look id is the right
			// reference. A hidden hand card is not projected here: the ask
			// may not have revealed it, and an unrevealed hand stays hidden.
			if obj.Zone != state.ZLibrary {
				lookable = false
				break
			}
			lookable = true
		}
		if lookable {
			return mixedSelect("other", true)(env, d)
		}
		src, err := ResolveSource(env, d)
		if err != nil {
			return nil, err
		}
		return SingleChoice(env, d, choice(src, "other"), func(o decision.Option) (protocol.Semantic, bool, error) {
			tg, err := TargetOf(env, d, o)
			if err != nil {
				return protocol.Semantic{}, false, err
			}
			return protocol.SelectObject(src, "other", tg, 0, 1, 1), true, nil
		})
	})
	Register("choose/option", func(env *Env, d *decision.Decision) (Transaction, error) {
		src, err := ResolveSource(env, d)
		if err != nil {
			return nil, err
		}
		count := uint32(len(d.Options))
		sem := func(o decision.Option) (protocol.Semantic, bool, error) {
			label := o.Kind
			return protocol.ChooseOption(src, "effect_option", uint32(o.Index), count, &label), true, nil
		}
		if d.Min > 1 {
			// A named-option ask that needs several answers (ChangeText's
			// from+to pair) is one choose_option per group substep.
			return &optionGroupTx{d: d, src: src}, nil
		}
		return SingleChoice(env, d, choice(src, "effect_option"), sem)
	})
	// choose/vote is the fixed-list ballot: one choose_option per ballot entry,
	// with the vocabulary's vote purpose. Selection is by option_index; the
	// engine reads the chosen Option's Obj (index+1), never the label (a fixed
	// ballot's Label is an internal SVar name).
	Register("choose/vote", func(env *Env, d *decision.Decision) (Transaction, error) {
		src, err := ResolveSource(env, d)
		if err != nil {
			return nil, err
		}
		count := uint32(len(d.Options))
		return SingleChoice(env, d, choice(src, "vote"), func(o decision.Option) (protocol.Semantic, bool, error) {
			label := o.Kind
			return protocol.ChooseOption(src, "vote", uint32(o.Index), count, &label), true, nil
		})
	})
	Register("replacement/madness", func(env *Env, d *decision.Decision) (Transaction, error) {
		src, err := ResolveSource(env, d)
		if err != nil {
			return nil, err
		}
		return SingleChoice(env, d, choice(src, "optional_replacement"), func(o decision.Option) (protocol.Semantic, bool, error) {
			return protocol.ChooseBoolean(src, "optional_replacement", o.Kind == "madness_exile"), true, nil
		})
	})
	Register("replacement/order", func(env *Env, d *decision.Decision) (Transaction, error) {
		affected := protocol.PlayerTarget(observe.Seat(d.Player))
		if d.Source != 0 {
			if r, err := env.Obs.Ref(d.Player, d.Source); err != nil {
				return nil, err
			} else if r != nil {
				affected = protocol.ObjectTarget(*r)
			}
		}
		event := "other"
		switch {
		case strings.Contains(d.Prompt, "modify damage"):
			event = "damage"
		case strings.Contains(d.Prompt, "counter"):
			event = "counters"
		case strings.Contains(d.Prompt, "enters with"):
			event = "enter_battlefield"
		}
		n := uint32(len(d.Options))
		return SingleChoice(env, d, choice(nil, ""), func(o decision.Option) (protocol.Semantic, bool, error) {
			rs, err := env.Obs.Ref(d.Player, o.Obj)
			return protocol.ChooseReplacement(affected, event, rs, uint32(o.Index), n), err == nil, err
		})
	})
	Register("choose/color", func(env *Env, d *decision.Decision) (Transaction, error) {
		src, err := ResolveSource(env, d)
		if err != nil {
			return nil, err
		}
		return SingleChoice(env, d, choice(src, "effect"), func(o decision.Option) (protocol.Semantic, bool, error) {
			c, ok := colors[strings.ToLower(strings.TrimSpace(o.Label))]
			if !ok {
				return protocol.Semantic{}, false, fmt.Errorf("%w:color/%q", ErrUnmapped, o.Label)
			}
			return protocol.ChooseColor(src, "effect", c), true, nil
		})
	})
	number := func(purp string) Builder {
		return func(env *Env, d *decision.Decision) (Transaction, error) {
			src, err := ResolveSource(env, d)
			if err != nil {
				return nil, err
			}
			lo, hi := int32(d.Options[0].Amount), int32(d.Options[0].Amount)
			for _, o := range d.Options {
				lo, hi = min(lo, int32(o.Amount)), max(hi, int32(o.Amount))
			}
			return SingleChoice(env, d, choice(src, purp), func(o decision.Option) (protocol.Semantic, bool, error) {
				return protocol.ChooseNumber(src, purp, int32(o.Amount), lo, hi), true, nil
			})
		}
	}
	Register("choose/x", number("x_value"))
	Register("choose/number", number("amount"))
	Register("choose/type", func(env *Env, d *decision.Decision) (Transaction, error) {
		src, err := ResolveSource(env, d)
		if err != nil {
			return nil, err
		}
		purpose := chooseTypePurpose(d)
		return SingleChoice(env, d, choice(src, purpose), func(o decision.Option) (protocol.Semantic, bool, error) {
			value := o.Label
			if purpose != "other" {
				value = observe.Normalize(o.Label)
			}
			return protocol.ChooseName(src, purpose, value), true, nil
		})
	})
	Register("choose/name", func(env *Env, d *decision.Decision) (Transaction, error) {
		src, err := ResolveSource(env, d)
		if err != nil {
			return nil, err
		}
		return SingleChoiceN(env, d, choice(src, "card_name"), env.NameWindow, func(o decision.Option) (protocol.Semantic, bool, error) {
			return protocol.ChooseName(src, "card_name", o.Label), env.Domain[o.Label], nil
		})
	})
}

// chooseTypeNamePurpose picks the v2 choose_name purpose for a "choose a type"
// decision from the option universe the engine offered, never from the
// resolving card. A universe is only called a family when every option's label
// belongs to that family's vocabulary, so a value is never emitted outside the
// purpose it is projected under; a universe that fits none is purpose "other"
// with the label as offered.
func chooseTypeNamePurpose(opts []decision.Option) string {
	switch {
	case labelsInTypeVocabulary(opts, protocol.Vocab["card_type"]):
		return "card_type"
	case labelsInTypeVocabulary(opts, effects.TypeChoiceLabels("Basic Land", "", "")):
		return "basic_land_type"
	case labelsInTypeVocabulary(opts, effects.TypeChoiceLabels("Land", "", "")):
		return "land_type"
	case labelsAreCreatureTypes(opts):
		return "creature_type"
	default:
		return "other"
	}
}

// labelsInTypeVocabulary reports whether opts names at least one option and
// every option's normalized label is one of vocab's normalized entries.
func labelsInTypeVocabulary(opts []decision.Option, vocab []string) bool {
	if len(opts) == 0 || len(vocab) == 0 {
		return false
	}
	set := make(map[string]bool, len(vocab))
	for _, v := range vocab {
		set[observe.Normalize(v)] = true
	}
	for _, o := range opts {
		if !set[observe.Normalize(o.Label)] {
			return false
		}
	}
	return true
}

// labelsAreCreatureTypes reports whether opts names at least one option and
// every option's label is a creature subtype word.
func labelsAreCreatureTypes(opts []decision.Option) bool {
	if len(opts) == 0 {
		return false
	}
	for _, o := range opts {
		if !effects.CreatureTypeWords(strings.TrimSpace(o.Label)) {
			return false
		}
	}
	return true
}

// optionGroupTx poses a Min>1 named-option ask as one choose_option substep
// per answer. Answer collects the native option indices and commits them as
// the decision's single intent on the last substep.
type optionGroupTx struct {
	d      *decision.Decision
	src    *protocol.ObjectRef
	chosen []int
	pose   *Pose
}

func (t *optionGroupTx) Pose() (*Pose, error) {
	p := &Pose{Seat: t.d.Player, Context: choice(t.src, "effect_option"),
		GroupStart: len(t.chosen) == 0, SubstepIndex: uint32(len(t.chosen)),
		SubstepCount: uint32(t.d.Min), Native: t.d}
	count := uint32(len(t.d.Options))
	for _, o := range t.d.Options {
		if !t.d.Repeatable && slices.Contains(t.chosen, o.Index) {
			continue
		}
		label := o.Kind
		p.Candidates = append(p.Candidates, Cand{
			Sem: protocol.ChooseOption(t.src, "effect_option", uint32(o.Index), count, &label),
			Op:  NativeOp{Op: "choose", Option: o.Index},
		})
	}
	if err := Finalize(p); err != nil {
		return nil, err
	}
	t.pose = p
	return p, nil
}

func (t *optionGroupTx) Answer(i int) ([]decision.Intent, bool, error) {
	t.chosen = append(t.chosen, t.pose.Candidates[i].Op.Option)
	if len(t.chosen) < t.d.Min {
		return nil, false, nil
	}
	return []decision.Intent{Intent(t.d, t.chosen...)}, true, nil
}
