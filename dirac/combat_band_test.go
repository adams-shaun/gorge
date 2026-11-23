package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// bandOpt is the ask's option of mode m (the kernel orders a decision's
// options by action word, not by mode).
func bandOpt(t *testing.T, d Decision, m uint8) Action {
	t.Helper()
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Mode() == m {
			return d.Options[i]
		}
	}
	t.Fatalf("no option of mode %d in %v", m, d.Options[:d.N])
	return 0
}

// bandAskScen declares attackers: two of the Test Ram creature, the first n
// of them granted banding, both attacking.
func bandAskScen(t *testing.T, banding int) (*kwBandingScen, [2]Obj) {
	t.Helper()
	f := newKwBandingScen(t)
	var rams [2]Obj
	for i := range rams {
		rams[i] = f.perm(f.ram, 0)
	}
	for i := 0; i < banding; i++ {
		f.s.applyOverrideFl(f.k, f.slot(rams[i]), f.band, f.ram, f.s.ovDurFl(kop.DurPermanent))
	}
	f.passUntil(t, kop.StepAttackers)
	for _, o := range rams {
		f.do(t, AkAttack, TCode(o), scAny)
	}
	f.do(t, AkDone, scAny, scAny)
	return f, rams
}

// TestBandAsk: the CR 702.22c band declaration ask gorge poses after the
// attackers are declared (engine/rules/banding.go): one ask per attacker with
// banding that has a candidate, in declaration order, option 0 declining and
// then each other attacker; a decline hands on to the priority round; an
// accepted candidate joins the leader's band (combat_band_form_test.go).
func TestBandAsk(t *testing.T) {
	t.Run("one leader", func(t *testing.T) {
		f, rams := bandAskScen(t, 1)
		d := f.expect(t, DkChoose, ChBand)
		if d.Player != 0 || d.N != 2 {
			t.Fatalf("ask for player %d with %d options, want player 0 and the decline plus one candidate", d.Player, d.N)
		}
		if a := bandOpt(t, d, 0); a.Source() != TCode(NoObj) {
			t.Fatalf("the decline names source %d, want none", a.Source())
		}
		if a := bandOpt(t, d, 1); a.Source() != TCode(rams[1]) {
			t.Fatalf("the candidate is source %d, want %d", a.Source(), rams[1])
		}
		f.s.Apply(bandOpt(t, d, 0))
		f.expect(t, DkPriority, 0)
	})
	t.Run("two leaders in declaration order", func(t *testing.T) {
		f, rams := bandAskScen(t, 2)
		for i, leader := range rams {
			d := f.expect(t, DkChoose, ChBand)
			if f.s.lastPick != f.slot(leader) || d.N != 2 || bandOpt(t, d, 1).Source() != TCode(rams[1-i]) {
				t.Fatalf("ask %d: leader slot %d (want %d), %d options, candidate %d (want %d)",
					i, f.s.lastPick, f.slot(leader), d.N, bandOpt(t, d, 1).Source(), rams[1-i])
			}
			f.s.Apply(bandOpt(t, d, 0))
		}
		f.expect(t, DkPriority, 0)
	})
	t.Run("no banding attacker asks nothing", func(t *testing.T) {
		f, _ := bandAskScen(t, 0)
		f.expect(t, DkPriority, 0)
	})
	t.Run("a lone banding attacker asks nothing", func(t *testing.T) {
		f := newKwBandingScen(t)
		o := f.perm(f.ram, 0)
		f.s.applyOverrideFl(f.k, f.slot(o), f.band, f.ram, f.s.ovDurFl(kop.DurPermanent))
		f.passUntil(t, kop.StepAttackers)
		f.do(t, AkAttack, TCode(o), scAny)
		f.do(t, AkDone, scAny, scAny)
		f.expect(t, DkPriority, 0)
	})
	t.Run("a candidate joins the band", func(t *testing.T) {
		f, rams := bandAskScen(t, 1)
		d := f.expect(t, DkChoose, ChBand)
		f.s.Apply(bandOpt(t, d, 1))
		if f.s.g.stalled != 0 {
			t.Fatalf("stalled %d (%s)", f.s.g.stalled, StallName(f.s.g.stalled))
		}
		if id := uint8(f.slot(rams[0])) + 1; f.s.bandOf(f.slot(rams[0])) != id || f.s.bandOf(f.slot(rams[1])) != id {
			t.Fatalf("bands %d and %d, want both %d", f.s.bandOf(f.slot(rams[0])), f.s.bandOf(f.slot(rams[1])), id)
		}
		f.expect(t, DkPriority, 0)
	})
}

