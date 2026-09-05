package cards

import "testing"

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
