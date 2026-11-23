package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// Four-seat edh pods (the Abzan Armor precon, 100 cards a seat) holding Axebane
// Guardian ("{T}: Add X mana in any combination of colors") beside Orzhov
// Signet, Sungrass Prairie and Faeburrow Elder: the table was refused up front
// (comboanymix) and now plays. gorge's PotentialMana prices a Combo Any
// production unbounded in every colour (rules addPotentialMana, producedOpen),
// so the Guardian alone covers a first-only filter's {1} fee, and the filter is
// a census gap beside it (CR 605.3a, 601.2g): its prefix search chains the
// Guardian's mana into the Signet's fee. The kernel's first-only admission
// (filtFirstAdmit, filtOtherMana) left the Guardian out, so with every land
// tapped it priced neither the Signet nor the plays its output pays.

// comboAnyFilterCases are the recordings and the decision each diverged at when
// the Guardian's potential mana was left out of a first-only filter's fee.
var comboAnyFilterCases = []struct {
	name string
	at   int
}{
	// {2}{G} activation (Assault Formation): Signet and Guardian untapped, nothing else
	{"combo_any_first_only_signet_fee_beside_axebane_four_seat_pod", 914},
	// a cast and an activation beside the Guardian, a Signet and Faeburrow Elder
	{"combo_any_filter_fee_activation_second_four_seat_pod", 1064},
	// a cast the Guardian funds through Sungrass Prairie's fee
	{"combo_any_filter_fee_cast_third_four_seat_pod", 1648},
}

// TestFourSeatComboAnyBesideFilterLockstep: each pod plays past its old
// divergence with no divergence and no kernel stall.
func TestFourSeatComboAnyBesideFilterLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	for _, c := range comboAnyFilterCases {
		t.Run(c.name, func(t *testing.T) {
			fourSeatNoStall(t, c.name, c.at)
		})
	}
}

// TestFourSeatComboAnyBesideFilterMutation: leaving the Guardian out of the
// filter's fee cover (dirac.DebugComboAnyNoPotential) must diverge at the same
// decision on every recording, which also shows each reaches the interaction.
func TestFourSeatComboAnyBesideFilterMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	old := dirac.DebugComboAnyNoPotential
	dirac.DebugComboAnyNoPotential = true
	defer func() { dirac.DebugComboAnyNoPotential = old }()
	for _, c := range comboAnyFilterCases {
		res := fourSeatEdhReplay(t, c.name)
		if res.Div == nil && !strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Errorf("%s: the mutation did not diverge", c.name)
			continue
		}
		if res.Div != nil && res.Div.At != c.at {
			t.Errorf("%s: diverged at %d, want %d", c.name, res.Div.At, c.at)
		}
	}
}
