package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestBattlefieldPickCommanderMove: a hidden battlefield pick (OpChooseMove)
// of a commander bound for its owner's hand parks at the CR 903.9b ask. The
// answer must perform the move (mode 0 to the command zone, mode 1 to the
// hand), remember the commander after it moved, and continue the program.
func TestBattlefieldPickCommanderMove(t *testing.T) {
	for _, mode := range [2]uint8{0, 1} {
		t.Run([2]string{"command zone", "hand"}[mode], func(t *testing.T) {
			tbl, cmd := commanderTable(0)
			tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou})
			f := kop.FilterID(len(tbl.Filters) - 1)
			tbl.Cards = append(tbl.Cards, kop.CardDef{
				Name:  "Test Commander Pick",
				Types: kop.TInstant,
				Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{
					{Op: kop.OpChooseMove, Obj: kop.RefController, A: kop.Const(1), F: f,
						C: int16(kop.ZoneHand) | int16(kop.SrMandatory|kop.SrRemember)<<3},
					{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(3)},
				}}},
			})
			card := CardID(len(tbl.Cards) - 1)
			sc := cmdZoneResScen(t, tbl, cmd, card)
			s := sc.s
			o := s.pl[0].cmd
			s.detach(o)
			sl := s.enter(sc.k, o, 0, false, kop.ZoneCommand)
			s.perms[sl].flags &^= pfSick
			if s.objs[o].zone != kop.ZoneBattlefield {
				t.Fatalf("the commander is in zone %v, want the battlefield", s.objs[o].zone)
			}
			life := s.pl[0].life

			s.runProg(sc.k)
			if s.res.ask != raGenSearch {
				t.Fatalf("ask %d, want the hidden pick", s.res.ask)
			}
			d := s.Step()
			if d.Sub != ChGenHiddenPick || d.N != 1 || d.Options[0].Source() != TCode(uint8(o)) {
				t.Fatalf("pick: sub %d with %d options, want ChGenHiddenPick over the commander alone", d.Sub, d.N)
			}
			s.Apply(d.Options[0])
			if s.res.ask != raGenCmdZone {
				t.Fatalf("ask %d after the pick, want raGenCmdZone", s.res.ask)
			}
			if z := s.objs[o].zone; z != kop.ZoneBattlefield {
				t.Fatalf("the parked commander is in zone %v, want the battlefield", z)
			}
			if s.g.nrem != 0 {
				t.Fatalf("remembered %v before the answer, want nothing", s.g.rem[:s.g.nrem])
			}
			d = s.Step()
			if d.Kind != DkChoose || d.Sub != ChCommanderZone {
				t.Fatalf("decision kind=%d sub=%d, want DkChoose/ChCommanderZone", d.Kind, d.Sub)
			}
			s.Apply(d.Options[mode])
			if s.Stalled() {
				t.Fatalf("the answer stalled: %s", StallName(s.StallReason()))
			}
			want := [2]kop.Zone{kop.ZoneCommand, kop.ZoneHand}[mode]
			if z := s.objs[o].zone; z != want {
				t.Fatalf("mode %d left the commander in zone %v, want %v", mode, z, want)
			}
			if s.g.pre.state != 0 {
				t.Fatalf("the answer left park state %d behind", s.g.pre.state)
			}
			if s.g.nrem != 1 || s.g.rem[0] != o {
				t.Fatalf("remembered %v after the answer, want the commander", s.g.rem[:s.g.nrem])
			}
			if got := s.pl[0].life; got != life+3 {
				t.Fatalf("life %d, want %d: the program did not continue after the move", got, life+3)
			}
		})
	}
}
