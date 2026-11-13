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

// declareBlock marks blocker b as blocking host w with declaration order bord
// (gorge deals the blockers' hit-backs in that order, not in slot order).
func declareBlock(s *State, b, w Slot, bord uint8) {
	s.perms[b].flags |= pfBlocking
	s.perms[b].blocking = w
	s.perms[b].bord = objWord(bord)
}

// TestHealCombatBlockOrder: the event of the pass that survives is the one
// gorge deals last: the blocker declared last, whatever its slot. The kernel
// lists the blockers' events in slot order, so the surviving damage and the
// deathtouch mark must follow bord. The Hill Giant (3, slot 1) and the Bears
// (2, slot 2) differ in power, and only one of them has deathtouch.
func TestHealCombatBlockOrder(t *testing.T) {
	for _, tc := range []struct {
		name             string
		giantFirst       bool // the Giant is declared before the Bears
		dtGiant, dtBear  bool
		infectGiant      bool
		wantTo           int16
		wantDT, wantSeen bool
	}{
		{"giant declared first", true, false, false, false, 2, false, false},
		{"bears declared first", false, false, false, false, 3, false, false},
		{"bears declared first, deathtouch on the later giant", false, true, false, false, 3, true, false},
		{"giant declared first, deathtouch on the earlier giant", true, true, false, false, 2, false, true},
		{"bears declared first, deathtouch on the earlier bears", false, false, true, false, 3, false, true},
		// infect on an earlier event: its counters stay (infectCombat takes them out of toPerm)
		{"infect giant first", true, true, false, true, 2, false, true},
		// infect on the last event: infectCombat brings toPerm to 0 and marks deathtouch
		{"infect giant last", false, true, false, true, 0, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sc, w, a, b := wolverineScen(t)
			s, sw, sa, sb := sc.s, sc.slot(w), sc.slot(a), sc.slot(b)
			if sa >= sb {
				t.Fatalf("precondition: the Giant's slot %d must precede the Bears' %d", sa, sb)
			}
			if tc.infectGiant {
				sc.k.t.Cards[s.cardAt(sa)].Infect = true
				sc.k.gen.anyInfect = true
			}
			if tc.dtGiant {
				s.perms[sa].eotKw |= kop.KwDeathtouch
			}
			if tc.dtBear {
				s.perms[sb].eotKw |= kop.KwDeathtouch
			}
			if tc.giantFirst {
				declareBlock(s, sa, sw, 1)
				declareBlock(s, sb, sw, 2)
			} else {
				declareBlock(s, sb, sw, 1)
				declareBlock(s, sa, sw, 2)
			}
			var r dmgRec
			for _, e := range []struct {
				src Slot
				n   int16
				dt  bool
			}{{sa, 3, tc.dtGiant}, {sb, 2, tc.dtBear}} { // slot order, as combatDamage adds them
				r.toPerm[sw] += e.n
				r.dealt[e.src] += e.n
				r.note(e.src, sw, e.n)
				if e.dt {
					r.dtPerm.set(sw)
				}
			}
			s.healCombat(sc.k, &r)
			if tc.infectGiant {
				s.infectCombat(sc.k, &r)
			}
			if got := r.toPerm[sw]; got != tc.wantTo {
				t.Fatalf("toPerm %d, want %d", got, tc.wantTo)
			}
			wantDT := tc.wantDT && !tc.infectGiant // infectCombat consumes dtPerm into the mark
			if got := r.dtPerm.has(sw); got != wantDT {
				t.Fatalf("dtPerm %v, want %v", got, wantDT)
			}
			if tc.infectGiant && tc.wantDT && s.perms[sw].flags&pfDeathtouched == 0 {
				t.Fatal("the last event's deathtouch mark was not placed")
			}
			if tc.infectGiant && !tc.wantDT && s.perms[sw].flags&pfDeathtouched != 0 {
				t.Fatal("a Deathtouched mark stays from a healed event")
			}
			if got := s.perms[sw].cseen&cseenDT != 0; got != tc.wantSeen {
				t.Fatalf("cseenDT %v, want %v", got, tc.wantSeen)
			}
			if r.dealt[sa] != 3 || r.dealt[sb] != 2 {
				t.Fatalf("dealt %d/%d, want 3/2", r.dealt[sa], r.dealt[sb])
			}
		})
	}
}
