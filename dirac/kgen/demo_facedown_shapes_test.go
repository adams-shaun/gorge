package kgen

import (
	"strings"
	"testing"
)

// TestDemoUginFacedownShapes: Ugin, the Ineffable lowers with no refusal. Its
// [+1] is a forced seen-window Dig into face-down exile (DigFaceDown), a 2/2
// Spirit token (ImprintTokens$), then an Effect that captures the token as its
// trigger and the exiled card as its remembered set, carries the view-only
// MayLook static over the remembered card, and lowers its ChangesZone return
// as the emblem's many-object return.
func TestDemoUginFacedownShapes(t *testing.T) {
	names := []string{"Ugin, the Ineffable"}
	lw := lowerNamedPool(t, names)
	if len(lw.cards) != len(names) {
		t.Fatalf("lowered %d cards, want %d", len(lw.cards), len(names))
	}
	c := lw.cards[0]
	for _, r := range c.Refusals {
		t.Errorf("%s refused: %s", c.Name, r)
	}
	_, bs, _, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	if len(bs) == 0 || bs[0].def.Name != c.Name {
		t.Fatalf("built %d cards, first %v", len(bs), bs)
	}
	for _, r := range bs[0].refusals {
		t.Errorf("%s build refusal: %v", c.Name, r)
	}
	if len(c.Emblems) != 1 {
		t.Fatalf("%s has %d emblems, want the imprint-token Effect's one", c.Name, len(c.Emblems))
	}
	em := c.Emblems[0]
	if !em.CaptureRem || !em.CaptureSet || em.Capture.Kind != rImprinted || !em.Once || !em.WhileHost || em.Trigger < 0 || em.Trigger >= len(c.Abilities) {
		t.Fatalf("%s emblem = %+v, want the captured imprinted token + remembered set, once and host-bound", c.Name, em)
	}
	trig := c.Abilities[em.Trigger]
	if trig.Trigger == nil || len(trig.Prog) != 1 {
		t.Fatalf("%s emblem trigger = %+v, want one return body", c.Name, trig)
	}
	body := trig.Prog[0]
	if body.Op != "Return" || body.Obj.Kind != rRemembered ||
		!strings.Contains(body.CS, "from=Exile") || !strings.Contains(body.CS, "DelayedMany") {
		t.Errorf("%s emblem body = %+v, want the many-object return of the remembered set from exile", c.Name, body)
	}
	found := false
	for _, in := range c.Abilities[0].Prog {
		switch {
		case in.Op == "Dig" && strings.Contains(in.CS, "ExileFaceDown"):
			found = true
		case in.Op == "CreateToken" && in.C&2048 != 0:
			found = true
		}
	}
	if !found {
		t.Error("Ugin, the Ineffable: no face-down Dig and imprinted token in the [+1] program")
	}
}
