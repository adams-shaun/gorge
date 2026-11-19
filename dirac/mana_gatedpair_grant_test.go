package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// CheckGatedPairGrantAfford: Goliath Spider's {6}{G}{G} is payable from four
// Forests, Whisperer of the Wilds' ferocious {T}: Add {G}{G} (Regisaur Alpha's
// power 4 opens the gate) and two creatures' granted any-colour {T} ability
// under Cryptolith Rite: the grant search's line taps the Whisperer for its
// {G}{G} (gorge's "choose a mana ability" ask lists the pair's larger
// alternative after the printed {G}, CR 605.1a) and the two creatures for
// their granted mana. Without the pair's alternative the search spent its node
// budget on the Whisperer's {G} and its own grant. One creature fewer leaves
// seven mana: not offered.
func CheckGatedPairGrantAfford(t *testing.T, tbl *kop.Table, id map[string]CardID) {
	t.Helper()
	base := []string{"Forest", "Forest", "Forest", "Forest", "Whisperer of the Wilds", "Regisaur Alpha", "Cryptolith Rite"}
	for _, cfg := range []Config{{NoElide: true}, {GorgeLowering: true, NoElide: true}, {GorgeAutopaySurface: true, NoElide: true}} {
		for _, c := range []struct {
			extra string
			want  bool
		}{
			{"Duskshell Crawler", !cfg.GorgeAutopaySurface}, // eight mana
			{"", false}, // seven: the Crawler's mana is missing
		} {
			sc := newScenTableLib(t, tbl, id["Forest"], cfg)
			if !sc.s.k().gen.anyGatePair {
				t.Fatal("anyGatePair unset: Whisperer is not a gated pair in this table")
			}
			for _, n := range append(append([]string(nil), base...), c.extra) {
				if n != "" {
					sc.perm(id[n], 0)
				}
			}
			sp := sc.hand(id["Goliath Spider"], 0)
			if got := sc.castOffered(0, sp); got != c.want {
				t.Errorf("lower %v autopay %v, base board plus %q: Goliath Spider offered = %v, want %v",
					cfg.GorgeLowering, cfg.GorgeAutopaySurface, c.extra, got, c.want)
			}
		}
	}
}
