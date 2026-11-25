package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// CheckExileRiderTwoKalitas (CR 616.1, 614.6; Lzeutschler/commander_ai issue
// #2: 58 of 1000 four-seat Wretched Ranks mirrors stopped with "exilerider"):
// a nontoken creature of seat 0 dies with lethal damage while seats 1 and 2
// each control Kalitas, Traitor of Ghet. Both "instead exile that card and
// create a 2/2 black Zombie" replacements apply, so the creature's controller
// picks one (gorge's CR 616.1 order ask over the two sources). The chosen
// Kalitas exiles the card and its controller creates the Zombie; the other no
// longer applies, as the creature no longer dies. The kernel stalled at the
// park, a dying slot recording one rider only. The last case kills the chosen
// Kalitas beside the creature (CR 704.3): the other Kalitas exiles it, and the
// chosen one's rider still runs for its last controller (CR 608.2h).
func CheckExileRiderTwoKalitas(t *testing.T, tbl *kop.Table, id map[string]CardID) {
	t.Helper()
	for _, n := range []string{"Swamp", "Kalitas, Traitor of Ghet", "Grizzly Bears"} {
		if _, ok := id[n]; !ok {
			t.Fatalf("precondition: the table lacks %s", n)
		}
	}
	zombies := func(s *State, p uint8) int {
		n := 0
		for m := s.ctrlM[p].and(s.occ); m.any(); m = m.rest() {
			if o := s.perms[m.first()].obj; s.objs[o].of&ofToken != 0 {
				n++
			}
		}
		return n
	}
	for _, tc := range []struct {
		name       string
		pick       int  // the seat whose Kalitas is chosen
		killChosen bool // the chosen Kalitas dies in the same pass
	}{
		{"seat 2's Kalitas", 2, false},
		{"seat 1's Kalitas", 1, false},
		{"the chosen Kalitas dies beside the creature", 2, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var decks [MaxSeats][]CardID
			for p := 0; p < 4; p++ {
				for i := 0; i < 20; i++ {
					decks[p] = append(decks[p], id["Swamp"])
				}
			}
			s := new(State)
			s.InitN(tbl, decks, 4, 1, Config{NoElide: true})
			s.turn, s.active, s.step, s.prio, s.passes, s.ctl = 2, 0, kop.StepMain1, 0, 0, ctlPriority
			s.g.exBase = s.active
			sc := &scen{s: s, k: s.k()}
			k := sc.k
			bear := sc.perm(id["Grizzly Bears"], 0)
			kal := [3]Obj{NoObj, sc.perm(id["Kalitas, Traitor of Ghet"], 1), sc.perm(id["Kalitas, Traitor of Ghet"], 2)}
			s.perms[sc.slot(bear)].damage = 2
			if tc.killChosen {
				s.perms[sc.slot(kal[tc.pick])].damage = 4
			}
			s.sba(k)
			if s.Stalled() {
				t.Fatalf("the two-Kalitas death stalled: %s", StallName(s.StallReason()))
			}
			if s.ctl != ctlExileOrder || !s.parkedWaiting() {
				t.Fatalf("ctl %d parkedWaiting %v, want the CR 616.1 order ask", s.ctl, s.parkedWaiting())
			}
			for s.parkedWaiting() && s.cold.parked[0].obj != bear {
				// the chosen Kalitas's own death asks first (seats 0 and 1 are its
				// opponents): seat 0 has no Kalitas, so one source applies -- its
				// ask, if any, takes the first option
				var d Decision
				s.exileOrderOptions(k, &d)
				s.applyExileOrder(k, d.Options[0])
			}
			var d Decision
			s.exileOrderOptions(k, &d)
			if d.Kind != DkChoose || d.Sub != ChGenReplOrder || d.Player != 0 || d.N != 2 {
				t.Fatalf("decision kind %d sub %d player %d n %d, want seat 0's DkChoose/ChGenReplOrder over the two Kalitas", d.Kind, d.Sub, d.Player, d.N)
			}
			var ans Action
			for i := 0; i < int(d.N); i++ {
				if Obj(d.Options[i].Source()) == kal[tc.pick] {
					ans = d.Options[i]
				}
			}
			if ans == 0 {
				t.Fatalf("no answer names seat %d's Kalitas: %v", tc.pick, d.Options[:d.N])
			}
			s.applyExileOrder(k, ans)
			if s.Stalled() {
				t.Fatalf("the answered death stalled: %s", StallName(s.StallReason()))
			}
			if z := s.objs[bear].zone; z != kop.ZoneExile {
				t.Fatalf("the bear is in zone %d, want exile", z)
			}
			other := 3 - tc.pick
			if got := zombies(s, uint8(tc.pick)); got != 1 {
				t.Errorf("seat %d (the chosen Kalitas) has %d Zombie tokens, want 1", tc.pick, got)
			}
			want := 0
			if tc.killChosen {
				want = 1 // the other Kalitas exiled the chosen one as it died (seat 2 is its opponent)
			}
			if got := zombies(s, uint8(other)); got != want {
				t.Errorf("seat %d (the Kalitas not chosen) has %d Zombie tokens, want %d", other, got, want)
			}
		})
	}
}
