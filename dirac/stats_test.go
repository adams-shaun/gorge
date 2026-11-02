package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/vanilla"
)

func TestGameStats(t *testing.T) {
	var turns, decs, skipped, lifeEnd, deckEnd, casts, attacks int
	var kinds [16]int
	const G = 400
	for g := 0; g < G; g++ {
		s := NewConfig(vanilla.Table, vanilla.Decks[g%4], vanilla.Decks[(g/4)%4], uint64(g), Config{Mulligans: 2})
		pol := NewRandomPolicy(uint64(g) * 31)
		var d Decision
		for {
			s.StepInto(&d)
			if d.Kind == DkNone {
				break
			}
			kinds[d.Kind]++
			a := pol.Pick(&d)
			switch a.Kind() {
			case AkCast, AkCastCasualty:
				casts++
			case AkAttack:
				attacks++
			}
			s.Apply(a)
			decs++
		}
		turns += int(s.turn)
		skipped += int(s.Skipped[0] + s.Skipped[1])
		if s.pl[0].drewEmpty || s.pl[1].drewEmpty {
			deckEnd++
		} else {
			lifeEnd++
		}
	}
	t.Logf("games %d: avg turns %.1f, decisions %.1f, skipped %.1f, casts %.1f, attacks %.1f; life-end %d deck-end %d",
		G, float64(turns)/G, float64(decs)/G, float64(skipped)/G, float64(casts)/G, float64(attacks)/G, lifeEnd, deckEnd)
	t.Logf("decision kinds: %v", kinds[:9])
}
