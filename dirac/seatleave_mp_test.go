//go:build dirac_mp

package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// Leaving the game at more than two seats (CR 800.4a, 104.2a, 104.4a). The
// lockstep test plays the combat games of seatcombat_mp_test.go on to the
// end in gorge and the kernel and compares them event by event, the losses
// and the result included; the kernel tests below force a loss where the
// policy games do not reach one.

// TestNSeatLeaveGameLockstep: seats lose in combat, the others play on
// without them, and the last seat left wins, as in gorge.
func TestNSeatLeaveGameLockstep(t *testing.T) {
	combatPlayOut = true
	defer func() { combatPlayOut = false }()
	playedOn, finished := 0, 0
	for _, n := range []int{3, 4} {
		for seed := uint64(1); seed <= 4; seed++ {
			evs := compareCombatGames(t, n, seed, 6000)
			var gone [MaxSeats]bool
			nGone := 0
			for i, e := range evs {
				switch e.kind {
				case "out":
					gone[e.player], nGone = true, nGone+1
					if nGone < n-1 && i+1 < len(evs) && evs[i+1].kind != "out" && evs[i+1].kind != "over" {
						playedOn++
					}
				case "over":
					if e.player >= 0 && nGone == n-1 && !gone[e.player] {
						finished++
					}
				default:
					if e.player >= 0 && gone[e.player] {
						t.Fatalf("n=%d seed %d: departed seat %d asked: %v", n, seed, e.player, e)
					}
				}
			}
		}
	}
	if playedOn == 0 || finished == 0 {
		t.Fatalf("played on after a loss %d times, won by the last seat %d times", playedOn, finished)
	}
}

// leaveGame4 plays a four-seat vanilla game (lands, then the dearest
// spell, no attacks) until stop holds at a priority decision.
func leaveGame4(t *testing.T, seed uint64, stop func(s *State) bool) (*State, *Decision) {
	t.Helper()
	var decks [MaxSeats][]CardID
	for p := range decks {
		decks[p] = combatDeck()
	}
	s := new(State)
	s.InitN(v.Table, decks, 4, seed, Config{NoElide: true})
	d := new(Decision)
	for i := 0; i < 20000; i++ {
		s.StepOne(d)
		if d.Kind == DkNone {
			t.Fatalf("game ended before the stop")
		}
		if d.Kind == DkPriority && stop(s) {
			return s, d
		}
		a := d.Options[0]
		for j := 0; j < int(d.N); j++ {
			switch x := d.Options[j]; x.Kind() {
			case AkKeep, AkPlayLand, AkCast:
				a = x
			case AkDone:
				if d.Kind == DkAttackers {
					a = x
				}
			}
		}
		s.Apply(a)
	}
	t.Fatalf("no stop")
	return nil, nil
}

// creaturesOf counts seat p's permanents.
func permsOf(s *State, p uint8) int {
	c := 0
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		c++
	}
	return c
}

// TestNSeatLeaveGameSweep: a non-active seat at 0 life leaves; everything it
// owned ceases, the others go on, and it never gets priority or a turn.
func TestNSeatLeaveGameSweep(t *testing.T) {
	s, d := leaveGame4(t, 3, func(s *State) bool { return s.turn >= 12 && s.active != 2 && permsOf(s, 2) > 0 })
	s.pl[2].life = 0
	passStep(t, s, d)
	if !s.pl[2].lost || s.over {
		t.Fatalf("seat 2 lost=%v over=%v", s.pl[2].lost, s.over)
	}
	for i := 0; i < int(s.nobj); i++ {
		if ob := &s.objs[i]; ob.owner() == 2 && ob.zone != kop.ZoneNone {
			t.Fatalf("seat 2's object %d still in zone %d", i, ob.zone)
		}
	}
	if s.ctrlM[2].any() {
		t.Fatalf("seat 2 still controls permanents")
	}
	turns := map[uint8]bool{}
	for i := 0; i < 3000 && d.Kind != DkNone && s.turn < 40; i++ {
		if d.Player == 2 || s.active == 2 {
			t.Fatalf("departed seat 2 asked (%v) or active (turn %d)", d.Kind, s.turn)
		}
		turns[s.active] = true
		s.Apply(d.Options[0])
		s.StepOne(d)
	}
	if !turns[0] || !turns[1] || !turns[3] {
		t.Fatalf("live seats' turns: %v", turns)
	}
}

// TestNSeatLeaveGameActive: the active player leaves in its main phase; the
// rest of its turn passes priority over it and the next turn is the next
// live seat's.
func TestNSeatLeaveGameActive(t *testing.T) {
	s, d := leaveGame4(t, 5, func(s *State) bool {
		return s.turn >= 6 && s.step == kop.StepMain1 && d0(s)
	})
	a := s.active
	s.pl[a].life = 0
	passStep(t, s, d)
	if !s.pl[a].lost || s.over || s.active != a {
		t.Fatalf("lost=%v over=%v active=%d", s.pl[a].lost, s.over, s.active)
	}
	for d.Kind != DkNone && s.active == a {
		if d.Player == a {
			t.Fatalf("departed active seat %d asked %v", a, d.Kind)
		}
		s.Apply(d.Options[0])
		s.StepOne(d)
	}
	if want := s.nextAlive(a); s.active != want {
		t.Fatalf("next turn seat %d, want %d", s.active, want)
	}
}

