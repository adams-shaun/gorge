package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// digAllOptionalScen builds a costless sorcery whose body is
// OpDigEx ChangeNum$ All beside Optional$ (gorge effDig's allOrNothing
// election), with two known cards on top of seat 0's library.
func digAllOptionalScen(t *testing.T) (*scen, CardID, [2]Obj) {
	t.Helper()
	tbl := v.NewTable()
	// a non-nil expression table makes prepareGen build the generated
	// extension the OpDigEx op needs
	tbl.Exprs = []kop.Expr{}
	c := kop.CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name:  "Test Dig All Optional",
		Types: kop.TSorcery,
		Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{
			{Op: kop.OpDigEx, Obj: kop.RefController, A: kop.Const(2), B: kop.Const(-1),
				C: int16(kop.ZoneExile) | int16(kop.DigOptional)<<3},
		}}},
	})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	pl := &sc.s.pl[0]
	var top [2]Obj
	for i := range top {
		top[i] = sc.obj(v.Island, 0)
		sc.s.objs[top[i]].zone = kop.ZoneLibrary
		pl.libTop(top[i])
	}
	return sc, c, top
}

// digAllOptionalAsk casts the card and returns the yes/no decision it poses.
func digAllOptionalAsk(t *testing.T, sc *scen, c CardID) *Decision {
	t.Helper()
	o := sc.hand(c, 0)
	sc.do(t, AkCast, TCode(o), scAny)
	sc.do(t, AkPass, scAny, scAny)
	sc.do(t, AkPass, scAny, scAny)
	d := sc.s.Step()
	return &d
}

func digAllOptionalAnswer(t *testing.T, d *Decision, mode uint8) Action {
	t.Helper()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkChoose && a.Mode() == mode {
			return a
		}
	}
	t.Fatalf("no mode-%d choice among %v", mode, d.Options[:d.N])
	return Action(0)
}

// TestDigAllOptionalAsksYesNo: ChangeNum$ All beside Optional$ poses one
// yes/no, never a subset. No leaves the looked-at window in place (the
// whole Dig is declined); yes takes every matching card.
func TestDigAllOptionalAsksYesNo(t *testing.T) {
	for _, c := range []struct {
		name string
		mode uint8
		move bool
	}{{"decline", 0, false}, {"accept", 1, true}} {
		t.Run(c.name, func(t *testing.T) {
			sc, card, top := digAllOptionalScen(t)
			d := digAllOptionalAsk(t, sc, card)
			if d.Kind != DkChoose || d.Sub != ChGenDigOpt || d.N != 2 {
				t.Fatalf("decision kind %d sub %d with %d options, want the yes/no (2)", d.Kind, d.Sub, d.N)
			}
			sc.s.Apply(digAllOptionalAnswer(t, d, c.mode))
			// finish the spell and pass out of its resolution
			for i := 0; i < 4; i++ {
				nd := sc.s.Step()
				if nd.Kind == DkPriority {
					sc.do(t, AkPass, scAny, scAny)
					continue
				}
				break
			}
			for _, o := range top {
				z := sc.s.objs[o].zone
				if c.move && z != kop.ZoneExile {
					t.Fatalf("accepted: card %d in zone %d, want exile", o, z)
				}
				if !c.move && z != kop.ZoneLibrary {
					t.Fatalf("declined: card %d in zone %d, want library", o, z)
				}
			}
			if !c.move {
				pl := &sc.s.pl[0]
				lib := pl.lib[int(pl.libLo) : int(pl.libLo)+pl.libLen()]
				if len(lib) < 2 || lib[0] != top[1] || lib[1] != top[0] {
					t.Fatalf("declined: library top %v, want the window untouched (top %d then %d)", lib[:min(2, len(lib))], top[1], top[0])
				}
			}
		})
	}
}
