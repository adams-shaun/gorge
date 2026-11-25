package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// exileOnLeaveRiderForms: the "exile it with a counter on it" dies-to-exile
// replacements whose counter lives on the ChangeZone body itself (the
// WithCountersType$ rider), not a SubAbility chain. ReplExileOnLeave's A
// carries the counter kind in its low byte and the count above it.
var exileOnLeaveRiderForms = []struct {
	card string
	kind kop.CounterKind // 0: no counter; the rider is a sub-ability prog (Kalitas's Token)
}{
	{card: "Dauthi Voidwalker", kind: kop.CtrVoid},
	{card: "Draugr Necromancer", kind: kop.CtrIce},
	{card: "Kalitas, Traitor of Ghet"},
	{card: "Head of the Hunt"},
	{card: "Nemata, Primeval Warden"},
}

func TestExileOnLeaveRiderForms(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	lower := func(name string) *lCard {
		card, ok := reg.Lookup(name)
		if !ok || card == nil || len(card.Faces) == 0 {
			t.Fatalf("%s not in the corpus", name)
		}
		p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
		return lowerCard(p, card.Faces[0].Name, card, false)
	}
	var lcs, toks []*lCard
	for _, f := range exileOnLeaveRiderForms {
		c := lower(f.card)
		lcs = append(lcs, c)
		seen := map[string]bool{}
		for _, ts := range c.Tokens { // the tokens the pool builds with the card (lowerPool)
			tc, ok := reg.Token(ts)
			if !ok {
				t.Fatalf("%s: token %q not in the corpus", f.card, ts)
			}
			if !seen[ts] {
				seen[ts] = true
				p := &pool{byName: map[string]*cards.Card{}, nDecks: map[string]int{}, reg: reg}
				toks = append(toks, lowerCard(p, ts, tc, true))
			}
		}
		for _, r := range c.Refusals {
			t.Errorf("%s: refused: %s", f.card, r.Reason)
		}
		if len(c.Repls) != 1 || c.Repls[0].Kind != "ExileOnLeave" {
			t.Errorf("%s: repls = %+v, want one ExileOnLeave", f.card, c.Repls)
			continue
		}
		if (c.Repls[0].Ctr == "") != (f.kind == 0) {
			t.Errorf("%s: ExileOnLeave counter kind = %q, want one only for a counter rider", f.card, c.Repls[0].Ctr)
		}
		if (len(c.Repls[0].Prog) == 0) != (f.kind != 0) {
			t.Errorf("%s: ExileOnLeave rider prog len = %d, want one only for a sub-ability rider", f.card, len(c.Repls[0].Prog))
		}
	}
	_, bs, _, err := buildPool(&lowered{cards: lcs, tokens: toks})
	if err != nil {
		t.Fatal(err)
	}
	for i, f := range exileOnLeaveRiderForms {
		if len(bs[i].refusals) != 0 {
			t.Errorf("%s: build refusals = %v", f.card, bs[i].refusals)
			continue
		}
		var repl *kop.Repl
		for j := range bs[i].def.Abilities {
			if ab := &bs[i].def.Abilities[j]; ab.Kind == kop.AbReplacement && ab.Repl.Kind == kop.ReplExileOnLeave {
				repl = &ab.Repl
			}
		}
		if repl == nil {
			t.Errorf("%s: no built ReplExileOnLeave", f.card)
			continue
		}
		if f.kind == 0 {
			if repl.A != 0 {
				t.Errorf("%s: Repl.A = %d, want 0 for a prog rider", f.card, repl.A)
			}
			var prog int
			for j := range bs[i].def.Abilities {
				if ab := &bs[i].def.Abilities[j]; ab.Kind == kop.AbReplacement && ab.Repl.Kind == kop.ReplExileOnLeave {
					prog = len(ab.Prog)
				}
			}
			if prog == 0 {
				t.Errorf("%s: built ReplExileOnLeave ability has no Prog", f.card)
			}
			continue
		}
		if got := kop.CounterKind(repl.A & 0xff); got != f.kind {
			t.Errorf("%s: Repl.A counter kind = %d, want %d", f.card, got, f.kind)
		}
		if got := repl.A >> 8; got != 1 {
			t.Errorf("%s: Repl.A counter count = %d, want 1", f.card, got)
		}
	}
}
