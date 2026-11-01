package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestNoncreatureLeavesCombat: CR 506.4. A permanent that stops being a
// creature (Imprisoned in the Moon here; Sugar Coat in the BLB pairs 128 509
// 546 619 805 1179 1281 1407 1483) is removed from combat by the
// state-based pass, as gorge's removeNoncreatureCombatants: an attacker
// stops attacking and its blockers stay blocking but block nothing; a
// blocker stops blocking and the attacker it blocked stays blocked (509.1h).
func TestNoncreatureLeavesCombat(t *testing.T) {
	deck := []cardset.Entry{{Name: "Imprisoned in the Moon", Count: 1}, {Name: "Eager Trufflesnout", Count: 3}, {Name: "Plains", Count: 10}}
	set := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
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
	if int(v.Plains) >= len(tbl.Cards) {
		t.Skip("table too small for the scenario library")
	}
	for _, onAttacker := range []bool{true, false} {
		sc := newScenTable(t, tbl, Config{NoElide: true})
		s := sc.s
		a := sc.perm(id["Eager Trufflesnout"], 0)
		b1 := sc.perm(id["Eager Trufflesnout"], 1)
		b2 := sc.perm(id["Eager Trufflesnout"], 1)
		as, s1, s2 := sc.slot(a), sc.slot(b1), sc.slot(b2)
		s.perms[as].flags |= pfAttacking | pfBlocked
		s.perms[as].nblock = 2
		s.nAttackers, s.declared = 1, true
		for _, b := range []Slot{s1, s2} {
			s.perms[b].flags |= pfBlocking
			s.perms[b].blocking = as
		}
		moon := sc.perm(id["Imprisoned in the Moon"], 0)
		host := as
		if !onAttacker {
			host = s1
		}
		s.perms[sc.slot(moon)].attached = host
		s.loseAllOnset(sc.k, sc.slot(moon), host)
		s.Step()
		ap, p1, p2 := &s.perms[as], &s.perms[s1], &s.perms[s2]
		if onAttacker {
			if ap.flags&(pfAttacking|pfBlocking) != 0 || s.nAttackers != 0 {
				t.Fatalf("attacker flags %x nAttackers %d, want removed from combat", ap.flags, s.nAttackers)
			}
			for _, p := range []*perm{p1, p2} {
				if p.flags&pfBlocking == 0 || p.blocking != NoSlot {
					t.Fatalf("blocker flags %x blocking %d, want still blocking nothing", p.flags, p.blocking)
				}
			}
		} else {
			if p1.flags&pfBlocking != 0 || p1.blocking != NoSlot {
				t.Fatalf("imprisoned blocker flags %x blocking %d, want removed", p1.flags, p1.blocking)
			}
			if ap.flags&(pfAttacking|pfBlocked) != pfAttacking|pfBlocked || ap.nblock != 1 || s.nAttackers != 1 {
				t.Fatalf("attacker flags %x nblock %d, want attacking, blocked, one blocker", ap.flags, ap.nblock)
			}
			if p2.flags&pfBlocking == 0 || p2.blocking != as {
				t.Fatal("other blocker lost its block")
			}
		}
	}
}

// TestRegenLeavesCombat: CR 701.19b. Regeneration's replacement effect
// removes the permanent from combat (506.4), exactly as the noncreature
// state-based action does: an attacker stops attacking and its blockers
// stay blocking but block nothing (their blocking pointer clears), and a
// blocker stops blocking while its attacker stays blocked with one blocker
// fewer (509.1h).
func TestRegenLeavesCombat(t *testing.T) {
	deck := []cardset.Entry{{Name: "Eager Trufflesnout", Count: 3}, {Name: "Plains", Count: 10}}
	set := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
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
	if int(v.Plains) >= len(tbl.Cards) {
		t.Skip("table too small for the scenario library")
	}
	for _, onAttacker := range []bool{true, false} {
		sc := newScenTable(t, tbl, Config{NoElide: true})
		s := sc.s
		a := sc.perm(id["Eager Trufflesnout"], 0)
		b1 := sc.perm(id["Eager Trufflesnout"], 1)
		b2 := sc.perm(id["Eager Trufflesnout"], 1)
		as, s1, s2 := sc.slot(a), sc.slot(b1), sc.slot(b2)
		s.perms[as].flags |= pfAttacking | pfBlocked
		s.perms[as].nblock = 2
		s.nAttackers, s.declared = 1, true
		for _, b := range []Slot{s1, s2} {
			s.perms[b].flags |= pfBlocking
			s.perms[b].blocking = as
		}
		target := as
		if !onAttacker {
			target = s1
		}
		s.addRegenShield(target)
		s.regenerate(target)
		ap, p1, p2 := &s.perms[as], &s.perms[s1], &s.perms[s2]
		if onAttacker {
			if ap.flags&(pfAttacking|pfBlocking|pfBlocked) != 0 || s.nAttackers != 0 {
				t.Fatalf("attacker flags %x nAttackers %d, want removed from combat", ap.flags, s.nAttackers)
			}
			for _, p := range []*perm{p1, p2} {
				if p.flags&pfBlocking == 0 || p.blocking != NoSlot {
					t.Fatalf("blocker flags %x blocking %d, want still blocking nothing", p.flags, p.blocking)
				}
			}
		} else {
			if p1.flags&pfBlocking != 0 || p1.blocking != NoSlot {
				t.Fatalf("regenerated blocker flags %x blocking %d, want removed", p1.flags, p1.blocking)
			}
			if ap.flags&(pfAttacking|pfBlocked) != pfAttacking|pfBlocked || ap.nblock != 1 || s.nAttackers != 1 {
				t.Fatalf("attacker flags %x nblock %d, want attacking, blocked, one blocker", ap.flags, ap.nblock)
			}
			if p2.flags&pfBlocking == 0 || p2.blocking != as {
				t.Fatal("other blocker lost its block")
			}
		}
	}
}
