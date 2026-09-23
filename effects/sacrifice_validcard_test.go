package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// TestExpertLevelSafeSacrificesItselfNotTheFirstPermanent is the head test of
// the ValidCard$ fix: Expert-Level Safe's real compiled corpus SA is the one
// player-targeted Sacrifice line in the corpus that carries `ValidCard$
// Card.Self` (its `DBSacrifice` SVar body, reached when its secret-choice
// ability matches). Before the fix effSacrifice read only SacValid$ (absent
// here, defaulting to "Permanent"), so the controller handed over whichever
// permanent sat first in zone order rather than "this artifact".
//
// The board puts a land and a creature on FIRST, in zone order, so an
// implementation that ignores ValidCard$ deterministically sacrifices the
// land (the no-host first-eligible stand-in) and the assertions below fail.
func TestExpertLevelSafeSacrificesItselfNotTheFirstPermanent(t *testing.T) {
	r := testutil.CorpusRegistry(t)
	card, ok := r.Lookup("Expert-Level Safe")
	if !ok {
		t.Fatal("corpus has no Expert-Level Safe")
	}
	f := card.Faces[0]
	sac := cards.ResolveSVar(f.SVars, "DBSacrifice")
	if sac == nil {
		t.Fatal("Expert-Level Safe has no DBSacrifice SVar")
	}
	if sac.API != "Sacrifice" {
		t.Fatalf("DBSacrifice API = %q, want Sacrifice", sac.API)
	}
	// Precondition: this is the exact player-targeted ValidCard$ shape the
	// fix reads. If the compiled params change, the test is vacuous.
	if got := sac.Params["Defined"]; got != "You" {
		t.Fatalf("DBSacrifice Defined = %q, want You", got)
	}
	if got := sac.Params["ValidCard"]; got != "Card.Self" {
		t.Fatalf("DBSacrifice ValidCard = %q, want Card.Self", got)
	}
	if got := sac.Params["SacValid"]; got != "" {
		t.Fatalf("DBSacrifice SacValid = %q, want empty (so the default would be Permanent)", got)
	}

	h := newHost(t, 2)
	// Zone order: land, creature, Safe. The pre-fix code sacrifices the land.
	land := putBattlefield(h, 0, "Name:Mountainside\nTypes:Basic Land Mountain\nOracle:x\n")
	creature := putBattlefield(h, 0, "Name:Grizzly\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	safe := h.g.AddObject(card, 0)
	h.g.Obj(safe.ID).Zone = state.ZBattlefield
	h.g.SetZone(state.ZBattlefield, 0, append(h.g.Zone(state.ZBattlefield, 0), safe.ID))

	// Precondition: three permanents on the battlefield and the Safe NOT
	// first, so the compared values (which permanent gets picked) differ.
	if n := len(h.g.Zone(state.ZBattlefield, 0)); n != 3 {
		t.Fatalf("battlefield size = %d, want 3", n)
	}
	if first := h.g.Zone(state.ZBattlefield, 0)[0]; first != land {
		t.Fatalf("first battlefield permanent = %d, want the land %d", first, land)
	}

	c := &Ctx{Source: safe.ID, Controller: 0}
	effSacrifice(h, c, sac)

	if got := sacZone(h, safe.ID); got != state.ZGraveyard {
		t.Fatalf("Safe zone = %v, want graveyard (ValidCard$ Card.Self must make it sacrifice itself)", got)
	}
	if got := sacZone(h, land); got != state.ZBattlefield {
		t.Fatalf("land zone = %v, want battlefield (the first permanent must not be taken)", got)
	}
	if got := sacZone(h, creature); got != state.ZBattlefield {
		t.Fatalf("creature zone = %v, want battlefield", got)
	}
	// Exactly one eligible permanent means there is no choice to record, so
	// no KChoose may be posed (and the no-host Note path must not run either).
	if h.askCount != 0 {
		t.Fatalf("posed %d ask(s), want none: a single eligible permanent has no choice", h.askCount)
	}
}

// TestValidCardNarrowsThePlayerTargetPool guards the class rather than the
// one card: an authored `Defined$ You` Sacrifice with a ValidCard$ filter
// wider than Card.Self must offer only its matches. Two lands and one
// creature are on the battlefield; `ValidCard$ Creature` must choose among
// the creature and leave both lands alone. The filter is conjoined with the
// default `Permanent`, so this also proves SacValid$'s default is not
// bypassed.
func TestValidCardNarrowsThePlayerTargetPool(t *testing.T) {
	h := newHost(t, 2)
	landA := putBattlefield(h, 1, "Name:Alpha Land\nTypes:Basic Land Mountain\nOracle:x\n")
	landB := putBattlefield(h, 1, "Name:Beta Land\nTypes:Basic Land Forest\nOracle:x\n")
	creature := putBattlefield(h, 1, "Name:Victim\nManaCost:1 B\nTypes:Creature Zombie\nPT:2/2\nOracle:x\n")

	c := &Ctx{Source: 0, Controller: 0, Targets: []state.Target{{Player: 1, IsPlayer: true}}}
	sac := sacrificeParams(map[string]string{
		"Defined": "Targeted", "ValidCard": "Creature",
	})
	effSacrifice(h, c, sac)

	if got := sacZone(h, creature); got != state.ZGraveyard {
		t.Fatalf("creature zone = %v, want graveyard", got)
	}
	if got := sacZone(h, landA); got != state.ZBattlefield {
		t.Fatalf("first land zone = %v, want battlefield", got)
	}
	if got := sacZone(h, landB); got != state.ZBattlefield {
		t.Fatalf("second land zone = %v, want battlefield", got)
	}
}

// TestValidCardSelfOnlyMatchesTheSource proves Card.Self in ValidCard$ is
// relative to the resolving source: an authored player-targeted sacrifice
// whose source is a DIFFERENT player's permanent must sacrifice nothing when
// that source is not among the sacrificing player's permanents.
func TestValidCardSelfOnlyMatchesTheSource(t *testing.T) {
	h := newHost(t, 2)
	other := putBattlefield(h, 0, "Name:Source\nTypes:Artifact\nOracle:x\n")
	p1Land := putBattlefield(h, 1, "Name:Their Land\nTypes:Basic Land Island\nOracle:x\n")

	// The source belongs to seat 0 but the ask is over seat 1's permanents:
	// Card.Self matches only the source, which is not in seat 1's pool.
	c := &Ctx{Source: other, Controller: 0, Targets: []state.Target{{Player: 1, IsPlayer: true}}}
	effSacrifice(h, c, sacrificeParams(map[string]string{
		"Defined": "Targeted", "ValidCard": "Card.Self",
	}))

	if got := sacZone(h, p1Land); got != state.ZBattlefield {
		t.Fatalf("opponent land zone = %v, want battlefield (Card.Self names only the source)", got)
	}
	if got := sacZone(h, other); got != state.ZBattlefield {
		t.Fatalf("source zone = %v, want battlefield (the ask is over seat 1's permanents)", got)
	}
	if h.askCount != 0 {
		t.Fatalf("posed %d ask(s), want none: no eligible permanent means no choice", h.askCount)
	}
}
