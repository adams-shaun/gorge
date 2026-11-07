package searchseat

import (
	"context"
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/searchprobe"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
)

func holdCard(t *testing.T, src string) *cards.Card {
	t.Helper()
	c, diags := cards.ParseBytes("holdupkeep_test.txt", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("parse: %v", diags)
	}
	c.Link()
	for _, f := range c.Faces {
		f.ApplyIntrinsics()
	}
	return c
}

// holdWindowEngine plays an Island and instant deck (seat 0) into an Island
// and bear deck with the plain bot until a priority decision of the active
// player in its own upkeep or draw step, stack empty, offers a cast of an
// instant from its hand; it taps an Island there first, since the manual
// vocabulary offers the cast once its mana floats. It returns the engine
// there and that option's index.
func holdWindowEngine(t *testing.T) (*rules.Engine, int) {
	t.Helper()
	opt := holdCard(t, "Name:Test Opt\nManaCost:U\nTypes:Instant\nA:SP$ Draw | NumCards$ 1 | SpellDescription:Draw a card.\nOracle:Draw a card.\n")
	bear := holdCard(t, "Name:Bear\nManaCost:1\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	island := holdCard(t, "Name:Island\nTypes:Basic Land Island\nOracle:x\n")
	var d0, d1 []*cards.Card
	for i := 0; i < 17; i++ {
		d0, d1 = append(d0, island), append(d1, island)
	}
	for i := 0; i < 8; i++ {
		d0, d1 = append(d0, opt), append(d1, bear)
	}
	inHand := func(g *state.Game, p state.PlayerID) bool {
		for _, id := range g.Zone(state.ZHand, p) {
			if o := g.Obj(id); o != nil && o.Card == opt {
				return true
			}
		}
		return false
	}
	for seed := uint64(1); seed < 40; seed++ {
		e := rules.New(rules.Config{Seed: seed, Names: []string{"a", "b"}, Decks: [][]*cards.Card{d0, d1}})
		e.Advance()
		rngs := searchprobe.BotRandoms(seed, 2)
		board := botpolicy.NewBoard(2)
		for steps := 0; steps < 3000 && !e.G.Over; steps++ {
			d := e.Pending()
			if d.Kind == decision.KPriority && d.Player == e.G.Active && len(e.G.Stack) == 0 &&
				(e.G.Step == state.StepUpkeep || e.G.Step == state.StepDraw) {
				tap := -1
				for i, o := range d.Options {
					if obj := e.G.Obj(o.Obj); o.Kind == "cast" && obj != nil && obj.Card == opt && obj.Zone == state.ZHand {
						return e, i
					}
					if o.Kind == "activate" && tap < 0 {
						tap = i
					}
				}
				if tap >= 0 && inHand(e.G, d.Player) {
					if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{tap}}); err != nil {
						t.Fatal(err)
					}
					continue
				}
			}
			in := botpolicy.Decide(botpolicy.BoardFromGameInto(e.G, e, d.Player, &board), d, rngs[d.Player])
			if err := e.Submit(in); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Fatal("no own-upkeep instant window reached")
	return nil, 0
}

// TestSearchseatHoldUpkeepFallsBackToPass: the bot's answer is a held cast
// (own upkeep or draw, empty stack, an instant from the hand). With the hold
// on, Choose plays pass and its Trace counts HeldUpkeep 1, and the seat
// itself plays pass with HeldUpkeep 1 on the unsearched path; with the hold
// off the bot's cast is played and nothing is counted.
func TestSearchseatHoldUpkeepFallsBackToPass(t *testing.T) {
	e, castAt := holdWindowEngine(t)
	d := e.Pending()
	pass := -1
	for i, o := range d.Options {
		if o.Kind == "pass" {
			pass = i
		}
	}
	if pass < 0 || pass == castAt {
		t.Fatalf("precondition: pass %d, cast %d", pass, castAt)
	}
	castBot := decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{castAt}}
	col := searchprobe.NewCollector(d.Player)

	off := Defaults()
	in, _, tr := Choose(searchprobe.PublicGame{}, searchprobe.History{}, col, e, d, castBot, searchprobe.Frame{}, off)
	if !reflect.DeepEqual(in.Choices, []int{castAt}) || tr.HeldUpkeep != 0 {
		t.Fatalf("off: Choose = %v, HeldUpkeep %d; want the bot's cast %d, 0", in.Choices, tr.HeldUpkeep, castAt)
	}
	on := Defaults()
	on.HoldUpkeep = true
	in, _, tr = Choose(searchprobe.PublicGame{}, searchprobe.History{}, col, e, d, castBot, searchprobe.Frame{}, on)
	if !reflect.DeepEqual(in.Choices, []int{pass}) || tr.HeldUpkeep != 1 {
		t.Fatalf("on: Choose = %v, HeldUpkeep %d; want pass %d, 1", in.Choices, tr.HeldUpkeep, pass)
	}
	ex := on
	ex.HoldUpkeepExempt = []string{"Test Opt"}
	if in, _, tr = Choose(searchprobe.PublicGame{}, searchprobe.History{}, col, e, d, castBot, searchprobe.Frame{}, ex); !reflect.DeepEqual(in.Choices, []int{castAt}) || tr.HeldUpkeep != 0 {
		t.Fatalf("exempt: Choose = %v, HeldUpkeep %d; want the bot's cast", in.Choices, tr.HeldUpkeep)
	}

	// The seat: its own bot's answer here, played through the stopped-feed
	// path that never reaches Choose.
	env := Env{Engine: e, Board: botpolicy.BoardFromGame(e.G, e, d.Player), Feed: &Feed{stopped: true}}
	sbOff := NewSearchBot(7, off)
	botIn, err := sbOff.DecideSearch(context.Background(), env, *d)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(botIn.Choices, []int{castAt}) {
		t.Skipf("the seat's bot answers %v here, not the cast %d: the seat path is not exercised", botIn.Choices, castAt)
	}
	sb := NewSearchBot(7, on)
	got, err := sb.DecideSearch(context.Background(), env, *d)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Choices, []int{pass}) || sb.HeldUpkeep() != 1 || sbOff.HeldUpkeep() != 0 {
		t.Fatalf("seat on = %v, HeldUpkeep %d (off %d); want pass %d, 1 (0)", got.Choices, sb.HeldUpkeep(), sbOff.HeldUpkeep(), pass)
	}
}
