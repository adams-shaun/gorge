package kgen

import (
	"strings"
	"testing"
)

// TestNoAbilitiesWord: the NoAbilities filter word reads the printed face
// (gorge FaceHasNoAbilities) and lowers off the battlefield only, where the
// card's CardDef.NoAbilities answers it; a permanent's granted or lost
// abilities would change the answer, so on the battlefield it refuses.
func TestNoAbilitiesWord(t *testing.T) {
	all := lowerNamed(t, []string{"Grizzly Bears", "Llanowar Elves", "Serra Angel", "Fang-Druid Summoner", "Rise from the Wreck"})
	want := map[string]bool{"Grizzly Bears": true, "Llanowar Elves": false, "Serra Angel": false, "Fang-Druid Summoner": false, "Rise from the Wreck": false}
	for _, c := range all {
		if w, ok := want[c.Name]; ok && c.NoAbilities != w {
			t.Errorf("%s: NoAbilities = %v, want %v", c.Name, c.NoAbilities, w)
		}
		if (c.Name == "Fang-Druid Summoner" || c.Name == "Rise from the Wreck") && len(c.Refusals) != 0 {
			t.Errorf("%s: refused: %s", c.Name, keclReason(c.Refusals))
		}
	}
	all, bs := keclBuild(t, []string{"Muraganda Petroglyphs"})
	if len(bs) != 1 || !strings.Contains(keclReason(bs[0].refusals), "NoAbilities on the battlefield") {
		t.Errorf("%s: refusals %v, want one naming the battlefield", all[0].Name, bs[0].refusals)
	}
}
