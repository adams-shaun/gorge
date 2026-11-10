package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// Two filter words the kernel already read and kgen now spells, played
// decision by decision against gorge: Radioactive Spider's sacrifice-activated
// library search for a card that is both a Spider and a Hero (its own Spider
// copies stay in the library and are not offered), and Lyra, Tolarian
// Archangel's until-end-of-turn "whenever Lyra deals combat damage to a
// player, draw two cards" (the Effect's source object is the damage source).
// TestStdSetReprosLockstep replays the repros; TestFilterThreeWordsReprosReach
// requires each recorded game to reach the card's effect, since a lockstep
// entry alone would pass a script that never used it.
//
// The games are recorded by TestFilterThreeWordsRecord (FILTER_THREE_WORDS_RECORD=1)
// on the kernel table built from the decks: seat 0 casts the case's card, then
// uses its ability, otherwise plays a land or picks at random from the seed's
// own stream.
func filterThreeWordsCases() []effectLongCase {
	return []effectLongCase{
		{
			name:   "radioactive_spider_searches_a_spider_hero",
			prefer: []string{"Radioactive Spider"},
			decks: [2][]string{
				append(append(append(deckOf(6, "Radioactive Spider"), deckOf(4, "Lady Spider, Maybelle Reilly")...), deckOf(2, "Iron Spider, Stark Upgrade")...),
					append(deckOf(6, "Forest"), deckOf(6, "Mountain")...)...),
				deckOf(24, "Mountain"),
			},
			surface: "autopay",
			reached: func(g *GorgeSide) bool {
				for _, e := range g.E.L.Events {
					if e.Kind == events.SearchedLibrary {
						return true
					}
				}
				return false
			},
		},
		{
			name:   "lyra_effect_trigger_draws_on_combat_damage",
			prefer: []string{"Lyra, Tolarian Archangel"},
			decks: [2][]string{
				append(deckOf(8, "Lyra, Tolarian Archangel"), deckOf(16, "Island")...),
				deckOf(24, "Mountain"),
			},
			surface: "autopay",
			reached: func(g *GorgeSide) bool {
				for _, e := range g.E.L.Events {
					// the only way this deck makes a token is Lyra's end-step
					// trigger, which needs three draws in a turn: the draw step
					// and the two her activated Effect's trigger draws
					if e.Kind == events.TokenCreate {
						return true
					}
				}
				return false
			},
		},
	}
}

// filterThreeWordsPolicy is effectLongPolicy, except that an activation of the
// case's card comes before a land play.
func filterThreeWordsPolicy(m *Matchup, c effectLongCase, seed uint64) Policy {
	inner := effectLongPolicy(m, c, seed)
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind == DPriority {
			for _, o := range p.D.Options {
				if o.Kind() == ActActivate && m.InstanceCard(int(o.Src())) == c.prefer[0] {
					return o
				}
			}
		}
		return inner.Pick(p)
	})
}

func filterThreeWordsPlay(t *testing.T, reg *cards.Registry, c effectLongCase, r *Repro, set cardset.Set, pol Policy) (Result, bool) {
	return effectLongPlay(t, reg, c, r, set, pol)
}

// TestFilterThreeWordsReprosReach replays the recorded games: no divergence,
// every scripted action offered, the card's effect reached.
func TestFilterThreeWordsReprosReach(t *testing.T) {
	reg := synthReg(t)
	for _, c := range filterThreeWordsCases() {
		t.Run(c.name, func(t *testing.T) {
			r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			script := &ScriptPolicy{Actions: r.Actions}
			res, reached := filterThreeWordsPlay(t, reg, c, r, printedDelveSet(t, r.Cards), script)
			if res.Div != nil {
				t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if script.Skipped != 0 {
				t.Errorf("%d scripted actions were not offered", script.Skipped)
			}
			if !reached {
				t.Fatalf("the recorded game never reaches the effect of %s", c.name)
			}
		})
	}
}

// TestFilterThreeWordsRecord plays the cases' policies over seeds until one
// reaches the effect, and writes the repro. It runs only with
// FILTER_THREE_WORDS_RECORD=1.
func TestFilterThreeWordsRecord(t *testing.T) {
	if os.Getenv("FILTER_THREE_WORDS_RECORD") == "" {
		return // a recording tool, not coverage: it returns rather than skips, since a skip here must name a missing pool (testdata/poolskips.txt)
	}
	reg := synthReg(t)
	for _, c := range filterThreeWordsCases() {
		t.Run(c.name, func(t *testing.T) {
			set := printedDelveSet(t, c.decks)
			for seed := uint64(1); seed <= 400; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom"}
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, reached := filterThreeWordsPlay(t, reg, c, r, set, filterThreeWordsPolicy(m, c, seed))
				if res.Div != nil {
					t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
				}
				if !reached {
					continue
				}
				r.setActions(res.Actions)
				if err := r.Write(filepath.Join("testdata", "repro", c.name+".repro.json")); err != nil {
					t.Fatal(err)
				}
				t.Logf("seed %d: %d decisions", seed, len(res.Actions))
				return
			}
			t.Fatalf("no seed reached the effect of %s", c.name)
		})
	}
}
