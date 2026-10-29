package mapping_test

// sbp-unmapped-dig-division: the v2 mapping must express the Dig spellings
// the corpus actually carries (a non-hand primary, an explicit library
// remainder with no second position, a random remainder, DigNum$ X), a
// two-blocker combat division whose blocker names contain commas, and the
// TwoPiles / hidden-pick choose shapes. These are the asks behind
// dig_shape, dig_num, dig_bottom_missing, division_layout and
// unmapped:choose/twopiles.

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/spellbench/gamecfg"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testcorpus"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testgame"
	"github.com/adams-shaun/gorge/server/spellbench/mapping"
	"github.com/adams-shaun/gorge/server/spellbench/secrets"
)

// findDigAbility returns the card's Dig SA, whether it is printed as an
// activated/spell ability or reached through a trigger's Execute$ SVar.
func findDigAbility(t *testing.T, c *cards.Card) *cards.SA {
	t.Helper()
	for _, ab := range c.Faces[0].Abilities {
		if ab.API == "Dig" {
			return ab
		}
	}
	for _, tr := range c.Faces[0].Triggers {
		if tr.Effect != nil && tr.Effect.API == "Dig" {
			return tr.Effect
		}
	}
	t.Fatalf("%s has no Dig ability", c.Faces[0].Name)
	return nil
}

func lookupCard(t *testing.T, reg *cards.Registry, name string) *cards.Card {
	t.Helper()
	c, ok := reg.Lookup(name)
	if !ok {
		t.Fatalf("%s missing from the corpus", name)
	}
	return c
}

// digAsk builds the take decision a Dig poses: Min 0 (the corpus cards are
// Optional$ True), Max ChangeNum, one option per offered card in window order.
func digTakeDecision(t *testing.T, g *gamecfg.Game, sa *cards.SA, prompt string, window int) *decision.Decision {
	t.Helper()
	lib := g.E.G.Zone(state.ZLibrary, 0)
	if len(lib) < window {
		t.Fatalf("library holds %d cards, want at least the %d-card window", len(lib), window)
	}
	d := &decision.Decision{Kind: decision.KChoose, Player: 0, Min: 0, Max: 1,
		ResumeSA: sa, Prompt: prompt, Source: 0}
	for i, id := range lib[:window] {
		d.Options = append(d.Options, decision.Option{Index: i, Kind: "dig", Label: "a card", Obj: id, Player: 0})
	}
	return d
}

