//go:build dirac_mp

package dirac

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
)

// A seat-round event: who was asked what, on whose turn, over how deep a
// stack. The N-seat tests play the same game in gorge and the kernel and
// compare these, decision by decision.
type seatEv struct {
	kind                     string // "priority", "mulligan", "bottom"
	turn, active, player, sp int
}

func (e seatEv) String() string {
	return fmt.Sprintf("%s t%d a%d p%d sp%d", e.kind, e.turn, e.active, e.player, e.sp)
}

// seatDeck is a 40-card deck of Forests, with a Grizzly Bears at every
// bears-th position (none when bears is 0).
func seatDeck(bears int) []CardID {
	d := make([]CardID, 40)
	for i := range d {
		d[i] = v.Forest
		if bears > 0 && i%bears == 0 {
			d[i] = v.GrizzlyBears
		}
	}
	return d
}

// seatPolicy plays both engines alike: play a land, else cast, else pass;
// declare no attackers; mulligan a seat's first hand when mullSeat says so.
type seatPolicy struct {
	mullSeat func(p, mulls int) bool
}

func gorgeSeatGame(t *testing.T, n int, seed uint64, bears, mulls int, pol seatPolicy, limit int) []seatEv {
	t.Helper()
	land := cmdTestCard(t, "Name:Forest\nTypes:Basic Land Forest\nOracle:x\n")
	bear := cmdTestCard(t, "Name:Grizzly Bears\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	names := make([]string, n)
	decks := make([][]*cards.Card, n)
	for p := range decks {
		names[p] = fmt.Sprintf("p%d", p)
		for _, c := range seatDeck(bears) {
			if c == v.Forest {
				decks[p] = append(decks[p], land)
			} else {
				decks[p] = append(decks[p], bear)
			}
		}
	}
	e := rules.New(rules.Config{Seed: seed, Names: names, Decks: decks, Mulligans: mulls})
	var out []seatEv
	taken := make([]int, n)
	for len(out) < limit && !e.G.Over {
		e.Advance()
		d := e.Pending()
		if d == nil || d.Kind == decision.KAttackers || (e.G.Step == state.StepBeginCombat && gorgeHasCreature(e)) {
			break // combat across defenders is not in these tests
		}
		ev := seatEv{turn: int(e.G.Turn), active: int(e.G.Active), player: int(d.Player), sp: len(e.G.Stack)}
		var ch []int
		switch d.Kind {
		case decision.KPriority:
			ev.kind = "priority"
			pick := -1
			for _, want := range []string{"play_land", "cast", "pass"} {
				for _, o := range d.Options {
					if o.Kind == want && pick < 0 {
						pick = o.Index
					}
				}
			}
			if d.Options[pick].Kind == "pass" && d.Player == e.G.Active && len(e.G.Stack) == 0 &&
				(e.G.Step == state.StepMain1 || e.G.Step == state.StepMain2) {
				// gorge casts from a floated pool: tap the lands first, as
				// the kernel's cast pays them; a mana activation is no
				// kernel decision
				for _, o := range d.Options {
					if o.Kind == "activate" {
						pick, ev.kind = o.Index, ""
						break
					}
				}
			}
			ch = []int{pick}
		case decision.KMulligan:
			ev.kind = "mulligan"
			ch = []int{0}
			if d.Options[0].Kind == "bottom" {
				ev.kind = "bottom"
				ch = nil
				for i := 0; i < d.Min; i++ {
					ch = append(ch, i)
				}
			} else if pol.mullSeat != nil && len(d.Options) > 1 && pol.mullSeat(int(d.Player), taken[d.Player]) {
				ch = []int{1}
				taken[d.Player]++
			}
		default:
			if len(d.Options) > 0 && d.Options[0].Kind == "bottom" {
				ev.kind = "bottom"
			}
			for i := 0; i < d.Min; i++ {
				ch = append(ch, i)
			}
		}
		if ev.kind != "" {
			out = append(out, ev)
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: ch}); err != nil {
			t.Fatalf("gorge submit %v: %v", d.Kind, err)
		}
	}
	return out
}

// gorgeHasCreature reports whether gorge's active player controls a creature.
func gorgeHasCreature(e *rules.Engine) bool {
	for _, id := range e.G.Zone(state.ZBattlefield, e.G.Active) {
		if o := e.G.Obj(id); o != nil && o.Controller == e.G.Active && o.Card != nil && o.Card.Faces[0].Name == "Grizzly Bears" {
			return true
		}
	}
	return false
}

