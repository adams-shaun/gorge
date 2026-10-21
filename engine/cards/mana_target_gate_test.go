package cards

// Task agent-20261009T085207Z-2ef0d22c / agent-20261010T044009Z-8af5f286:
// CR 605.1a -- an ability that requires a target is not a mana ability.
// IsManaAbilitySA gates on a ValidTgts$ anywhere in the ability's SubAbility$
// chain, so the printed AB$ Mana carriers that target -- on the head (Radiant
// Lotus, The Warring Triad, Jetfire) or only through a sub-ability (Witch
// Engine's ChangeControl, the two Chandra loyalty abilities' damage) -- leave
// Face.ManaAbilities and take the ordinary activated-ability path, while
// every other AB$ Mana, AB$ ManaReflected and the four ChooseColor chain
// carriers classify as before.

import (
	"slices"
	"testing"
)

// headTargetingManaCarriers are the printed activated Mana abilities whose own
// parameters declare ValidTgts$ (measured 2026-10-09 over the pinned corpus;
// AB$ ManaReflected has none).
var headTargetingManaCarriers = []string{"Radiant Lotus", "The Warring Triad", "Jetfire, Ingenious Scientist"}

// chainTargetingManaCarriers are the printed Mana abilities that target only
// through a SubAbility$ ("Target opponent gains control of Witch Engine"; the
// two Chandra loyalty abilities' "deals damage to target player"). The
// whole-chain gate moves them with the head carriers; the leaf pins that
// population so a corpus change moves it deliberately.
var chainTargetingManaCarriers = []string{"Witch Engine", "Chandra, Bold Pyromancer", "Chandra, Dressed to Kill"}

func TestManaAbilityTargetGateMovesTheTargetingCarriers(t *testing.T) {
	reg := openCardsTestCorpus(t)
	var head, chain, ungated []string
	for _, c := range reg.AllCards() {
		name := c.Faces[0].Name
		for _, f := range c.Faces {
			for _, a := range f.Abilities {
				if a == nil || !a.IsActivated() || !IsManaAbilityAPI(a.API) {
					continue
				}
			if !requiresTarget(a) {
				// A library-moving ability is the other CR 605.1a gate (TestCR605_1aManaAbilityLibraryGateMovesTheLibraryCarriers).
				if !IsManaAbilitySA(a) && !movesLibraryCard(a) {
					t.Errorf("%s: an untargeted %s ability no longer classifies: %s", name, a.API, a.Line)
				}
				continue
			}
				if IsManaAbilitySA(a) {
					t.Errorf("%s: a targeting %s ability still classifies as a mana ability: %s", name, a.API, a.Line)
					ungated = append(ungated, name)
				}
				if slices.Contains(f.ManaAbilities(), a) {
					t.Errorf("%s: a targeting ability is listed in Face.ManaAbilities(): %s", name, a.Line)
				}
				// A copy without Sub reads the head's own parameters only.
				if requiresTarget(&SA{Params: a.Params}) {
					head = append(head, name)
				} else {
					chain = append(chain, name)
				}
			}
		}
	}
	if len(ungated) != 0 {
		t.Errorf("chain-targeting Mana abilities that still classify = %v, want none", ungated)
	}
	slices.Sort(head)
	slices.Sort(chain)
	wantHead := slices.Sorted(slices.Values(headTargetingManaCarriers))
	wantChain := slices.Sorted(slices.Values(chainTargetingManaCarriers))
	if !slices.Equal(head, wantHead) {
		t.Errorf("head-targeting Mana abilities = %v, want %v", head, wantHead)
	}
	if !slices.Equal(chain, wantChain) {
		t.Errorf("sub-ability-targeting Mana abilities = %v, want %v", chain, wantChain)
	}
}

// TestRequiresTargetWalksTheChainBoundedByManaChainDepth pins the walk on
// hand-built chains: a target on the head, a target on a sub, a target past
// the depth cap (not read), and a cycle (terminates).
func TestRequiresTargetWalksTheChainBoundedByManaChainDepth(t *testing.T) {
	node := func(tgt bool, sub *SA) *SA {
		sa := &SA{Kind: "DB", API: "Mana", Sub: sub, Params: map[string]string{}}
		if tgt {
			sa.Params["ValidTgts"] = "Player"
		}
		return sa
	}
	if requiresTarget(node(false, node(false, nil))) {
		t.Errorf("an untargeted two-node chain requires a target")
	}
	if !requiresTarget(node(true, nil)) {
		t.Errorf("a head ValidTgts$ was not read")
	}
	if !requiresTarget(node(false, node(false, node(true, nil)))) {
		t.Errorf("a ValidTgts$ two subs down was not read")
	}
	// The target sits at depth manaChainDepth: one node past the cap.
	tail := node(true, nil)
	for i := 0; i < manaChainDepth; i++ {
		tail = node(false, tail)
	}
	if requiresTarget(tail) {
		t.Errorf("a ValidTgts$ past manaChainDepth was read")
	}
	cyc := node(false, nil)
	cyc.Sub = cyc
	if requiresTarget(cyc) {
		t.Errorf("a cyclic untargeted chain requires a target")
	}
}