// passStep passes priority and steps to the next decision: the state-based
// check before it sees the life totals a test set.
func passStep(t *testing.T, s *State, d *Decision) {
	t.Helper()
	for j := 0; j < int(d.N); j++ {
		if d.Options[j].Kind() == AkPass {
			s.Apply(d.Options[j])
			s.StepOne(d)
			return
		}
	}
	t.Fatalf("no pass in %v", d.Kind)
}

func d0(s *State) bool { return s.prio == s.active && s.sp == 0 }

// TestNSeatLeaveGameCombat: a defending player leaves while attacked; the
// creatures attacking it are removed from combat, the others stay.
func TestNSeatLeaveGameCombat(t *testing.T) {
	var s *State
	var d *Decision
	var def uint8
	for seed := uint64(1); seed <= 16 && s == nil; seed++ {
		var decks [MaxSeats][]CardID
		for p := range decks {
			decks[p] = combatDeck()
		}
		st := new(State)
		st.InitN(v.Table, decks, 4, seed, Config{NoElide: true})
		dd := new(Decision)
		var plan []Action
		inDecl := false
		for i := 0; i < 20000 && s == nil; i++ {
			st.StepOne(dd)
			if dd.Kind == DkNone {
				break
			}
			if dd.Kind == DkPriority && st.step == kop.StepBlockers && st.nAttackers >= 2 {
				// attackers at two defenders: the lower one leaves
				var at [MaxSeats]int
				for m := st.ctrlM[st.active]; m.any(); m = m.rest() {
					if a := m.first(); st.perms[a].flags&pfAttacking != 0 {
						at[st.atkSeat(st.k(), a)]++
					}
				}
				hit := 0
				for q := range at {
					if at[q] > 0 {
						hit++
					}
				}
				if hit >= 2 {
					s, d = st, dd
					for q := range at {
						if at[q] > 0 {
							def = uint8(q)
							break
						}
					}
					break
				}
			}
			a := dd.Options[0]
			switch dd.Kind {
			case DkAttackers:
				if !inDecl {
					// every attacker, alternately at the two opponents after
					// the active player
					inDecl, plan = true, plan[:0]
					seen := map[uint8]bool{}
					o1, o2 := st.nextAlive(st.active), st.nextAlive(st.nextAlive(st.active))
					for j := 0; j < int(dd.N); j++ {
						x := dd.Options[j]
						if x.Kind() != AkAttack || seen[x.Source()] {
							continue
						}
						seen[x.Source()] = true
						to := o1
						if len(plan)%2 == 1 {
							to = o2
						}
						plan = append(plan, MakeAction(AkAttack, x.Source(), PlayerCode(to), 0))
					}
				}
				a = MakeAction(AkDone, 0, 0, 0)
				if len(plan) > 0 {
					a, plan = plan[0], plan[1:]
				}
			case DkBlockers:
				a = MakeAction(AkDone, 0, 0, 0)
			default:
				inDecl = false
				for j := 0; j < int(dd.N); j++ {
					switch x := dd.Options[j]; x.Kind() {
					case AkKeep, AkPlayLand, AkCast:
						a = x
					}
				}
			}
			st.Apply(a)
		}
	}
	if s == nil {
		t.Fatalf("no seed attacked two defenders")
	}
	before := s.nAttackers
	var atDef int
	for m := s.ctrlM[s.active]; m.any(); m = m.rest() {
		if a := m.first(); s.perms[a].flags&pfAttacking != 0 && s.atkSeat(s.k(), a) == def {
			atDef++
		}
	}
	s.pl[def].life = 0
	passStep(t, s, d)
	if !s.pl[def].lost || s.over {
		t.Fatalf("defender %d lost=%v over=%v", def, s.pl[def].lost, s.over)
	}
	if int(s.nAttackers) != int(before)-atDef || s.nAttackers == 0 {
		t.Fatalf("attackers %d -> %d, %d were attacking seat %d", before, s.nAttackers, atDef, def)
	}
	for m := s.ctrlM[s.active]; m.any(); m = m.rest() {
		if a := m.first(); s.perms[a].flags&pfAttacking != 0 && s.atkSeat(s.k(), a) == def {
			t.Fatalf("an attacker still attacks departed seat %d", def)
		}
	}
}

// TestNSeatLeaveGameResult: losses one after another leave a winner; two
// last seats losing together draw (CR 104.4a).
func TestNSeatLeaveGameResult(t *testing.T) {
	s, d := leaveGame4(t, 7, func(s *State) bool { return s.turn >= 4 })
	s.pl[1].life = 0
	s.pl[3].life = -2
	passStep(t, s, d)
	if !s.pl[1].lost || !s.pl[3].lost || s.over {
		t.Fatalf("two of four lost: over=%v", s.over)
	}
	passStep(t, s, d)
	s.pl[0].life = 0
	passStep(t, s, d)
	if over, w := s.Terminal(); !over || w != 2 {
		t.Fatalf("last seat: over=%v winner=%d, want seat 2", over, w)
	}

	s, d = leaveGame4(t, 7, func(s *State) bool { return s.turn >= 4 })
	s.pl[1].life = 0
	passStep(t, s, d)
	passStep(t, s, d)
	s.pl[0].life, s.pl[2].life, s.pl[3].life = 0, -1, 0
	passStep(t, s, d)
	if over, w := s.Terminal(); !over || w != -1 {
		t.Fatalf("all lost together: over=%v winner=%d, want a draw", over, w)
	}
}
