package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// planebodiesDigRefusals lowers one synthetic probe plane whose ChaosEnsues
// trigger carries the given DigUntil line and returns its refusal list
// ("Node: Reason" joined).
func planebodiesDigRefusals(t *testing.T, name, digLine string) string {
	t.Helper()
	src := strings.Join([]string{
		"Name:" + name,
		"ManaCost:no cost",
		"Types:Plane Probe",
		"T:Mode$ ChaosEnsues | TriggerZones$ Command | Execute$ PlanebodiesDig | TriggerDescription$ chaos: dig the planar deck",
		"SVar:PlanebodiesDig:" + digLine,
	}, "\n")
	card, diags := cards.ParseBytes(name+".txt", []byte(src))
	if card == nil || len(diags) != 0 {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{names: []string{name}, byName: map[string]*cards.Card{name: card},
		nDecks: map[string]int{}, reg: &cards.Registry{}}
	var out []string
	for _, r := range lowerPool(p).cards[0].Refusals {
		out = append(out, r.Node+": "+r.Reason)
	}
	return strings.Join(out, "; ")
}

// The planar dig's destination rule: a DigZone$ PlanarDeck scan lowers only
// with BOTH placements named (FoundDestination$ PlanarDeck and
// RevealedDestination$ PlanarDeck). An absent FoundDestination$ is gorge's
// found-joins-the-revealed-pile order (diguntil.go's foundJoinedRevealed: the
// pile returns in scan order, e.g. [a,b,F] lands a,b,F), which the kernel's
// found-then-rest move does not model (it lands F,a,b) -- so the shape
// refuses instead of lowering something the two engines place differently.
// The corpus carrier (The Fertile Lands of Saulvinia) names both; its pin is
// TestPlanechasePlaneBodiesRefused's gone row.
func TestPlanebodiesPlanarDigRefused(t *testing.T) {
	dig := "DB$ DigUntil | Valid$ Plane | DigZone$ PlanarDeck | RememberFound$ True | FoundLibraryPosition$ -1 | RevealedDestination$ PlanarDeck | RevealedLibraryPosition$ -1"
	full := dig + " | FoundDestination$ PlanarDeck"

	t.Run("absent FoundDestination refuses", func(t *testing.T) {
		got := planebodiesDigRefusals(t, "Planebodies No Dest Plane", dig)
		const want = "DigUntil planar scan without FoundDestination$ PlanarDeck"
		if !strings.Contains(got, want) {
			t.Fatalf("want refusal %q, got %q", want, got)
		}
	})
	t.Run("absent positions refuse", func(t *testing.T) {
		bare := "DB$ DigUntil | Valid$ Plane | DigZone$ PlanarDeck | RememberFound$ True | FoundDestination$ PlanarDeck | RevealedDestination$ PlanarDeck"
		got := planebodiesDigRefusals(t, "Planebodies No Position Plane", bare)
		const want = "DigUntil planar scan without FoundLibraryPosition$ -1"
		if !strings.Contains(got, want) {
			t.Fatalf("want refusal %q, got %q", want, got)
		}
	})
	t.Run("both destinations named lowers", func(t *testing.T) {
		got := planebodiesDigRefusals(t, "Planebodies Dig Probe Plane", full)
		if strings.Contains(got, "DigUntil planar") {
			t.Fatalf("the dig shape with both destinations named must lower, got %q", got)
		}
	})
}
