package oracle

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Squandered Resources variants: the card's sacrificed-land reflection
// must price the same plan on both engines as soon as the board is more than
// basic Forests/Swamps and a one-pip cast. Each case runs the scripted line
// (play a land, cast Squandered Probe, then on even seeds cast whatever the
// engine offers) and asserts a divergence-free lockstep plus that gorge did
// sacrifice a land under Squandered: a vacuous comparison proves nothing.
//
//   - dual: a victim producing two symbols (Tai Probe's {R}/{G}) asks a
//     colour after the sacrifice; the answer must be the play's owed one, in
//     gorge's script order.
//   - blank: a victim that produces nothing (Blank Probe) still resolves as
//     the search's answer and adds no mana at all.
//   - filter: a {1},{T} filter beside the source (Sig Probe) funds its fee
//     from the reflected activation, so the cast only the two together pay
//     is offered.
//   - grant: a granted {T}: Add {G} beside the source (Grant Probe) must
//     never float a symbol the sacrificed victim does not make.
const taiProbeScript = "Name:Tai Probe\nTypes:Land\n" +
	"A:AB$ Mana | Cost$ T | Produced$ Combo R G\n" +
	"Oracle:{T}: Add {R} or {G}.\n"

const blankProbeScript = "Name:Blank Probe\nTypes:Land\n" +
	"Oracle:\n"

const sigProbeScript = "Name:Sig Probe\nManaCost:1\nTypes:Artifact\n" +
	"A:AB$ Mana | Cost$ 1 T | Produced$ G W\n" +
	"Oracle:{1}, {T}: Add {G} or {W}.\n"

const grantProbeScript = "Name:Grant Probe\nManaCost:G\nTypes:Creature Human\nPT:1/1\n" +
	"S:Mode$ Continuous | Affected$ Creature.YouCtrl | AddAbility$ Tap\n" +
	"SVar:Tap:AB$ Mana | Cost$ T | Produced$ G\n" +
	"Oracle:Creatures you control have \"{T}: Add {G}.\"\n"

const doveProbeScript = "Name:Dove Probe\nManaCost:W\nTypes:Creature Bird\nPT:1/1\n" +
	"Oracle:1/1\n"

const duoProbeScript = "Name:Duo Probe\nManaCost:G G\nTypes:Creature Elf\nPT:1/1\n" +
	"Oracle:1/1\n"

