package kgen

import (
	"strings"
	"testing"
)

// TestInstantLoyaltyFlashLowers: the CastWithFlash statics whose ValidSA$ is
// Activated.Loyalty (gorge loyaltyAtInstantSpeed) lower to the loyalty-timing
// grant (lStatic Kind LoyaltyFlash, kop.StatLoyaltyFlash), a printed static or
// an Effect-delivered emblem static, never a spell-flash grant. Teferi, Master
// of Time keeps only its AddTurn refusal, and an Activated.Loyalty grant
// without Caster$ You, an Activated.Equip grant, or a ValidCard$ word the
// lowerer cannot read still refuses.
func TestInstantLoyaltyFlashLowers(t *testing.T) {
	for _, tc := range []struct{ card, refused string }{
		{"Teferi's Talent", ""},           // the granted -12's emblem (Planeswalker.YouCtrl)
		{"Teferi, Temporal Archmage", ""}, // the -10's emblem (Planeswalker.YouCtrl)
		{"Jace's Machinations", ""},       // the EOT Effect over Jace planeswalkers you control
		{"The Wandering Emperor", ""},     // the printed Card.Self+ThisTurnEntered grant
		{"Teferi, Master of Time", "AddTurn 2"},
	} {
		c := lowerCorpusCard(t, tc.card)
		var reasons []string
		for _, rf := range c.Refusals {
			reasons = append(reasons, rf.Reason)
		}
		got := strings.Join(reasons, "; ")
		if strings.Contains(got, "CastWithFlash") {
			t.Errorf("%s: still refused on the grant: %s", tc.card, got)
		}
		if tc.refused == "" && got != "" {
			t.Errorf("%s: refused: %s", tc.card, got)
		}
		if tc.refused != "" {
			if !strings.Contains(got, tc.refused) {
				t.Errorf("%s: want refusal %q, got %q", tc.card, tc.refused, got)
			}
			continue
		}
		if n := loyaltyFlashStatics(c); n != 1 {
			t.Errorf("%s: %d LoyaltyFlash statics, want 1", tc.card, n)
		}
	}
	const pw = "ManaCost:2 U U\nTypes:Legendary Planeswalker\nLoyalty:3\n" +
		"A:AB$ Draw | Cost$ AddCounter<1/LOYALTY> | Planeswalker$ True | NumCards$ 1 | SpellDescription$ Probe: draw one.\n"
	for _, tc := range []struct{ name, line, want string }{
		{"Probe Any Caster", "S:Mode$ CastWithFlash | ValidCard$ Card.Self | ValidSA$ Activated.Loyalty | Caster$ Player | Description$ x.\n", "CastWithFlash"},
		{"Probe No Caster", "S:Mode$ CastWithFlash | ValidCard$ Card.Self | ValidSA$ Activated.Loyalty | Description$ x.\n", "CastWithFlash"},
		{"Probe Equip", "S:Mode$ CastWithFlash | ValidCard$ Card.Self | ValidSA$ Activated.Equip | Caster$ You | Description$ x.\n", "CastWithFlash Activated.Equip with ValidCard$"},
		{"Probe Unread Word", "S:Mode$ CastWithFlash | ValidCard$ Card.Self+Zzyzxword | ValidSA$ Activated.Loyalty | Caster$ You | Description$ x.\n", "Zzyzxword"},
	} {
		got := strings.Join(lowerScript(t, tc.name, "Name:"+tc.name+"\n"+pw+tc.line), "; ")
		if !strings.Contains(got, tc.want) {
			t.Errorf("%s: want a refusal containing %q, got %q", tc.name, tc.want, got)
		}
	}
	// the control: the same planeswalker with Caster$ You lowers the grant
	ok := lowerScriptCard(t, "Probe Self", "Name:Probe Self\n"+pw+
		"S:Mode$ CastWithFlash | ValidCard$ Card.Self | ValidSA$ Activated.Loyalty | Caster$ You | Description$ x.\n")
	if len(ok.Refusals) != 0 || loyaltyFlashStatics(ok) != 1 {
		t.Errorf("Probe Self: refusals %v, %d LoyaltyFlash statics", ok.Refusals, loyaltyFlashStatics(ok))
	}
}

// loyaltyFlashStatics counts c's LoyaltyFlash statics: printed, granted and
// carried by its emblems.
func loyaltyFlashStatics(c *lCard) int {
	n := 0
	count := func(sts []lStatic) {
		for _, s := range sts {
			if s.Kind == "LoyaltyFlash" {
				n++
			}
		}
	}
	count(c.Statics)
	count(c.GrantedStatics)
	for _, e := range c.Emblems {
		count(e.Statics)
	}
	return n
}
