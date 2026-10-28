package mapping

import (
	"fmt"
	"strings"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/server/spellbench/observe"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
)

var colors = map[string]string{"white": "white", "blue": "blue", "black": "black", "red": "red", "green": "green"}

var boolPurpose = map[string]string{"search_confirm": "may_ability", "copy_optional": "may_ability",
	"repeat_optional": "may_ability", "search_mayshuffle": "other"}

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
		purpose := chooseTypeNamePurpose(d.Options)
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
