package azmcts

import (
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/searchprobe"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
)

func handHolds(g *state.Game, p state.PlayerID, c *cards.Card) bool {
	for _, id := range g.Zone(state.ZHand, p) {
		if o := g.Obj(id); o != nil && o.Card == c {
			return true
		}
	}
	return false
}

// upkeepWindow plays an Island and instant deck (seat 0) into an Island and
// bear deck with the plain bot until a priority decision of the active
// player in its own upkeep or draw step, stack empty, offers a cast of an
// instant from its hand (it taps an Island there first: the manual
// vocabulary offers the cast once its mana floats). It returns the engine there and that option's
// index.
func upkeepWindow(t *testing.T) (*rules.Engine, int) {
	t.Helper()
	opt := payWindowCard(t, "Name:Test Opt\nManaCost:U\nTypes:Instant\nA:SP$ Draw | NumCards$ 1 | SpellDescription:Draw a card.\nOracle:Draw a card.\n")
	bear := payWindowCard(t, "Name:Bear\nManaCost:1\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	island := payWindowCard(t, "Name:Island\nTypes:Basic Land Island\nOracle:x\n")
	var d0, d1 []*cards.Card
	for i := 0; i < 17; i++ {
		d0, d1 = append(d0, island), append(d1, island)
	}
	for i := 0; i < 8; i++ {
		d0, d1 = append(d0, opt), append(d1, bear)
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
				// The manual vocabulary offers a cast once its mana floats:
				// tap an Island (a mana ability, the stack stays empty).
				if tap >= 0 && handHolds(e.G, d.Player, opt) {
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

func passIndex(d *decision.Decision) int {
	for i, o := range d.Options {
		if o.Kind == "pass" {
			return i
		}
	}
	return -1
}

// castsOption reports whether any candidate plays option i.
func castsOption(cands []cand, i int) bool {
	for _, c := range cands {
		if len(c.in.Choices) == 1 && c.in.Choices[0] == i {
			return true
		}
	}
	return false
}

// TestAzmctsHoldUpkeepOffIsIdentical: with the hold off -- no scratch, a
// scratch without it, and the default Options' Search -- the candidates are
// byte-equal to the enumeration without any scratch, over the existing
// candidate fixtures and the upkeep window, where the held cast IS still a
// candidate.
func TestAzmctsHoldUpkeepOffIsIdentical(t *testing.T) {
	if DefaultOptions().HoldUpkeep {
		t.Fatal("HoldUpkeep is on by default")
	}
	e, castAt := upkeepWindow(t)
	d := e.Pending()
	bot := decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{passIndex(d)}}
	type fixture struct {
		e   *rules.Engine
		d   *decision.Decision
		bot decision.Intent
	}
	ability := passOrAbility(5)
	target := playerTarget(4, 2)
	for _, f := range []fixture{
		{nil, ability, decision.Intent{Seq: 5, Player: 0, Choices: []int{1}}},
		{nil, target, decision.Intent{Seq: 4, Player: 0, Choices: []int{1}}},
		{e, d, bot},
	} {
		want, _, _, wok := enumerateWhy(searchprobe.NewCollector(f.d.Player), f.e, f.d, f.bot, AllKinds(), 6)
		var off boardScratch
		got, _, _, gok := enumerateWhyInto(searchprobe.NewCollector(f.d.Player), f.e, f.d, f.bot, AllKinds(), 6, &off)
		if gok != wok || !reflect.DeepEqual(got, want) {
			t.Fatalf("seq %d: candidates with an unarmed scratch %v, want %v", f.d.Seq, choicesOf(got), choicesOf(want))
		}
	}
	want, _, _, ok := enumerateWhy(searchprobe.NewCollector(d.Player), e, d, bot, AllKinds(), 6)
	if !ok || !castsOption(want, castAt) {
		t.Fatalf("precondition: the upkeep window's candidates %v do not cast option %d", choicesOf(want), castAt)
	}
}

// TestAzmctsHoldUpkeepDropsCast: with the hold armed, in the own-upkeep
// window with an instant in hand, no candidate casts it and the pass
// remains; a bot answer that casts it is replaced by pass, and the root
// counts Stats.HeldUpkeep.
func TestAzmctsHoldUpkeepDropsCast(t *testing.T) {
	e, castAt := upkeepWindow(t)
	d := e.Pending()
	pass := passIndex(d)
	if pass < 0 {
		t.Fatal("precondition: the window offers no pass")
	}
	bot := decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{pass}}
	off, _, _, _ := enumerateWhy(searchprobe.NewCollector(d.Player), e, d, bot, AllKinds(), 6)
	if !castsOption(off, castAt) {
		t.Fatalf("precondition: off, candidates %v do not cast option %d", choicesOf(off), castAt)
	}
	on := boardScratch{hold: &upkeepHold{}}
	cands, _, why, ok := enumerateWhyInto(searchprobe.NewCollector(d.Player), e, d, bot, AllKinds(), 6, &on)
	if castsOption(cands, castAt) {
		t.Fatalf("on: candidates %v still cast option %d", choicesOf(cands), castAt)
	}
	if ok && !castsOption(cands, pass) {
		t.Fatalf("on: candidates %v lost the pass", choicesOf(cands))
	}
	if !ok && why != SkipFewCandidates {
		t.Fatalf("on: not searched for %v, want few-candidates", why)
	}
	if on.hold.drops == 0 {
		t.Fatal("on: no drop counted")
	}
	// An exempt name is never held.
	ex := boardScratch{hold: &upkeepHold{exempt: []string{"Test Opt"}}}
	if c, _, _, _ := enumerateWhyInto(searchprobe.NewCollector(d.Player), e, d, bot, AllKinds(), 6, &ex); !castsOption(c, castAt) {
		t.Fatalf("exempt: candidates %v do not cast option %d", choicesOf(c), castAt)
	}
	// The bot's held cast is played as pass, through Search.
	castBot := decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{castAt}}
	opts := DefaultOptions()
	opts.Sims = 0
	opts.HoldUpkeep = true
	res, err := Search(t.Context(), Root{Engine: e, Decision: d, Bot: castBot, Observer: searchprobe.NewCollector(d.Player)}, nil, nil, opts)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(res.Intent.Choices, []int{pass}) || res.Stats.HeldUpkeep != 1 {
		t.Fatalf("Search with a held bot cast = %v, HeldUpkeep %d; want pass %d, 1", res.Intent.Choices, res.Stats.HeldUpkeep, pass)
	}
	opts.HoldUpkeep = false
	res, err = Search(t.Context(), Root{Engine: e, Decision: d, Bot: castBot, Observer: searchprobe.NewCollector(d.Player)}, nil, nil, opts)
	if err != nil || !reflect.DeepEqual(res.Intent.Choices, []int{castAt}) || res.Stats.HeldUpkeep != 0 {
		t.Fatalf("Search off = %v, HeldUpkeep %d, %v; want the bot's cast", res.Intent.Choices, res.Stats.HeldUpkeep, err)
	}
	// The auto-payment vocabulary drops the held card's payment cast too.
	obj := d.Options[castAt].Obj
	paysFor := func(cands []cand) bool {
		for _, c := range cands {
			if c.in.Payment != nil {
				for _, a := range e.EnsurePaymentActions() {
					if a.ID == c.in.Payment.ActionID && a.Cast.Object == obj {
						return true
					}
				}
			}
		}
		return false
	}
	payOff, _, _, _, _ := enumerateCutInto(searchprobe.NewCollector(d.Player), e, d, bot, AllKinds(), 6, true, &boardScratch{})
	if !paysFor(payOff) {
		t.Fatalf("precondition: off, the auto-pay candidates %d do not pay for the instant", len(payOff))
	}
	payOn, _, _, _, _ := enumerateCutInto(searchprobe.NewCollector(d.Player), e, d, bot, AllKinds(), 6, true, &boardScratch{hold: &upkeepHold{}})
	if paysFor(payOn) || castsOption(payOn, castAt) {
		t.Fatalf("on: the auto-pay candidates (%d) still cast the instant", len(payOn))
	}
}
