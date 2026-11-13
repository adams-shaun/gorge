package oracle

import (
	"slices"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
)

// TestMBCTriggerReferents1Lowers plays fixture pools of the dr-mbc halves in
// lockstep with gorge leading, one per spelling the mbc-trigger-referents-1
// family lowers, each deck pair holding the carrier beside what makes its
// trigger fire:
//
//   - BecomesTarget with no source restriction (Angelic Protector:
//     Card.Self; Daru Spiritualist: Cleric.Creature+YouCtrl, the targeted
//     permanent as TriggeredTarget): burn aimed at them. Cephalid Illusionist,
//     the family's other Card.Self card, is refused by name for its own
//     prevention ability (dirac/support/refused/cephalid-illusionist.json).
//   - TargetsWithDefinedController$ TriggeredTarget under ValidTarget$ Player
//     (Soltari Visionary's enchantment, Sigil of Sleep's creature, which an
//     opponent's enchanted creature can aim at its own side) and its
//     graveyard card slot (Skullsnatcher).
//
// Each carrier must be played by some game, so the pool reaches the card; the
// comparison at every decision is gorge's.
func TestMBCTriggerReferents1Lowers(t *testing.T) {
	if isolate(t) {
		return
	}
	e := func(n string, c int) cardset.Entry { return cardset.Entry{Name: n, Count: c} }
	cases := []struct {
		pool     string
		a, b     []cardset.Entry
		carriers []string
	}{
		{"dr-mbc-aa", // Card.Self, any source
			[]cardset.Entry{e("Angelic Protector", 8), e("Adanto Vanguard", 4), e("Aven Squire", 4), e("Plains", 12), e("Island", 12)},
			[]cardset.Entry{e("Lightning Bolt", 4), e("Abrade", 4), e("Chandra's Pyrohelix", 4), e("Fireball", 3), e("Axgard Cavalry", 4), e("Blood Knight", 4), e("Mountain", 17)},
			[]string{"Angelic Protector"}},
		{"dr-mbc-ab", // a Cleric watcher, and Skullsnatcher's graveyard slot
			[]cardset.Entry{e("Daru Spiritualist", 4), e("Skullsnatcher", 4), e("Swords to Plowshares", 4), e("Beloved Chaplain", 4), e("Cast Down", 4), e("Plains", 10), e("Swamp", 10)},
			[]cardset.Entry{e("Galvanic Blast", 4), e("Bombard", 4), e("Staggershock", 3), e("Battle Cry Goblin", 4), e("Burning Prophet", 4), e("Ardent Elementalist", 3), e("Mountain", 18)},
			[]string{"Daru Spiritualist", "Skullsnatcher"}},
		{"dr-mbc-ba", // Soltari Visionary destroys the damaged player's enchantment
			[]cardset.Entry{e("Soltari Visionary", 4), e("Accorder Paladin", 4), e("Ajani's Pridemate", 4), e("Aerial Responder", 4), e("Angelic Page", 4), e("Plains", 20)},
			[]cardset.Entry{e("Narcolepsy", 4), e("Seal of Removal", 4), e("Augur of Bolas", 4), e("Battlewing Mystic", 4), e("Amphin Pathmage", 4), e("Cerulean Wyvern", 3), e("Roiling Dragonstorm", 3), e("Island", 14)},
			[]string{"Soltari Visionary"}},
		{"dr-mbc-bb", // Sigil of Sleep bounces a creature of the damaged player, on either side
			[]cardset.Entry{e("Sigil of Sleep", 4), e("Amoeboid Changeling", 4), e("Academy Journeymage", 4), e("Aethersnipe", 3), e("Essence Scatter", 4), e("Reality Shift", 4), e("Island", 17)},
			[]cardset.Entry{e("Angel of Vitality", 4), e("Blessed Spirits", 4), e("Ballynock Cohort", 4), e("Akroan Phalanx", 4), e("Spirited Companion", 4), e("Condemn", 3), e("Plains", 17)},
			[]string{"Sigil of Sleep"}},
	}
	reg := synthReg(t)
	for _, c := range cases {
		t.Run(c.pool, func(t *testing.T) {
			t.Parallel()
			decks := [2][]cardset.Entry{c.a, c.b}
			set := cardsettest.Synth(t, c.pool)
			var cs [2][]*cards.Card
			for s, d := range decks {
				n := 0
				for _, en := range d {
					card, ok := reg.Lookup(en.Name)
					if !ok {
						t.Fatalf("%s not in the corpus", en.Name)
					}
					for i := 0; i < en.Count; i++ {
						cs[s] = append(cs[s], card)
					}
					n += en.Count
				}
				if n != 40 {
					t.Fatalf("deck %d has %d cards", s, n)
				}
			}
			var mu sync.Mutex
			played := map[string]int{}
			tbl, _, _, err := set.Matchup(decks[0], decks[1])
			if err != nil {
				t.Fatalf("%s: %v", c.pool, err)
			}
			for seed := uint64(1); seed <= 60; seed++ {
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				m, err := NewMatchup(reg, [2]string{"fixture-a", "fixture-b"}, cs, 0)
				if err != nil {
					t.Fatal(err)
				}
				k, err := SetSide(set, tbl, cfg)(m, seed)
				if err != nil {
					t.Fatal(err)
				}
				res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
				CloseSide(k)
				if d := res.Div; d != nil {
					t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v\nrecent %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions, res.RecentSources(m, 6))
				}
				for _, name := range res.RecentSources(m, len(res.Actions)) {
					if slices.Contains(c.carriers, name) {
						mu.Lock()
						played[name]++
						mu.Unlock()
					}
				}
			}
			for _, name := range c.carriers {
				if played[name] == 0 {
					t.Errorf("%s was never played in 60 games: the pool does not reach it", name)
				}
			}
			t.Logf("%s: carriers played %v", c.pool, played)
		})
	}
}
