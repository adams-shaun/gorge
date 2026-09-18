package rules

import (
	"sort"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/effects"
)

// compiledText contains only immutable interpretations of configured card
// text. It never participates in events or mutable game state and is safe to
// share with engine clones.
type compiledText struct {
	predicates *effects.PredicatePrograms
	costs      map[string]Cost
}

func newCompiledText(cfg Config) *compiledText {
	predicateTexts := make(map[string]struct{})
	costTexts := make(map[string]struct{})
	seen := make(map[*cards.SA]struct{})
	addParams := func(params map[string]string) {}
	var addAbility func(*cards.SA)
	addAbility = func(sa *cards.SA) {
		if sa == nil {
			return
		}
		if _, ok := seen[sa]; ok {
			return
		}
		seen[sa] = struct{}{}
		addParams(sa.Params)
		addAbility(sa.Sub)
	}
	addParams = func(params map[string]string) {
		keys := make([]string, 0, len(params))
		for key := range params {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			value := params[key]
			if value != "" {
				predicateTexts[value] = struct{}{}
			}
			if (key == "Cost" || key == "UnlessCost") && value != "" {
				costTexts[value] = struct{}{}
			}
		}
	}
	addCard := func(c *cards.Card) {
		if c == nil {
			return
		}
		for _, f := range c.Faces {
			if f == nil {
				continue
			}
			if f.ManaCost != "" {
				costTexts[f.ManaCost] = struct{}{}
			}
			for _, sa := range f.Abilities {
				addAbility(sa)
			}
			for _, tr := range f.Triggers {
				addParams(tr.Params)
				addAbility(tr.Effect)
			}
			for _, st := range f.Statics {
				addParams(st.Params)
			}
			for _, rp := range f.Repls {
				addParams(rp.Params)
				addAbility(rp.With)
			}
		}
	}
	for _, deck := range cfg.Decks {
		for _, c := range deck {
			addCard(c)
		}
	}
	tokenKeys := make([]string, 0, len(cfg.Tokens))
	for key := range cfg.Tokens {
		tokenKeys = append(tokenKeys, key)
	}
	sort.Strings(tokenKeys)
	for _, key := range tokenKeys {
		addCard(cfg.Tokens[key])
	}
	preds := make([]string, 0, len(predicateTexts))
	for text := range predicateTexts {
		preds = append(preds, text)
	}
	sort.Strings(preds)
	costTextList := make([]string, 0, len(costTexts))
	for text := range costTexts {
		costTextList = append(costTextList, text)
	}
	sort.Strings(costTextList)
	costs := make(map[string]Cost, len(costTextList))
	for _, text := range costTextList {
		costs[text] = freezeCost(ParseCost(text))
	}
	return &compiledText{predicates: effects.CompilePredicatePrograms(preds), costs: costs}
}

func freezeCost(c Cost) Cost {
	c.Hybrid = c.Hybrid[:len(c.Hybrid):len(c.Hybrid)]
	c.Phyrexian = c.Phyrexian[:len(c.Phyrexian):len(c.Phyrexian)]
	c.Twobrid = c.Twobrid[:len(c.Twobrid):len(c.Twobrid)]
	c.HybridPhyrexian = c.HybridPhyrexian[:len(c.HybridPhyrexian):len(c.HybridPhyrexian)]
	c.Sac = c.Sac[:len(c.Sac):len(c.Sac)]
	c.Discard = c.Discard[:len(c.Discard):len(c.Discard)]
	c.SubCounter = c.SubCounter[:len(c.SubCounter):len(c.SubCounter)]
	c.AddCounter = c.AddCounter[:len(c.AddCounter):len(c.AddCounter)]
	c.Exile = c.Exile[:len(c.Exile):len(c.Exile)]
	c.Reveal = c.Reveal[:len(c.Reveal):len(c.Reveal)]
	c.Behold = c.Behold[:len(c.Behold):len(c.Behold)]
	c.TapPermanent = c.TapPermanent[:len(c.TapPermanent):len(c.TapPermanent)]
	c.Blight = c.Blight[:len(c.Blight):len(c.Blight)]
	c.Draw = c.Draw[:len(c.Draw):len(c.Draw)]
	c.Energy = c.Energy[:len(c.Energy):len(c.Energy)]
	c.LifeX = c.LifeX[:len(c.LifeX):len(c.LifeX)]
	c.DamageYou = c.DamageYou[:len(c.DamageYou):len(c.DamageYou)]
	c.Return = c.Return[:len(c.Return):len(c.Return)]
	c.Unknown = c.Unknown[:len(c.Unknown):len(c.Unknown)]
	return c
}

func (e *Engine) parseCost(raw string) Cost {
	if e != nil && e.compiledText != nil {
		if c, ok := e.compiledText.costs[raw]; ok {
			return c
		}
	}
	return ParseCost(raw)
}
