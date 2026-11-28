package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// anyColourUnlessLand is the Tendo Ice Bridge shape: a plain {T}: Add {C}
// beside a source-anchored "{T}, Remove a charge counter: Add one mana of any
// color". The open-choice alternative is the only source of a coloured pip,
// so an unless-pay offer that leaves it out hides a payment the window can
// complete.
const anyColourUnlessLand = "Name:Any Colour Isle\nTypes:Land\n" +
	"A:AB$ Mana | Cost$ T | Produced$ C\n" +
	"A:AB$ Mana | Cost$ T SubCounter<1/CHARGE> | Produced$ Any\nOracle:x\n"

// TestUnlessCostPayableOpenChoiceColourSource pins the unless-cost census's
// reading of a source-anchored SubCounter fee whose production is an OPEN
// CHOICE. The payer can choose the colour of Tendo Ice Bridge's "{T}, Remove a
// charge counter: Add one mana of any color", so a {B} tax is payable when
// that ability is the only black source. The census used to price only the
// fixed-production SubCounter shape and dropped the choice-shaped one, so the
// gate hid the Pay option the payment window could complete.
func TestUnlessCostPayableOpenChoiceColourSource(t *testing.T) {
	t.Parallel()
	e := handEngine(t)
	e.G.Players[0].Pool = state.Mana{}
	id := onBoard(t, e, 0, anyColourUnlessLand)
	o := e.G.Obj(id)
	o.AddCounter("CHARGE", 1)
	// Precondition: the source is where the window looks, untapped, carries
	// the counter its activation must remove, and its open-choice production
	// is really in the census this walk reads as a black source.
	if o.Zone != state.ZBattlefield || o.Tapped {
		t.Fatalf("precondition: source %d not an untapped battlefield permanent: %+v", id, o)
	}
	if o.Counter("CHARGE") != 1 {
		t.Fatalf("precondition: source carries %d charge counters, want 1", o.Counter("CHARGE"))
	}
	blackAlt := false
	for _, u := range e.unlessWindowUnits(0) {
		if u.ID != id {
			continue
		}
		for _, a := range u.Alts {
			if a.Counts[state.ManaIndex('B')] > 0 {
				blackAlt = true
			}
		}
	}
	if !blackAlt {
		t.Fatalf("precondition: the open-choice mana alternative is not in the unless window census: %+v", e.unlessWindowUnits(0))
	}
	if !e.UnlessCostPayable(0, "B") {
		t.Fatal("the open-choice ability can produce {B}; a {B} tax must be offered")
	}
	if !e.UnlessCostPayable(0, "U") {
		t.Fatal("the open-choice ability can produce {U}; a {U} tax must be offered")
	}
	if !e.UnlessCostPayable(0, "1") {
		t.Fatal("the open-choice ability can produce one mana; a {1} tax must be offered")
	}
	// Control: without the counter the ability cannot pay its own cost, so the
	// only remaining source is the colourless one.
	o.AddCounter("CHARGE", -1)
	if e.UnlessCostPayable(0, "B") {
		t.Fatal("control: the ability cannot be activated without its charge counter")
	}
}