func TestDigShapeCoversRepoSpellings(t *testing.T) {
	reg := testcorpus.Registry(t)
	g := testgame.New(t, reg, "Spy", "Spy", 1, "none")
	env := envFor(t, g)

	squad := findDigAbility(t, lookupCard(t, reg, "Squad Rallier"))
	if squad.Params["DestinationZone"] != "Hand" || squad.Params["DestinationZone2"] != "Library" ||
		squad.Params["LibraryPosition2"] != "" || !strings.EqualFold(squad.Params["RestRandomOrder"], "True") {
		t.Fatalf("Squad Rallier's Dig shape drifted: %v", squad.Params)
	}
	plunderer := findDigAbility(t, lookupCard(t, reg, "Gutless Plunderer"))
	if plunderer.Params["DestinationZone"] != "Library" || plunderer.Params["LibraryPosition"] != "0" ||
		plunderer.Params["DestinationZone2"] != "Graveyard" {
		t.Fatalf("Gutless Plunderer's Dig shape drifted: %v", plunderer.Params)
	}
	loot := findDigAbility(t, lookupCard(t, reg, "Loot, Exuberant Explorer"))
	if loot.Params["DestinationZone"] != "Battlefield" || !strings.EqualFold(loot.Params["RestRandomOrder"], "True") {
		t.Fatalf("Loot, Exuberant Explorer's Dig shape drifted: %v", loot.Params)
	}

	cases := []struct {
		name   string
		sa     *cards.SA
		window int
		prompt string
		random bool
	}{
		{"Squad Rallier", squad, 4, "Look at the top 4 card(s) of your library", true},
		{"Gutless Plunderer", plunderer, 3, "Look at the top 3 card(s) of your library", false},
		{"Loot, Exuberant Explorer", loot, 6, "Look at the top 6 card(s) of your library", true},
		{"DigNum X resolves from the prompt", &cards.SA{API: "Dig", Params: map[string]string{
			"DigNum": "X", "ChangeNum": "1", "DestinationZone": "Hand", "DestinationZone2": "Graveyard"}},
			4, "Look at the top 4 card(s) of your library", false},
	}
	t.Run("WithTotalCMC budget dead end", func(t *testing.T) {
		// Min 2 over values 1, 2, 3 with a cap of 3: only the 1-value card
		// leaves room for a second pick (1+2), so the 2- and 3-value partition
		// candidates must not be offered at the first step.
		sa := &cards.SA{API: "Dig", Params: map[string]string{
			"DigNum": "3", "ChangeNum": "2", "DestinationZone": "Hand", "WithTotalCMC": "X"}}
		d := digTakeDecision(t, g, sa, "Look at the top 3 card(s) of your library", 3)
		d.Min, d.Max, d.MaxSum = 2, 2, 3
		d.Options[0].Value = 1
		d.Options[1].Value = 2
		d.Options[2].Value = 3
		tx, err := mapping.Begin(env, d)
		if err != nil {
			t.Fatalf("Begin: %v", err)
		}
		p, err := tx.Pose()
		if err != nil {
			t.Fatalf("Pose: %v", err)
		}
		primary := 0
		for _, c := range p.Candidates {
			if c.Sem.Kind == "arrange_card" && c.Sem.Fields["destination"] == "hand" {
				primary++
			}
			if err := c.Sem.Check(); err != nil {
				t.Fatalf("candidate %s fails the validator: %v", c.Sem.Kind, err)
			}
		}
		if primary != 1 {
			t.Fatalf("%d primary candidates, want only the 1-value card", primary)
		}
	})

	t.Run("WithTotalCMC budget", func(t *testing.T) {
		// The corpus's budgeted Dig (a sacrificed artifact's mana value as the
		// cap): only the budget-eligible card is offered, and the picked set
		// must stay within Decision.MaxSum.
		sa := &cards.SA{API: "Dig", Params: map[string]string{
			"DigNum": "8", "ChangeNum": "2", "Optional": "True", "WithTotalCMC": "X",
			"DestinationZone": "Battlefield", "DestinationZone2": "Library", "RestRandomOrder": "True"}}
		d := digTakeDecision(t, g, sa, "Look at the top 8 card(s) of your library", 8)
		d.MaxSum = 4
		d.Options = d.Options[:1]
		d.Options[0].Value = 4
		tx, err := mapping.Begin(env, d)
		if err != nil {
			t.Fatalf("Begin: %v", err)
		}
		commit := answerAll(t, tx, func(p *mapping.Pose) int {
			for i, c := range p.Candidates {
				if err := c.Sem.Check(); err != nil {
					t.Fatalf("candidate %s fails the validator: %v", c.Sem.Kind, err)
				}
				if c.Sem.Kind == "arrange_card" && c.Sem.Fields["destination"] == "battlefield" {
					return i
				}
			}
			return 0
		})
		if len(commit) != 1 || len(commit[0].Choices) > 1 {
			t.Fatalf("budgeted dig committed %v, want one take", commit)
		}
	})

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := digTakeDecision(t, g, tc.sa, tc.prompt, tc.window)
			tx, err := mapping.Begin(env, d)
			if err != nil {
				t.Fatalf("Begin: %v", err)
			}
			var commit []decision.Intent
			for {
				p, err := tx.Pose()
				if err != nil {
					t.Fatalf("Pose: %v", err)
				}
				if tc.random && p.Followups["dig_bottom"] != nil {
					t.Fatalf("a random remainder posed a dig_bottom follow-up")
				}
				for _, c := range p.Candidates {
					if err := c.Sem.Check(); err != nil {
						t.Fatalf("candidate %s fails the validator: %v", c.Sem.Kind, err)
					}
				}
				pick := 0
				for i, c := range p.Candidates {
					if c.Sem.Kind == "arrange_card" && c.Sem.Fields["destination"] != "bottom" {
						pick = i
						break
					}
				}
				ins, done, err := tx.Answer(pick)
				if err != nil {
					t.Fatalf("Answer: %v", err)
				}
				commit = append(commit, ins...)
				if done {
					break
				}
			}
			if len(commit) == 0 {
				t.Fatal("no intents committed")
			}
			if tc.random && len(commit) != 1 {
				t.Fatalf("a random remainder commits the take alone, got %d intents: %v", len(commit), commit)
			}
			for _, in := range commit {
				if in.Seq != d.Seq || in.Player != d.Player || len(in.Choices) == 0 {
					t.Fatalf("bad intent %+v for decision seq %d player %d", in, d.Seq, d.Player)
				}
			}
		})
	}
}

