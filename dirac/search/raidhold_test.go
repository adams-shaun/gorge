package search

import (
	"math"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Tests for the raid options: Options.RaidHold (raidhold.go) and
// PriorTemp (points.go temper). The RaidHold scenarios are an FDN red
// mirror of Mountains, Gorehorn Raider ({4}{R} 4/4, "Raid -- when this
// creature enters, if you attacked this turn, it deals 2 damage to any
// target") and Fanatical Firebrand (the {R} haste 1/1 attacker), played
// by a script that never casts the Raider to the decision a test needs.

const gorehorn = "Gorehorn Raider"

var raidDeck = []cardset.Entry{{Name: "Mountain", Count: 17}, {Name: gorehorn, Count: 12}, {Name: firebrand, Count: 11}}

// deckRoot is redRoot over the mirror of deck.
func deckRoot(t *testing.T, deck []cardset.Entry, pick func(s *dirac.State, d *dirac.Decision) int, want func(s *dirac.State, d *dirac.Decision) bool) *dirac.State {
	t.Helper()
	set := cardsettest.Set(t, "fdn")
	tbl, a, b, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	s := new(dirac.State)
	var d dirac.Decision
	for seed := uint64(1); seed <= 40; seed++ {
		s.Init(tbl, a, b, seed, dirac.Config{})
		for n := 0; n < 4000; n++ {
			s.StepInto(&d)
			if d.Kind == dirac.DkNone {
				break
			}
			if want(s, &d) {
				return s
			}
			s.Apply(d.Options[pick(s, &d)])
		}
	}
	t.Fatal("no raid-mirror game reached the scenario")
	return s
}

func develop(s *dirac.State, d *dirac.Decision) int { return scriptPick(s, d, true) }

// raidCastable: player p's priority decision offers a Gorehorn Raider cast.
func raidCastable(s *dirac.State, d *dirac.Decision, p uint8) bool {
	return d.Kind == dirac.DkPriority && d.Player == p && offers(s, d, func(a dirac.Action) bool { return castsCard(s, a, gorehorn, false) })
}

// mainOne is seat 0 in its own first main phase after its land drop, the
// stack empty, the Raider castable and a creature able to attack (a
// Firebrand the script cast).
func mainOne(s *dirac.State, d *dirac.Decision) bool {
	return raidCastable(s, d, 0) && s.ActivePlayer() == 0 && s.StepNow() == kop.StepMain1 && s.StackLen() == 0 &&
		!offers(s, d, func(a dirac.Action) bool { return a.Kind() == dirac.AkPlayLand }) && s.HasAttacker()
}

func isRaider(s *dirac.State) func(dirac.Action) bool {
	return func(a dirac.Action) bool { return castsCard(s, a, gorehorn, false) }
}

// TestRaidCardsFDN pins search.RaidTrigger (the default set: the enters
// triggers under a raid condition) and RaidCard (those and the as-enters
// replacement) over the FDN pool, and none of the end-step raid cards.
func TestRaidCardsFDN(t *testing.T) {
	set := cardsettest.Set(t, "fdn")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	triggers := []string{gorehorn, "Gutless Plunderer", "Skyship Buccaneer", "Strongbox Raider"}
	for _, c := range []struct {
		name string
		f    func(*kop.Table, kop.CardID) bool
		want []string
	}{{"RaidTrigger", RaidTrigger, triggers}, {"RaidCard", RaidCard, append([]string{"Goblin Boarders"}, triggers...)}} {
		var got []string
		for i := range tbl.Cards {
			if c.f(tbl, kop.CardID(i)) {
				got = append(got, tbl.Cards[i].Name)
			}
		}
		slices.Sort(got)
		if !slices.Equal(got, c.want) {
			t.Fatalf("%s: %q, want %q", c.name, got, c.want)
		}
		if c.f(tbl, kop.CardID(len(tbl.Cards))) {
			t.Fatalf("%s: a CardID past the table is one", c.name)
		}
	}
}

// TestRaidHoldMainOne: seat 0's first main phase, the Raider castable and a
// Firebrand able to attack. Off, the stand-in bot's answer is the Raider
// (the costliest castable spell) and it is searched as candidate 0. On, no
// candidate casts it, pass stands in as candidate 0 and the rest is
// searched (RaidHeld, RaidHeldBot); with the bot passing the held root is
// the unheld one minus the Raider's casts; Sims 0 plays pass; a
// RaidHoldOnly that does not name it changes nothing, one that does holds
// it.
func TestRaidHoldMainOne(t *testing.T) {
	s := deckRoot(t, raidDeck, develop, mainOne)
	raider := isRaider(s)
	isPass := func(a dirac.Action) bool { return a.Kind() == dirac.AkPass }
	o := timingOpts()
	o.Limit = MaxCands
	off := NewSearcher(o, HeuristicLeaf{}, Heuristic{})
	ro := *off.Search(s, 3)
	if ro.Stats.Searched != 1 || !raider(dirac.Action(off.RootCand(0)[0])) || ro.Stats.RaidHeld != 0 {
		t.Fatalf("off: searched %d, candidate 0 %v, RaidHeld %d; want the Raider searched first", ro.Stats.Searched, dirac.Action(off.RootCand(0)[0]), ro.Stats.RaidHeld)
	}
	offKeys := map[Key]bool{}
	for c := 0; c < ro.N; c++ {
		if !raider(dirac.Action(off.RootCand(c)[0])) {
			offKeys[ro.Keys[c]] = true
		}
	}
	o.RaidHold = true
	on := NewSearcher(o, HeuristicLeaf{}, Heuristic{})
	rh := *on.Search(s, 3)
	if rh.Stats.Searched != 1 || rh.Stats.RaidHeld != 1 || rh.Stats.RaidHeldBot != 1 {
		t.Fatalf("on: searched %d RaidHeld %d RaidHeldBot %d, want 1 1 1", rh.Stats.Searched, rh.Stats.RaidHeld, rh.Stats.RaidHeldBot)
	}
	if !isPass(dirac.Action(on.RootCand(0)[0])) || rootHas(on, &rh, raider) {
		t.Fatalf("on: candidate 0 %v, a Raider cast among the candidates %v; want pass first and no Raider", dirac.Action(on.RootCand(0)[0]), rootHas(on, &rh, raider))
	}
	if rh.N != len(offKeys) || rh.N < 2 {
		t.Fatalf("on: %d candidates, want the unheld root's %d others", rh.N, len(offKeys))
	}
	for c := 0; c < rh.N; c++ {
		if !offKeys[rh.Keys[c]] {
			t.Fatalf("on: candidate %d is none of the unheld root's", c)
		}
	}
	if rh.Choice == 0 && (rh.NActs != 1 || !isPass(rh.Acts[0])) {
		t.Fatal("on: the choice is candidate 0 but the answer is not pass")
	}
	// the bot passing: the held root is the unheld one minus the casts
	pb := NewSearcher(timingOptsLimit(), HeuristicLeaf{}, passBot{})
	po := *pb.Search(s, 3)
	var want []Key
	for c := 0; c < po.N; c++ {
		if !raider(dirac.Action(pb.RootCand(c)[0])) {
			want = append(want, po.Keys[c])
		}
	}
	if len(want) == po.N {
		t.Fatal("off, passing bot: no Raider cast among the candidates")
	}
	oh := timingOptsLimit()
	oh.RaidHold = true
	ph := *NewSearcher(oh, HeuristicLeaf{}, passBot{}).Search(s, 3)
	if ph.Stats.RaidHeld != 1 || ph.Stats.RaidHeldBot != 0 || ph.N != len(want) {
		t.Fatalf("on, passing bot: RaidHeld %d RaidHeldBot %d, %d candidates; want 1 0 %d", ph.Stats.RaidHeld, ph.Stats.RaidHeldBot, ph.N, len(want))
	}
	for c := range want {
		if ph.Keys[c] != want[c] {
			t.Fatalf("on, passing bot: candidate %d differs from the unheld root's", c)
		}
	}
	// Sims 0: no point is built; the bot's Raider is played as pass
	o0 := o
	o0.Sims = 0
	if r := NewSearcher(o0, HeuristicLeaf{}, Heuristic{}).Search(s, 3); r.Stats.Searched != 0 || r.NActs != 1 || !isPass(r.Acts[0]) || r.Stats.RaidHeldBot != 1 {
		t.Fatalf("sims 0: searched %d plays %v RaidHeldBot %d; want pass", r.Stats.Searched, r.Acts[0].Kind(), r.Stats.RaidHeldBot)
	}
	// RaidHoldOnly: another raid card leaves the search the unheld one
	oo := o
	oo.RaidHoldOnly[0] = "Strongbox Raider"
	if err := oo.Validate(); err != nil {
		t.Fatal(err)
	}
	if r := *NewSearcher(oo, HeuristicLeaf{}, Heuristic{}).Search(s, 3); !sameSearch(&r, &ro) || r.Stats.RaidHeld != 0 {
		t.Fatalf("only Strongbox Raider: the search differs from the unheld one (RaidHeld %d)", r.Stats.RaidHeld)
	}
	oo.RaidHoldOnly[1] = gorehorn
	if r := *NewSearcher(oo, HeuristicLeaf{}, Heuristic{}).Search(s, 3); !sameSearch(&r, &rh) {
		t.Fatal("only Strongbox Raider and Gorehorn Raider: the search differs from the held one")
	}
}

func timingOptsLimit() Options {
	o := timingOpts()
	o.Limit = MaxCands
	return o
}

// attackPick is the scenario driver (scriptPick, developing with dev)
// with seat 0 attacking with every creature it can (dirac asks one
// attacker per pick, then Done) and seat 1 never blocking; *attacked
// records the turn of seat 0's attack.
func attackPick(attacked *int, dev bool) func(s *dirac.State, d *dirac.Decision) int {
	return func(s *dirac.State, d *dirac.Decision) int {
		if d.Kind == dirac.DkAttackers && d.Player == 0 {
			for i := 0; i < int(d.N); i++ {
				if a := d.Options[i]; a.Kind() == dirac.AkAttack && a.Target() >= dirac.PlayerTarget0 {
					*attacked = s.Turn()
					return i
				}
			}
		}
		return scriptPick(s, d, dev)
	}
}

// TestRaidHoldWindow: outside the window the root's candidates are the
// unheld ones and RaidHeld stays 0 -- seat 0's first main phase with no
// creature that could attack, and its second main phase after attacking
// or with creatures that could still attack (combat is over) -- and the
// Raider is among them.
func TestRaidHoldWindow(t *testing.T) {
	noAttacker := func(s *dirac.State, d *dirac.Decision) bool {
		return raidCastable(s, d, 0) && s.ActivePlayer() == 0 && s.StepNow() == kop.StepMain1 && s.StackLen() == 0 &&
			!offers(s, d, func(a dirac.Action) bool { return a.Kind() == dirac.AkPlayLand }) && !s.HasAttacker()
	}
	attacked := -1
	afterAttack := func(s *dirac.State, d *dirac.Decision) bool {
		return raidCastable(s, d, 0) && s.ActivePlayer() == 0 && s.StepNow() == kop.StepMain2 && s.StackLen() == 0 && attacked == s.Turn()
	}
	// the script never attacks: its creatures could still attack in main 2,
	// but the turn's combat is over
	noAttack := func(s *dirac.State, d *dirac.Decision) bool {
		return raidCastable(s, d, 0) && s.ActivePlayer() == 0 && s.StepNow() == kop.StepMain2 && s.StackLen() == 0 && s.HasAttacker()
	}
	for _, c := range []struct {
		name string
		pick func(s *dirac.State, d *dirac.Decision) int
		want func(s *dirac.State, d *dirac.Decision) bool
	}{{"main 1, no attacker", lands, noAttacker}, {"main 2 after attacking", attackPick(&attacked, true), afterAttack},
		{"main 2, untapped attackers", develop, noAttack}} {
		s := deckRoot(t, raidDeck, c.pick, c.want)
		o := timingOptsLimit()
		o.Sims = 8
		ro := *NewSearcher(o, HeuristicLeaf{}, Heuristic{}).Search(s, 5)
		o.RaidHold = true
		sr := NewSearcher(o, HeuristicLeaf{}, Heuristic{})
		rh := *sr.Search(s, 5)
		// the root only: the walk may open the window (a haste Firebrand
		// cast in main 1 is an attacker, and then the Raider is held)
		if ro.Stats.Searched != 1 || rh.N != ro.N || rh.Keys != ro.Keys || rh.Prior != ro.Prior || rh.Stats.RaidHeld != 0 || rh.Stats.RaidHeldBot != 0 || !rootHas(sr, &rh, isRaider(s)) {
			t.Fatalf("%s: held root %d candidates (RaidHeld %d), unheld %d (searched %d); want the same root, the Raider offered", c.name, rh.N, rh.Stats.RaidHeld, ro.N, ro.Stats.Searched)
		}
	}
}

// stepTo answers every decision with pick until stop holds (or the game
// ends), returning whether a Gorehorn Raider trigger was built on the way:
// a target decision for the Raider's own ability (its raid trigger, the
// card's only ability that targets).
func stepTo(s *dirac.State, pick func(s *dirac.State, d *dirac.Decision) int, stop func(s *dirac.State, d *dirac.Decision) bool) (fired bool) {
	var d dirac.Decision
	for n := 0; n < 2000; n++ {
		s.StepInto(&d)
		if d.Kind == dirac.DkNone || stop(s, &d) {
			return fired
		}
		if d.Kind == dirac.DkTarget {
			if card, _, _, ok := s.Building(); ok && s.Table().Cards[card].Name == gorehorn {
				fired = true
			}
		}
		s.Apply(d.Options[pick(s, &d)])
	}
	return fired
}

// TestRaidHoldRaidFires is the rule's purpose, played out. With RaidHold
// the first main phase offers no Raider cast; the turn goes on to an
// attack, and in the second main phase the Raider is offered (the window
// shut, RaidHeld 0) and, cast there, its raid trigger fires. Cast in the
// first main phase instead (the unheld root's candidate 0), it enters
// with no trigger, and none follows that turn.
func TestRaidHoldRaidFires(t *testing.T) {
	root := deckRoot(t, raidDeck, develop, mainOne)
	o := timingOptsLimit()
	o.RaidHold = true
	sr := NewSearcher(o, HeuristicLeaf{}, Heuristic{})
	if r := *sr.Search(root, 4); rootHas(sr, &r, isRaider(root)) || r.Stats.RaidHeld != 1 {
		t.Fatalf("main 1: a Raider cast among the held root's candidates (RaidHeld %d)", r.Stats.RaidHeld)
	}
	turn := root.Turn()
	attacked := -1
	var s dirac.State
	s.CopyFrom(root)
	main2 := func(s *dirac.State, d *dirac.Decision) bool {
		return d.Kind == dirac.DkPriority && d.Player == 0 && s.StepNow() == kop.StepMain2 && s.StackLen() == 0 || s.Turn() != turn
	}
	// no casts on the way: the turn's mana stays for the Raider
	if stepTo(&s, attackPick(&attacked, false), main2) || attacked != turn || s.Turn() != turn {
		t.Fatalf("no attack on turn %d before its second main phase (attacked %d, now turn %d)", turn, attacked, s.Turn())
	}
	var d dirac.Decision
	s.StepInto(&d)
	r := *sr.Search(&s, 4)
	if !rootHas(sr, &r, isRaider(&s)) || r.Stats.RaidHeld != 0 {
		t.Fatalf("main 2: the Raider not offered (RaidHeld %d)", r.Stats.RaidHeld)
	}
	cast := -1
	for i := 0; i < int(d.N); i++ {
		if castsCard(&s, d.Options[i], gorehorn, false) {
			cast = i
			break
		}
	}
	if cast < 0 {
		t.Fatal("main 2: no Raider cast option")
	}
	life := s.LifeOf(1)
	s.Apply(d.Options[cast])
	endTurn := func(s *dirac.State, d *dirac.Decision) bool { return s.Turn() != turn }
	if !stepTo(&s, lands, endTurn) {
		t.Fatal("main 2 after attacking: the Raider entered without its raid trigger")
	}
	t.Logf("raid trigger fired; seat 1's life %d -> %d", life, s.LifeOf(1))

	// cast in the first main phase: no trigger that turn
	s.CopyFrom(root)
	s.StepInto(&d)
	for i := 0; i < int(d.N); i++ {
		if castsCard(&s, d.Options[i], gorehorn, false) {
			s.Apply(d.Options[i])
			break
		}
	}
	attacked = -1
	if stepTo(&s, attackPick(&attacked, false), endTurn) || attacked != turn {
		t.Fatalf("main 1: the raid trigger fired on a precombat cast (attacked %d)", attacked)
	}
}

// raidSpy is the heuristic leaf counting the leaves at which seat 0's own
// Gorehorn Raider spell is the bottom of the stack in its first main phase
// while it could attack: a cast the rule holds, played inside a simulation.
type raidSpy struct{ hits *int }

func (sp raidSpy) Value(s *dirac.State, d *dirac.Decision, seat uint8) float32 {
	if s.ActivePlayer() == 0 && s.StepNow() == kop.StepMain1 && s.StackLen() > 0 && s.HasAttacker() {
		if kind, _, card, ctrl := s.StackEntry(0); kind == 0 && ctrl == 0 && s.Table().Cards[card].Name == gorehorn {
			*sp.hits++
		}
	}
	return HeuristicLeaf{}.Value(s, d, seat)
}

func (raidSpy) Priors(_ *dirac.State, d *dirac.Decision, _ []bool, out []float32) {
	for i := 0; i < int(d.N); i++ {
		out[i] = 0
	}
}

// raidNext: player p decides at priority with the stack empty in the
// other player's end step, holding a Gorehorn Raider with at least five
// lands and a creature in play, so its next first main phase opens the
// window.
func raidNext(p uint8) func(s *dirac.State, d *dirac.Decision) bool {
	return func(s *dirac.State, d *dirac.Decision) bool {
		if d.Kind != dirac.DkPriority || d.Player != 0 || s.ActivePlayer() == p || s.StepNow() != kop.StepEnd || s.StackLen() != 0 {
			return false
		}
		var pr dirac.Projection
		s.Project(&pr)
		landsN, creatures, raider := 0, 0, false
		for i := 0; i < int(pr.NObj); i++ {
			o := &pr.Objs[i]
			name := s.Table().Cards[o.Card].Name
			switch {
			case o.Zone == kop.ZoneBattlefield && o.Ctrl == p && s.Table().Cards[o.Card].Types&kop.TLand != 0:
				landsN++
			case o.Zone == kop.ZoneBattlefield && o.Ctrl == p && s.IsCreature(dirac.Obj(i)):
				creatures++
			case o.Zone == kop.ZoneHand && o.Owner == p && name == gorehorn:
				raider = true
			}
		}
		return landsN >= 5 && creatures > 0 && raider
	}
}

// TestRaidHoldWalk: from seat 0's priority in the opponent's end step the
// walk reaches seat 0's own first main phase, where the rule holds the
// Raider (RaidHeldWalk) and the stand-in bot's cast becomes pass, so no
// leaf sees the cast on the stack, where without the rule some do; the
// root is the same and the tree differs; with a walk bot that passes the
// held points still count. From seat 0's own end step a
// mirrored opponent's walk reaches the opponent's first main phase
// (RaidHeldOpp); an unmirrored opponent is never held.
func TestRaidHoldWalk(t *testing.T) {
	s := deckRoot(t, raidDeck, develop, raidNext(0))
	var res [2]Result
	for i, hold := range []bool{false, true} {
		o := timingOpts()
		// the unmirrored opponent's decisions are points (never held), so
		// a leaf sees seat 0's cast on the stack before it resolves
		o.Sims, o.RaidHold, o.OpponentNodes = 300, hold, true
		hits := 0
		res[i] = *NewSearcher(o, raidSpy{&hits}, Heuristic{}).Search(s, 9)
		r := &res[i]
		if hold && (hits != 0 || r.Stats.RaidHeldWalk == 0 || r.Stats.RaidHeldOpp != 0) || !hold && (hits == 0 || r.Stats.RaidHeldWalk != 0) {
			t.Fatalf("hold %v: %d leaves saw seat 0's precombat Raider (RaidHeldWalk %d, RaidHeldOpp %d)", hold, hits, r.Stats.RaidHeldWalk, r.Stats.RaidHeldOpp)
		}
	}
	if res[1].N != res[0].N || res[1].Keys != res[0].Keys || sameSearch(&res[1], &res[0]) || res[1].Stats.RaidHeld != 0 {
		t.Fatal("opponent's end step: the root should be the same (not held) and the tree differ")
	}
	// a walk bot that passes: the held walk decisions are points that
	// dropped the Raider, still counted
	o := timingOpts()
	o.Sims, o.RaidHold = 300, true
	if r := NewSearcher(o, HeuristicLeaf{}, passBot{}).Search(s, 9); r.Stats.RaidHeldWalk == 0 {
		t.Fatal("passing walk bot: RaidHeldWalk 0")
	}

	s = deckRoot(t, raidDeck, develop, raidNext(1))
	for _, c := range []struct {
		nodes, mirror, held bool
	}{{true, true, true}, {true, false, false}, {false, false, false}} {
		o := timingOpts()
		o.Sims, o.RaidHold, o.OpponentNodes, o.OpponentMirror = 300, true, c.nodes, c.mirror
		r := NewSearcher(o, HeuristicLeaf{}, Heuristic{}).Search(s, 9)
		if got := r.Stats.RaidHeldOpp > 0; got != c.held {
			t.Fatalf("own end step, oppnodes %v oppfull %v: RaidHeldOpp %d, want held %v", c.nodes, c.mirror, r.Stats.RaidHeldOpp, c.held)
		}
	}
}

// TestRaidHoldForce: CardSimsForce never adds a held Raider back, though a
// csims= entry names it and the cands= cap would otherwise drop it.
func TestRaidHoldForce(t *testing.T) {
	s := deckRoot(t, raidDeck, develop, mainOne)
	o := timingOpts()
	o.Sims, o.Limit, o.RaidHold, o.CardSimsForce = 16, 2, true, true
	o.CardSims[0] = CardBudget{Name: gorehorn, Sims: 40}
	sr := NewSearcher(o, HeuristicLeaf{}, Heuristic{})
	r := *sr.Search(s, 2)
	if rootHas(sr, &r, isRaider(s)) || r.Stats.CardForced != 0 || r.Sims != 16 {
		t.Fatalf("a held Raider among the candidates (forced %d, sims %d)", r.Stats.CardForced, r.Sims)
	}
	o.RaidHold = false
	sr = NewSearcher(o, HeuristicLeaf{}, Heuristic{})
	if r := *sr.Search(s, 2); !rootHas(sr, &r, isRaider(s)) || r.Sims != 40 {
		t.Fatalf("unheld: the Raider not searched at its csims budget (sims %d)", r.Sims)
	}
}

// TestDoc040OptionsValidate: RaidHoldOnly's list rules and PriorTemp's
// range.
func TestDoc040OptionsValidate(t *testing.T) {
	for _, c := range []struct {
		name string
		set  func(o *Options)
	}{
		{"only without raidhold", func(o *Options) { o.RaidHoldOnly[0] = gorehorn }},
		{"only gap", func(o *Options) { o.RaidHold, o.RaidHoldOnly[1] = true, gorehorn }},
		{"only twice", func(o *Options) { o.RaidHold, o.RaidHoldOnly[0], o.RaidHoldOnly[1] = true, gorehorn, gorehorn }},
		{"ptemp negative", func(o *Options) { o.PriorTemp = -1 }},
		{"ptemp NaN", func(o *Options) { o.PriorTemp = math.NaN() }},
		{"ptemp +Inf", func(o *Options) { o.PriorTemp = math.Inf(1) }},
	} {
		o := DefaultOptions()
		c.set(&o)
		if err := o.Validate(); err == nil {
			t.Errorf("%s: accepted", c.name)
		}
	}
	o := DefaultOptions()
	o.RaidHold, o.PriorTemp = true, 0.5
	o.RaidHoldOnly[0], o.RaidHoldOnly[1] = gorehorn, "Strongbox Raider"
	if err := o.Validate(); err != nil {
		t.Fatal(err)
	}
}

// tempered is p^(1/t) renormalised (the definition PriorTemp implements as
// softmax(logits/t)).
func tempered(p []float64, t float64) []float64 {
	out := make([]float64, len(p))
	sum := 0.0
	for i, x := range p {
		out[i] = math.Pow(x, 1/t)
		sum += out[i]
	}
	for i := range out {
		out[i] /= sum
	}
	return out
}

// TestPriorTemp: under a network prior (fakeNet's spread logits), T 1 is
// the untempered search bit for bit (the tree, so the in-walk points too);
// T 2 makes the root prior P^(1/2) renormalised and flatter (a lower top
// prior), T 0.5 P^2 renormalised and sharper, with and without a topk= cut
// (which keeps the same candidates); the in-walk points are tempered (the
// tree differs beyond the root's prior: a seat point built off the root),
// an opponent's only under OpponentMirror; a uniform prior is untouched.
func TestPriorTemp(t *testing.T) {
	s := redRoot(t, lands, postDrop)
	for _, topk := range []int{0, 3} {
		o := timingOpts()
		o.Sims, o.PriorTopK, o.Limit = 64, topk, MaxCands
		o.OpponentNodes, o.OpponentMirror = true, true
		off := *NewSearcher(o, &fakeNet{val: -1}, Heuristic{}).Search(s, 6)
		if off.Stats.Searched != 1 || off.N < 3 {
			t.Fatalf("topk %d: searched %d with %d candidates", topk, off.Stats.Searched, off.N)
		}
		o1 := o
		o1.PriorTemp = 1
		if r := *NewSearcher(o1, &fakeNet{val: -1}, Heuristic{}).Search(s, 6); !sameSearch(&r, &off) || r.Stats != off.Stats {
			t.Fatalf("topk %d: T 1 differs from off", topk)
		}
		top := func(p []float64) float64 { return slices.Max(p) }
		for _, temp := range []float64{2, 0.5} {
			ot := o
			ot.PriorTemp = temp
			r := *NewSearcher(ot, &fakeNet{val: -1}, Heuristic{}).Search(s, 6)
			if r.N != off.N || r.Keys != off.Keys {
				t.Fatalf("topk %d T %g: the candidates changed", topk, temp)
			}
			want := tempered(off.Prior[:off.N], temp)
			for i := range want {
				if math.Abs(r.Prior[i]-want[i]) > 1e-12 {
					t.Fatalf("topk %d T %g: prior %v, want %v", topk, temp, r.Prior[:r.N], want)
				}
			}
			if flatter := top(r.Prior[:r.N]) < top(off.Prior[:off.N]); flatter != (temp > 1) {
				t.Fatalf("topk %d T %g: top prior %g against %g untempered", topk, temp, top(r.Prior[:r.N]), top(off.Prior[:off.N]))
			}
		}
	}

	// the points off the root: a seat point and an opponent's, built as
	// the walk builds them
	o := timingOpts()
	o.Limit, o.OpponentNodes = MaxCands, true
	for _, c := range []struct {
		player uint8
		mirror bool
		tempd  bool
	}{{0, false, true}, {1, true, true}, {1, false, false}} {
		ps := redRoot(t, lands, func(s *dirac.State, d *dirac.Decision) bool {
			return d.Kind == dirac.DkPriority && d.Player == c.player && offers(s, d, func(a dirac.Action) bool { return castsCard(s, a, burst, false) })
		})
		var d dirac.Decision
		ps.StepInto(&d)
		build := func(temp float64) point {
			oc := o
			oc.OpponentMirror, oc.PriorTemp = c.mirror, temp
			sr := NewSearcher(oc, &fakeNet{val: -1}, Heuristic{})
			sr.seat = 0 // the point is seat 0's or the opponent's, off the root
			var pt point
			if !sr.buildPoint(ps, &d, Heuristic{}.Pick(ps, &d), &pt) {
				t.Fatalf("player %d: no point", c.player)
			}
			return pt
		}
		p1, p2 := build(1), build(2)
		want2 := tempered(p1.prior[:p1.n], 2)
		for i := 0; i < p1.n; i++ {
			exp := p1.prior[i]
			if c.tempd {
				exp = want2[i]
			}
			if math.Abs(p2.prior[i]-exp) > 1e-12 {
				t.Fatalf("player %d mirror %v: prior %v, want %v (tempered %v)", c.player, c.mirror, p2.prior[:p2.n], p1.prior[:p1.n], c.tempd)
			}
		}
	}

	// a uniform prior (no network): T changes nothing
	o = timingOpts()
	u := *NewSearcher(o, HeuristicLeaf{}, Heuristic{}).Search(s, 6)
	o.PriorTemp = 3
	if r := *NewSearcher(o, HeuristicLeaf{}, Heuristic{}).Search(s, 6); !sameSearch(&r, &u) {
		t.Fatal("uniform prior: T 3 changed the search")
	}
}

// TestDoc040ZeroAllocs: a search with RaidHold and PriorTemp on (and the
// opponent mirrored) allocates nothing (a self-play host's hot loop).
func TestDoc040ZeroAllocs(t *testing.T) {
	o := timingOpts()
	o.Sims, o.RaidHold, o.PriorTemp, o.PriorTopK = 32, true, 2, 3
	o.RaidHoldOnly[0] = gorehorn
	o.OpponentNodes, o.OpponentMirror = true, true
	sr := NewSearcher(o, &fakeNet{val: -1}, Heuristic{})
	for _, want := range []func(s *dirac.State, d *dirac.Decision) bool{mainOne, raidNext(0)} {
		s := deckRoot(t, raidDeck, develop, want)
		if allocs := testing.AllocsPerRun(5, func() { sr.Search(s, 1) }); allocs != 0 {
			t.Fatalf("allocs per search = %v", allocs)
		}
	}
}
