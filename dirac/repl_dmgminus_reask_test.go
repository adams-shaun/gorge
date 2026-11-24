package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestReplDmgMinusCombatReask: n minus lines on one combat hit to a player
// pose gorge's no-op CR 616.1 order ask, which re-asks while two or more
// lines remain: a minus candidate stays live while its line still matches
// (replCandLive), so three lines ask twice and two ask once.
func TestReplDmgMinusCombatReask(t *testing.T) {
	for _, lines := range []int{2, 3, 4} {
		t.Run(string(rune('0'+lines))+" lines", func(t *testing.T) {
			tb := newSPTable()
			var hosts []CardID
			for i := 0; i < lines; i++ {
				hosts = append(hosts, tb.host("Test Minus "+string(rune('A'+i)),
					kop.Repl{Kind: kop.ReplDamageMinus, A: 1, C: kop.ReplDmgPlayer}))
			}
			sc := tb.scen(t)
			for _, h := range hosts {
				sc.perm(h, 1)
			}
			giant := sc.perm(v.HillGiant, 0)
			s := sc.s
			p := &s.perms[sc.slot(giant)]
			p.flags |= pfAttacking
			p.atk = TCode(PlayerTarget1)
			s.active, s.nAttackers = 0, 1
			if !s.combatDamage(sc.k, false) {
				t.Fatal("no ask posed: the walk dealt the hit without the minus lines' order ask")
			}
			ro := &s.g.ro
			if int(ro.n) != lines {
				t.Fatalf("ask has %d candidates, want %d", ro.n, lines)
			}
			// every unpicked line counts toward the re-ask while it matches
			for pick := 1; pick < lines; pick++ {
				var d Decision
				s.replOrderOptions(&d)
				again := s.replOrderPick(d.Options[0])
				if want := lines-pick >= 2; again != want {
					t.Fatalf("after pick %d of %d lines: asks again %v, want %v", pick, lines, again, want)
				}
				if !again {
					break
				}
			}
		})
	}
}

// TestReplDmgMinusCombatBesidePlus: on a combat hit to a player two minus
// lines pose the order ask without stalling, while a minus line beside a plus-N
// addition stalls (StallReplPlus): the two do not commute on a small amount,
// and the kernel would guess gorge's order (minusBesidePlus).
func TestReplDmgMinusCombatBesidePlus(t *testing.T) {
	for _, c := range []struct {
		name  string
		hosts string // m: a minus line (the damaged player's), p: a plus-N addition (the attacker's)
		stall bool
	}{
		{"two minus ask", "mm", false},
		{"minus beside plus-N", "mp", true},
		{"plus-N beside minus", "pm", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			tb := newSPTable()
			fid := tb.filter(kop.Filter{Ctrl: kop.CtrlYou})
			var hosts []CardID
			for i, h := range c.hosts {
				r := kop.Repl{Kind: kop.ReplDamageMinus, A: 1, C: kop.ReplDmgPlayer}
				if h == 'p' {
					r = kop.Repl{Kind: kop.ReplDamagePlus, A: 1, C: int16(kop.ReplDmgOppPlayer | kop.ReplDmgOppPerm), Filter: fid}
				}
				hosts = append(hosts, tb.host("Test Host "+string(rune('A'+i)), r))
			}
			sc := tb.scen(t)
			for i, h := range hosts {
				seat := uint8(1)
				if c.hosts[i] == 'p' {
					seat = 0
				}
				sc.perm(h, seat)
			}
			giant := sc.perm(v.HillGiant, 0)
			s := sc.s
			p := &s.perms[sc.slot(giant)]
			p.flags |= pfAttacking
			p.atk = TCode(PlayerTarget1)
			s.active, s.nAttackers = 0, 1
			asked := s.combatDamage(sc.k, false)
			stalled := s.StallReason() == StallReplPlus || s.g.stalled == StallReplPlus
			if stalled != c.stall {
				t.Fatalf("stalled %v (reason %d, g.stalled %d), want %v", stalled, s.StallReason(), s.g.stalled, c.stall)
			}
			if !c.stall && !asked {
				t.Fatal("no order ask posed")
			}
		})
	}
}
