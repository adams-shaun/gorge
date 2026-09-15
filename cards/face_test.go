package cards

import "testing"

func TestManaValuePricesTwobridAtGenericFace(t *testing.T) {
	for _, mc := range []string{"2W", "2/W", "{2/W}"} {
		f := &Face{ManaCost: mc}
		if got := f.ManaValue(); got != 2 {
			t.Errorf("ManaValue(%q) = %d, want 2", mc, got)
		}
	}
}

func TestKeywordParam(t *testing.T) {
	c, _ := ParseBytes("k.txt", []byte("Name:K\nTypes:Creature\nK:Kicker:B\nK:Flash\nK:Flashback:Sac<1/Creature>\nK:Protection from blue\nOracle:x\n"))
	f := c.Faces[0]
	for head, want := range map[string]string{"Kicker": "B", "Flashback": "Sac<1/Creature>", "Flash": "", "Protection from blue": ""} {
		if got, ok := f.KeywordParam(head); !ok || got != want {
			t.Errorf("%s: %q %v", head, got, ok)
		}
	}
	if _, ok := f.KeywordParam("Delve"); ok {
		t.Error("absent keyword reported present")
	}
}
