package oracle

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/rules"
)

// TestGorgeETBAttachOffersItself pins gorge's side of the triggered-Attach
// question (kernel TestETBAttachOffersItself is the kernel's): gorge excludes
// an attach ability's own source only on the player-cast path
// (cast_targets.go, legal_targets.go: sa.API == "Attach", CR 701.3a). A
// trigger's ask (trigger_queue.go askTarget -> target_ask.go
// legalTargetCandidates) excludes only the stack object (CR 115.5), so with
// March of the Machines making every noncreature artifact a creature,
// Embercleave's "attach it to target creature you control" is offered
// Embercleave itself, and effAttach then refuses the self-attach. The kernel
// therefore must NOT carry kop.TgfNotSource-style exclusion on triggered
// Attach targets; widening kgen's Equip gate to them would diverge.
func TestGorgeETBAttachOffersItself(t *testing.T) {
	for _, tc := range []struct {
		bf      []string
		options []string
	}{
		{[]string{"March of the Machines"}, []string{"p0:Embercleave"}},
		{[]string{"March of the Machines", "Grizzly Bears"}, []string{"p0:Grizzly Bears", "p0:Embercleave"}},
	} {
		sc := layerScen("etb-attach-self", tc.bf, []string{"Embercleave"},
			[]lsStep{{"op": "cast", "seat": 0, "card": "p0:Embercleave", "mana": "RRRRRR"}}, nil)
		res, err := rules.RunOracleScenarioJSON(scenRegistry(t), sc)
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Fails) != 0 {
			t.Fatalf("%v: %v", tc.bf, res.Fails)
		}
		var asks []rules.OracleDecision
		for _, d := range res.Decisions {
			if d.Kind == "target" {
				asks = append(asks, d)
			}
		}
		if len(asks) != 1 || !slices.Equal(asks[0].OptionRefs, tc.options) {
			t.Fatalf("%v: ETB target asks %+v, want one offering %v", tc.bf, asks, tc.options)
		}
		// the first pick resolves: with only itself on offer the attach is
		// refused (CR 701.3a), so the Equipment is on the battlefield, unattached
		last := res.Snapshots[len(res.Snapshots)-1]
		found := false
		for _, p := range last.Permanents {
			if p.Name == "Embercleave" {
				found = true
				if len(tc.options) == 1 && p.AttachedTo != "" {
					t.Errorf("%v: Embercleave attached to %q, want unattached", tc.bf, p.AttachedTo)
				}
			}
		}
		if !found {
			t.Errorf("%v: Embercleave not on the battlefield at the end", tc.bf)
		}
	}
}