// bandDivScen is xbScen's table plus one granted-banding override record
// (index 0).
func bandDivScen(t *testing.T) *scen {
	t.Helper()
	tbl := &kop.Table{
		Cards: []kop.CardDef{
			{Name: "Bear", Types: kop.TCreature, Power: 2, Toughness: 2},
			{Name: "Twins", Types: kop.TCreature, Power: 7, Toughness: 7, ExtraBlocks: 1},
			{Name: "Four", Types: kop.TCreature, Power: 4, Toughness: 4},
		},
		Overrides: []kop.Override{{Banding: true, UntilEndCombat: true}},
		Filters:   []kop.Filter{{}},
		Conds:     []kop.Cond{},
	}
	sc := newScenTableLib(t, tbl, xbBear, Config{NoElide: true})
	if sc.k.gen == nil {
		t.Fatal("the table is not a generated one")
	}
	return sc
}

func (sc *scen) grantBanding(o Obj) {
	sc.s.applyOverrideFl(sc.k, sc.slot(o), 0, 0, sc.s.ovDurFl(kop.DurPermanent))
}

// bandDivide plays the attackers' declaration (declining a band ask), the
// blocks (blocker, attacker pairs) and returns the first damage division.
func bandDivide(t *testing.T, sc *scen, atk []Obj, blocks [][2]Obj) Decision {
	t.Helper()
	sc.passUntil(t, kop.StepAttackers)
	for _, a := range atk {
		sc.do(t, AkAttack, TCode(a), scAny)
	}
	sc.do(t, AkDone, scAny, scAny)
	if d := sc.s.Step(); d.Kind == DkChoose && d.Sub == ChBand {
		sc.s.Apply(bandOpt(t, d, 0))
	}
	sc.passUntil(t, kop.StepBlockers)
	for _, b := range blocks {
		sc.do(t, AkBlock, TCode(b[0]), TCode(b[1]))
	}
	sc.do(t, AkDone, scAny, scAny)
	return xbToDivide(t, sc)
}

// TestBandDivider: CR 702.22j and k, who divides combat damage (gorge
// divisionChooser). An attacker blocked by a creature with banding is
// divided by the defending player; a blocker of two attackers one of which
// has banding is divided by the attacking player.
func TestBandDivider(t *testing.T) {
	for _, tc := range []struct {
		name   string
		banded bool
		want   uint8
		run    func(t *testing.T, sc *scen, banded bool) Decision
	}{
		{"blocker with banding: the defender divides", true, 1, func(t *testing.T, sc *scen, banded bool) Decision {
			a, b1, b2 := sc.perm(2, 0), sc.perm(xbBear, 1), sc.perm(xbBear, 1)
			if banded {
				sc.grantBanding(b1)
			}
			return bandDivide(t, sc, []Obj{a}, [][2]Obj{{b1, a}, {b2, a}})
		}},
		{"no banding: the attacker's controller divides", false, 0, func(t *testing.T, sc *scen, banded bool) Decision {
			a, b1, b2 := sc.perm(2, 0), sc.perm(xbBear, 1), sc.perm(xbBear, 1)
			return bandDivide(t, sc, []Obj{a}, [][2]Obj{{b1, a}, {b2, a}})
		}},
		{"blocker of a banding attacker: the attacker divides", true, 0, func(t *testing.T, sc *scen, banded bool) Decision {
			a1, a2, tw := sc.perm(xbBear, 0), sc.perm(xbBear, 0), sc.perm(1, 1)
			if banded {
				sc.grantBanding(a1)
			}
			return bandDivide(t, sc, []Obj{a1, a2}, [][2]Obj{{tw, a1}, {tw, a2}})
		}},
		{"no banding: the blocker's controller divides", false, 1, func(t *testing.T, sc *scen, banded bool) Decision {
			a1, a2, tw := sc.perm(xbBear, 0), sc.perm(xbBear, 0), sc.perm(1, 1)
			return bandDivide(t, sc, []Obj{a1, a2}, [][2]Obj{{tw, a1}, {tw, a2}})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := tc.run(t, bandDivScen(t), tc.banded)
			if d.Player != tc.want {
				t.Fatalf("division asked of player %d, want %d", d.Player, tc.want)
			}
		})
	}
}
