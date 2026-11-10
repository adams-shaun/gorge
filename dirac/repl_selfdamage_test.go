package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// wolverineTable is vanilla's table with a 3/5 whose own DamageDone
// replacement heals the damage already marked on it first (Wolverine, Fierce
// Fighter's ReplDamageHealFirst).
func wolverineTable() (*kop.Table, CardID) {
	tbl := *v.Table
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Exprs = []kop.Expr{{}} // prepareGen only runs a table with generated side tables
	host := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Wolverine", Types: kop.TCreature, Power: 3, Toughness: 5,
		Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplDamageHealFirst}}}})
	return &tbl, host
}

// wolverineScen: the heal host on seat 0, a Hill Giant (power 3) and a
// Grizzly Bears (power 2) on seat 1.
func wolverineScen(t *testing.T) (sc *scen, w, a, b Obj) {
	t.Helper()
	tbl, host := wolverineTable()
	sc = newScenTable(t, tbl, Config{NoElide: true, AutoDivide: true})
	w = sc.perm(host, 0)
	a = sc.perm(v.HillGiant, 1)
	b = sc.perm(v.GrizzlyBears, 1)
	if sc.k.gen == nil || sc.k.gen.anyRepl&rbHealSelf == 0 {
		t.Fatal("the table has no heal-self replacement")
	}
	if !sc.s.selfDamageHost(sc.k, sc.slot(w), rbHealSelf) {
		t.Fatal("the host is not a heal host")
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
