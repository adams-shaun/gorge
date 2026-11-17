package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestCastFreeMandatoryHidesDone: a mandatory Play (no Optional$ True,
// kgen's freeMandatory bit) offers its candidates with no Done; the same
// offer without the bit has one. The optional control is the precondition
// that the ask was posed and the Done would otherwise be there.
func TestCastFreeMandatoryHidesDone(t *testing.T) {
	run := func(t *testing.T, c int16, wantDone bool) {
		t.Helper()
		sc, synth, id := m11Scen(t, []kop.Instr{{Op: kop.OpCastFree, Obj: kop.RefRemembered, C: c}})
		var rem [2]Obj
		for i := range rem {
			rem[i] = sc.obj(id["Opt"], 0)
			sc.s.place(rem[i], kop.ZoneExile, false)
			sc.s.g.rem[i] = rem[i]
		}
		sc.s.g.nrem = uint8(len(rem))
		m11Start(sc, synth, nil)
		d := m11FreeAsk(t, sc)
		cards, done := 0, false
		for i := 0; i < int(d.N); i++ {
			switch d.Options[i].Kind() {
			case AkChoose:
				cards++
			case AkDone:
				done = true
			}
		}
		if cards != len(rem) {
			t.Fatalf("C %#x: %d card options, want %d", c, cards, len(rem))
		}
		if done != wantDone {
			t.Fatalf("C %#x: Done offered %v, want %v (%d options)", c, done, wantDone, d.N)
		}
	}
	t.Run("mandatory", func(t *testing.T) { run(t, int16(kop.ZoneExile)|freeMandatory, false) })
	t.Run("optional", func(t *testing.T) { run(t, int16(kop.ZoneExile), true) })
}
