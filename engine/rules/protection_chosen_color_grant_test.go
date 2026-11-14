package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestProtectionChosenColorGrantStatic pins a Continuous static's granted
// `Protection:Card.ChosenColor:chosenColor` (Ward Sliver, Pentarch Ward,
// Benevolent Blessing): the HOST's recorded colour resolves into the derived
// keyword, and the CR 702.16 exception fields spare the named attachments.
func TestProtectionChosenColorGrantStatic(t *testing.T) {
	t.Parallel()
	choose := func(e *Engine, host state.ObjID, letter string) {
		t.Helper()
		e.emit(events.Event{Kind: events.Choose, Obj: host, Counter: "color", Text: letter})
		e.pending = nil
		if got := e.G.Obj(host).ChosenColor; got != letter {
			t.Fatalf("precondition: host ChosenColor = %q, want %q", got, letter)
		}
	}

	t.Run("sliver", func(t *testing.T) {
		const (
			sliverSrc = "Name:Ward Sliver\nManaCost:4 W\nTypes:Creature Sliver\nPT:2/2\n" +
				"S:Mode$ Continuous | Affected$ Sliver | AddKeyword$ Protection:Card.ChosenColor:chosenColor\nOracle:x\n"
			blueSrc = "Name:Blue Bear\nManaCost:U\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
			redSrc  = "Name:Red Bear\nManaCost:R\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
		)
		e, cfg, _ := newFixtureDeck(t, 81, sliverSrc)
		sliver := putCreature(t, e, 0, sliverSrc)
		blue := putToken(t, e, 1, blueSrc, state.ZBattlefield)
		red := putToken(t, e, 1, redSrc, state.ZBattlefield)

		// No recorded choice grants nothing (never a protection from a
		// colour the host did not choose).
		if slices.ContainsFunc(e.Keywords(sliver), func(k string) bool { _, ok := protectionQuality(k); return ok }) {
			t.Fatalf("no choice: keywords=%v, want no protection keyword", e.Keywords(sliver))
		}
		if e.protectedFrom(sliver, blue) || e.protectedFrom(sliver, red) {
			t.Fatal("no choice: protected from a source")
		}

		choose(e, sliver, "U")
		if !slices.Contains(e.Keywords(sliver), "Protection:blue:chosenColor") {
			t.Fatalf("blue chosen: keywords=%v, want the resolved protection keyword", e.Keywords(sliver))
		}
		if !e.protectedFrom(sliver, blue) {
			t.Fatalf("blue chosen: not protected from the blue source; keywords=%v", e.Keywords(sliver))
		}
		if e.protectedFrom(sliver, red) {
			t.Fatal("blue chosen: protected from the red source")
		}
		replayCheck(t, e, cfg)
	})

	t.Run("pentarch exception", func(t *testing.T) {
		const (
			bearSrc = "Name:Plain Bear\nManaCost:G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
			wardSrc = "Name:Pentarch Ward\nManaCost:2 W\nTypes:Enchantment Aura\nK:Enchant:Creature\n" +
				"S:Mode$ Continuous | AffectedDefined$ Enchanted | Affected$ Creature | AddKeyword$ Protection:Card.ChosenColor:chosenColor:Card.CardUID_HostCardUID\nOracle:x\n"
			otherSrc = "Name:White Aura\nManaCost:W\nTypes:Enchantment Aura\nK:Enchant:Creature\nOracle:x\n"
		)
		e, cfg, _ := newFixtureDeck(t, 82, bearSrc, wardSrc, otherSrc)
		bear := putCreature(t, e, 0, bearSrc)
		ward := putCreature(t, e, 0, wardSrc)
		other := putCreature(t, e, 0, otherSrc)
		e.emit(events.Event{Kind: events.Attach, Obj: ward, IDs: []state.ObjID{bear}})
		e.emit(events.Event{Kind: events.Attach, Obj: other, IDs: []state.ObjID{bear}})
		e.pending = nil
		for _, a := range []state.ObjID{ward, other} {
			if o := e.G.Obj(a); o.AttachedTo != bear {
				t.Fatalf("precondition: Aura %d AttachedTo=%d, want the bear %d", a, o.AttachedTo, bear)
			}
		}
		choose(e, ward, "W")
		e.checkStateBased()
		if !slices.ContainsFunc(e.Keywords(bear), func(k string) bool { return k == "Protection:white:chosenColor:Card.CardUID_"+itoa(int(ward)) }) {
			t.Fatalf("white chosen: bear keywords=%v, want the resolved protection with the host's uid", e.Keywords(bear))
		}
		if !e.protectedFrom(bear, other) {
			t.Fatal("precondition: the bear is not protected from another white Aura, so the exception is untested")
		}
		if o := e.G.Obj(other); o.Zone != state.ZGraveyard {
			t.Fatalf("another white Aura stayed on the protected bear (zone %v)", o.Zone)
		}
		if o := e.G.Obj(ward); o.Zone != state.ZBattlefield || o.AttachedTo != bear {
			t.Fatalf("Pentarch Ward fell off its own bearer: zone %v attached to %d", o.Zone, o.AttachedTo)
		}
		replayCheck(t, e, cfg)
	})

	t.Run("benevolent blessing exception is SBA only", func(t *testing.T) {
		const (
			bearSrc  = "Name:Plain Bear\nManaCost:G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
			blessSrc = "Name:Benevolent Blessing\nManaCost:1 W\nTypes:Enchantment Aura\nK:Enchant:Creature\n" +
				"S:Mode$ Continuous | AffectedDefined$ Enchanted | Affected$ Creature | AddKeyword$ Protection:Card.ChosenColor:chosenColor:Aura.ControlledBy Player.PlayerUID_HostCardControllerUID,Equipment.ControlledBy Player.PlayerUID_HostCardControllerUID:SBA\nOracle:x\n"
			ownSrc = "Name:Own White Aura\nManaCost:W\nTypes:Enchantment Aura\nK:Enchant:Creature\nOracle:x\n"
			oppSrc = "Name:Opp White Aura\nManaCost:W\nTypes:Enchantment Aura\nK:Enchant:Creature\nOracle:x\n"
		)
		e, cfg, _ := newFixtureDeckWithOpponentCard(t, 83, blessSrc, ownSrc, oppSrc)
		bear := putToken(t, e, 0, bearSrc, state.ZBattlefield)
		bless := putCreature(t, e, 0, blessSrc)
		own := putCreature(t, e, 0, ownSrc)
		opp := moveSeeded(t, e, 1, oppSrc, state.ZBattlefield)
		for _, a := range []state.ObjID{bless, own, opp} {
			e.emit(events.Event{Kind: events.Attach, Obj: a, IDs: []state.ObjID{bear}})
		}
		e.pending = nil
		for _, a := range []state.ObjID{bless, own, opp} {
			if o := e.G.Obj(a); o.AttachedTo != bear {
				t.Fatalf("precondition: Aura %d AttachedTo=%d, want the bear %d", a, o.AttachedTo, bear)
			}
		}
		if got := e.controllerOf(opp); got != 1 {
			t.Fatalf("precondition: opposing Aura controller = %d, want seat 1", got)
		}
		choose(e, bless, "W")
		e.checkStateBased()
		if !e.protectedFrom(bear, opp) {
			t.Fatalf("white chosen: bear not protected from the opposing white Aura; keywords=%v", e.Keywords(bear))
		}
		// The exception spares an already-attached Aura only; a new one
		// would still be refused (CR 702.16c), so the plain predicate holds.
		if !e.protectedFrom(bear, own) {
			t.Fatal("the SBA-only exception also spared a NEW attachment of the host's controller")
		}
		for _, a := range []state.ObjID{bless, own} {
			if o := e.G.Obj(a); o.Zone != state.ZBattlefield || o.AttachedTo != bear {
				t.Fatalf("Aura %d (controlled by the host's controller) fell off: zone %v attached to %d", a, o.Zone, o.AttachedTo)
			}
		}
		if o := e.G.Obj(opp); o.Zone != state.ZGraveyard {
			t.Fatalf("the opponent's white Aura stayed on the protected bear (zone %v)", o.Zone)
		}
		replayCheck(t, e, cfg)
	})
}
