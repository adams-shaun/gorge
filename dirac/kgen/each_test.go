package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// An "EACH A & B" pick lowers to an Or chain with its head marked (one clause
// per listed type), for a library search (Lotuslight Dancers, Aang's Journey's
// kicked arm) and an optional hand move (Michelangelo, Improviser). The shapes
// the kernel does not read are refused by name.
func TestEachPickLowers(t *testing.T) {
	kp := kfhBuild(t, []string{"Lotuslight Dancers", "Michelangelo, Improviser", "Aang's Journey"}, nil)
	for _, n := range []string{"Lotuslight Dancers", "Michelangelo, Improviser", "Aang's Journey"} {
		kp.accepted(t, n)
	}
	var heads, longest int
	for _, f := range kp.pt.filters {
		if !f.Each {
			continue
		}
		heads++
		n := 0
		for cur := f; ; n++ {
			if cur.Or == 0 {
				n++
				break
			}
			cur = kp.pt.filters[cur.Or]
		}
		longest = max(longest, n)
	}
	if heads != 3 || longest != 3 {
		t.Fatalf("%d EACH heads, longest chain %d; want 3 heads, a three-type chain", heads, longest)
	}
}

func TestEachPickRefusals(t *testing.T) {
	const lot, mich = "Lotuslight Dancers", "Michelangelo, Improviser"
	// edit rewrites one param of the card's trigger body (a copy: the corpus
	// card is shared)
	edit := func(name, key, val string) kfhPool {
		return kfhBuild(t, []string{name}, func(p *pool) {
			c := kfhClone(p.byName[name])
			f := c.Faces[0]
			f.Triggers = append([]cards.Trigger(nil), f.Triggers...)
			e := *f.Triggers[0].Effect
			e.Params = map[string]string{}
			for k, v := range f.Triggers[0].Effect.Params {
				e.Params[k] = v
			}
			if val == "" {
				delete(e.Params, key)
			} else {
				e.Params[key] = val
			}
			f.Triggers[0].Effect = &e
			p.byName[name] = c
		})
	}
	edit(lot, "ChangeNum", "2").refusedWith(t, lot, "EACH pick of")
	edit(lot, "ChangeType", "EACH Card.Black, Card.Red & Card.Blue").refusedWith(t, lot, "is not one object clause")
	edit(lot, "DifferentNames", "True").refusedWith(t, lot, "EACH pick with DifferentNames$")
	edit(mich, "Optional", "").refusedWith(t, mich, "required hand move of one card per type")
}
