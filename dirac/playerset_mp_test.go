//go:build dirac_mp

package dirac

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
)

// setPair is the same n-seat seating in gorge and the kernel, with seat
// gone (if any) out of the game in both. The kernel side is a generated
// table's state (the each-player ops are generated-table ops) whose seat
// count is set to n.
func setPair(t *testing.T, n int, gone int) (*rules.Engine, *State, *prep) {
	t.Helper()
	land := cmdTestCard(t, "Name:Plains\nTypes:Basic Land Plains\nOracle:x\n")
	names := make([]string, n)
	gd := make([][]*cards.Card, n)
	for p := 0; p < n; p++ {
		names[p] = fmt.Sprintf("p%d", p)
		for i := 0; i < 40; i++ {
			gd[p] = append(gd[p], land)
		}
	}
	e := rules.New(rules.Config{Seed: 1, Names: names, Decks: gd})
	sc, _ := newBLBScen(t, []cardset.Entry{{Name: "Plains", Count: 8}})
	s := sc.s
	s.g.nseat = uint8(n)
	for p := 0; p < n; p++ {
		s.pl[p].life = 20
	}
	if gone >= 0 {
		e.G.Players[gone].Lost = true
		s.pl[gone].lost = true
	}
	return e, s, sc.k
}

func seatsOf(q seatSeq) string { return fmt.Sprint(q.s[:q.n]) }

func gorgeSeats(ps []state.PlayerID, skip int) string {
	var o []uint8
	for _, p := range ps {
		if int(p) != skip {
			o = append(o, uint8(p))
		}
	}
	return fmt.Sprint(o)
}

// TestEachOpponentAPNAP: "each opponent" is gorge's Defined$ Opponent,
// AliveFrom(controller) without the controller, at three and four seats
// with and without a seat out of the game, for every expansion (the
// each-player ops' eachPlayers, the life/draw/mill ops' players), and an
// each-opponent DealDamage hits exactly those seats.
func TestEachOpponentAPNAP(t *testing.T) {
	for _, n := range []int{3, 4} {
		for gone := -1; gone < n; gone++ {
			for ctrl := 0; ctrl < n; ctrl++ {
				if ctrl == gone {
					continue
				}
				e, s, k := setPair(t, n, gone)
				s.res.e.ctrl = uint8(ctrl)
				want := gorgeSeats(e.G.AliveFrom(state.PlayerID(ctrl)), ctrl)
				if got := seatsOf(s.eachPlayers(k, kop.RefEachOpponent)); got != want {
					t.Fatalf("n%d gone%d ctrl%d: eachPlayers %s, gorge %s", n, gone, ctrl, got, want)
				}
				if got := seatsOf(s.players(k, kop.RefEachOpponent)); got != want {
					t.Fatalf("n%d gone%d ctrl%d: players %s, gorge %s", n, gone, ctrl, got, want)
				}
				all := gorgeSeats(e.G.AliveFrom(state.PlayerID(ctrl)), -1)
				if got := seatsOf(s.players(k, kop.RefEachPlayer)); got != all {
					t.Fatalf("n%d gone%d ctrl%d: each player %s, gorge %s", n, gone, ctrl, got, all)
				}
				var before [MaxSeats]int16
				for p := 0; p < n; p++ {
					before[p] = s.pl[p].life
				}
				s.exec(k, &kop.Instr{Op: kop.OpDealDamage, Obj: kop.RefEachOpponent, A: kop.Const(3)})
				for p := 0; p < n; p++ {
					hit := p != ctrl && p != gone
					if d := before[p] - s.pl[p].life; (d == 3) != hit || d != 0 && d != 3 {
						t.Fatalf("n%d gone%d ctrl%d: seat %d lost %d life", n, gone, ctrl, p, d)
					}
				}
			}
		}
	}
}

// TestRepeatEachPlayerN: OpRepeatEachPlayer binds its players in gorge's
// orders at three and four seats: AliveFrom(controller) for the C&1 form,
// AliveFrom(active) (APNAP) for C=0, AliveFrom(0) for the seat-order form,
// and AliveFrom(controller) without it for each opponent; the chain
// (g.repNext) hands them out one by one and then runs dry.
func TestRepeatEachPlayerN(t *testing.T) {
	type form struct {
		name string
		obj  kop.Ref
		c    int16
	}
	forms := []form{
		{"ctrlFirst", kop.RefEachPlayer, 1},
		{"apnap", kop.RefEachPlayer, 0},
		{"seatOrder", kop.RefEachPlayer, kop.RepeatEachSeatOrder},
		{"eachOpp", kop.RefEachOpponent, 1},
	}
	for _, n := range []int{3, 4} {
		for gone := -1; gone < n; gone++ {
			for ctrl := 0; ctrl < n; ctrl++ {
				for active := 0; active < n; active++ {
					if ctrl == gone || active == gone {
						continue
					}
					for _, f := range forms {
						e, s, k := setPair(t, n, gone)
						s.active = uint8(active)
						s.res.e.ctrl = uint8(ctrl)
						var want string
						switch f.name {
						case "ctrlFirst":
							want = gorgeSeats(e.G.AliveFrom(state.PlayerID(ctrl)), -1)
						case "apnap":
							want = gorgeSeats(e.G.AliveFrom(state.PlayerID(active)), -1)
						case "seatOrder":
							want = gorgeSeats(e.G.AliveFrom(0), -1)
						case "eachOpp":
							want = gorgeSeats(e.G.AliveFrom(state.PlayerID(ctrl)), ctrl)
						}
						s.exec(k, &kop.Instr{Op: kop.OpRepeatEachPlayer, Obj: f.obj, A: kop.Const(0), C: f.c})
						got := seatSeq{}
						got.add(s.g.eachPl)
						for s.repPending() {
							got.add(s.repPop())
						}
						if s.repPop() != 255 {
							t.Fatalf("the chain did not run dry")
						}
						if seatsOf(got) != want {
							t.Fatalf("n%d gone%d ctrl%d active%d %s: kernel %s, gorge %s", n, gone, ctrl, active, f.name, seatsOf(got), want)
						}
					}
				}
			}
		}
	}
}