func kernelSeatGame(t *testing.T, n int, seed uint64, bears, mulls int, pol seatPolicy, limit int) []seatEv {
	t.Helper()
	var decks [MaxSeats][]CardID
	for p := 0; p < n; p++ {
		decks[p] = seatDeck(bears)
	}
	s := new(State)
	s.InitN(v.Table, decks, uint8(n), seed, Config{NoElide: true, Mulligans: uint8(mulls)})
	var out []seatEv
	var bottomed [MaxSeats]bool
	taken := make([]int, n)
	var d Decision
	for len(out) < limit {
		s.StepOne(&d)
		if d.Kind == DkNone || d.Kind == DkAttackers || (s.step == kop.StepBeginCombat && s.creaturesCtrl(s.k(), s.active) > 0) {
			break
		}
		ev := seatEv{turn: int(s.turn), active: int(s.active), player: int(d.Player), sp: int(s.sp)}
		a := d.Options[0]
		switch d.Kind {
		case DkPriority:
			ev.kind = "priority"
			best := -1
			for i := 0; i < int(d.N); i++ {
				r := map[ActionKind]int{AkPlayLand: 0, AkCast: 1, AkPass: 2}
				if k, ok := r[d.Options[i].Kind()]; ok && (best < 0 || k < r[d.Options[best].Kind()]) {
					best = i
				}
			}
			a = d.Options[best]
		case DkMulligan:
			ev.kind = "mulligan"
			for i := 0; i < int(d.N); i++ {
				if d.Options[i].Kind() == AkKeep {
					a = d.Options[i]
				}
			}
			if pol.mullSeat != nil && pol.mullSeat(int(d.Player), taken[d.Player]) {
				for i := 0; i < int(d.N); i++ {
					if d.Options[i].Kind() == AkMulligan {
						a = d.Options[i]
						taken[d.Player]++
					}
				}
			}
		default:
			if s.ctl == ctlMullBottom {
				// gorge poses a seat's bottoming as one decision; the
				// kernel picks card by card
				if !bottomed[d.Player] {
					ev.kind = "bottom"
					bottomed[d.Player] = true
				}
			}
		}
		if ev.kind != "" {
			out = append(out, ev)
		}
		s.Apply(a)
	}
	return out
}

func compareSeatGames(t *testing.T, n int, seed uint64, bears, mulls int, pol seatPolicy, limit int) []seatEv {
	t.Helper()
	g := gorgeSeatGame(t, n, seed, bears, mulls, pol, limit)
	k := kernelSeatGame(t, n, seed, bears, mulls, pol, limit)
	for i := 0; i < len(g) && i < len(k); i++ {
		if g[i] != k[i] {
			lo := max(0, i-4)
			t.Fatalf("n=%d seed %d: event %d: gorge %v, kernel %v\n gorge  %v\n kernel %v", n, seed, i, g[i], k[i], g[lo:i+1], k[lo:i+1])
		}
	}
	if len(g) != len(k) {
		t.Fatalf("n=%d seed %d: gorge %d events, kernel %d", n, seed, len(g), len(k))
	}
	return g
}

// TestNSeatTurnRotation: turns pass in seat order from the starting player
// (CR 102.1, 500.1), and each step's priority opens with the active player,
// as gorge plays the same lands-only game.
func TestNSeatTurnRotation(t *testing.T) {
	for _, n := range []int{2, 3, 4} {
		for seed := uint64(1); seed <= 6; seed++ {
			evs := compareSeatGames(t, n, seed, 0, 0, seatPolicy{}, 600)
			// the kernel matches gorge; check gorge's own rotation too
			last, turns := -1, 0
			for _, e := range evs {
				if e.turn != last {
					if last >= 0 && e.active != (evs[0].active+turns)%n {
						t.Fatalf("n=%d seed %d: turn %d active %d", n, seed, e.turn, e.active)
					}
					last = e.turn
					turns++
				}
			}
			if turns < 2*n {
				t.Fatalf("n=%d seed %d: only %d turns", n, seed, turns)
			}
		}
	}
}

// TestNSeatPriorityRound: a spell on the stack passes priority round every
// seat in turn order and resolves only when all of them have passed in
// succession (CR 117.3d, 117.4), as gorge does.
func TestNSeatPriorityRound(t *testing.T) {
	for _, n := range []int{2, 3, 4} {
		for seed := uint64(1); seed <= 6; seed++ {
			evs := compareSeatGames(t, n, seed, 3, 0, seatPolicy{}, 800)
			rounds := 0
			for i, e := range evs {
				if e.sp == 1 && e.player == e.active && i+n <= len(evs) {
					for j := 1; j < n; j++ {
						if q := evs[i+j]; q.sp != 1 || q.player != (e.active+j)%n {
							break
						} else if j == n-1 {
							rounds++
						}
					}
				}
			}
			if rounds == 0 {
				t.Fatalf("n=%d seed %d: no full priority round over a spell", n, seed)
			}
		}
	}
}

// TestNSeatMulliganOrder: the London mulligan round asks every seat in turn
// order from the starting player, redraws the seats that mulliganed, asks
// those again, and bottoms in the same order (CR 103.5), as gorge does.
func TestNSeatMulliganOrder(t *testing.T) {
	pol := seatPolicy{mullSeat: func(p, mulls int) bool { return p%2 == 1 && mulls < p }}
	for _, n := range []int{2, 3, 4} {
		for seed := uint64(1); seed <= 8; seed++ {
			evs := compareSeatGames(t, n, seed, 0, 3, pol, 40)
			var asked []int
			for _, e := range evs {
				if e.kind == "mulligan" {
					asked = append(asked, e.player)
				}
			}
			if len(asked) < n+1 {
				t.Fatalf("n=%d seed %d: mulligan asks %v", n, seed, asked)
			}
			for i := 0; i < n; i++ {
				if asked[i] != (asked[0]+i)%n {
					t.Fatalf("n=%d seed %d: first pass %v", n, seed, asked)
				}
			}
		}
	}
}
