package oracle

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// Pirate Hat and Thinking Cap each carry a restricted equip beside a plain
// one ("Equip Pirate {1}", CR 702.6a): the cheaper equip targets only a
// creature of its subtype you control. kgen lowers it as gorge expands it, an
// activated Attach at sorcery speed with the subtype target. The recorded
// games (TestEquipRestrictedRecord, EQUIP_RESTRICTED_RECORD=1, played on
// animateRenameEquipPolicy) reach the Equipment attached to a creature of the
// subtype; TestStdSetReprosLockstep replays them, and
// TestEquipRestrictedReprosReach requires the reach.
func equipRestrictedCases() []animateRenameEquipCase {
	return []animateRenameEquipCase{{
		name:   "equip_restricted_pirate_hat_equips_a_pirate",
		prefer: []string{"Pirate Hat", "Spyglass Siren"},
		decks: [2][]string{
			append(deckOf(8, "Pirate Hat"), append(deckOf(12, "Spyglass Siren"), deckOf(20, "Island")...)...),
			deckOf(40, "Forest"),
		},
		reached: equippedSubtype("Pirate Hat", "Pirate"),
	}, {
		name:   "equip_restricted_thinking_cap_equips_a_detective",
		prefer: []string{"Thinking Cap", "Novice Inspector"},
		decks: [2][]string{
			append(deckOf(8, "Thinking Cap"), append(deckOf(12, "Novice Inspector"), deckOf(20, "Plains")...)...),
			deckOf(40, "Forest"),
		},
		reached: equippedSubtype("Thinking Cap", "Detective"),
	}}
}

// equippedSubtype: the named Equipment on the battlefield is attached to a
// creature with subtype sub.
func equippedSubtype(card, sub string) func(g *GorgeSide) bool {
	return func(g *GorgeSide) bool {
		gm := g.E.G
		for i := range gm.Objs {
			o := &gm.Objs[i]
			if o.Zone != state.ZBattlefield || o.Face() == nil || o.Face().Name != card || o.AttachedTo == 0 {
				continue
			}
			host := gm.Obj(o.AttachedTo)
			if host == nil || host.Zone != state.ZBattlefield {
				continue
			}
			if ch := g.E.Chars(host.ID); ch != nil && slices.Contains(ch.Types, "Creature") && slices.Contains(ch.Types, sub) {
				return true
			}
		}
		return false
	}
}

// TestEquipRestrictedReprosReach replays the recorded games: no divergence,
// every scripted action offered, and the restricted Equipment attached.
func TestEquipRestrictedReprosReach(t *testing.T) {
	reg := synthReg(t)
	for _, c := range equipRestrictedCases() {
		t.Run(c.name, func(t *testing.T) {
			r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			script := &ScriptPolicy{Actions: r.Actions}
			res, hit := animateRenameEquipPlay(t, reg, c, r, charmRepeatSet(t, r.Cards), script)
			if res.Div != nil {
				t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if script.Skipped != 0 {
				t.Errorf("%d scripted actions were not offered", script.Skipped)
			}
			if !hit {
				t.Fatal("the recorded game never attached the Equipment to a creature of its subtype")
			}
		})
	}
}

// TestEquipRestrictedRecord plays the cases' policy over seeds until one
// reaches the attachment, and writes the repro. It runs only with
// EQUIP_RESTRICTED_RECORD=1; the games it writes are the committed repros.
func TestEquipRestrictedRecord(t *testing.T) {
	if os.Getenv("EQUIP_RESTRICTED_RECORD") == "" {
		return // a recording tool, not coverage: it returns rather than skips (testdata/poolskips.txt)
	}
	reg := synthReg(t)
	for _, c := range equipRestrictedCases() {
		t.Run(c.name, func(t *testing.T) {
			set := charmRepeatSet(t, c.decks)
			for seed := uint64(1); seed <= 400; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: "autopay", Kernel: "custom"}
				res, hit := animateRenameEquipPlay(t, reg, c, r, set, nil)
				if res.Div != nil {
					t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
				}
				if !hit {
					continue
				}
				r.setActions(res.Actions)
				if err := r.Write(filepath.Join("testdata", "repro", c.name+".repro.json")); err != nil {
					t.Fatal(err)
				}
				t.Logf("seed %d: %d decisions", seed, len(res.Actions))
				return
			}
			t.Fatalf("no seed reached %s", c.name)
		})
	}
}
