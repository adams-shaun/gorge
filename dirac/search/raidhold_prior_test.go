package search

import (
	"fmt"
	"math"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Tests for a review's findings on RaidHold and PriorTemp: the
// prior's bot mark at a held decision, the window's attacker test at its
// edges (a summoning-sick creature, a defender, a kicked Goblin
// Bushwhacker's haste), the default set against an opt-in replacement raid
// card, and PriorTemp through CardSimsForce's rebuilt prior.

// botNet is fakeNet's spread prior plus policynet's bot residual (Model
// ResidualW: an option the bot's answer contains scores 3 higher), so the
// prior depends on the bot marks the search passes, as the network's does.
// It checks every priority ask against the marks the search makes without
// RaidHold -- the stand-in bot's own answer, alone -- and counts the asks
// whose answer is a Gorehorn Raider cast in RaidHold's window (raidAsks).
type botNet struct {
	fakeNet
	asks, raidAsks int
	bad            string
}

func (b *botNet) Priors(s *dirac.State, d *dirac.Decision, bot []bool, out []float32) {
	b.fakeNet.Priors(s, d, bot, out)
	for i := 0; i < int(d.N) && i < len(bot); i++ {
		if bot[i] {
			out[i] += 3
		}
	}
	if d.Kind != dirac.DkPriority {
		return
	}
	b.asks++
	h := Heuristic{}.Pick(s, d)
	for i := 0; i < int(d.N) && b.bad == ""; i++ {
		if bot[i] != (i == h) {
			b.bad = fmt.Sprintf("seat %d turn %d step %d: option %d (%v) marked %v, the bot's answer is %d (%v)",
				d.Player, s.Turn(), s.StepNow(), i, d.Options[i].Kind(), bot[i], h, d.Options[h].Kind())
		}
	}
	if castsCard(s, d.Options[h], gorehorn, false) && d.Player == s.ActivePlayer() && s.StepNow() == kop.StepMain1 && s.StackLen() == 0 && s.HasAttacker() {
		b.raidAsks++
	}
}

// TestRaidHoldPrior: at a held decision the network is asked as without
// the rule -- the bot's mark on its own answer, the held Raider cast, not
// on the pass standing in for it -- so the held root's prior is the unheld
// root's with the Raider's casts removed and renormalised, and every
// in-walk ask (the seat's, a mirrored opponent's) marks the bot's own
// answer too. Before the fix the mark moved to pass, whose residual then
// raised its prior over the other plays.
func TestRaidHoldPrior(t *testing.T) {
	s := deckRoot(t, raidDeck, develop, mainOne)
	raider := isRaider(s)
	o := timingOptsLimit()
	off := NewSearcher(o, &botNet{fakeNet: fakeNet{val: -1}}, Heuristic{})
	ro := *off.Search(s, 3)
	if ro.Stats.Searched != 1 || !raider(dirac.Action(off.RootCand(0)[0])) {
		t.Fatal("off: the bot's Raider is not candidate 0")
	}
	want := map[Key]float64{}
	sum := 0.0
	for c := 0; c < ro.N; c++ {
		if !raider(dirac.Action(off.RootCand(c)[0])) {
			want[ro.Keys[c]] = ro.Prior[c]
			sum += ro.Prior[c]
		}
	}
	o.RaidHold = true
	net := &botNet{fakeNet: fakeNet{val: -1}}
	on := NewSearcher(o, net, Heuristic{})
	rh := *on.Search(s, 3)
	if rh.Stats.RaidHeldBot != 1 || rh.N != len(want) || rh.N < 2 || dirac.Action(on.RootCand(0)[0]).Kind() != dirac.AkPass {
		t.Fatalf("on: RaidHeldBot %d, %d candidates (want %d), candidate 0 %v", rh.Stats.RaidHeldBot, rh.N, len(want), dirac.Action(on.RootCand(0)[0]).Kind())
	}
	if net.bad != "" {
		t.Fatalf("on: %s", net.bad)
	}
	spread := false
	for c := 0; c < rh.N; c++ {
		p, ok := want[rh.Keys[c]]
		if !ok || math.Abs(rh.Prior[c]-p/sum) > 1e-12 {
			t.Fatalf("on: candidate %d prior %g, want the unheld %g renormalised (%g)", c, rh.Prior[c], p, p/sum)
		}
		spread = spread || math.Abs(rh.Prior[c]-rh.Prior[0]) > 1e-9
	}
	if !spread {
		t.Fatal("on: a flat prior; the comparison shows nothing")
	}
	var d dirac.Decision
	var sc dirac.State
	sc.CopyFrom(s)
	sc.StepInto(&d)
	if rh.RaidBot == 0 || !raider(d.Options[rh.RaidBot-1]) {
		t.Fatalf("on: RaidBot %d, want the bot's Raider cast", rh.RaidBot)
	}
	if ro.RaidBot != 0 {
		t.Fatalf("off: RaidBot %d", ro.RaidBot)
	}

	// the walk: from the opponent's end step to seat 0's first main phase,
	// and from seat 0's own end step to a mirrored opponent's
	for _, c := range []struct {
		active uint8
		mirror bool
	}{{1, false}, {0, true}} {
		ws := deckRoot(t, raidDeck, develop, raidNext(c.active))
		wo := timingOpts()
		wo.Sims, wo.RaidHold, wo.OpponentNodes, wo.OpponentMirror = 300, true, c.mirror, c.mirror
		net := &botNet{fakeNet: fakeNet{val: -1}}
		r := NewSearcher(wo, net, Heuristic{}).Search(ws, 9)
		held := r.Stats.RaidHeldWalk
		if c.mirror {
			held = r.Stats.RaidHeldOpp
		}
		if net.bad != "" || net.raidAsks == 0 || held == 0 {
			t.Fatalf("walk, mirror %v: %d held decisions, %d of %d asks at a held Raider; %s", c.mirror, held, net.raidAsks, net.asks, net.bad)
		}
	}
}

const (
	frenzied    = "Frenzied Goblin"    // {R} 1/1, no haste
	barrier     = "Gleaming Barrier"   // {2} 0/4 defender
	bushwhacker = "Goblin Bushwhacker" // {R} 1/1, kicker {R}: creatures you control get +1/+0 and haste
	boarders    = "Goblin Boarders"    // {2}{R} 3/2, enters with a +1/+1 counter if you attacked this turn
)

// seatBoard is seat p's side of the battlefield: its untapped lands, its
// creatures, and of those the summoning-sick, the hasty and the defenders.
type seatBoard struct{ lands, creatures, sick, haste, defender int }

func boardOf(s *dirac.State, p uint8) seatBoard {
	var pr dirac.Projection
	s.Project(&pr)
	var b seatBoard
	for i := 0; i < int(pr.NObj); i++ {
		o := &pr.Objs[i]
		if o.Zone != kop.ZoneBattlefield || o.Ctrl != p {
			continue
		}
		switch ob := dirac.Obj(i); {
		case s.IsCreature(ob):
			b.creatures++
			if o.Flags&dirac.ProjSick != 0 {
				b.sick++
			}
			if s.HasKeyword(ob, kop.KwHaste) {
				b.haste++
			}
			if s.HasKeyword(ob, kop.KwDefender) {
				b.defender++
			}
		case s.Table().Cards[o.Card].Types&kop.TLand != 0 && o.Flags&dirac.ProjTapped == 0:
			b.lands++
		}
	}
	return b
}

func isLandPlay(a dirac.Action) bool { return a.Kind() == dirac.AkPlayLand }

// castOnce is the scenario driver (lands, pass, Done) that also casts name
// (its kicked mode when kicked) in seat 0's own first main phase after the
// land drop, while seat 0 controls no creature and has need untapped lands:
// seat 0's one creature is name, cast on that turn.
func castOnce(name string, kicked bool, need int) func(s *dirac.State, d *dirac.Decision) int {
	return func(s *dirac.State, d *dirac.Decision) int {
		if d.Kind == dirac.DkPriority && d.Player == 0 && s.ActivePlayer() == 0 && s.StepNow() == kop.StepMain1 && s.StackLen() == 0 && !offers(s, d, isLandPlay) {
			if b := boardOf(s, 0); b.creatures == 0 && b.lands >= need {
				for i := 0; i < int(d.N); i++ {
					if a := d.Options[i]; castsCard(s, a, name, false) && (a.Mode()&1 != 0) == kicked {
						return i
					}
				}
			}
		}
		return lands(s, d)
	}
}

// raidWindowAt: seat 0's own first main phase after the land drop, the
// stack empty and a Gorehorn Raider castable (the window's step test).
func raidWindowAt(s *dirac.State, d *dirac.Decision) bool {
	return raidCastable(s, d, 0) && s.ActivePlayer() == 0 && s.StepNow() == kop.StepMain1 && s.StackLen() == 0 && !offers(s, d, isLandPlay)
}

// TestRaidHoldWindowAttackers: the window opens only when seat 0 controls
// a creature that could attack now. Seat 0's only creature summoning sick
// (a Frenzied Goblin, or an unkicked Goblin Bushwhacker, cast this turn) or
// a defender (a Gleaming Barrier from an earlier turn): the Raider is
// offered and the root is the unheld one. A kicked Bushwhacker cast this
// turn, sick but given haste by its own trigger: the Raider is held.
func TestRaidHoldWindowAttackers(t *testing.T) {
	deck := func(x string) []cardset.Entry {
		return []cardset.Entry{{Name: "Mountain", Count: 17}, {Name: gorehorn, Count: 12}, {Name: x, Count: 11}}
	}
	sickOnly := func(b seatBoard) bool {
		return b.creatures > 0 && b.sick == b.creatures && b.haste == 0 && b.defender == 0
	}
	for _, c := range []struct {
		name  string
		card  string
		kick  bool
		need  int
		board func(b seatBoard) bool
		held  bool
	}{
		{"a summoning-sick Frenzied Goblin", frenzied, false, 6, sickOnly, false},
		{"an unkicked Bushwhacker", bushwhacker, false, 6, sickOnly, false},
		{"a defender", barrier, false, 2, func(b seatBoard) bool { return b.creatures > 0 && b.sick == 0 && b.defender == b.creatures }, false},
		{"a kicked Bushwhacker's haste", bushwhacker, true, 7, func(b seatBoard) bool {
			return b.creatures > 0 && b.sick == b.creatures && b.haste == b.creatures && b.defender == 0
		}, true},
	} {
		s := deckRoot(t, deck(c.card), castOnce(c.card, c.kick, c.need), func(s *dirac.State, d *dirac.Decision) bool {
			return raidWindowAt(s, d) && c.board(boardOf(s, 0))
		})
		if s.HasAttacker() != c.held {
			t.Fatalf("%s: HasAttacker %v", c.name, s.HasAttacker())
		}
		raider := isRaider(s)
		o := timingOptsLimit()
		o.Sims = 8
		off := NewSearcher(o, HeuristicLeaf{}, Heuristic{})
		ro := *off.Search(s, 5)
		o.RaidHold = true
		sr := NewSearcher(o, HeuristicLeaf{}, Heuristic{})
		rh := *sr.Search(s, 5)
		if ro.Stats.Searched != 1 || !rootHas(off, &ro, raider) {
			t.Fatalf("%s: unheld root searched %d, the Raider offered %v", c.name, ro.Stats.Searched, rootHas(off, &ro, raider))
		}
		if c.held {
			// (the bot's answer is the kicked cast of another Bushwhacker
			// in hand, when there is one: Heuristic prefers kicked casts)
			if rh.Stats.RaidHeld != 1 || rootHas(sr, &rh, raider) {
				t.Fatalf("%s: RaidHeld %d, the Raider offered %v; want it held", c.name, rh.Stats.RaidHeld, rootHas(sr, &rh, raider))
			}
			continue
		}
		if rh.N != ro.N || rh.Keys != ro.Keys || rh.Prior != ro.Prior || rh.Stats.RaidHeld != 0 || rh.Stats.RaidHeldBot != 0 || !rootHas(sr, &rh, raider) {
			t.Fatalf("%s: held root %d candidates (RaidHeld %d), unheld %d; want the same root, the Raider offered", c.name, rh.N, rh.Stats.RaidHeld, ro.N)
		}
	}
}

// TestRaidHoldDefaultSet: the default set is the enters-trigger raid cards
// (RaidTrigger); Goblin Boarders, whose raid is an as-enters replacement (a
// +1/+1 counter), is held only when RaidHoldOnly names it.
func TestRaidHoldDefaultSet(t *testing.T) {
	deck := []cardset.Entry{{Name: "Mountain", Count: 17}, {Name: boarders, Count: 12}, {Name: firebrand, Count: 11}}
	s := deckRoot(t, deck, develop, func(s *dirac.State, d *dirac.Decision) bool {
		return d.Kind == dirac.DkPriority && d.Player == 0 && s.ActivePlayer() == 0 && s.StepNow() == kop.StepMain1 && s.StackLen() == 0 &&
			!offers(s, d, isLandPlay) && offers(s, d, func(a dirac.Action) bool { return castsCard(s, a, boarders, false) }) && s.HasAttacker()
	})
	isB := func(a dirac.Action) bool { return castsCard(s, a, boarders, false) }
	o := timingOptsLimit()
	o.Sims = 8
	ro := *NewSearcher(o, HeuristicLeaf{}, Heuristic{}).Search(s, 5)
	o.RaidHold = true
	sr := NewSearcher(o, HeuristicLeaf{}, Heuristic{})
	if r := *sr.Search(s, 5); r.N != ro.N || r.Keys != ro.Keys || r.Stats.RaidHeld != 0 || !rootHas(sr, &r, isB) {
		t.Fatalf("default: RaidHeld %d, Boarders offered %v; want the unheld root", r.Stats.RaidHeld, rootHas(sr, &r, isB))
	}
	o.RaidHoldOnly[0] = boarders
	if err := o.Validate(); err != nil {
		t.Fatal(err)
	}
	sr = NewSearcher(o, HeuristicLeaf{}, Heuristic{})
	if r := *sr.Search(s, 5); r.Stats.RaidHeld != 1 || rootHas(sr, &r, isB) {
		t.Fatalf("named: RaidHeld %d, Boarders offered %v; want it held", r.Stats.RaidHeld, rootHas(sr, &r, isB))
	}
}

// TestPriorTempForce: CardSimsForce rebuilds the root's prior over the cut
// candidates and the forced ones (forceCards); under PriorTemp T that
// prior is the untempered rebuilt one P^(1/T) renormalised, the candidates
// the same.
func TestPriorTempForce(t *testing.T) {
	s := redRoot(t, lands, postDrop)
	o := timingOpts()
	o.Sims, o.PriorTopK, o.CardSimsForce = 20, 2, true
	o.CardSims[0] = CardBudget{Name: burst, Sims: 30}
	off := *NewSearcher(o, cardPrior{}, otherBot{}).Search(s, 2)
	if off.Stats.CardForced < 1 {
		t.Fatalf("CardForced %d: nothing forced past the cut", off.Stats.CardForced)
	}
	for _, temp := range []float64{2, 0.5} {
		ot := o
		ot.PriorTemp = temp
		r := *NewSearcher(ot, cardPrior{}, otherBot{}).Search(s, 2)
		if r.N != off.N || r.Keys != off.Keys || r.Stats.CardForced != off.Stats.CardForced {
			t.Fatalf("T %g: the candidates changed", temp)
		}
		want := tempered(off.Prior[:off.N], temp)
		for i := range want {
			if math.Abs(r.Prior[i]-want[i]) > 1e-12 {
				t.Fatalf("T %g: prior %v, want %v (untempered %v)", temp, r.Prior[:r.N], want, off.Prior[:off.N])
			}
		}
		if r.Prior == off.Prior {
			t.Fatalf("T %g: the prior is the untempered one", temp)
		}
	}
}
