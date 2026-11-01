package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The trigger-own N-card graveyard exile lockstep:
// Aegis Sculptor (N=2), Skywarp Skaab (N=2, creature cards) and Emeritus of
// Ideation (N=8) pay ExileFromGrave<N/Spec> as their trigger's own cost
// (gorge's triggered-cost window, CR 118.3 / 601.2h): fewer than N eligible
// skips the body (EG<, a decline-only ask), exactly N is taken unasked (EG=),
// more asks an exile_cost pick (EG>).
var kexilegravenDecks = [][2][]cardset.Entry{{
	{
		{Name: "Aegis Sculptor", Count: 6}, {Name: "Skywarp Skaab", Count: 4}, {Name: "Emeritus of Ideation", Count: 2},
		{Name: "Thought Scour", Count: 6}, {Name: "Opt", Count: 4}, {Name: "Storm Crow", Count: 2}, {Name: "Island", Count: 16},
	},
	{
		{Name: "Grizzly Bears", Count: 8}, {Name: "Shock", Count: 8}, {Name: "Mountain", Count: 12}, {Name: "Forest", Count: 12},
	},
}, {
	{
		{Name: "Aegis Sculptor", Count: 6}, {Name: "Skywarp Skaab", Count: 4}, {Name: "Emeritus of Ideation", Count: 2},
		{Name: "Thought Scour", Count: 6}, {Name: "Opt", Count: 4}, {Name: "Storm Crow", Count: 2}, {Name: "Island", Count: 16},
	},
	{
		{Name: "Aegis Sculptor", Count: 6}, {Name: "Skywarp Skaab", Count: 4}, {Name: "Emeritus of Ideation", Count: 2},
		{Name: "Thought Scour", Count: 6}, {Name: "Opt", Count: 4}, {Name: "Storm Crow", Count: 2}, {Name: "Island", Count: 16},
	},
}}

// kexilegravenCounts is the per-branch tally, in games where the branch
// occurred and in total, keyed "<card>/<branch>".
type kexilegravenCounts struct {
	total map[string]int
	games map[string]int
}

func kexilegravenPolicy(seed uint64, cnt *kexilegravenCounts, seen map[string]bool) Policy {
	rnd := RandomPolicy(seed)
	flip := int(seed)
	awaiting := ""
	note := func(k string) {
		cnt.total[k]++
		if !seen[k] {
			seen[k] = true
			cnt.games[k]++
		}
	}
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			d := c.G.Pending()
			if awaiting != "" {
				if d != nil && len(d.Options) > 0 && d.Options[0].Kind == "exile_cost" {
					note(awaiting + "/EG>")
				} else {
					note(awaiting + "/EG=")
				}
				awaiting = ""
			}
			if d != nil {
				name := sourcecountercostsName(c, d)
				hasPay, hasDecline := false, false
				for _, o := range d.Options {
					switch o.Kind {
					case "trigger_cost_pay":
						hasPay = true
					case "trigger_cost_decline":
						hasDecline = true
					}
				}
				if hasPay {
					want := "trigger_cost_decline"
					if (flip+c.Seq)%3 != 0 {
						want = "trigger_cost_pay"
					}
					flip++
					for i, o := range d.Options {
						if o.Kind == want {
							if want == "trigger_cost_pay" {
								awaiting = name
							}
							return c.D.Options[i]
						}
					}
				} else if hasDecline && (name == "Aegis Sculptor" || name == "Skywarp Skaab" || name == "Emeritus of Ideation") {
					note(name + "/EG<")
				}
				if name == "Thought Scour" {
					for i, o := range d.Options {
						if o.Kind == "player" && o.Player == d.Player {
							return c.D.Options[i]
						}
					}
				}
			}
		}
		var buf []Canon
		for _, a := range c.D.Options {
			if a.Kind() == ActPlayLand {
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 {
			for _, a := range c.D.Options {
				if k := a.Kind(); k == ActCast || k == ActActivate {
					buf = append(buf, a)
				}
			}
		}
		if len(buf) == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

func TestExilegravenLockstep(t *testing.T) {
	t.Parallel()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("exilegraven")
	if err != nil {
		t.Fatal(err)
	}
	cnt := &kexilegravenCounts{total: map[string]int{}, games: map[string]int{}}
	div, games := 0, 0
	for mi, md := range kexilegravenDecks {
		var decks [2][]*cards.Card
		for s, d := range md {
			for _, e := range d {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					decks[s] = append(decks[s], c)
				}
			}
			if len(decks[s]) != 40 {
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= 200; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"eg-a", "eg-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			res := Lockstep(m, seed, k, kexilegravenPolicy(seed, cnt, map[string]bool{}), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30})
			CloseSide(k)
			games++
			if res.Stall != "" && res.Stall != "turns" {
				t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
			}
			if d := res.Div; d != nil {
				div++
				lo := max(d.At-12, 0)
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	t.Logf("%d games, %d divergences; games with a branch (total): %v / %v", games, div, cnt.games, cnt.total)
	for _, card := range []string{"Aegis Sculptor", "Skywarp Skaab"} {
		for _, br := range []string{"/EG>", "/EG=", "/EG<"} {
			if cnt.games[card+br] < 10 {
				t.Errorf("%s%s in %d games; want at least 10", card, br, cnt.games[card+br])
			}
		}
	}
}
