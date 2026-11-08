package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TargetedPlayerOwn is the one-token spelling of OwnedBy Targeted: the
// candidate's owner is a player target of the resolving entry (gorge
// matchTargetedPlayerOwn through controlReferentPlayers, which reads
// "Targeted" and "TargetedPlayer" as one code, so the spellings resolve to
// the same player set). Off the battlefield the kernel reads the owner
// through the same F2TgtPlayerCtrl bit a controller role sets (ownerRoleF2);
// on the battlefield or stack the role refuses.

func TestTargetedPlayerOwnLowers(t *testing.T) {
	// the word lowers where the owner role already has a home: a count over
	// graveyard cards (the twin OwnedBy Targeted's shape)
	kp := kfhBuild(t, []string{"Villainous Wrath"}, func(p *pool) {
		c := kfhClone(p.byName["Villainous Wrath"])
		c.Faces[0].SVars["X"] = "Count$ValidGraveyard Card.TargetedPlayerOwn"
		p.byName["Villainous Wrath"] = c
	})
	kp.accepted(t, "Villainous Wrath")
	fs := kp.filtersWith(kop.F2TgtPlayerCtrl)
	if len(fs) != 1 || fs[0].Ctrl != kop.CtrlAny || fs[0].Owner != kop.CtrlAny || fs[0].Zone != kop.ZoneGraveyard {
		t.Fatalf("TargetedPlayerOwn filters %+v, want one graveyard filter reading the owner through F2TgtPlayerCtrl", fs)
	}

	// Binding Negotiation's DB$ ChangeZone Exile->Graveyard pick lowers: the
	// hidden pick keeps the owner role's F2TgtPlayerCtrl filter over the
	// exile zone, and the move is one confirmed ChooseMove into the
	// graveyard.
	kp = kfhBuild(t, []string{"Binding Negotiation"}, nil)
	kp.accepted(t, "Binding Negotiation")
	fs = kp.filtersWith(kop.F2TgtPlayerCtrl)
	found := false
	for _, f := range fs {
		if f.Zone == kop.ZoneExile {
			found = true
		}
	}
	if !found {
		t.Fatalf("Binding Negotiation filters %+v, want one exile filter reading the owner through F2TgtPlayerCtrl", fs)
	}
	moves := 0
	for _, a := range kp.cards["Binding Negotiation"].Abilities {
		for _, in := range a.Prog {
			if in.Op != "ChooseMove" || in.C != zGraveyard || !strings.Contains(in.CS, "from=Exile") || !strings.Contains(in.CS, "Confirm") {
				continue
			}
			moves++
		}
	}
	if moves != 1 {
		t.Fatalf("Binding Negotiation has %d confirmed Exile->Graveyard ChooseMoves, want 1", moves)
	}

	// the word's remaining corpus carrier parses it and refuses on a later
	// shape of its own instruction, never on the word; the refusal below is
	// that later blocker, pinned so a fix is made against this test:
	//   - Strategic Betrayal's ChangeZoneAll Origin$ Battlefield,Graveyard
	for _, tc := range []struct {
		name, blocker string
	}{
		{"Strategic Betrayal", `DB$ ChangeZoneAll: Origin "Battlefield,Graveyard"`},
	} {
		kp := kfhBuild(t, []string{tc.name}, nil)
		c := kp.cards[tc.name]
		var got []string
		for _, r := range c.Refusals {
			got = append(got, r.Reason)
			if strings.Contains(r.Reason, `unknown property "TargetedPlayerOwn"`) {
				t.Fatalf("%s: the word still refuses: %s", tc.name, r.Reason)
			}
		}
		if all := strings.Join(got, " | "); !strings.Contains(all, tc.blocker) {
			t.Fatalf("%s refusals %q, want the later blocker %q", tc.name, all, tc.blocker)
		}
	}
}
