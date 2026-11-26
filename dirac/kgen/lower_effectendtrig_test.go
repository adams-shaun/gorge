package kgen

import (
	"strings"
	"testing"
)

// TestEffectEndingTriggerStatics: Orcish Farmer and Soul Sculptor each lower
// to one override on the target with the ending carried on the Override row
// (UntilCtrlUntap / UntilCast); a variant body with an extra key on the
// trigger, the static or the exile still refuses by name.
func TestEffectEndingTriggerStatics(t *testing.T) {
	farmer := kfhBuild(t, []string{"Orcish Farmer"}, nil)
	farmer.accepted(t, "Orcish Farmer")
	if n := len(farmer.pt.overrides); n != 1 {
		t.Fatalf("Orcish Farmer builds %d override rows, want 1", n)
	}
	if o := farmer.pt.overrides[0]; !o.UntilCtrlUntap || o.UntilCast != 0 || !o.SetLandSub || !o.LoseAbilities {
		t.Errorf("Orcish Farmer row = %+v, want UntilCtrlUntap + the CR 305.7 land-type setter", o)
	}
	sculptor := kfhBuild(t, []string{"Soul Sculptor"}, nil)
	sculptor.accepted(t, "Soul Sculptor")
	if n := len(sculptor.pt.overrides); n != 1 {
		t.Fatalf("Soul Sculptor builds %d override rows, want 1", n)
	}
	if o := sculptor.pt.overrides[0]; o.UntilCtrlUntap || o.UntilCast == 0 || !o.LoseAbilities || !o.HasTypes {
		t.Errorf("Soul Sculptor row = %+v, want UntilCast + a type set with LoseAbilities", o)
	}
	for _, kp := range []kfhPool{farmer, sculptor} {
		if len(kp.pt.emblems) != 0 {
			t.Errorf("an ending-trigger Effect made %d emblems, want none", len(kp.pt.emblems))
		}
	}

	for _, tc := range []struct {
		card, svar, from, to, want string
	}{
		{"Orcish Farmer", "ExileEff", "| Static$ True", "| Static$ True | OneOff$ True", "trigger unread"},
		{"Orcish Farmer", "ExileEff", "Player.controlsCard.IsRemembered", "Player", "Phase trigger"},
		{"Orcish Farmer", "ExileEff", "Phase$ Untap", "Phase$ Upkeep", "Phase trigger"},
		{"Orcish Farmer", "Exile", "Defined$ Self", "Defined$ Self | Mandatory$ True", "does not only exile"},
		{"Orcish Farmer", "Animate", "RemoveLandTypes$ True", "RemoveLandTypes$ True | AddKeyword$ Flying", "static unread"},
		{"Soul Sculptor", "TrigCastSpell", "ValidActivatingPlayer$ You", "ValidActivatingPlayer$ Player", "SpellCast trigger"},
		{"Soul Sculptor", "TrigCastSpell", "Static$ True", "Static$ True | Secondary$ True | IsPresent$ Card", "trigger unread"},
		{"Soul Sculptor", "RemoveEffect", "Destination$ Exile", "Destination$ Graveyard", "does not only exile"},
		{"Soul Sculptor", "Enchant", "RemoveCardTypes$ True", "RemoveCardTypes$ True | SetPower$ 1", "StaticEffect"},
	} {
		t.Run(tc.card+"/"+tc.svar+"/"+tc.want, func(t *testing.T) {
			kp := kfhBuild(t, []string{tc.card}, func(p *pool) {
				c := kfhClone(p.byName[tc.card])
				f := c.Faces[0]
				if !strings.Contains(f.SVars[tc.svar], tc.from) {
					t.Fatalf("SVar %s %q lacks %q", tc.svar, f.SVars[tc.svar], tc.from)
				}
				f.SVars[tc.svar] = strings.Replace(f.SVars[tc.svar], tc.from, tc.to, 1)
				p.byName[tc.card] = c
			})
			kp.refusedWith(t, tc.card, tc.want)
		})
	}
}
