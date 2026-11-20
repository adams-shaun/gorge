package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
)

// CR 602.2b: potentialTwinCut is gorge's half of the lockstep twin-cut
// mirror. dirac's isAbilityPlay cuts a printed activation's base encoding and
// a gained activation's 0xC0 pair, and walks every 0x80 grant; the option
// shapes below are the ones engine/rules/legal_walk_battlefield.go's offer
// loops append (printed at :485, gained at :720, SVar grant at :760,
// max-speed grant at :862) and the off-battlefield grant walk
// (legal_walk_offbattlefield_grants.go:115). The SVar-grant shape is the one
// the mirror must exclude even though its Kind is "ability": dirac cannot
// tell it from a max-speed grant by its 0x80 encoding.
func TestPotentialTwinCutMirrorsDiracAbilityEncoding(t *testing.T) {
	printed := decision.Option{Kind: "ability", Obj: 7, Ability: 0}
	printedAlt := decision.Option{Kind: "ability", Obj: 7, Ability: 3, AltCostIndex: 1}
	gained := decision.Option{Kind: "ability", Obj: 7, GainedSource: 13, GainedIdx: 1}
	svarGrant := decision.Option{Kind: "ability", Obj: 7, SVar: "ABMana", GrantSource: 9}
	offFieldGrant := decision.Option{Kind: "ability", Obj: 7, SVar: "ABMana", GrantSource: 11}
	maxSpeed := decision.Option{Kind: "granted", Obj: 7, SVar: "ABMana"}
	exileGrant := decision.Option{Kind: "granted", Obj: 7, SVar: "ABMana"}
	cast := decision.Option{Kind: "cast", Obj: 7}

	// Preconditions: the SVar grant really is Kind "ability" (the round-2
	// MAJOR's population) and carries both anchors, while a max-speed or exile
	// grant is the separate Kind "granted"; a gained ability is anchored, not
	// printed. A test that stopped distinguishing these shapes would prove
	// nothing.
	if svarGrant.Kind != printed.Kind || svarGrant.SVar == "" || svarGrant.GrantSource == 0 {
		t.Fatalf("precondition: the SVar grant shape is a Kind %q option with non-empty anchors: %+v", printed.Kind, svarGrant)
	}
	if maxSpeed.Kind == svarGrant.Kind || exileGrant.Kind != maxSpeed.Kind {
		t.Fatalf("precondition: max-speed Kind %q and exile Kind %q are the separate granted Kind, not the SVar grant's %q",
			maxSpeed.Kind, exileGrant.Kind, svarGrant.Kind)
	}
	if gained.GainedSource == 0 || gained.GainedSource == svarGrant.GrantSource {
		t.Fatalf("precondition: the gained anchor %d must be non-zero and distinct from the grant anchor %d", gained.GainedSource, svarGrant.GrantSource)
	}

	for _, tc := range []struct {
		name string
		opt  decision.Option
		cut  bool
	}{
		{"printed", printed, true},
		{"printed-alt-cost", printedAlt, true},
		{"gained", gained, true},
		{"svar-grant", svarGrant, false},
		{"off-battlefield-svar-grant", offFieldGrant, false},
		{"max-speed-grant", maxSpeed, false},
		{"exile-grant", exileGrant, false},
		{"cast", cast, false},
	} {
		if got := potentialTwinCut(tc.opt); got != tc.cut {
			t.Errorf("%s: potentialTwinCut = %v, want %v (option %+v)", tc.name, got, tc.cut, tc.opt)
		}
	}
}
