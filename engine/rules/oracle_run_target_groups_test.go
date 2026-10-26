package rules

// Regression test for the oracle runner's target matcher and
// decision.Option.Group. A CanRepeatModes$ Charm poses ONE combined target
// ask whose options carry one exclusivity Group per chosen-mode instance (CR
// 700.2d, CR 115.3). matchPick used to return the first unused option whose
// object matched, ignoring Group, so a later mode's target could land on an
// option belonging to a group an earlier pick already filled; Submit then
// rejected the answer as "choices ... are mutually exclusive". The matcher
// must skip an already-used Group and take the matching option in the next
// unused one.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestOracleRunTargetGroups(t *testing.T) {
	reg := testutil.CorpusRegistry(t)

	// Unite the Coalition (CharmNum$ 5, CanRepeatModes$ True). The phase-out
	// mode is chosen twice, so its two instances offer Grizzly Bears in two
	// separate groups. The damage mode is chosen last and also offers Serra
	// Angel, which the FIRST phase-out group offers too: without the Group
	// check the damage pick lands back in that used group.
	sc := oracleScenario{
		Name: "oracle-run-target-groups",
		Setup: map[string]oracleSeat{
			"p0": {Hand: []string{"Unite the Coalition"}},
			"p1": {Battlefield: []string{"Grizzly Bears", "Serra Angel", "Ornithopter"}},
		},
		Steps: []oracleStep{
			{Op: "cast", Seat: 0, Card: "p0:Unite the Coalition", Mana: "WUBRGCC",
				Targets: []string{"p1:Grizzly Bears", "p1:Grizzly Bears", "p1", "p1", "p1:Serra Angel"},
				Answers: []oracleAnswer{{Kind: "modes", Pick: []string{
					"Target permanent phases out.",
					"Target permanent phases out.",
					"Target player draws a card.",
					"Exile target player's graveyard.",
					"CARDNAME deals 2 damage to any target.",
				}}}},
			{Op: "resolve"},
		},
	}
	fails, _, run := runOracleScenario(reg, sc)
	if len(fails) > 0 {
		t.Fatalf("target-group scenario failed:\n  %s", strings.Join(fails, "\n  "))
	}

	var target *OracleDecision
	for i := range run.decisions {
		if run.decisions[i].GorgeKind == "target" {
			target = &run.decisions[i]
		}
	}
	if target == nil {
		t.Fatal("precondition: the scenario posed no target decision")
	}
	if len(target.PickIdx) != 5 {
		t.Fatalf("target answer submitted %v, want five picks (one per mode instance)", target.PickIdx)
	}

	// The mutation trigger: the first pick (Grizzly Bears, the first phase-out
	// instance) and the last pick (Serra Angel, the damage mode) must land in
	// DIFFERENT groups. Without the Group check the damage pick takes the
	// first unused Serra Angel option, which is the first group's, and Submit
	// rejects the pair. The decision also offers two Grizzly Bears options
	// (one per phase-out instance), so the same creature really is in two
	// groups.
	first, second, last := target.PickIdx[0], target.PickIdx[1], target.PickIdx[len(target.PickIdx)-1]
	firstGroup := target.OptionGroups[first]
	if firstGroup == "" {
		t.Fatalf("precondition: first target pick has no Group: %v", target.OptionGroups)
	}
	if !strings.Contains(target.Picks[0], "Grizzly Bears") || !strings.Contains(target.Picks[1], "Grizzly Bears") {
		t.Fatalf("precondition: first two picks = %v, want Grizzly Bears from both phase-out instances", target.Picks)
	}
	if target.OptionGroups[second] == firstGroup {
		t.Fatalf("precondition: both phase-out picks share group %q: %v", firstGroup, target.PickIdx)
	}
	if !strings.Contains(target.Picks[len(target.Picks)-1], "Serra Angel") {
		t.Fatalf("precondition: last pick = %q, want the damage mode's Serra Angel", target.Picks[len(target.Picks)-1])
	}
	if target.OptionGroups[last] == firstGroup {
		t.Fatalf("precondition: the damage pick is already in the first group %q", firstGroup)
	}
	serraOptions := 0
	for _, ref := range target.OptionRefs {
		if strings.Contains(ref, "Serra Angel") {
			serraOptions++
		}
	}
	if serraOptions < 2 {
		t.Fatalf("precondition: %d Serra Angel options offered, want at least two (a used group's and the damage group's)", serraOptions)
	}

	// One pick per mode instance means one distinct non-empty Group per pick.
	seen := map[string]bool{}
	for _, idx := range target.PickIdx {
		group := target.OptionGroups[idx]
		if group == "" {
			t.Fatalf("pick %d has no Group: %v", idx, target.OptionGroups)
		}
		if seen[group] {
			t.Fatalf("two picks share group %q: %v", group, target.PickIdx)
		}
		seen[group] = true
	}
}
