package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// warpGrantCard lowers and builds a pool of a creature carrying the given
// static lines.
func warpGrantCard(t *testing.T, statics ...string) (*lCard, []built, error) {
	t.Helper()
	src := "Name:Hand Warp Probe\nManaCost:2 R R\nTypes:Legendary Creature Kavu Pilot\nPT:3/5\n"
	for _, s := range statics {
		src += s + "\n"
	}
	card, diags := cards.ParseBytes("probe.txt", []byte(src))
	if card == nil {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{names: []string{"Hand Warp Probe"}, byName: map[string]*cards.Card{"Hand Warp Probe": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	lw := lowerPool(p)
	_, bs, _, err := buildPool(lw)
	return lw.cards[0], bs, err
}

// The hand warp grant in the shape Tannuk, Steadfast Second prints, and its
// haste static.
const (
	warpGrantLine = "S:Mode$ Continuous | AffectedZone$ Hand | Affected$ Artifact.YouOwn,Creature.Red+YouOwn | AddKeyword$ Warp:2 R | EffectZone$ Battlefield | Description$ The probe's controller's hand artifacts and red creatures gain warp {2}{R}."
	warpHasteLine = "S:Mode$ Continuous | AddKeyword$ Haste | Affected$ Creature.Other+YouCtrl | Description$ The probe's other creatures gain haste."
)

// TestLowerAffzoneHandWarpGrant: a Continuous static granting warp to cards
// in its controller's hand lowers to kop.StatWarpGrant with the grant's warp
// cost (the kernel reads it live, kw_warpgrant.go); Tannuk, Steadfast Second
// is that grant beside a haste static. The shapes the kernel does not read
// still refuse by name.
func TestLowerAffzoneHandWarpGrant(t *testing.T) {
	lc, bs, err := warpGrantCard(t, warpHasteLine, warpGrantLine)
	if err != nil {
		t.Fatal(err)
	}
	if len(lc.Refusals) != 0 {
		t.Fatalf("refusals %v, want none", lc.Refusals)
	}
	var sts []kop.Static
	for _, a := range bs[0].def.Abilities {
		if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatWarpGrant {
			sts = append(sts, a.Static)
		}
	}
	if len(sts) != 1 {
		t.Fatalf("WarpGrant statics %+v, want one", sts)
	}
	want := kop.ManaCost{Generic: 2}
	want.Pips[kop.R] = 1
	if sts[0].GrantCost != want || sts[0].Filter == 0 || sts[0].Cond2 != 0 {
		t.Errorf("grant %+v: want cost {2}{R}, a filter and no condition", sts[0])
	}
	if bs[0].def.HasWarp {
		t.Errorf("the grant's source prints no warp, got HasWarp")
	}
	for _, c := range []struct{ name, line, want string }{
		{"opponent's hand", strings.Replace(warpGrantLine, "Artifact.YouOwn", "Artifact.OppOwn", 1), "not your hand"},
		{"another keyword", strings.Replace(warpGrantLine, "AddKeyword$ Warp:2 R", "AddKeyword$ Flash", 1), "AffectedZone \"Hand\" without MayPlay"},
		{"no cost", strings.Replace(warpGrantLine, "Warp:2 R", "Warp", 1), "AffectedZone Hand grant of \"Warp\""},
		{"effect zone", strings.Replace(warpGrantLine, "EffectZone$ Battlefield", "EffectZone$ Graveyard", 1), "EffectZone"},
		{"x cost", strings.Replace(warpGrantLine, "Warp:2 R", "Warp:X R", 1), "warp grant cost"},
		{"hybrid cost", strings.Replace(warpGrantLine, "Warp:2 R", "Warp:2 R/G", 1), "warp grant cost"},
		{"non-mana cost", strings.Replace(warpGrantLine, "Warp:2 R", "Warp:Sac<1/Creature>", 1), "warp grant cost"},
		{"land grant", strings.Replace(warpGrantLine, "Artifact.YouOwn", "Land.YouOwn", 1), "not an artifact or creature"},
		{"any card grant", strings.Replace(warpGrantLine, "Artifact.YouOwn", "Card.YouOwn", 1), "not an artifact or creature"},
		{"condition", warpGrantLine + " | Condition$ PlayerTurn", "with a condition"},
		{"pump beside", warpGrantLine + " | AddPower$ 1", "beside AddPower"},
	} {
		lc, _, _ := warpGrantCard(t, c.line)
		found := false
		for _, r := range lc.Refusals {
			found = found || strings.Contains(r.Reason, c.want)
		}
		if !found {
			t.Errorf("%s: refusals %v, want one naming %q", c.name, lc.Refusals, c.want)
		}
	}
}
