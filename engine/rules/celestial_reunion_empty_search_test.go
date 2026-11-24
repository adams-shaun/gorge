package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Celestial Reunion's search says NoShuffle$ True and its Defined$ Remembered
// legs close with an explicit DB$ Shuffle. When X=0 and the library holds no
// creature with mana value 0, the search finds nothing: the empty fetch must
// leave the "Then shuffle" to the script's own DB$ Shuffle (CR 701.24a, one
// randomize of the library; CR 608.2c, instructions in order). This is the
// negative control for Nissa's Pilgrimage: both defer the search shuffle, but
// Nissa has no explicit tail, so its empty leg pays
// (TestFarmNissasPilgrimage). Paying here too would emit two Shuffle events
// and consume the shuffle RNG twice.
func TestCelestialReunionEmptySearchShufflesOnce(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, _ := searchEngine(t, reg, "Celestial Reunion")
	_, d := castSearchSpell(t, e, "Celestial Reunion")
	start := len(e.L.Events)
	if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind != "x" {
		t.Fatalf("precondition: first decision after cast = %+v, want the X choice", d)
	}
	if d.Options[0].Label != "X = 0" {
		t.Fatalf("precondition: X offer = %q, want the absorbing X = 0", d.Options[0].Label)
	}
	submitChoices(t, e, d.Options[0].Index)
	// The optional additional cost offers a creature type and then two
	// behold picks. Answering pays it (the deterministic first options), but
	// the search itself still finds no creature: the deck's creatures all
	// cost more than X=0.
	for n := 0; n < 3; n++ {
		d = e.Pending()
		if d == nil || d.Kind != decision.KChoose {
			break
		}
		if d.Max > 1 {
			picks := make([]int, 0, d.Max)
			for i := 0; i < d.Max && i < len(d.Options); i++ {
				picks = append(picks, d.Options[i].Index)
			}
			submitChoices(t, e, picks...)
			continue
		}
		submitChoices(t, e, d.Options[0].Index)
	}
	for n := 0; n < 20 && len(e.G.Stack) > 0; n++ {
		d = e.Pending()
		if d == nil || d.Kind != decision.KPriority {
			t.Fatalf("precondition: mid-resolution decision %+v while draining the spell", d)
		}
		pass := -1
		for _, o := range d.Options {
			if o.Kind == "pass" {
				pass = o.Index
			}
		}
		if pass < 0 {
			t.Fatalf("precondition: priority decision without a pass option: %+v", d.Options)
		}
		submitChoices(t, e, pass)
	}
	if len(e.G.Stack) != 0 {
		t.Fatalf("precondition: Celestial Reunion did not resolve (stack depth %d)", len(e.G.Stack))
	}

	searched, moved, shuffles := false, 0, 0
	for _, ev := range e.L.Events[start:] {
		switch ev.Kind {
		case events.SearchedLibrary:
			searched = true
		case events.MoveZone:
			if ev.From == state.ZLibrary && (ev.To == state.ZBattlefield || ev.To == state.ZHand) {
				moved++
			}
		case events.Shuffle:
			if ev.Player == 0 {
				shuffles++
			}
		}
	}
	if !searched {
		t.Fatal("precondition: the X=0 search never happened")
	}
	if moved != 0 {
		t.Fatalf("precondition: the X=0 search moved %d library cards, want an empty search leg", moved)
	}
	if shuffles != 1 {
		t.Fatalf("Celestial Reunion's empty search emitted %d Shuffle events for seat 0, want exactly 1 (the script's DB$ Shuffle)", shuffles)
	}
}
