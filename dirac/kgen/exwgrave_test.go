package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestExwGraveExileFlag: a trigger object's Origin$ Graveyard exile carries
// MvFromGraveyard (the kernel skips a card another effect already moved,
// CR 400.7), and an Origin$ Battlefield one (Norin) does not.
func TestExwGraveExileFlag(t *testing.T) {
	cases := []struct {
		card string
		want int // the expected C&(MvFromGraveyard|MvRemember)
	}{
		{"Promise of Tomorrow", kop.MvFromGraveyard},
		{"Purgatory", kop.MvFromGraveyard},
		{"Bag of Holding", kop.MvFromGraveyard},
		{"Tinybones, Bauble Burglar", kop.MvFromGraveyard},
		{"Flameskull", kop.MvFromGraveyard | kop.MvRemember},
		{"Norin, Swift Survivalist", kop.MvRemember},
	}
	names := make([]string, len(cases))
	for i, c := range cases {
		names[i] = c.card
	}
	list := filepath.Join(t.TempDir(), "cards.txt")
	if err := os.WriteFile(list, []byte(strings.Join(names, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, ratchetCards(t))
	if err != nil {
		t.Fatal(err)
	}
	all, bs, _, err := buildPool(lowerPool(p))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		i := -1
		for j := range all {
			if all[j].Name == c.card {
				i = j
			}
		}
		if i < 0 {
			t.Errorf("%s: not lowered", c.card)
			continue
		}
		if r := lowerRefusals(all[i]); len(r) > 0 {
			t.Errorf("%s: refused: %v", c.card, r)
			continue
		}
		found := false
		for _, a := range bs[i].def.Abilities {
			for _, in := range a.Prog {
				if (in.Op != kop.OpExile && in.Op != kop.OpExileEx) || in.Obj != kop.RefTriggerObj {
					continue
				}
				found = true
				if got := int(in.C) & (kop.MvFromGraveyard | kop.MvRemember); got != c.want {
					t.Errorf("%s: %s %+v, C&(MvFromGraveyard|MvRemember) = %#x, want %#x", c.card, in.Op, in, got, c.want)
				}
			}
		}
		if !found {
			t.Errorf("%s: no trigger-object exile", c.card)
		}
	}
}
