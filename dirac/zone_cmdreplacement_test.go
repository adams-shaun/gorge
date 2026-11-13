package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// cmdZoneReplScen puts seat 0's commander on the battlefield and starts a
// resolving spell whose single instruction is op (C is its operand): a bounce
// or a tuck. The spell has no object of its own; the target slot names the
// commander.
func cmdZoneReplScen(t *testing.T, op kop.Op, c int16) (*scen, Obj) {
	t.Helper()
	tbl, cmd := commanderTable(0)
	card := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name:  "Test Commander Move",
		Types: kop.TInstant,
		Abilities: []kop.Ability{{Prog: []kop.Instr{{
			Op: op, Obj: kop.RefTarget0, C: c,
		}}}},
	})
	sc := newCommanderScen(t, tbl, cmd, true)
	s := sc.s
	o := s.pl[0].cmd
	s.detach(o)
	sl := s.enter(sc.k, o, 0, false, kop.ZoneCommand)
	s.perms[sl].flags &^= pfSick
	e := stackEntry{kind: seSpell, card: card, ctrl: 0}
	e.nt, e.tgt[0], e.tver[0] = 1, TCode(o), s.objs[o].ver
	s.res = resCtx{e: e, active: true, lastTok: NoObj, eachObj: NoObj}
	s.ctl = ctlResolve
	s.runProg(sc.k)
	return sc, o
}

// assertCmdZoneReplaceAsk: the parked move is asked to the owner before it
// happens, with the command-zone option first and let-it-go second.
func assertCmdZoneReplaceAsk(t *testing.T, sc *scen, o Obj, from kop.Zone) Decision {
	t.Helper()
	if z := sc.s.objs[o].zone; z != from {
		t.Fatalf("the commander is in zone %v before the ask, want %v (the move must be parked, not applied)", z, from)
	}
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChCommanderZone {
		t.Fatalf("decision kind=%d sub=%d, want DkChoose/ChCommanderZone", d.Kind, d.Sub)
	}
	if d.Player != 0 {
		t.Fatalf("asked player %d, want the commander's owner 0", d.Player)
	}
	if d.N != 2 {
		t.Fatalf("%d options, want exactly 2", d.N)
	}
	for i, wantMode := range [2]uint8{0, 1} {
		a := d.Options[i]
		if a.Kind() != AkChoose || a.Source() != TCode(uint8(o)) || a.Mode() != wantMode {
			t.Fatalf("option %d = kind %d src %d mode %d, want AkChoose src %d mode %d", i, a.Kind(), a.Source(), a.Mode(), o, wantMode)
		}
	}
	return d
}

// assertNoRepeatAsk: after the answer the commander is not offered the same
// choice again for that move.
func assertNoRepeatAsk(t *testing.T, sc *scen, o Obj) {
	t.Helper()
	if d := sc.s.Step(); d.Kind == DkChoose && d.Sub == ChCommanderZone {
		t.Fatalf("the commander %d was asked about again without moving", o)
	}
}

func TestCommanderBounceAsk(t *testing.T) {
	for _, mode := range [2]uint8{0, 1} {
		t.Run([2]string{"command zone", "hand"}[mode], func(t *testing.T) {
			sc, o := cmdZoneReplScen(t, kop.OpBounce, 0)
			s := sc.s
			d := assertCmdZoneReplaceAsk(t, sc, o, kop.ZoneBattlefield)
			hand := s.pl[0].handN
			s.Apply(d.Options[mode])
			if s.Stalled() {
				t.Fatalf("the answer stalled: %s", StallName(s.StallReason()))
			}
			if mode == 0 {
				if z := s.objs[o].zone; z != kop.ZoneCommand {
					t.Fatalf("mode 0 left the commander in zone %v, want the command zone", z)
				}
				if s.pl[0].handN != hand {
					t.Fatalf("mode 0 changed the hand %d -> %d", hand, s.pl[0].handN)
				}
			} else {
				if z := s.objs[o].zone; z != kop.ZoneHand {
					t.Fatalf("mode 1 left the commander in zone %v, want the hand", z)
				}
				if s.pl[0].handN != hand+1 {
					t.Fatalf("mode 1 hand count %d, want %d", s.pl[0].handN, hand+1)
				}
			}
			assertNoRepeatAsk(t, sc, o)
		})
	}
}

func TestCommanderTuckAsk(t *testing.T) {
	for _, mode := range [2]uint8{0, 1} {
		t.Run([2]string{"command zone", "library"}[mode], func(t *testing.T) {
			sc, o := cmdZoneReplScen(t, kop.OpToLibrary, 2) // bottom, then shuffle
			s := sc.s
			d := assertCmdZoneReplaceAsk(t, sc, o, kop.ZoneBattlefield)
			s.Apply(d.Options[mode])
			if s.Stalled() {
				t.Fatalf("the answer stalled: %s", StallName(s.StallReason()))
			}
			if mode == 0 {
				if z := s.objs[o].zone; z != kop.ZoneCommand {
					t.Fatalf("mode 0 left the commander in zone %v, want the command zone", z)
				}
			} else {
				if z := s.objs[o].zone; z != kop.ZoneLibrary {
					t.Fatalf("mode 1 left the commander in zone %v, want the library", z)
				}
				found := false
				for i := int(s.pl[0].libLo); i < int(s.pl[0].libHi); i++ {
					if s.pl[0].lib[i] == o {
						found = true
					}
				}
				if !found {
					t.Fatal("mode 1 put the commander in the library zone but not its library list")
				}
			}
			assertNoRepeatAsk(t, sc, o)
		})
	}
}

func TestCommanderGraveNotReplaced(t *testing.T) {
	tbl, cmd := commanderTable(0)
	sc := newCommanderScen(t, tbl, cmd, true)
	s := sc.s
	o := s.pl[0].cmd
	s.detach(o)
	sl := s.enter(sc.k, o, 0, false, kop.ZoneCommand)
	s.perms[sl].flags &^= pfSick
	s.npend = 0
	s.destroy(sc.k, sl)
	if z := s.objs[o].zone; z != kop.ZoneGraveyard {
		t.Fatalf("the destroyed commander is in zone %v, want the graveyard: a graveyard destination is not replaced", z)
	}
	d := s.Step()
	if d.Kind != DkChoose || d.Sub != ChCommanderZone || d.Player != 0 || d.N != 2 {
		t.Fatalf("decision kind=%d sub=%d player=%d n=%d, want the graveyard commander's CR 903.9a ask to its owner", d.Kind, d.Sub, d.Player, d.N)
	}
	if z := s.objs[o].zone; z != kop.ZoneGraveyard {
		t.Fatalf("the commander is in zone %v at the ask, want the graveyard (the move already happened)", z)
	}
	if got := s.pl[0].cmdAsk; got != s.objs[o].ver {
		t.Fatalf("ask watermark %d, want the commander's graveyard ver %d: the ask is not the state-based one", got, s.objs[o].ver)
	}
	s.Apply(d.Options[1])
	if z := s.objs[o].zone; z != kop.ZoneGraveyard {
		t.Fatalf("mode 1 moved the commander to zone %v, want the graveyard", z)
	}
}
