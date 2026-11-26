package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestMorphPickLowers: a Morph-family keyword whose turn-up cost is a single
// pick part lowers through CardDef.MorphPick -- the kernel's turn-up window
// poses gorge's pick ask for it before the permanent turns face up
// (dirac/face_morphpick.go; gorge rules/morph_turnup.go's turnUpSacAsk,
// turnUpDiscardAsk, turnUpReturnAsk). MorphCost is then {0}:
//
//   - Gathan Raiders: K:Morph:Discard<1/Card> (a discard of any one hand
//     card), a hand-scoped filter;
//   - Fathom Seer: K:Morph:Return<2/Island> (return two Islands you control),
//     N 2;
//   - Gift of Doom: a Sac pick of another creature (the K:Morph token names
//     Creature.Other), the face-down permanent itself never a candidate, and
//     its turn-up attach replacement (kop.ReplTurnUpAttach,
//     dirac/face_turnupattach.go): the card lowers with no refusal at all.
//
// Anything the turn-up window cannot pose stays refused: a pick part beside
// mana or another part, an announced count (Sac<X/..>), a whole-hand discard,
// a self return.
func TestMorphPickLowers(t *testing.T) {
	all, bs := costProgCastBuild(t, []string{"Gathan Raiders", "Fathom Seer", "Gift of Doom"})
	byName := map[string]int{}
	for i, c := range all {
		byName[c.Name] = i
	}
	type want struct {
		kind kop.CostFlags
		n    int
	}
	wants := map[string]want{
		"Gathan Raiders": {kind: kop.CostDiscard, n: 1},
		"Fathom Seer":    {kind: kop.CostReturn, n: 2},
		"Gift of Doom":   {kind: kop.CostSac, n: 1},
	}
	for name, w := range wants {
		i, ok := byName[name]
		if !ok {
			t.Fatalf("%s: not lowered", name)
		}
		reasons := costProgCastRouteReason(all[i])
		if reasons != "" {
			t.Errorf("%s: refused: %s", name, reasons) // fully lowered: no refusal of any shape
			continue
		}
		b := bs[i]
		if b.def.MorphFamily == 0 || b.def.MorphCost != (kop.ManaCost{}) {
			t.Errorf("%s: MorphFamily %d MorphCost %+v, want a {0} keyword turn-up", name, b.def.MorphFamily, b.def.MorphCost)
		}
		pick := b.def.MorphPick
		if pick.Flags != w.kind || int(pick.N) != w.n || pick.ExileBf {
			t.Errorf("%s: MorphPick %+v, want one %d pick of %d", name, pick, w.kind, w.n)
		}
		if w.kind == kop.CostDiscard && pick.Filter != 0 {
			t.Errorf("%s: discard of any card carries no filter, got %d", name, pick.Filter)
		}
		if w.kind != kop.CostDiscard && pick.Filter == 0 {
			t.Errorf("%s: pick filter missing", name)
		}
		attach := 0
		for _, ab := range b.def.Abilities {
			if ab.Kind == kop.AbReplacement && ab.Repl.Kind == kop.ReplTurnUpAttach {
				attach++
			}
		}
		if want := map[bool]int{true: 1}[name == "Gift of Doom"]; attach != want {
			t.Errorf("%s: %d ReplTurnUpAttach replacements, want %d (Gift of Doom's \"may attach it\" body)", name, attach, want)
		}
	}
}
