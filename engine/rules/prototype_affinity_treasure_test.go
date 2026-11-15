package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
)

const frogmyrSrc = "Name:Frogmyr Test\nManaCost:7\nTypes:Artifact Creature Frog Myr\nPT:4/4\n" +
	"K:Affinity:Artifact\nK:Prototype:3 R:2:2\nOracle:x\n"

// prototypePotentialReason is the verdict PotentialPaymentPlans gives the
// prototype cast of the one card in seat 0's hand, over three lands and a
// Treasure (the only red source and the only artifact).
func prototypePotentialReason(t *testing.T, src string) string {
	t.Helper()
	e := mayPlayBase(t)
	spell := handCard(e, card(t, src), 0)
	for _, b := range []string{
		"Name:PT Island\nTypes:Basic Land Island\nOracle:x\n",
		"Name:PT Island\nTypes:Basic Land Island\nOracle:x\n",
		"Name:PT Plains\nTypes:Basic Land Plains\nOracle:x\n",
		altAddTreasureSrc,
	} {
		onBoardGrant(t, e, 0, b)
	}
	e.priorityRound()
	if d := e.Pending(); d == nil || d.Kind != decision.KPriority {
		t.Fatalf("no priority decision: %+v", d)
	}
	for _, pp := range e.PotentialPaymentPlans(0) {
		if pp.Action.Obj == spell && pp.Action.Mode == "prototype" {
			if pp.Reason == "" && pp.Plan == nil {
				t.Fatal("a priced prototype cast carries no plan")
			}
			return pp.Reason + "/" + pp.Detail
		}
	}
	t.Fatal("the walk lists no prototype cast")
	return ""
}

// The potential walk floats the witness's mana before the cast, so the
// Treasure it sacrifices no longer counts for affinity when the cast is
// priced (CR 601.2f fixed the total before the sacrifice, which the lowering
// cannot reach): the prototype cast over two lands and the Treasure is
// withheld as witness_unreachable, where the same cast without affinity is
// priced. dirac's faceWitnessReaches must agree (lockstep repro
// prototype_affinity_castface).
func TestPrototypeAffinityTreasureWitnessUnreachable(t *testing.T) {
	t.Parallel()
	noAffinity := strings.Replace(frogmyrSrc, "K:Affinity:Artifact\n", "", 1)
	if got := prototypePotentialReason(t, noAffinity); got != "/" {
		t.Fatalf("precondition: the prototype cast without affinity: %q, want priced", got)
	}
	if got := prototypePotentialReason(t, frogmyrSrc); got != "unsupported/witness_unreachable" {
		t.Fatalf("prototype cast with affinity over a Treasure: %q, want unsupported/witness_unreachable", got)
	}
}
