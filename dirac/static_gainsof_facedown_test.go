package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// gainedOptions counts the gained-ability activations the priority decision
// offers recipient r (static_gainsof.go's gainedMode actions sourced at r).
func (sc *scen) gainedOptions(r Obj) (n int) {
	d := sc.s.Step()
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() == AkActivate && a.Source() == TCode(uint8(r)) && a.Mode()&gainedMode != 0 {
			n++
		}
	}
	return n
}

// TestGainsOfSkipsFaceDownSource: a face-down permanent has no text (CR
// 708.2a), so Marvin's GainsAbilitiesOf$ walk grants nothing from it, where
// the same face-up creature's ping is offered.
func TestGainsOfSkipsFaceDownSource(t *testing.T) {
	set := cardsettest.Set(t, "marvingainsof")
	deck := []cardset.Entry{{Name: "Marvin, Murderous Mimic", Count: 20}}
	other := []cardset.Entry{{Name: "Prodigal Sorcerer", Count: 20}}
	tbl, _, _, err := set.Matchup(deck, other)
	if err != nil {
		t.Fatal(err)
	}
	var marvin, sorcerer CardID
	for i := range tbl.Cards {
		switch tbl.Cards[i].Name {
		case "Marvin, Murderous Mimic":
			marvin = CardID(i)
		case "Prodigal Sorcerer":
			sorcerer = CardID(i)
		}
	}
	sc := newScenTableLib(t, tbl, marvin, Config{NoElide: true})
	m := sc.perm(marvin, 0)
	src := sc.perm(sorcerer, 0)
	if got := sc.gainedOptions(m); got == 0 {
		t.Fatal("precondition: Marvin is offered no gained activation from the face-up Sorcerer")
	}
	sc.s.perms[sc.slot(src)].gty |= gtyFaceDown
	if !sc.s.faceDown(sc.k, sc.slot(src)) {
		t.Fatal("precondition: the Sorcerer slot is not face down")
	}
	if got := sc.gainedOptions(m); got != 0 {
		t.Fatalf("Marvin is offered %d gained activations from a face-down permanent, want 0", got)
	}
}
