package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// kwascastLower lowers a synthetic creature carrying the given script lines
// and returns its refusals and the Pred kinds (with PredN) its filters hold.
func kwascastLower(t *testing.T, lines string) (refs []refusal, preds map[kop.Pred]int16) {
	t.Helper()
	return kwascastLowerHead(t, "Name:Cast Probe\nManaCost:2 B\nTypes:Creature Human\nPT:2/2\n", lines)
}

// kwascastLowerHead is kwascastLower over a card with the given header lines.
func kwascastLowerHead(t *testing.T, head, lines string) (refs []refusal, preds map[kop.Pred]int16) {
	t.Helper()
	src := head + lines + "\n"
	card, diags := cards.ParseBytes("cast_probe.txt", []byte(src))
	if card == nil {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{names: []string{"Cast Probe"}, byName: map[string]*cards.Card{"Cast Probe": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	lw := lowerPool(p)
	_, _, pt, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	preds = map[kop.Pred]int16{}
	for _, f := range pt.filters {
		if f.Pred != kop.PredNone {
			preds[f.Pred] = f.PredN
		}
	}
	return lw.cards[0].Refusals, preds
}

const kwascastTrig = "T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ %s | Execute$ ProbeDraw | TriggerZones$ Battlefield\nSVar:ProbeDraw:DB$ Draw | Defined$ You | NumCards$ 1"

// TestWascastLowering: wasCast, !wasCast and wasCastFromGraveyard lower in a
// battlefield-entering trigger's ValidCard$ to PredWasCastEnter with the
// origin and negation in PredN, and the Count$wasCastFromYourHandByYou.1.0 head
// of a self enters replacement lowers to origin 2; everything else refuses.
func TestWascastLowering(t *testing.T) {
	for _, c := range []struct {
		vc string
		n  int16
	}{
		{"Card.Self+wasCast", wasCastAny},
		{"Creature.Other+YouCtrl+!wasCast", wasCastNot},
		{"Creature.Other+YouCtrl+wasCastFromGraveyard", wasCastGrave},
	} {
		refs, preds := kwascastLower(t, strings.Replace(kwascastTrig, "%s", c.vc, 1))
		if len(refs) != 0 {
			t.Errorf("%s refused: %v", c.vc, refs)
			continue
		}
		if n, ok := preds[kop.PredWasCastEnter]; !ok || n != c.n {
			t.Errorf("%s lowered to preds %v, want PredWasCastEnter %d", c.vc, preds, c.n)
		}
	}

	etb := "K:etbCounter:P1P1:2:CheckSVar$ FromHand:enters with two +1/+1 counters if you cast it from your hand.\nSVar:FromHand:%s"
	refs, preds := kwascastLower(t, strings.Replace(etb, "%s", "Count$wasCastFromYourHandByYou.1.0", 1))
	if len(refs) != 0 {
		t.Errorf("etbCounter from hand refused: %v", refs)
	}
	if n, ok := preds[kop.PredWasCastEnter]; !ok || n != wasCastHand {
		t.Errorf("etbCounter from hand lowered to preds %v, want PredWasCastEnter %d", preds, wasCastHand)
	}
	for _, arms := range []string{"Count$wasCastFromYourHandByYou.2.0", "Count$wasCastFromYourHandByYou.0.1", "Count$wasCastFromYourHandByYou.1.X"} {
		refs, _ := kwascastLower(t, strings.Replace(etb, "%s", arms, 1))
		if !hasRefusal(refs, "wasCastFromYourHandByYou") {
			t.Errorf("%s: want a named refusal, got %v", arms, refs)
		}
	}
}

// TestWascastRefusals: the word reads the entering spell, so a trigger that
// leaves the battlefield, an ability or a static filter that names it refuses.
func TestWascastRefusals(t *testing.T) {
	for name, lines := range map[string]string{
		"dies trigger":   "T:Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Creature.YouCtrl+wasCast | Execute$ ProbeDraw | TriggerZones$ Battlefield\nSVar:ProbeDraw:DB$ Draw | Defined$ You | NumCards$ 1",
		"attack trigger": "T:Mode$ Attacks | ValidCard$ Creature.YouCtrl+wasCast | Execute$ ProbeDraw | TriggerZones$ Battlefield\nSVar:ProbeDraw:DB$ Draw | Defined$ You | NumCards$ 1",
		"static":         "S:Mode$ Continuous | Affected$ Creature.YouCtrl+wasCast | AddPower$ 1",
	} {
		refs, preds := kwascastLower(t, lines)
		if len(refs) == 0 {
			t.Errorf("%s lowered (preds %v), want a refusal", name, preds)
		}
	}
}
