package gorgebe

import (
	"fmt"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
	"github.com/adams-shaun/gorge/engine/cards"
)

// Origin says who defines a served card.
type Origin uint8

const (
	OriginCardc Origin = iota + 1
	OriginForge
)

// Entry is one compiled card's serving decision.
type Entry struct {
	Name   string
	Origin Origin
	// Reason is "" for OriginCardc; else "fallback:<class>", "refused:<Key>",
	// "unprintable:<What>" or "parse:<first diag>".
	Reason string
	Forge  *cards.Card // the Forge card of this name
	Card   *cards.Card // what games seat: the cardc card, or Forge when Origin is OriginForge
	Script []byte      // the printed text, for OriginCardc
}

// Registry serves each compiled card from cardc or Forge.
type Registry struct {
	forge   *cards.Registry
	entries []*Entry
	byForge map[*cards.Card]*Entry // read by key only, never ranged
}

// Build decides each compiled card's origin; a repeated name keeps its first.
// The caller seats forge.Tokens and forge.Universe().
func Build(forge *cards.Registry, compiled []*ir.Card, fb map[string]Fallback) (*Registry, error) {
	r := &Registry{forge: forge, byForge: map[*cards.Card]*Entry{}}
	seen := map[string]bool{}
	var served []*cards.Card
	for _, c := range compiled {
		if seen[c.Name] {
			continue
		}
		seen[c.Name] = true
		fc, ok := forge.Lookup(c.Name)
		if !ok {
			return nil, fmt.Errorf("gorgebe: no Forge card %q", c.Name)
		}
		e := &Entry{Name: c.Name, Origin: OriginForge, Forge: fc, Card: fc}
		r.entries = append(r.entries, e)
		r.byForge[fc] = e
		if f, ok := fb[c.Name]; ok {
			e.Reason = "fallback:" + f.Class
			continue
		}
		if key := firstRefusal(c); key != "" {
			e.Reason = "refused:" + key
			continue
		}
		oracle := make([]string, len(fc.Faces))
		for i, f := range fc.Faces {
			oracle[i] = f.Oracle
		}
		text, err := PrintWith(c, oracle, forge.Tokens)
		if err == nil {
			text = followForge(text, fc)
		}
		if err != nil {
			if u, ok := err.(*Unprintable); ok {
				e.Reason = "unprintable:" + u.What
				continue
			}
			return nil, err
		}
		pc, diags := ParseScript("cardc/"+c.Name+".txt", text)
		if len(diags) > 0 {
			e.Reason = "parse:" + diags[0].Msg
			continue
		}
		e.Origin, e.Card, e.Script = OriginCardc, pc, text
		served = append(served, pc)
	}
	if err := Bind(served); err != nil {
		return nil, err
	}
	return r, nil
}

func firstRefusal(c *ir.Card) string {
	for _, f := range c.Faces {
		if len(f.Refusals) > 0 {
			return f.Refusals[0].Key
		}
	}
	return ""
}

// Entries lists the entries in compiled order.
func (r *Registry) Entries() []*Entry { return r.entries }

// Deck returns a new slice with every served card swapped, keeping order and
// duplicates; cards without an entry stay as they are.
func (r *Registry) Deck(d []*cards.Card) []*cards.Card {
	out := make([]*cards.Card, len(d))
	for i, c := range d {
		out[i] = c
		if e, ok := r.byForge[c]; ok {
			out[i] = e.Card
		}
	}
	return out
}

// Served counts the entries with OriginCardc.
func (r *Registry) Served() int {
	n := 0
	for _, e := range r.entries {
		if e.Origin == OriginCardc {
			n++
		}
	}
	return n
}
