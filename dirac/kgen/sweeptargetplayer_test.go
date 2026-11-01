package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// kgen-side regressions for the target-player sweep
// scope. ctrlRoleF2 is entry-wide; gorge's targetedGroup/allPlayersFor read
// only the resolving link's targets, so the lowering must refuse any shape
// where the difference can show.

func lowerSweepSynthetic(t *testing.T, face *cards.Face) *lCard {
	t.Helper()
	card := &cards.Card{Faces: []*cards.Face{face}}
	p := &pool{byName: map[string]*cards.Card{face.Name: card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	return lowerCard(p, face.Name, card, false)
}

func wantRefusal(t *testing.T, c *lCard, want string) {
	t.Helper()
	for _, r := range c.Refusals {
		if strings.Contains(r.Reason, want) {
			return
		}
	}
	t.Fatalf("want a refusal containing %q, got %v", want, c.Refusals)
}

// TestSweeptgtRefusesCrossModePlayerTarget: a multi-pick charm may pick a
// player for mode A's sweep and a player for mode B's other effect. The
// kernel's F2TgtPlayerCtrl matches every chosen player target of the stack
// entry, so mode A's sweep would hit mode B's player too; gorge's
// targetedGroup reads only the resolving link's targets. Refuse, counting
// every player slot of the ability whatever its Mode.
func TestSweeptgtRefusesCrossModePlayerTarget(t *testing.T) {
	face := &cards.Face{
		Name:     "Synthetic",
		ManaCost: "2 W",
		Types:    []string{"Instant"},
		SVars: map[string]string{
			"DBUntap": "DB$ UntapAll | ValidTgts$ Player | ValidCards$ Creature",
			"DBDrain": "DB$ LoseLife | ValidTgts$ Player | LifeAmount$ 1",
		},
		Abilities: []*cards.SA{{
			Kind:   "SP",
			API:    "Charm",
			Params: map[string]string{"Choices": "DBUntap,DBDrain", "CharmNum": "2"},
		}},
	}
	wantRefusal(t, lowerSweepSynthetic(t, face), "sweep ValidTgts$ Player beside another player target slot")
}

// TestSweeptgtRefusesTargetUnique: gorge's effPutCounterAll treats any
// non-empty TargetUnique$ as an exotic shape and places NOTHING, while the
// kernel would sweep. Refuse rather than lower a no-op differently.
func TestSweeptgtRefusesTargetUnique(t *testing.T) {
	face := &cards.Face{
		Name:     "Synthetic",
		ManaCost: "2 W",
		Types:    []string{"Sorcery"},
		Abilities: []*cards.SA{{
			Kind: "SP",
			API:  "PutCounterAll",
			Params: map[string]string{
				"ValidTgts":    "Player",
				"TargetUnique": "True",
				"ValidCards":   "Creature.Other",
				"CounterType":  "P1P1",
				"CounterNum":   "1",
			},
		}},
	}
	wantRefusal(t, lowerSweepSynthetic(t, face), "PutCounterAll TargetUnique$")
}

// TestSweeptgtScopesSecondBatch: the second PutCounterAll batch shares the
// first batch's ValidTgts$ Player scope in gorge (putCounterAllSweep reads
// TargetsOf(sa) on both calls). No corpus card carries both, so cover the
// scoped ValidCards2$ fallback here.
func TestSweeptgtScopesSecondBatch(t *testing.T) {
	face := &cards.Face{
		Name:     "Synthetic",
		ManaCost: "2 W",
		Types:    []string{"Sorcery"},
		Abilities: []*cards.SA{{
			Kind: "SP",
			API:  "PutCounterAll",
			Params: map[string]string{
				"ValidTgts":    "Player",
				"ValidCards":   "Creature",
				"CounterType":  "P1P1",
				"CounterNum":   "1",
				"ValidCards2":  "Creature.Self",
				"CounterType2": "PLAN",
				"CounterNum2":  "1",
			},
		}},
	}
	c := lowerSweepSynthetic(t, face)
	if len(c.Refusals) != 0 {
		t.Fatalf("unexpected refusals: %v", c.Refusals)
	}
	var scoped []*lFilter
	for _, ab := range c.Abilities {
		for _, in := range ab.Prog {
			if in.Op == "PutCounterAll" && in.F != nil {
				scoped = append(scoped, in.F)
			}
		}
	}
	if len(scoped) != 2 {
		t.Fatalf("got %d PutCounterAll instructions, want 2: %v", len(scoped), scoped)
	}
	for i, f := range scoped {
		found := false
		for _, cl := range f.Clauses {
			if cl.Ctrl == whoTgtPlayer {
				found = true
			}
		}
		if !found {
			t.Errorf("batch %d filter %q is not scoped to the player target", i, f)
		}
	}
}