// customDeck builds a 40-card deck from counts in the order given.
func customDeck(t *testing.T, reg *cards.Registry, entries []struct {
	name  string
	count int
}) []*cards.Card {
	t.Helper()
	var out []*cards.Card
	for _, e := range entries {
		c := lookupCard(t, reg, e.name)
		for i := 0; i < e.count; i++ {
			out = append(out, c)
		}
	}
	forest := lookupCard(t, reg, "Forest")
	for len(out) < 40 {
		out = append(out, forest)
	}
	if len(out) != 40 {
		t.Fatalf("deck has %d cards, want 40", len(out))
	}
	return out
}

// divisionAsk drives a real game to a two-blocker damage division where one
// blocker's printed name contains a comma ("Sram, Senior Edificer"), the
// shape a label split on commas misreads.
func divisionAsk(t *testing.T) (*gamecfg.Game, *decision.Decision) {
	t.Helper()
	reg := testcorpus.Registry(t)
	attacker := customDeck(t, reg, []struct {
		name  string
		count int
	}{{"Grizzly Bears", 20}})
	blocker := customDeck(t, reg, []struct {
		name  string
		count int
	}{{"Sram, Senior Edificer", 8}, {"Grizzly Bears", 16}, {"Plains", 8}, {"Forest", 8}})
	for seed := byte(1); seed < 40; seed++ {
		buf := make([]byte, 32)
		buf[0] = seed
		g, err := gamecfg.New(reg, secrets.NewGame(buf), [2][]*cards.Card{attacker, blocker},
			gamecfg.Rules{Mulligan: "none"})
		if err != nil {
			t.Fatal(err)
		}
		bots := testgame.Bots(uint64(seed))
		for n := 0; n < 40000; n++ {
			d := g.E.Pending()
			if g.E.G.Over || d == nil {
				break
			}
			if d.Kind == decision.KChoose {
				for _, o := range d.Options {
					if o.Kind == "division" {
						return g, d
					}
				}
			}
			var in decision.Intent
			switch {
			case d.Kind == decision.KAttackers && d.Player == 0:
				var ch []int
				for _, o := range d.Options {
					if o.Obj != 0 {
						ch = append(ch, o.Index)
						break
					}
				}
				in = decision.Intent{Seq: d.Seq, Player: d.Player, Choices: ch}
			case d.Kind == decision.KBlockers && d.Player == 1:
				// Block with the comma-named creature plus one other, so the
				// division's printed label carries a comma.
				att := state.ObjID(0)
				seen := map[state.ObjID]bool{}
				var commaOpt, otherOpt int
				for _, o := range d.Options {
					if o.Attacker == 0 || seen[o.Obj] {
						continue
					}
					if att == 0 {
						att = o.Attacker
					}
					if o.Attacker != att {
						continue
					}
					seen[o.Obj] = true
					if strings.Contains(g.E.G.Obj(o.Obj).Face().Name, ",") {
						if commaOpt == 0 {
							commaOpt = o.Index + 1
						}
					} else if otherOpt == 0 {
						otherOpt = o.Index + 1
					}
				}
				var ch []int
				if commaOpt != 0 {
					ch = append(ch, commaOpt-1)
					if otherOpt != 0 {
						ch = append(ch, otherOpt-1)
					} else {
						// A second comma-named blocker is still a second blocker.
						for _, o := range d.Options {
							if o.Attacker == att && o.Index != commaOpt-1 && !slices.Contains(ch, o.Index) {
								if b := g.E.G.Obj(o.Obj); b != nil && strings.Contains(b.Face().Name, ",") {
									ch = append(ch, o.Index)
									break
								}
							}
						}
					}
				}
				in = decision.Intent{Seq: d.Seq, Player: d.Player, Choices: ch}
			default:
				in, _ = bots[d.Player].Decide(context.Background(), view.Project(g.E.G, g.E, d.Player, d), *d)
			}
			if err := g.Submit(in); err != nil {
				t.Fatalf("seed %d: %v", seed, err)
			}
		}
	}
	t.Fatal("no two-blocker division reached")
	return nil, nil
}

