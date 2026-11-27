package kgen

import (
	"strings"
	"testing"
)

// definedRefsSlice3Carriers are the next Defined$ referent spellings gorge
// resolves and kgen refused: ThisTargetedPlayer (the player target of the
// node declaring it: Draw and Token), SourceController (the resolving
// source's controller for the player-valued consumers: Mill and Draw), and
// ParentTargetedController (the parent link's target's controller: a
// DealDamage after a Destroy and a Draw after a charm's Exile).
var definedRefsSlice3Carriers = []string{
	"Cybernetica Datasmith", "Depth Defiler", "Prismari Command",
	"Covenant of Minds",
	"Unlicensed Disintegration", "Season of the Burrow",
}

// definedRefsSlice3StillRefused are the same ThisTargetedPlayer spelling in
// cards whose next blocker is unrelated (PutCounterAll's placer/target
// uniqueness and a loyalty-ability Counter), plus Glorious End: its
// LosesGame reads Defined$ SourceController through Defined (not
// definedPlayers), where gorge fails closed on the unmodelled selector and
// names no player at all, so that shape stays refused rather than turning a
// no-op into the caster's loss.
//
// Shadrix's PutCounterAll refusal is raised while lowering that mode's body,
// before the charm's combined target step (charmMultiTargets) runs, so
// TargetUnique on the Token mode does not move its first refusal. The Token
// mode's Unique is asserted by TestDefinedRefsSlice3Shapes through
// Cybernetica Datasmith.
var definedRefsSlice3StillRefused = map[string]string{
	"Shadrix Silverquill":          "PutCounterAll Placer$",
	"Splinter & Leo, Father & Son": "PutCounterAll TargetUnique$",
	"Verdant Command":              `Counter TargetType "Activated" without Triggered`,
	"Glorious End":                 "LosesGame with Defined$ SourceController",
}

func TestDefinedRefsSlice3(t *testing.T) {
	for _, name := range definedRefsSlice3Carriers {
		if c := lowerNamedCard(t, name); len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
		}
	}
	for name, want := range definedRefsSlice3StillRefused {
		c := lowerNamedCard(t, name)
		var got strings.Builder
		for _, r := range c.Refusals {
			got.WriteString(r.Reason)
			got.WriteString("; ")
		}
		if !strings.Contains(got.String(), want) {
			t.Errorf("%s refusals %q, want one naming %q", name, got.String(), want)
		}
	}
}

// The lowered spellings name the same referent the kernel already models:
// ThisTargetedPlayer is the node's own player target slot, SourceController
// is the resolving controller, and ParentTargetedController is the parent
// link's target controller read live (a target that left reads its owner).
// Cybernetica Datasmith's token carries TargetUnique, the "another target
// player" restriction gorge's target_unique_chain enforces.
func TestDefinedRefsSlice3Shapes(t *testing.T) {
	cases := []struct {
		card   string
		op     string
		obj    refKind
		unique bool
	}{
		{"Cybernetica Datasmith", "CreateToken", rTarget, true},
		{"Depth Defiler", "Draw", rTarget, false},
		{"Prismari Command", "Draw", rTarget, false},
		{"Covenant of Minds", "Mill", rYou, false},
		{"Unlicensed Disintegration", "DealDamage", rTargetCtrlLive, false},
		{"Season of the Burrow", "Draw", rTargetCtrlLive, false},
	}
	for _, tc := range cases {
		c := lowerNamedCard(t, tc.card)
		found := false
		for _, ab := range c.Abilities {
			for _, in := range ab.Prog {
				if in.Op == tc.op && in.Obj.Kind == tc.obj {
					found = true
					if tc.unique {
						tgt := ab.Targets[in.Obj.N]
						if !tgt.Unique || tgt.F == nil || tgt.F.Players == 0 {
							t.Errorf("%s: %s target %d is not a unique player target (%+v)", tc.card, tc.op, in.Obj.N, tgt)
						}
					}
				}
			}
		}
		if !found {
			t.Errorf("%s: no %s with %s ref", tc.card, tc.op, refNames[tc.obj])
		}
	}
}
