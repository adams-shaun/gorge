package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestExpandedKeywordsVizierCloneLower pins the as-enters clone's Embalm$
// gate (Vizier of Many Faces): the body lowers with no refusal, its BecomeCopy
// op carries the Embalm flag beside an exception override that holds exactly
// the gated riders (Zombie, white, no mana cost), and the gorge shape of the
// gate (effects clone_params: an Embalm$ other than True fails closed, a
// RemoveCost$ outside an Embalm$ True body is withheld) refuses by name.
func TestExpandedKeywordsVizierCloneLower(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup("Vizier of Many Faces")
	if !ok || card == nil {
		t.Fatal("Vizier of Many Faces not in the corpus")
	}
	lower := func(mut func(p map[string]string)) *lCard {
		face := *card.Faces[0]
		face.Repls = append(face.Repls[:0:0], face.Repls...)
		hit := false
		for i := range face.Repls {
			if w := face.Repls[i].With; w != nil && w.API == "Clone" {
				sa := *w
				sa.Params = map[string]string{}
				for k, v := range w.Params {
					sa.Params[k] = v
				}
				mut(sa.Params)
				face.Repls[i].With = &sa
				hit = true
			}
		}
		if !hit {
			t.Fatal("Vizier of Many Faces has no Clone replacement body")
		}
		c2 := *card
		c2.Faces = []*cards.Face{&face}
		p := &pool{byName: map[string]*cards.Card{face.Name: &c2}, nDecks: map[string]int{}, reg: reg}
		return lowerCard(p, face.Name, &c2, false)
	}

	lc := lower(func(map[string]string) {})
	if len(lc.Refusals) != 0 {
		t.Fatalf("Vizier of Many Faces refusals = %v, want none", lc.Refusals)
	}
	var op *lInstr
	for ri := range lc.Repls {
		for i := range lc.Repls[ri].Prog {
			if lc.Repls[ri].Prog[i].Op == "BecomeCopy" {
				op = &lc.Repls[ri].Prog[i]
			}
		}
	}
	if op == nil {
		t.Fatal("no BecomeCopy instruction lowered")
	}
	if !isConstN(op.B, int(kop.BecomeCopyEmbalm)) {
		t.Errorf("BecomeCopy B = %v, want the BecomeCopyEmbalm flag", op.B)
	}
	x := op.C >> 1
	if x == 0 {
		t.Fatal("BecomeCopy carries no exception override")
	}
	exc := lc.overrides[x-1]
	if !exc.NoCost || !exc.HasSetColors || exc.SetColors != "W" || len(exc.AddSubtypes) != 1 || exc.AddSubtypes[0] != "Zombie" {
		t.Errorf("exception override = %+v, want no cost, white, Zombie", *exc)
	}

	refusesBy := func(name string, mut func(p map[string]string), want string) {
		t.Helper()
		lc := lower(mut)
		for _, r := range lc.Refusals {
			if strings.Contains(r.Reason, want) {
				return
			}
		}
		t.Errorf("%s: refusals = %v, want one naming %q", name, lc.Refusals, want)
	}
	refusesBy("RemoveCost beside Embalm False", func(p map[string]string) { p["Embalm"] = "False" }, "Embalm")
	refusesBy("RemoveCost without Embalm", func(p map[string]string) { delete(p, "Embalm") }, "RemoveCost")
	refusesBy("Embalm True beside an ungated exception", func(p map[string]string) { p["AddKeywords"] = "Flying" }, "Embalm")
}
