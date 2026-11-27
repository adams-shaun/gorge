package kgen

import (
	"testing"
)

// Granted Unleash (CR 702.86; kop.Static.Unleash): Tesak, Judith's Hellhound
// prints K:Unleash and its Dog.Other+YouCtrl static grants it to the
// controller's other Dogs (lower_unleash.go, dirac/kw_unleash.go). Before the
// grant lowered, the static refused with keyword "Unleash".
func TestUnleashGrantCarriersLower(t *testing.T) {
	c := lowerNamedCard(t, "Tesak, Judith's Hellhound")
	if len(c.Refusals) != 0 {
		t.Fatalf("Tesak refusals = %v, want none", c.Refusals)
	}
	if !c.Unleash {
		t.Error("Tesak: CardDef.Unleash not set")
	}
	var grant, haste *lStatic
	for i := range c.Statics {
		s := &c.Statics[i]
		switch {
		case s.Unleash:
			grant = s
		case s.Kind == "Continuous" && len(s.Kw) == 1 && s.Kw[0] == "Haste":
			haste = s
		}
	}
	if grant == nil {
		t.Fatal("Tesak: no Continuous static grants Unleash")
	}
	if grant.Kind != "Continuous" {
		t.Errorf("the Unleash grant is Kind %s, want Continuous", grant.Kind)
	}
	if grant.F == nil || len(grant.F.Clauses) != 1 {
		t.Fatalf("the Unleash grant filter = %+v, want one Dog clause", grant.F)
	}
	cl := grant.F.Clauses[0]
	if cl.Subtype != "Dog" || cl.Flags&fOther == 0 || cl.Ctrl != whoYou {
		t.Errorf("the Unleash grant clause = %+v, want Dog+Other+YouCtrl", cl)
	}
	if haste == nil {
		t.Error("Tesak: the counters-then-haste static did not lower")
	}
}

// The splitter keeps the other AddKeyword$ terms for the rest of the
// keyword path and folds repeated Unleash terms into one grant.
func TestSplitStaticUnleashGrant(t *testing.T) {
	if rest, u := splitStaticUnleashGrant("Unleash"); rest != "" || !u {
		t.Errorf(`splitStaticUnleashGrant("Unleash") = %q, %v; want "", true`, rest, u)
	}
	if rest, u := splitStaticUnleashGrant("Trample & Unleash & Unleash"); rest != "Trample" || !u {
		t.Errorf(`mixed list = %q, %v; want "Trample", true`, rest, u)
	}
	if rest, u := splitStaticUnleashGrant("Flying & Haste"); rest != "Flying & Haste" || u {
		t.Errorf(`plain list = %q, %v; want the list back, false`, rest, u)
	}
}