func TestDivisionLayoutMatchesEngineOptions(t *testing.T) {
	g, d := divisionAsk(t)
	env := envFor(t, g)
	if got := mapping.Route(d); got != "choose/division" {
		t.Fatalf("Route = %q, want choose/division", got)
	}
	a := g.E.G.Obj(d.Source)
	live := 0
	comma := false
	for _, bid := range a.BlockedBy {
		if b := g.E.G.Obj(bid); b != nil && b.Zone == state.ZBattlefield {
			live++
			if strings.Contains(b.Face().Name, ",") {
				comma = true
			}
		}
	}
	if live != 2 || !comma {
		t.Fatalf("precondition: %d live blockers, comma name %v; want a two-blocker ask whose label carries a comma", live, comma)
	}
	in, ok, err := mapping.Internal(env, d)
	if err != nil {
		t.Fatalf("engine_order division refused: %v", err)
	}
	if !ok {
		t.Fatal("choose/division is not registered as engine-internal")
	}
	if !mapping.Accepts(env, in) {
		t.Fatalf("the engine rejected its own division answer %+v", in)
	}
}

func runToDecision(t *testing.T, g *gamecfg.Game, seed uint64, pred func(*decision.Decision) bool) *decision.Decision {
	t.Helper()
	bots := testgame.Bots(seed)
	for n := 0; n < 40000; n++ {
		d := g.E.Pending()
		if g.E.G.Over || d == nil {
			t.Fatal("the game ended before the wanted decision")
		}
		if pred(d) {
			return d
		}
		in, _ := bots[d.Player].Decide(context.Background(), view.Project(g.E.G, g.E, d.Player, d), *d)
		if err := g.Submit(in); err != nil {
			t.Fatal(err)
		}
	}
	t.Fatal("the wanted decision never appeared")
	return nil
}

