package oracle

import (
	"slices"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// TestMBCTriggerReferents1Lowers plays fixture decks in lockstep with gorge
// leading, in the dr-mbc split that holds the carrier (every card of a case is
// in that split), one per spelling the mbc-trigger-referents-1 family lowers,
// each deck pair holding the carrier beside what makes its trigger fire:
//
//   - BecomesTarget with no source restriction (Angelic Protector:
//     Card.Self; Daru Spiritualist: Cleric.Creature+YouCtrl, the targeted
//     permanent as TriggeredTarget): burn aimed at them. Cephalid Illusionist,
//     the family's other Card.Self card, is refused by name for its own
//     prevention ability (dirac/support/refused/cephalid-illusionist.json).
//   - TargetsWithDefinedController$ TriggeredTarget under ValidTarget$ Player
//     (Soltari Visionary's enchantment, Sigil of Sleep's creature) and its
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
		a, b     []cardset.Entry
		carriers []string
	}{
		{ // Card.Self, any source: burn aimed at it
			[]cardset.Entry{e("Angelic Protector", 16), e("Plains", 24)},
			[]cardset.Entry{e("Grapeshot", 4), e("Solar Blast", 4), e("Blastfire Bolt", 4), e("Mountain", 28)},
			[]string{"Angelic Protector"}},
		{ // a Cleric watcher, the Cleric itself the target
			[]cardset.Entry{e("Daru Spiritualist", 16), e("Plains", 24)},
			[]cardset.Entry{e("Forked Lightning", 4), e("Torrent of Fire", 4), e("Battle Cry Goblin", 4), e("Burning Prophet", 4), e("Mountain", 24)},
			[]string{"Daru Spiritualist"}},
		{ // Skullsnatcher's graveyard slot: the damaged player's burn in their graveyard
			[]cardset.Entry{e("Skullsnatcher", 12), e("Changeling Outcast", 4), e("Swamp", 24)},
			[]cardset.Entry{e("Pyrotechnics", 6), e("Anarchist", 6), e("Mountain", 28)},
			[]string{"Skullsnatcher"}},
		{ // Soltari Visionary destroys the damaged player's enchantment
			[]cardset.Entry{e("Soltari Visionary", 16), e("Plains", 24)},
			[]cardset.Entry{e("Bastion of Remembrance", 4), e("Baffling End", 4), e("The Birth of Meletis", 4), e("Plains", 14), e("Swamp", 14)},
			[]string{"Soltari Visionary"}},
		{ // Sigil of Sleep bounces a creature of the damaged player
			[]cardset.Entry{e("Sigil of Sleep", 8), e("Daring Saboteur", 8), e("Codecracker Hound", 4), e("Island", 20)},
			[]cardset.Entry{e("Kangee's Lieutenant", 4), e("Devout Witness", 4), e("Inspiring Overseer", 4), e("Akroan Phalanx", 4), e("Plains", 24)},
			[]string{"Sigil of Sleep"}},
	}
	reg := synthReg(t)
	for _, c := range cases {
		t.Run(c.carriers[0], func(t *testing.T) {
			t.Parallel()
			decks := [2][]cardset.Entry{c.a, c.b}
			pool, set := mbcSplitOf(t, c.carriers[0])
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
				t.Fatalf("%s: %v", pool, err)
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
			t.Logf("%s: carriers played %v", pool, played)
		})
	}
}
