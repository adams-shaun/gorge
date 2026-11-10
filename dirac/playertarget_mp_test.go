//go:build dirac_mp

package dirac

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
)

// burnDeck is 40 cards: Mountains with a Shock at every third position.
func burnDeck() []CardID {
	d := make([]CardID, 40)
	for i := range d {
		d[i] = v.Mountain
		if i%3 == 0 {
			d[i] = v.Shock
		}
	}
	return d
}

// A target event: the chooser, the offered player targets in order, the
// pick, and every seat's life once the spell has resolved.
type tgtEv struct {
	player int
	offers string
	pick   int
	life   string
}

func (e tgtEv) String() string {
	return fmt.Sprintf("p%d offers[%s] pick%d life[%s]", e.player, e.offers, e.pick, e.life)
}

func gorgeBurnGame(t *testing.T, n int, seed uint64, limit int) []tgtEv {
	t.Helper()
	land := cmdTestCard(t, "Name:Mountain\nTypes:Basic Land Mountain\nOracle:x\n")
	shock := cmdTestCard(t, "Name:Shock\nManaCost:R\nTypes:Instant\nA:SP$ DealDamage | NumDmg$ 2 | ValidTgts$ Any | SpellDescription$ Two to anything.\nOracle:x\n")
	names := make([]string, n)
	decks := make([][]*cards.Card, n)
	for p := range decks {
		names[p] = fmt.Sprintf("p%d", p)
		for _, c := range burnDeck() {
			if c == v.Mountain {
				decks[p] = append(decks[p], land)
			} else {
				decks[p] = append(decks[p], shock)
			}
		}
	}
	e := rules.New(rules.Config{Seed: seed, Names: names, Decks: decks})
	var out []tgtEv
	pending := -1
	for len(out) < limit && !e.G.Over {
		e.Advance()
		d := e.Pending()
		if d == nil {
			break
		}
		if pending >= 0 && len(e.G.Stack) == 0 {
			var l []string
			for _, pl := range e.G.Players {
				l = append(l, fmt.Sprint(pl.Life))
			}
			out[pending].life = strings.Join(l, ",")
			pending = -1
		}
		var ch []int
		switch d.Kind {
		case decision.KPriority:
			main := d.Player == e.G.Active && len(e.G.Stack) == 0 && (e.G.Step == state.StepMain1 || e.G.Step == state.StepMain2)
			pick := -1
			for _, want := range []string{"play_land", "cast", "pass"} {
				for _, o := range d.Options {
					if o.Kind == want && pick < 0 && (want != "cast" || main) {
						pick = o.Index
					}
				}
			}
			if d.Options[pick].Kind == "pass" && main {
				for _, o := range d.Options {
					if o.Kind == "activate" {
						pick = o.Index
						break
					}
				}
			}
			ch = []int{pick}
		case decision.KTarget:
			var offers []string
			for _, o := range d.Options {
				if o.Obj == 0 {
					offers = append(offers, fmt.Sprintf("P%d", o.Player))
				} else {
					offers = append(offers, "obj")
				}
			}
			ev := tgtEv{player: int(d.Player), offers: strings.Join(offers, ","), pick: int(e.G.Turn) % len(d.Options)}
			out = append(out, ev)
			pending = len(out) - 1
			ch = []int{ev.pick}
		case decision.KAttackers:
			ch = nil
		default:
			for i := 0; i < d.Min; i++ {
				ch = append(ch, i)
			}
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: ch}); err != nil {
			t.Fatalf("gorge submit %v: %v", d.Kind, err)
		}
	}
	return out
}

func kernelBurnGame(t *testing.T, n int, seed uint64, limit int) []tgtEv {
	t.Helper()
	var decks [MaxSeats][]CardID
	for p := 0; p < n; p++ {
		decks[p] = burnDeck()
	}
	s := new(State)
	s.InitN(v.Table, decks, uint8(n), seed, Config{NoElide: true})
	var out []tgtEv
	pending := -1
	var d Decision
	for steps := 0; len(out) < limit && steps < 100000; steps++ {
		s.StepOne(&d)
		if d.Kind == DkNone {
			break
		}
		if pending >= 0 && s.sp == 0 {
			var l []string
			for p := 0; p < n; p++ {
				l = append(l, fmt.Sprint(s.pl[p].life))
			}
			out[pending].life = strings.Join(l, ",")
			pending = -1
		}
		a := d.Options[0]
		switch d.Kind {
		case DkPriority:
			main := d.Player == s.active && s.sp == 0 && (s.step == kop.StepMain1 || s.step == kop.StepMain2)
			r := map[ActionKind]int{AkPlayLand: 0, AkCast: 1, AkPass: 2}
			best := -1
			for i := 0; i < int(d.N); i++ {
				k, ok := r[d.Options[i].Kind()]
				if ok && (k != 1 || main) && (best < 0 || k < r[d.Options[best].Kind()]) {
					best = i
				}
			}
			a = d.Options[best]
		case DkTarget:
			var offers []string
			for i := 0; i < int(d.N); i++ {
				c := d.Options[i].Target()
				if isPlayerTarget(c) {
					offers = append(offers, fmt.Sprintf("P%d", CodeSeat(c)))
				} else {
					offers = append(offers, "obj")
				}
			}
			ev := tgtEv{player: int(d.Player), offers: strings.Join(offers, ","), pick: int(s.turn) % int(d.N)}
			out = append(out, ev)
			pending = len(out) - 1
			a = d.Options[ev.pick]
		}
		s.Apply(a)
	}
	return out
}

// TestNSeatPlayerTargets: a Shock offers every seat as a player target, in
// gorge's order (seat order, AliveFrom(0)), and the damage lands on the
// picked seat, at three and four seats.
func TestNSeatPlayerTargets(t *testing.T) {
	for _, n := range []int{3, 4} {
		for seed := uint64(1); seed <= 4; seed++ {
			t.Run(fmt.Sprintf("n%d/seed%d", n, seed), func(t *testing.T) {
				want := gorgeBurnGame(t, n, seed, 12)
				got := kernelBurnGame(t, n, seed, 12)
				if len(want) < 4 {
					t.Fatalf("gorge reached only %d target decisions", len(want))
				}
				for i := range want {
					if i >= len(got) || got[i] != want[i] {
						g := "<none>"
						if i < len(got) {
							g = got[i].String()
						}
						t.Fatalf("target event %d: kernel %s, gorge %s", i, g, want[i])
					}
				}
				if len(got) != len(want) {
					t.Fatalf("kernel %d target events, gorge %d", len(got), len(want))
				}
			})
		}
	}
}