func TestTwoPilesAndHiddenPickRoute(t *testing.T) {
	reg := testcorpus.Registry(t)

	t.Run("Curator of Destinies split and pile pick", func(t *testing.T) {
		curator := customDeck(t, reg, []struct {
			name  string
			count int
		}{{"Curator of Destinies", 12}, {"Island", 28}})
		opp := customDeck(t, reg, []struct {
			name  string
			count int
		}{{"Forest", 40}})
		buf := make([]byte, 32)
		buf[0] = 7
		g, err := gamecfg.New(reg, secrets.NewGame(buf), [2][]*cards.Card{curator, opp},
			gamecfg.Rules{Mulligan: "none"})
		if err != nil {
			t.Fatal(err)
		}
		split := runToDecision(t, g, 7, func(d *decision.Decision) bool {
			return d.Kind == decision.KChoose && len(d.Options) > 0 && d.Options[0].Kind == "twopiles"
		})
		if got := mapping.Route(split); got != "choose/twopiles" {
			t.Fatalf("Route = %q, want choose/twopiles", got)
		}
		env := envFor(t, g)
		tx, err := mapping.Begin(env, split)
		if err != nil {
			t.Fatalf("Begin split: %v", err)
		}
		commit := answerAll(t, tx, func(p *mapping.Pose) int {
			if len(p.Candidates) == 0 {
				t.Fatal("no split candidates")
			}
			for _, k := range p.Known {
				if k.Zone == "library" && k.PositionFromTop == nil {
					t.Fatalf("known library entry %q has no position", k.CardName)
				}
			}
			selects := 0
			for i, c := range p.Candidates {
				switch c.Sem.Kind {
				case "select_object":
					selects++
				case "finish_selection":
				default:
					t.Fatalf("split candidate kind %s, want select_object or finish_selection", c.Sem.Kind)
				}
				if err := c.Sem.Check(); err != nil {
					t.Fatalf("split candidate fails the validator: %v", err)
				}
				if c.Sem.Kind == "select_object" {
					return i
				}
			}
			if selects == 0 {
				t.Fatal("the split pose offers no selectable card")
			}
			return 0 // finish_selection: at least one card selected
		})
		if len(commit) != 1 {
			t.Fatalf("split committed %d intents, want 1: %v", len(commit), commit)
		}
		if got := len(commit[0].Choices); got == 0 || got > len(split.Options) {
			t.Fatalf("split chose %d cards over %d options", got, len(split.Options))
		}
		if err := g.Submit(commit[0]); err != nil {
			t.Fatalf("the engine rejected the split: %v", err)
		}

		pick := runToDecision(t, g, 7, func(d *decision.Decision) bool {
			if d.Kind != decision.KChoose || len(d.Options) != 2 {
				return false
			}
			return d.Options[0].Kind == "pile-a" && d.Options[1].Kind == "pile-b"
		})
		if got := mapping.Route(pick); got != "choose/pile_pick" {
			t.Fatalf("Route = %q, want choose/pile_pick", got)
		}
		env = envFor(t, g)
		tx, err = mapping.Begin(env, pick)
		if err != nil {
			t.Fatalf("Begin pile pick: %v", err)
		}
		p, err := tx.Pose()
		if err != nil {
			t.Fatalf("Pose pile pick: %v", err)
		}
		if len(p.Candidates) != 2 {
			t.Fatalf("%d pile candidates, want 2", len(p.Candidates))
		}
		for _, c := range p.Candidates {
			if c.Sem.Kind != "choose_option" {
				t.Fatalf("pile pick candidate kind %s, want choose_option", c.Sem.Kind)
			}
			if err := c.Sem.Check(); err != nil {
				t.Fatalf("pile pick candidate fails the validator: %v", err)
			}
		}
		ins, done, err := tx.Answer(0)
		if err != nil || !done || len(ins) != 1 {
			t.Fatalf("pile pick answer %v done %v err %v", ins, done, err)
		}
		if err := g.Submit(ins[0]); err != nil {
			t.Fatalf("the engine rejected the pile pick: %v", err)
		}
	})

	t.Run("Inspiration from Beyond hidden pick", func(t *testing.T) {
		inspiration := lookupCard(t, reg, "Inspiration from Beyond")
		var sa *cards.SA
		for _, ab := range inspiration.Faces[0].Abilities {
			if ab.Sub != nil && strings.EqualFold(ab.Sub.Params["Hidden"], "True") {
				sa = ab.Sub
			}
		}
		if sa == nil || !strings.EqualFold(sa.Params["Destination"], "Hand") {
			t.Fatalf("Inspiration from Beyond's hidden ChangeZone shape drifted")
		}
		self := customDeck(t, reg, []struct {
			name  string
			count int
		}{{"Inspiration from Beyond", 8}, {"Lightning Bolt", 16}, {"Island", 16}})
		opp := customDeck(t, reg, []struct {
			name  string
			count int
		}{{"Island", 40}})
		buf := make([]byte, 32)
		buf[0] = 9
		g, err := gamecfg.New(reg, secrets.NewGame(buf), [2][]*cards.Card{self, opp},
			gamecfg.Rules{Mulligan: "none"})
		if err != nil {
			t.Fatal(err)
		}
		d := runToDecision(t, g, 9, func(d *decision.Decision) bool {
			return d.Kind == decision.KChoose && len(d.Options) > 0 && d.Options[0].Kind == "hidden_pick" && d.ResumeSA != nil
		})
		if got := mapping.Route(d); got != "choose/hidden_pick" {
			t.Fatalf("Route = %q, want choose/hidden_pick", got)
		}
		env := envFor(t, g)
		tx, err := mapping.Begin(env, d)
		if err != nil {
			t.Fatalf("Begin hidden pick: %v", err)
		}
		p, err := tx.Pose()
		if err != nil {
			t.Fatalf("Pose hidden pick: %v", err)
		}
		if len(p.Candidates) == 0 {
			t.Fatal("no hidden-pick candidates")
		}
		for _, k := range p.Known {
			if k.Zone == "library" && k.PositionFromTop == nil {
				t.Fatalf("known library entry %q has no position", k.CardName)
			}
		}
		for _, c := range p.Candidates {
			if c.Sem.Kind != "select_object" {
				t.Fatalf("hidden pick candidate kind %s, want select_object", c.Sem.Kind)
			}
			if err := c.Sem.Check(); err != nil {
				t.Fatalf("hidden pick candidate fails the validator: %v", err)
			}
		}
		ins, done, err := tx.Answer(0)
		if err != nil || !done || len(ins) != 1 {
			t.Fatalf("hidden pick answer %v done %v err %v", ins, done, err)
		}
		if err := g.Submit(ins[0]); err != nil {
			t.Fatalf("the engine rejected the hidden pick: %v", err)
		}
	})
}
