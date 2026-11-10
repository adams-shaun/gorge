package kgen

import (
	"strings"
	"testing"
)

// Inline stand-in for the one admitted shape: a targeted graveyard-to-
// battlefield move that remembers what it moved, then a life loss sized by
// the remembered mana values. The holes are what each variant changes.
const remLKIMVProbe = `Name:Mv Probe
ManaCost:B
Types:Sorcery
A:SP$ ChangeZone | ValidTgts$ Creature | Origin$ Graveyard | Destination$ Battlefield | ChangeNum$ 1 | GainControl$ True | RememberLKI$ True PARAMS| SubAbility$ DBPay
SVar:DBPay:DB$ LoseLife | LifeAmount$ Cost | Defined$ You | SubAbility$ DBEnd
SVar:DBEnd:DB$ Cleanup | ClearRemembered$ True
SVar:Cost:RememberedLKI$CardManaCost
Oracle:probe
`

// TestRemLKIManaValue: RememberedLKI$CardManaCost lowers over the set one
// targeted Return RememberLKI$ wrote (Reanimate, Grave Researcher's prepared
// face) and refuses every other shape and every other RememberedLKI$ head by
// name.
func TestRemLKIManaValue(t *testing.T) {
	lc := lowerProbe(t, remLKIMVProbe, "Mv Probe", "")
	if r := probeRefusals(lc); r != "" {
		t.Fatalf("the probe refused: %s", r)
	}
	read := false
	for _, in := range lc.Abilities[0].Prog {
		read = read || strings.Contains(in.String(), "MVOf") && strings.Contains(in.String(), "Remembered")
	}
	if !read {
		t.Fatalf("no remembered mana value read in %v", lc.Abilities[0].Prog)
	}

	for _, tc := range []struct{ name, script, params, want string }{
		{"no writer", strings.Replace(remLKIMVProbe, " | RememberLKI$ True PARAMS", "", 1), "", "(0 earlier writers)"},
		{"clear before the read", strings.Replace(strings.Replace(remLKIMVProbe, "SubAbility$ DBPay", "SubAbility$ DBEnd", 1),
			"ClearRemembered$ True", "ClearRemembered$ True | SubAbility$ DBPay", 1), "", "ClearRemembered$ before the read"},
		{"another head", strings.Replace(remLKIMVProbe, "RememberedLKI$CardManaCost", "RememberedLKI$CardCMC", 1), "", "RememberedLKI$CardCMC"},
		{"loyalty", strings.Replace(remLKIMVProbe, "RememberedLKI$CardManaCost", "RememberedLKI$CardLoyalty", 1), "", "RememberedLKI$CardLoyalty"},
	} {
		got := probeRefusals(lowerProbe(t, tc.script, "Mv Probe", tc.params))
		if !strings.Contains(got, tc.want) {
			t.Errorf("%s: refusals %q, want %q", tc.name, got, tc.want)
		}
	}

	// a second writer on the card: another ability remembering
	second := strings.Replace(remLKIMVProbe, "Oracle:probe", "A:AB$ ChangeZone | Cost$ T | Origin$ Graveyard | Destination$ Exile | ValidTgts$ Card | RememberChanged$ True\nOracle:probe", 1)
	if got := probeRefusals(lowerProbe(t, second, "Mv Probe", "")); !strings.Contains(got, "remember writers on the card") {
		t.Errorf("second writer: refusals %q, want the writer count named", got)
	}

	// a Defined$ Self move is not a targeted Return
	self := strings.Replace(remLKIMVProbe, "ValidTgts$ Creature | Origin$", "Defined$ Self | Origin$", 1)
	if got := probeRefusals(lowerProbe(t, self, "Mv Probe", "")); !strings.Contains(got, remLKIMVShape) {
		t.Errorf("self move: refusals %q, want %q", got, remLKIMVShape)
	}

	// the corpus: Reanimate and the card whose prepared face is a copy of it
	lw := lowerNamedPool(t, []string{"Grave Researcher", "Reanimate"})
	if _, _, _, err := buildPool(lw); err != nil {
		t.Fatal(err)
	}
	for i, c := range lw.cards {
		if r := lowerRefusals(c); len(r) != 0 {
			t.Errorf("card %d %s: refused %v", i, c.Name, r)
		}
	}
	if len(lw.cards) < 2 {
		t.Errorf("pool lowered %d cards, want Grave Researcher and Reanimate", len(lw.cards))
	}
}