func concatDecks(parts ...[]string) []string {
	var out []string
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// hostedCmdrSquanderedVariants is the variants subtest of
// TestHostedCmdrManaShapesLockstep.
func hostedCmdrSquanderedVariants(t *testing.T) {
	cases := []struct {
		name     string
		fixtures map[string]string
		deck     []string
	}{
		{"dual", map[string]string{"Tai Probe": taiProbeScript, "Cub Probe": cubProbeScript, "Squandered Probe": squanderedProbeScript},
			concatDecks(deckOf(14, "Squandered Probe"), deckOf(4, "Tai Probe"), deckOf(10, "Forest"), deckOf(8, "Swamp"), deckOf(4, "Cub Probe"))},
		{"blank", map[string]string{"Blank Probe": blankProbeScript, "Cub Probe": cubProbeScript, "Squandered Probe": squanderedProbeScript},
			concatDecks(deckOf(14, "Squandered Probe"), deckOf(4, "Blank Probe"), deckOf(12, "Forest"), deckOf(6, "Swamp"), deckOf(4, "Cub Probe"))},
		{"filter", map[string]string{"Sig Probe": sigProbeScript, "Dove Probe": doveProbeScript, "Duo Probe": duoProbeScript, "Squandered Probe": squanderedProbeScript},
			concatDecks(deckOf(12, "Squandered Probe"), deckOf(4, "Sig Probe"), deckOf(8, "Forest"), deckOf(8, "Swamp"), deckOf(4, "Dove Probe"), deckOf(4, "Duo Probe"))},
		{"grant", map[string]string{"Grant Probe": grantProbeScript, "Dove Probe": doveProbeScript, "Duo Probe": duoProbeScript, "Squandered Probe": squanderedProbeScript},
			concatDecks(deckOf(12, "Squandered Probe"), deckOf(4, "Grant Probe"), deckOf(8, "Forest"), deckOf(8, "Swamp"), deckOf(4, "Dove Probe"), deckOf(4, "Duo Probe"))},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if len(c.deck) != 40 {
				t.Fatalf("deck size = %d, want 40", len(c.deck))
			}
			dir := filters11Corpus(t, c.fixtures, "Forest", "Swamp")
			reg, err := cards.SharedCorpus(dir)
			if err != nil {
				t.Fatal(err)
			}
			decks := [2][]string{c.deck, deckOf(40, "Forest")}
			var cardDecks [2][]*cards.Card
			for s, names := range decks {
				for _, n := range names {
					card, ok := reg.Lookup(n)
					if !ok {
						t.Fatalf("%s not in the corpus", n)
					}
					cardDecks[s] = append(cardDecks[s], card)
				}
			}
			entries := func(names []string) []cardset.Entry {
				es := make([]cardset.Entry, len(names))
				for i, n := range names {
					es[i] = cardset.Entry{Name: n, Count: 1}
				}
				return es
			}
			set := printedDelveSetIn(t, dir, decks)
			tbl, _, _, err := set.Matchup(entries(decks[0]), entries(decks[1]))
			if err != nil {
				t.Fatal(err)
			}
			seen := false
			for seed := uint64(1); seed <= 12; seed++ {
				m, err := NewMatchup(reg, [2]string{"hms-squa-var-a", "hms-squa-var-b"}, cardDecks, 1)
				if err != nil {
					t.Fatal(err)
				}
				k, err := SetSide(set, tbl, Config{})(m, seed)
				if err != nil {
					t.Fatal(err)
				}
				pol := hostedCmdrSquanderedProbePolicy(m, seed, seed%2 == 0)
				rc := RunConfig{Config: Config{}, MaxDecisions: 6000, MaxTurns: 40}
				rc.Finish = func(g *GorgeSide) {
					if hostedCmdrSquanderedSacrificed(g) {
						seen = true
					}
				}
				res := Lockstep(m, seed, k, pol, rc)
				if res.Div != nil {
					d := res.Div
					n := len(res.Actions) - 8
					if n < 0 {
						n = 0
					}
					var names []string
					for _, a := range res.Actions[n:] {
						names = append(names, fmt.Sprintf("%v:%q", a, m.InstanceCard(int(a.Src()))))
					}
					CloseSide(k)
					t.Fatalf("seed %d: divergence at %d: %s %s %v detail=%q last=%v\ngorge %v\ndirac %v",
						seed, d.At, d.What, d.Field, d.Diff, d.Detail, names, d.RefOptions, d.KerOptions)
				}
				CloseSide(k)
			}
			if !seen {
				t.Fatalf("Squandered's sacrifice-reflected activation was not used on any seed, so the lockstep is vacuous")
			}
		})
	}
}

// hostedCmdrSquanderedProbePolicy drives the variants: cast Squandered Probe
// whenever offered, cast anything else on even seeds, and otherwise play a
// land or answer randomly.
func hostedCmdrSquanderedProbePolicy(m *Matchup, seed uint64, even bool) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.G == nil || p.D.Player != 0 || p.D.Kind != DPriority {
			return rnd.Pick(p)
		}
		for _, a := range p.D.Options {
			if a.Kind() == ActCast && m.InstanceCard(int(a.Src())) == "Squandered Probe" {
				return a
			}
		}
		if even {
			for _, a := range p.D.Options {
				if a.Kind() == ActCast {
					return a
				}
			}
		}
		for _, a := range p.D.Options {
			if a.Kind() == ActPlayLand {
				return a
			}
		}
		return rnd.Pick(p)
	})
}

// hostedCmdrSquanderedSacrificed reports whether seat 0 sacrificed a land
// while a Squandered Probe was on the battlefield: the sacrifice-reflected
// activation's own cost, read from gorge's events rather than a cast the
// policy happened to offer.
func hostedCmdrSquanderedSacrificed(g *GorgeSide) bool {
	if g == nil || g.E == nil {
		return false
	}
	squandered := false
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		if e.Kind == events.MoveZone && e.To == state.ZBattlefield {
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Squandered Probe" && o.Controller == 0 {
				squandered = true
			}
		}
		if !squandered || !events.IsSacrifice(*e) {
			continue
		}
		o := g.E.G.Obj(e.Obj)
		if o == nil || o.Face() == nil {
			continue
		}
		switch o.Face().Name {
		case "Forest", "Swamp", "Tai Probe", "Blank Probe":
			return true
		}
	}
	return false
}
