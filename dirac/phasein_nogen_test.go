package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestPhaseUntapWithoutGen: a hand-written table can enable phasing (an
// ExtPhases program) without a generated prep (k.gen == nil); the untap step's
// phase-in must still bring the permanent back, and there is no override-derived
// type state to refresh (CR 702.26a).
func TestPhaseUntapWithoutGen(t *testing.T) {
	tbl := *v.Table
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name: "Test Phasing Source", Types: kop.TCreature, Power: 1, Toughness: 1,
		Abilities: []kop.Ability{{Kind: kop.AbActivated, Prog: []kop.Instr{kop.ExtInstr(kop.ExtPhases)}}},
	})
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	if !sc.k.phasing || sc.k.gen != nil {
		t.Fatalf("precondition: phasing=%v gen=%v, want phasing without gen", sc.k.phasing, sc.k.gen != nil)
	}
	sc.perm(v.GrizzlyBears, 0)
	sc.s.phaseOut(sc.k, 0, false)
	if !sc.s.hiddenAt(0) {
		t.Fatalf("precondition: slot 0 is not phased out")
	}
	sc.s.phaseUntap(sc.k)
	if sc.s.hiddenAt(0) || !sc.s.occ.has(0) {
		t.Fatalf("slot 0 did not phase in")
	}
}
