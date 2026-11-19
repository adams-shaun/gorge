package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Tomb of Annihilation's Oubliette room is "Discard a card and sacrifice an
// artifact, a creature, and a land": its DB line is `Sacrifice | Defined$ You
// | SacValid$ Artifact & Creature & Land | SacEachValid$ True` (the corpus's
// only token-script SacEachValid$ spelling; the card-folder ones are Catch //
// Release, Perilous Predicament, To the Slaughter and Structural Collapse).
// SacEachValid$ splits the SacValid$ list on " & " and sacrifices one
// permanent per listed type, part by part; these tests drive the registered
// Sacrifice handler directly
// directly with that shape, the way sacrifice_test.go drives the plain
// player-targeted sacrifice.

// TestSacrificeEachValidTombOubliette is the head test: the resolving player's
// artifact, creature and land are each eligible for exactly one part, so one
// of each is sacrificed.
func TestSacrificeEachValidTombOubliette(t *testing.T) {
	h := newHost(t, 2)
	art := putBattlefield(h, 0, "Name:Mox Opal\nManaCost:0\nTypes:Artifact\nOracle:x\n")
	crea := putBattlefield(h, 0, "Name:Bear\nTypes:Creature\nPT:2/2\nOracle:x\n")
	land := putBattlefield(h, 0, "Name:Isle\nTypes:Basic Land Island\nOracle:x\n")
	// Precondition the assertions depend on: all three on the battlefield.
	for _, id := range []state.ObjID{art, crea, land} {
		if z := sacZone(h, id); z != state.ZBattlefield {
			t.Fatalf("setup: object %d zone = %v, want battlefield", id, z)
		}
	}

	c := &Ctx{Source: art, Controller: 0}
	sacrificeEachValid(h, c, sacrificeParams(map[string]string{
		"Defined":      "You",
		"SacValid":     "Artifact & Creature & Land",
		"SacEachValid": "True",
	}))

	for id, kind := range map[state.ObjID]string{art: "artifact", crea: "creature", land: "land"} {
		if z := sacZone(h, id); z != state.ZGraveyard {
			t.Fatalf("the %s (%d) zone = %v, want graveyard", kind, id, z)
		}
	}
	sacs := 0
	for _, e := range h.log {
		if events.IsSacrifice(e) {
			sacs++
		}
	}
	if sacs != 3 {
		t.Fatalf("Sacrifice events = %d, want one per part (3)", sacs)
	}
}

// TestSacrificeEachValidSacrificesAMultiPartPermanentOnce pins the
// SacEachValid$ dedup: an artifact creature land satisfies the artifact part,
// so it leaves with that part's sacrifice and no later part sacrifices it
// again -- it is sacrificed once, beside the plain land that answers the land
// part.
func TestSacrificeEachValidSacrificesAMultiPartPermanentOnce(t *testing.T) {
	h := newHost(t, 2)
	acl := putBattlefield(h, 0, "Name:Guildpage\nManaCost:2\nTypes:Artifact Creature Land\nOracle:x\n")
	land := putBattlefield(h, 0, "Name:Isle\nTypes:Basic Land Island\nOracle:x\n")
	if sacZone(h, acl) != state.ZBattlefield || sacZone(h, land) != state.ZBattlefield {
		t.Fatalf("setup: board not ready (%v %v)", sacZone(h, acl), sacZone(h, land))
	}

	c := &Ctx{Source: land, Controller: 0}
	sacrificeEachValid(h, c, sacrificeParams(map[string]string{
		"Defined":      "You",
		"SacValid":     "Artifact & Creature & Land",
		"SacEachValid": "True",
	}))

	if z := sacZone(h, acl); z != state.ZGraveyard {
		t.Fatalf("the artifact creature land (%d) zone = %v, want graveyard", acl, z)
	}
	if z := sacZone(h, land); z != state.ZGraveyard {
		t.Fatalf("the plain land (%d) zone = %v, want graveyard", land, z)
	}
	for _, e := range h.log {
		if events.IsSacrifice(e) && e.Obj == acl {
			return
		}
	}
	t.Fatalf("no Sacrifice event names the artifact creature land %d", acl)
}
