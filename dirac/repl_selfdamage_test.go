package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

const wolverineName = "Wolverine, Fierce Fighter"

// wolverineScen is a scenario on the std-msh pool: Wolverine on seat 0 and
// two creatures on seat 1 (the first with power 3, the second with power 2).
func wolverineScen(t *testing.T) (sc *scen, w, a, b Obj) {
	t.Helper()
	deck := []cardset.Entry{{Name: wolverineName, Count: 1}, {Name: "Atlantean Cavalry", Count: 1}, {Name: "Agent of Atlas", Count: 1}, {Name: "Plains", Count: 8}}
	set := cardsettest.Synth(t, "std-msh")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not supported in the std-msh table", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc = newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true, AutoDivide: true})
	w = sc.perm(id[wolverineName], 0)
	a = sc.perm(id["Atlantean Cavalry"], 1)
	b = sc.perm(id["Agent of Atlas"], 1)
	if sc.k.gen == nil || sc.k.gen.anyRepl&rbHealSelf == 0 {
		t.Fatal("the table has no heal-self replacement")
	}
	if !sc.s.selfDamageHost(sc.k, sc.slot(w), rbHealSelf) {
		t.Fatal("Wolverine is not a heal host")
	}
	return
}

// TestHealCombatTwoBlockers: two combat events to Wolverine in one pass heal
// each other (gorge deals the pass event by event): only the last event's
// damage and deathtouch mark stay, every event is still dealt (lifelink,
// damage triggers).
func TestHealCombatTwoBlockers(t *testing.T) {
	for _, tc := range []struct {
		name         string
		dtA, dtB     bool
		wantDT       bool // r.dtPerm set after healCombat
		wantSeenDTOn bool // the healed first mark is listed at 0
	}{
		{"plain", false, false, false, false},
		{"first has deathtouch", true, false, false, true},
		{"last has deathtouch", false, true, true, false},
		{"both have deathtouch", true, true, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sc, w, a, b := wolverineScen(t)
			s, sw, sa, sb := sc.s, sc.slot(w), sc.slot(a), sc.slot(b)
			if tc.dtA {
				s.perms[sa].eotKw |= kop.KwDeathtouch
			}
			if tc.dtB {
				s.perms[sb].eotKw |= kop.KwDeathtouch
			}
			var r dmgRec
			add := func(src Slot, n int16, dt bool) {
				r.toPerm[sw] += n
				r.dealt[src] += n
				r.note(src, sw, n)
				if dt {
					r.dtPerm.set(sw)
				}
			}
			add(sa, 2, tc.dtA)
			add(sb, 3, tc.dtB)
			if r.toPerm[sw] != 5 || r.nev != 2 {
				t.Fatalf("precondition: toPerm %d, %d events", r.toPerm[sw], r.nev)
			}
			if s.perms[sw].cseen&cseenDT != 0 {
				t.Fatal("precondition: the deathtouch mark is already listed")
			}
			s.healCombat(sc.k, &r)
			if r.toPerm[sw] != 3 || r.dealt[sa] != 2 || r.dealt[sb] != 3 || r.ev[0].n != 2 || r.ev[1].n != 3 {
				t.Fatalf("toPerm %d dealt %d/%d ev %d/%d, want 3 2/3 2/3", r.toPerm[sw], r.dealt[sa], r.dealt[sb], r.ev[0].n, r.ev[1].n)
			}
			if got := r.dtPerm.has(sw); got != tc.wantDT {
				t.Fatalf("dtPerm %v, want %v", got, tc.wantDT)
			}
			if got := s.perms[sw].cseen&cseenDT != 0; got != tc.wantSeenDTOn {
				t.Fatalf("cseenDT %v, want %v", got, tc.wantSeenDTOn)
			}
			if got := s.dtEvents(sc.k, &r, sw); got != 1 {
				t.Fatalf("dtEvents %d, want 1", got)
			}
		})
	}
}

// TestCombatDamageWolverineDoubleBlockNoStall: Wolverine attacks, is blocked
// by two creatures and takes both blockers' damage in one pass: the kernel
// does not stall and only the last assignment's damage stays marked.
func TestCombatDamageWolverineDoubleBlockNoStall(t *testing.T) {
	sc, w, a, b := wolverineScen(t)
	s, sw := sc.s, sc.slot(w)
	if tg := s.typesAt(sc.k, sw); tg&kop.TCreature == 0 {
		t.Fatal("Wolverine is not a creature")
	}
	pa, pb := s.power(sc.k, sc.slot(a)), s.power(sc.k, sc.slot(b))
	if pa == pb || pa < 1 || pb < 1 {
		t.Fatalf("precondition: blockers' power %d/%d must differ", pa, pb)
	}
	// a tough Wolverine survives both: the kernel's damage mark is read back
	s.perms[sw].eotKw |= kop.KwIndestructible
	sc.combat(t, []Obj{w}, [][2]Obj{{a, w}, {b, w}})
	if st := s.g.stalled; st != 0 {
		t.Fatalf("stalled (%s)", StallName(st))
	}
	if !sc.onBF(w) {
		t.Fatal("Wolverine left the battlefield")
	}
	want := pb // the second blocker's event is the last of the pass
	if got := int(s.perms[sw].damage); got != want {
		t.Fatalf("marked damage %d, want %d (the last blocker's power; the first dealt %d)", got, want, pa)
	}
}
