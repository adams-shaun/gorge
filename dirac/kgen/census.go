package kgen

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// svarAbilityParams are the SA parameters whose value names SVar-held
// abilities (comma-separated) that the resolving effect runs.
var svarAbilityParams = []string{"Choices", "Execute", "RepeatSubAbility", "ChosenPile", "UnchosenPile", "FallbackAbility", "TrueSubAbility", "FalseSubAbility"}

// census tabulates every (context, key) pair the pool uses: ability
// (API, param), trigger (Mode, param), static (Mode, param), replacement
// (Event, param), keyword heads and cost tokens, each with the cards using
// it.
type census struct {
	use map[string]map[string]bool // "api:Pump Param" -> card set
}

func (c *census) add(label, card string) {
	if c.use == nil {
		c.use = map[string]map[string]bool{}
	}
	if c.use[label] == nil {
		c.use[label] = map[string]bool{}
	}
	c.use[label][card] = true
}

func runCensus(p *pool, w io.Writer) {
	var c census
	for _, n := range p.names {
		card := p.byName[n]
		for _, f := range card.Faces {
			c.face(n, f)
		}
	}
	var labels []string
	for l := range c.use {
		labels = append(labels, l)
	}
	sort.Strings(labels)
	for _, l := range labels {
		var cs []string
		for k := range c.use[l] {
			cs = append(cs, k)
		}
		sort.Strings(cs)
		fmt.Fprintf(w, "%s\t%d\t%s\n", l, len(cs), strings.Join(cs, "; "))
	}
}

func (c *census) face(card string, f *cards.Face) {
	seen := map[*cards.SA]bool{}
	// seenSVar bounds the walk by SVar NAME: cards.ResolveSVar parses a
	// fresh *SA on every call, so the pointer set above cannot catch an
	// SVar that reaches itself (directly or through another SVar) and the
	// walk recursed until the stack overflowed on the full corpus.
	seenSVar := map[string]bool{}
	var walk func(sa *cards.SA, ctx string)
	resolve := func(name string) *cards.SA {
		if seenSVar[name] {
			return nil
		}
		sub := cards.ResolveSVar(f.SVars, name)
		if sub != nil {
			seenSVar[name] = true
		}
		return sub
	}
	walk = func(sa *cards.SA, ctx string) {
		for ; sa != nil; sa = sa.Sub {
			if seen[sa] {
				return
			}
			seen[sa] = true
			c.add(ctx+sa.Kind+" "+sa.API, card)
			for k, v := range sa.Params {
				c.add("api:"+sa.API+" "+k, card)
				if k == "Cost" || k == "UnlessCost" {
					for _, t := range costTokens(v) {
						c.add("cost:"+costLabel(t), card)
					}
				}
			}
			for _, k := range svarAbilityParams {
				for _, name := range strings.Split(sa.Params[k], ",") {
					name = strings.TrimSpace(name)
					if name == "" {
						continue
					}
					if cards.ResolveSVar(f.SVars, name) != nil {
						if sub := resolve(name); sub != nil {
							walk(sub, "via:"+k+" ")
						}
					} else if t, ok := cards.ParseTriggerLine(f.SVars[name]); ok && !seenSVar[name] {
						seenSVar[name] = true
						c.trigger(card, f, t, "inner ")
						if t.Effect == nil {
							if sub := resolve(t.Params["Execute"]); sub != nil {
								walk(sub, "via:Execute ")
							}
						}
					}
				}
			}
			for _, k := range []string{"Triggers", "StaticAbilities", "StaticEffect", "AddAbility"} {
				for _, name := range strings.FieldsFunc(sa.Params[k], func(r rune) bool { return r == ',' || r == ' ' }) {
					body := f.SVars[name]
					if t, ok := cards.ParseTriggerLine(body); ok && k == "Triggers" {
						if seenSVar[name] {
							continue
						}
						seenSVar[name] = true
						c.trigger(card, f, t, "inner ")
						if sub := resolve(t.Params["Execute"]); sub != nil {
							walk(sub, "via:Execute ")
						}
					} else if ss, ok := cards.ParseStaticLines(body); ok && (k == "StaticAbilities" || k == "StaticEffect") {
						for _, s := range ss {
							c.static(card, s, "inner ")
						}
					} else if sub := resolve(name); sub != nil {
						walk(sub, "via:"+k+" ")
					}
				}
			}
		}
	}
	for _, k := range f.Keywords {
		c.add("kw:"+cards.KeywordHead(k), card)
	}
	for _, a := range f.Abilities {
		walk(a, "")
	}
	for _, t := range f.Triggers {
		c.trigger(card, f, t, "")
		walk(t.Effect, "trig ")
	}
	for _, s := range f.Statics {
		c.static(card, s, "")
		for _, k := range []string{"AddAbility"} {
			if sub := cards.ResolveSVar(f.SVars, s.Params[k]); sub != nil {
				walk(sub, "via:static.AddAbility ")
			}
		}
	}
	for _, r := range f.Repls {
		c.add("repl:"+r.Event, card)
		for k := range r.Params {
			c.add("repl:"+r.Event+" "+k, card)
		}
		walk(r.With, "repl ")
	}
	for name, body := range f.SVars {
		if cards.ResolveSVar(f.SVars, name) != nil {
			continue
		}
		if i := strings.IndexByte(body, '$'); i > 0 && !strings.Contains(body, "|") {
			c.add("svar:"+body[:i], card)
		}
	}
}

func (c *census) trigger(card string, f *cards.Face, t cards.Trigger, ctx string) {
	c.add(ctx+"trig:"+t.Mode, card)
	for k := range t.Params {
		c.add("trig:"+t.Mode+" "+k, card)
	}
}

func (c *census) static(card string, s cards.Static, ctx string) {
	c.add(ctx+"stat:"+s.Mode, card)
	for k := range s.Params {
		c.add("stat:"+s.Mode+" "+k, card)
	}
}

// costTokens splits a Cost$ value on spaces outside <...>.
func costTokens(cost string) []string {
	var out []string
	depth, start := 0, -1
	for i, r := range cost {
		switch r {
		case '<':
			depth++
		case '>':
			depth--
		}
		if r == ' ' && depth == 0 {
			if start >= 0 {
				out = append(out, cost[start:i])
				start = -1
			}
			continue
		}
		if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		out = append(out, cost[start:])
	}
	return out
}

func costLabel(t string) string {
	if i := strings.IndexByte(t, '<'); i > 0 {
		return t[:i] + "<" + t[i+1:]
	}
	return t
}
